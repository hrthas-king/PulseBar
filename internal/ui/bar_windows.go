//go:build windows && (amd64 || arm64)

package ui

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sync/atomic"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/hrthas-king/PulseBar/internal/app"
	"github.com/hrthas-king/PulseBar/internal/config"
	"github.com/hrthas-king/PulseBar/internal/market"
	"github.com/hrthas-king/PulseBar/internal/monitor"
	"github.com/hrthas-king/PulseBar/internal/winapi"
)

type Action uint32

const (
	ActionRefresh Action = 1001 + iota
	ActionToggleNetDetail
	ActionToggleLock
	ActionDockLeft
	ActionToggleColor
	ActionGroupVOOQQQ
	ActionGroupSOXXSMH
	ActionGroupTop2US
	ActionGroupCNIndices
	ActionGroupHKIndices
	ActionGroupAsiaIndices
	ActionGroupEUIndices
	ActionInterval1
	ActionInterval2
	ActionInterval5
	ActionDiagnostics
	ActionAutostart
	ActionExit
)

func GroupForAction(act Action) (string, bool) {
	if act < ActionGroupVOOQQQ || act > ActionGroupEUIndices {
		return "", false
	}
	idx := int(act - ActionGroupVOOQQQ)
	if idx < 0 || idx >= len(market.Groups) {
		return "", false
	}
	return market.Groups[idx].ID, true
}

const (
	wmTray   = 0x8001
	wmUpdate = 0x8002
)

const (
	slotWidthLogical    = 228
	compactWidthLogical = 98 + 2*slotWidthLogical
	detailWidthLogical  = 256 + 2*slotWidthLogical
	barHeightLogical    = 30
)

var (
	user32               = winapi.DLL("user32.dll")
	gdi32                = winapi.DLL("gdi32.dll")
	shell32              = winapi.DLL("shell32.dll")
	kernel32             = winapi.DLL("kernel32.dll")
	comctl32             = winapi.DLL("comctl32.dll")
	registerClass        = user32.NewProc("RegisterClassExW")
	unregisterClass      = user32.NewProc("UnregisterClassW")
	createWindow         = user32.NewProc("CreateWindowExW")
	defWindowProc        = user32.NewProc("DefWindowProcW")
	destroyWindow        = user32.NewProc("DestroyWindow")
	showWindow           = user32.NewProc("ShowWindow")
	setWindowPos         = user32.NewProc("SetWindowPos")
	setLayeredAttrs      = user32.NewProc("SetLayeredWindowAttributes")
	getWindowLongPtr     = user32.NewProc("GetWindowLongPtrW")
	setWindowLongPtr     = user32.NewProc("SetWindowLongPtrW")
	getMessage           = user32.NewProc("GetMessageW")
	translateMessage     = user32.NewProc("TranslateMessage")
	dispatchMessage      = user32.NewProc("DispatchMessageW")
	postMessage          = user32.NewProc("PostMessageW")
	sendMessage          = user32.NewProc("SendMessageW")
	postQuit             = user32.NewProc("PostQuitMessage")
	registerMessage      = user32.NewProc("RegisterWindowMessageW")
	findWindow           = user32.NewProc("FindWindowW")
	getWindowRect        = user32.NewProc("GetWindowRect")
	getCursor            = user32.NewProc("GetCursorPos")
	setCapture           = user32.NewProc("SetCapture")
	releaseCapture       = user32.NewProc("ReleaseCapture")
	trackMouseEvent      = user32.NewProc("TrackMouseEvent")
	getSystemMetrics     = user32.NewProc("GetSystemMetrics")
	getDpiForSystem      = user32.NewProc("GetDpiForSystem")
	monitorFromRect      = user32.NewProc("MonitorFromRect")
	invalidateRect       = user32.NewProc("InvalidateRect")
	beginPaint           = user32.NewProc("BeginPaint")
	endPaint             = user32.NewProc("EndPaint")
	fillRect             = user32.NewProc("FillRect")
	drawText             = user32.NewProc("DrawTextW")
	loadCursor           = user32.NewProc("LoadCursorW")
	createIcon           = user32.NewProc("CreateIcon")
	destroyIcon          = user32.NewProc("DestroyIcon")
	createMenu           = user32.NewProc("CreatePopupMenu")
	appendMenu           = user32.NewProc("AppendMenuW")
	destroyMenu          = user32.NewProc("DestroyMenu")
	setForeground        = user32.NewProc("SetForegroundWindow")
	trackMenu            = user32.NewProc("TrackPopupMenu")
	setDPI               = user32.NewProc("SetProcessDpiAwarenessContext")
	messageBox           = user32.NewProc("MessageBoxW")
	getModule            = kernel32.NewProc("GetModuleHandleW")
	rtlMoveMemory        = kernel32.NewProc("RtlMoveMemory")
	shellNotify          = shell32.NewProc("Shell_NotifyIconW")
	createSolidBrush     = gdi32.NewProc("CreateSolidBrush")
	createFont           = gdi32.NewProc("CreateFontW")
	selectObject         = gdi32.NewProc("SelectObject")
	deleteObject         = gdi32.NewProc("DeleteObject")
	setTextColor         = gdi32.NewProc("SetTextColor")
	setBkMode            = gdi32.NewProc("SetBkMode")
	ellipse              = gdi32.NewProc("Ellipse")
	initCommonControlsEx = comctl32.NewProc("InitCommonControlsEx")
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }

