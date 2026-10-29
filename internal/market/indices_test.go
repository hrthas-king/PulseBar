package market

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

const referenceIndexFixture = `{"data":[
 {"s":"TVC:CAC40","d":[7986.68,1.2642370897822615,99.71,"index","EUR","streaming","CAC 40","market"]},
 {"s":"TVC:DEU40","d":[26073.41,1.437050921587008,369.38,"index","EUR","streaming","GERMAN STOCK INDEX (DAX)","market"]},
 {"s":"TVC:NI225","d":[64659.25,1.1027931495104977,705.28,"index","JPY","streaming","Japan 225 Index","out_of_session"]},
 {"s":"KRX:KOSPI","d":[5937.66,-1.2437546154306696,-74.78,"index",null,"streaming","KOSPI Composite Index","out_of_session"]}
]}`

func TestReferenceIndicesKeepSourceIdentitySessionAndUnknownTickTime(t *testing.T) {
	now := time.Date(2026, 10, 29, 16, 0, 0, 0, time.UTC)
	symbols := []string{"^FCHI", "^GDAXI", "^N225", "^KS11"}
	batch, err := parseIndexQuotes(symbols, []byte(referenceIndexFixture), 50, now)
	if err != nil || len(batch) != 4 {
		t.Fatalf("real index response failed: %v %#v", err, batch)
	}
	for _, symbol := range symbols {
		q := batch[symbol].Quote
		if q == nil || q.Symbol != symbol || q.ProviderSymbol != indexSpecs[symbol].ticker || q.QuoteType != "INDEX" || q.Currency != indexSpecs[symbol].currency {
			t.Fatalf("wrong index identity: %s %#v", symbol, q)
		}
		if q.TradeTime != 0 || q.Reference != (symbol != "^KS11") || q.UpdateMode != "streaming" {
			t.Errorf("fabricated tick time or lost source distinction: %#v", q)
		}
		wantState := StateRegular
		if symbol == "^N225" || symbol == "^KS11" {
			wantState = StateClosed
		}
		if q.State != wantState {
			t.Errorf("wrong session for %s: %s", symbol, q.State)
		}
		tip := (View{Symbol: symbol, Quote: q, Healthy: true}).Tooltip()
		if !strings.Contains(tip, q.ProviderSymbol) || !strings.Contains(tip, "源未提供逐笔时间") || strings.Contains(tip, "实时数据正常") {
			t.Errorf("source or timing disclosure missing: %s", tip)
		}
	}
}

