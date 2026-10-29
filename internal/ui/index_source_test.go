package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/hrthas-king/Samuel/internal/app"
	"github.com/hrthas-king/Samuel/internal/market"
)

func TestIndexSourceLabelsReachTaskbarTrayAndDiagnostics(t *testing.T) {
	q := market.Quote{Symbol: "^FCHI", Price: 7986.68, ChangePercent: 1.26, ChangeAbs: 99.71, State: market.StateRegular,
		Reference: true, ProviderSymbol: "TVC:CAC40", QuoteSource: "TradingView 参考指数 (TVC:CAC40)", UpdateMode: "streaming"}
	for _, tag := range []string{"参考", "参考·延迟约15m"} {
		if strings.Contains(tag, "延迟") {
			q.DelaySeconds = 900
		}
		s := app.Snapshot{MarketGroup: market.GroupEUIndices, Quotes: []market.View{{Symbol: q.Symbol, Quote: &q, Healthy: true}}}
		for _, text := range []string{quotePriceChangeText(q), QuoteSummary(s), TrayTooltip(s), DiagnosticsText("test", s)} {
			if !strings.Contains(text, tag) {
				t.Errorf("provenance label %s missing: %s", tag, text)
			}
		}
	}
	q.Reference, q.DelaySeconds, q.ProviderSymbol = false, 0, ""
	q.TradeTime = time.Now().Add(-16 * time.Minute).Unix()
	if text := quotePriceChangeText(q); !strings.Contains(text, "延迟约16m") || strings.Contains(text, "参考") {
		t.Errorf("fallback cash quote mislabeled: %s", text)
	}
	q.State = market.StateClosed
	if text := quotePriceChangeText(q); strings.Contains(text, "延迟") {
		t.Errorf("closed market age presented as delay: %s", text)
	}
}
