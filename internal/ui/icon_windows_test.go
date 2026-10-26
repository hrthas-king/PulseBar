//go:build windows && (amd64 || arm64)

package ui

import (
	"testing"
	"unsafe"

	"github.com/hrthas-king/PulseBar/internal/app"
	"github.com/hrthas-king/PulseBar/internal/config"
	"github.com/hrthas-king/PulseBar/internal/ipinfo"
	"github.com/hrthas-king/PulseBar/internal/monitor"
	"github.com/hrthas-king/PulseBar/internal/winapi"
)

func TestNativeTrayIconIsUpright(t *testing.T) {
	u := NewUI("dev", config.Default())
	u.instance, _, _ = getModule.Call(0)
	snapshot := app.Snapshot{Status: monitor.Online, Geo: ipinfo.Info{Country: "TH"}}
	_ = u.update(snapshot)
	if u.data.Icon == 0 {
		t.Fatal("native icon was not created")
	}
	defer func() {
		for _, icon := range u.icons {
			destroyIcon.Call(icon)
		}
	}()
	var info struct {
		Icon               uint32
		HotspotX, HotspotY uint32
		_                  uint32
		Mask, Color        uintptr
	}
	getIconInfo := user32.NewProc("GetIconInfo")
	if ok, _, err := getIconInfo.Call(u.data.Icon, uintptr(unsafe.Pointer(&info))); ok == 0 {
		t.Fatal(err)
	}
	gdi := winapi.DLL("gdi32.dll")
	delObj := gdi.NewProc("DeleteObject")
	defer delObj.Call(info.Mask)
	defer delObj.Call(info.Color)
	var bitmap struct {
		Size                        uint32
		Width, Height               int32
		Planes, BitCount            uint16
		Compression, SizeImage      uint32
		XPels, YPels                int32
		ColorsUsed, ColorsImportant uint32
		Color                       uint32
	}
	bitmap.Size = 40
	bitmap.Width = 32
	bitmap.Height = -32
	bitmap.Planes = 1
	bitmap.BitCount = 32
	var pixels [4096]byte
	dc, _, _ := user32.NewProc("GetDC").Call(0)
	if dc == 0 {
		t.Fatal("GetDC failed")
	}
	defer user32.NewProc("ReleaseDC").Call(0, dc)
	rows, _, err := gdi.NewProc("GetDIBits").Call(dc, info.Color, 0, 32, uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&bitmap)), 0)
	if rows != 32 {
		t.Fatalf("GetDIBits rows=%d err=%v", rows, err)
	}
	want := Pixels("TH", monitor.Online, false, false, true)
	for y := 4; y < 18; y++ {
		for x := 5; x < 27; x++ {
			i := (y*32 + x) * 4
			if pixels[i] != want[i+2] || pixels[i+1] != want[i+1] || pixels[i+2] != want[i] {
				t.Fatalf("country letters vertically inverted at (%d,%d)", x, y)
			}
		}
	}
}
