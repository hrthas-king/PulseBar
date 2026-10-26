package market

import (
	"testing"
	"time"
)

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

	// Monday 02:30 EDT = 06:30 UTC -> Overnight session
	overnightTime := time.Date(2026, 10, 26, 6, 30, 0, 0, time.UTC)
	qVOO, err := ParseYahooChart("VOO", vooJSON, 120.0, overnightTime)
	if err != nil {
		t.Fatalf("unexpected VOO parse error: %v", err)
	}
	if qVOO.Price != 699.16 || qVOO.ChangePercent != -0.447 || qVOO.Session != "夜盘 (Overnight)" {
		t.Fatalf("unexpected VOO quote: %+v", qVOO)
	}

	// Monday 08:00 EDT = 12:00 UTC -> Pre-Market session
	preTime := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	qQQQ, err := ParseYahooChart("QQQ", qqqJSON, 110.0, preTime)
	if err != nil {
		t.Fatalf("unexpected QQQ parse error: %v", err)
	}
	if qQQQ.Price != 703.68 || qQQQ.Session != "盘前 (Pre-Market)" {
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

	// Inconsistent change percent must be rejected
	badVOO := []byte(`{"chart":{"result":[{"meta":{` +
		`"currency":"USD","symbol":"VOO","instrumentType":"ETF",` +
		`"fulldayPrice":699.16,"fulldayChange":-3.14,"fulldayChangePercent":5.55` +
		`}}]}}`)
	if _, err := ParseYahooChart("VOO", badVOO, 50.0, overnightTime); err == nil {
		t.Fatal("expected inconsistent fulldayChangePercent to fail validation")
	}
}
