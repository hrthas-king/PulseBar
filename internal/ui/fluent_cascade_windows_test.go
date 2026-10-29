//go:build windows && (amd64 || arm64)

package ui

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/hrthas-king/Samuel/internal/config"
)

func mouseMoveFluent(p *fluentPopup, x, y int32) {
	sendMessage.Call(p.hwnd, 0x200, 0, uintptr(uint32(uint16(x))|uint32(uint16(y))<<16))
}

func TestFluentLeftCascadeSurvivesPointerTransition(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for _, diagonal := range []bool{false, true} {
		name := "edge"
		if diagonal {
			name = "diagonal"
		}
		t.Run(name, func(t *testing.T) {
			u := NewUI("test", config.Default())
			s := &fluentMenu{u: u, visual: newFluentVisuals(1, fluentColors(true))}
			u.flyout = s
			defer u.closeFluentMenu()
			work := popupWorkArea(point{100, 100})
			entries := []fluentEntry{{label: "Refresh", action: ActionRefresh, enabled: true}, {label: "More", enabled: true, children: []fluentEntry{{label: "Select", action: ActionRefresh, enabled: true}}}}
			if !s.open(entries, point{work.Right - 20, work.Top + 100}, nil) {
				t.Fatal("menu creation failed")
			}
			root := s.windows[0]
			r := root.rowBounds(1)
			y := (r.Top + r.Bottom) / 2
			mouseMoveFluent(root, root.width/2, y)
			root.openChild(1)
			if len(s.windows) != 2 {
				t.Fatal("cascade did not open")
			}
			child := s.windows[1]
			if child.bounds.Left >= root.bounds.Left {
				t.Fatal("test did not open the cascade to the left")
			}
			x := int32(3)
			if diagonal {
				x = root.width / 3
				y = r.Top - 3
			}
			mouseMoveFluent(root, x, y)
			if len(s.windows) != 2 || s.windows[1] != child {
				t.Fatal("moving toward the left cascade dismissed it before the pointer could enter")
			}
			mouseMoveFluent(child, child.width-10, child.rowBounds(0).Top+10)
			if len(s.windows) != 2 || child.hover != 0 {
				t.Fatal("left cascade was not usable after entering")
			}
		})
	}
}

func TestFluentNaturalMenuHasNoFalseOverflow(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for _, scale := range []float64{1, 1.5, 2} {
		t.Run(fmt.Sprintf("%g", scale), func(t *testing.T) {
			u := NewUI("test", config.Default())
			s := &fluentMenu{u: u, visual: newFluentVisuals(scale, fluentColors(false))}
			defer s.close()
			if !s.open([]fluentEntry{{label: "Refresh", action: ActionRefresh, enabled: true}}, point{100, 100}, nil) {
				t.Fatal("menu creation failed")
			}
			p := s.windows[0]
			if p.height < p.content {
				t.Fatalf("fully visible menu has false overflow: client=%d content=%d", p.height, p.content)
			}
		})
	}
}

func TestFluentCascadeGraceCancelsOnEntryAndStillSwitchesSiblings(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for _, left := range []bool{false, true} {
		name := "right"
		if left {
			name = "left"
		}
		t.Run(name, func(t *testing.T) {
			u := NewUI("test", config.Default())
			s := &fluentMenu{u: u, visual: newFluentVisuals(1, fluentColors(false))}
			u.flyout = s
			defer u.closeFluentMenu()
			work := popupWorkArea(point{100, 100})
			x := work.Left + 20
			if left {
				x = work.Right - 20
			}
			entries := []fluentEntry{{label: "Another group", enabled: true, children: []fluentEntry{{label: "Other", enabled: true}}}, {label: "More", enabled: true, children: []fluentEntry{{label: "Select", action: ActionRefresh, enabled: true}}}}
			if !s.open(entries, point{x, work.Top + 100}, nil) {
				t.Fatal("menu creation failed")
			}
			root := s.windows[0]
			root.setHover(1)
			root.openChild(1)
			child := s.windows[1]
			start := time.Now()
			r := root.rowBounds(0)
			at := point{root.width / 3, (r.Top + r.Bottom) / 2}
			root.pointerMove(at, start)
			if len(s.windows) != 2 || s.windows[1] != child {
				t.Fatal("sibling hover removed child before crossing")
			}
			child.pointerMove(point{20, 20}, start.Add(100*time.Millisecond))
			s.advanceHover(start.Add(time.Second), point{child.bounds.Left + 20, child.bounds.Top + 20})
			if len(s.windows) != 2 || s.windows[1] != child || !root.hoverSwitchDeadline.IsZero() {
				t.Fatal("stale parent hover dismissed child after entry")
			}
			root.pointerMove(at, start.Add(2*time.Second))
			s.advanceHover(start.Add(2400*time.Millisecond), point{root.bounds.Left + at.X, root.bounds.Top + at.Y})
			if len(s.windows) != 2 || s.windows[1] == child || root.hover != 0 {
				t.Fatal("deliberate sibling hover stopped switching menus")
			}
			s.close()
		})
	}
}

func TestFluentMenuDoesNotDrawScrollIndicator(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	v := newFluentVisuals(1, fluentColors(true))
	defer v.close()
	p := &fluentPopup{menu: &fluentMenu{visual: v}, width: 300, height: 240, content: 900, offset: 300, hover: -1}
	img := renderFluentBitmap(t, 300, 240, p.draw)
	for y := 20; y < 220; y++ {
		pixel := img.NRGBAAt(297, y)
		if pixel.R != 44 || pixel.G != 44 || pixel.B != 44 {
			t.Fatalf("scroll indicator remains visible at y=%d: %+v", y, pixel)
		}
	}
}
