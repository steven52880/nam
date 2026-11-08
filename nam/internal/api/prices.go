package api

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/pricing"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/scheduler"
	"github.com/steven52880/nam/internal/store"
)

func (s *Server) registerPriceRoutes(p *gin.RouterGroup) {
	p.GET("/prices", s.listPrices)
	p.POST("/prices/sync", s.syncPrices)
	p.GET("/prices/:model", s.priceDetail)
	p.PUT("/prices/:model/override", s.putOverride)
	p.DELETE("/prices/:model/override", s.deleteOverride)
	p.GET("/aliases", s.listAliases)
	p.PUT("/aliases/:official", s.putAliases)
}

// PriceRow is one model of the 模型与价格 table.
type PriceRow struct {
	Model        string         `json:"model"`
	Vendor       string         `json:"vendor"`
	Aliases      []string       `json:"aliases"`
	Official     *pricing.Price `json:"official"` // nil = 缺价
	Source       string         `json:"source"`   // 价格库来源 / override / ""
	NewAPI       *newAPIPrice   `json:"newapi"`   // nil = NewAPI 没有配置该模型
	Deviation    *deviation     `json:"deviation"`
	ChannelCount int            `json:"channel_count"`
	Status       string         `json:"status"`
	rank         int
}

// PriceCounts is the per-status tally under the current vendor / search filter.
type PriceCounts struct {
	All        int `json:"all"`
	Normal     int `json:"normal"`
	Deviation  int `json:"deviation"`
	NoPrice    int `json:"no_price"`
	PerRequest int `json:"per_request"`
}

func (c *PriceCounts) add(status string) {
	c.All++
	switch status {
	case statusNormal:
		c.Normal++
	case statusDeviation:
		c.Deviation++
	case statusNoPrice:
		c.NoPrice++
	case statusPerRequest:
		c.PerRequest++
	}
}

func aliasIndex(as []store.ModelAlias) map[string][]string {
	out := map[string][]string{}
	for _, a := range as {
		out[a.Official] = append(out[a.Official], a.Alias)
	}
	for _, v := range out {
		sort.Strings(v)
	}
	return out
}

// matchRank: 0 = prefix hit on the model or an alias, 1 = infix hit, -1 = no hit.
func matchRank(q string, names []string) int {
	best := -1
	for _, n := range names {
		n = strings.ToLower(n)
		switch {
		case strings.HasPrefix(n, q):
			return 0
		case strings.Contains(n, q):
			best = 1
		}
	}
	return best
}

// GET /prices?status=&vendor=&q=
func (s *Server) listPrices(c *gin.Context) {
	ctx := c.Request.Context()
	set, err := pricing.LoadSiteModels(ctx, s.Store, s.Ports.Channels)
	if err != nil {
		resp.FailFrom(c, resp.Upstream(err))
		return
	}
	book, err := pricing.LoadBook(ctx, s.Store)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	var as []store.ModelAlias
	if err := s.Store.DB.WithContext(ctx).Find(&as).Error; err != nil {
		resp.FailFrom(c, err)
		return
	}
	prices, err := loadNewAPIPrices(ctx, s.Ports.Options)
	if err != nil {
		resp.FailFrom(c, resp.Upstream(err))
		return
	}
	byOfficial := aliasIndex(as)
	status, vendor := c.Query("status"), c.Query("vendor")
	q := strings.ToLower(strings.TrimSpace(c.Query("q")))
	counts := PriceCounts{}
	rows := make([]PriceRow, 0, len(set.Models))
	for _, m := range set.Models {
		row := PriceRow{Model: m, Vendor: set.VendorOf[m], ChannelCount: set.ChannelCount[m], Aliases: []string{}}
		row.Aliases = append(row.Aliases, byOfficial[book.Canonical(m)]...)
		if vendor != "" && row.Vendor != vendor {
			continue
		}
		if q != "" {
			if row.rank = matchRank(q, append([]string{m}, row.Aliases...)); row.rank < 0 {
				continue
			}
		}
		if p, ok := book.Resolve(m); ok {
			row.Official, row.Source = &p, p.Source
		}
		row.NewAPI = prices.lookup(m)
		row.Deviation, row.Status = classify(row.Official, row.NewAPI)
		counts.add(row.Status)
		if status == "" || status == "all" || status == row.Status {
			rows = append(rows, row)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if (a.Official == nil) != (b.Official == nil) {
			return a.Official == nil
		}
		return a.Model < b.Model
	})
	resp.OK(c, gin.H{"items": rows, "counts": counts, "note": priceNote})
}

// POST /prices/sync: runs the price_book job and waits for it (like /jobs/price_book?wait=1).
func (s *Server) syncPrices(c *gin.Context) {
	ctx, cancel := contextTimeout(c.Request.Context(), jobWaitTimeout)
	defer cancel()
	trigErr := s.Sched.Trigger(ctx, scheduler.JobPriceBook, true)
	for _, st := range s.Sched.Statuses(ctx) {
		if st.Name != scheduler.JobPriceBook {
			continue
		}
		if trigErr != nil {
			resp.FailData(c, http.StatusInternalServerError, trigErr.Error(), st)
			return
		}
		resp.OK(c, st)
		return
	}
	resp.FailFrom(c, trigErr)
}
