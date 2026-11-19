package vendorprice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/steven52880/nam/internal/audit"
	"github.com/steven52880/nam/internal/channels"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
	"github.com/steven52880/nam/ports/portstest"
)

// env wires a real nam.db and the in-memory NewAPI fake. Site rate 7, 美元参考汇率 7.2 (store
// defaults); the upstream's currency rate is 7.2, so a channel's 上游渠道售价 equals its discount.
type env struct {
	t    *testing.T
	ctx  context.Context
	st   *store.Store
	fake *portstest.Fake
	raw  ports.OptionStore // unaudited, for seeding and reading NewAPI options
	svc  *Service
	up   uint
	n    int
}

func newEnv(t *testing.T) *env { return newEnvWith(t, nil) }

func newEnvWith(t *testing.T, wrap func(ports.OptionStore) ports.OptionStore) *env {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "nam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fake := portstest.NewFake()
	p := fake.Ports()
	rec := audit.New(st.DB)
	opts := p.Options
	if wrap != nil {
		opts = wrap(opts)
	}
	e := &env{t: t, ctx: context.Background(), st: st, fake: fake, raw: p.Options}
	e.svc = &Service{St: st, Options: rec.Options(opts), Channels: &channels.Service{St: st, Channels: rec.Channels(p.Channels)}, Audit: rec}
	up := store.Upstream{Name: "up", Type: store.UpstreamNewAPI, BaseURL: "http://up", CurrencyRate: 7.2}
	if err := st.DB.Create(&up).Error; err != nil {
		t.Fatal(err)
	}
	e.up = up.ID
	return e
}

func sp(s string) *string { return &s }

func nan() float64 { return math.NaN() }

func nowUnix() int64 { return time.Now().Unix() }

func (e *env) vendor(name, group string) uint {
	e.t.Helper()
	e.n++
	v := store.Vendor{Name: name, GroupName: group, Order: e.n}
	if err := e.st.DB.Create(&v).Error; err != nil {
		e.t.Fatal(err)
	}
	return v.ID
}

// channel adds a NewAPI channel in group with an upstream; its 上游渠道售价 = discount.
func (e *env) channel(group string, prio int64, weight uint, discount float64, status int) int {
	e.t.Helper()
	e.n++
	id, err := e.fake.Create(e.ctx, ports.ChannelCreate{Type: 58, Name: fmt.Sprintf("ch-%d", e.n), Group: group, Priority: prio, Weight: weight, Status: status, Models: []string{"m"}})
	if err != nil {
		e.t.Fatal(err)
	}
	up := e.up
	if err := e.st.DB.Create(&store.ChannelExt{ChannelID: id, UpstreamID: &up, ManualDiscount: &discount}).Error; err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) setOpt(k, v string) {
	e.t.Helper()
	if err := e.raw.Set(e.ctx, k, v); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) opt(k string) string {
	e.t.Helper()
	m, err := e.raw.Get(e.ctx, k)
	if err != nil {
		e.t.Fatal(err)
	}
	return m[k]
}

func (e *env) groupRatio() map[string]float64 {
	e.t.Helper()
	m, err := ParseGroupRatio(e.opt(KeyGroupRatio))
	if err != nil {
		e.t.Fatal(err)
	}
	return m
}

func (e *env) overview() *OverviewResult {
	e.t.Helper()
	ov, err := e.svc.Overview(e.ctx)
	if err != nil {
		e.t.Fatal(err)
	}
	return ov
}

func (e *env) item(group string) Item {
	e.t.Helper()
	for _, it := range e.overview().Items {
		if it.Group == group {
			return it
		}
	}
	e.t.Fatalf("no vendor with group %s", group)
	return Item{}
}

func (e *env) update(id uint, in UpdateInput) *Item {
	e.t.Helper()
	it, err := e.svc.Update(e.ctx, id, in)
	if err != nil {
		e.t.Fatal(err)
	}
	return it
}

func (e *env) apply(in ApplyInput) *ApplyResult {
	e.t.Helper()
	r, err := e.svc.Apply(e.ctx, in, store.TriggerManual)
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}

func (e *env) histories() []store.PriceHistory {
	e.t.Helper()
	var hs []store.PriceHistory
	if err := e.st.DB.Order("id").Find(&hs).Error; err != nil {
		e.t.Fatal(err)
	}
	return hs
}

func (e *env) auditCount(target, action, result string) int64 {
	e.t.Helper()
	var n int64
	q := e.st.DB.Model(&store.AuditLog{}).Where("target = ?", target)
	if action != "" {
		q = q.Where("action = ?", action)
	}
	if result != "" {
		q = q.Where("result = ?", result)
	}
	if err := q.Count(&n).Error; err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *env) row(id uint) store.VendorPricing {
	e.t.Helper()
	var r store.VendorPricing
	if err := e.st.DB.First(&r, id).Error; err != nil {
		e.t.Fatal(err)
	}
	return r
}

func eq(p *float64, want float64) bool { return p != nil && near(*p, want) }

func eqTol(p *float64, want, tol float64) bool { return p != nil && math.Abs(*p-want) <= tol }

func (e *env) rows() int64 {
	e.t.Helper()
	var n int64
	e.st.DB.Model(&store.VendorPricing{}).Count(&n)
	return n
}

