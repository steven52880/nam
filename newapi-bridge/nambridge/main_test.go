package nambridge

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// TestMain boots NewAPI's real model layer on a throw-away SQLite file, exactly as
// main.InitResources does (InitDB -> InitOptionMap -> InitLogDB), so the adapters are
// exercised against NewAPI's own schema, migrations and option loading.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "nambridge-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := run(m, dir)
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func run(m *testing.M, dir string) int {
	gin.SetMode(gin.TestMode)
	common.IsMasterNode = true
	common.SQLitePath = filepath.ToSlash(filepath.Join(dir, "newapi.db")) + "?_busy_timeout=30000"
	_ = os.Setenv("SQL_DSN", "")
	_ = os.Setenv("LOG_SQL_DSN", "")
	ratio_setting.InitRatioSettings()
	if err := model.InitDB(); err != nil {
		fmt.Fprintln(os.Stderr, "InitDB:", err)
		return 1
	}
	model.InitOptionMap()
	if err := model.InitLogDB(); err != nil {
		fmt.Fprintln(os.Stderr, "InitLogDB:", err)
		return 1
	}
	_ = i18n.Init()
	code := m.Run()
	_ = model.CloseDB()
	return code
}