type message struct {
	Window         uintptr
	Message        uint32
	_              uint32
	WParam, LParam uintptr
	Time           uint32
	Point          point
	_              uint32
}

type windowClass struct {
	Size, Style                        uint32
	Procedure                          uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	SmallIcon                          uintptr
}

type paintStruct struct {
	HDC         uintptr
	Erase       int32
	Paint       rect
	Restore     int32
	IncUpdate   int32
	RGBReserved [32]byte
	_           [4]byte
}

// notifyIcon 对应 64 位 Windows NOTIFYICONDATAW（976 字节）。
type notifyIcon struct {
	Size                uint32
	_                   uint32
	Window              uintptr
	ID, Flags, Callback uint32
	_                   uint32
	Icon                uintptr
	Tip                 [128]uint16
	State, StateMask    uint32
	Info                [256]uint16
	Version             uint32
	InfoTitle           [64]uint16
	InfoFlags           uint32
	GUID                [16]byte
	BalloonIcon         uintptr
}

// toolInfo 对应 64 位 Windows TTTOOLINFOW（72 字节）。
type toolInfo struct {
	Size     uint32
	Flags    uint32
	Hwnd     uintptr
	ID       uintptr
	Rect     rect
	Hinst    uintptr
	Text     *uint16
	LParam   uintptr
	Reserved uintptr
}

type trackMouse struct {
	Size      uint32
	Flags     uint32
	HwndTrack uintptr
	HoverTime uint32
	_         uint32
}

func colorRef(r, g, b byte) uintptr {
	return uintptr(uint32(r) | (uint32(g) << 8) | (uint32(b) << 16))
}

type UI struct {
	version        string
	hwnd           uintptr
	tooltipHwnd    uintptr
	tool           toolInfo
	tooltipBuf     [2048]uint16
	instance       uintptr
	scale          float64
	width, height  int32
	bgBrush        uintptr
	brushGreen     uintptr
	brushBlue      uintptr
	brushYellow    uintptr
	brushRed       uintptr
	brushGray      uintptr
	font           uintptr
	data           notifyIcon
	icons          map[string]uintptr
	iconCountry    string
	trayAdded      bool
	taskbarMessage uint32
	latest         atomic.Pointer[app.Snapshot]
	updatePending  atomic.Bool
	startup        atomic.Bool
	cfg            config.Config
	dragging       bool
	dragOffsetX    int32
	dragOffsetY    int32
	hoverTracking  bool
	tooltipVisible bool
	Actions        chan Action
	callback       uintptr
}

func NewUI(version string, cfg config.Config) *UI {
	return &UI{
		version: version,
		cfg:     cfg,
		scale:   1.0,
		icons:   make(map[string]uintptr),
		Actions: make(chan Action, 16),
	}
}

func (u *UI) SetAutostart(enabled bool) { u.startup.Store(enabled) }
func (u *UI) Config() config.Config     { return u.cfg }

func (u *UI) Publish(s app.Snapshot) {
	u.latest.Store(&s)
	if !u.updatePending.Swap(true) {
		ok, _, _ := postMessage.Call(u.hwnd, wmUpdate, 0, 0)
		if ok == 0 {
			u.updatePending.Store(false)
		}
	}
}

func (u *UI) px(v int) int32 {
	return int32(math.Round(float64(v) * u.scale))
}

func (u *UI) updateDimensions() {
	w := compactWidthLogical
	if u.cfg.ShowNetDetail {
		w = detailWidthLogical
	}
	u.width = u.px(w)
	u.height = u.px(barHeightLogical)
}

func (u *UI) initialPosition() (int32, int32) {
	if u.cfg.Placement == "manual" && u.cfg.X != nil && u.cfg.Y != nil {
		x, y := int32(*u.cfg.X), int32(*u.cfg.Y)
		rc := rect{Left: x, Top: y, Right: x + u.width, Bottom: y + u.height}
		if mon, _, _ := monitorFromRect.Call(uintptr(unsafe.Pointer(&rc)), 0); mon != 0 {
			return x, y
		}
	}
	shellTray := winapi.UTF16("Shell_TrayWnd")
	taskbar, _, _ := findWindow.Call(uintptr(unsafe.Pointer(shellTray)), 0)
	var rc rect
	if taskbar != 0 {
		if ok, _, _ := getWindowRect.Call(taskbar, uintptr(unsafe.Pointer(&rc))); ok != 0 {
			top := rc.Top + max(0, (rc.Bottom-rc.Top-u.height)/2)
			return rc.Left + u.px(8), top
		}
	}
	screenH, _, _ := getSystemMetrics.Call(1)
	return u.px(8), max(0, int32(screenH)-u.height-u.px(12))
}

