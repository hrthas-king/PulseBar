//go:build windows && (amd64 || arm64)

package ui

import (
	"context"
	"os"
	"runtime"
	"strings"
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
		menu, commands := u.buildMenu()
		work := popupWorkArea(point{100, 100})
		u.showFluentMenu(menu, point{work.Right - 20, work.Top + 100}, commands)
		destroyMenu.Call(menu)
		if u.flyout == nil {
			t.Fatal("Fluent menu did not open")
		}
		root := u.flyout.windows[0]
		moreIndex := -1
		for i, e := range root.entries {
			if strings.HasPrefix(e.label, "More") {
				moreIndex = i
				break
			}
		}
		if moreIndex < 0 {
			t.Fatal("More entry missing")
		}
		r := root.rowBounds(moreIndex)
		mouseMoveFluent(root, root.width/2, (r.Top+r.Bottom)/2)
		u.flyout.advanceHover(time.Now().Add(200*time.Millisecond), point{root.bounds.Left + root.width/2, root.bounds.Top + (r.Top+r.Bottom)/2})
		if len(u.flyout.windows) != 2 {
			t.Fatal("hover did not open More")
		}
		child := u.flyout.windows[1]
		if child.bounds.Left >= root.bounds.Left {
			t.Fatal("visible test did not open to the left")
		}
		mouseMoveFluent(root, 3, (r.Top+r.Bottom)/2)
		mouseMoveFluent(root, root.width/3, r.Top-3)
		if len(u.flyout.windows) != 2 {
			t.Fatal("left cascade closed during pointer crossing")
		}
		mouseMoveFluent(child, child.width-10, child.rowBounds(0).Top+10)
		u.flyout.advanceHover(time.Now().Add(time.Second), point{child.bounds.Left + 20, child.bounds.Top + 20})
		if len(u.flyout.windows) != 2 || u.flyout.windows[1] != child {
			t.Fatal("left cascade closed after entry")
		}
		sendMessage.Call(root.hwnd, 0x100, 35, 0) // Other is the last entry.
		sendMessage.Call(root.hwnd, 0x100, 39, 0) // Other
		sendMessage.Call(root.hwnd, 0x100, 13, 0) // Add; must close menu before opening the input.
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
