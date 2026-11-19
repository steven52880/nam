package channels

import "github.com/steven52880/nam/ports"

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
	// Vendor never enters a draft: it was already turned into a group change.
	switch {
	case b.Discount != nil:
		m := manualSource
		a.Discount, a.DiscountSource = b.Discount, &m
	case b.DiscountSource != nil && *b.DiscountSource == autoSource:
		au := autoSource
		a.Discount, a.DiscountSource = nil, &au // 清掉手填，回到自动
	}
	return a
}

// Discount sources a draft can carry: a number sets the manual value, "auto" clears it.
const (
	manualSource = "manual"
	autoSource   = "auto"
)

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
	add(e.Discount != nil || e.DiscountSource != nil, "discount")
	return out
}
