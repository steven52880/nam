package standalone

import (
	"context"
	"errors"
	"testing"

	"github.com/steven52880/nam/ports"
)

type portsAlias = ports.Ports

func TestFetchAfterPagesAndSorts(t *testing.T) {
	f, _, p := newTestPorts(t, 1)
	f.seedLogs(250)
	got, err := p.Logs.FetchAfter(context.Background(), ports.LogQuery{AfterID: 100, Since: 1, Types: []int{ports.LogTypeConsume}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 150 || got[0].ID != 101 || got[149].ID != 250 {
		t.Fatalf("got %d logs, first/last = %d/%d", len(got), got[0].ID, got[len(got)-1].ID)
	}
	for i := 1; i < len(got); i++ {
		if got[i].ID != got[i-1].ID+1 {
			t.Fatalf("not ascending/contiguous at %d", i)
		}
	}
	reqs := f.logRequests()
	if len(reqs) == 0 {
		t.Fatal("no log requests")
	}
	for _, q := range reqs {
		if q["start_timestamp"] == nil || q["start_timestamp"][0] == "" {
			t.Fatalf("request without start_timestamp: %v", q)
		}
	}
	// 250 rows: page 1 for total, then oldest-first pages 3 (1..50), 2 (51..150), 1 (151..250).
	if len(reqs) != 4 {
		t.Fatalf("expected 4 page requests, got %d", len(reqs))
	}
}

func TestFetchAfterStopsAtLimitFromOldestPage(t *testing.T) {
	f, _, p := newTestPorts(t, 1)
	f.seedLogs(1000)
	got, err := p.Logs.FetchAfter(context.Background(), ports.LogQuery{AfterID: 0, Since: 1, Types: []int{ports.LogTypeConsume}, Limit: 150})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 150 || got[0].ID != 1 || got[149].ID != 150 {
		t.Fatalf("got %d rows, first/last %d/%d", len(got), got[0].ID, got[len(got)-1].ID)
	}
	// page 1 (total) + pages 10 and 9 only: never walks the newer pages.
	if n := len(f.logRequests()); n != 3 {
		t.Fatalf("expected 3 requests, got %d", n)
	}
}

func TestFetchAfterTwoTypesInterleavedDrainsWithoutGapsOrDuplicates(t *testing.T) {
	// Given 1300 logs of types 2 and 5 with interleaved ids (type 5 every third id).
	f, _, p := newTestPorts(t, 1)
	const n = 1300
	f.seedTypedLogs(n, func(id int) int {
		if id%3 == 0 {
			return ports.LogTypeError
		}
		return ports.LogTypeConsume
	})
	// When FetchAfter is called repeatedly with the advancing cursor.
	var all []int64
	after := int64(0)
	for i := 0; i < 20; i++ {
		got, err := p.Logs.FetchAfter(context.Background(), ports.LogQuery{
			AfterID: after, Since: 1, Types: []int{ports.LogTypeConsume, ports.LogTypeError}, Limit: 500,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) == 0 {
			break
		}
		for _, r := range got {
			all = append(all, r.ID)
		}
		after = got[len(got)-1].ID
	}
	// Then every id 1..n is returned exactly once, ascending.
	if len(all) != n {
		t.Fatalf("got %d ids, want %d", len(all), n)
	}
	for i, id := range all {
		if id != int64(i+1) {
			t.Fatalf("position %d: id %d", i, id)
		}
	}
}

func TestFetchAfterSingleScanAllTypesWithConcurrentInserts(t *testing.T) {
	// Given 450 interleaved type-2/type-5/type-3 logs, and NewAPI receiving a type-2 then a
	// type-5 log after each of the first 30 log requests.
	f, _, p := newTestPorts(t, 1)
	f.seedTypedLogs(450, func(id int) int { return []int{ports.LogTypeConsume, ports.LogTypeError, 3}[id%3] })
	f.afterLogGet = func(n int) {
		if n <= 30 {
			f.appendLogLocked(ports.LogTypeConsume)
			f.appendLogLocked(ports.LogTypeError)
		}
	}
	// When draining with the advancing cursor.
	var all []int64
	after := int64(0)
	for i := 0; i < 100; i++ {
		got, err := p.Logs.FetchAfter(context.Background(), ports.LogQuery{
			AfterID: after, Since: 1, Types: []int{ports.LogTypeConsume, ports.LogTypeError}, Limit: 100,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) == 0 && len(f.logRequests()) > 30 {
			break
		}
		for _, r := range got {
			if r.Type == 3 {
				t.Fatalf("unrequested type in result: %+v", r)
			}
			all = append(all, r.ID)
			after = r.ID
		}
	}
	// Then every type-2/5 id is returned exactly once, ascending, and only one type=0 scan is used.
	var want []int64
	f.mu.Lock()
	for _, l := range f.logs {
		if l["type"].(int) != 3 {
			want = append(want, int64(l["id"].(int)))
		}
	}
	f.mu.Unlock()
	if len(all) != len(want) {
		t.Fatalf("got %d ids, want %d", len(all), len(want))
	}
	for i := range want {
		if all[i] != want[i] {
			t.Fatalf("position %d: id %d want %d", i, all[i], want[i])
		}
	}
	for _, q := range f.logRequests() {
		if q["type"][0] != "0" {
			t.Fatalf("expected type=0 scans only: %v", q)
		}
	}
}

func TestFetchAfterPageCapIsAnError(t *testing.T) {
	f, _, p := newTestPorts(t, 1)
	f.seedLogs(500)
	old := logMaxPages
	logMaxPages = 2
	t.Cleanup(func() { logMaxPages = old })
	got, err := p.Logs.FetchAfter(context.Background(), ports.LogQuery{Since: 1, Types: []int{ports.LogTypeConsume}})
	if !errors.Is(err, errScanTooLong) || got != nil {
		t.Fatalf("want errScanTooLong and no rows, got %d rows err=%v", len(got), err)
	}
}

func TestFetchAfterDefaultsStartTimestamp(t *testing.T) {
	f, _, p := newTestPorts(t, 1)
	if _, err := p.Logs.FetchAfter(context.Background(), ports.LogQuery{}); err != nil {
		t.Fatalf("zero Since must still send start_timestamp: %v", err)
	}
	if len(f.logRequests()) != 1 {
		t.Fatal("expected one request")
	}
}

func TestFetchAfterCapsLimitKeepingOldest(t *testing.T) {
	f, _, p := newTestPorts(t, 1)
	f.seedLogs(250)
	got, err := p.Logs.FetchAfter(context.Background(), ports.LogQuery{AfterID: 0, Since: 1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 10 || got[0].ID != 1 || got[9].ID != 10 {
		t.Fatalf("limit must keep the oldest rows: %+v", got)
	}
}

func TestRecentFiltersByChannelAndDeleteBefore(t *testing.T) {
	f, _, p := newTestPorts(t, 1)
	f.seedLogs(30)
	recent, err := p.Logs.Recent(context.Background(), 7, ports.LogTypeConsume, 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 5 || recent[0].ID != 30 {
		t.Fatalf("recent must be newest first: %+v", recent)
	}
	other, err := p.Logs.Recent(context.Background(), 8, ports.LogTypeConsume, 1, 5)
	if err != nil || len(other) != 0 {
		t.Fatalf("other channel: %v %v", other, err)
	}
	n, err := p.Logs.DeleteBefore(context.Background(), 1011)
	if err != nil || n != 10 {
		t.Fatalf("delete before: n=%d err=%v", n, err)
	}
}
