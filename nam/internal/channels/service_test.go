package channels

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/steven52880/nam/internal/crypto"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
	"github.com/steven52880/nam/ports/portstest"
)

type env struct {
	svc  *Service
	fake *portstest.Fake
	tok  store.UpstreamToken
	up   store.Upstream
	ctx  context.Context
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "nam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	box, _ := crypto.New("k")
	fake := portstest.NewFake()
	up := store.Upstream{Name: "mock-a", Type: store.UpstreamNewAPI, BaseURL: "http://u", CurrencyRate: 0.5}
	must(t, st.DB.Create(&up).Error)
	key, _ := box.Seal([]byte("sk-a"))
	tok := store.UpstreamToken{UpstreamID: up.ID, Name: "a", Group: "a", Key: key}
	must(t, st.DB.Create(&tok).Error)
	must(t, st.DB.Create(&store.Vendor{Name: "anthropic", GroupName: "anthropic", Models: `["test-model"]`, ChannelType: 58}).Error)
	must(t, st.DB.Create(&store.Vendor{Name: "gemini", GroupName: "gemini", Models: `["gem-x"]`, ChannelType: 24, Order: 1}).Error)
	return &env{svc: &Service{St: st, Box: box, Channels: fake.Ports().Channels}, fake: fake, tok: tok, up: up, ctx: context.Background()}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func p[T any](v T) *T { return &v }

func (e *env) batch(t *testing.T, models ...string) []int {
	t.Helper()
	prev, err := e.svc.BatchPreview(e.ctx, BatchRequest{UpstreamID: e.up.ID, TokenIDs: []uint{e.tok.ID},
		TokenModels: map[string][]string{fmt.Sprint(e.tok.ID): models}, Weight: 1})
	must(t, err)
	specs := make([]CreateSpec, len(prev.Items))
	for i, it := range prev.Items {
		specs[i] = it.CreateSpec
	}
	ids, issues, err := e.svc.AddCreateDrafts(e.ctx, specs)
	must(t, err)
	if len(issues) > 0 {
		t.Fatalf("issues: %+v", issues)
	}
	return ids
}

func TestBatchSplitsByVendorAndApplies(t *testing.T) {
	e := newEnv(t)
	must(t, e.svc.St.DB.Create(&store.UpstreamGroup{UpstreamID: e.up.ID, Name: "a", Ratio: 1}).Error)
	prev, err := e.svc.BatchPreview(e.ctx, BatchRequest{UpstreamID: e.up.ID, TokenIDs: []uint{e.tok.ID},
		TokenModels: map[string][]string{fmt.Sprint(e.tok.ID): {"test-model", "gem-x", "mystery"}}})
	must(t, err)
	items, unassigned := prev.Items, prev.Unassigned
	if len(items) != 2 || len(unassigned) != 1 || unassigned[0] != "mystery" {
		t.Fatalf("items=%d unassigned=%v", len(items), unassigned)
	}
	if items[1].ChannelType != 24 || items[0].ChannelType != 58 || items[0].Group != "anthropic" {
		t.Fatalf("vendor mapping wrong: %+v", items)
	}
	ids := e.batch(t, "test-model", "gem-x")
	if len(ids) != 2 || ids[0] >= 0 {
		t.Fatalf("temp ids: %v", ids)
	}
	rows := mustRows(t, e)
	if len(rows) != 2 || rows[0].SyncState != StateDraftCreate {
		t.Fatalf("rows: %+v", rows)
	}
	plan, _ := e.svc.Plan(e.ctx, nil)
	if len(plan) != 2 || plan[0].Request == nil || plan[0].Request.Method != "POST" {
		t.Fatalf("plan: %+v", plan)
	}
	res, issues, err := e.svc.Apply(e.ctx, nil)
	must(t, err)
	if len(issues) > 0 || len(res) != 2 || !res[0].OK || !res[1].OK {
		t.Fatalf("apply: %+v %+v", res, issues)
	}
	live, _ := e.fake.List(e.ctx)
	if len(live) != 2 || live[0].Key != "sk-a" {
		t.Fatalf("live: %+v", live)
	}
	for _, r := range mustRows(t, e) {
		if r.SyncState != StateSynced || !r.Managed || r.UpstreamName != "mock-a" || r.TokenName != "a" {
			t.Fatalf("row after apply: %+v", r)
		}
		if r.ChannelCost == nil || !near(*r.ChannelCost, 0.5/store.DefaultUSDCNYRate) { // group ratio 1 × rate 0.5 ÷ 7.2
			t.Fatalf("cost: %v", r.ChannelCost)
		}
	}
	if n, _ := e.svc.DraftCount(e.ctx); n != 0 {
		t.Fatalf("drafts left: %d", n)
	}
}

func TestEditPruneAndApplyUpdate(t *testing.T) {
	e := newEnv(t)
	e.batch(t, "test-model")
	_, _, _ = e.svc.Apply(e.ctx, nil)
	id := mustRows(t, e)[0].ID

	issues, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Fields: Fields{Priority: p(int64(5))}, Ext: ExtFields{Discount: p(0.35)}}})
	must(t, err)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	r := mustRow(t, e, id)
	if r.SyncState != StateDraft || r.Priority != 5 || r.Discount == nil || *r.Discount != 0.35 || r.DiscountSource != "manual" {
		t.Fatalf("draft row: %+v", r)
	}
	plan, _ := e.svc.Plan(e.ctx, nil)
	if len(plan[0].Changes) != 2 || plan[0].Changes[1].NewAPI {
		t.Fatalf("plan changes: %+v", plan[0].Changes)
	}
	// edit back to original → draft keeps only discount
	_, _ = e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Fields: Fields{Priority: p(int64(0))}}})
	if r := mustRow(t, e, id); len(r.DirtyFields) != 1 || r.DirtyFields[0] != "discount" {
		t.Fatalf("prune: %+v", r.DirtyFields)
	}
	_, _ = e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Fields: Fields{Priority: p(int64(5))}}})
	before := e.fake.Writes
	res, _, _ := e.svc.Apply(e.ctx, nil)
	if !res[0].OK || e.fake.Writes != before+1 {
		t.Fatalf("apply update: %+v writes=%d", res, e.fake.Writes-before)
	}
	if c, _ := e.fake.Get(e.ctx, id); c.Priority != 5 {
		t.Fatalf("priority not written: %d", c.Priority)
	}
	if r := mustRow(t, e, id); r.SyncState != StateSynced || r.Discount == nil || *r.Discount != 0.35 {
		t.Fatalf("after: %+v", r)
	}
}

