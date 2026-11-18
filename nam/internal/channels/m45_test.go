package channels

import (
	"testing"

	"github.com/steven52880/nam/internal/pricing"
	"github.com/steven52880/nam/internal/store"
)

func seedPrices(t *testing.T, e *env) {
	t.Helper()
	f := func(v float64) *float64 { return &v }
	must(t, e.svc.St.DB.Create(&store.OfficialPrice{Model: "test-model", Input: f(3), Output: f(15), CacheRead: f(0.3), CacheWrite5m: f(3.75), Source: "litellm"}).Error)
	must(t, e.svc.St.DB.Create(&store.OfficialPrice{Model: "test-model-2", Input: f(1), Output: f(4)}).Error)
}

func costByID(t *testing.T, items []ModelCost, id int) ModelCost {
	t.Helper()
	for _, it := range items {
		if it.ChannelID == id {
			return it
		}
	}
	t.Fatalf("channel %d not in costs: %+v", id, items)
	return ModelCost{}
}

// A draft-created channel (negative id) is part of the row set and gets costs too.
func TestModelCostsDraftCreate(t *testing.T) {
	e := newEnv(t)
	seedPrices(t, e)
	ids := e.batch(t, "test-model")
	if len(ids) != 1 || ids[0] >= 0 {
		t.Fatalf("temp ids: %v", ids)
	}
	models := []string{"test-model", "unpriced"}
	_, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: ids[0], Fields: Fields{Models: &models}}})
	must(t, err)
	got, err := e.svc.ModelCosts(e.ctx, "test-model")
	must(t, err)
	if len(got) != 1 {
		t.Fatalf("items: %+v", got)
	}
	c := got[0]
	if c.ChannelID != ids[0] || !c.HasModel || c.Discount == nil || *c.Discount != 1 ||
		c.ChannelCost == nil || !near(*c.ChannelCost, 0.5) || c.UnitCost == nil || !near(*c.UnitCost, 0.00996*0.5) {
		t.Fatalf("draft cost: %+v", c)
	}
	// a model the channel carries but the price book lacks: discount and channel cost, no unit cost
	got, err = e.svc.ModelCosts(e.ctx, "unpriced")
	must(t, err)
	c = got[0]
	if !c.HasModel || c.Discount == nil || c.ChannelCost == nil || c.UnitCost != nil {
		t.Fatalf("unpriced: %+v", c)
	}
}

func TestModelCostsOverridesAndMissingModel(t *testing.T) {
	e := newEnv(t)
	seedPrices(t, e)
	e.batch(t, "test-model")
	_, issues, err := e.svc.Apply(e.ctx, nil)
	must(t, err)
	if len(issues) > 0 {
		t.Fatalf("issues: %+v", issues)
	}
	id := mustRows(t, e)[0].ID
	models := []string{"test-model", "test-model-2"}
	_, err = e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Fields: Fields{Models: &models}}})
	must(t, err)
	must(t, e.svc.St.DB.Model(&store.ChannelExt{}).Where("channel_id = ?", id).
		Updates(map[string]any{"discount": 0.35, "discount_source": store.DiscountManual, "model_discount": `{"test-model-2":0.2}`}).Error)

	// channel-level discount only
	got, err := e.svc.ModelCosts(e.ctx, "test-model")
	must(t, err)
	c := costByID(t, got, id)
	if !c.HasModel || *c.Discount != 0.35 || !near(*c.ChannelCost, 0.175) || !near(*c.UnitCost, 0.00996*0.35*0.5) {
		t.Fatalf("test-model: %+v", c)
	}
	// per-model discount override wins over the channel discount
	book, err := pricing.LoadBook(e.ctx, e.svc.St)
	must(t, err)
	p2, ok := book.Resolve("test-model-2")
	if !ok {
		t.Fatal("test-model-2 not priced")
	}
	got, err = e.svc.ModelCosts(e.ctx, "test-model-2")
	must(t, err)
	c = costByID(t, got, id)
	want := pricing.OfficialUSD(p2, pricing.StandardRequest, pricing.CacheRatio{}) * 0.2 * 0.5
	if !c.HasModel || *c.Discount != 0.2 || !near(*c.ChannelCost, 0.1) || !near(*c.UnitCost, want) {
		t.Fatalf("test-model-2: %+v want unit %v", c, want)
	}
	// a channel without the model: HasModel=false and no numbers
	got, err = e.svc.ModelCosts(e.ctx, "not-served")
	must(t, err)
	c = costByID(t, got, id)
	if c.HasModel || c.Discount != nil || c.ChannelCost != nil || c.UnitCost != nil {
		t.Fatalf("missing model: %+v", c)
	}
}

// priceRows now goes through costInputs/unitCostCNY: List must still report the same
// per-request cost as before (ModelCosts on the test model agrees with it).
func TestPriceRowsUnchangedAfterSplit(t *testing.T) {
	e := newEnv(t)
	seedPrices(t, e)
	e.batch(t, "test-model")
	_, _, err := e.svc.Apply(e.ctx, nil)
	must(t, err)
	row := mustRows(t, e)[0]
	if row.UnitCost == nil || row.UnitCostModel != "test-model" || !near(*row.UnitCost, 0.00996*0.5) {
		t.Fatalf("row unit cost: %+v", row)
	}
	got, err := e.svc.ModelCosts(e.ctx, "test-model")
	must(t, err)
	if c := costByID(t, got, row.ID); c.UnitCost == nil || *c.UnitCost != *row.UnitCost {
		t.Fatalf("ModelCosts %+v vs row %v", c, *row.UnitCost)
	}
	// no price for the model → priceRows leaves the cost nil (unchanged behaviour)
	unpriced := newEnv(t)
	unpriced.batch(t, "test-model") // no price book rows seeded
	if r := mustRows(t, unpriced)[0]; r.UnitCost != nil || r.UnitCostModel != "" {
		t.Fatalf("unpriced row: %+v", r)
	}
}
