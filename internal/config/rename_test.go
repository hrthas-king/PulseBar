package config

import (
	"path/filepath"
	"testing"
)

func TestRenamedApplicationKeepsLegacyConfiguration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("SAMUEL_CONFIG", "")
	t.Setenv("PULSEBAR_CONFIG", "")
	want := filepath.Join(dir, "PulseBar", "config.json")
	if Path() != want {
		t.Fatalf("legacy settings path changed: %s", Path())
	}
	c := Default()
	c.UIScalePercent = 85
	c.MarketGroup = "hk_indices"
	if err := SaveTo(want, c); err != nil {
		t.Fatal(err)
	}
	if got := Load(); got.UIScalePercent != 85 || got.MarketGroup != "hk_indices" {
		t.Fatalf("legacy settings not loaded: %+v", got)
	}
	t.Setenv("PULSEBAR_CONFIG", filepath.Join(dir, "old-override.json"))
	t.Setenv("SAMUEL_CONFIG", filepath.Join(dir, "new-override.json"))
	if Path() != filepath.Join(dir, "new-override.json") {
		t.Fatal("Samuel override must take precedence")
	}
	t.Setenv("SAMUEL_CONFIG", "")
	if Path() != filepath.Join(dir, "old-override.json") {
		t.Fatal("legacy override lost")
	}
}
