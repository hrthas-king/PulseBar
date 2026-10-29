package app

import (
	"context"
	"testing"
	"time"

	"github.com/hrthas-king/Samuel/internal/ipinfo"
	"github.com/hrthas-king/Samuel/internal/market"
	"github.com/hrthas-king/Samuel/internal/monitor"
	"github.com/hrthas-king/Samuel/internal/watchlist"
)

func TestCustomSelectionAndDeletedSelectionRefresh(t *testing.T) {
	p, _ := watchlist.NewPair("600519", "000858")
	market.SetCustomPairs([]watchlist.Pair{p})
	market.SetActiveGroup(market.DefaultGroup)
	defer market.SetCustomPairs(nil)
	defer market.SetActiveGroup(market.DefaultGroup)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	e := New(Dependencies{
		ReadCounters: func(string) (monitor.Counters, error) {
			return monitor.Counters{LUID: 1, Index: 1, At: time.Now()}, nil
		},
		Ping:     func(string) monitor.Probe { return monitor.Probe{Success: true, At: time.Now()} },
		FetchGeo: func(context.Context) (ipinfo.Info, error) { return ipinfo.Info{Country: "CN"}, nil },
		FetchQuotes: func(context.Context) (map[string]market.SymbolResult, error) {
			out := map[string]market.SymbolResult{}
			for _, sym := range market.ActiveSymbols() {
				q := market.Quote{Symbol: sym, Price: 100, ChangeAbs: 0, RegularPrice: 100, UpdateMode: "test", State: market.StateRegular}
				out[sym] = market.SymbolResult{Quote: &q}
			}
			return out, nil
		},
	})
	updates := make(chan Snapshot, 64)
	done := make(chan struct{})
	go func() { defer close(done); e.Run(ctx, 1, func(s Snapshot) { updates <- s }) }()
	defer func() { cancel(); <-done }()
	wait := func(group, first string) {
		t.Helper()
		for {
			select {
			case s := <-updates:
				if s.MarketGroup == group && len(s.Quotes) == 2 && s.Quotes[0].Symbol == first && s.Quotes[0].Healthy {
					return
				}
			case <-ctx.Done():
				t.Fatalf("group %s did not refresh", group)
			}
		}
	}
	wait(market.DefaultGroup, "VOO")
	e.Group <- p.ID()
	wait(p.ID(), "600519.SS")
	market.SetCustomPairs(nil)
	e.Group <- market.DefaultGroup
	wait(market.DefaultGroup, "VOO")
}
