package market

import (
	"math"
	"testing"
	"time"
)

func TestParseYahooQuoteBatchAllSessions(t *testing.T) {
	SetActiveGroup(GroupVOOQQQ)
	batchJSON := []byte(`{"quoteResponse":{"result":[` +
		`{"symbol":"VOO","currency":"USD","quoteType":"ETF","longName":"Vanguard S&P 500 ETF",` +
		`"marketState":"OVERNIGHT","quoteSourceName":"BOATS Real Time Price","gmtOffSetMilliseconds":-14400000,` +
		`"regularMarketPrice":698.58,"regularMarketChange":-3.72,"regularMarketChangePercent":-0.53,"regularMarketTime":1792785600,` +
		`"postMarketPrice":699.53,"postMarketChange":0.95,"postMarketChangePercent":0.14,"postMarketTime":1792799914,` +
		`"overnightMarketPrice":699.05,"overnightMarketChange":0.47,"overnightMarketChangePercent":0.06728,"overnightMarketTime":1792997930},` +
		`{"symbol":"QQQ","currency":"USD","quoteType":"ETF","longName":"Invesco QQQ Trust",` +
		`"marketState":"OVERNIGHT","quoteSourceName":"BOATS Real Time Price","gmtOffSetMilliseconds":-14400000,` +
		`"regularMarketPrice":704.40,"regularMarketChange":-0.07,"regularMarketChangePercent":-0.01,"regularMarketTime":1792785600,` +
		`"postMarketPrice":704.88,"postMarketChange":0.48,"postMarketChangePercent":0.07,"postMarketTime":1792799985,` +
		`"overnightMarketPrice":703.55,"overnightMarketChange":-0.85,"overnightMarketChangePercent":-0.12067,"overnightMarketTime":1792997935}` +
		`]}}`)

	overnightNow := time.Date(2026, 10, 26, 6, 59, 0, 0, time.UTC)
	batch, err := ParseYahooQuoteBatch(batchJSON, 185.0, overnightNow)
	if err != nil {
		t.Fatalf("unexpected ParseYahooQuoteBatch error: %v", err)
	}
	voo := batch["VOO"].Quote
	qqq := batch["QQQ"].Quote
	if voo == nil || qqq == nil {
		t.Fatalf("expected both VOO and QQQ quotes: %+v", batch)
	}
	if voo.Price != 699.05 || math.Abs(voo.ChangePercent-0.06728) > 1e-4 || voo.State != StateOvernight || voo.Alias != "VOO" || voo.QuoteSource != "BOATS Real Time Price" {
		t.Fatalf("unexpected VOO overnight quote: %+v", voo)
	}
	if qqq.Price != 703.55 || math.Abs(qqq.ChangePercent - -0.12067) > 1e-4 || qqq.State != StateOvernight || qqq.Alias != "QQQ" {
		t.Fatalf("unexpected QQQ overnight quote: %+v", qqq)
	}
}

