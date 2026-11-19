package stats

import (
	"testing"
	"time"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// A channel whose discount cannot be resolved (no manual value, no cached upstream price or
// group ratio) is not costed; its revenue still counts. Once the upstream group ratio is
// cached the very next Pricer prices it, without any stored discount.
func TestPricer_null_discount_has_revenue_but_no_cost(t *testing.T) {
	e := newEnv(t)
	var up store.Upstream
	must(t, e.st.DB.First(&up).Error)
	tok := store.UpstreamToken{UpstreamID: up.ID, Name: "t", Group: "g"}
	must(t, e.st.DB.Create(&tok).Error)
	id, err := e.fake.Create(e.ctx, ports.ChannelCreate{Name: "auto", Group: "anthropic", Models: []string{"test-model"}, Status: 1})
	must(t, err)
	must(t, e.st.DB.Where("1 = 1").Delete(&store.ChannelExt{}).Error)
	must(t, e.st.DB.Create(&store.ChannelExt{ChannelID: id, UpstreamID: &up.ID, TokenID: &tok.ID}).Error)

	smp, ok := Parse(stdLog(id, time.Now().Unix()-60))
	if !ok {
		t.Fatal("parse")
	}
	pr, err := NewPricer(e.ctx, e.st, e.svc.Backend, e.svc.Channels)
	must(t, err)
	got := pr.Price(smp)
	if got.OK || got.Cost != 0 || got.Revenue == 0 {
		t.Fatalf("no discount: %+v", got)
	}
	if v := stdPrice(pr, mustChannel(t, e, id), nil, ""); v != nil {
		t.Fatalf("no std price without a discount: %v", *v)
	}

	// upstream group ratio arrives (ratio sync): priced immediately, 0.4 × 0.00996 USD × rate 0.5
	must(t, e.st.DB.Create(&store.UpstreamGroup{UpstreamID: up.ID, Name: "g", Ratio: 0.4}).Error)
	pr, err = NewPricer(e.ctx, e.st, e.svc.Backend, e.svc.Channels)
	must(t, err)
	got = pr.Price(smp)
	if !got.OK || !approx(got.Cost, 0.00996*0.4*0.5) {
		t.Fatalf("group ratio: %+v", got)
	}
	// a cheaper per-model upstream price changes the cost on that model only
	in := 0.9 // 0.9 / 3 = 0.3
	must(t, e.st.DB.Create(&store.UpstreamModelPrice{UpstreamID: up.ID, Group: "g", Model: "test-model", Input: &in}).Error)
	pr, err = NewPricer(e.ctx, e.st, e.svc.Backend, e.svc.Channels)
	must(t, err)
	if got = pr.Price(smp); !got.OK || !approx(got.Cost, 0.00996*0.3*0.5) {
		t.Fatalf("model price: %+v", got)
	}
}

func mustChannel(t *testing.T, e *env, id int) ports.Channel {
	t.Helper()
	c, err := e.fake.Get(e.ctx, id)
	must(t, err)
	return c
}
