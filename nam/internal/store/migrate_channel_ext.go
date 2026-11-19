package store

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// legacyChannelExtColumns are the channel_ext columns M5.5 removed: the vendor now follows
// the NewAPI channel groups, the discount is computed from cached upstream prices, and the
// drift snapshot (applied_*, drift_fields) and model_mapping are gone.
var legacyChannelExtColumns = []string{
	"vendor", "discount", "discount_source", "model_mapping", "applied_hash", "applied_snap", "drift_fields",
}

// migrateChannelExt upgrades a pre-M5.5 channel_exts table (gorm's name for ChannelExt). AutoMigrate has already added
// manual_discount. Rows whose discount_source was 'manual' keep their discount as the manual
// value, everything else becomes NULL (computed). Then the old columns (and the index on
// vendor) are dropped. It only runs while an old column still exists, so restarts are harmless,
// and it runs in one transaction (SQLite DDL is transactional).
func migrateChannelExt(db *gorm.DB) error {
	old, err := existingLegacyColumns(db)
	if err != nil || len(old) == 0 {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if has(old, "discount_source") && has(old, "discount") {
			if err := tx.Exec("UPDATE channel_exts SET manual_discount = discount WHERE discount_source = 'manual' AND discount > 0").Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE channel_exts SET manual_discount = NULL WHERE discount_source IS NOT 'manual'").Error; err != nil {
				return err
			}
		}
		// SQLite refuses to drop an indexed column: drop every index that covers an old column.
		idx, err := indexesOn(tx, "channel_exts")
		if err != nil {
			return err
		}
		for name, cols := range idx {
			for _, c := range cols {
				if has(old, c) {
					if err := tx.Exec("DROP INDEX IF EXISTS `" + name + "`").Error; err != nil {
						return err
					}
					break
				}
			}
		}
		for _, c := range old {
			if err := tx.Exec("ALTER TABLE channel_exts DROP COLUMN `" + c + "`").Error; err != nil {
				return fmt.Errorf("drop %s: %w", c, err)
			}
		}
		return nil
	})
}

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func existingLegacyColumns(db *gorm.DB) ([]string, error) {
	var cols []struct{ Name string }
	if err := db.Raw("SELECT name FROM pragma_table_info('channel_exts')").Scan(&cols).Error; err != nil {
		return nil, err
	}
	var out []string
	for _, c := range cols {
		if has(legacyChannelExtColumns, c.Name) {
			out = append(out, c.Name)
		}
	}
	return out, nil
}

// indexesOn returns explicit (CREATE INDEX) index names of a table with their columns.
func indexesOn(db *gorm.DB, table string) (map[string][]string, error) {
	var idx []struct {
		Name   string
		Origin string
	}
	if err := db.Raw("SELECT name, origin FROM pragma_index_list(?)", table).Scan(&idx).Error; err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, i := range idx {
		if strings.ToLower(i.Origin) != "c" {
			continue
		}
		var cols []struct{ Name string }
		if err := db.Raw("SELECT name FROM pragma_index_info(?)", i.Name).Scan(&cols).Error; err != nil {
			return nil, err
		}
		for _, c := range cols {
			out[i.Name] = append(out[i.Name], c.Name)
		}
	}
	return out, nil
}
