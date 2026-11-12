package channels

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/steven52880/nam/ports"
)

// snapshot is the set of NewAPI fields nam manages; drift = NewAPI value differs from the
// snapshot taken at nam's last apply. Key is excluded (never read back reliably).
type snapshot struct {
	Models    []string `json:"models"`
	Group     string   `json:"group"`
	Priority  int64    `json:"priority"`
	Weight    uint     `json:"weight"`
	Tag       string   `json:"tag"`
	Status    int      `json:"status"`
	TestModel string   `json:"test_model"`
	BaseURL   string   `json:"base_url"`
}

func snapOf(c ports.Channel) snapshot {
	m := append([]string(nil), c.Models...)
	sort.Strings(m)
	return snapshot{Models: m, Group: c.Group, Priority: c.Priority, Weight: c.Weight, Tag: c.Tag,
		Status: normStatus(c.Status), TestModel: c.TestModel, BaseURL: c.BaseURL}
}

// normStatus folds auto-disabled (3) into enabled (1): NewAPI flips 1↔3 by itself, which
// is reported as its own state, not as drift.
func normStatus(s int) int {
	if s == ports.ChannelAutoDisabled {
		return ports.ChannelEnabled
	}
	return s
}

func (s snapshot) hash() string {
	b, _ := json.Marshal(s)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (s snapshot) json() string {
	b, _ := json.Marshal(s)
	return string(b)
}

func parseSnap(raw string) (snapshot, bool) {
	var s snapshot
	if raw == "" || json.Unmarshal([]byte(raw), &s) != nil {
		return s, false
	}
	return s, true
}

// diffSnap lists managed fields that differ between the applied snapshot and NewAPI.
// Only managed groups are compared, judged by the current vendor set.
func diffSnap(applied, now snapshot, isManaged func(string) bool) []string {
	var out []string
	if !equalStrings(applied.Models, now.Models) {
		out = append(out, "models")
	}
	if !sameGroupSet(managedSubset(splitGroups(applied.Group), isManaged), managedSubset(splitGroups(now.Group), isManaged)) {
		out = append(out, "group")
	}
	if applied.Priority != now.Priority {
		out = append(out, "priority")
	}
	if applied.Weight != now.Weight {
		out = append(out, "weight")
	}
	if applied.Tag != now.Tag {
		out = append(out, "tag")
	}
	if applied.Status != now.Status {
		out = append(out, "status")
	}
	if applied.TestModel != now.TestModel {
		out = append(out, "test_model")
	}
	if applied.BaseURL != now.BaseURL {
		out = append(out, "base_url")
	}
	return out
}

func equalStrings(a, b []string) bool {
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

// overlay applies draft fields on top of a NewAPI channel (for display/validation).
func overlay(c ports.Channel, f Fields) ports.Channel {
	if f.Name != nil {
		c.Name = *f.Name
	}
	if f.BaseURL != nil {
		c.BaseURL = *f.BaseURL
	}
	if f.Models != nil {
		c.Models = append([]string(nil), (*f.Models)...)
	}
	if f.Group != nil {
		c.Group = *f.Group
	}
	if f.Priority != nil {
		c.Priority = *f.Priority
	}
	if f.Weight != nil {
		c.Weight = *f.Weight
	}
	if f.Tag != nil {
		c.Tag = *f.Tag
	}
	if f.Status != nil {
		c.Status = *f.Status
	}
	if f.TestModel != nil {
		c.TestModel = *f.TestModel
	}
	if f.Type != nil {
		c.Type = *f.Type
	}
	return c
}

// mergeFields layers b over a (b wins).
func mergeFields(a, b Fields) Fields {
	if b.Name != nil {
		a.Name = b.Name
	}
	if b.BaseURL != nil {
		a.BaseURL = b.BaseURL
	}
	if b.Models != nil {
		a.Models = b.Models
	}
	if b.Group != nil {
		a.Group = b.Group
	}
	if b.Priority != nil {
		a.Priority = b.Priority
	}
	if b.Weight != nil {
		a.Weight = b.Weight
	}
	if b.Tag != nil {
		a.Tag = b.Tag
	}
	if b.Status != nil {
		a.Status = b.Status
	}
	if b.TestModel != nil {
		a.TestModel = b.TestModel
	}
	if b.Type != nil {
		a.Type = b.Type
	}
	return a
}

func mergeExt(a, b ExtFields) ExtFields {
	if b.UpstreamID != nil {
		a.UpstreamID = b.UpstreamID
	}
	if b.TokenID != nil {
		a.TokenID = b.TokenID
	}
	if b.Vendor != nil {
		a.Vendor = b.Vendor
	}
	if b.Discount != nil {
		a.Discount = b.Discount
	}
	if b.DiscountSource != nil {
		a.DiscountSource = b.DiscountSource
	}
	return a
}

// pruneFields drops fields whose value already equals NewAPI's (so a draft that was edited
// back to the original disappears).
func pruneFields(c ports.Channel, f Fields, isManaged func(string) bool) Fields {
	if f.Name != nil && *f.Name == c.Name {
		f.Name = nil
	}
	if f.BaseURL != nil && *f.BaseURL == c.BaseURL {
		f.BaseURL = nil
	}
	if f.Models != nil && equalStrings(*f.Models, c.Models) {
		f.Models = nil
	}
	if f.Group != nil && joinGroups(mergeManaged(splitGroups(c.Group), splitGroups(*f.Group), isManaged)) == normGroups(c.Group) {
		f.Group = nil
	}
	if f.Priority != nil && *f.Priority == c.Priority {
		f.Priority = nil
	}
	if f.Weight != nil && *f.Weight == c.Weight {
		f.Weight = nil
	}
	if f.Tag != nil && *f.Tag == c.Tag {
		f.Tag = nil
	}
	if f.Status != nil && *f.Status == c.Status {
		f.Status = nil
	}
	if f.TestModel != nil && *f.TestModel == c.TestModel {
		f.TestModel = nil
	}
	f.Type = nil // type of an existing channel is never changed by nam
	return f
}

// dirtyNames lists the json names of set fields (for cell highlighting).
func dirtyNames(f Fields, e ExtFields) []string {
	var out []string
	add := func(ok bool, n string) {
		if ok {
			out = append(out, n)
		}
	}
	add(f.Name != nil, "name")
	add(f.BaseURL != nil, "base_url")
	add(f.Models != nil, "models")
	add(f.Group != nil, "group")
	add(f.Priority != nil, "priority")
	add(f.Weight != nil, "weight")
	add(f.Tag != nil, "tag")
	add(f.Status != nil, "status")
	add(f.TestModel != nil, "test_model")
	add(e.UpstreamID != nil, "upstream_id")
	add(e.TokenID != nil, "token_id")
	add(e.Vendor != nil, "vendor")
	add(e.Discount != nil, "discount")
	return out
}
