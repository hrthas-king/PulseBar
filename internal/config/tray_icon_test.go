package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTrayIconDefaultsLegacyConfigAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	for _, legacy := range []string{
		`{"interval_seconds":1,"market_group":"asia_indices","ui_scale_percent":85,"locked":true}`,
		`{"interval_seconds":1,"market_group":"asia_indices","ui_scale_percent":85,"locked":true,"tray_icon":"unknown"}`,
	} {
		if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
			t.Fatal(err)
		}
		got := LoadFrom(path)
		if got.TrayIcon != TrayIconApplication || got.IntervalSeconds != 1 || got.MarketGroup != "asia_indices" || got.UIScalePercent != 85 || !got.Locked {
			t.Fatalf("legacy settings changed during icon migration: %#v", got)
		}
	}
	for _, style := range []string{TrayIconApplication, TrayIconNetwork} {
		c := LoadFrom(path)
		c.TrayIcon = style
		if err := SaveTo(path, c); err != nil {
			t.Fatal(err)
		}
		if got := LoadFrom(path); !reflect.DeepEqual(got, c) {
			t.Fatalf("icon choice not restored after restart: got=%#v want=%#v", got, c)
		}
	}
}

func TestTrayIconSaveNormalizesEmptyAndRejectsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c := Default()
	c.TrayIcon = ""
	if err := SaveTo(path, c); err != nil || LoadFrom(path).TrayIcon != TrayIconApplication {
		t.Fatalf("missing old icon setting not normalized: %v", err)
	}
	before, _ := os.ReadFile(path)
	c.TrayIcon = "invalid"
	if err := SaveTo(path, c); err == nil {
		t.Fatal("invalid icon style saved")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("invalid choice changed existing settings")
	}
}
