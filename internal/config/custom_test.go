package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hrthas-king/PulseBar/internal/watchlist"
)

func TestCustomPairPersistenceAndDeletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	a, _ := watchlist.NewPair("600519", "000858")
	b, _ := watchlist.NewPair("300750", "920002")
	c := Default()
	c.UIScalePercent = 85
	c.ShowNetDetail = true
	c, _ = c.AddPair(a)
	c, _ = c.AddPair(b)
	if err := SaveTo(path, c); err != nil {
		t.Fatal(err)
	}
	loaded := LoadFrom(path)
	if len(loaded.CustomPairs) != 2 || loaded.MarketGroup != b.ID() || loaded.UIScalePercent != 85 || !loaded.ShowNetDetail {
		t.Fatalf("lost saved state: %+v", loaded)
	}
	loaded, _ = loaded.RemovePair(a.ID())
	if loaded.MarketGroup != b.ID() {
		t.Fatal("deleting inactive pair changed selection")
	}
	loaded, _ = loaded.RemovePair(b.ID())
	if loaded.MarketGroup != DefaultMarketGroup || len(loaded.CustomPairs) != 0 {
		t.Fatal("deleting active pair did not select default")
	}
	if err := SaveTo(path, loaded); err != nil {
		t.Fatal(err)
	}
	if got := LoadFrom(path); len(got.CustomPairs) != 0 || got.MarketGroup != DefaultMarketGroup {
		t.Fatal("deleted pair returned after restart")
	}
	duplicate, _ := c.AddPair(b)
	if len(duplicate.CustomPairs) != 2 {
		t.Fatal("duplicate pair appended")
	}
	before, _ := os.ReadFile(path)
	c.CustomPairs = append(c.CustomPairs, b)
	if err := SaveTo(path, c); err == nil {
		t.Fatal("accepted duplicate storage rows")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("invalid save changed disk")
	}
}
