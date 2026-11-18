package vendorprice

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// NewAPI option keys vendor pricing reads and writes.
const (
	KeyGroupRatio       = "GroupRatio"
	KeyUserUsableGroups = "UserUsableGroups"
	KeyAutoGroups       = "AutoGroups"
)

// optionOrder is the order in which changed options are written.
var optionOrder = []string{KeyGroupRatio, KeyUserUsableGroups, KeyAutoGroups}

// DefaultAutoLabel is the UserUsableGroups description nam gives "auto" when it is missing.
const DefaultAutoLabel = "自动选择"

// Options are the three NewAPI options, parsed.
type Options struct {
	GroupRatio       map[string]float64
	UserUsableGroups map[string]string
	AutoGroups       []string
}

// VendorRef is the part of a vendor the option merge needs (callers pass them in nam order).
type VendorRef struct {
	Name  string
	Group string
}

func blank(raw string) bool { return strings.TrimSpace(raw) == "" }

// ParseGroupRatio parses the GroupRatio option. An empty value is an empty object; invalid
// JSON is an error (callers must never overwrite an option they cannot parse).
func ParseGroupRatio(raw string) (map[string]float64, error) {
	out := map[string]float64{}
	if blank(raw) {
		return out, nil
	}
	var m map[string]float64
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("GroupRatio 不是合法的 JSON 对象（分组→倍率）：%w", err)
	}
	for k, v := range m {
		out[k] = v
	}
	return out, nil
}

// ParseUserUsableGroups parses the UserUsableGroups option (group → description).
func ParseUserUsableGroups(raw string) (map[string]string, error) {
	out := map[string]string{}
	if blank(raw) {
		return out, nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("UserUsableGroups 不是合法的 JSON 对象（分组→描述）：%w", err)
	}
	for k, v := range m {
		out[k] = v
	}
	return out, nil
}

// ParseAutoGroups parses the AutoGroups option (JSON array of group names).
func ParseAutoGroups(raw string) ([]string, error) {
	out := []string{}
	if blank(raw) {
		return out, nil
	}
	var a []string
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return nil, fmt.Errorf("AutoGroups 不是合法的 JSON 数组：%w", err)
	}
	return append(out, a...), nil
}

// MergeGroupRatio returns cur with the given group keys set. Other keys are kept as is.
func MergeGroupRatio(cur map[string]float64, set map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(cur)+len(set))
	for k, v := range cur {
		out[k] = v
	}
	for k, v := range set {
		out[k] = v
	}
	return out
}

// MergeUserUsableGroups adds every vendor group (description = vendor name) and "auto"
// (description 自动选择) when missing. Existing entries are never changed.
func MergeUserUsableGroups(cur map[string]string, vs []VendorRef) map[string]string {
	out := make(map[string]string, len(cur)+len(vs)+1)
	for k, v := range cur {
		out[k] = v
	}
	for _, v := range vs {
		if _, ok := out[v.Group]; !ok {
			out[v.Group] = v.Name
		}
	}
	if _, ok := out["auto"]; !ok {
		out["auto"] = DefaultAutoLabel
	}
	return out
}

// MergeAutoGroups lists the vendor groups first (nam order), then the other existing items
// in their original relative order. Duplicates are dropped.
func MergeAutoGroups(cur []string, vs []VendorRef) []string {
	isVendor := make(map[string]bool, len(vs))
	out := make([]string, 0, len(cur)+len(vs))
	seen := map[string]bool{}
	for _, v := range vs {
		isVendor[v.Group] = true
		if !seen[v.Group] {
			seen[v.Group] = true
			out = append(out, v.Group)
		}
	}
	for _, g := range cur {
		if isVendor[g] || seen[g] {
			continue
		}
		seen[g] = true
		out = append(out, g)
	}
	return out
}

// Marshal helpers. Maps are serialized with sorted keys; nil values become empty containers.

func MarshalGroupRatio(m map[string]float64) string {
	if m == nil {
		m = map[string]float64{}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func MarshalUserUsableGroups(m map[string]string) string {
	if m == nil {
		m = map[string]string{}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func MarshalAutoGroups(a []string) string {
	if a == nil {
		a = []string{}
	}
	b, _ := json.Marshal(a)
	return string(b)
}

// Comparison by parsed value (key order and whitespace do not matter).

func EqualGroupRatio(a, b map[string]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, x := range a {
		y, ok := b[k]
		if !ok || math.Abs(x-y) > 1e-9 {
			return false
		}
	}
	return true
}

func EqualUserUsableGroups(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, x := range a {
		if y, ok := b[k]; !ok || x != y {
			return false
		}
	}
	return true
}

func EqualAutoGroups(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ParseOptions parses the three options from a raw option map. Options that fail to parse
// are reported in errs (key → error) and left empty in opts.
func ParseOptions(raw map[string]string) (opts Options, errs map[string]error) {
	errs = map[string]error{}
	var err error
	if opts.GroupRatio, err = ParseGroupRatio(raw[KeyGroupRatio]); err != nil {
		errs[KeyGroupRatio] = err
		opts.GroupRatio = map[string]float64{}
	}
	if opts.UserUsableGroups, err = ParseUserUsableGroups(raw[KeyUserUsableGroups]); err != nil {
		errs[KeyUserUsableGroups] = err
		opts.UserUsableGroups = map[string]string{}
	}
	if opts.AutoGroups, err = ParseAutoGroups(raw[KeyAutoGroups]); err != nil {
		errs[KeyAutoGroups] = err
		opts.AutoGroups = []string{}
	}
	return opts, errs
}
