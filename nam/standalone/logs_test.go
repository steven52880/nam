package standalone

import (
	"context"
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
	// 250 rows, AfterID=100: page 1 = 250..151, page 2 = 150..51 (contains id 100) -> stop.
	if len(reqs) != 2 {
		t.Fatalf("expected 2 page requests, got %d", len(reqs))
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
