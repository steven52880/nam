package api

import (
	"context"
	"testing"

	"github.com/steven52880/nam/internal/store"
)

func TestTokenChannelCounts(t *testing.T) {
	s := newCredTestServer(t)
	ctx := context.Background()
	db := s.Store.DB.WithContext(ctx)

	up := store.Upstream{Name: "u", CurrencyRate: 1}
	if err := db.Create(&up).Error; err != nil {
		t.Fatal(err)
	}
	a := store.UpstreamToken{UpstreamID: up.ID, Name: "a"}
	b := store.UpstreamToken{UpstreamID: up.ID, Name: "b"}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	for _, chID := range []int{1, 2} {
		if err := db.Create(&store.ChannelExt{ChannelID: chID, UpstreamID: &up.ID, TokenID: &a.ID}).Error; err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.tokenChannelCounts(ctx, []uint{a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got[a.ID] != 2 || got[b.ID] != 0 {
		t.Fatalf("counts %v, want a=2 b=0", got)
	}

	empty, err := s.tokenChannelCounts(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty ids: %v, %v", empty, err)
	}
}
