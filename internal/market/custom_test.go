package market

import (
	"context"
	"errors"
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

func TestCustomETFPairsAndPrecision(t *testing.T) {
	symbols := []string{"513500.SS", "159632.SZ"}
	body := customRow("sh", "513500", "标普500ETF", "ETF") + customRow("sz", "159632", "纳指ETF", "ETF")
	batch, err := ParseTencentQuoteBatch(symbols, []byte(body), 10, time.Date(2026, 10, 28, 2, 0, 1, 0, time.UTC))
	if err != nil || batch[symbols[0]].Quote == nil || batch[symbols[1]].Quote == nil {
		t.Fatalf("ETF pair rejected: %+v, %v", batch, err)
	}
	for _, s := range symbols {
		if batch[s].Quote.QuoteType != "ETF" || batch[s].Quote.Currency != "CNY" || PriceDecimals(s) != 3 {
			t.Errorf("ETF type/precision: %s, %+v, decimals=%d", s, batch[s], PriceDecimals(s))
		}
	}
}

func TestCustomPairRoutesMixedMarketsAndKeepsPartialQuote(t *testing.T) {
	var calls [][]string
	batch, err := fetchCustomQuoteBatch(context.Background(), []string{"AAPL", "513500.SS"}, func(_ context.Context, symbols []string) (map[string]SymbolResult, error) {
		calls = append(calls, append([]string(nil), symbols...))
		if len(symbols) != 1 {
			t.Fatalf("mixed provider request: %v", symbols)
		}
		if symbols[0] == "AAPL" {
			return nil, errors.New("US provider unavailable")
		}
		return map[string]SymbolResult{symbols[0]: {Quote: &Quote{Symbol: symbols[0]}}}, nil
	})
	if err != nil || len(calls) != 2 || batch["513500.SS"].Quote == nil || batch["AAPL"].Err != "US provider unavailable" {
		t.Fatalf("routing/partial failure: calls=%v batch=%+v err=%v", calls, batch, err)
	}
}

func TestCustomYahooUSQuotesAndIsolation(t *testing.T) {
	body := []byte(`{"quoteResponse":{"result":[
		{"symbol":"AAPL","currency":"USD","quoteType":"EQUITY","shortName":"Apple Inc.","regularMarketPrice":101,"regularMarketChange":1,"regularMarketChangePercent":1,"regularMarketTime":1793196000,"marketState":"REGULAR","gmtOffSetMilliseconds":-14400000},
		{"symbol":"VOO","currency":"USD","quoteType":"ETF","regularMarketPrice":101,"regularMarketChange":1,"regularMarketChangePercent":1}
	]}}`)
	batch, err := ParseYahooQuoteBatchForSymbols([]string{"AAPL", "MSFT"}, body, 10, time.Date(2026, 10, 28, 14, 0, 0, 0, time.UTC))
	q := batch["AAPL"].Quote
	if err != nil || q == nil || q.DisplayName() != "Apple Inc." || batch["MSFT"].Err == "" {
		t.Fatalf("custom US quote: %+v, %v", batch, err)
	}
	if _, ok := batch["VOO"]; ok {
		t.Fatal("unsolicited quote escaped the selected pair")
	}
}

func TestCustomUSChartAndAssetValidation(t *testing.T) {
	body := []byte(`{"chart":{"result":[{"meta":{"symbol":"AAPL","currency":"USD","instrumentType":"EQUITY","shortName":"Apple Inc.","regularMarketPrice":101,"chartPreviousClose":100,"regularMarketTime":1793196000,"gmtoffset":-14400}}]}}`)
	q, err := ParseYahooChart("AAPL", body, 10, time.Date(2026, 10, 28, 14, 0, 0, 0, time.UTC))
	if err != nil || q.DisplayName() != "Apple Inc." || q.ChangeAbs != 1 || q.ChangePercent != 1 {
		t.Fatalf("custom chart quote: %+v, %v", q, err)
	}
	if _, err := ParseYahooChart("MSFT", body, 10, time.Now()); err == nil {
		t.Fatal("chart returned a different stock")
	}
	for _, invalid := range []Quote{
		{Symbol: "AAPL", Currency: "CNY", QuoteType: "EQUITY"},
		{Symbol: "AAPL", Currency: "USD", QuoteType: "INDEX"},
	} {
		invalid.Price, invalid.ChangeAbs, invalid.ChangePercent, invalid.UpdateMode = 101, 1, 1, "regular_realtime"
		if err := ValidateQuote(invalid); err == nil {
			t.Fatalf("invalid US asset accepted: %+v", invalid)
		}
	}
}
