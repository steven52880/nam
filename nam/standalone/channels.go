package standalone

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/steven52880/nam/ports"
)

const (
	channelPageSize = 100 // NewAPI's GetPageQuery caps page_size at 100
	maxPages        = 1000
)

// apiChannel mirrors the model.Channel JSON fields nam cares about.
type apiChannel struct {
	ID           int     `json:"id"`
	Type         int     `json:"type"`
	Name         string  `json:"name"`
	BaseURL      *string `json:"base_url"`
	Models       string  `json:"models"`
	Group        string  `json:"group"`
	Priority     *int64  `json:"priority"`
	Weight       *uint   `json:"weight"`
	Tag          *string `json:"tag"`
	Status       int     `json:"status"`
	TestModel    *string `json:"test_model"`
	ResponseTime int     `json:"response_time"`
	TestTime     int64   `json:"test_time"`
	Balance      float64 `json:"balance"`
	UsedQuota    int64   `json:"used_quota"`
	CreatedTime  int64   `json:"created_time"`
}

type channelPage struct {
	Items          []apiChannel `json:"items"`
	CollapsedItems []apiChannel `json:"collapsed_items"`
	Total          int          `json:"total"`
}

func (a apiChannel) toPort() ports.Channel {
	return ports.Channel{
		ID: a.ID, Type: a.Type, Name: a.Name, BaseURL: deref(a.BaseURL), Models: splitModels(a.Models),
		Group: a.Group, Priority: derefInt64(a.Priority), Weight: derefUint(a.Weight), Tag: deref(a.Tag),
		Status: a.Status, TestModel: deref(a.TestModel), ResponseTime: a.ResponseTime, TestTime: a.TestTime,
		Balance: a.Balance, UsedQuota: a.UsedQuota, CreatedTime: a.CreatedTime,
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func derefUint(v *uint) uint {
	if v == nil {
		return 0
	}
	return *v
}

func splitModels(s string) []string {
	out := make([]string, 0)
	for _, m := range strings.Split(s, ",") {
		if m = strings.TrimSpace(m); m != "" {
			out = append(out, m)
		}
	}
	return out
}

// channelStore implements ports.ChannelStore over /api/channel/*.
type channelStore struct{ c *Client }

func (s channelStore) List(ctx context.Context) ([]ports.Channel, error) {
	seen := map[int]struct{}{}
	out := make([]ports.Channel, 0)
	add := func(items []apiChannel) {
		for _, it := range items {
			if _, dup := seen[it.ID]; !dup {
				seen[it.ID] = struct{}{}
				out = append(out, it.toPort())
			}
		}
	}
	for page := 1; page <= maxPages; page++ {
		var res channelPage
		q := url.Values{"p": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(channelPageSize)}, "id_sort": {"true"}}
		if err := s.c.call(ctx, http.MethodGet, "/api/channel/", q, nil, &res); err != nil {
			return nil, err
		}
		add(res.Items) // collapsed_items are returned in full on every page
		add(res.CollapsedItems)
		if len(res.Items) < channelPageSize {
			break
		}
	}
	return out, nil
}

func (s channelStore) Get(ctx context.Context, id int) (ports.Channel, error) {
	var ch apiChannel
	if err := s.c.call(ctx, http.MethodGet, "/api/channel/"+strconv.Itoa(id), nil, nil, &ch); err != nil {
		if isNotFound(err) {
			return ports.Channel{}, notFound("channel", id, err)
		}
		return ports.Channel{}, err
	}
	if ch.ID == 0 {
		return ports.Channel{}, notFound("channel", id, fmt.Errorf("empty response"))
	}
	return ch.toPort(), nil
}

// channelWire is the request body for create/update. Pointer fields let NewAPI's GORM
// Updates write explicit zeros (priority 0, weight 0, empty tag); plain string/int fields
// that are zero are skipped by NewAPI, so they can never be cleared through PUT (nam never
// needs to: status 0 is never written and name/group/models are non-empty).
type channelWire struct {
	ID        int     `json:"id,omitempty"`
	Type      int     `json:"type"`
	Name      string  `json:"name"`
	Key       string  `json:"key,omitempty"` // empty keeps the stored key on PUT
	BaseURL   *string `json:"base_url"`
	Models    string  `json:"models"`
	Group     string  `json:"group"`
	Priority  *int64  `json:"priority"`
	Weight    *uint   `json:"weight"`
	Tag       *string `json:"tag"`
	Status    int     `json:"status"`
	TestModel *string `json:"test_model"`
}

func (s channelStore) Create(ctx context.Context, in ports.ChannelCreate) (int, error) {
	wire := channelWire{
		Type: in.Type, Name: in.Name, Key: in.Key, BaseURL: &in.BaseURL, Models: strings.Join(in.Models, ","),
		Group: in.Group, Priority: &in.Priority, Weight: &in.Weight, Tag: &in.Tag, Status: in.Status, TestModel: &in.TestModel,
	}
	body := map[string]any{"mode": "single", "channel": wire}
	if err := s.c.call(ctx, http.MethodPost, "/api/channel/", nil, body, nil); err != nil {
		return 0, err
	}
	// The create response carries no ID: find the new channel by exact name.
	matches, err := s.findByName(ctx, in.Name)
	if err != nil {
		return 0, fmt.Errorf("channel %q was created but its id could not be resolved: %w", in.Name, err)
	}
	switch len(matches) {
	case 1:
		return matches[0].ID, nil
	case 0:
		return 0, fmt.Errorf("channel %q was created but not found by name search", in.Name)
	default:
		return 0, fmt.Errorf("channel name %q is ambiguous: %d channels share it", in.Name, len(matches))
	}
}

func (s channelStore) findByName(ctx context.Context, name string) ([]apiChannel, error) {
	seen := map[int]struct{}{}
	var matches []apiChannel
	collect := func(items []apiChannel) {
		for _, it := range items {
			if _, dup := seen[it.ID]; !dup && it.Name == name {
				seen[it.ID] = struct{}{}
				matches = append(matches, it)
			}
		}
	}
	for page := 1; page <= maxPages; page++ {
		var res channelPage
		q := url.Values{"keyword": {name}, "id_sort": {"true"}, "p": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(channelPageSize)}}
		if err := s.c.call(ctx, http.MethodGet, "/api/channel/search", q, nil, &res); err != nil {
			return nil, err
		}
		collect(res.Items)
		collect(res.CollapsedItems)
		if len(res.Items) < channelPageSize || page*channelPageSize >= res.Total {
			break
		}
	}
	return matches, nil
}

