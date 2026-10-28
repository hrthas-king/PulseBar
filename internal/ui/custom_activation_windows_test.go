//go:build windows && (amd64 || arm64)

package ui

import (
	"context"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/hrthas-king/PulseBar/internal/config"
)

func TestVisibleAddFromMenuRemainsOpen(t *testing.T) {
	if os.Getenv("PULSEBAR_VISIBLE_PANEL_TEST") != "1" {
		t.Skip("requires an interactive Windows desktop")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cfg := config.Default()
	cfg.Locked = true
	u := NewUI("activation-test", cfg)
	var panelExists, visible, foreground bool
	isVisible := user32.NewProc("IsWindowVisible")
	getForeground := user32.NewProc("GetForegroundWindow")
	getFocus := user32.NewProc("GetFocus")
	var inputFocused bool
	killTimer := user32.NewProc("KillTimer")
	check := syscall.NewCallback(func(hwnd uintptr, _ uint32, id uintptr, _ uint32) uintptr {
		killTimer.Call(hwnd, id)
		if p := u.customPanel; p != nil {
			panelExists = true
			v, _, _ := isVisible.Call(p.hwnd)
			visible = v != 0
			active, _, _ := getForeground.Call()
			foreground = active == p.hwnd
			focus, _, _ := getFocus.Call()
			inputFocused = focus == p.first || focus == p.second
		}
		cancel()
		return 0
	})
	err := u.Run(ctx, func() {
		setForeground.Call(u.hwnd)
		menu, _, _ := createMenu.Call()
		u.handleMenuSelection(uintptr(actionAddCustom), nil)
		destroyMenu.Call(menu)
		panelSetTimer.Call(u.hwnd, 9999, 250, check)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !panelExists || !visible || !inputFocused {
		t.Fatalf("Add returned but input panel not usable: exists=%v visible=%v foreground=%v inputFocus=%v", panelExists, visible, foreground, inputFocused)
	}
	t.Logf("panel stays visible and the input control is focused; system foreground=%v", foreground)
}