func copyText(dst []uint16, text string) {
	clear(dst)
	enc := utf16.Encode([]rune(text))
	if len(enc) >= len(dst) {
		enc = enc[:len(dst)-1]
		if len(enc) > 0 && enc[len(enc)-1] >= 0xd800 && enc[len(enc)-1] <= 0xdbff {
			enc = enc[:len(enc)-1]
		}
	}
	copy(dst, enc)
}

func (u *UI) callNotify(op uintptr) bool {
	ok, _, _ := shellNotify.Call(op, uintptr(unsafe.Pointer(&u.data)))
	return ok != 0
}

func (u *UI) addTrayIcon() bool {
	u.data.Flags = 1 | 2 | 4
	if !u.callNotify(0) {
		u.trayAdded = false
		return false
	}
	u.data.Version = 4
	if u.callNotify(4) {
		u.data.Flags = 1 | 2 | 4 | 0x80
		u.callNotify(1)
	}
	u.trayAdded = true
	return true
}

func (u *UI) initTooltip() {
	var icc struct {
		Size, Classes uint32
	}
	icc.Size = 8
	icc.Classes = 8
	initCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))

	cls := winapi.UTF16("tooltips_class32")
	u.tooltipHwnd, _, _ = createWindow.Call(
		0x8|0x80|0x08000000,
		uintptr(unsafe.Pointer(cls)),
		0,
		0x80000000|3,
		0, 0, 0, 0,
		u.hwnd, 0, u.instance, 0,
	)
	if u.tooltipHwnd == 0 {
		return
	}
	u.tool = toolInfo{
		Size:  uint32(unsafe.Sizeof(toolInfo{})),
		Flags: 0x20 | 0x80,
		Hwnd:  u.hwnd,
		ID:    1,
		Text:  &u.tooltipBuf[0],
	}
	sendMessage.Call(u.tooltipHwnd, 0x432, 0, uintptr(unsafe.Pointer(&u.tool))) // TTM_ADDTOOLW
	sendMessage.Call(u.tooltipHwnd, 0x418, 0, uintptr(u.px(680)))               // TTM_SETMAXTIPWIDTH
}

func (u *UI) showTooltip() {
	if u.tooltipHwnd == 0 {
		return
	}
	s := u.latest.Load()
	if s == nil {
		return
	}
	copyText(u.tooltipBuf[:], OverlayTooltip(*s))
	sendMessage.Call(u.tooltipHwnd, 0x439, 0, uintptr(unsafe.Pointer(&u.tool))) // TTM_UPDATETIPTEXTW
	var pt point
	getCursor.Call(uintptr(unsafe.Pointer(&pt)))
	x := int(pt.X)
	y := int(pt.Y - u.px(130))
	lparam := uintptr(uint32(uint16(x)) | (uint32(uint16(y)) << 16))
	sendMessage.Call(u.tooltipHwnd, 0x412, 0, lparam)                           // TTM_TRACKPOSITION
	sendMessage.Call(u.tooltipHwnd, 0x411, 1, uintptr(unsafe.Pointer(&u.tool))) // TTM_TRACKACTIVATE
	u.tooltipVisible = true
}

func (u *UI) hideTooltip() {
	if u.tooltipHwnd != 0 && u.tooltipVisible {
		sendMessage.Call(u.tooltipHwnd, 0x411, 0, uintptr(unsafe.Pointer(&u.tool)))
	}
	u.tooltipVisible = false
}

