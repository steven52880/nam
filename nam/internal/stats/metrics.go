package stats

// Metrics is the JSON shape shared by every stats endpoint (ratios are 0–1, nullable
// fields are null when undefined).
type Metrics struct {
	Requests    int64            `json:"requests"`
	Success     int64            `json:"success"`
	Fail        int64            `json:"fail"`
	SuccessRate *float64         `json:"success_rate"`
	Wilson      *float64         `json:"wilson"`
	FailByClass map[string]int64 `json:"fail_by_class"`

	FRTP50         *float64 `json:"frt_p50"`
	FRTP95         *float64 `json:"frt_p95"`
	SpeedP50       *float64 `json:"speed_p50"`
	SpeedP95       *float64 `json:"speed_p95"`
	SpeedEstimated bool     `json:"speed_estimated"`

	CacheHit         *float64 `json:"cache_hit"`
	PromptTokens     int64    `json:"prompt_tokens"`
	CompletionTokens int64    `json:"completion_tokens"`
	CacheReadTokens  int64    `json:"cache_read_tokens"`
	CacheWriteTokens int64    `json:"cache_write_tokens"`

	CostCNY            float64  `json:"cost_cny"`
	RevenueCNY         float64  `json:"revenue_cny"`
	PricedRequests     int64    `json:"priced_requests"`
	EffectiveCostPrice *float64 `json:"effective_cost_price"`
	MeasuredCostPrice  *float64 `json:"measured_cost_price"`
	Margin             *float64 `json:"margin"`
}

// Metrics derives the display metrics. speed_estimated is true only when the speed
// percentiles come from the estimated histogram (no exact sample, at least one estimate).
func (a Agg) Metrics() Metrics {
	m := Metrics{
		Requests: a.Success + a.Fail, Success: a.Success, Fail: a.Fail,
		FailByClass:  map[string]int64{},
		PromptTokens: a.Input, CompletionTokens: a.Output, CacheReadTokens: a.Read, CacheWriteTokens: a.Write,
		CostCNY: a.CostCNY, RevenueCNY: a.RevenueCNY, PricedRequests: a.PricedRequests,
	}
	for k, n := range a.FailByClass {
		if n != 0 {
			m.FailByClass[k] = n
		}
	}
	if m.Requests > 0 {
		m.SuccessRate = ptr(float64(a.Success) / float64(m.Requests))
		m.Wilson = ptr(Wilson(a.Success, m.Requests))
	}
	m.FRTP50, m.FRTP95 = quantiles(a.FRT)
	speed := a.Speed
	if speed.Count() == 0 && a.SpeedEst.Count() > 0 {
		speed, m.SpeedEstimated = a.SpeedEst, true
	}
	m.SpeedP50, m.SpeedP95 = quantiles(speed)
	m.CacheHit = ratio(float64(a.Read), float64(a.Input+a.Read+a.Write))
	m.EffectiveCostPrice = ratio(a.CostCNY, a.OfficialNoCacheUSD)
	m.MeasuredCostPrice = ratio(a.CostCNY, a.OfficialUSD)
	m.Margin = ratio(a.RevenueCNY-a.CostCNY, a.RevenueCNY)
	return m
}

func quantiles(h Hist) (p50, p95 *float64) {
	if v, ok := h.Quantile(0.5); ok {
		p50 = ptr(v)
	}
	if v, ok := h.Quantile(0.95); ok {
		p95 = ptr(v)
	}
	return p50, p95
}

func ratio(num, den float64) *float64 {
	if den == 0 {
		return nil
	}
	return ptr(num / den)
}

func ptr(v float64) *float64 { return &v }