func TestMultiMarketGroupsCurrenciesAndIndices(t *testing.T) {
	defer SetActiveGroup(DefaultGroup)

	// 1. A股指数 (CNY / INDEX)
	SetActiveGroup(GroupCNIndices)
	cnJSON := []byte(`{"quoteResponse":{"result":[` +
		`{"symbol":"000001.SS","currency":"CNY","quoteType":"INDEX","shortName":"SSE Composite Index","marketState":"REGULAR","gmtOffSetMilliseconds":28800000,"regularMarketPrice":3697.52,"regularMarketChange":7.04,"regularMarketChangePercent":0.1908,"regularMarketTime":1792998024},` +
		`{"symbol":"399006.SZ","currency":"CNY","quoteType":"INDEX","shortName":"ChiNext","marketState":"REGULAR","gmtOffSetMilliseconds":28800000,"regularMarketPrice":2638.23,"regularMarketChange":-45.87,"regularMarketChangePercent":-1.7089,"regularMarketTime":1793003353}` +
		`]}}`)
	cnBatch, err := ParseYahooQuoteBatch(cnJSON, 95.0, time.Now())
	if err != nil {
		t.Fatalf("CN indices parse failed: %v", err)
	}
	if cnBatch["000001.SS"].Quote == nil || cnBatch["000001.SS"].Quote.Alias != "上证" || cnBatch["000001.SS"].Quote.State != StateRegular {
		t.Fatalf("unexpected SSE quote: %+v", cnBatch["000001.SS"])
	}
	if cnBatch["399006.SZ"].Quote == nil || cnBatch["399006.SZ"].Quote.Alias != "创指" || cnBatch["399006.SZ"].Quote.Currency != "CNY" {
		t.Fatalf("unexpected ChiNext quote: %+v", cnBatch["399006.SZ"])
	}

	// 2. 港股指数 (^HSI + HSTECH.HK 归一化为 ^HSTECH)：
	// 验证凌晨 01:20 HKT (17:20 UTC) Yahoo 返回 PREPRE 时应识别为休市 (StateClosed)，
	// 而在早间 09:15 HKT (01:15 UTC) 集合竞价窗口时才识别为盘前 (StatePreMarket)。
	SetActiveGroup(GroupHKIndices)
	hkJSON := []byte(`{"quoteResponse":{"result":[` +
		`{"symbol":"^HSI","currency":"HKD","quoteType":"INDEX","shortName":"HANG SENG INDEX","marketState":"PREPRE","gmtOffSetMilliseconds":28800000,"regularMarketPrice":23211.81,"regularMarketChange":118.44,"regularMarketChangePercent":0.5129,"regularMarketTime":1793002108},` +
		`{"symbol":"HSTECH.HK","quoteType":"INDEX","shortName":"Hang Seng TECH Index","marketState":"PREPRE","gmtOffSetMilliseconds":28800000,"regularMarketPrice":3857.53,"regularMarketChange":-14.13,"regularMarketChangePercent":-0.365,"regularMarketTime":1793002108}` +
		`]}}`)
	midnightHK := time.Date(2026, 10, 26, 17, 20, 0, 0, time.UTC) // 2026-10-27 01:20 HKT (周二凌晨休市)
	hkClosedBatch, err := ParseYahooQuoteBatch(hkJSON, 110.0, midnightHK)
	if err != nil {
		t.Fatalf("HK indices midnight parse failed: %v", err)
	}
	if hkClosedBatch["^HSI"].Quote == nil || hkClosedBatch["^HSI"].Quote.Alias != "恒指" || hkClosedBatch["^HSI"].Quote.State != StateClosed {
		t.Fatalf("expected HSI at 01:20 HKT with PREPRE to be StateClosed, got: %+v", hkClosedBatch["^HSI"].Quote)
	}
	if hkClosedBatch["^HSTECH"].Quote == nil || hkClosedBatch["^HSTECH"].Quote.Alias != "恒科" || hkClosedBatch["^HSTECH"].Quote.State != StateClosed {
		t.Fatalf("expected HSTECH at 01:20 HKT with PREPRE to be StateClosed, got: %+v", hkClosedBatch["^HSTECH"].Quote)
	}

	auctionHK := time.Date(2026, 10, 26, 1, 15, 0, 0, time.UTC) // 2026-10-26 09:15 HKT (周一盘前竞价)
	hkPreBatch, err := ParseYahooQuoteBatch(hkJSON, 110.0, auctionHK)
	if err != nil {
		t.Fatalf("HK indices auction parse failed: %v", err)
	}
	if hkPreBatch["^HSI"].Quote.State != StatePreMarket || hkPreBatch["^HSTECH"].Quote.State != StatePreMarket {
		t.Fatalf("expected HSI/HSTECH at 09:15 HKT to be StatePreMarket, got: %v / %v", hkPreBatch["^HSI"].Quote.State, hkPreBatch["^HSTECH"].Quote.State)
	}

	// 3. 欧洲指数 (EUR & POST/POSTPOST/CLOSED 现金指数收盘后均应为 StateClosed)
	SetActiveGroup(GroupEUIndices)
	euJSON := []byte(`{"quoteResponse":{"result":[` +
		`{"symbol":"^FCHI","currency":"EUR","quoteType":"INDEX","shortName":"CAC 40","marketState":"POSTPOST","gmtOffSetMilliseconds":3600000,"regularMarketPrice":7847.32,"regularMarketChange":64.50,"regularMarketChangePercent":0.8287,"regularMarketTime":1793032860},` +
		`{"symbol":"^GDAXI","currency":"EUR","quoteType":"INDEX","shortName":"DAX P","marketState":"POST","gmtOffSetMilliseconds":7200000,"regularMarketPrice":25410.84,"regularMarketChange":229.19,"regularMarketChangePercent":0.9101,"regularMarketTime":1793032214}` +
		`]}}`)
	euBatch, err := ParseYahooQuoteBatch(euJSON, 120.0, midnightHK)
	if err != nil {
		t.Fatalf("EU indices parse failed: %v", err)
	}
	if euBatch["^FCHI"].Quote == nil || euBatch["^FCHI"].Quote.Alias != "CAC40" || euBatch["^FCHI"].Quote.State != StateClosed {
		t.Fatalf("unexpected CAC40 quote: %+v", euBatch["^FCHI"].Quote)
	}
	if euBatch["^GDAXI"].Quote == nil || euBatch["^GDAXI"].Quote.Alias != "DAX" || euBatch["^GDAXI"].Quote.State != StateClosed {
		t.Fatalf("unexpected DAX quote: %+v", euBatch["^GDAXI"].Quote)
	}
}

