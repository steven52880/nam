package channels

import "strings"

// Managed groups are the GroupName of every vendor. nam only adds, removes and renames
// those; any other group on a NewAPI channel is kept as is, in its original position.

// splitGroups splits a comma separated group list, trimming items and dropping empty
// and duplicate ones (first occurrence wins).
func splitGroups(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, g := range strings.Split(s, ",") {
		g = strings.TrimSpace(g)
		if g == "" || seen[g] {
			continue
		}
		seen[g] = true
		out = append(out, g)
	}
	return out
}

func joinGroups(gs []string) string { return strings.Join(gs, ",") }

func normGroups(s string) string { return joinGroups(splitGroups(s)) }

func containsGroup(gs []string, g string) bool {
	for _, x := range gs {
		if x == g {
			return true
		}
	}
	return false
}

func (ix *vendorIndex) isManaged(g string) bool {
	_, ok := ix.byGroup[g]
	return ok
}

// groupPred reports whether a group is managed or listed in drop (groups a vendor rename
// retired: they are no longer managed but must still be removed from NewAPI).
func (st *state) groupPred(drop []string) func(string) bool {
	return func(g string) bool {
		return st.vendors.isManaged(g) || containsGroup(drop, g)
	}
}

// managedSubset keeps the managed groups of gs, in order.
func managedSubset(gs []string, isManaged func(string) bool) []string {
	var out []string
	for _, g := range gs {
		if isManaged(g) {
			out = append(out, g)
		}
	}
	return out
}

// sameGroupSet compares two group lists ignoring order.
func sameGroupSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, g := range a {
		if !containsGroup(b, g) {
			return false
		}
	}
	return true
}

// mergeManaged takes the non-managed groups from live and the managed groups from draft.
// Managed groups keep their live position; new ones fill the slots of removed ones, then
// go to the end.
func mergeManaged(live, draft []string, isManaged func(string) bool) []string {
	want := managedSubset(draft, isManaged)
	used := map[string]bool{}
	type item struct {
		g    string
		slot bool
	}
	var items []item
	for _, g := range live {
		switch {
		case !isManaged(g):
			items = append(items, item{g: g})
		case containsGroup(want, g):
			used[g] = true
			items = append(items, item{g: g})
		default:
			items = append(items, item{slot: true})
		}
	}
	var rest []string
	for _, g := range want {
		if !used[g] {
			rest = append(rest, g)
		}
	}
	var out []string
	for _, it := range items {
		switch {
		case !it.slot:
			out = append(out, it.g)
		case len(rest) > 0:
			out = append(out, rest[0])
			rest = rest[1:]
		}
	}
	return dedupGroups(append(out, rest...))
}

func dedupGroups(gs []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, g := range gs {
		if !seen[g] {
			seen[g] = true
			out = append(out, g)
		}
	}
	return out
}

// swapVendorGroup moves a channel from vendor group oldG to newG. Clearing the vendor
// (newG == "") only removes oldG. Otherwise every managed group is replaced by newG at the
// position of oldG: a channel bound to a vendor must not carry another vendor's group.
func swapVendorGroup(gs []string, oldG, newG string, isManaged func(string) bool) []string {
	if newG == "" {
		var out []string
		for _, g := range gs {
			if g != oldG {
				out = append(out, g)
			}
		}
		return out
	}
	pos := len(gs)
	for i, g := range gs {
		if g == oldG {
			pos = i
			break
		}
	}
	if pos == len(gs) {
		for i, g := range gs {
			if isManaged(g) {
				pos = i
				break
			}
		}
	}
	var out []string
	for i := 0; i <= len(gs); i++ {
		if i == pos {
			out = append(out, newG)
		}
		if i < len(gs) && gs[i] != oldG && !isManaged(gs[i]) {
			out = append(out, gs[i])
		}
	}
	return dedupGroups(out)
}

// renameGroup replaces oldG by newG in place.
func renameGroup(gs []string, oldG, newG string) []string {
	out := make([]string, len(gs))
	for i, g := range gs {
		if g == oldG {
			g = newG
		}
		out[i] = g
	}
	return dedupGroups(out)
}