func TestGetReadsNewAPIAndNeverWritesTheDatabase(t *testing.T) {
	e := newEnv(t)
	e.vendor("Anthropic", "claude")
	e.vendor("OpenAI", "openai")
	e.setOpt(KeyGroupRatio, `{"claude":0.2857,"default":1}`)
	writes := e.fake.Writes

	it := e.item("claude")
	if it.Mode != store.PricingFixed || !eq(it.SellPrice, 0.2857*7/7.2) || !eq(it.NewAPIRatio, 0.2857) ||
		!eq(it.TargetRatio, 0.2857) || it.DraftRatio != nil || it.Change == nil || *it.Change != 0 || it.State != store.PricingSynced {
		t.Fatalf("fixed mode with no draft reads NewAPI: %+v", it)
	}
	op := e.item("openai")
	if op.Mode != store.PricingFixed || op.SellPrice != nil || op.NewAPIRatio != nil || op.DraftRatio != nil || op.State != store.PricingUnset {
		t.Fatalf("vendor without a NewAPI ratio is unset: %+v", op)
	}
	if e.rows() != 0 || e.fake.Writes != writes {
		t.Fatalf("GET must not write nam.db (%d rows) or NewAPI", e.rows())
	}
	// somebody changes NewAPI by hand: there is no drift, the vendor just follows NewAPI
	e.setOpt(KeyGroupRatio, `{"claude":0.5,"default":1}`)
	if again := e.item("claude"); again.State != store.PricingSynced || !eq(again.SellPrice, 0.5*7/7.2) || !eq(again.NewAPIRatio, 0.5) {
		t.Fatalf("fixed mode follows NewAPI: %+v", again)
	}
	if e.rows() != 0 {
		t.Fatal("GET must not write nam.db")
	}
}

func TestFixedDraftPreviewApplyFlow(t *testing.T) {
	e := newEnv(t)
	id := e.vendor("Anthropic", "claude")
	e.setOpt(KeyGroupRatio, `{"default":1,"vip":2}`)
	e.setOpt(KeyUserUsableGroups, `{"default":"默认"}`)
	e.setOpt(KeyAutoGroups, `["default","claude"]`)
	e.setOpt("ModelRatio", `{"m":1.5}`)

	if it := e.item("claude"); it.State != store.PricingUnset {
		t.Fatalf("before pricing: %+v", it)
	}
	writes := e.fake.Writes
	it := e.update(id, UpdateInput{SellPrice: fp(0.3)})
	if it.State != store.PricingToWrite || !eq(it.TargetRatio, 0.309) || !eq(it.DraftRatio, 0.309) || it.Mode != store.PricingFixed ||
		!eq(it.SellPrice, 0.309*7/7.2) || it.NewAPIRatio != nil || it.Change != nil {
		t.Fatalf("after update: %+v", it)
	}
	if r := e.row(id); r.DraftRatio == nil || *r.DraftRatio != 0.309 || r.Mode != store.PricingFixed {
		t.Fatalf("draft is stored in nam: %+v", r)
	}
	if e.fake.Writes != writes || len(e.groupRatio()) != 2 {
		t.Fatal("editing writes nam only")
	}

	pv, err := e.svc.Preview(e.ctx, ApplyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pv.Vendors) != 1 || !eq(pv.Vendors[0].AfterRatio, 0.309) || pv.Vendors[0].BeforeRatio != nil || pv.Vendors[0].BeforePrice != nil || len(pv.Options) != 3 {
		t.Fatalf("preview: %+v", pv)
	}
	if got := []string{pv.Options[0].Key, pv.Options[1].Key, pv.Options[2].Key}; strings.Join(got, ",") != "GroupRatio,UserUsableGroups,AutoGroups" {
		t.Fatalf("option order: %v", got)
	}
	if e.fake.Writes != writes {
		t.Fatal("preview must not write")
	}
	if e.row(id).DraftRatio == nil {
		t.Fatal("preview must keep the draft")
	}

	res := e.apply(ApplyInput{})
	if strings.Join(res.Written, ",") != "GroupRatio,UserUsableGroups,AutoGroups" || len(res.Vendors) != 1 {
		t.Fatalf("apply: %+v", res)
	}
	gr := e.groupRatio()
	if len(gr) != 3 || gr["default"] != 1 || gr["vip"] != 2 || gr["claude"] != 0.309 {
		t.Fatalf("GroupRatio: %v", gr)
	}
	uug, _ := ParseUserUsableGroups(e.opt(KeyUserUsableGroups))
	if len(uug) != 3 || uug["default"] != "默认" || uug["claude"] != "Anthropic" || uug["auto"] != "自动选择" {
		t.Fatalf("UserUsableGroups: %v", uug)
	}
	if ag, _ := ParseAutoGroups(e.opt(KeyAutoGroups)); strings.Join(ag, ",") != "claude,default" {
		t.Fatalf("AutoGroups: %v", ag)
	}
	if e.opt("ModelRatio") != `{"m":1.5}` {
		t.Fatalf("ModelRatio must not be touched: %s", e.opt("ModelRatio"))
	}

	hs := e.histories()
	if len(hs) != 1 || hs[0].Key != fmt.Sprintf("vendor:%d", id) || hs[0].VendorID == nil || *hs[0].VendorID != id ||
		hs[0].Reason != ReasonManual || hs[0].AuditID == 0 || hs[0].RolledBack {
		t.Fatalf("history: %+v", hs)
	}
	var before, after historyDoc
	_ = json.Unmarshal([]byte(hs[0].Before), &before)
	_ = json.Unmarshal([]byte(hs[0].After), &after)
	if before.Mode != store.PricingFixed || before.SellPrice != nil || before.GroupRatio != nil ||
		after.Mode != store.PricingFixed || !eq(after.SellPrice, 0.309*7/7.2) || !eq(after.GroupRatio, 0.309) {
		t.Fatalf("history docs: %s | %s", hs[0].Before, hs[0].After)
	}
	if n := e.auditCount("option:GroupRatio", "set", store.ResultOK); n != 1 {
		t.Fatalf("GroupRatio audit rows: %d", n)
	}
	if r := e.row(id); r.DraftRatio != nil {
		t.Fatalf("the draft is cleared after the write: %+v", r)
	}
	if it := e.item("claude"); it.State != store.PricingSynced || it.DraftRatio != nil || !eq(it.NewAPIRatio, 0.309) {
		t.Fatalf("overview after apply: %+v", it)
	}

	// nothing left to write: no NewAPI call, no new history
	writes = e.fake.Writes
	if again := e.apply(ApplyInput{}); len(again.Written) != 0 || len(again.Vendors) != 0 || e.fake.Writes != writes || len(e.histories()) != 1 {
		t.Fatalf("second apply: %+v", again)
	}

	// the next change records NewAPI's value as "before"
	e.update(id, UpdateInput{SellPrice: fp(0.35)})
	e.apply(ApplyInput{IncludeGroups: new(bool)})
	hs = e.histories()
	if len(hs) != 2 {
		t.Fatalf("history rows: %d", len(hs))
	}
	_ = json.Unmarshal([]byte(hs[1].Before), &before)
	if !eq(before.SellPrice, 0.309*7/7.2) || !eq(before.GroupRatio, 0.309) {
		t.Fatalf("second history before: %s", hs[1].Before)
	}
	if got := e.groupRatio()["claude"]; got != 0.36 {
		t.Fatalf("0.35 × 7.2 ÷ 7 = 0.36, got %v", got)
	}
}

