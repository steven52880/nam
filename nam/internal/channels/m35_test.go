package channels

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// patchRecorder wraps a ChannelStore and remembers every Update patch.
type patchRecorder struct {
	ports.ChannelStore
	patches []ports.ChannelPatch
}

func (r *patchRecorder) Update(ctx context.Context, id int, p ports.ChannelPatch) error {
	r.patches = append(r.patches, p)
	return r.ChannelStore.Update(ctx, id, p)
}

func (e *env) addToken(t *testing.T, name, key string) store.UpstreamToken {
	t.Helper()
	sealed, err := e.svc.Box.Seal([]byte(key))
	must(t, err)
	tok := store.UpstreamToken{UpstreamID: e.up.ID, Name: name, Group: name, Key: sealed}
	must(t, e.svc.St.DB.Create(&tok).Error)
	return tok
}

func TestBatchTwoTokensOneVendorEach(t *testing.T) {
	e := newEnv(t)
	b := e.addToken(t, "b", "sk-b")
	prev, err := e.svc.BatchPreview(e.ctx, BatchRequest{UpstreamID: e.up.ID, TokenIDs: []uint{e.tok.ID, b.ID},
		TokenModels: map[string][]string{fmt.Sprint(e.tok.ID): {"test-model"}, fmt.Sprint(b.ID): {"gem-x"}}})
	must(t, err)
	if len(prev.Items) != 2 || len(prev.Warnings) != 0 {
		t.Fatalf("items=%d warnings=%v", len(prev.Items), prev.Warnings)
	}
	if prev.Items[0].Vendor != "anthropic" || prev.Items[0].TokenName != "a" || prev.Items[1].Vendor != "gemini" || prev.Items[1].TokenName != "b" {
		t.Fatalf("mapping: %+v", prev.Items)
	}
}

func TestBatchTokenSpanningTwoVendorsWarns(t *testing.T) {
	e := newEnv(t)
	prev, err := e.svc.BatchPreview(e.ctx, BatchRequest{UpstreamID: e.up.ID, TokenIDs: []uint{e.tok.ID},
		TokenModels: map[string][]string{fmt.Sprint(e.tok.ID): {"test-model", "gem-x", "mystery", "test-model"}}})
	must(t, err)
	if len(prev.Items) != 2 || len(prev.Warnings) != 1 || !strings.Contains(prev.Warnings[0], "2 个厂商") {
		t.Fatalf("items=%d warnings=%v", len(prev.Items), prev.Warnings)
	}
	if got := prev.UnassignedByToken[fmt.Sprint(e.tok.ID)]; len(got) != 1 || got[0] != "mystery" {
		t.Fatalf("unassigned_by_token: %v", prev.UnassignedByToken)
	}
	if len(prev.Unassigned) != 1 || prev.Unassigned[0] != "mystery" {
		t.Fatalf("unassigned: %v", prev.Unassigned)
	}
}

func TestBatchRequiresTokenModels(t *testing.T) {
	e := newEnv(t)
	prev, err := e.svc.BatchPreview(e.ctx, BatchRequest{UpstreamID: e.up.ID, TokenIDs: []uint{e.tok.ID},
		TokenModels: map[string][]string{fmt.Sprint(e.tok.ID): {" ", "test-model", "test-model"}}})
	must(t, err)
	if len(prev.Items) != 1 || prev.Items[0].Vendor != "anthropic" || len(*prev.Items[0].Fields.Models) != 1 {
		t.Fatalf("items: %+v", prev.Items)
	}
	// no models in the request → 400 naming the token (no fallback to a stored list)
	b := e.addToken(t, "empty-tok", "sk-e")
	_, err = e.svc.BatchPreview(e.ctx, BatchRequest{UpstreamID: e.up.ID, TokenIDs: []uint{b.ID}})
	if err == nil || !strings.Contains(err.Error(), "empty-tok") || !strings.Contains(err.Error(), "没有选择模型") {
		t.Fatalf("expected 400 naming token, got %v", err)
	}
}