func TestValidation(t *testing.T) {
	e := newEnv(t)
	e.batch(t, "test-model")
	_, _, _ = e.svc.Apply(e.ctx, nil)
	id := mustRows(t, e)[0].ID
	cases := map[string]Change{
		"wrong vendor model": {ChannelID: id, Fields: Fields{Models: &[]string{"test-model", "gem-x"}}},
		"group mismatch":     {ChannelID: id, Fields: Fields{Group: p("gemini")}},
		"empty name":         {ChannelID: id, Fields: Fields{Name: p(" ")}},
		"bad discount":       {ChannelID: id, Ext: ExtFields{Discount: p(0.0)}},
	}
	for name, ch := range cases {
		issues, err := e.svc.SaveChanges(e.ctx, []Change{ch})
		if err != nil || len(issues) == 0 {
			t.Errorf("%s: expected issues, got %v %v", name, issues, err)
		}
	}
	if n, _ := e.svc.DraftCount(e.ctx); n != 0 {
		t.Fatalf("invalid changes must not be saved: %d", n)
	}
	prev, _ := e.svc.BatchPreview(e.ctx, BatchRequest{UpstreamID: e.up.ID, TokenIDs: []uint{e.tok.ID},
		TokenModels: map[string][]string{fmt.Sprint(e.tok.ID): {"test-model"}}})
	if len(prev.Items) == 0 || !prev.Items[0].Duplicate {
		t.Fatal("duplicate not flagged")
	}
}

