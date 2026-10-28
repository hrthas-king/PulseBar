package config

import (
	"path/filepath"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	def := LoadFrom(path)
	if def.IntervalSeconds != 2 || def.Placement != "left" || def.ShowNetDetail || def.MarketGroup != DefaultMarketGroup || def.UIScalePercent != DefaultUIScalePercent {
		t.Fatalf("unexpected default config: %+v", def)
	}

	x, y := 120, 1040
	custom := Config{
		IntervalSeconds: 5,
		ShowNetDetail:   true,
		Locked:          true,
		UpIsRed:         true,
		MarketGroup:     "hk_indices",
		UIScalePercent:  85,
		Placement:       "manual",
		X:               &x,
		Y:               &y,
	}
	if err := SaveTo(path, custom); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	loaded := LoadFrom(path)
	if loaded.IntervalSeconds != 5 || !loaded.ShowNetDetail || !loaded.Locked || !loaded.UpIsRed ||
		loaded.MarketGroup != "hk_indices" || loaded.UIScalePercent != 85 ||
		loaded.Placement != "manual" || loaded.X == nil || *loaded.X != 120 || *loaded.Y != 1040 {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}

	if !ValidMarketGroup("gold_oil") {
		t.Fatalf("expected gold_oil to be valid market group")
	}
	if ValidMarketGroup("invalid_group") {
		t.Fatalf("expected invalid_group to be invalid")
	}
}

func TestETFGroupConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c := Default()
	c.MarketGroup = "etf_159632_159655"
	if err := SaveTo(path, c); err != nil {
		t.Fatalf("save ETF group: %v", err)
	}
	if got := LoadFrom(path).MarketGroup; got != c.MarketGroup {
		t.Fatalf("saved ETF group %q loaded as %q", c.MarketGroup, got)
	}
}
