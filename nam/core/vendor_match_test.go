package core_test

import (
	"encoding/json"
	"testing"
)

func TestVendorMatchEndpoint(t *testing.T) {
	_, h, _ := newCore(t)
	if w := do(h, "POST", "/api/nam/vendors/match", `{"models":["gpt-4"]}`, ""); w.Code != 401 {
		t.Fatalf("anon: %d", w.Code)
	}
	for _, body := range []string{
		`{"name":"openai","group_name":"openai","prefixes":["gpt-"],"models":["o3"]}`,
		`{"name":"image","group_name":"image","prefixes":["gpt-image-"]}`,
	} {
		if w := do(h, "POST", "/api/nam/vendors", body, "100"); w.Code != 200 {
			t.Fatalf("create: %d %s", w.Code, w.Body)
		}
	}
	if w := do(h, "POST", "/api/nam/vendors", `{"name":"x","group_name":"x","prefixes":["GPT-"]}`, "100"); w.Code != 409 {
		t.Fatalf("prefix conflict: %d %s", w.Code, w.Body)
	}
	w := do(h, "POST", "/api/nam/vendors/match", `{"models":["gpt-image-1","GPT-4","o3","claude"]}`, "100")
	if w.Code != 200 {
		t.Fatalf("match: %d %s", w.Code, w.Body)
	}
	var out struct {
		Data struct {
			Items []struct {
				Model, Vendor, By, Prefix string
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	it := out.Data.Items
	if len(it) != 4 || it[0].Vendor != "image" || it[0].By != "prefix" || it[0].Prefix != "gpt-image-" ||
		it[1].Vendor != "openai" || it[2].By != "explicit" || it[3].Vendor != "" || it[3].By != "" {
		t.Fatalf("items: %+v", it)
	}
}
