package market

import (
	"testing"
	"time"
)

func TestParseTradingViewAndTracker(t *testing.T) {
	validJSON := []byte(`{"totalCount":2,"data":[` +
		`{"s":"CME_MINI:ES1!","d":["ES1!","E-mini S&P 500 Futures","futures",5050.0,1.0,50.0,"USD","delayed_streaming_600"]},` +
		`{"s":"CME_MINI:NQ1!","d":["NQ1!","E-mini Nasdaq-100 Futures","futures",19800.0,-1.0,-200.0,"USD","streaming"]}` +
		`]}`)

	res, err := ParseTradingView(validJSON, 45.0)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if res["ES"].Quote == nil || res["NQ"].Quote == nil {
		t.Fatalf("expected both quotes valid: %+v", res)
	}
	if res["ES"].Quote.DelayText() != "延时 10 分钟" {
		t.Fatalf("unexpected ES delay text: %s", res["ES"].Quote.DelayText())
	}
	if res["NQ"].Quote.DelayText() != "源标记 streaming（不作实时保证）" {
		t.Fatalf("unexpected NQ delay text: %s", res["NQ"].Quote.DelayText())
	}

	tr := NewTracker()
	now := time.Now()
	tr.ApplyBatch(res, now)
	views := tr.Views(now.Add(2*time.Second), 3)
	if len(views) != 2 || !views[0].Healthy || !views[1].Healthy {
		t.Fatalf("expected both views healthy: %+v", views)
	}

	// Independent symbol failure: ES fails consistency check, NQ stays healthy
	partialJSON := []byte(`{"totalCount":2,"data":[` +
		`{"s":"CME_MINI:ES1!","d":["ES1!","E-mini S&P 500 Futures","futures",5050.0,9.99,50.0,"USD","delayed_streaming_600"]},` +
		`{"s":"CME_MINI:NQ1!","d":["NQ1!","E-mini Nasdaq-100 Futures","futures",19800.0,-1.0,-200.0,"USD","streaming"]}` +
		`]}`)
	res2, err := ParseTradingView(partialJSON, 30.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr.ApplyBatch(res2, now.Add(3*time.Second))
	views2 := tr.Views(now.Add(3*time.Second), 3)
	if views2[0].Healthy || views2[0].Quote == nil {
		t.Fatalf("ES should be unhealthy while preserving last valid quote: %+v", views2[0])
	}
	if !views2[1].Healthy {
		t.Fatalf("NQ should remain healthy: %+v", views2[1])
	}

	// Watchdog expiration after maxAge
	staleViews := tr.Views(now.Add(20*time.Second), 3)
	if staleViews[1].Healthy {
		t.Fatalf("expected watchdog expiration for NQ: %+v", staleViews[1])
	}
}