func TestFiveStateSessionResolutionAndFallback(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC) // 08:00 EDT (美股盘前时段)
	cases := []struct {
		raw   string
		want  SessionState
		badge string
	}{
		{"PRE", StatePreMarket, "☼"},
		{"PREPRE", StatePreMarket, "☼"},
		{"REGULAR", StateRegular, "🟢"},
		{"POST", StatePostMarket, "🔵"},
		{"POSTPOST", StateClosed, "⚪"},
		{"OVERNIGHT", StateOvernight, "☾"},
		{"CLOSED", StateClosed, "⚪"},
	}
	for _, tc := range cases {
		got, _ := ResolveSessionState(tc.raw, now, -14400)
		if got != tc.want || got.Badge() != tc.badge {
			t.Fatalf("ResolveSessionState(%q) = (%v, %q), want (%v, %q)", tc.raw, got, got.Badge(), tc.want, tc.badge)
		}
	}

	// 验证 A股/港股/日韩指数在午夜 PREPRE 时必须返回休市 StateClosed，绝不能误报太阳 StatePreMarket
	midnightCST := time.Date(2026, 10, 26, 17, 25, 0, 0, time.UTC) // 01:25 CST
	if got, _ := ResolveSessionStateForType("PREPRE", "INDEX", midnightCST, 28800); got != StateClosed {
		t.Fatalf("expected 01:25 CST INDEX PREPRE to be StateClosed, got %v", got)
	}

	// Clock fallback when marketState is empty
	stOvn, _ := ResolveSessionState("", time.Date(2026, 10, 26, 6, 30, 0, 0, time.UTC), -14400)
	if stOvn != StateOvernight {
		t.Fatalf("expected clock fallback StateOvernight, got %v", stOvn)
	}
	stPre, _ := ResolveSessionState("", time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC), -14400)
	if stPre != StatePreMarket {
		t.Fatalf("expected clock fallback StatePreMarket, got %v", stPre)
	}
	stReg, _ := ResolveSessionState("", time.Date(2026, 10, 26, 15, 0, 0, 0, time.UTC), -14400)
	if stReg != StateRegular {
		t.Fatalf("expected clock fallback StateRegular, got %v", stReg)
	}
	stPost, _ := ResolveSessionState("", time.Date(2026, 10, 26, 21, 0, 0, 0, time.UTC), -14400)
	if stPost != StatePostMarket {
		t.Fatalf("expected clock fallback StatePostMarket, got %v", stPost)
	}
	stWknd, _ := ResolveSessionState("", time.Date(2026, 10, 24, 15, 0, 0, 0, time.UTC), -14400)
	if stWknd != StateClosed {
		t.Fatalf("expected clock fallback StateClosed on Saturday, got %v", stWknd)
	}
}

