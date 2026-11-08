package pricing

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// SiteSet is the set of models the site serves: the union of every NewAPI channel's
// models and every vendor's model list. It is the model set of the price-book sync
// and of the 模型与价格 page.
type SiteSet struct {
	Models       []string       // sorted, deduplicated
	ChannelCount map[string]int // NewAPI channels listing the model
	VendorOf     map[string]string
}

// LoadSiteModels reads NewAPI channels (read only) and the vendor table.
func LoadSiteModels(ctx context.Context, st *store.Store, chs ports.ChannelStore) (SiteSet, error) {
	live, err := chs.List(ctx)
	if err != nil {
		return SiteSet{}, err
	}
	set := SiteSet{ChannelCount: map[string]int{}, VendorOf: map[string]string{}}
	seen := map[string]bool{}
	for _, ch := range live {
		own := map[string]bool{}
		for _, m := range ch.Models {
			m = strings.TrimSpace(m)
			if m == "" || own[m] {
				continue
			}
			own[m] = true
			seen[m] = true
			set.ChannelCount[m]++
		}
	}
	var vs []store.Vendor
	if err := st.DB.WithContext(ctx).Order("`order`, id").Find(&vs).Error; err != nil {
		return SiteSet{}, err
	}
	for _, v := range vs {
		var ms []string
		if v.Models != "" {
			_ = json.Unmarshal([]byte(v.Models), &ms)
		}
		for _, m := range ms {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			seen[m] = true
			if _, ok := set.VendorOf[m]; !ok {
				set.VendorOf[m] = v.Name
			}
		}
	}
	set.Models = make([]string, 0, len(seen))
	for m := range seen {
		set.Models = append(set.Models, m)
	}
	sort.Strings(set.Models)
	return set, nil
}