func (u *UI) update(s app.Snapshot) error {
	country := Country(s)
	if u.iconCountry != country {
		old := u.icons
		u.icons = make(map[string]uintptr)
		u.iconCountry = country
		defer func() {
			for _, h := range old {
				destroyIcon.Call(h)
			}
		}()
	}
	key := IconKey(s)
	icon := u.icons[key]
	if icon == 0 {
		pixels := Pixels(country, s.Status, s.Rates.Up > 0, s.Rates.Down > 0, s.At.Unix()%2 == 0)
		var xor [4096]byte
		var and [128]byte
		for y := 0; y < 32; y++ {
			for x := 0; x < 32; x++ {
				src := (y*32 + x) * 4
				dst := (y*32 + x) * 4
				xor[dst], xor[dst+1], xor[dst+2], xor[dst+3] = pixels[src+2], pixels[src+1], pixels[src], pixels[src+3]
				if pixels[src+3] == 0 {
					and[y*4+x/8] |= 1 << uint(7-x%8)
				}
			}
		}
		var err error
		icon, _, err = createIcon.Call(u.instance, 32, 32, 1, 32, uintptr(unsafe.Pointer(&and[0])), uintptr(unsafe.Pointer(&xor[0])))
		if icon == 0 {
			return winapi.Error("CreateIcon", err)
		}
		u.icons[key] = icon
	}
	u.data.Icon = icon
	copyText(u.data.Tip[:], TrayTooltip(s))
	if u.trayAdded {
		if !u.callNotify(1) {
			u.addTrayIcon()
		}
	} else {
		u.addTrayIcon()
	}
	if u.hwnd != 0 {
		invalidateRect.Call(u.hwnd, 0, 0)
	}
	if u.tooltipVisible {
		u.showTooltip()
	}
	return nil
}

func (u *UI) drawTextSegment(dc uintptr, text string, leftLogical, rightLogical int, color uintptr) {
	setTextColor.Call(dc, color)
	rc := rect{Left: u.px(leftLogical), Top: 0, Right: u.px(rightLogical), Bottom: u.height}
	ptr := winapi.UTF16(text)
	drawText.Call(dc, uintptr(unsafe.Pointer(ptr)), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), 0x20|0x4|0x8000)
	runtime.KeepAlive(ptr)
}

func (u *UI) drawTextCentered(dc uintptr, text string, leftLogical, rightLogical int, color uintptr) {
	setTextColor.Call(dc, color)
	rc := rect{Left: u.px(leftLogical), Top: 0, Right: u.px(rightLogical), Bottom: u.height}
	ptr := winapi.UTF16(text)
	drawText.Call(dc, uintptr(unsafe.Pointer(ptr)), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), 0x20|0x4|0x1|0x8000)
	runtime.KeepAlive(ptr)
}

func (u *UI) drawCircle(dc uintptr, brush uintptr, leftLogical, topLogical, sizeLogical int) {
	old, _, _ := selectObject.Call(dc, brush)
	ellipse.Call(
		dc,
		uintptr(u.px(leftLogical)),
		uintptr(u.px(topLogical)),
		uintptr(u.px(leftLogical+sizeLogical)),
		uintptr(u.px(topLogical+sizeLogical)),
	)
	selectObject.Call(dc, old)
}

