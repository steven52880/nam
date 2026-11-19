package channels

import (
	"fmt"
	"math"
	"testing"

	"github.com/steven52880/nam/internal/store"
)

func near(a, b float64) bool { return math.Abs(a-b) <= 1e-6*math.Max(1, math.Abs(b)) }

// Plan QA M3 steps 2-4: derived discount 0.4, manual 0.35 kept, balance 5 USD → 2.5 CNY,
// 单次成本 = 0.00996 × 0.35 × 0.5 = 0.001743.
func TestBatchDerivesDiscountAndRowsShowCost(t *testing.T) {
	e := newEnv(t)
	f := func(v float64) *float64 { return &v }
	must(t, e.svc.St.DB.Create(&store.OfficialPrice{Model: "test-model", Input: f(3), Output: f(15), CacheRead: f(0.3), CacheWrite5m: f(3.75), Source: "litellm"}).Error)
	must(t, e.svc.St.DB.Create(&store.UpstreamModelPrice{UpstreamID: e.up.ID, Group: "a", Model: "test-model", Input: f(1.2)}).Error)
	must(t, e.svc.St.DB.Model(&store.Upstream{}).Where("id = ?", e.up.ID).Update("balance_usd", 5.0).Error)

	prev, err := e.svc.BatchPreview(e.ctx, BatchRequest{UpstreamID: e.up.ID, TokenIDs: []uint{e.tok.ID},
		TokenModels: map[string][]string{fmt.Sprint(e.tok.ID): {"test-model"}}, Weight: 1})
	must(t, err)
	items := prev.Items
	if len(items) != 1 || items[0].Discount == nil || !near(*items[0].Discount, 0.4) || items[0].DiscountSource != "auto" || items[0].Ext.Discount != nil {
		t.Fatalf("derived discount (preview shows it, nothing is stored): %+v", items[0])
	}
	if *items[0].Fields.TestModel != "test-model" {
		t.Fatalf("test model = %s", *items[0].Fields.TestModel)
	}

	e.batch(t, "test-model")
	_, issues, err := e.svc.Apply(e.ctx, nil)
	must(t, err)
	if len(issues) > 0 {
		t.Fatalf("issues: %+v", issues)
	}
	name := ChannelName("mock-a", "a", "anthropic")
	var row Row
	for _, r := range mustRows(t, e) {
		if r.Name == name {
			row = r
		}
	}
	if row.UnitCost == nil || !near(*row.UnitCost, 0.00996*0.4*0.5) || row.UnitCostModel != "test-model" {
		t.Fatalf("unit cost (auto): %+v", row)
	}
	if row.BalanceUSD == nil || *row.BalanceUSD != 5 || !near(*row.BalanceCNY, 2.5) {
		t.Fatalf("balance: %v %v", row.BalanceUSD, row.BalanceCNY)
	}

	must(t, e.svc.St.DB.Model(&store.ChannelExt{}).Where("channel_id = ?", row.ID).Update("manual_discount", 0.35).Error)
	row = mustRow(t, e, row.ID)
	if row.UnitCost == nil || !near(*row.UnitCost, 0.001743) {
		t.Fatalf("unit cost (manual): %v", row.UnitCost)
	}

	// model detail: per-model override + cost per 1M + deviation flag
	must(t, e.svc.St.DB.Create(&store.OfficialPrice{Model: "test-model-2", Input: f(1), Output: f(4)}).Error)
	must(t, e.svc.St.DB.Create(&store.UpstreamModelPrice{UpstreamID: e.up.ID, Group: "a", Model: "test-model-2", Input: f(0.6)}).Error)
	models := []string{"test-model", "test-model-2", "unpriced"}
	_, err = e.svc.SaveChanges(e.ctx, []Change{{ChannelID: row.ID, Fields: Fields{Models: &models}}})
	must(t, err)
	must(t, e.svc.St.DB.Model(&store.ChannelExt{}).Where("channel_id = ?", row.ID).Update("model_discount", `{"test-model-2":0.2}`).Error)
	d, err := e.svc.Models(e.ctx, row.ID)
	must(t, err)
	if len(d.Models) != 3 || d.DerivedMedian == nil || !near(*d.DerivedMedian, 0.5) {
		t.Fatalf("detail median: %+v", d.DerivedMedian)
	}
	m1, m2, m3 := d.Models[0], d.Models[1], d.Models[2]
	if !m1.Deviates || !m2.Deviates || m3.Reason != "缺原厂价" || m3.Price != nil {
		t.Fatalf("deviation flags: %+v %+v %+v", m1, m2, m3)
	}
	if m2.Override == nil || m2.Effective == nil || *m2.Effective != 0.2 || m2.EffectiveSource != "override" || !near(*m2.CostInPerM, 1*0.2*0.5) || !near(*m2.CostOutPerM, 4*0.2*0.5) {
		t.Fatalf("override cost: %+v", m2)
	}
	if m1.Effective == nil || *m1.Effective != 0.35 || m1.EffectiveSource != "manual" || !near(*m1.CostInPerM, 3*0.35*0.5) {
		t.Fatalf("channel discount cost: %+v", m1)
	}
}
