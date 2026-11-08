package pricing

import "encoding/json"

// Usage is a token breakdown. Input excludes cached tokens (NewAPI prompt_tokens minus
// cache reads, Anthropic semantics normalised by the stats layer).
type Usage struct {
	Input        float64 `json:"input"`
	CacheRead    float64 `json:"cache_read"`
	CacheWrite5m float64 `json:"cache_write_5m"`
	CacheWrite1h float64 `json:"cache_write_1h"`
	Output       float64 `json:"output"`
	Requests     float64 `json:"requests"` // per-request models
}

// StandardRequest is the fixed request used for "单次成本" (plan §11.1):
// prompt 1000 tokens of which 200 cached, completion 500.
var StandardRequest = Usage{Input: 800, CacheRead: 200, Output: 500, Requests: 1}

// CacheRatio is a per-channel cache price override, as a fraction of the input price.
// Nil fields fall back to the official cache price ("缓存价默认按原厂比例").
type CacheRatio struct {
	Read  *float64 `json:"read,omitempty"`
	Write *float64 `json:"write,omitempty"`
}

// ParseCacheRatio decodes channel_ext.cache_override ("" → zero value).
func ParseCacheRatio(raw string) CacheRatio {
	var c CacheRatio
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &c)
	}
	return c
}

func val(p *float64, def float64) float64 {
	if p == nil {
		return def
	}
	return *p
}

// OfficialUSD is what the usage would cost at the official price (USD).
// Missing cache prices fall back to the input price (no cache discount).
func OfficialUSD(p Price, u Usage, cr CacheRatio) float64 {
	if p.IsPerRequest() {
		return *p.PerRequest * u.Requests
	}
	in := val(p.Input, 0)
	read := val(p.CacheRead, in)
	w5 := val(p.CacheWrite5m, in)
	w1 := val(p.CacheWrite1h, w5)
	if cr.Read != nil {
		read = in * *cr.Read
	}
	if cr.Write != nil {
		w5 = in * *cr.Write
		w1 = w5
	}
	out := val(p.Output, in)
	return (u.Input*in + u.CacheRead*read + u.CacheWrite5m*w5 + u.CacheWrite1h*w1 + u.Output*out) / 1e6
}

// ChannelCNY converts an official cost to the channel's real cost:
// 成本(元) = 原厂价(美元) × 上游分组折扣 × 上游货币汇率.
func ChannelCNY(officialUSD, discount, currencyRate float64) float64 {
	return officialUSD * discount * currencyRate
}

// PerMillionCNY returns the channel's CNY price per 1M input / output tokens.
func PerMillionCNY(p Price, discount, currencyRate float64) (in, out *float64) {
	if p.IsPerRequest() || p.Input == nil {
		return nil, nil
	}
	i := *p.Input * discount * currencyRate
	o := val(p.Output, *p.Input) * discount * currencyRate
	return &i, &o
}
