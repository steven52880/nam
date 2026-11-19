package store

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// legacyVendorPricingSchema is vendor_pricing as it was before M5.5.
const legacyVendorPricingSchema = "CREATE TABLE `vendor_pricings` (`vendor_id` integer,`mode` text DEFAULT 'fixed'," +
	"`sell_price` real,`markup` real,`last_written_price` real,`last_written_ratio` real,`last_written_at` integer," +
	"`state` text DEFAULT 'unset',`pending_price` real,`reference_cost` real,`updated_at` integer,PRIMARY KEY (`vendor_id`))"

func vendorPricingColumns(t *testing.T, db *gorm.DB) map[string]bool {
	t.Helper()
	var cols []struct{ Name string }
	if err := db.Raw("SELECT name FROM pragma_table_info('vendor_pricings')").Scan(&cols).Error; err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, c := range cols {
		out[c.Name] = true
	}
	return out
}

func seedLegacyVendorPricing(t *testing.T, path string, settings ...string) {
	t.Helper()
	raw, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	qs := []string{
		legacyVendorPricingSchema,
		"CREATE INDEX `idx_vendor_pricings_state` ON `vendor_pricings`(`state`)",
		// 1: fixed, to_write → draft; 2: fixed, synced; 3: follow, to_write; 4: fixed, to_write without a price
		// 5: fixed, pending (never produced by the old code for fixed mode, must not become a draft)
		// 6: fixed, to_write, price so small the ratio rounds to 0
		"INSERT INTO vendor_pricings VALUES (1, 'fixed', 0.3, NULL, 0.25, 0.257, 100, 'to_write', NULL, 0.1, 100)",
		"INSERT INTO vendor_pricings VALUES (2, 'fixed', 0.2857, NULL, 0.2857, 0.2857, 100, 'synced', NULL, NULL, 100)",
		"INSERT INTO vendor_pricings VALUES (3, 'follow', 0.4, 1.5, 0.4, 0.4, 100, 'to_write', NULL, 0.2, 100)",
		"INSERT INTO vendor_pricings VALUES (4, 'fixed', NULL, NULL, NULL, NULL, 0, 'to_write', NULL, NULL, 100)",
		"INSERT INTO vendor_pricings VALUES (5, 'fixed', 0.5, NULL, 0.4, 0.4, 100, 'pending', 0.5, NULL, 100)",
		"INSERT INTO vendor_pricings VALUES (6, 'fixed', 0.0001, NULL, NULL, NULL, 0, 'to_write', NULL, NULL, 100)",
	}
	qs = append(qs, "ALTER TABLE `vendor_pricings` ADD COLUMN `last_written` real") // stray column of an older schema
	if len(settings) > 0 {
		qs = append([]string{"CREATE TABLE `settings` (`key` text,`value` text,PRIMARY KEY (`key`))"}, qs...)
		for i := 0; i+1 < len(settings); i += 2 {
			qs = append(qs, "INSERT INTO settings VALUES ('"+settings[i]+"', '"+settings[i+1]+"')")
		}
	}
	for _, q := range qs {
		if err := raw.Exec(q).Error; err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	sqlDB, _ := raw.DB()
	_ = sqlDB.Close()
}

func checkMigratedVendorPricing(t *testing.T, round int, st *Store, wantDraft1 float64, edited uint) {
	t.Helper()
	cols := vendorPricingColumns(t, st.DB)
	for _, gone := range legacyVendorPricingColumns {
		if cols[gone] {
			t.Fatalf("round %d: column %s still exists", round, gone)
		}
	}
	for _, kept := range []string{"vendor_id", "mode", "markup", "draft_ratio", "updated_at"} {
		if !cols[kept] {
			t.Fatalf("round %d: column %s missing", round, kept)
		}
	}
	var idx []struct{ Name string }
	if err := st.DB.Raw("SELECT name FROM pragma_index_list('vendor_pricings')").Scan(&idx).Error; err != nil {
		t.Fatal(err)
	}
	for _, i := range idx {
		if i.Name == "idx_vendor_pricings_state" {
			t.Fatalf("round %d: state index survived", round)
		}
	}
	var rows []VendorPricing
	if err := st.DB.Order("vendor_id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 6 {
		t.Fatalf("round %d: rows %d", round, len(rows))
	}
	if rows[0].DraftRatio == nil || *rows[0].DraftRatio != wantDraft1 || rows[0].Mode != PricingFixed {
		t.Fatalf("round %d: to_write fixed row must keep its change as a draft: %+v", round, rows[0])
	}
	for _, i := range []int{1, 3, 4, 5} {
		if rows[i].VendorID != edited && rows[i].DraftRatio != nil {
			t.Fatalf("round %d: row %d must have no draft: %+v", round, rows[i].VendorID, rows[i])
		}
	}
	if rows[2].DraftRatio != nil || rows[2].Mode != PricingFollow || rows[2].Markup == nil || *rows[2].Markup != 1.5 {
		t.Fatalf("round %d: follow row keeps mode and markup, no draft: %+v", round, rows[2])
	}
}

func TestMigrateVendorPricingDraftsAndDropsOldColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nam.db")
	seedLegacyVendorPricing(t, path, KeyUSDCNYRate, "7.5", KeySiteRate, "6")
	for round := 1; round <= 2; round++ { // the second Open must be a harmless no-op
		st, err := Open(path)
		if err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		// 0.3 × 7.5 ÷ 6 = 0.375
		var edited uint
		if round == 2 {
			edited = 2
		}
		checkMigratedVendorPricing(t, round, st, 0.375, edited)
		if round == 1 { // an edit made after the migration survives the next start
			d := 0.9
			if err := st.DB.Model(&VendorPricing{}).Where("vendor_id = ?", 2).Update("draft_ratio", d).Error; err != nil {
				t.Fatal(err)
			}
		}
		if round == 2 {
			var r VendorPricing
			if err := st.DB.First(&r, 2).Error; err != nil || r.DraftRatio == nil || *r.DraftRatio != 0.9 {
				t.Fatalf("draft set after the migration was lost: %+v %v", r, err)
			}
			if err := st.DB.Model(&VendorPricing{}).Where("vendor_id = ?", 2).Update("draft_ratio", nil).Error; err != nil {
				t.Fatal(err)
			}
		}
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMigrateVendorPricingDefaultRates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nam.db")
	seedLegacyVendorPricing(t, path) // no settings table: 美元参考汇率 7.2, 站点汇率 7
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	checkMigratedVendorPricing(t, 1, st, 0.309, 0) // 0.3 × 7.2 ÷ 7 = 0.30857
}

func TestMigrateVendorPricingFreshDatabase(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "nam.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	cols := vendorPricingColumns(t, st.DB)
	if cols["sell_price"] || cols["state"] || !cols["draft_ratio"] {
		t.Fatalf("fresh columns: %v", cols)
	}
}
