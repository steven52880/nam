package stats

import (
	"strconv"
	"testing"
	"time"

	"github.com/steven52880/nam/internal/store"
)

// seedRows writes one stats row per (channel, model, vendor) in the current hour - 1h.
func seedRows(t *testing.T, e *env) int64 {
	t.Helper()
	h := hourStart(time.Now().Unix()) - 3600
	for _, r := range []store.StatsHourly{
		{ChannelID: 1, Model: "a", Vendor: "v1", Success: 1},
		{ChannelID: 1, Model: "b", Vendor: "v1", Success: 2},
		{ChannelID: 2, Model: "a", Vendor: "v2", Success: 4},
		{ChannelID: 3, Model: "a", Vendor: "", Success: 8},
	} {
		r.Hour = h
		must(t, e.st.DB.Create(&r).Error)
	}
	// channel 2 has an ext row without upstream; channel 3 has none; channel 1 is managed.
	must(t, e.st.DB.Create(&store.ChannelExt{ChannelID: 2}).Error)
	return h
}

func TestSeries_filters_each_dimension(t *testing.T) {
	e := newEnv(t)
	h := seedRows(t, e)
	var up store.ChannelExt
	must(t, e.st.DB.First(&up, "channel_id = 1").Error)
	upKey := strconv.FormatUint(uint64(*up.UpstreamID), 10)
	cases := []struct {
		dim, key string
		want     int64
	}{
		{"", "", 15}, {DimChannel, "1", 3}, {DimChannel, "x", 0}, {DimModel, "a", 13},
		{DimVendor, "v2", 4}, {DimVendor, "", 8}, {DimUpstream, upKey, 3}, {DimUpstream, "", 12},
		{DimUpstream, "999", 0},
	}
	for _, c := range cases {
		res, err := e.svc.Series(e.ctx, h-3600, h+3600, c.dim, c.key, BucketHour)
		must(t, err)
		var got int64
		for _, p := range res.Points {
			got += p.Success
		}
		if got != c.want {
			t.Errorf("dim=%q key=%q: success %d want %d", c.dim, c.key, got, c.want)
		}
	}
}

func TestOverview_and_channel_models_stream_rows(t *testing.T) {
	e := newEnv(t)
	h := seedRows(t, e)
	ov, err := e.svc.Overview(e.ctx, h-3600, h+3600, DimModel)
	must(t, err)
	if ov.Totals.Success != 15 || len(ov.Rows) != 2 || ov.Rows[0].Key != "a" || ov.Rows[0].Success != 13 {
		t.Fatalf("overview %+v", ov.Rows)
	}
	ms, err := e.svc.ChannelModels(e.ctx, 1, Window3d)
	must(t, err)
	if len(ms.Items) != 2 || ms.Items[0].Model != "b" || ms.Items[0].Success != 2 {
		t.Fatalf("models %+v", ms.Items)
	}
	if ms, err = e.svc.ChannelModels(e.ctx, 2, Window7d); err != nil || len(ms.Items) != 1 {
		t.Fatalf("channel 2 models %+v %v", ms.Items, err)
	}
}
