package portstest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/steven52880/nam/ports"
)

// ContractEnv lets the suite drive a real backend.
type ContractEnv struct {
	Ports ports.Ports
	// Prefix makes channel names unique per run (e.g. "nam-contract-<unix>").
	Prefix string
	// SkipTest skips ChannelStore.Test (it needs a reachable upstream).
	SkipTest bool
}

// RunContract exercises ChannelStore and OptionStore semantics every adapter must honor.
// It creates and deletes its own channels and restores the options it touches.
func RunContract(t *testing.T, env ContractEnv) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cs, os := env.Ports.Channels, env.Ports.Options
	if env.Prefix == "" {
		env.Prefix = fmt.Sprintf("nam-contract-%d", time.Now().UnixNano())
	}

	t.Run("channel create returns id and fields round-trip", func(t *testing.T) {
		in := ports.ChannelCreate{
			Type: 58, Name: env.Prefix + "-a", Key: "sk-contract-a", BaseURL: "http://127.0.0.1:1",
			Models: []string{"test-model", "test-model-2"}, Group: "default", Priority: 3, Weight: 2,
			Tag: "contract", Status: ports.ChannelEnabled, TestModel: "test-model",
		}
		id, err := cs.Create(ctx, in)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if id <= 0 {
			t.Fatalf("create returned id %d", id)
		}
		t.Cleanup(func() { _ = cs.Delete(context.Background(), id) })
		got, err := cs.Get(ctx, id)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Name != in.Name || got.Type != 58 || got.Group != "default" || got.Priority != 3 || got.Weight != 2 ||
			got.Tag != "contract" || got.Status != ports.ChannelEnabled || got.TestModel != "test-model" || got.BaseURL != in.BaseURL {
			t.Fatalf("round-trip mismatch: %+v", got)
		}
		if len(got.Models) != 2 || got.Models[0] != "test-model" || got.Models[1] != "test-model-2" {
			t.Fatalf("models mismatch: %v", got.Models)
		}

		list, err := cs.List(ctx)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		found := false
		for _, c := range list {
			if c.ID == id {
				found = true
			}
		}
		if !found {
			t.Fatal("created channel missing from List")
		}

		t.Run("patch updates only given fields, including priority 0", func(t *testing.T) {
			zero := int64(0)
			w := uint(7)
			st := ports.ChannelManualDisabled
			if err := cs.Update(ctx, id, ports.ChannelPatch{Priority: &zero, Weight: &w, Status: &st}); err != nil {
				t.Fatalf("update: %v", err)
			}
			got, err := cs.Get(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if got.Priority != 0 || got.Weight != 7 || got.Status != ports.ChannelManualDisabled {
				t.Fatalf("patch not applied: %+v", got)
			}
			if got.Tag != "contract" || len(got.Models) != 2 || got.Group != "default" {
				t.Fatalf("patch clobbered other fields: %+v", got)
			}
			models := []string{"test-model"}
			if err := cs.Update(ctx, id, ports.ChannelPatch{Models: &models}); err != nil {
				t.Fatal(err)
			}
			got, _ = cs.Get(ctx, id)
			if len(got.Models) != 1 {
				t.Fatalf("models patch: %v", got.Models)
			}
		})

		if !env.SkipTest {
			t.Run("test does not error", func(t *testing.T) {
				if _, err := cs.Test(ctx, id, "test-model"); err != nil {
					t.Fatalf("test: %v", err)
				}
			})
		}

		t.Run("delete then get is not found", func(t *testing.T) {
			if err := cs.Delete(ctx, id); err != nil {
				t.Fatalf("delete: %v", err)
			}
			if _, err := cs.Get(ctx, id); err == nil {
				t.Fatal("get after delete must fail")
			}
		})
	})

	t.Run("get unknown channel", func(t *testing.T) {
		if _, err := cs.Get(ctx, 987654321); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("option set and read back", func(t *testing.T) {
		const key = "GroupRatio"
		orig, err := os.Get(ctx, key)
		if err != nil {
			t.Fatalf("get option: %v", err)
		}
		before, had := orig[key]
		t.Cleanup(func() {
			if had {
				_ = os.Set(context.Background(), key, before)
			}
		})
		val := `{"default":1,"nam-contract":0.05}`
		if err := os.Set(ctx, key, val); err != nil {
			t.Fatalf("set: %v", err)
		}
		got, err := os.Get(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		if !jsonEqual(got[key], val) {
			t.Fatalf("read back %q, want %q", got[key], val)
		}
	})

	t.Run("backend info", func(t *testing.T) {
		info, err := env.Ports.Backend.Info(ctx)
		if err != nil {
			t.Fatalf("info: %v", err)
		}
		if info.QuotaPerUnit <= 0 {
			t.Fatalf("quota_per_unit = %v", info.QuotaPerUnit)
		}
	})
}