func (s channelStore) Update(ctx context.Context, id int, p ports.ChannelPatch) error {
	var cur apiChannel
	if err := s.c.call(ctx, http.MethodGet, "/api/channel/"+strconv.Itoa(id), nil, nil, &cur); err != nil {
		if isNotFound(err) {
			return notFound("channel", id, err)
		}
		return err
	}
	w := channelWire{
		ID: id, Type: cur.Type, Name: cur.Name, BaseURL: cur.BaseURL, Models: cur.Models, Group: cur.Group,
		Priority: cur.Priority, Weight: cur.Weight, Tag: cur.Tag, Status: cur.Status, TestModel: cur.TestModel,
	}
	if p.Name != nil {
		w.Name = *p.Name
	}
	if p.Key != nil {
		w.Key = *p.Key
	}
	if p.BaseURL != nil {
		w.BaseURL = p.BaseURL
	}
	if p.Models != nil {
		w.Models = strings.Join(*p.Models, ",")
	}
	if p.Group != nil {
		w.Group = *p.Group
	}
	if p.Priority != nil {
		w.Priority = p.Priority
	}
	if p.Weight != nil {
		w.Weight = p.Weight
	}
	if p.Tag != nil {
		w.Tag = p.Tag
	}
	if p.Status != nil {
		w.Status = *p.Status
	}
	if p.TestModel != nil {
		w.TestModel = p.TestModel
	}
	return s.c.call(ctx, http.MethodPut, "/api/channel/", nil, w, nil)
}

func (s channelStore) Delete(ctx context.Context, id int) error {
	err := s.c.call(ctx, http.MethodDelete, "/api/channel/"+strconv.Itoa(id), nil, nil, nil)
	if err != nil && isNotFound(err) {
		return notFound("channel", id, err)
	}
	return err
}

// Test runs NewAPI's channel test. A failed test is a result (Success=false), not an error.
func (s channelStore) Test(ctx context.Context, id int, model string) (ports.ChannelTestResult, error) {
	var res struct {
		Success bool    `json:"success"`
		Message string  `json:"message"`
		Time    float64 `json:"time"`
	}
	q := url.Values{}
	if model != "" {
		q.Set("model", model)
	}
	if err := s.c.do(ctx, http.MethodGet, "/api/channel/test/"+strconv.Itoa(id), q, nil, &res); err != nil {
		return ports.ChannelTestResult{}, err
	}
	return ports.ChannelTestResult{Success: res.Success, Message: res.Message, Duration: res.Time}, nil
}
