package vendors

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"gorm.io/gorm"
)

func newSvc(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "nam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return &Service{St: st}
}

// update runs UpdateTx in its own transaction (the production caller is channels.Service.UpdateVendor).
func update(s *Service, id uint, in Input) (store.Vendor, error) {
	var cur store.Vendor
	err := s.St.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		_, cur, err = UpdateTx(tx, id, in)
		return err
	})
	return cur, err
}

func TestCreateValidation(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()
	bad := map[string]Input{
		"empty name":    {Name: " ", GroupName: "g"},
		"long name":     {Name: strings.Repeat("n", 65), GroupName: "g"},
		"empty group":   {Name: "n", GroupName: ""},
		"long group":    {Name: "n", GroupName: strings.Repeat("g", 65)},
		"group chars":   {Name: "n", GroupName: "a b"},
		"channel type":  {Name: "n", GroupName: "g", ChannelType: 1},
		"empty prefix":  {Name: "n", GroupName: "g", Prefixes: []string{"gpt-", "  "}},
		"non-ascii grp": {Name: "n", GroupName: "组"},
	}
	for name, in := range bad {
		if _, err := s.Create(ctx, in); err == nil || resp.Status(err) != 400 {
			t.Errorf("%s: want 400, got %v", name, err)
		}
	}
	v, err := s.Create(ctx, Input{Name: "anthropic", GroupName: "anthropic", Models: []string{" m1 ", "m1", "", "m2"}})
	if err != nil || v.ChannelType != 58 || v.Models != `["m1","m2"]` {
		t.Fatalf("create: %+v %v", v, err)
	}
	g, err := s.Create(ctx, Input{Name: "gemini", GroupName: "gemini", ChannelType: 24})
	if err != nil || g.Order != 1 {
		t.Fatalf("second vendor: %+v %v", g, err)
	}
}

func TestUniquenessAndModelOverlap(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()
	_, _ = s.Create(ctx, Input{Name: "anthropic", GroupName: "anthropic", Models: []string{"m1"}})
	if _, err := s.Create(ctx, Input{Name: "anthropic", GroupName: "x"}); resp.Status(err) != 409 {
		t.Errorf("dup name: %v", err)
	}
	if _, err := s.Create(ctx, Input{Name: "x", GroupName: "anthropic"}); resp.Status(err) != 409 {
		t.Errorf("dup group: %v", err)
	}
	_, err := s.Create(ctx, Input{Name: "openai", GroupName: "openai", Models: []string{"m1"}})
	if resp.Status(err) != 409 || !strings.Contains(err.Error(), "anthropic") || !strings.Contains(err.Error(), "openai") {
		t.Errorf("overlap must name both vendors: %v", err)
	}
	o, _ := s.Create(ctx, Input{Name: "openai", GroupName: "openai", Models: []string{"m2"}})
	if _, err := update(s, o.ID, Input{Name: "openai", GroupName: "openai", Models: []string{"m1"}}); resp.Status(err) != 409 {
		t.Errorf("update overlap: %v", err)
	}
	if _, err := update(s, o.ID, Input{Name: "openai", GroupName: "openai", Models: []string{"m2", "m3"}}); err != nil {
		t.Errorf("update keeps own models: %v", err)
	}
}

func TestInUseVendorIsProtected(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()
	v, _ := s.Create(ctx, Input{Name: "anthropic", GroupName: "anthropic"})
	s.St.DB.Create(&store.ChannelExt{ChannelID: 7, Vendor: "anthropic"})
	if err := s.Delete(ctx, v.ID); resp.Status(err) != 409 {
		t.Errorf("delete in use: %v", err)
	}
	if _, err := update(s, v.ID, Input{Name: "renamed", GroupName: "anthropic"}); err != nil {
		t.Errorf("rename in use is allowed (channels migrate it): %v", err)
	}
	if _, err := update(s, v.ID, Input{Name: "renamed", GroupName: "anthropic", Models: []string{"m"}}); err != nil {
		t.Errorf("editing models in use is fine: %v", err)
	}
	// channels.Service.UpdateVendor migrates channel_ext.vendor; emulate it here
	s.St.DB.Model(&store.ChannelExt{}).Where("channel_id = ?", 7).Update("vendor", "renamed")
	if err := s.Delete(ctx, v.ID); resp.Status(err) != 409 {
		t.Errorf("delete in use after rename: %v", err)
	}
	s.St.DB.Where("channel_id = ?", 7).Delete(&store.ChannelExt{})
	if err := s.Delete(ctx, v.ID); err != nil {
		t.Errorf("delete unused: %v", err)
	}
	if err := s.Delete(ctx, v.ID); resp.Status(err) != 404 {
		t.Errorf("delete missing: %v", err)
	}
}

func TestReorder(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()
	a, _ := s.Create(ctx, Input{Name: "a", GroupName: "a"})
	b, _ := s.Create(ctx, Input{Name: "b", GroupName: "b"})
	c, _ := s.Create(ctx, Input{Name: "c", GroupName: "c"})
	if err := s.Reorder(ctx, []uint{c.ID, a.ID}); resp.Status(err) != 400 {
		t.Errorf("partial list: %v", err)
	}
	if err := s.Reorder(ctx, []uint{c.ID, c.ID, a.ID}); resp.Status(err) != 400 {
		t.Errorf("duplicate ids: %v", err)
	}
	if err := s.Reorder(ctx, []uint{c.ID, a.ID, b.ID}); err != nil {
		t.Fatal(err)
	}
	vs, _ := s.List(ctx)
	if len(vs) != 3 || vs[0].Name != "c" || vs[1].Name != "a" || vs[2].Name != "b" {
		t.Fatalf("order: %+v", vs)
	}
}