func TestVendorChangeSetsGroup(t *testing.T) {
	e := newEnv(t)
	id, err := e.fake.Create(e.ctx, ports.ChannelCreate{Type: 58, Name: "old", Key: "sk-a", Models: []string{"test-model"}, Group: "default"})
	must(t, err)
	issues, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Ext: ExtFields{Vendor: p("anthropic")}}})
	must(t, err)
	if len(issues) > 0 {
		t.Fatalf("issues: %+v", issues)
	}
	if r := mustRow(t, e, id); r.Group != "default,anthropic" || r.Vendor != "anthropic" || r.SyncState != StateDraft {
		t.Fatalf("row: %+v", r)
	}
	if c, _ := e.fake.Get(e.ctx, id); c.Type != 58 {
		t.Fatalf("type changed: %d", c.Type)
	}
	// unknown vendor still fails validation
	issues, err = e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Ext: ExtFields{Vendor: p("nope")}}})
	must(t, err)
	if len(issues) == 0 {
		t.Fatal("unknown vendor must be rejected")
	}
}

func adopt(t *testing.T, e *env, liveKey string) (*patchRecorder, int) {
	t.Helper()
	rec := &patchRecorder{ChannelStore: e.svc.Channels}
	e.svc.Channels = rec
	id, err := e.fake.Create(e.ctx, ports.ChannelCreate{Type: 58, Name: "old", Key: liveKey, Models: []string{"test-model"}, Group: "default"})
	must(t, err)
	issues, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Ext: ExtFields{UpstreamID: &e.up.ID, TokenID: &e.tok.ID, Vendor: p("anthropic")}}})
	must(t, err)
	if len(issues) > 0 {
		t.Fatalf("issues: %+v", issues)
	}
	res, issues, err := e.svc.Apply(e.ctx, nil)
	must(t, err)
	if len(issues) > 0 || len(res) != 1 || !res[0].OK {
		t.Fatalf("apply: %+v %+v", res, issues)
	}
	return rec, id
}

func TestAdoptIdenticalKeyDoesNotWriteKey(t *testing.T) {
	e := newEnv(t)
	rec, id := adopt(t, e, " sk-a\n")
	if len(rec.patches) != 1 || rec.patches[0].Key != nil {
		t.Fatalf("patches: %+v", rec.patches)
	}
	if c, _ := e.fake.Get(e.ctx, id); c.Group != "default,anthropic" || c.Key != " sk-a\n" {
		t.Fatalf("channel: %+v", c)
	}
}

func TestAdoptDifferentKeyWritesKey(t *testing.T) {
	e := newEnv(t)
	rec, id := adopt(t, e, "sk-other")
	if len(rec.patches) != 1 || rec.patches[0].Key == nil || *rec.patches[0].Key != "sk-a" {
		t.Fatalf("patches: %+v", rec.patches)
	}
	if c, _ := e.fake.Get(e.ctx, id); c.Key != "sk-a" {
		t.Fatalf("key: %q", c.Key)
	}
}

func TestAdoptUnreadableKeyFirstTimeSkipsThenSwitchWrites(t *testing.T) {
	e := newEnv(t)
	rec, id := adopt(t, e, "") // standalone: live key hidden, first adoption
	if len(rec.patches) != 1 || rec.patches[0].Key != nil {
		t.Fatalf("first adoption must not write key: %+v", rec.patches)
	}
	// now managed: switching to another token writes the key even if live key is hidden
	b := e.addToken(t, "b", "sk-b")
	_, err := e.svc.SaveChanges(e.ctx, []Change{{ChannelID: id, Ext: ExtFields{TokenID: &b.ID}, Fields: Fields{Tag: p("x")}}})
	must(t, err)
	res, _, err := e.svc.Apply(e.ctx, nil)
	must(t, err)
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("apply: %+v", res)
	}
	if len(rec.patches) != 2 || rec.patches[1].Key == nil || *rec.patches[1].Key != "sk-b" {
		t.Fatalf("switch must write key: %+v", rec.patches)
	}
}

