package channels

import "context"

// LiveChannel is a channel as NewAPI has it right now (drafts are NOT overlaid), with its
// 上游渠道售价. Vendor pricing uses it so the reference cost only reflects what is in effect.
type LiveChannel struct {
	ID       int
	Name     string
	Group    string // comma separated, as in NewAPI
	Status   int
	Priority int64
	Weight   uint
	// ChannelCost is the 上游渠道售价 (same formula as Row.ChannelCost); nil when the channel
	// has no upstream or its upstream is unknown.
	ChannelCost *float64
}

// LiveChannels returns every NewAPI channel with its effective (published) values.
func (s *Service) LiveChannels(ctx context.Context) ([]LiveChannel, error) {
	st, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LiveChannel, 0, len(st.live))
	for _, c := range st.live {
		lc := LiveChannel{ID: c.ID, Name: c.Name, Group: c.Group, Status: c.Status, Priority: c.Priority, Weight: c.Weight}
		if e, ok := st.ext[c.ID]; ok && e.UpstreamID != nil {
			if u, ok := st.ups[*e.UpstreamID]; ok {
				cost := st.channelCost(e.Discount, u.CurrencyRate)
				lc.ChannelCost = &cost
			}
		}
		out = append(out, lc)
	}
	return out, nil
}
