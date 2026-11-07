package nambridge

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

// validateChannel mirrors controller.validateChannel (unexported upstream), so that
// channels written by nam pass exactly the checks the NewAPI UI applies.
// Keep in sync with controller/channel.go when upgrading NewAPI.
func validateChannel(channel *model.Channel, isAdd bool) error {
	if err := channel.ValidateSettings(); err != nil {
		return fmt.Errorf("channel setting format error: %w", err)
	}
	if channel.Type == constant.ChannelTypePassThrough && strings.TrimSpace(channel.GetBaseURL()) == "" {
		return fmt.Errorf("pass-through channel requires a base url")
	}
	if channel.ManualUpstreamRateMultiplier != nil && *channel.ManualUpstreamRateMultiplier < 0 {
		return fmt.Errorf("manual upstream rate multiplier must not be negative")
	}
	if channel.GetResponseTimeout() < 0 {
		return fmt.Errorf("channel timeout must not be negative")
	}
	if channel.Type == constant.ChannelTypeAdvancedPassThrough {
		s := channel.GetOtherSettings()
		if strings.TrimSpace(s.AdvancedOpenAIBaseURL) == "" || strings.TrimSpace(s.AdvancedAnthropicBaseURL) == "" {
			return fmt.Errorf("advanced pass-through channel requires both upstream urls")
		}
	}
	if isAdd {
		if channel.Key == "" {
			return fmt.Errorf("channel key must not be empty")
		}
		for _, m := range channel.GetModels() {
			if len(m) > 255 {
				return fmt.Errorf("model name too long: %s", m)
			}
		}
	}
	if channel.Type == constant.ChannelTypeVertexAi {
		if channel.Other == "" {
			return fmt.Errorf("vertex region must not be empty")
		}
		regionMap, err := common.StrToMap(channel.Other)
		if err != nil || regionMap["default"] == nil {
			return fmt.Errorf("vertex region must be JSON with a default field")
		}
	}
	if channel.Type == constant.ChannelTypeCodex {
		trimmed := strings.TrimSpace(channel.Key)
		if isAdd || trimmed != "" {
			var keyMap map[string]any
			if !strings.HasPrefix(trimmed, "{") || common.Unmarshal([]byte(trimmed), &keyMap) != nil {
				return fmt.Errorf("codex key must be a JSON object")
			}
			for _, f := range []string{"access_token", "account_id"} {
				if v, ok := keyMap[f]; !ok || v == nil || strings.TrimSpace(fmt.Sprintf("%v", v)) == "" {
					return fmt.Errorf("codex key JSON must include %s", f)
				}
			}
		}
	}
	return nil
}
