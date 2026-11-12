package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/internal/upstream"
	"github.com/steven52880/nam/ports"
)

const maxTokenNameRunes = 128

// MatchItem is the token match result for one channel.
type MatchItem struct {
	ChannelID   int    `json:"channel_id"`
	TokenID     *uint  `json:"token_id"`
	TokenName   string `json:"token_name"`
	Created     bool   `json:"created"`
	NeedsManual bool   `json:"needs_manual"`
	Message     string `json:"message"`
}

// MatchTokens finds, for each channel, the token of the upstream whose key equals the
// channel's key. When none matches, a manual token is recorded (key sealed, models taken
// from the channel). It only writes nam.db: never NewAPI, never drafts.
// Channels whose key cannot be read (standalone) or holds several keys need manual choice.
func (s *Service) MatchTokens(ctx context.Context, upstreamID uint, channelIDs []int) ([]MatchItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.St.GetUpstream(ctx, upstreamID); err != nil {
		return nil, err
	}
	var toks []store.UpstreamToken
	if err := s.St.DB.WithContext(ctx).Where("upstream_id = ?", upstreamID).Order("id").Find(&toks).Error; err != nil {
		return nil, err
	}
	keys := make([]string, len(toks)) // decrypted, trimmed; "" = unusable
	names := map[string]bool{}
	for i, t := range toks {
		names[t.Name] = true
		if len(t.Key) == 0 {
			continue
		}
		if pt, err := s.Box.Open(t.Key); err == nil {
			keys[i] = strings.TrimSpace(string(pt))
		}
	}
	out := make([]MatchItem, 0, len(channelIDs))
	for _, id := range channelIDs {
		item := MatchItem{ChannelID: id}
		ch, err := s.Channels.Get(ctx, id)
		if errors.Is(err, ports.ErrNotFound) {
			item.NeedsManual, item.Message = true, "渠道不存在"
			out = append(out, item)
			continue
		}
		if err != nil {
			return nil, resp.Upstream(fmt.Errorf("读取渠道 %d 失败：%w", id, err))
		}
		key := strings.TrimSpace(ch.Key)
		switch {
		case key == "":
			item.NeedsManual, item.Message = true, "独立模式拿不到渠道 key，请手动选择令牌"
		case strings.Contains(key, "\n"):
			item.NeedsManual, item.Message = true, "多 key 渠道，请手动选择令牌"
		default:
			idx := -1
			for i, k := range keys {
				if k != "" && k == key {
					idx = i
					break
				}
			}
			if idx < 0 {
				t, err := s.createMatchToken(ctx, upstreamID, ch, key, names)
				if err != nil {
					return nil, err
				}
				toks = append(toks, t)
				keys = append(keys, key)
				names[t.Name] = true
				idx = len(toks) - 1
				item.Created = true
			}
			tid := toks[idx].ID
			item.TokenID, item.TokenName = &tid, toks[idx].Name
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) createMatchToken(ctx context.Context, upstreamID uint, ch ports.Channel, key string, names map[string]bool) (store.UpstreamToken, error) {
	sealed, err := s.Box.Seal([]byte(key))
	if err != nil {
		return store.UpstreamToken{}, err
	}
	t := store.UpstreamToken{UpstreamID: upstreamID, Name: uniqueTokenName(ch.Name, ch.ID, names), Key: sealed,
		KeyHint: upstream.KeyHint(key), Status: 1, Manual: true, Models: dedupe(ch.Models), UpdatedAt: time.Now().Unix()}
	if t.Models == nil {
		t.Models = []string{}
	}
	if err := s.St.DB.WithContext(ctx).Create(&t).Error; err != nil {
		return t, err
	}
	return t, nil
}

// uniqueTokenName truncates the channel name to 128 runes; a name already used on the
// upstream gets "-<channelId>" appended (keeping the total within the limit).
func uniqueTokenName(channelName string, channelID int, taken map[string]bool) string {
	name := truncateRunes(strings.TrimSpace(channelName), maxTokenNameRunes)
	if name == "" {
		name = fmt.Sprintf("channel-%d", channelID)
	}
	if !taken[name] {
		return name
	}
	suffix := fmt.Sprintf("-%d", channelID)
	return truncateRunes(name, maxTokenNameRunes-len([]rune(suffix))) + suffix
}

func truncateRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
