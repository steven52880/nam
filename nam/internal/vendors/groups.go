package vendors

import "strings"

// VendorOfGroups returns the vendor of a channel from its NewAPI group list (comma
// separated): the vendor, in vendor order, that owns the first group the channel carries.
// vs must be in vendor order (`order`, id). "" when no group belongs to a vendor.
func VendorOfGroups(vs []VendorGroup, groups string) string {
	have := map[string]bool{}
	for _, g := range strings.Split(groups, ",") {
		if g = strings.TrimSpace(g); g != "" {
			have[g] = true
		}
	}
	if len(have) == 0 {
		return ""
	}
	for _, v := range vs {
		if have[v.Group] {
			return v.Name
		}
	}
	return ""
}

// VendorGroup is the (vendor, group) pair VendorOfGroups needs.
type VendorGroup struct{ Name, Group string }
