package standalone

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// optionStore implements ports.OptionStore over /api/option/.
type optionStore struct{ c *Client }

type optionRow struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (s optionStore) Get(ctx context.Context, keys ...string) (map[string]string, error) {
	var rows []optionRow
	if err := s.c.call(ctx, http.MethodGet, "/api/option/", nil, nil, &rows); err != nil {
		return nil, err
	}
	want := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		want[k] = struct{}{}
	}
	out := make(map[string]string, len(keys))
	for _, r := range rows {
		if _, ok := want[r.Key]; ok || len(want) == 0 {
			out[r.Key] = r.Value
		}
	}
	return out, nil
}

// Set writes one option (value is sent as a string) and reads it back to verify.
func (s optionStore) Set(ctx context.Context, key, value string) error {
	body := map[string]string{"key": key, "value": value}
	if err := s.c.call(ctx, http.MethodPut, "/api/option/", nil, body, nil); err != nil {
		return err
	}
	got, err := s.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("read back option %q: %w", key, err)
	}
	v, present := got[key]
	if !present {
		if isHiddenOption(key) {
			return nil // NewAPI never lists secrets, so there is nothing to compare
		}
		return fmt.Errorf("option %q missing after set", key)
	}
	if !valuesEqual(v, value) {
		return fmt.Errorf("option %q read back %q, wanted %q", key, v, value)
	}
	return nil
}

// isHiddenOption mirrors controller.GetOptions, which omits sensitive-looking keys.
func isHiddenOption(key string) bool {
	for _, suffix := range []string{"Token", "Secret", "Key", "secret", "api_key"} {
		if strings.HasSuffix(key, suffix) {
			return true
		}
	}
	return false
}

func valuesEqual(a, b string) bool {
	if a == b {
		return true
	}
	var x, y any
	if json.Unmarshal([]byte(a), &x) != nil || json.Unmarshal([]byte(b), &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}
