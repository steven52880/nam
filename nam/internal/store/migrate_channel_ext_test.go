package store

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// legacySchema is channel_ext as it was before M5.5 (columns, defaults and the vendor index).
const legacySchema = "CREATE TABLE `channel_exts` (`channel_id` integer,`upstream_id` integer,`token_id` integer," +
	"`vendor` text,`discount` real NOT NULL DEFAULT 1,`discount_source` text DEFAULT 'auto',`cache_override` text," +
	"`model_discount` text,`model_mapping` text,`applied_hash` text,`applied_snap` text,`drift_fields` text,`updated_at` integer," +
	"PRIMARY KEY (`channel_id`))"

func channelExtColumns(t *testing.T, db *gorm.DB) map[string]bool {
	t.Helper()
	var cols []struct{ Name string }
	if err := db.Raw("SELECT name FROM pragma_table_info('channel_exts')").Scan(&cols).Error; err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, c := range cols {
		out[c.Name] = true
	}
	return out
}

func TestMigrateChannelExtKeepsManualDropsOldColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nam.db")
	raw, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		legacySchema,
		"CREATE INDEX `idx_channel_exts_vendor` ON `channel_exts`(`vendor`)",
		"CREATE INDEX `idx_channel_exts_upstream_id` ON `channel_exts`(`upstream_id`)",
		"INSERT INTO channel_exts VALUES (1, 7, 11, 'anthropic', 0.35, 'manual', '{\"read\":0.1}', '{\"m\":0.2}', '', 'h', 's', 'weight', 100)",
		"INSERT INTO channel_exts VALUES (2, 7, 12, 'gemini', 0.4, 'auto', '', '', '', 'h', 's', '', 100)",
		"INSERT INTO channel_exts VALUES (3, NULL, NULL, '', 1, NULL, '', '', '', '', '', '', 100)",
		"INSERT INTO channel_exts VALUES (4, 7, 13, 'x', 0, 'manual', '', '', '', '', '', '', 100)",
	} {
		if err := raw.Exec(q).Error; err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	sqlDB, _ := raw.DB()
	_ = sqlDB.Close()

	for round := 1; round <= 2; round++ { // the second Open must be a harmless no-op
		st, err := Open(path)
		if err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		cols := channelExtColumns(t, st.DB)
		for _, gone := range legacyChannelExtColumns {
			if cols[gone] {
				t.Fatalf("round %d: column %s still exists", round, gone)
			}
		}
		for _, kept := range []string{"channel_id", "upstream_id", "token_id", "manual_discount", "cache_override", "model_discount", "updated_at"} {
			if !cols[kept] {
				t.Fatalf("round %d: column %s missing", round, kept)
			}
		}
		var exts []ChannelExt
		if err := st.DB.Order("channel_id").Find(&exts).Error; err != nil {
			t.Fatal(err)
		}
		if len(exts) != 4 {
			t.Fatalf("round %d: rows %d", round, len(exts))
		}
		if exts[0].ManualDiscount == nil || *exts[0].ManualDiscount != 0.35 || exts[0].CacheOverride != `{"read":0.1}` ||
			exts[0].ModelDiscount != `{"m":0.2}` || exts[0].UpstreamID == nil || *exts[0].UpstreamID != 7 || exts[0].TokenID == nil || *exts[0].TokenID != 11 {
			t.Fatalf("round %d: manual row: %+v", round, exts[0])
		}
		if (round == 1 && exts[1].ManualDiscount != nil) || exts[2].ManualDiscount != nil || exts[3].ManualDiscount != nil {
			t.Fatalf("round %d: only 'manual' rows with discount > 0 keep a value: %+v", round, exts)
		}
		if round == 2 && (exts[1].ManualDiscount == nil || *exts[1].ManualDiscount != 0.5) {
			t.Fatalf("round 2: value set after the migration was touched: %+v", exts[1])
		}
		var idx []struct{ Name string }
		if err := st.DB.Raw("SELECT name FROM pragma_index_list('channel_exts')").Scan(&idx).Error; err != nil {
			t.Fatal(err)
		}
		for _, i := range idx {
			if i.Name == "idx_channel_exts_vendor" {
				t.Fatalf("round %d: vendor index survived", round)
			}
		}
		// a manual value set after the migration survives the next start
		if round == 1 {
			if err := st.DB.Model(&ChannelExt{}).Where("channel_id = ?", 2).Update("manual_discount", 0.5).Error; err != nil {
				t.Fatal(err)
			}
		}
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
	}
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	var e ChannelExt
	if err := st.DB.First(&e, 2).Error; err != nil || e.ManualDiscount == nil || *e.ManualDiscount != 0.5 {
		t.Fatalf("manual value lost on restart: %+v %v", e, err)
	}
}

func TestMigrateChannelExtFreshDatabase(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "nam.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if cols := channelExtColumns(t, st.DB); cols["vendor"] || cols["discount"] || !cols["manual_discount"] {
		t.Fatalf("fresh columns: %v", cols)
	}
}
