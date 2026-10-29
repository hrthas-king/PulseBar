package market

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestGlobalIndexDelayIsNotReportedAsRealtime(t *testing.T) {
	now := time.Date(2026, 10, 29, 15, 56, 41, 0, time.UTC)
	body := []byte(`v_gzFCHI="FCHI~法国CAC40指数~2026-10-29 23:39:45~7984.51~97.55~1.24~EU~";`)
	batch, err := ParseTencentQuoteBatch([]string{"^FCHI"}, body, 20, now)
	if err != nil || batch["^FCHI"].Quote == nil {
		t.Fatalf("valid delayed cash index unavailable: %v, %#v", err, batch)
	}
	q := batch["^FCHI"].Quote
	if age := now.Unix() - q.TradeTime; age != 1016 {
		t.Fatalf("provider Beijing timestamp parsed incorrectly: age=%d", age)
	}
	if strings.Contains(strings.ToLower(q.UpdateMode+q.QuoteSource), "real") || strings.Contains(q.SessionText(), "实时") {
		t.Errorf("16-minute-old cash index claims real-time: %#v", q)
	}
	tip := (View{Symbol: q.Symbol, Quote: q, Healthy: true}).Tooltip()
	if strings.Contains(tip, "实时数据正常") || !strings.Contains(tip, "延迟") {
		t.Errorf("delayed index presented as live: %s", tip)
	}
}

func TestGlobalIndexRejectsUnusableTimestampAndIdentity(t *testing.T) {
	now := time.Date(2026, 10, 29, 15, 56, 41, 0, time.UTC)
	for _, tc := range []struct{ name, code, timestamp string }{
		{"missing_time", "FCHI", ""},
		{"malformed_time", "FCHI", "not-a-time"},
		{"future_time", "FCHI", "2026-10-30 23:39:45"},
		{"wrong_index", "KS11", "2026-10-29 23:39:45"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`v_gzFCHI="%s~index~%s~7984.51~97.55~1.24~EU~";`, tc.code, tc.timestamp))
			batch, _ := ParseTencentQuoteBatch([]string{"^FCHI"}, body, 20, now)
			if batch["^FCHI"].Quote != nil || batch["^FCHI"].Err == "" {
				t.Fatalf("unverifiable quote accepted: %#v", batch["^FCHI"])
			}
		})
	}
}