func TestCheckReportsRuleIssuesAndAutoDisabled(t *testing.T) {
	e := newEnv(t)
	e.batch(t, "test-model")
	_, _, _ = e.svc.Apply(e.ctx, nil)
	id := mustRows(t, e)[0].ID
	rep, err := e.svc.Check(e.ctx)
	must(t, err)
	if rep.Checked != 1 || len(rep.Issues) != 0 || len(rep.AutoDisabled) != 0 || rep.Issues == nil {
		t.Fatalf("clean: %+v", rep)
	}
	// edits made directly in NewAPI are not "drift" any more: the row simply shows them
	must(t, e.fake.Update(e.ctx, id, ports.ChannelPatch{Weight: p(uint(9)), Status: p(ports.ChannelAutoDisabled)}))
	rep, err = e.svc.Check(e.ctx)
	must(t, err)
	if len(rep.Issues) != 0 || len(rep.AutoDisabled) != 1 {
		t.Fatalf("auto-disabled only: %+v", rep)
	}
	if r := mustRow(t, e, id); r.SyncState != StateSynced || r.Weight != 9 {
		t.Fatalf("row: %+v", r)
	}
	// a model of another vendor and a second vendor group are rule issues, found without any edit
	must(t, e.fake.Update(e.ctx, id, ports.ChannelPatch{Models: &[]string{"test-model", "gem-x"}, Group: p("anthropic,gemini")}))
	other := e.liveChannel(t, "plain", "default")
	rep, err = e.svc.Check(e.ctx)
	must(t, err)
	var fields []string
	for _, is := range rep.Issues {
		if is.ChannelID != id {
			t.Fatalf("unrelated channel %d flagged: %+v", other, is)
		}
		fields = append(fields, is.Field)
	}
	if rep.Checked != 2 || len(rep.Issues) < 2 || !slices.Contains(fields, "group") || !slices.Contains(fields, "models") {
		t.Fatalf("issues: %+v", rep)
	}
	got, err := e.svc.Issues(e.ctx)
	must(t, err)
	if len(got) != len(rep.Issues) {
		t.Fatalf("Issues() = %d, Check = %d", len(got), len(rep.Issues))
	}
}

func TestAdoptStoresNoSnapshot(t *testing.T) {
	e := newEnv(t)
	id := e.liveChannel(t, "old", "default", "test-model")
	must(t, e.svc.Adopt(e.ctx, id, ExtFields{UpstreamID: &e.up.ID, TokenID: &e.tok.ID, Discount: p(0.3)}))
	var ext store.ChannelExt
	must(t, e.svc.St.DB.First(&ext, "channel_id = ?", id).Error)
	if ext.ManualDiscount == nil || *ext.ManualDiscount != 0.3 || ext.UpstreamID == nil || ext.TokenID == nil {
		t.Fatalf("ext: %+v", ext)
	}
	if r := mustRow(t, e, id); !r.Managed || r.DiscountSource != "manual" {
		t.Fatalf("row: %+v", r)
	}
}
func TestDeleteAndFailureKeepsDraft(t *testing.T) {
	e := newEnv(t)
	ids := e.batch(t, "test-model")
	must(t, e.svc.MarkDelete(e.ctx, ids)) // deleting a create draft just drops it
	if n, _ := e.svc.DraftCount(e.ctx); n != 0 {
		t.Fatal("create draft should be dropped")
	}
	e.batch(t, "test-model")
	_, _, _ = e.svc.Apply(e.ctx, nil)
	id := mustRows(t, e)[0].ID
	must(t, e.svc.MarkDelete(e.ctx, []int{id}))
	if r := mustRow(t, e, id); r.SyncState != StateDraftDelete {
		t.Fatal(r.SyncState)
	}
	must(t, e.fake.Delete(e.ctx, id)) // vanished meanwhile: delete still succeeds
	res, _, _ := e.svc.Apply(e.ctx, nil)
	if !res[0].OK {
		t.Fatal(res)
	}
	var tok store.UpstreamToken
	must(t, e.svc.St.DB.First(&tok, e.tok.ID).Error)
	tok.Key = nil
	must(t, e.svc.St.DB.Save(&tok).Error)
	e.batch(t, "test-model")
	res, _, _ = e.svc.Apply(e.ctx, nil)
	if res[0].OK || res[0].Error == "" {
		t.Fatalf("expected failure: %+v", res)
	}
	if r := mustRows(t, e)[0]; r.SyncState != StateError || r.DraftError == "" {
		t.Fatalf("failed draft must stay with error: %+v", r)
	}
}

func mustRows(t *testing.T, e *env) []Row {
	t.Helper()
	rows, err := e.svc.List(e.ctx)
	must(t, err)
	return rows
}

func mustRow(t *testing.T, e *env, id int) Row {
	t.Helper()
	for _, r := range mustRows(t, e) {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("row %d not found", id)
	return Row{}
}
