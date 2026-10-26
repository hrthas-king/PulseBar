package market

import (
	"math"
	"testing"
	"time"
)

func TestParseYahooQuoteBatchAllSessions(t *testing.T) {
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
	if voo.Price != 699.05 || math.Abs(voo.ChangePercent-0.06728) > 1e-4 || voo.Session != "夜盘 (Overnight BOATS)" || voo.QuoteSource != "BOATS Real Time Price" {
		t.Fatalf("unexpected VOO overnight quote: %+v", voo)
	}
	if qqq.Price != 703.55 || math.Abs(qqq.ChangePercent - -0.12067) > 1e-4 || qqq.Session != "夜盘 (Overnight BOATS)" {
		t.Fatalf("unexpected QQQ overnight quote: %+v", qqq)
	}
}

func TestParseYahooChartAndSession(t *testing.T) {
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

	// Monday 02:30 EDT = 06:30 UTC -> Overnight session; baseline corrected to Friday close 698.58
	overnightTime := time.Date(2026, 10, 26, 6, 30, 0, 0, time.UTC)
	qVOO, err := ParseYahooChart("VOO", vooJSON, 120.0, overnightTime)
	if err != nil {
		t.Fatalf("unexpected VOO parse error: %v", err)
	}
	if qVOO.Price != 699.16 || math.Abs(qVOO.ChangeAbs-0.58) > 1e-4 || qVOO.Session != "夜盘 (Overnight)" {
		t.Fatalf("unexpected VOO quote: %+v", qVOO)
	}

	// Monday 08:00 EDT = 12:00 UTC -> Pre-Market session; baseline corrected to Friday close 704.40
	preTime := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	qQQQ, err := ParseYahooChart("QQQ", qqqJSON, 110.0, preTime)
	if err != nil {
		t.Fatalf("unexpected QQQ parse error: %v", err)
	}
	if qQQQ.Price != 703.68 || math.Abs(qQQQ.ChangeAbs - -0.72) > 1e-4 || qQQQ.Session != "盘前 (Pre-Market)" {
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

	// 1 or 2 transient failures within 30s grace window should keep green (Healthy=true)
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
	// 3rd consecutive failure should turn red (Healthy=false)
	tr.FailAll("persistent failure 3")
	views = tr.Views(overnightTime.Add(6*time.Second), 2)
	if views[0].Healthy || views[1].Healthy {
		t.Fatalf("expected 3rd consecutive failure to mark unhealthy: %+v", views)
	}

	// Inconsistent change percent must be rejected
	badVOO := []byte(`{"chart":{"result":[{"meta":{` +
		`"currency":"USD","symbol":"VOO","instrumentType":"ETF",` +
		`"fulldayPrice":699.16,"fulldayChange":-3.14,"fulldayChangePercent":5.55` +
		`}}]}}`)
	if _, err := ParseYahooChart("VOO", badVOO, 50.0, overnightTime); err == nil {
		t.Fatal("expected inconsistent fulldayChangePercent to fail validation")
	}
}
