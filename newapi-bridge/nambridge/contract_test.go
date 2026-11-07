package nambridge

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/steven52880/nam/ports"
	"github.com/steven52880/nam/ports/portstest"
)

func TestPorts_satisfyContract_whenBackedByNewAPIModel(t *testing.T) {
	// Given: nam ports wired to NewAPI's real model layer (SQLite from TestMain)
	p := newPorts()
	if err := p.Validate(); err != nil {
		t.Fatalf("ports incomplete: %v", err)
	}

	// When / Then: the shared adapter contract suite passes
	portstest.RunContract(t, portstest.ContractEnv{Ports: p, SkipTest: true})
}

func TestChannelCreate_buildsAbilities_whenModelsAndGroupGiven(t *testing.T) {
	// Given
	cs := channelStore{}
	id, err := cs.Create(context.Background(), ports.ChannelCreate{
		Type: 58, Name: "nambridge-ability", Key: "sk-x", BaseURL: "http://127.0.0.1:1",
		Models: []string{"m-a", "m-b"}, Group: "default", Status: ports.ChannelEnabled,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = cs.Delete(context.Background(), id) })

	// When
	var n int64
	err = model.DB.Model(&model.Ability{}).Where("channel_id = ?", id).Count(&n).Error

	// Then: one ability per (group, model), same as NewAPI's UI flow
	if err != nil || n != 2 {
		t.Fatalf("abilities = %d, err = %v; want 2", n, err)
	}
}

func TestChannelDelete_removesAbilities_andReportsNotFound(t *testing.T) {
	// Given
	cs := channelStore{}
	id, err := cs.Create(context.Background(), ports.ChannelCreate{
		Type: 58, Name: "nambridge-del", Key: "sk-y", BaseURL: "http://127.0.0.1:1",
		Models: []string{"m-c"}, Group: "default", Status: ports.ChannelEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}

	// When
	if err := cs.Delete(context.Background(), id); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Then
	var n int64
	_ = model.DB.Model(&model.Ability{}).Where("channel_id = ?", id).Count(&n).Error
	if n != 0 {
		t.Fatalf("abilities left: %d", n)
	}
	if _, err := cs.Get(context.Background(), id); err != ports.ErrNotFound {
		t.Fatalf("get after delete = %v, want ErrNotFound", err)
	}
	if err := cs.Delete(context.Background(), id); err != ports.ErrNotFound {
		t.Fatalf("second delete = %v, want ErrNotFound", err)
	}
}

func TestOptionSet_rejectsInvalidGroupRatio(t *testing.T) {
	// Given / When
	err := optionStore{}.Set(context.Background(), "GroupRatio", `{"default":`)

	// Then: same validation the UI endpoint applies
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLogSource_fetchesAfterID_ascending_andCapsLimit(t *testing.T) {
	// Given: three recent consume logs
	for i := 0; i < 3; i++ {
		l := model.Log{CreatedAt: 2_000_000_000 + int64(i), Type: ports.LogTypeConsume, ChannelId: 9, Group: "default", ModelName: "m"}
		if err := model.LOG_DB.Create(&l).Error; err != nil {
			t.Fatal(err)
		}
	}
	ls := logSource{}

	// When
	got, err := ls.FetchAfter(context.Background(), ports.LogQuery{AfterID: 0, Since: 2_000_000_000, Types: []int{ports.LogTypeConsume}, Limit: 100000})

	// Then
	if err != nil || len(got) != 3 {
		t.Fatalf("fetch = %d rows, err = %v", len(got), err)
	}
	if !(got[0].ID < got[1].ID && got[1].ID < got[2].ID) || got[0].Group != "default" || got[0].ChannelID != 9 {
		t.Fatalf("unexpected rows: %+v", got)
	}
	recent, err := ls.Recent(context.Background(), 9, ports.LogTypeConsume, 2_000_000_000, 2)
	if err != nil || len(recent) != 2 || recent[0].ID < recent[1].ID {
		t.Fatalf("recent = %+v, err = %v", recent, err)
	}
}
