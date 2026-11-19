package upstream

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/steven52880/nam/internal/store"
)

const (
	partMsgLimit     = 300
	msgUnsupported   = "该上游类型不支持"
	msgNoModelPrices = "上游没有提供逐模型价格"
	// partsBase is sync_parts normalized to a JSON object: NULL, '' and malformed → {}.
	partsBase = "CASE WHEN json_valid(sync_parts) THEN CASE WHEN json_type(sync_parts) = 'object' THEN sync_parts ELSE '{}' END ELSE '{}' END"
)

// namedPart is one entry of Upstream.SyncParts.
type namedPart struct {
	name string
	part store.SyncPart
}

func truncateRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// outcome classifies the result of one sync step: nil → ok, ErrUnsupported → unsupported,
// anything else → fail. count is only kept for ok.
func outcome(credType string, err error, count *int) store.SyncPart {
	p := store.SyncPart{At: time.Now().Unix()}
	switch {
	case err == nil:
		p.State, p.Count = store.SyncOK, count
	case errors.Is(err, ErrUnsupported):
		p.State, p.Message = store.SyncUnsupported, msgUnsupported
	default:
		p.State, p.Message = store.SyncFail, truncateRunes(AuthErrorText(credType, err), partMsgLimit)
	}
	return p
}

func intp(n int) *int { return &n }

// groupParts builds the groups and model_prices entries, which are always updated together.
func groupParts(credType string, err error, groups, prices int) []namedPart {
	g := outcome(credType, err, intp(groups))
	m := g
	if g.State == store.SyncOK {
		m.Count = intp(prices)
		if prices == 0 {
			m.State, m.Message = store.SyncEmpty, msgNoModelPrices
		}
	}
	return []namedPart{{store.SyncPartGroups, g}, {store.SyncPartModelPrices, m}}
}

// partsExpr returns a SQL expression that sets the given parts inside the sync_parts JSON
// column atomically (no read-modify-write in Go). A NULL, empty or malformed column is
// treated as {}.
func partsExpr(parts []namedPart) (string, []any) {
	var b strings.Builder
	b.WriteString("json_set(" + partsBase)
	args := make([]any, 0, len(parts)*2)
	for _, p := range parts {
		raw, _ := json.Marshal(p.part)
		b.WriteString(", ?, json(?)")
		args = append(args, "$."+p.name, string(raw))
	}
	b.WriteString(")")
	return b.String(), args
}
