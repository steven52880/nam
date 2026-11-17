package stats

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
)

// histBase is the bucket growth factor 2^(1/8): about 4.5% relative error at the midpoint.
var (
	histBase   = math.Pow(2, 1.0/8)
	histLnBase = math.Log(histBase)
)

// Hist is a log-bucketed histogram: bucket index -> count. Used for first response
// time (ms) and output speed (token/s) with the same bucketing.
type Hist map[int]int64

// Add records one positive value; v <= 0 is ignored.
func (h Hist) Add(v float64) {
	if v <= 0 || math.IsInf(v, 0) || math.IsNaN(v) {
		return
	}
	h[int(math.Floor(math.Log(v)/histLnBase))]++
}

// Merge adds every bucket of o into h.
func (h Hist) Merge(o Hist) {
	for k, n := range o {
		h[k] += n
	}
}

// Count is the number of recorded values.
func (h Hist) Count() int64 {
	var n int64
	for _, c := range h {
		n += c
	}
	return n
}

// Quantile returns the geometric midpoint of the bucket holding quantile q (0–1).
// ok=false when the histogram is empty.
func (h Hist) Quantile(q float64) (float64, bool) {
	total := h.Count()
	if total == 0 {
		return 0, false
	}
	keys := make([]int, 0, len(h))
	for k, n := range h {
		if n > 0 {
			keys = append(keys, k)
		}
	}
	sort.Ints(keys)
	rank := int64(math.Ceil(q * float64(total)))
	rank = max(rank, 1)
	var cum int64
	for _, k := range keys {
		cum += h[k]
		if cum >= rank {
			return math.Pow(histBase, float64(k)+0.5), true
		}
	}
	return math.Pow(histBase, float64(keys[len(keys)-1])+0.5), true
}

// Encode serialises to the stored JSON form {"52":3,"53":5}; empty -> "".
func (h Hist) Encode() string {
	if len(h) == 0 {
		return ""
	}
	m := make(map[string]int64, len(h))
	for k, n := range h {
		if n != 0 {
			m[strconv.Itoa(k)] = n
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		// map[string]int64 always marshals; keep the zero value on the impossible path.
		return ""
	}
	return string(b)
}

// ParseHist decodes the stored JSON form. Invalid input yields an empty histogram
// (a corrupt row must not stop ingestion).
func ParseHist(raw string) Hist {
	h := Hist{}
	if raw == "" {
		return h
	}
	var m map[string]int64
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return h
	}
	for k, n := range m {
		if i, err := strconv.Atoi(k); err == nil {
			h[i] += n
		}
	}
	return h
}
