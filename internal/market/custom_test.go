package market

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hrthas-king/PulseBar/internal/watchlist"
)

func customRow(prefix, code, name, kind string) string {
	f := make([]string, 62)
	f[1] = name
	f[2] = code
	f[3] = "100.50"
	f[4] = "100.00"
	f[30] = "20261028100000"
	f[61] = kind
	return fmt.Sprintf("v_%s=\"%s\";", prefix+code, strings.Join(f, "~"))
}
func TestDynamicGroupRegistryAndQuoteIsolation(t *testing.T) {
	defer SetCustomPairs(nil)
	defer SetActiveGroup(DefaultGroup)
	p, _ := watchlist.NewPair("600519", "000858")
	if err := SetCustomPairs([]watchlist.Pair{p}); err != nil {
		t.Fatal(err)
	}
	if !ValidGroup(p.ID()) {
		t.Fatal("custom group unavailable")
	}
	g := SetActiveGroup(p.ID())
	g.Symbols[0] = "corrupted"
	if ActiveSymbols()[0] != "600519.SS" {
		t.Fatal("caller mutated active group")
	}
	query, ok := TencentQueryForSymbols(ActiveSymbols())
	if !ok || query != "sh600519,sz000858" {
		t.Fatalf("%s %v", query, ok)
	}
	body := customRow("sh", "600519", "贵州茅台", "GP-A") + customRow("sz", "000858", "五粮液", "GP-A") + customRow("sz", "300750", "宁德时代", "GP-A-CYB")
	batch, err := ParseTencentQuoteBatch(ActiveSymbols(), []byte(body), 35, time.Date(2026, 10, 28, 2, 0, 1, 0, time.UTC))
	if err != nil || len(batch) != 2 || batch["600519.SS"].Quote == nil || batch["000858.SZ"].Quote == nil {
		t.Fatalf("%+v %v", batch, err)
	}
	q := batch["600519.SS"].Quote
	if q.Alias != "贵州茅台" || q.QuoteType != "EQUITY" || q.State != StateRegular || q.Currency != "CNY" {
		t.Fatalf("%+v", q)
	}
	if got := (View{Symbol: q.Symbol, Alias: q.Symbol, Quote: q, Healthy: true}).DisplayName(); got != "贵州茅台" {
		t.Fatalf("name: %s", got)
	}
	SetCustomPairs(nil)
	if ActiveGroup().ID != DefaultGroup || ValidGroup(p.ID()) {
		t.Fatal("removed current group remained selected")
	}
}
func TestCustomStockValidationRejectsFundAndMissingQuotes(t *testing.T) {
	symbols := []string{"600519.SS", "920002.BJ"}
	now := time.Date(2026, 10, 28, 2, 0, 1, 0, time.UTC)
	body := customRow("sh", "600519", "基金", "ETF") + customRow("bj", "920002", "万达轴承", "GP")
	batch, err := ParseTencentQuoteBatch(symbols, []byte(body), 10, now)
	if err != nil || batch[symbols[0]].Err == "" || batch[symbols[1]].Quote == nil {
		t.Fatalf("%+v %v", batch, err)
	}
	batch, err = ParseTencentQuoteBatch(symbols, []byte(`v_pv_none_match="1";`), 10, now)
	if err == nil || batch[symbols[0]].Err == "" || batch[symbols[1]].Err == "" {
		t.Fatal("nonexistent codes were accepted")
	}
}