func TestParseYahooChartAndSession(t *testing.T) {
	SetActiveGroup(GroupVOOQQQ)
	vooJSON := []byte(`{"chart":{"result":[{"meta":{` +
		`"currency":"USD","symbol":"VOO","instrumentType":"ETF",` +
		`"longName":"Vanguard S&P 500 ETF","gmtoffset":-14400,` +
		`"regularMarketPrice":698.58,"regularMarketChangePercent":-0.53,` +
		`"fulldayPrice":699.16,"fulldayChange":-3.14,"fulldayChangePercent":-0.447,` +
		`"chartPreviousClose":702.3` +
		`}}]}}`)

	qqqJSON := []byte(`{"chart":{"result":[{"meta":{` +
		`"currency":"USD","symbol":"QQQ","instrumentType":"ETF",` +
		`"longName":"Invesco QQQ Trust","gmtoffset":-14400,` +
		`"regularMarketPrice":704.4,"regularMarketChangePercent":-0.01,` +
		`"fulldayPrice":703.68,"fulldayChange":-0.79,"fulldayChangePercent":-0.112,` +
		`"chartPreviousClose":704.47` +
		`}}]}}`)

	overnightTime := time.Date(2026, 10, 26, 6, 30, 0, 0, time.UTC)
	qVOO, err := ParseYahooChart("VOO", vooJSON, 120.0, overnightTime)
	if err != nil {
		t.Fatalf("unexpected VOO parse error: %v", err)
	}
	if qVOO.Price != 699.16 || math.Abs(qVOO.ChangeAbs-0.58) > 1e-4 || qVOO.State != StateOvernight || qVOO.Session != "夜盘 (Overnight)" {
		t.Fatalf("unexpected VOO quote: %+v", qVOO)
	}

	preTime := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	qQQQ, err := ParseYahooChart("QQQ", qqqJSON, 110.0, preTime)
	if err != nil {
		t.Fatalf("unexpected QQQ parse error: %v", err)
	}
	if qQQQ.Price != 703.68 || math.Abs(qQQQ.ChangeAbs - -0.72) > 1e-4 || qQQQ.State != StatePreMarket || qQQQ.Session != "盘前 (Pre-Market)" {
		t.Fatalf("unexpected QQQ quote: %+v", qQQQ)
	}

	tr := NewTracker()
	tr.ApplyBatch(map[string]SymbolResult{
		"VOO": {Quote: &qVOO},
		"QQQ": {Quote: &qQQQ},
	}, overnightTime)
	views := tr.Views(overnightTime.Add(time.Second), 2)
	if len(views) != 2 || !views[0].Healthy || !views[1].Healthy {
		t.Fatalf("expected both VOO and QQQ healthy: %+v", views)
	}

	tr.FailAll("transient proxy switch")
	views = tr.Views(overnightTime.Add(2*time.Second), 2)
	if !views[0].Healthy || !views[1].Healthy {
		t.Fatalf("expected 1 transient failure to stay healthy: %+v", views)
	}
	tr.FailAll("transient proxy switch 2")
	views = tr.Views(overnightTime.Add(4*time.Second), 2)
	if !views[0].Healthy || !views[1].Healthy {
		t.Fatalf("expected 2 transient failures to stay healthy: %+v", views)
	}
	tr.FailAll("persistent failure 3")
	views = tr.Views(overnightTime.Add(6*time.Second), 2)
	if views[0].Healthy || views[1].Healthy {
		t.Fatalf("expected 3rd consecutive failure to mark unhealthy: %+v", views)
	}

	// Test dynamic group switch on Tracker
	tr.SetGroup(GroupAsiaIndices)
	asiaViews := tr.Views(overnightTime, 2)
	if len(asiaViews) != 2 || asiaViews[0].Alias != "日经" || asiaViews[1].Alias != "韩指" || asiaViews[0].Healthy {
		t.Fatalf("unexpected asiaViews after SetGroup: %+v", asiaViews)
	}

	badVOO := []byte(`{"chart":{"result":[{"meta":{` +
		`"currency":"USD","symbol":"VOO","instrumentType":"ETF",` +
		`"fulldayPrice":699.16,"fulldayChange":-3.14,"fulldayChangePercent":5.55` +
		`}}]}}`)
	if _, err := ParseYahooChart("VOO", badVOO, 50.0, overnightTime); err == nil {
		t.Fatal("expected inconsistent fulldayChangePercent to fail validation")
	}
}
