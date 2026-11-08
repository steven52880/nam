package api

import (
	"context"
	"encoding/json"
	"math"

	"github.com/steven52880/nam/internal/pricing"
	"github.com/steven52880/nam/ports"
)

// Row statuses of the 模型与价格 table.
const (
	statusNormal     = "normal"
	statusDeviation  = "deviation"
	statusNoPrice    = "no_price"
	statusPerRequest = "per_request"
)

// priceNote explains how NewAPI's current price is read (M0 ④).
const priceNote = "NewAPI 当前价格取自选项 ModelRatio（×2 = 输入美元/百万 token）、CompletionRatio、CacheRatio、" +
	"CreateCacheRatio、ModelPrice。选项里没有的补全倍率/缓存倍率标为「未知」，不参与偏差比较：" +
	"NewAPI 对部分模型有内置默认值（M0 ④：gpt-4.1-mini 补全倍率偏低；gemini-2.5 与 o3 没有缓存折扣）。" +
	"偏差按字段分别比较（输入、输出、缓存读），超过 5% 标为偏差。"

var priceOptionKeys = []string{"ModelRatio", "CompletionRatio", "CacheRatio", "CreateCacheRatio", "ModelPrice"}

// newAPIPrice is the price NewAPI currently charges for one model (USD per 1M tokens,
// or USD per request). Fields absent from the option maps are nil and listed in Unknown.
type newAPIPrice struct {
	Input      *float64 `json:"input"`
	Output     *float64 `json:"output"`
	CacheRead  *float64 `json:"cache_read"`
	CacheWrite *float64 `json:"cache_write"`
	PerRequest *float64 `json:"per_request"`
	Unknown    []string `json:"unknown"`
}

type newAPIPrices struct {
	modelRatio, completion, cacheRead, cacheWrite, modelPrice map[string]float64
}

// loadNewAPIPrices reads the five price options (read only).
func loadNewAPIPrices(ctx context.Context, opts ports.OptionStore) (newAPIPrices, error) {
	raw, err := opts.Get(ctx, priceOptionKeys...)
	if err != nil {
		return newAPIPrices{}, err
	}
	return newAPIPrices{
		modelRatio: parseRatioMap(raw["ModelRatio"]), completion: parseRatioMap(raw["CompletionRatio"]),
		cacheRead: parseRatioMap(raw["CacheRatio"]), cacheWrite: parseRatioMap(raw["CreateCacheRatio"]),
		modelPrice: parseRatioMap(raw["ModelPrice"]),
	}, nil
}

func parseRatioMap(raw string) map[string]float64 {
	out := map[string]float64{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &out)
	}
	return out
}

// lookup returns NewAPI's configured price for a site model name, nil when none is set.
func (p newAPIPrices) lookup(model string) *newAPIPrice {
	mr, hasRatio := p.modelRatio[model]
	mp, hasPrice := p.modelPrice[model]
	if !hasRatio && !hasPrice {
		return nil
	}
	out := &newAPIPrice{Unknown: []string{}}
	if hasPrice {
		out.PerRequest = &mp
	}
	if !hasRatio {
		return out
	}
	in := mr * 2
	out.Input = &in
	if v, ok := p.completion[model]; ok {
		o := in * v
		out.Output = &o
	} else {
		out.Unknown = append(out.Unknown, "output")
	}
	if v, ok := p.cacheRead[model]; ok {
		r := in * v
		out.CacheRead = &r
	} else {
		out.Unknown = append(out.Unknown, "cache_read")
	}
	if v, ok := p.cacheWrite[model]; ok {
		w := in * v
		out.CacheWrite = &w
	}
	return out
}

// deviation is NewAPI's price relative to the official one, per field (fraction, 0.05 = +5%).
// Fields that cannot be compared are nil.
type deviation struct {
	Input      *float64 `json:"input"`
	Output     *float64 `json:"output"`
	CacheRead  *float64 `json:"cache_read"`
	PerRequest *float64 `json:"per_request"`
}

func relDiff(actual, official *float64) *float64 {
	if actual == nil || official == nil || *official <= 0 {
		return nil
	}
	d := (*actual - *official) / *official
	return &d
}

func exceeds(v *float64) bool { return v != nil && math.Abs(*v) > pricing.DeviationThreshold }

// classify compares NewAPI's price with the official one and picks the row status.
func classify(p *pricing.Price, na *newAPIPrice) (*deviation, string) {
	switch {
	case p == nil:
		return nil, statusNoPrice
	case p.IsPerRequest():
		if na == nil {
			return nil, statusPerRequest
		}
		return &deviation{PerRequest: relDiff(na.PerRequest, p.PerRequest)}, statusPerRequest
	case na == nil:
		return nil, statusNormal
	}
	d := &deviation{Input: relDiff(na.Input, p.Input), Output: relDiff(na.Output, p.Output), CacheRead: relDiff(na.CacheRead, p.CacheRead)}
	if exceeds(d.Input) || exceeds(d.Output) || exceeds(d.CacheRead) {
		return d, statusDeviation
	}
	return d, statusNormal
}