func TestIndexParserRejectsWrongDataAndIsolatesSingleSideFailures(t *testing.T) {
	for _, tc := range []struct{ name, from, to string }{
		{"wrong_currency", `"EUR"`, `"USD"`},
		{"wrong_type", `"index"`, `"cfd"`},
		{"missing_price", `7986.68`, `null`},
		{"string_price", `7986.68`, `"7986.68"`},
		{"nonfinite_price", `7986.68`, `1e999`},
		{"inconsistent_change", `99.71`, `900`},
		{"unknown_mode", `"streaming"`, `"unknown"`},
		{"unknown_session", `"market"`, `"unknown"`},
		{"wrong_ticker", `"TVC:CAC40"`, `"OANDA:FR40EUR"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.Replace(referenceIndexFixture, tc.from, tc.to, 1)
			batch, err := parseIndexQuotes([]string{"^FCHI", "^GDAXI"}, []byte(body), 20, time.Now())
			if err != nil || batch["^GDAXI"].Quote == nil || batch["^FCHI"].Quote != nil || batch["^FCHI"].Err == "" {
				t.Fatalf("single-side failure was not isolated: %v %#v", err, batch)
			}
		})
	}
	t.Run("duplicate", func(t *testing.T) {
		row := `{"s":"TVC:CAC40","d":[7986.68,1.2642370897822615,99.71,"index","EUR","streaming","CAC 40","market"]}`
		batch, err := parseIndexQuotes([]string{"^FCHI"}, []byte(`{"data":[`+row+`,`+row+`]}`), 20, time.Now())
		if err == nil || batch["^FCHI"].Quote != nil {
			t.Fatal("duplicate source rows accepted")
		}
	})
}

func TestIndexSourcePriorityAndFallbackArePerSymbol(t *testing.T) {
	good, _ := parseIndexQuotes([]string{"^FCHI", "^GDAXI"}, []byte(referenceIndexFixture), 20, time.Now())
	primary := func(context.Context, []string) (map[string]SymbolResult, error) { return good, nil }
	fallback := func(context.Context, []string) (map[string]SymbolResult, error) {
		t.Fatal("fallback called despite valid primary data")
		return nil, nil
	}
	batch, err := fetchIndexQuotes(context.Background(), []string{"^FCHI", "^GDAXI"}, primary, fallback)
	if err != nil || !batch["^FCHI"].Quote.Reference || !batch["^GDAXI"].Quote.Reference {
		t.Fatalf("primary reference source not preferred: %v %#v", err, batch)
	}
	primary = func(context.Context, []string) (map[string]SymbolResult, error) {
		return map[string]SymbolResult{"^FCHI": good["^FCHI"], "^GDAXI": {Err: "unavailable"}}, nil
	}
	fallback = func(_ context.Context, symbols []string) (map[string]SymbolResult, error) {
		if !reflect.DeepEqual(symbols, []string{"^GDAXI"}) {
			t.Errorf("fallback unnecessarily replaced healthy primary: %v", symbols)
		}
		return nil, errors.New("fallback offline")
	}
	batch, err = fetchIndexQuotes(context.Background(), []string{"^FCHI", "^GDAXI"}, primary, fallback)
	if err != nil || batch["^FCHI"].Quote == nil || batch["^GDAXI"].Quote != nil || batch["^GDAXI"].Err == "" {
		t.Fatalf("partial fallback failure discarded valid side: %v %#v", err, batch)
	}
	fallback = func(_ context.Context, symbols []string) (map[string]SymbolResult, error) {
		q := *good["^GDAXI"].Quote
		q.Reference, q.DelaySeconds, q.ProviderSymbol = false, 900, ""
		return map[string]SymbolResult{"^GDAXI": {Quote: &q}}, nil
	}
	batch, err = fetchIndexQuotes(context.Background(), []string{"^FCHI", "^GDAXI"}, primary, fallback)
	if err != nil || batch["^GDAXI"].Quote.Reference || batch["^GDAXI"].Quote.DelaySeconds != 900 {
		t.Fatalf("fallback provenance lost: %v %#v", err, batch)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fetchIndexQuotes(ctx, []string{"^FCHI"}, primary, fallback); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request not stopped: %v", err)
	}
}

func TestIndexDelayLabelsDoNotConfuseClosedMarketWithDelay(t *testing.T) {
	now := time.Date(2026, 10, 29, 16, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		q    Quote
		want string
	}{
		{"streaming_reference", Quote{Symbol: "^FCHI", Reference: true}, "参考"},
		{"delayed_reference", Quote{Symbol: "^FCHI", Reference: true, DelaySeconds: 900}, "参考·延迟约15m"},
		{"old_cash", Quote{Symbol: "^FCHI", State: StateRegular, TradeTime: now.Add(-16 * time.Minute).Unix()}, "延迟约16m"},
		{"closed_cash", Quote{Symbol: "^N225", State: StateClosed, TradeTime: now.Add(-9 * time.Hour).Unix()}, ""},
		{"other_stock_unchanged", Quote{Symbol: "AAPL", State: StateRegular, TradeTime: now.Add(-16 * time.Minute).Unix()}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.q.FeedTag(now); got != tc.want {
				t.Errorf("label %q, want %q", got, tc.want)
			}
		})
	}
	body := strings.Replace(referenceIndexFixture, `"streaming"`, `"delayed_streaming_900"`, 1)
	batch, err := parseIndexQuotes([]string{"^FCHI"}, []byte(body), 20, now)
	if err != nil || batch["^FCHI"].Quote.DelaySeconds != 900 {
		t.Fatalf("provider delay was ignored: %v %#v", err, batch)
	}
}

func TestIndexRequestUsesOnlyRequestedReferenceSymbols(t *testing.T) {
	body, err := indexRequest([]string{"^FCHI", "^GDAXI"})
	if err != nil || !json.Valid(body) || !strings.Contains(string(body), `"TVC:CAC40","TVC:DEU40"`) || strings.Contains(string(body), `"time"`) {
		t.Fatalf("wrong request or bar timestamp requested: %s %v", body, err)
	}
	if _, err := indexRequest([]string{"AAPL"}); err == nil {
		t.Fatal("unrelated instrument accepted as index")
	}
}

func TestIndexProviderPreAndPostSessionsRemainValid(t *testing.T) {
	for _, tc := range []struct {
		session string
		state   SessionState
	}{
		{"market", StateRegular},
		{"pre_market", StatePreMarket},
		{"post_market", StateClosed},
		{"out_of_session", StateClosed},
	} {
		t.Run(tc.session, func(t *testing.T) {
			body := strings.Replace(referenceIndexFixture, `"market"`, `"`+tc.session+`"`, 1)
			batch, err := parseIndexQuotes([]string{"^FCHI"}, []byte(body), 20, time.Now())
			if err != nil || batch["^FCHI"].Quote == nil || batch["^FCHI"].Quote.State != tc.state {
				t.Fatalf("valid provider session rejected or misclassified: %s %v %#v", tc.session, err, batch)
			}
		})
	}
}
