//go:build windows && (amd64 || arm64)

package ui

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/hrthas-king/Samuel/internal/app"
	"github.com/hrthas-king/Samuel/internal/config"
	"github.com/hrthas-king/Samuel/internal/ipinfo"
	"github.com/hrthas-king/Samuel/internal/monitor"
	"github.com/hrthas-king/Samuel/internal/winapi"
)

func TestTrayIconMenuSelectionPersistsAndChecksOneOption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	u := NewUI("test", config.Default())
	u.saveSettings = func(c config.Config) error { return config.SaveTo(path, c) }
	for _, tc := range []struct {
		action Action
		style  string
	}{
		{ActionTrayIconNetwork, config.TrayIconNetwork},
		{ActionTrayIconApplication, config.TrayIconApplication},
	} {
		u.handleMenuSelection(uintptr(tc.action), nil)
		if loaded := config.LoadFrom(path); loaded.TrayIcon != tc.style || u.cfg.TrayIcon != tc.style {
			t.Fatalf("menu choice not saved: %#v", loaded)
		}
		restarted := NewUI("test", config.LoadFrom(path))
		menu, commands := restarted.buildMenu()
		options := fluentFind(readFluentMenu(menu, commands), "托盘图标")
		destroyMenu.Call(menu)
		if len(options) != 2 {
			t.Fatalf("missing icon choices in native/Fluent menu: %#v", options)
		}
		checked := 0
		for _, option := range options {
			if option.checked {
				checked++
				if option.action != tc.action {
					t.Fatal("menu highlights a different icon than the saved choice")
				}
			}
		}
		if checked != 1 {
			t.Fatalf("icon choices not mutually exclusive: %d checked", checked)
		}
	}
	u.saveSettings = func(config.Config) error { return errors.New("disk unavailable") }
	if err := u.setTrayIcon(config.TrayIconNetwork); err == nil || u.cfg.TrayIcon != config.TrayIconApplication || config.LoadFrom(path).TrayIcon != config.TrayIconApplication {
		t.Fatal("failed save changed icon selection")
	}
}

func TestNativeTrayChoiceUsesEmbeddedLogoAndReusesHandles(t *testing.T) {
	exe := os.Getenv("SAMUEL_TRAY_RESOURCE_EXE")
	if exe == "" {
		t.Skip("set SAMUEL_TRAY_RESOURCE_EXE to a built Samuel executable")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	name := winapi.UTF16(exe)
	module, _, err := kernel32.NewProc("LoadLibraryExW").Call(uintptr(unsafe.Pointer(name)), 0, 2) // resources only; no code execution
	runtime.KeepAlive(name)
	if module == 0 {
		t.Fatal("load icon resource module:", err)
	}
	defer kernel32.NewProc("FreeLibrary").Call(module)
	u := NewUI("test", config.Default())
	u.instance = module
	defer u.releaseTrayIcons()
	path := filepath.Join(t.TempDir(), "config.json")
	u.saveSettings = func(c config.Config) error { return config.SaveTo(path, c) }
	s := app.Snapshot{Status: monitor.Online, Geo: ipinfo.Info{Country: "KR"}, At: time.Now()}
	u.latest.Store(&s)
	if err := u.update(s); err != nil {
		t.Fatal(err)
	}
	logo := u.data.Icon
	if logo == 0 || logo != u.appIcon || len(u.icons) != 0 {
		t.Fatal("default tray icon is not the embedded application logo")
	}
	iconPath, err := filepath.Abs(filepath.Join("..", "..", "assets", "Samuel.ico"))
	if err != nil {
		t.Fatal(err)
	}
	fileName := winapi.UTF16(iconPath)
	expected, _, err := loadImage.Call(0, uintptr(unsafe.Pointer(fileName)), 1, 32, 32, 0x10)
	runtime.KeepAlive(fileName)
	if expected == 0 {
		t.Fatal("load independent ICO:", err)
	}
	defer destroyIcon.Call(expected)
	if nativeIconPixels(t, logo) != nativeIconPixels(t, expected) {
		t.Fatal("tray logo pixels differ from Samuel.ico")
	}
	process, _, _ := kernel32.NewProc("GetCurrentProcess").Call()
	// Warm the network cache before comparing resource counts.
	if err := u.setTrayIcon(config.TrayIconNetwork); err != nil {
		t.Fatal(err)
	}
	networkIcon := u.data.Icon
	if networkIcon == 0 || networkIcon == logo {
		t.Fatal("network selection did not immediately change the native icon")
	}
	before, _, _ := user32.NewProc("GetGuiResources").Call(process, 1)
	for i := 0; i < 40; i++ {
		if err := u.setTrayIcon(config.TrayIconApplication); err != nil || u.data.Icon != logo {
			t.Fatal("application icon was not reused:", err)
		}
		changed := s
		changed.Geo.Country = "US"
		changed.Status = monitor.Offline
		if err := u.update(changed); err != nil || u.data.Icon != logo {
			t.Fatal("network changes replaced the application logo:", err)
		}
		if err := u.setTrayIcon(config.TrayIconNetwork); err != nil || u.data.Icon != networkIcon {
			t.Fatal("network icon was not restored immediately:", err)
		}
	}
	after, _, _ := user32.NewProc("GetGuiResources").Call(process, 1)
	if after > before+2 {
		t.Fatalf("native icon handles leaked: USER before=%d after=%d", before, after)
	}
	if config.LoadFrom(path).TrayIcon != config.TrayIconNetwork {
		t.Fatal("final icon choice not persisted")
	}
}
