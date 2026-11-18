package stats

import (
	"context"
	"time"

	"github.com/steven52880/nam/internal/store"
)

// VendorTotal is one vendor's revenue and cost over a window (元).
type VendorTotal struct {
	Revenue float64
	Cost    float64
}

// VendorTotals sums revenue_cny and cost_cny per vendor name over the last `days` days
// (same hour window as the stats queries). Rows without a vendor are skipped.
func VendorTotals(ctx context.Context, st *store.Store, days int) (map[string]VendorTotal, error) {
	now := time.Now().Unix()
	from, to := hourStart(now)-int64(days)*86400, hourStart(now)+3600
	var rows []struct {
		Vendor  string
		Revenue float64
		Cost    float64
	}
	err := st.DB.WithContext(ctx).Model(&store.StatsHourly{}).
		Select("vendor, COALESCE(SUM(revenue_cny), 0) AS revenue, COALESCE(SUM(cost_cny), 0) AS cost").
		Where("hour >= ? AND hour < ? AND vendor <> ''", from, to).
		Group("vendor").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]VendorTotal, len(rows))
	for _, r := range rows {
		out[r.Vendor] = VendorTotal{Revenue: r.Revenue, Cost: r.Cost}
	}
	return out, nil
}