func (u *UI) paint(hwnd uintptr) {
	var ps paintStruct
	dc, _, _ := beginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if dc == 0 {
		return
	}
	defer endPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	rc := rect{Left: 0, Top: 0, Right: u.width, Bottom: u.height}
	fillRect.Call(dc, uintptr(unsafe.Pointer(&rc)), u.bgBrush)

	oldFont, _, _ := selectObject.Call(dc, u.font)
	defer selectObject.Call(dc, oldFont)
	setBkMode.Call(dc, 1) // TRANSPARENT

	var snap app.Snapshot
	if p := u.latest.Load(); p != nil {
		snap = *p
	} else {
		snap = app.Snapshot{Status: monitor.Unknown}
	}

	// 1. 绘制 GeoPulse 段
	netBrush := u.brushGray
	switch snap.Status {
	case monitor.Online:
		netBrush = u.brushGreen
	case monitor.Degraded:
		netBrush = u.brushYellow
	case monitor.Offline:
		netBrush = u.brushRed
	}
	u.drawCircle(dc, netBrush, 6, 11, 8)
	u.drawTextSegment(dc, Country(snap), 19, 48, colorRef(240, 246, 255))

	quoteOffset := 98
	if !u.cfg.ShowNetDetail {
		upColor := colorRef(100, 116, 139)
		if snap.Rates.Up > 0 {
			upColor = colorRef(0, 229, 255)
		}
		downColor := colorRef(100, 116, 139)
		if snap.Rates.Down > 0 {
			downColor = colorRef(56, 189, 248)
		}
		u.drawTextSegment(dc, "↑", 50, 66, upColor)
		u.drawTextSegment(dc, "↓", 66, 84, downColor)
		u.drawTextSegment(dc, "|", 86, 96, colorRef(120, 130, 145))
	} else {
		upStr, downStr := "--", "--"
		if snap.Rates.Valid {
			upStr, downStr = CompactSpeed(snap.Rates.Up), CompactSpeed(snap.Rates.Down)
		}
		rateColor := colorRef(148, 163, 184)
		if snap.Rates.Up > 0 || snap.Rates.Down > 0 {
			rateColor = colorRef(125, 211, 252)
		}
		u.drawTextSegment(dc, fmt.Sprintf("↑%s ↓%s", upStr, downStr), 50, 184, rateColor)
		rttColor := colorRef(148, 163, 184)
		if snap.Probe.Success {
			rttColor = colorRef(167, 243, 208)
		}
		u.drawTextSegment(dc, CompactRTT(snap), 186, 242, rttColor)
		u.drawTextSegment(dc, "|", 244, 254, colorRef(120, 130, 145))
		quoteOffset = 256
	}

	// 2. 绘制多市场行情段（五态时段信号 + 异常感叹号 + 简洁别名 + 自适应价格与涨跌幅）
	views := snap.Quotes
	if len(views) == 0 {
		g := market.GroupByID(u.cfg.MarketGroup)
		for _, sym := range g.Symbols {
			views = append(views, market.View{
				Symbol:  sym,
				Alias:   market.AliasFor(sym),
				State:   market.StateClosed,
				Healthy: false,
			})
		}
	}
	for i, v := range views {
		base := quoteOffset + i*slotWidthLogical
		switch v.EffectiveState() {
		case market.StatePreMarket:
			u.drawTextCentered(dc, "☼", base+2, base+18, colorRef(255, 193, 7))
		case market.StateOvernight:
			u.drawTextCentered(dc, "☾", base+2, base+18, colorRef(226, 232, 240))
		case market.StateRegular:
			u.drawCircle(dc, u.brushGreen, base+6, 11, 8)
		case market.StatePostMarket:
			u.drawCircle(dc, u.brushBlue, base+6, 11, 8)
		default:
			u.drawCircle(dc, u.brushGray, base+6, 11, 8)
		}

		aliasLeft := base + 19
		if !v.Healthy {
			u.drawTextSegment(dc, "!", base+17, base+25, colorRef(255, 82, 82))
			aliasLeft = base + 25
		}

		alias := v.DisplayName()
		aliasW := 42
		if len(alias) >= 5 {
			aliasW = 48
		}
		u.drawTextSegment(dc, alias, aliasLeft, aliasLeft+aliasW, colorRef(240, 240, 240))

		text := "---.-- --.--%"
		pctColor := colorRef(170, 170, 170)
		if v.Quote != nil {
			pct := v.Quote.ChangePercent
			text = fmt.Sprintf("%.2f %+.2f%%", v.Quote.Price, pct)
			if math.Abs(pct) < 0.005 {
				pctColor = colorRef(240, 240, 240)
			} else if (pct > 0) == u.cfg.UpIsRed {
				pctColor = colorRef(255, 119, 119)
			} else {
				pctColor = colorRef(119, 204, 153)
			}
		}
		u.drawTextSegment(dc, text, aliasLeft+aliasW+2, base+slotWidthLogical-12, pctColor)
		if i == 0 && len(views) > 1 {
			u.drawTextSegment(dc, "|", base+slotWidthLogical-11, base+slotWidthLogical-1, colorRef(120, 130, 145))
		}
	}
}

func (u *UI) applyLockStyle() {
	style, _, _ := getWindowLongPtr.Call(u.hwnd, ^uintptr(19)) // GWL_EXSTYLE = -20
	if u.cfg.Locked {
		style |= 0x20 // WS_EX_TRANSPARENT
	} else {
		style &^= 0x20
	}
	setWindowLongPtr.Call(u.hwnd, ^uintptr(19), style)
	setWindowPos.Call(u.hwnd, ^uintptr(0), 0, 0, 0, 0, 0x1|0x2|0x10|0x20)
}

func (u *UI) dockLeft() {
	u.cfg.Placement = "left"
	u.cfg.X = nil
	u.cfg.Y = nil
	x, y := u.initialPosition()
	setWindowPos.Call(u.hwnd, ^uintptr(0), uintptr(x), uintptr(y), uintptr(u.width), uintptr(u.height), 0x10)
	_ = config.Save(u.cfg)
}

func (u *UI) showDiagnostics() {
	var snap app.Snapshot
	if p := u.latest.Load(); p != nil {
		snap = *p
	}
	report := DiagnosticsText(u.version, snap)
	text, title := winapi.UTF16(report), winapi.UTF16("PulseBar — 网络与 VOO/QQQ 24h 行情诊断")
	messageBox.Call(u.hwnd, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x40)
}