func matchEnv(t *testing.T) (*env, int) {
	t.Helper()
	e := newEnv(t)
	id, err := e.fake.Create(e.ctx, ports.ChannelCreate{Type: 1, Name: "ch-one", Key: "sk-a", Models: []string{"m1", "m2"}})
	must(t, err)
	return e, id
}

func tokenCount(t *testing.T, e *env) int64 {
	t.Helper()
	var n int64
	must(t, e.svc.St.DB.Model(&store.UpstreamToken{}).Count(&n).Error)
	return n
}

func TestMatchTokensExisting(t *testing.T) {
	e, id := matchEnv(t)
	items, err := e.svc.MatchTokens(e.ctx, e.up.ID, []int{id})
	must(t, err)
	if len(items) != 1 || items[0].Created || items[0].NeedsManual || items[0].TokenID == nil || *items[0].TokenID != e.tok.ID {
		t.Fatalf("items: %+v", items)
	}
	if n := tokenCount(t, e); n != 1 {
		t.Fatalf("tokens: %d", n)
	}
	if n, _ := e.svc.DraftCount(e.ctx); n != 0 {
		t.Fatal("match must not create drafts")
	}
}

func TestMatchTokensCreatesManualToken(t *testing.T) {
	e, _ := matchEnv(t)
	id, err := e.fake.Create(e.ctx, ports.ChannelCreate{Type: 1, Name: "a", Key: " sk-new-key-123 ", Models: []string{"m1", "m2"}})
	must(t, err)
	writes := e.fake.Writes
	items, err := e.svc.MatchTokens(e.ctx, e.up.ID, []int{id})
	must(t, err)
	if len(items) != 1 || !items[0].Created || items[0].TokenID == nil {
		t.Fatalf("items: %+v", items)
	}
	if e.fake.Writes != writes {
		t.Fatal("match-tokens must not write NewAPI")
	}
	var tok store.UpstreamToken
	must(t, e.svc.St.DB.First(&tok, *items[0].TokenID).Error)
	pt, err := e.svc.Box.Open(tok.Key)
	must(t, err)
	if string(pt) != "sk-new-key-123" || !tok.Manual || tok.Status != 1 || tok.Name != "a-"+fmt.Sprint(id) ||
		tok.KeyHint != "sk-ne…-123" || tok.UpstreamID != e.up.ID {
		t.Fatalf("token: %+v key=%q", tok, pt)
	}
	// running again matches the token just created
	again, err := e.svc.MatchTokens(e.ctx, e.up.ID, []int{id})
	must(t, err)
	if again[0].Created || *again[0].TokenID != tok.ID || tokenCount(t, e) != 2 {
		t.Fatalf("second run: %+v", again)
	}
}

func TestMatchTokensNeedsManual(t *testing.T) {
	e := newEnv(t)
	empty, _ := e.fake.Create(e.ctx, ports.ChannelCreate{Type: 1, Name: "empty", Key: "  "})
	multi, _ := e.fake.Create(e.ctx, ports.ChannelCreate{Type: 1, Name: "multi", Key: "sk-a\nsk-b"})
	items, err := e.svc.MatchTokens(e.ctx, e.up.ID, []int{empty, multi, 999})
	must(t, err)
	if len(items) != 3 {
		t.Fatalf("items: %+v", items)
	}
	for _, it := range items {
		if !it.NeedsManual || it.TokenID != nil || it.Created || it.Message == "" {
			t.Fatalf("item: %+v", it)
		}
	}
	if n := tokenCount(t, e); n != 1 {
		t.Fatalf("no token may be created: %d", n)
	}
}