func TestDraftIsClearedWhenItEqualsNewAPIAndCanBeDiscarded(t *testing.T) {
	e := newEnv(t)
	id := e.vendor("A", "ga")
	e.setOpt(KeyGroupRatio, `{"ga":0.2857}`)

	it := e.update(id, UpdateInput{GroupRatio: fp(0.5)})
	if it.State != store.PricingToWrite || !eq(it.DraftRatio, 0.5) || !eq(it.TargetRatio, 0.5) || !eq(it.NewAPIRatio, 0.2857) {
		t.Fatalf("draft: %+v", it)
	}
	if it.Change == nil || *it.Change <= 0 {
		t.Fatalf("change is relative to NewAPI's current price: %+v", it)
	}
	// back to (about) what NewAPI has: the draft disappears
	it = e.update(id, UpdateInput{GroupRatio: fp(0.286)})
	if it.State != store.PricingSynced || it.DraftRatio != nil || e.row(id).DraftRatio != nil {
		t.Fatalf("a draft equal to NewAPI must be cleared: %+v", it)
	}
	it = e.update(id, UpdateInput{GroupRatio: fp(0.5)})
	if it.DraftRatio == nil {
		t.Fatalf("draft: %+v", it)
	}
	// the same through the price
	it = e.update(id, UpdateInput{SellPrice: fp(0.2857 * 7 / 7.2)})
	if it.State != store.PricingSynced || e.row(id).DraftRatio != nil {
		t.Fatalf("a price that gives NewAPI's ratio clears the draft: %+v", it)
	}

	e.update(id, UpdateInput{GroupRatio: fp(0.5)})
	it = e.update(id, UpdateInput{DiscardDraft: true})
	if it.State != store.PricingSynced || it.DraftRatio != nil || !eq(it.NewAPIRatio, 0.2857) || e.row(id).DraftRatio != nil {
		t.Fatalf("discard: %+v", it)
	}
	if e.groupRatio()["ga"] != 0.2857 {
		t.Fatal("discard never touches NewAPI")
	}
	// discarding when there is nothing to discard is fine
	e.update(id, UpdateInput{DiscardDraft: true})
	// discard cannot be combined with other fields
	for name, in := range map[string]UpdateInput{
		"mode":   {DiscardDraft: true, Mode: sp(store.PricingFixed)},
		"price":  {DiscardDraft: true, SellPrice: fp(0.3)},
		"markup": {DiscardDraft: true, Markup: fp(1)},
		"ratio":  {DiscardDraft: true, GroupRatio: fp(0.3)},
	} {
		if _, err := e.svc.Update(e.ctx, id, in); resp.Status(err) != http.StatusBadRequest {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestSwitchingToFollowClearsTheDraft(t *testing.T) {
	e := newEnv(t)
	id := e.vendor("A", "ga")
	e.setOpt(KeyGroupRatio, `{"ga":0.1}`)
	e.channel("ga", 1, 1, 0.1, ports.ChannelEnabled)
	e.update(id, UpdateInput{GroupRatio: fp(0.5)})
	it := e.update(id, UpdateInput{Mode: sp(store.PricingFollow), Markup: fp(1)})
	if it.DraftRatio != nil || e.row(id).DraftRatio != nil || it.Mode != store.PricingFollow {
		t.Fatalf("follow mode has no draft: %+v", it)
	}
	// and back to fixed: the vendor reads NewAPI again
	it = e.update(id, UpdateInput{Mode: sp(store.PricingFixed)})
	if it.State != store.PricingSynced || !eq(it.TargetRatio, 0.1) {
		t.Fatalf("back to fixed: %+v", it)
	}
}

func TestIncludeGroupsFalseOnlyWritesGroupRatio(t *testing.T) {
	e := newEnv(t)
	id := e.vendor("Anthropic", "claude")
	e.setOpt(KeyAutoGroups, `["default"]`)
	e.update(id, UpdateInput{SellPrice: fp(0.3)})
	res := e.apply(ApplyInput{IncludeGroups: new(bool)})
	if strings.Join(res.Written, ",") != "GroupRatio" || e.opt(KeyAutoGroups) != `["default"]` || e.opt(KeyUserUsableGroups) != "" {
		t.Fatalf("written %v, AutoGroups %s", res.Written, e.opt(KeyAutoGroups))
	}
}

func TestGroupsOnlyChange(t *testing.T) {
	e := newEnv(t)
	e.vendor("Anthropic", "claude")
	e.setOpt(KeyGroupRatio, `{"claude":0.1}`)
	e.setOpt(KeyAutoGroups, `["claude"]`)
	e.setOpt(KeyUserUsableGroups, `{"claude":"c","auto":"a"}`)
	if g := e.overview().Groups; g.Changed {
		t.Fatalf("already right: %+v", g)
	}
	e.vendor("OpenAI", "openai")
	g := e.overview().Groups
	if !g.Changed || strings.Join(g.AutoGroups.Target, ",") != "claude,openai" || g.UserUsableGroups.Target["openai"] != "OpenAI" {
		t.Fatalf("groups: %+v", g)
	}
	res := e.apply(ApplyInput{})
	if strings.Join(res.Written, ",") != "UserUsableGroups,AutoGroups" || len(res.Vendors) != 0 || len(e.histories()) != 0 {
		t.Fatalf("groups-only apply: %+v", res)
	}
}

func TestInvalidOptionJSONIsNeverOverwritten(t *testing.T) {
	for _, key := range optionOrder {
		t.Run(key, func(t *testing.T) {
			e := newEnv(t)
			id := e.vendor("Anthropic", "claude")
			e.setOpt(KeyGroupRatio, `{"default":1}`)
			e.update(id, UpdateInput{SellPrice: fp(0.3)})

			e.setOpt(key, `{bad json`)
			writes := e.fake.Writes
			ov := e.overview()
			if ov.Error == "" || !strings.Contains(ov.Error, key) {
				t.Fatalf("error text should name %s: %q", key, ov.Error)
			}
			if _, err := e.svc.Apply(e.ctx, ApplyInput{}, store.TriggerManual); err == nil {
				t.Fatal("apply must fail")
			}
			if _, err := e.svc.Preview(e.ctx, ApplyInput{}); err == nil {
				t.Fatal("preview must fail")
			}
			if _, err := e.svc.Confirm(e.ctx, id); err == nil {
				t.Fatal("confirm must fail")
			}
			if msg, err := e.svc.Job(e.ctx); err == nil || msg != "" {
				t.Fatalf("job must fail: %q %v", msg, err)
			}
			if e.fake.Writes != writes || e.opt(key) != `{bad json` || len(e.histories()) != 0 {
				t.Fatal("an unparsable option must never be written")
			}
			if e.row(id).DraftRatio == nil {
				t.Fatal("the draft must survive a failed write")
			}
		})
	}
}

func TestUpdateIsRefusedWhenOptionsAreBroken(t *testing.T) {
	e := newEnv(t)
	id := e.vendor("Anthropic", "claude")
	e.setOpt(KeyGroupRatio, `{bad`)
	if ov := e.overview(); ov.Error == "" || len(ov.Items) != 1 || ov.Items[0].State != store.PricingUnset {
		t.Fatalf("overview with broken option: %+v", ov)
	}
	if e.rows() != 0 {
		t.Fatalf("nothing is stored while options are unreadable: %d rows", e.rows())
	}
	if _, err := e.svc.Update(e.ctx, id, UpdateInput{SellPrice: fp(0.3)}); resp.Status(err) != http.StatusConflict {
		t.Fatalf("update: %v", err)
	}
	if e.rows() != 0 {
		t.Fatal("a refused update must not create a row")
	}
}

// liar stores a different GroupRatio than it is given.
type liar struct{ ports.OptionStore }

func (l liar) Set(ctx context.Context, k, v string) error {
	if k == KeyGroupRatio {
		v = `{"claude":123}`
	}
	return l.OptionStore.Set(ctx, k, v)
}

func TestReadBackMismatchIsAnErrorAndNotRecorded(t *testing.T) {
	e := newEnvWith(t, func(o ports.OptionStore) ports.OptionStore { return liar{o} })
	id := e.vendor("Anthropic", "claude")
	e.update(id, UpdateInput{SellPrice: fp(0.3)})
	_, err := e.svc.Apply(e.ctx, ApplyInput{}, store.TriggerManual)
	if err == nil || !strings.Contains(err.Error(), "核对") {
		t.Fatalf("want a read-back error, got %v", err)
	}
	if len(e.histories()) != 0 {
		t.Fatal("a failed verification must not create price_history")
	}
	if r := e.row(id); r.DraftRatio == nil || *r.DraftRatio != 0.309 {
		t.Fatalf("the draft must stay: %+v", r)
	}
	if n := e.auditCount("option:GroupRatio", "verify", store.ResultFail); n != 1 {
		t.Fatalf("failed audit rows: %d", n)
	}
}

// failSet makes Set fail for one key.
type failSet struct {
	ports.OptionStore
	key string
}

func (f failSet) Set(ctx context.Context, k, v string) error {
	if k == f.key {
		return errors.New("boom")
	}
	return f.OptionStore.Set(ctx, k, v)
}

func TestPartialWriteStillRecordsGroupRatio(t *testing.T) {
	e := newEnvWith(t, func(o ports.OptionStore) ports.OptionStore { return failSet{o, KeyUserUsableGroups} })
	id := e.vendor("Anthropic", "claude")
	e.setOpt(KeyAutoGroups, `["default"]`)
	e.update(id, UpdateInput{SellPrice: fp(0.3)})
	_, err := e.svc.Apply(e.ctx, ApplyInput{}, store.TriggerManual)
	if err == nil || !strings.Contains(err.Error(), KeyUserUsableGroups) {
		t.Fatalf("error should name the failed step: %v", err)
	}
	if e.groupRatio()["claude"] != 0.309 || e.opt(KeyAutoGroups) != `["default"]` {
		t.Fatalf("GroupRatio is written, AutoGroups is not: %s %s", e.opt(KeyGroupRatio), e.opt(KeyAutoGroups))
	}
	if len(e.histories()) != 1 || e.row(id).DraftRatio != nil {
		t.Fatal("what was written must be recorded and its draft cleared")
	}
}

func TestJobAutoWritesOnlyFollowModeAndSkipsSmallChange(t *testing.T) {
	e := newEnv(t)
	a := e.vendor("A", "ga")
	b := e.vendor("B", "gb")
	c := e.vendor("C", "gc")
	e.setOpt(KeyGroupRatio, `{"ga":0.1,"gb":0.1,"gc":0.1}`)
	e.channel("ga", 1, 1, 0.1, ports.ChannelEnabled)
	e.channel("gb", 1, 1, 0.1, ports.ChannelEnabled)
	e.channel("gc", 1, 1, 0.0985, ports.ChannelEnabled)
	e.update(a, UpdateInput{Mode: sp(store.PricingFollow), Markup: fp(1)})
	e.update(b, UpdateInput{GroupRatio: fp(0.206)})
	e.update(c, UpdateInput{Mode: sp(store.PricingFollow), Markup: fp(1)})

	items := map[string]Item{}
	for _, it := range e.overview().Items {
		items[it.Group] = it
	}
	if items["ga"].State != store.PricingToWrite || items["ga"].SmallChange || !eq(items["ga"].TargetPrice, 0.1) {
		t.Fatalf("A: %+v", items["ga"])
	}
	if items["gb"].State != store.PricingToWrite || items["gb"].SmallChange || !eq(items["gb"].DraftRatio, 0.206) {
		t.Fatalf("B: %+v", items["gb"])
	}
	if items["gc"].State != store.PricingToWrite || !items["gc"].SmallChange || !eq(items["gc"].TargetPrice, 0.0985) {
		t.Fatalf("C: %+v", items["gc"])
	}

	// auto-write is off by default: the job only computes
	writes := e.fake.Writes
	msg, err := e.svc.Job(e.ctx)
	if err != nil || msg != "3 个厂商；写入 0；待确认 0；无可用渠道 0" || e.fake.Writes != writes {
		t.Fatalf("job (off): %q %v", msg, err)
	}

	if err := e.st.SetKV(e.ctx, store.KeyPricingAutoWrite, "true"); err != nil {
		t.Fatal(err)
	}
	msg, err = e.svc.Job(e.ctx)
	if err != nil || msg != "3 个厂商；写入 1；待确认 0；无可用渠道 0" {
		t.Fatalf("job (on): %q %v", msg, err)
	}
	gr := e.groupRatio()
	if gr["ga"] != 0.103 || gr["gb"] != 0.1 || gr["gc"] != 0.1 {
		t.Fatalf("only the follow-mode vendor with a real change is written: %v", gr)
	}
	if e.row(b).DraftRatio == nil {
		t.Fatal("the job must keep the fixed-mode draft")
	}
	hs := e.histories()
	if len(hs) != 1 || hs[0].Reason != ReasonAuto || hs[0].Key != fmt.Sprintf("vendor:%d", a) {
		t.Fatalf("history: %+v", hs)
	}
	var last store.AuditLog
	if err := e.st.DB.Where("target = ?", "option:GroupRatio").Order("id DESC").First(&last).Error; err != nil || last.Trigger != store.TriggerAuto {
		t.Fatalf("audit trigger: %+v %v", last, err)
	}
	if it := e.item("ga"); it.State != store.PricingSynced {
		t.Fatalf("A after job: %+v", it)
	}
	// the manual preview still lists the small change and the fixed draft
	pv, err := e.svc.Preview(e.ctx, ApplyInput{})
	if err != nil || len(pv.Vendors) != 2 {
		t.Fatalf("preview: %+v %v", pv, err)
	}
}

func TestFollowModeIsWrittenBackWhenNewAPIIsEditedByHand(t *testing.T) {
	e := newEnv(t)
	id := e.vendor("A", "ga")
	e.setOpt(KeyGroupRatio, `{"ga":0.103}`)
	e.channel("ga", 1, 1, 0.1, ports.ChannelEnabled)
	e.update(id, UpdateInput{Mode: sp(store.PricingFollow), Markup: fp(1)})
	if it := e.item("ga"); it.State != store.PricingSynced || !eq(it.TargetRatio, 0.103) {
		t.Fatalf("synced: %+v", it)
	}
	if err := e.st.SetKV(e.ctx, store.KeyPricingAutoWrite, "true"); err != nil {
		t.Fatal(err)
	}
	e.setOpt(KeyGroupRatio, `{"ga":0.11}`) // somebody edits NewAPI
	it := e.item("ga")
	if it.State != store.PricingToWrite || it.SmallChange || !eq(it.NewAPIRatio, 0.11) || it.Change == nil || *it.Change >= 0 {
		t.Fatalf("hand edit: %+v", it)
	}
	msg, err := e.svc.Job(e.ctx)
	if err != nil || msg != "1 个厂商；写入 1；待确认 0；无可用渠道 0" || e.groupRatio()["ga"] != 0.103 {
		t.Fatalf("job: %q %v %s", msg, err, e.opt(KeyGroupRatio))
	}
	if it := e.item("ga"); it.State != store.PricingSynced {
		t.Fatalf("after write back: %+v", it)
	}
	// a hand edit beyond the protection threshold waits for a confirmation
	e.setOpt(KeyGroupRatio, `{"ga":0.5}`)
	if it := e.item("ga"); it.State != store.PricingPending {
		t.Fatalf("protected: %+v", it)
	}
	if msg, err := e.svc.Job(e.ctx); err != nil || msg != "1 个厂商；写入 0；待确认 1；无可用渠道 0" || e.groupRatio()["ga"] != 0.5 {
		t.Fatalf("job: %q %v", msg, err)
	}
	if _, err := e.svc.Confirm(e.ctx, id); err != nil || e.groupRatio()["ga"] != 0.103 {
		t.Fatalf("confirm: %v %s", err, e.opt(KeyGroupRatio))
	}
}

func TestPendingConfirmAndClearing(t *testing.T) {
	e := newEnv(t)
	d := e.vendor("D", "gd")
	e.vendor("E", "ge")
	e.setOpt(KeyGroupRatio, `{"gd":0.1,"ge":0.1}`)
	e.channel("gd", 1, 1, 0.1, ports.ChannelEnabled)
	chE := e.channel("ge", 1, 1, 0.1, ports.ChannelEnabled)
	idE := e.item("ge").VendorID
	e.update(d, UpdateInput{Mode: sp(store.PricingFollow), Markup: fp(2)})
	e.update(idE, UpdateInput{Mode: sp(store.PricingFollow), Markup: fp(2)})
	if err := e.st.SetKV(e.ctx, store.KeyPricingAutoWrite, "true"); err != nil {
		t.Fatal(err)
	}

	it := e.item("gd")
	if it.State != store.PricingPending || !eq(it.TargetPrice, 0.2) || it.Change == nil || *it.Change < 1.05 {
		t.Fatalf("pending: %+v", it)
	}
	writes := e.fake.Writes
	msg, err := e.svc.Job(e.ctx)
	if err != nil || msg != "2 个厂商；写入 0；待确认 2；无可用渠道 0" || e.fake.Writes != writes {
		t.Fatalf("job: %q %v", msg, err)
	}
	pv, err := e.svc.Preview(e.ctx, ApplyInput{})
	if err != nil || len(pv.Vendors) != 0 || len(pv.Skipped) != 2 || pv.Skipped[0].State != store.PricingPending ||
		!strings.Contains(pv.Skipped[0].Reason, "变化 +105.7%") || !strings.Contains(pv.Skipped[0].Reason, "超过变化保护阈值") {
		t.Fatalf("preview: %+v %v", pv, err)
	}

	// the change falls back inside the threshold: pending clears by itself
	if err := e.st.DB.Model(&store.ChannelExt{}).Where("channel_id = ?", chE).Update("manual_discount", 0.06).Error; err != nil {
		t.Fatal(err)
	}
	if it := e.item("ge"); it.State != store.PricingToWrite {
		t.Fatalf("pending should clear: %+v", it)
	}
	// confirm only works on a pending vendor
	if _, err := e.svc.Confirm(e.ctx, idE); resp.Status(err) != http.StatusConflict {
		t.Fatalf("confirm on a to_write vendor: %v", err)
	}

	// confirm writes the current target
	res, err := e.svc.Confirm(e.ctx, d)
	if err != nil || len(res.Vendors) != 1 || strings.Join(res.Written, ",") != "GroupRatio,UserUsableGroups,AutoGroups" {
		t.Fatalf("confirm: %+v %v", res, err)
	}
	if got := e.groupRatio()["gd"]; got != 0.206 {
		t.Fatalf("0.2 × 7.2 ÷ 7 = 0.206, got %v", got)
	}
	hs := e.histories()
	if len(hs) != 1 || hs[0].Reason != ReasonConfirm {
		t.Fatalf("history: %+v", hs)
	}
	if it := e.item("gd"); it.State != store.PricingSynced {
		t.Fatalf("after confirm: %+v", it)
	}
	if _, err := e.svc.Confirm(e.ctx, d); resp.Status(err) != http.StatusConflict {
		t.Fatalf("confirm on a synced vendor: %v", err)
	}
	if _, err := e.svc.Confirm(e.ctx, 9999); resp.Status(err) != http.StatusNotFound {
		t.Fatalf("confirm unknown: %v", err)
	}
}

func TestNoChannelAndOnlyLiveChannelsCount(t *testing.T) {
	e := newEnv(t)
	g := e.vendor("G", "gg")
	e.setOpt(KeyGroupRatio, `{"default":1}`)
	e.update(g, UpdateInput{Mode: sp(store.PricingFollow), Markup: fp(1.5)})
	if it := e.item("gg"); it.State != store.PricingNoChannel || it.Reference != nil || it.TargetPrice != nil {
		t.Fatalf("no channel at all: %+v", it)
	}
	// a disabled channel does not count
	e.channel("gg", 1, 1, 0.1, ports.ChannelManualDisabled)
	if it := e.item("gg"); it.State != store.PricingNoChannel {
		t.Fatalf("disabled: %+v", it)
	}
	// a draft that moves a channel into the group does not count (NewAPI is what is in effect)
	other := e.channel("elsewhere", 1, 1, 0.1, ports.ChannelEnabled)
	draft := store.ChannelDraft{ChannelID: other, Op: store.DraftUpdate, Diff: `{"fields":{"group":"gg"},"ext":{}}`}
	if err := e.st.DB.Create(&draft).Error; err != nil {
		t.Fatal(err)
	}
	if it := e.item("gg"); it.State != store.PricingNoChannel {
		t.Fatalf("draft: %+v", it)
	}
	pv, err := e.svc.Preview(e.ctx, ApplyInput{})
	if err != nil || len(pv.Vendors) != 0 || len(pv.Skipped) != 1 || pv.Skipped[0].State != store.PricingNoChannel {
		t.Fatalf("preview: %+v %v", pv, err)
	}
	// an unpriced channel (no upstream) does not count either
	_, _ = e.fake.Create(e.ctx, ports.ChannelCreate{Type: 58, Name: "plain", Group: "gg", Status: ports.ChannelEnabled, Models: []string{"m"}})
	if it := e.item("gg"); it.State != store.PricingNoChannel {
		t.Fatalf("unpriced: %+v", it)
	}
	// a real one makes the vendor computable; the group list may hold several groups
	e.channel("x, gg ,y", 3, 1, 0.1, ports.ChannelEnabled)
	e.channel("gg", 1, 1, 0.9, ports.ChannelEnabled) // lower priority: ignored
	it := e.item("gg")
	if it.State != store.PricingToWrite || it.Reference == nil || it.Reference.Priority != 3 || len(it.Reference.Channels) != 1 || !eq(it.TargetPrice, 0.15) {
		t.Fatalf("with a channel: %+v", it)
	}
	if it.Change != nil {
		t.Fatalf("NewAPI has no group yet, so there is no change: %+v", it)
	}
}

func TestUpdateGroupRatioDirectly(t *testing.T) {
	e := newEnv(t)
	id := e.vendor("A", "ga")

	// fixed mode (the default of a new vendor): the target ratio is exactly the rounded input
	it := e.update(id, UpdateInput{GroupRatio: fp(0.30949)})
	if it.Mode != store.PricingFixed || it.State != store.PricingToWrite || it.TargetRatio == nil || *it.TargetRatio != 0.309 ||
		!eq(it.DraftRatio, 0.309) || !eq(it.SellPrice, 0.309*7/7.2) {
		t.Fatalf("fixed edit: %+v", it)
	}
	if n := e.auditCount(fmt.Sprintf("nam:vendor_pricing:%d", id), "update", ""); n != 1 {
		t.Fatalf("update audit rows: %d", n)
	}
	writes := e.fake.Writes
	e.update(id, UpdateInput{Mode: sp(store.PricingFixed), GroupRatio: fp(0.31)})
	if it := e.item("ga"); it.TargetRatio == nil || *it.TargetRatio != 0.31 {
		t.Fatalf("explicit fixed mode: %+v", it)
	}
	if e.fake.Writes != writes {
		t.Fatal("editing the ratio writes nam only")
	}
	e.apply(ApplyInput{IncludeGroups: new(bool)})
	if got := e.groupRatio()["ga"]; got != 0.31 {
		t.Fatalf("written ratio: %v", got)
	}
	if it := e.item("ga"); it.State != store.PricingSynced || it.DraftRatio != nil {
		t.Fatalf("after apply: %+v", it)
	}
	for _, ratio := range []float64{0.001, 0.286, 2.16, 7.777, 10} {
		if it := e.update(id, UpdateInput{GroupRatio: fp(ratio)}); it.TargetRatio == nil || *it.TargetRatio != ratio {
			t.Fatalf("ratio %v: %+v", ratio, it)
		}
	}

	// follow mode: the ratio is decided by the reference cost
	e.channel("ga", 1, 1, 0.1, ports.ChannelEnabled)
	e.update(id, UpdateInput{Mode: sp(store.PricingFollow), Markup: fp(1)})
	for name, in := range map[string]UpdateInput{
		"follow mode ratio":   {GroupRatio: fp(0.5)},
		"follow mode price":   {SellPrice: fp(0.5)},
		"switching to follow": {Mode: sp(store.PricingFollow), GroupRatio: fp(0.5)},
	} {
		_, err := e.svc.Update(e.ctx, id, in)
		if resp.Status(err) != http.StatusBadRequest || !strings.Contains(err.Error(), "固定模式") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if r := e.row(id); r.Mode != store.PricingFollow || r.DraftRatio != nil {
		t.Fatalf("a rejected edit must not change the row: %+v", r)
	}
	// ... but the same request may switch to fixed and set the ratio
	if it := e.update(id, UpdateInput{Mode: sp(store.PricingFixed), GroupRatio: fp(0.5)}); it.Mode != store.PricingFixed || *it.TargetRatio != 0.5 {
		t.Fatalf("follow → fixed with a ratio: %+v", it)
	}

	before := e.row(id)
	for name, in := range map[string]UpdateInput{
		"both fields":    {GroupRatio: fp(0.5), SellPrice: fp(0.3)},
		"zero":           {GroupRatio: fp(0)},
		"negative":       {GroupRatio: fp(-0.5)},
		"NaN":            {GroupRatio: fp(nan())},
		"rounds to zero": {GroupRatio: fp(0.0004)},
		"price > 10":     {GroupRatio: fp(11)},
		"tiny sell":      {SellPrice: fp(0.0001)},
	} {
		if _, err := e.svc.Update(e.ctx, id, in); resp.Status(err) != http.StatusBadRequest {
			t.Errorf("%s: %v", name, err)
		}
	}
	if after := e.row(id); after.DraftRatio == nil || *after.DraftRatio != *before.DraftRatio || after.Mode != before.Mode {
		t.Fatalf("rejected edits must not change the row: %+v", after)
	}
}

func TestSyncedNewAPIRatioIsNotRewritten(t *testing.T) {
	e := newEnv(t)
	id := e.vendor("A", "ga")
	e.setOpt(KeyGroupRatio, `{"ga":0.2857}`)
	e.setOpt(KeyUserUsableGroups, `{"ga":"A","auto":"自动选择"}`)
	e.setOpt(KeyAutoGroups, `["ga"]`)
	writes := e.fake.Writes
	if it := e.item("ga"); it.State != store.PricingSynced || !eq(it.NewAPIRatio, 0.2857) {
		t.Fatalf("synced: %+v", it)
	}
	res := e.apply(ApplyInput{})
	if len(res.Written) != 0 || len(res.Vendors) != 0 || e.fake.Writes != writes || e.opt(KeyGroupRatio) != `{"ga":0.2857}` {
		t.Fatalf("an already synced vendor must not be rewritten: %+v", res)
	}
	// a real change is written with 3 decimals
	e.update(id, UpdateInput{SellPrice: fp(0.3)})
	e.apply(ApplyInput{IncludeGroups: new(bool)})
	if got := e.groupRatio()["ga"]; got != 0.309 {
		t.Fatalf("ratio: %v", got)
	}
}

func TestFollowPriceRoundingToZeroRatioIsNotWritten(t *testing.T) {
	e := newEnv(t)
	id := e.vendor("A", "ga")
	e.setOpt(KeyGroupRatio, `{"ga":0.1}`)
	e.channel("ga", 1, 1, 0.0004, ports.ChannelEnabled) // 上游渠道售价 0.0004 → ratio 0.0004 × 7.2 ÷ 7 rounds to 0
	e.update(id, UpdateInput{Mode: sp(store.PricingFollow), Markup: fp(1)})
	it := e.item("ga")
	if it.State != store.PricingUnset || it.TargetRatio != nil {
		t.Fatalf("item: %+v", it)
	}
	writes := e.fake.Writes
	pv, err := e.svc.Preview(e.ctx, ApplyInput{VendorIDs: []uint{id}})
	if err != nil || len(pv.Vendors) != 0 || len(pv.Skipped) != 1 || !strings.Contains(pv.Skipped[0].Reason, "舍入") {
		t.Fatalf("preview: %+v %v", pv, err)
	}
	if e.fake.Writes != writes || e.groupRatio()["ga"] != 0.1 {
		t.Fatal("a ratio of 0 must never be written")
	}
}

func TestUpdateValidation(t *testing.T) {
	e := newEnv(t)
	id := e.vendor("A", "ga")
	bad := map[string]UpdateInput{
		"price 0":      {SellPrice: fp(0)},
		"price neg":    {SellPrice: fp(-1)},
		"price > 10":   {SellPrice: fp(10.01)},
		"price NaN":    {SellPrice: fp(nan())},
		"markup 0":     {Markup: fp(0)},
		"markup > 100": {Markup: fp(100.5)},
		"mode":         {Mode: sp("auto")},
	}
	for name, in := range bad {
		if _, err := e.svc.Update(e.ctx, id, in); resp.Status(err) != http.StatusBadRequest {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := e.svc.Update(e.ctx, 999, UpdateInput{SellPrice: fp(1)}); resp.Status(err) != http.StatusNotFound {
		t.Errorf("unknown vendor: %v", err)
	}
	it := e.update(id, UpdateInput{SellPrice: fp(10), Markup: fp(100)})
	if !eqTol(it.SellPrice, 10, 1e-3) || !eq(it.Markup, 100) || it.Mode != store.PricingFixed {
		t.Fatalf("boundaries are allowed: %+v", it)
	}
	if n := e.auditCount(fmt.Sprintf("nam:vendor_pricing:%d", id), "update", ""); n != 1 {
		t.Fatalf("update audit rows: %d", n)
	}
}

func TestMarginFromStats(t *testing.T) {
	e := newEnv(t)
	e.vendor("A", "ga")
	e.vendor("B", "gb")
	hour := func(h int64) int64 { return h - h%3600 }
	now := nowUnix()
	rows := []store.StatsHourly{
		{ChannelID: 1, Model: "m", Hour: hour(now - 3600), Vendor: "A", RevenueCNY: 100, CostCNY: 60},
		{ChannelID: 2, Model: "m", Hour: hour(now - 2*86400), Vendor: "A", RevenueCNY: 100, CostCNY: 80},
		{ChannelID: 1, Model: "m", Hour: hour(now - 9*86400), Vendor: "A", RevenueCNY: 999, CostCNY: 1}, // outside 7 days
	}
	if err := e.st.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	a, b := e.item("ga"), e.item("gb")
	if !near(a.Revenue7d, 200) || !near(a.Cost7d, 140) || !eq(a.Margin7d, 0.3) {
		t.Fatalf("A: %+v", a)
	}
	if b.Revenue7d != 0 || b.Margin7d != nil {
		t.Fatalf("B has no revenue: %+v", b)
	}
}

func TestConcurrentApplyWritesOnce(t *testing.T) {
	e := newEnv(t)
	id := e.vendor("A", "ga")
	e.update(id, UpdateInput{SellPrice: fp(0.3)})
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.svc.Apply(e.ctx, ApplyInput{}, store.TriggerManual); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if n := len(e.histories()); n != 1 {
		t.Fatalf("history rows: %d", n)
	}
	if n := e.auditCount("option:GroupRatio", "set", store.ResultOK); n != 1 {
		t.Fatalf("GroupRatio written %d times", n)
	}
}

func TestInvalidSiteRateBlocksWrites(t *testing.T) {
	e := newEnv(t)
	id := e.vendor("A", "ga")
	e.update(id, UpdateInput{SellPrice: fp(0.3)})
	if err := e.st.SetKV(e.ctx, store.KeySiteRate, "0"); err != nil {
		t.Fatal(err)
	}
	writes := e.fake.Writes
	if ov := e.overview(); ov.Error == "" {
		t.Fatal("site rate 0 must be reported")
	}
	if _, err := e.svc.Apply(e.ctx, ApplyInput{}, store.TriggerManual); err == nil || e.fake.Writes != writes {
		t.Fatalf("apply must not write: %v", err)
	}
}
