package store

import (
	"fmt"
	"math"
	"strconv"

	"gorm.io/gorm"
)

// legacyVendorPricingColumns are the vendor_pricing columns M5.5 removed: the fixed price is
// read from NewAPI's GroupRatio, so nam keeps only the not-yet-written draft ratio.
// last_written is a stray column an older schema left in some databases (always NULL).
var legacyVendorPricingColumns = []string{
	"sell_price", "last_written", "last_written_price", "last_written_ratio", "last_written_at", "state", "pending_price", "reference_cost",
}

// migrateVendorPricing upgrades a pre-M5.5 vendor_pricings table (gorm's name for
// VendorPricing). AutoMigrate has already added draft_ratio. Fixed-mode rows that were 'to_write'
// with a sell price keep that change as draft_ratio (sell × 美元参考汇率 ÷ 站点汇率, 3 decimals,
// using the current settings); every other row gets NULL. Then the old columns (and indexes on
// them) are dropped. It only runs while an old column still exists, so restarts are harmless,
// and it runs in one transaction (SQLite DDL is transactional).
func migrateVendorPricing(db *gorm.DB) error {
	old, err := existingColumns(db, "vendor_pricings", legacyVendorPricingColumns)
	if err != nil || len(old) == 0 {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("UPDATE vendor_pricings SET draft_ratio = NULL").Error; err != nil {
			return err
		}
		if has(old, "state") && has(old, "sell_price") {
			usd := settingFloat(tx, KeyUSDCNYRate, DefaultUSDCNYRate)
			site := settingFloat(tx, KeySiteRate, 7)
			var rows []struct {
				VendorID  uint
				SellPrice float64
			}
			if err := tx.Raw("SELECT vendor_id, sell_price FROM vendor_pricings " +
				"WHERE mode = 'fixed' AND state = 'to_write' AND sell_price > 0").Scan(&rows).Error; err != nil {
				return err
			}
			for _, r := range rows {
				ratio := math.Round(r.SellPrice*usd/site*1000) / 1000
				if !(ratio > 0) || math.IsInf(ratio, 0) {
					continue
				}
				if err := tx.Exec("UPDATE vendor_pricings SET draft_ratio = ? WHERE vendor_id = ?", ratio, r.VendorID).Error; err != nil {
					return err
				}
			}
		}
		idx, err := indexesOn(tx, "vendor_pricings")
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
			if err := tx.Exec("ALTER TABLE vendor_pricings DROP COLUMN `" + c + "`").Error; err != nil {
				return fmt.Errorf("drop %s: %w", c, err)
			}
		}
		return nil
	})
}

// settingFloat reads a numeric setting straight from the settings table (the migration runs
// before the Store exists). Missing, unparsable or non-positive values give def.
func settingFloat(db *gorm.DB, key string, def float64) float64 {
	var vals []string
	if err := db.Raw("SELECT value FROM settings WHERE key = ?", key).Scan(&vals).Error; err != nil || len(vals) == 0 {
		return def
	}
	f, err := strconv.ParseFloat(vals[0], 64)
	if err != nil || !(f > 0) || math.IsInf(f, 0) {
		return def
	}
	return f
}

// existingColumns returns the columns of table that are in want.
func existingColumns(db *gorm.DB, table string, want []string) ([]string, error) {
	var cols []struct{ Name string }
	if err := db.Raw("SELECT name FROM pragma_table_info(?)", table).Scan(&cols).Error; err != nil {
		return nil, err
	}
	var out []string
	for _, c := range cols {
		if has(want, c.Name) {
			out = append(out, c.Name)
		}
	}
	return out, nil
}
