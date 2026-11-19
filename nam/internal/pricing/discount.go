package pricing

import (
	"math"
	"sort"
)

// DeviationThreshold is the 5% bound used by the NewAPI price compare page.
const DeviationThreshold = 0.05

// UpstreamPrice is one model's price on the upstream for the channel's token group
// (upstream-USD per 1M tokens, group ratio already applied by the adapter).
type UpstreamPrice struct {
	Input      *float64
	PerRequest *float64
}

// ModelDiscount is the derivation result for one model of a channel.
type ModelDiscount struct {
	Model     string   `json:"model"`
	Official  string   `json:"official"`  // price-book name
	Discount  *float64 `json:"discount"`  // upstream / official; nil when not derivable
	Reason    string   `json:"reason"`    // why nil: 缺原厂价 / 上游无价 / 计费方式不同
	Deviation *float64 `json:"deviation"` // relative to the baseline (group ratio, else median)
	Deviates  bool     `json:"deviates"`  // |deviation| > 1%
}

// Derivation is the channel-level result.
type Derivation struct {
	Median *float64        `json:"median"` // 渠道各模型 上游价 ÷ 原厂价 的中位数
	Models []ModelDiscount `json:"models"`
}

// lookupUpstream finds a model's upstream price: the model name first, then its alias.
func lookupUpstream(book *Book, model string, upstream map[string]UpstreamPrice) (UpstreamPrice, bool) {
	if up, ok := upstream[model]; ok {
		return up, true
	}
	if c := book.Canonical(model); c != model {
		up, ok := upstream[c]
		return up, ok
	}
	return UpstreamPrice{}, false
}

// deriveOne is 上游价 ÷ 原厂价 for one model (per-request models divide the per-request
// prices). Discount is nil with a Reason when it cannot be derived.
func deriveOne(book *Book, m string, upstream map[string]UpstreamPrice) ModelDiscount {
	md := ModelDiscount{Model: m, Official: book.Canonical(m)}
	op, ok := book.Resolve(m)
	up, hasUp := lookupUpstream(book, m, upstream)
	switch {
	case !ok:
		md.Reason = "缺原厂价"
	case !hasUp:
		md.Reason = "上游无价"
	case op.IsPerRequest() && up.PerRequest != nil && *up.PerRequest > 0:
		v := *up.PerRequest / *op.PerRequest
		md.Discount = &v
	case !op.IsPerRequest() && up.Input != nil && *up.Input > 0:
		v := *up.Input / *op.Input
		md.Discount = &v
	default:
		md.Reason = "计费方式不同"
	}
	return md
}

// Derive computes 上游分组折扣 = 上游模型价 ÷ 原厂价 for every model and takes the median.
// For NewAPI-style upstreams 上游模型价 = 模型倍率 × 2 × 分组倍率, so this is the plan's
// "上游模型倍率 × 上游分组倍率 ÷ 原厂模型倍率". Deviation here is relative to the median;
// Resolver.Channel re-bases it on the upstream group ratio when there is one.
func Derive(models []string, book *Book, upstream map[string]UpstreamPrice) Derivation {
	d := Derivation{Models: make([]ModelDiscount, 0, len(models))}
	var vals []float64
	for _, m := range models {
		md := deriveOne(book, m, upstream)
		if md.Discount != nil {
			vals = append(vals, *md.Discount)
		}
		d.Models = append(d.Models, md)
	}
	if len(vals) == 0 {
		return d
	}
	med := median(vals)
	d.Median = &med
	for i := range d.Models {
		if d.Models[i].Discount == nil {
			continue
		}
		dev := (*d.Models[i].Discount - med) / med
		d.Models[i].Deviation = &dev
		d.Models[i].Deviates = math.Abs(dev) > ModelDeviationThreshold
	}
	return d
}

func median(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// RoundDiscount rounds to 6 decimals so repeated reads don't churn on float noise.
func RoundDiscount(v float64) float64 { return math.Round(v*1e6) / 1e6 }
