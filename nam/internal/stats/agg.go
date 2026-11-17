package stats

import (
	"encoding/json"
	"math"

	"github.com/steven52880/nam/internal/store"
)

// Priced is the money side of one successful sample (see Pricer.Price).
type Priced struct {
	OK              bool    // cost could be computed (official price + managed channel with upstream)
	Cost            float64 // CNY
	Official        float64 // USD at official cache prices
	OfficialNoCache float64 // USD as if nothing was cached
	Revenue         float64 // CNY, always set for successes
}

// Agg mirrors the statistic columns of store.StatsHourly with histograms decoded.
type Agg struct {
	Success     int64
	Fail        int64
	FailByClass map[string]int64

	Input  int64 // non-cached input
	Read   int64
	Write  int64 // 5m + 1h
	Output int64

	FRT      Hist
	Speed    Hist // exact
	SpeedEst Hist // estimated

	CostCNY            float64
	OfficialUSD        float64
	OfficialNoCacheUSD float64
	PricedRequests     int64
	Quota              int64
	RevenueCNY         float64
}

// NewAgg returns an empty aggregate with initialised maps.
func NewAgg() Agg {
	return Agg{FailByClass: map[string]int64{}, FRT: Hist{}, Speed: Hist{}, SpeedEst: Hist{}}
}

// Add accumulates one sample. p is ignored for failures.
func (a *Agg) Add(s Sample, p Priced) {
	a.ensure()
	if !s.OK {
		a.Fail++
		a.FailByClass[s.FailClass]++
		return
	}
	a.Success++
	a.Input += s.Input
	a.Read += s.Read
	a.Write += s.W5 + s.W1
	a.Output += s.Output
	a.FRT.Add(s.FRT)
	if s.SpeedEst {
		a.SpeedEst.Add(s.Speed)
	} else {
		a.Speed.Add(s.Speed)
	}
	a.Quota += s.Quota
	a.RevenueCNY += p.Revenue
	if p.OK {
		a.CostCNY += p.Cost
		a.OfficialUSD += p.Official
		a.OfficialNoCacheUSD += p.OfficialNoCache
		a.PricedRequests++
	}
}

// Merge adds o into a.
func (a *Agg) Merge(o Agg) {
	a.ensure()
	a.Success += o.Success
	a.Fail += o.Fail
	for k, n := range o.FailByClass {
		a.FailByClass[k] += n
	}
	a.Input += o.Input
	a.Read += o.Read
	a.Write += o.Write
	a.Output += o.Output
	a.FRT.Merge(o.FRT)
	a.Speed.Merge(o.Speed)
	a.SpeedEst.Merge(o.SpeedEst)
	a.CostCNY += o.CostCNY
	a.OfficialUSD += o.OfficialUSD
	a.OfficialNoCacheUSD += o.OfficialNoCacheUSD
	a.PricedRequests += o.PricedRequests
	a.Quota += o.Quota
	a.RevenueCNY += o.RevenueCNY
}

func (a *Agg) ensure() {
	if a.FailByClass == nil {
		a.FailByClass = map[string]int64{}
	}
	if a.FRT == nil {
		a.FRT = Hist{}
	}
	if a.Speed == nil {
		a.Speed = Hist{}
	}
	if a.SpeedEst == nil {
		a.SpeedEst = Hist{}
	}
}

// FromRow decodes a stored row.
func FromRow(r store.StatsHourly) Agg {
	a := NewAgg()
	if r.FailByClass != "" {
		// A corrupt class map only loses the breakdown; totals stay in Fail.
		_ = json.Unmarshal([]byte(r.FailByClass), &a.FailByClass)
	}
	a.Success, a.Fail = r.Success, r.Fail
	a.Input, a.Read, a.Write, a.Output = r.PromptTokens, r.CacheReadTokens, r.CacheWriteTokens, r.CompletionTokens
	a.FRT, a.Speed, a.SpeedEst = ParseHist(r.FRTHist), ParseHist(r.SpeedHist), ParseHist(r.SpeedEstHist)
	a.CostCNY, a.OfficialUSD, a.OfficialNoCacheUSD = r.CostCNY, r.OfficialUSD, r.OfficialNoCacheUSD
	a.PricedRequests, a.Quota, a.RevenueCNY = r.PricedRequests, r.Quota, r.RevenueCNY
	return a
}

// ApplyTo writes the statistic columns into r (keys and group/vendor are untouched).
func (a Agg) ApplyTo(r *store.StatsHourly) {
	r.Success, r.Fail = a.Success, a.Fail
	r.FailByClass = ""
	if len(a.FailByClass) > 0 {
		b, err := json.Marshal(a.FailByClass)
		if err == nil {
			r.FailByClass = string(b)
		}
	}
	r.PromptTokens, r.CacheReadTokens, r.CacheWriteTokens, r.CompletionTokens = a.Input, a.Read, a.Write, a.Output
	r.FRTHist, r.SpeedHist, r.SpeedEstHist = a.FRT.Encode(), a.Speed.Encode(), a.SpeedEst.Encode()
	r.CostCNY, r.OfficialUSD, r.OfficialNoCacheUSD = a.CostCNY, a.OfficialUSD, a.OfficialNoCacheUSD
	r.PricedRequests, r.Quota, r.RevenueCNY = a.PricedRequests, a.Quota, a.RevenueCNY
}

// Wilson is the Wilson score lower bound (z = 1.96) of s successes in n trials.
func Wilson(s, n int64) float64 {
	if n <= 0 {
		return 0
	}
	const z = 1.96
	fn := float64(n)
	p := float64(s) / fn
	z2 := z * z
	num := p + z2/(2*fn) - z*math.Sqrt(p*(1-p)/fn+z2/(4*fn*fn))
	return num / (1 + z2/fn)
}
