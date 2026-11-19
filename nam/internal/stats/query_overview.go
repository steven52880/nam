package stats

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
)

// names resolves display names and upstream ownership for dimension keys.
type names struct {
	channel  map[int]string
	upstream map[int]uint // managed channel -> upstream id
	upName   map[uint]string
}

func (s *Service) loadNames(ctx context.Context) (names, error) {
	chans, err := s.Channels.List(ctx)
	if err != nil {
		return names{}, resp.Upstream(fmt.Errorf("读取渠道失败：%w", err))
	}
	n := names{channel: map[int]string{}, upstream: map[int]uint{}, upName: map[uint]string{}}
	for _, ch := range chans {
		n.channel[ch.ID] = ch.Name
	}
	db := s.St.DB.WithContext(ctx)
	var exts []store.ChannelExt
	if err := db.Select("channel_id", "upstream_id").Find(&exts).Error; err != nil {
		return n, err
	}
	for _, e := range exts {
		if e.UpstreamID != nil {
			n.upstream[e.ChannelID] = *e.UpstreamID
		}
	}
	var ups []store.Upstream
	if err := db.Select("id", "name").Find(&ups).Error; err != nil {
		return n, err
	}
	for _, u := range ups {
		n.upName[u.ID] = u.Name
	}
	return n, nil
}

func (n names) channelName(id int) string {
	if name, ok := n.channel[id]; ok {
		return name
	}
	return fmt.Sprintf("#%d（已删除）", id)
}

// keyOf returns the dimension key and display name of a row.
func (n names) keyOf(dim string, r store.StatsHourly) (string, string) {
	switch dim {
	case DimChannel:
		return strconv.Itoa(r.ChannelID), n.channelName(r.ChannelID)
	case DimModel:
		return r.Model, r.Model
	case DimUpstream:
		id, ok := n.upstream[r.ChannelID]
		if !ok {
			return "", "未纳管"
		}
		name, ok := n.upName[id]
		if !ok {
			name = fmt.Sprintf("#%d", id)
		}
		return strconv.FormatUint(uint64(id), 10), name
	default: // DimVendor
		if r.Vendor == "" {
			return "", "未归属"
		}
		return r.Vendor, r.Vendor
	}
}

type group struct {
	name string
	agg  Agg
}

// Overview is GET /stats/overview.
func (s *Service) Overview(ctx context.Context, from, to int64, dim string) (OverviewResult, error) {
	out := OverviewResult{From: from, To: to, Dim: dim, Rows: []StatsRow{}, Scatter: []ScatterPoint{}, ErrorClasses: map[string]int64{}}
	if err := validRange(from, to); err != nil {
		return out, err
	}
	if !validDim(dim) {
		return out, resp.Invalid("dim 必须是 channel、model、upstream 或 vendor")
	}
	nm, err := s.loadNames(ctx) // before streaming: no queries while rows are open
	if err != nil {
		return out, err
	}
	totals := NewAgg()
	groups := map[string]*group{}
	perChannel := map[int]Agg{}
	vendorOf := map[int]string{}
	err = scanRows(s.hourRange(ctx, from, to), func(r store.StatsHourly) {
		a := FromRow(r)
		totals.Merge(a)
		key, name := nm.keyOf(dim, r)
		g, ok := groups[key]
		if !ok {
			g = &group{name: name, agg: NewAgg()}
			groups[key] = g
		}
		g.agg.Merge(a)
		c, ok := perChannel[r.ChannelID]
		if !ok {
			c = NewAgg()
		}
		c.Merge(a)
		perChannel[r.ChannelID] = c
		if r.Vendor != "" {
			vendorOf[r.ChannelID] = r.Vendor
		}
	})
	if err != nil {
		return out, err
	}
	usd := s.St.USDRate(ctx)
	out.Totals = totals.Metrics(usd)
	for k, n := range out.Totals.FailByClass {
		out.ErrorClasses[k] = n
	}
	for key, g := range groups {
		out.Rows = append(out.Rows, StatsRow{Metrics: g.agg.Metrics(usd), Key: key, Name: g.name})
	}
	sort.Slice(out.Rows, func(i, j int) bool {
		if out.Rows[i].Requests != out.Rows[j].Requests {
			return out.Rows[i].Requests > out.Rows[j].Requests
		}
		return out.Rows[i].Key < out.Rows[j].Key
	})
	if out.Scatter, err = s.scatter(ctx, nm, perChannel, vendorOf); err != nil {
		return out, err
	}
	out.PatchP1 = s.PatchP1(ctx)
	out.ErrorLogHint, err = s.ErrorLogHint(ctx)
	return out, err
}

func (s *Service) scatter(ctx context.Context, nm names, perChannel map[int]Agg, vendorOf map[int]string) ([]ScatterPoint, error) {
	out := []ScatterPoint{}
	chans, err := s.Channels.List(ctx)
	if err != nil {
		return out, resp.Upstream(fmt.Errorf("读取渠道失败：%w", err))
	}
	pr, err := NewPricer(ctx, s.St, s.Backend, s.Channels)
	if err != nil {
		return out, err
	}
	rt, err := s.rate(ctx, pr, chans, "")
	if err != nil {
		return out, err
	}
	for id, a := range perChannel {
		m := a.Metrics(pr.usdRate)
		out = append(out, ScatterPoint{
			ChannelID: id, Name: nm.channelName(id), Vendor: vendorOf[id],
			StdCostPrice: rt.std[id], Wilson: m.Wilson, Requests: m.Requests,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChannelID < out[j].ChannelID })
	return out, nil
}
