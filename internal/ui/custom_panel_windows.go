//go:build windows && (amd64 || arm64)

package ui

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"github.com/hrthas-king/PulseBar/internal/winapi"
)

var (
	setWindowText    = user32.NewProc("SetWindowTextW")
	getWindowText    = user32.NewProc("GetWindowTextW")
	enableWindow     = user32.NewProc("EnableWindow")
	setFocus         = user32.NewProc("SetFocus")
	panelForeground  = user32.NewProc("GetForegroundWindow")
	getAsyncKeyState = user32.NewProc("GetAsyncKeyState")
	panelSetTimer    = user32.NewProc("SetTimer")
	callWindowProc   = user32.NewProc("CallWindowProcW")
	getMonitorInfo   = user32.NewProc("GetMonitorInfoW")
	panelWindows     sync.Map
	panelEdits       sync.Map
	panelProcedure   = syscall.NewCallback(customPanelProc)
	editProcedure    = syscall.NewCallback(customEditProc)
)

type customPanel struct {
	u                                   *UI
	hwnd, first, second, submit, status uintptr
	busy                                bool
	outsideArmed                        bool
	focusEstablished                    bool
}
type editBinding struct {
	panel    *customPanel
	original uintptr
}

func panelText(hwnd uintptr) string {
	var buf [128]uint16
	getWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf[:])
}
func (p *customPanel) setStatus(text string) {
	s := winapi.UTF16(text)
	setWindowText.Call(p.status, uintptr(unsafe.Pointer(s)))
}

func (u *UI) closeCustomPanel() {
	p := u.customPanel
	if p == nil {
		return
	}
	u.customPanel = nil
	u.customEpoch++
	if u.customCancel != nil {
		u.customCancel()
		u.customCancel = nil
	}
	destroyWindow.Call(p.hwnd)
}

func (u *UI) openCustomPanel(show bool) {
	u.closeCustomPanel()
	u.customEpoch++
	instance := u.instance
	if instance == 0 {
		instance, _, _ = getModule.Call(0)
	}
	cls := winapi.UTF16("PulseBar.CustomStocks")
	wc := windowClass{Size: uint32(unsafe.Sizeof(windowClass{})), Procedure: panelProcedure, Instance: instance, Background: 6, ClassName: cls}
	cursor, _, _ := loadCursor.Call(0, 32512)
	wc.Cursor = cursor
	if atom, _, err := registerClass.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 && err != syscall.Errno(1410) {
		ShowError(winapi.Error("RegisterClassExW", err))
		return
	}
	var pos point
	getCursor.Call(uintptr(unsafe.Pointer(&pos)))
	w, h := u.px(340), u.px(218)
	x, y := pos.X+u.px(8), pos.Y+u.px(8)
	anchor := rect{pos.X, pos.Y, pos.X + 1, pos.Y + 1}
	monitor, _, _ := monitorFromRect.Call(uintptr(unsafe.Pointer(&anchor)), 2)
	var info struct {
		Size          uint32
		Monitor, Work rect
		Flags         uint32
	}
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, _ := getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok != 0 {
		if x+w > info.Work.Right {
			x = info.Work.Right - w
		}
		if y+h > info.Work.Bottom {
			y = info.Work.Bottom - h
		}
		if x < info.Work.Left {
			x = info.Work.Left
		}
		if y < info.Work.Top {
			y = info.Work.Top
		}
	}
	title := winapi.UTF16("新增自定义股票 / ETF组合")
	hwnd, _, err := createWindow.Call(0x8|0x80, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)), 0x80000000, uintptr(x), uintptr(y), uintptr(w), uintptr(h), u.hwnd, 0, instance, 0)
	if hwnd == 0 {
		ShowError(winapi.Error("CreateWindowExW", err))
		return
	}
	p := &customPanel{u: u, hwnd: hwnd}
	u.customPanel = p
	panelWindows.Store(hwnd, p)
	child := func(class, text string, style uintptr, id uintptr, l, t, width, height int) uintptr {
		c, s := winapi.UTF16(class), winapi.UTF16(text)
		control, _, _ := createWindow.Call(0, uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(s)), 0x40000000|0x10000000|style, uintptr(u.px(l)), uintptr(u.px(t)), uintptr(u.px(width)), uintptr(u.px(height)), hwnd, id, instance, 0)
		if control != 0 && u.font != 0 {
			sendMessage.Call(control, 0x30, u.font, 1)
		}
		return control
	}
	child("STATIC", "自定义股票 / ETF组合", 0, 0, 14, 10, 300, 24)
	child("STATIC", "股票1", 0, 0, 14, 44, 54, 22)
	p.first = child("EDIT", "", 0x800000|0x10000|0x80, 1, 72, 40, 250, 27)
	child("STATIC", "股票2", 0, 0, 14, 80, 54, 22)
	p.second = child("EDIT", "", 0x800000|0x10000|0x80, 2, 72, 76, 250, 27)
	p.status = child("STATIC", "沪深北6位代码 / 美股代码（如AAPL）\nEnter提交；点击外部关闭", 0, 0, 14, 113, 308, 48)
	p.submit = child("BUTTON", "添加并显示", 0x10000, 3, 116, 172, 118, 30)
	child("BUTTON", "取消", 0x10000, 4, 242, 172, 80, 30)
	if p.first == 0 || p.second == 0 || p.submit == 0 || p.status == 0 {
		u.closeCustomPanel()
		ShowError(fmt.Errorf("无法创建输入控件"))
		return
	}
	for _, edit := range []uintptr{p.first, p.second} {
		sendMessage.Call(edit, 0xc5, 10, 0) // EM_SETLIMITTEXT: also allow US share classes.
		original, _, _ := setWindowLongPtr.Call(edit, ^uintptr(3), editProcedure)
		panelEdits.Store(edit, editBinding{p, original})
	}
	if show {
		showWindow.Call(hwnd, 5)
		setForeground.Call(hwnd)
		setWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, 0x1|0x2|0x40)
		setFocus.Call(p.first)
		active, _, _ := panelForeground.Call()
		p.focusEstablished = active == hwnd
		panelSetTimer.Call(hwnd, 1, 50, 0)
	}
}

func customPanelProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	if value, ok := panelWindows.Load(hwnd); ok {
		p := value.(*customPanel)
		switch msg {
		case 0x0113: // WM_TIMER: also catch clicks on non-activating taskbar windows.
			active, _, _ := panelForeground.Call()
			if active == hwnd {
				p.focusEstablished = true
			}
			var cursor point
			getCursor.Call(uintptr(unsafe.Pointer(&cursor)))
			left, _, _ := getAsyncKeyState.Call(1)
			right, _, _ := getAsyncKeyState.Call(2)
			p.outsideClick(cursor, (left|right)&0x8000 != 0)
			return 0
		case 0x0006: // WM_ACTIVATE
			// Initial activation may fail while the native menu/shell is releasing focus.
			// Only a genuine foreground-to-background transition is a dismissal.
			if wp&0xffff == 0 && p.focusEstablished {
				p.u.closeCustomPanel()
			}
			return 0
		case 0x0010:
			p.u.closeCustomPanel()
			return 0
		case 0x0111:
			switch wp & 0xffff {
			case 3:
				p.u.submitCustomPair()
			case 4:
				p.u.closeCustomPanel()
			}
			return 0
		case 0x0100:
			if wp == 27 {
				p.u.closeCustomPanel()
				return 0
			}
			if wp == 13 {
				p.u.submitCustomPair()
				return 0
			}
		case 0x0082:
			panelWindows.Delete(hwnd)
		}
	}
	r, _, _ := defWindowProc.Call(hwnd, uintptr(msg), wp, lp)
	return r
}

func (p *customPanel) outsideClick(cursor point, pressed bool) {
	if !pressed {
		p.outsideArmed = true
		return
	}
	if !p.outsideArmed {
		return
	} // Ignore the click that opened the menu/panel.
	var bounds rect
	getWindowRect.Call(p.hwnd, uintptr(unsafe.Pointer(&bounds)))
	if cursor.X < bounds.Left || cursor.X >= bounds.Right || cursor.Y < bounds.Top || cursor.Y >= bounds.Bottom {
		p.u.closeCustomPanel()
	}
}

func customEditProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	value, ok := panelEdits.Load(hwnd)
	if !ok {
		r, _, _ := defWindowProc.Call(hwnd, uintptr(msg), wp, lp)
		return r
	}
	b := value.(editBinding)
	if msg == 0x0007 {
		active, _, _ := panelForeground.Call()
		if active == b.panel.hwnd {
			b.panel.focusEstablished = true
		}
	}
	if msg == 0x0102 && (wp == 13 || wp == 27 || wp == 9) {
		return 0
	}
	if msg == 0x0100 {
		switch wp {
		case 13:
			b.panel.u.submitCustomPair()
			return 0
		case 27:
			b.panel.u.closeCustomPanel()
			return 0
		case 9:
			next := b.panel.first
			if hwnd == b.panel.first {
				next = b.panel.second
			}
			setFocus.Call(next)
			return 0
		}
	}
	if msg == 0x0082 {
		panelEdits.Delete(hwnd)
	}
	r, _, _ := callWindowProc.Call(b.original, hwnd, uintptr(msg), wp, lp)
	return r
}