func (u *UI) menu() {
	menu, _, _ := createMenu.Call()
	if menu == 0 {
		return
	}
	defer destroyMenu.Call(menu)

	add := func(id Action, label string, flags uintptr) {
		p := winapi.UTF16(label)
		appendMenu.Call(menu, flags, uintptr(id), uintptr(unsafe.Pointer(p)))
	}

	s := u.latest.Load()
	if s != nil {
		ip := s.Geo.IP
		if ip == "" {
			ip = "查询中/未知"
		}
		add(0, fmt.Sprintf("PulseBar [%s] %s | IP: %s", Country(*s), StatusText(s.Status), ip), 2)
		up, down := "--", "--"
		if s.Rates.Valid {
			up, down = Speed(s.Rates.Up), Speed(s.Rates.Down)
		}
		add(0, fmt.Sprintf("网络: ↑ %s  ↓ %s | RTT: %s", up, down, RTT(*s)), 2)
		add(0, "行情: "+QuoteSummary(*s), 2)
		if s.EgressActive {
			add(0, "TUN/代理活跃：地区每 30 秒自动复核", 2)
		}
	}
	add(0, "", 0x800)
	add(ActionRefresh, "立即刷新 IP 与行情", 0)

	detailFlags := uintptr(0)
	if u.cfg.ShowNetDetail {
		detailFlags = 8
	}
	add(ActionToggleNetDetail, "任务栏显示实时网速与延迟", detailFlags)

	lockLabel := "锁定位置（鼠标穿透）"
	if u.cfg.Locked {
		lockLabel = "解锁位置（允许拖动）"
	}
	add(ActionToggleLock, lockLabel, 0)
	add(ActionDockLeft, "移到任务栏最左侧", 0)

	colorLabel := "涨跌配色：绿涨红跌（点击切换）"
	if u.cfg.UpIsRed {
		colorLabel = "涨跌配色：红涨绿跌（点击切换）"
	}
	add(ActionToggleColor, colorLabel, 0)

	add(0, "", 0x800)
	currentGroup := u.cfg.MarketGroup
	if s != nil && market.ValidGroup(s.MarketGroup) {
		currentGroup = s.MarketGroup
	}
	if !market.ValidGroup(currentGroup) {
		currentGroup = market.DefaultGroup
	}
	for i, g := range market.Groups {
		flags := uintptr(0)
		if g.ID == currentGroup {
			flags = 8
		}
		add(Action(uint32(ActionGroupVOOQQQ)+uint32(i)), g.MenuLabel, flags)
	}

	add(0, "", 0x800)
	interval := u.cfg.IntervalSeconds
	if s != nil && app.ValidInterval(s.IntervalSeconds) {
		interval = s.IntervalSeconds
	}
	for i, n := range []int{1, 2, 5} {
		flags := uintptr(0)
		if interval == n {
			flags = 8
		}
		add(Action(uint32(ActionInterval1)+uint32(i)), fmt.Sprintf("检测/显示间隔 %ds（心跳固定 1s）", n), flags)
	}
	add(0, "", 0x800)
	add(ActionDiagnostics, "查看详细诊断信息", 0)

	autoFlags := uintptr(0)
	if u.startup.Load() {
		autoFlags = 8
	}
	add(ActionAutostart, "开机自启", autoFlags)
	add(0, "", 0x800)
	add(ActionExit, "退出", 0)

	var p point
	getCursor.Call(uintptr(unsafe.Pointer(&p)))
	setForeground.Call(u.hwnd)
	cmd, _, _ := trackMenu.Call(menu, 0x100|2, uintptr(p.X), uintptr(p.Y), 0, u.hwnd, 0)
	postMessage.Call(u.hwnd, 0, 0, 0)
	u.callNotify(3)
	if cmd != 0 {
		u.handleAction(Action(cmd))
	}
}

func (u *UI) handleAction(act Action) {
	switch act {
	case ActionToggleNetDetail:
		u.cfg.ShowNetDetail = !u.cfg.ShowNetDetail
		u.updateDimensions()
		var rc rect
		getWindowRect.Call(u.hwnd, uintptr(unsafe.Pointer(&rc)))
		setWindowPos.Call(u.hwnd, ^uintptr(0), uintptr(rc.Left), uintptr(rc.Top), uintptr(u.width), uintptr(u.height), 0x10)
		invalidateRect.Call(u.hwnd, 0, 0)
		_ = config.Save(u.cfg)
	case ActionToggleLock:
		u.hideTooltip()
		u.cfg.Locked = !u.cfg.Locked
		u.applyLockStyle()
		_ = config.Save(u.cfg)
	case ActionDockLeft:
		u.dockLeft()
	case ActionToggleColor:
		u.cfg.UpIsRed = !u.cfg.UpIsRed
		invalidateRect.Call(u.hwnd, 0, 0)
		_ = config.Save(u.cfg)
	case ActionDiagnostics:
		u.showDiagnostics()
	default:
		if grp, ok := GroupForAction(act); ok {
			u.cfg.MarketGroup = grp
			_ = config.Save(u.cfg)
			spec := market.SetActiveGroup(grp)
			if s := u.latest.Load(); s != nil {
				snap := *s
				snap.MarketGroup = spec.ID
				snap.Quotes = make([]market.View, 0, len(spec.Symbols))
				for _, sym := range spec.Symbols {
					snap.Quotes = append(snap.Quotes, market.View{
						Symbol:  sym,
						Alias:   market.AliasFor(sym),
						State:   market.StateClosed,
						Healthy: false,
						Reason:  "等待 " + market.AliasFor(sym) + " 实时行情",
					})
				}
				u.latest.Store(&snap)
				invalidateRect.Call(u.hwnd, 0, 0)
			}
		} else if act == ActionInterval1 || act == ActionInterval2 || act == ActionInterval5 {
			n := map[Action]int{ActionInterval1: 1, ActionInterval2: 2, ActionInterval5: 5}[act]
			u.cfg.IntervalSeconds = n
			_ = config.Save(u.cfg)
		}
		select {
		case u.Actions <- act:
		default:
		}
	}
}

