package channels

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/internal/vendors"
	"github.com/steven52880/nam/ports"
)

type listFails struct{ ports.ChannelStore }

func (listFails) List(context.Context) ([]ports.Channel, error) {
	return nil, errors.New("boom")
}

func (e *env) vendor(t *testing.T, name string) store.Vendor {
	t.Helper()
	var v store.Vendor
	must(t, e.svc.St.DB.Where("name = ?", name).First(&v).Error)
	return v
}

func inputOf(v store.Vendor) vendors.Input {
	return vendors.Input{Name: v.Name, GroupName: v.GroupName, Models: []string{"test-model"}, ChannelType: v.ChannelType}
}

func TestVendorGroupRenameCreatesDrafts(t *testing.T) {
	e := newEnv(t)
	a := e.liveChannel(t, "bound", "default,anthropic", "test-model")
	e.bind(t, a, "anthropic")
	b := e.liveChannel(t, "plain", "anthropic,vip")
	del := e.liveChannel(t, "doomed", "anthropic")
	must(t, e.svc.MarkDelete(e.ctx, []int{del}))
	prio := e.liveChannel(t, "prio", "default,anthropic", "test-model")
	issues, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: prio, Fields: Fields{Priority: p(int64(5))}}})
	must(t, err)
	if len(issues) > 0 {
		t.Fatalf("issues: %+v", issues)
	}
	created := e.batch(t, "test-model")

	v := e.vendor(t, "anthropic")
	in := inputOf(v)
	in.GroupName = "claude"
	writes := e.fake.Writes
	out, err := e.svc.UpdateVendor(e.ctx, v.ID, in)
	must(t, err)
	if out.GroupDrafts != 4 || out.GroupName != "claude" {
		t.Fatalf("result: %+v", out)
	}
	if e.fake.Writes != writes {
		t.Fatal("UpdateVendor must not write NewAPI")
	}
	if r := mustRow(t, e, del); r.SyncState != StateDraftDelete || len(r.DirtyFields) != 0 {
		t.Fatalf("delete draft must stay untouched: %+v", r)
	}
	if r := mustRow(t, e, prio); r.Priority != 5 || r.Group != "default,claude" || r.SyncState != StateDraft {
		t.Fatalf("priority draft must be kept: %+v", r)
	}
	want := map[int]string{a: "default,claude", b: "claude,vip", created[0]: "claude"}
	for id, g := range want {
		if r := mustRow(t, e, id); r.Group != g {
			t.Errorf("row %d group = %q, want %q", id, r.Group, g)
		}
	}
	if g := e.liveGroup(t, a); g != "default,anthropic" {
		t.Fatalf("live must be untouched until apply: %q", g)
	}

	res, issues, err := e.svc.Apply(e.ctx, nil)
	must(t, err)
	if len(issues) > 0 {
		t.Fatalf("apply issues: %+v", issues)
	}
	for _, r := range res {
		if !r.OK {
			t.Fatalf("apply result: %+v", r)
		}
	}
	if g := e.liveGroup(t, a); g != "default,claude" {
		t.Fatalf("bound channel group: %q", g)
	}
	if g := e.liveGroup(t, b); g != "claude,vip" {
		t.Fatalf("plain channel group: %q", g)
	}
	if g := e.liveGroup(t, prio); g != "default,claude" {
		t.Fatalf("priority channel group: %q", g)
	}
	if c, err := e.fake.Get(e.ctx, prio); err != nil || c.Priority != 5 {
		t.Fatalf("priority not applied: %+v %v", c, err)
	}
}

func TestVendorNameRenameNeedsNoMigration(t *testing.T) {
	e := newEnv(t)
	a := e.liveChannel(t, "bound", "default,anthropic", "test-model")
	e.bind(t, a, "anthropic")
	f := e.liveChannel(t, "pending", "default", "test-model")
	issues, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: f, Ext: ExtFields{Vendor: p("anthropic")}}})
	must(t, err)
	if len(issues) > 0 {
		t.Fatalf("issues: %+v", issues)
	}

	v := e.vendor(t, "anthropic")
	in := inputOf(v)
	in.Name = "claude-ai"
	writes := e.fake.Writes
	out, err := e.svc.UpdateVendor(e.ctx, v.ID, in)
	must(t, err)
	if out.GroupDrafts != 0 || out.Name != "claude-ai" {
		t.Fatalf("result: %+v", out)
	}
	if e.fake.Writes != writes {
		t.Fatal("UpdateVendor must not write NewAPI")
	}
	// the vendor follows the groups: both rows show the new name, the draft keeps only the group change
	if r := mustRow(t, e, f); r.Vendor != "claude-ai" || r.SyncState != StateDraft {
		t.Fatalf("draft row: %+v", r)
	}
	if r := mustRow(t, e, a); r.Vendor != "claude-ai" {
		t.Fatalf("bound row: %+v", r)
	}
	var d store.ChannelDraft
	must(t, e.svc.St.DB.Where("channel_id = ?", f).First(&d).Error)
	if parseDraft(d).Ext.Vendor != nil || strings.Contains(d.Diff, `"vendor"`) {
		t.Fatalf("vendor must not be stored in the draft: %s", d.Diff)
	}
}
func TestVendorRenameConflict(t *testing.T) {
	e := newEnv(t)
	a := e.liveChannel(t, "bound", "default,anthropic", "test-model")
	e.bind(t, a, "anthropic")
	v := e.vendor(t, "anthropic")
	in := inputOf(v)
	in.GroupName = "gemini"
	if _, err := e.svc.UpdateVendor(e.ctx, v.ID, in); resp.Status(err) != 409 {
		t.Fatalf("want 409, got %v", err)
	}
	if got := e.vendor(t, "anthropic"); got.GroupName != "anthropic" {
		t.Fatalf("vendor changed: %+v", got)
	}
	if n, _ := e.svc.DraftCount(e.ctx); n != 0 {
		t.Fatalf("drafts created: %d", n)
	}
}

func TestVendorRenameListFails(t *testing.T) {
	e := newEnv(t)
	v := e.vendor(t, "anthropic")
	e.svc.Channels = listFails{e.svc.Channels}
	in := inputOf(v)
	in.GroupName = "claude"
	if _, err := e.svc.UpdateVendor(e.ctx, v.ID, in); err == nil {
		t.Fatal("expected error when NewAPI cannot be listed")
	}
	if got := e.vendor(t, "anthropic"); got.GroupName != "anthropic" {
		t.Fatalf("vendor changed: %+v", got)
	}
}
