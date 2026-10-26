package config

import (
	"path/filepath"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	def := LoadFrom(path)
	if def.IntervalSeconds != 2 || def.Placement != "left" || def.ShowNetDetail || def.MarketGroup != DefaultMarketGroup {
		t.Fatalf("unexpected default config: %+v", def)
	}

	x, y := 120, 1040
	custom := Config{
		IntervalSeconds: 5,
		ShowNetDetail:   true,
		Locked:          true,
		UpIsRed:         true,
		MarketGroup:     "hk_indices",
		Placement:       "manual",
		X:               &x,
		Y:               &y,
	}
	if err := SaveTo(path, custom); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	loaded := LoadFrom(path)
	if loaded.IntervalSeconds != 5 || !loaded.ShowNetDetail || !loaded.Locked || !loaded.UpIsRed ||
		loaded.MarketGroup != "hk_indices" ||
		loaded.Placement != "manual" || loaded.X == nil || *loaded.X != 120 || *loaded.Y != 1040 {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
}