func (u *UI) windowProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	if msg == u.taskbarMessage && u.taskbarMessage != 0 {
		u.addTrayIcon()
		return 0
	}
	switch msg {
	case 0x000F: // WM_PAINT
		u.paint(hwnd)
		return 0
	case 0x0014: // WM_ERASEBKGND
		return 1
	case wmUpdate:
		u.updatePending.Store(false)
		if s := u.latest.Load(); s != nil {
			_ = u.update(*s)
		}
		return 0
	case wmTray:
		event := uint32(lparam & 0xffff)
		if event == 0x7b || event == 0x205 || event == 0x400 || event == 0x401 {
			u.menu()
		} else if event == 0x203 {
			u.showDiagnostics()
		}
		return 0
	case winapi.WMDockLeft:
		u.dockLeft()
		return 0
	case winapi.WMDiag:
		u.showDiagnostics()
		return 0
	case 0x0205: // WM_RBUTTONUP on taskbar bar
		u.menu()
		return 0
	case 0x0201: // WM_LBUTTONDOWN
		if !u.cfg.Locked {
			u.hideTooltip()
			var pt point
			var rc rect
			getCursor.Call(uintptr(unsafe.Pointer(&pt)))
			getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
			u.dragOffsetX = pt.X - rc.Left
			u.dragOffsetY = pt.Y - rc.Top
			u.dragging = true
			setCapture.Call(hwnd)
		}
		return 0
	case 0x0200: // WM_MOUSEMOVE
		if u.dragging {
			var pt point
			getCursor.Call(uintptr(unsafe.Pointer(&pt)))
			setWindowPos.Call(hwnd, ^uintptr(0), uintptr(pt.X-u.dragOffsetX), uintptr(pt.Y-u.dragOffsetY), 0, 0, 0x1|0x10)
			return 0
		}
		if !u.hoverTracking && !u.cfg.Locked {
			tm := trackMouse{
				Size:      uint32(unsafe.Sizeof(trackMouse{})),
				Flags:     1 | 2, // TME_HOVER | TME_LEAVE
				HwndTrack: hwnd,
				HoverTime: 400,
			}
			trackMouseEvent.Call(uintptr(unsafe.Pointer(&tm)))
			u.hoverTracking = true
		}
		return 0
	case 0x02A1: // WM_MOUSEHOVER
		if !u.dragging {
			u.showTooltip()
		}
		return 0
	case 0x02A3: // WM_MOUSELEAVE
		u.hoverTracking = false
		u.hideTooltip()
		return 0
	case 0x0202: // WM_LBUTTONUP
		if u.dragging {
			u.dragging = false
			releaseCapture.Call()
			var rc rect
			getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
			x, y := int(rc.Left), int(rc.Top)
			u.cfg.Placement = "manual"
			u.cfg.X = &x
			u.cfg.Y = &y
			_ = config.Save(u.cfg)
		}
		return 0
	case 0x0215: // WM_CAPTURECHANGED
		u.dragging = false
		return 0
	case 0x007E: // WM_DISPLAYCHANGE
		x, y := u.initialPosition()
		setWindowPos.Call(hwnd, ^uintptr(0), uintptr(x), uintptr(y), 0, 0, 0x1|0x10)
		return 0
	case 0x02E0: // WM_DPICHANGED
		dpi := uint32(wparam & 0xffff)
		if dpi > 0 {
			u.scale = float64(dpi) / 96.0
			u.updateDimensions()
			if u.font != 0 {
				deleteObject.Call(u.font)
			}
			fontName := winapi.UTF16("Segoe UI")
			u.font, _, _ = createFont.Call(uintptr(int(-u.px(14))), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 3, 0, uintptr(unsafe.Pointer(fontName)))
			if lparam != 0 {
				var suggested rect
				rtlMoveMemory.Call(uintptr(unsafe.Pointer(&suggested)), lparam, unsafe.Sizeof(suggested))
				setWindowPos.Call(hwnd, ^uintptr(0), uintptr(suggested.Left), uintptr(suggested.Top), uintptr(u.width), uintptr(u.height), 0x10)
			}
			invalidateRect.Call(hwnd, 0, 0)
		}
		return 0
	case 0x0010: // WM_CLOSE
		destroyWindow.Call(hwnd)
		return 0
	case 0x0002: // WM_DESTROY
		postQuit.Call(0)
		return 0
	}
	r, _, _ := defWindowProc.Call(hwnd, uintptr(msg), wparam, lparam)
	return r
}

