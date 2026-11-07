package nambridge

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
)

func itoa(i int) string { return strconv.Itoa(i) }

// jsonSuccessNotFalse reports whether a NewAPI envelope does not carry "success": false.
// nam's own handlers may return bare JSON, so absence of the flag counts as success.
func jsonSuccessNotFalse(body string) bool {
	var env struct {
		Success *bool `json:"success"`
	}
	if err := common.Unmarshal([]byte(body), &env); err != nil {
		return false
	}
	return env.Success == nil || *env.Success
}
