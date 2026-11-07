package nambridge

import (
	"context"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// optionStore implements ports.OptionStore on top of common.OptionMap / model.UpdateOption.
type optionStore struct{}

func (optionStore) Get(ctx context.Context, keys ...string) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := common.OptionMap[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

func checkJSON(v string) error {
	var x any
	return common.Unmarshal([]byte(v), &x)
}

func wrap(prefix string, f func(string) error) func(string) error {
	return func(v string) error {
		if err := f(v); err != nil {
			return fmt.Errorf("%s: %w", prefix, err)
		}
		return nil
	}
}

func statusCodes(v string) error {
	_, err := operation_setting.ParseHTTPStatusCodeRanges(v)
	return err
}

func console(kind string) func(string) error {
	return func(v string) error { return console_setting.ValidateConsoleSettings(v, kind) }
}

// validators mirrors the per-key checks in controller.UpdateOption. Keys that are plain
// JSON documents but have no dedicated validator only get a JSON validity check.
var validators = map[string]func(string) error{
	"GroupRatio":                         ratio_setting.CheckGroupRatio,
	"ModelPurpose":                       wrap("model purpose", ratio_setting.ValidateModelPurposeJSONString),
	"ImageRatio":                         wrap("image ratio", ratio_setting.UpdateImageRatioByJSONString),
	"AudioRatio":                         wrap("audio ratio", ratio_setting.UpdateAudioRatioByJSONString),
	"AudioCompletionRatio":               wrap("audio completion ratio", ratio_setting.UpdateAudioCompletionRatioByJSONString),
	"CreateCacheRatio":                   wrap("create cache ratio", ratio_setting.UpdateCreateCacheRatioByJSONString),
	"ModelRequestRateLimitGroup":         setting.CheckModelRequestRateLimitGroup,
	"AutomaticDisableStatusCodes":        statusCodes,
	"AutomaticRetryStatusCodes":          statusCodes,
	"console_setting.api_info":           console("ApiInfo"),
	"console_setting.announcements":      console("Announcements"),
	"console_setting.faq":                console("FAQ"),
	"console_setting.uptime_kuma_groups": console("UptimeKumaGroups"),
	"ModelRatio":                         checkJSON,
	"ModelPrice":                         checkJSON,
	"CompletionRatio":                    checkJSON,
	"CacheRatio":                         checkJSON,
	"GroupGroupRatio":                    checkJSON,
	"AutoGroups":                         checkJSON,
	"UserUsableGroups":                   checkJSON,
}

func (optionStore) Set(ctx context.Context, key, value string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if key == "" {
		return errors.New("option key is empty")
	}
	if check, ok := validators[key]; ok {
		if err := check(value); err != nil {
			return fmt.Errorf("invalid %s: %w", key, err)
		}
	}
	if err := model.UpdateOption(key, value); err != nil {
		return err
	}
	model.InvalidatePricingCache()
	// model.UpdateOption ignores database write errors, so read the row back.
	var row model.Option
	if err := model.DB.Where(&model.Option{Key: key}).First(&row).Error; err != nil {
		return fmt.Errorf("read back option %s: %w", key, err)
	}
	if row.Value != value {
		return fmt.Errorf("option %s was not persisted", key)
	}
	return nil
}