func (u *UI) Run(ctx context.Context, ready func()) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if setDPI.Find() == nil {
		setDPI.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
	}
	if getDpiForSystem.Find() == nil {
		if dpi, _, _ := getDpiForSystem.Call(); dpi > 0 {
			u.scale = float64(dpi) / 96.0
		}
	}
	u.updateDimensions()

	u.instance, _, _ = getModule.Call(0)
	u.bgBrush, _, _ = createSolidBrush.Call(colorRef(1, 1, 1))
	u.brushGreen, _, _ = createSolidBrush.Call(colorRef(0, 230, 118))
	u.brushBlue, _, _ = createSolidBrush.Call(colorRef(56, 189, 248))
	u.brushYellow, _, _ = createSolidBrush.Call(colorRef(245, 158, 11))
	u.brushRed, _, _ = createSolidBrush.Call(colorRef(255, 23, 68))
	u.brushGray, _, _ = createSolidBrush.Call(colorRef(148, 163, 184))
	fontName := winapi.UTF16("Segoe UI")
	u.font, _, _ = createFont.Call(uintptr(int(-u.px(14))), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 3, 0, uintptr(unsafe.Pointer(fontName)))

	defer func() {
		for _, obj := range []uintptr{u.font, u.bgBrush, u.brushGreen, u.brushBlue, u.brushYellow, u.brushRed, u.brushGray} {
			if obj != 0 {
				deleteObject.Call(obj)
			}
		}
	}()

	cursor, _, _ := loadCursor.Call(0, 32512)
	u.callback = syscall.NewCallback(u.windowProc)
	clsName := winapi.UTF16(winapi.WindowClass)
	wc := windowClass{
		Size:       uint32(unsafe.Sizeof(windowClass{})),
		Procedure:  u.callback,
		Instance:   u.instance,
		Cursor:     cursor,
		Background: u.bgBrush,
		ClassName:  clsName,
	}
	if atom, _, err := registerClass.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return winapi.Error("RegisterClassExW", err)
	}
	defer unregisterClass.Call(uintptr(unsafe.Pointer(clsName)), u.instance)

	x, y := u.initialPosition()
	exStyle := uintptr(0x00080000 | 0x80 | 0x8 | 0x08000000) // WS_EX_LAYERED | WS_EX_TOOLWINDOW | WS_EX_TOPMOST | WS_EX_NOACTIVATE
	if u.cfg.Locked {
		exStyle |= 0x20 // WS_EX_TRANSPARENT
	}
	title := winapi.UTF16("PulseBar")
	hwnd, _, err := createWindow.Call(
		exStyle,
		uintptr(unsafe.Pointer(clsName)),
		uintptr(unsafe.Pointer(title)),
		0x80000000, // WS_POPUP
		uintptr(x), uintptr(y), uintptr(u.width), uintptr(u.height),
		0, 0, u.instance, 0,
	)
	if hwnd == 0 {
		return winapi.Error("CreateWindowExW", err)
	}
	u.hwnd = hwnd
	defer destroyWindow.Call(hwnd)

	setLayeredAttrs.Call(hwnd, colorRef(1, 1, 1), 255, 1) // LWA_COLORKEY = 1

	taskbar := winapi.UTF16("TaskbarCreated")
	r, _, _ := registerMessage.Call(uintptr(unsafe.Pointer(taskbar)))
	u.taskbarMessage = uint32(r)

	u.data = notifyIcon{
		Size:     uint32(unsafe.Sizeof(notifyIcon{})),
		Window:   hwnd,
		ID:       1,
		Flags:    1 | 2 | 4 | 0x80,
		Callback: wmTray,
	}
	defer func() {
		u.callNotify(2)
		for _, icon := range u.icons {
			destroyIcon.Call(icon)
		}
	}()

	if err := u.update(app.Snapshot{Status: monitor.Unknown}); err != nil {
		return err
	}
	u.initTooltip()
	showWindow.Call(hwnd, 4) // SW_SHOWNOACTIVATE

	stop := context.AfterFunc(ctx, func() { postMessage.Call(hwnd, 0x0010, 0, 0) })
	defer stop()
	ready()

	var msg message
	for {
		r, _, err := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) == -1 {
			return winapi.Error("GetMessageW", err)
		}
		if r == 0 {
			return nil
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func ShowError(err error) {
	text, title := winapi.UTF16(err.Error()), winapi.UTF16("PulseBar")
	messageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}
