package channels

import (
	"encoding/json"
	"testing"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// liveChannel creates a NewAPI channel directly in the fake.
func (e *env) liveChannel(t *testing.T, name, group string, models ...string) int {
	t.Helper()
	id, err := e.fake.Create(e.ctx, ports.ChannelCreate{Type: 58, Name: name, Key: "sk-a", Models: models, Group: group})
	must(t, err)
	return id
}

// bind attaches a vendor to a live channel (by group) and applies the resulting draft, if any.
func (e *env) bind(t *testing.T, id int, vendor string) {
	t.Helper()
	issues, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Ext: ExtFields{Vendor: p(vendor)}}})
	must(t, err)
	if len(issues) > 0 {
		t.Fatalf("issues: %+v", issues)
	}
	res, issues, err := e.svc.Apply(e.ctx, []int{id})
	must(t, err)
	// a channel that already carries the vendor group has nothing to change: no draft, no apply
	if len(issues) > 0 || len(res) > 1 || len(res) == 1 && !res[0].OK {
		t.Fatalf("apply: %+v %+v", res, issues)
	}
}

func (e *env) liveGroup(t *testing.T, id int) string {
	t.Helper()
	c, err := e.fake.Get(e.ctx, id)
	must(t, err)
	return c.Group
}

func TestVendorClearRemovesOnlyVendorGroup(t *testing.T) {
	e := newEnv(t)
	id := e.liveChannel(t, "a", "default,anthropic", "test-model")
	e.bind(t, id, "anthropic")
	issues, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Ext: ExtFields{Vendor: p("")}}})
	must(t, err)
	if len(issues) > 0 {
		t.Fatalf("issues: %+v", issues)
	}
	if r := mustRow(t, e, id); r.Group != "default" || r.Vendor != "" {
		t.Fatalf("row: %+v", r)
	}
	res, _, err := e.svc.Apply(e.ctx, []int{id})
	must(t, err)
	if len(res) != 1 || !res[0].OK || e.liveGroup(t, id) != "default" {
		t.Fatalf("apply: %+v group=%q", res, e.liveGroup(t, id))
	}
}

func TestValidationManagedGroups(t *testing.T) {
	e := newEnv(t)
	id := e.liveChannel(t, "a", "default,anthropic", "test-model")
	e.bind(t, id, "anthropic")
	// the vendor group is present next to a non-managed one: valid
	if issues, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Fields: Fields{Tag: p("x")}}}); err != nil || len(issues) > 0 {
		t.Fatalf("default,anthropic must pass: %v %v", issues, err)
	}
	// the vendor follows the groups: dropping the vendor group is fine (the channel has no vendor now)
	if issues, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Fields: Fields{Group: p("default")}}}); err != nil || len(issues) > 0 {
		t.Fatalf("no vendor group is valid: %v %v", issues, err)
	}
	cases := map[string]struct{ group, field string }{
		"two vendor groups":      {"anthropic,gemini", "group"},
		"models of other vendor": {"gemini", "models"},
	}
	for name, c := range cases {
		issues, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Fields: Fields{Group: p(c.group)}}})
		if err != nil || len(issues) == 0 || issues[0].Field != c.field {
			t.Errorf("%s: expected %s issue, got %v %v", name, c.field, issues, err)
		}
	} // one model in two managed groups is rejected, in two non-managed groups it is fine
	b := e.liveChannel(t, "b", "gemini", "shared-m")
	e.liveChannel(t, "c", "anthropic", "shared-m")
	issues, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: b, Fields: Fields{Tag: p("x")}}})
	must(t, err)
	if len(issues) == 0 || issues[0].Field != "models" {
		t.Fatalf("two managed groups for one model must fail: %+v", issues)
	}
	d := e.liveChannel(t, "d", "vip", "other-m")
	e.liveChannel(t, "f", "foo", "other-m")
	if issues, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: d, Fields: Fields{Tag: p("x")}}}); err != nil || len(issues) > 0 {
		t.Fatalf("non-managed groups must not conflict: %+v %v", issues, err)
	}
}

func TestGroupFieldOnlyManagedPart(t *testing.T) {
	e := newEnv(t)
	id := e.liveChannel(t, "a", "default")
	issues, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Fields: Fields{Group: p("foo,gemini")}}})
	must(t, err)
	if len(issues) > 0 {
		t.Fatalf("issues: %+v", issues)
	}
	if r := mustRow(t, e, id); r.Group != "default,gemini" {
		t.Fatalf("group: %q", r.Group)
	}
}

func TestApplyMergesLiveGroups(t *testing.T) {
	e := newEnv(t)
	id := e.liveChannel(t, "a", "default", "test-model")
	issues, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Ext: ExtFields{Vendor: p("anthropic")}}})
	must(t, err)
	if len(issues) > 0 {
		t.Fatalf("issues: %+v", issues)
	}
	// someone edits the groups in NewAPI after the draft was saved
	must(t, e.fake.Update(e.ctx, id, ports.ChannelPatch{Group: p("default,vip")}))
	plan, err := e.svc.Plan(e.ctx, nil)
	must(t, err)
	if len(plan) != 1 || plan[0].Request == nil {
		t.Fatalf("plan: %+v", plan)
	}
	const want = "default,vip,anthropic"
	var shown bool
	for _, c := range plan[0].Changes {
		if c.Field == "group" {
			shown = c.Old == "default,vip" && c.New == want
		}
	}
	patch, ok := plan[0].Request.Body.(ports.ChannelPatch)
	if !shown || !ok || patch.Group == nil || *patch.Group != want {
		t.Fatalf("plan must show the merged group: shown=%v body=%+v", shown, plan[0].Request.Body)
	}
	res, _, err := e.svc.Apply(e.ctx, nil)
	must(t, err)
	if len(res) != 1 || !res[0].OK || e.liveGroup(t, id) != want {
		t.Fatalf("apply: %+v group=%q", res, e.liveGroup(t, id))
	}
}

// appliedChannel creates and applies one vendor-bound channel through the batch flow.
func appliedChannel(t *testing.T, e *env) int {
	t.Helper()
	e.batch(t, "test-model")
	_, _, err := e.svc.Apply(e.ctx, nil)
	must(t, err)
	return mustRows(t, e)[0].ID
}

func TestOldDraftsHaveNoGroupDropOrVendor(t *testing.T) {
	var body draftBody
	must(t, json.Unmarshal([]byte(`{"fields":{"group":"anthropic"},"ext":{}}`), &body))
	if body.GroupDrop != nil {
		t.Fatalf("group_drop: %v", body.GroupDrop)
	}
	// ext.vendor of an old draft is ignored; a stored auto discount is dropped too
	d := store.ChannelDraft{Diff: `{"fields":{},"ext":{"vendor":"anthropic","discount":0.4,"discount_source":"auto"}}`}
	if ext := parseDraft(d).Ext; ext.Vendor != nil || ext.Discount != nil || ext.empty() == false && ext.DiscountSource == nil {
		t.Fatalf("legacy ext: %+v", ext)
	}
}
