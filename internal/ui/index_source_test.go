package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/hrthas-king/Samuel/internal/app"
	"github.com/hrthas-king/Samuel/internal/market"
)

func TestIndexSourceStaysInTooltipWhileBarOnlyShowsDelay(t *testing.T) {
	q := market.Quote{Symbol: "^FCHI", Price: 7986.68, ChangePercent: 1.26, ChangeAbs: 99.71, State: market.StateRegular,
		Reference: true, ProviderSymbol: "TVC:CAC40", QuoteSource: "TradingView 参考指数 (TVC:CAC40)", UpdateMode: "streaming"}
	for _, delay := range []int{0, 900} {
		q.DelaySeconds = delay
		s := app.Snapshot{MarketGroup: market.GroupEUIndices, Quotes: []market.View{{Symbol: q.Symbol, Quote: &q, Healthy: true}}}
		for _, text := range []string{quotePriceChangeText(q), QuoteSummary(s), TrayTooltip(s)} {
			if strings.Contains(text, "参考") {
				t.Errorf("reference label still follows the compact quote: %s", text)
			}
			if strings.Contains(text, "延迟约15m") != (delay > 0) {
				t.Errorf("delay label was lost or invented: %s", text)
			}
		}
		if text := DiagnosticsText("test", s); !strings.Contains(text, "参考") || !strings.Contains(text, "TVC:CAC40") {
			t.Errorf("source disclosure lost from diagnostics: %s", text)
		}
		if !q.Reference {
			t.Fatal("formatting changed the quote's reference metadata")
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
