//go:build windows

// Package desktop adds an EarTrumpet-style Windows tray experience on top of
// the local server: a dynamic tray icon reflecting the Sonar master level, a
// borderless WebView2 "flyout" hosting the compact mixer (/flyout), and a
// global hotkey — all driven by a single Win32 message loop on the main OS
// thread. It reuses the running HTTP server; nothing here talks to Sonar
// directly.
package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

// ── Win32 plumbing ──────────────────────────────────────────────────────────

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	pRegisterClassExW     = user32.NewProc("RegisterClassExW")
	pCreateWindowExW      = user32.NewProc("CreateWindowExW")
	pDefWindowProcW       = user32.NewProc("DefWindowProcW")
	pGetMessageW          = user32.NewProc("GetMessageW")
	pTranslateMessage     = user32.NewProc("TranslateMessage")
	pDispatchMessageW     = user32.NewProc("DispatchMessageW")
	pPostQuitMessage      = user32.NewProc("PostQuitMessage")
	pShowWindow           = user32.NewProc("ShowWindow")
	pSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	pSetWindowPos         = user32.NewProc("SetWindowPos")
	pGetClientRect        = user32.NewProc("GetClientRect")
	pLoadCursorW          = user32.NewProc("LoadCursorW")
	pLoadImageW           = user32.NewProc("LoadImageW")
	pRegisterHotKey       = user32.NewProc("RegisterHotKey")
	pCreatePopupMenu      = user32.NewProc("CreatePopupMenu")
	pAppendMenuW          = user32.NewProc("AppendMenuW")
	pTrackPopupMenu       = user32.NewProc("TrackPopupMenu")
	pDestroyMenu          = user32.NewProc("DestroyMenu")
	pPostMessageW         = user32.NewProc("PostMessageW")
	pSystemParametersInfo = user32.NewProc("SystemParametersInfoW")
	pCreateIconIndirect   = user32.NewProc("CreateIconIndirect")
	pDestroyIcon          = user32.NewProc("DestroyIcon")

	pCreateDIBSection = gdi32.NewProc("CreateDIBSection")
	pCreateBitmap     = gdi32.NewProc("CreateBitmap")
	pDeleteObject     = gdi32.NewProc("DeleteObject")
	pCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")

	pShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	pShellExecuteW    = shell32.NewProc("ShellExecuteW")

	pDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
	pGetModuleHandleW      = kernel32.NewProc("GetModuleHandleW")
)

const (
	wmDestroy     = 0x0002
	wmSize        = 0x0005
	wmActivate    = 0x0006
	wmCommand     = 0x0111
	wmHotkey      = 0x0312
	wmContextMenu = 0x007B
	wmLButtonUp   = 0x0202
	wmRButtonUp   = 0x0205
	wmApp         = 0x8000
	wmUser        = 0x0400

	wmTray           = wmApp + 1 // tray icon callback
	wmUpdateIcon     = wmApp + 2 // posted by the poller (wParam=vol%, lParam=muted)
	wmDestroyWebview = wmApp + 3 // posted by the idle timer (wParam=generation)

	ninSelect    = wmUser + 0
	ninKeySelect = wmUser + 1

	waInactive = 0

	swHide = 0
	swShow = 5

	csDropShadow = 0x00020000
	csHRedraw    = 0x0002
	csVRedraw    = 0x0001

	wsPopup        = 0x80000000
	wsExToolWindow = 0x00000080
	wsExTopmost    = 0x00000008

	swpNoSize     = 0x0001
	swpNoActivate = 0x0010
	swpShowWindow = 0x0040
	hwndTopmost   = ^uintptr(0) // (HWND)-1

	spiGetWorkArea = 0x0030

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	tpmBottomAlign = 0x0020
	tpmRightAlign  = 0x0008

	mfString    = 0x0000
	mfSeparator = 0x0800

	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modWin      = 0x0008
	modNoRepeat = 0x4000

	nimAdd        = 0x0000
	nimModify     = 0x0001
	nimDelete     = 0x0002
	nimSetVersion = 0x0004

	nifMessage = 0x0001
	nifIcon    = 0x0002
	nifTip     = 0x0004
	nifShowTip = 0x0080

	notifyIconVersion4 = 4

	idcArrow       = 32512
	biRGB          = 0
	dibRGBColors   = 0
	dwmCornerPref  = 33 // DWMWA_WINDOW_CORNER_PREFERENCE
	dwmCornerRound = 2  // DWMWCP_ROUND

	hotkeyID = 1

	idOpen = 0xE001
	idQuit = 0xE002

	flyoutW = 384
	flyoutH = 568
	margin  = 12

	// Tear down WebView2 (freeing its browser processes) after the flyout has
	// been closed this long, so an idle tray costs almost nothing.
	idleDestroy = 30 * time.Second

	// ICoreWebView2Controller::Close is at vtable slot 24 (IUnknown's 3 slots +
	// 21 preceding controller methods). The go-webview2 version is pinned in
	// go.mod, so this layout is fixed; the wrapper doesn't expose Close itself.
	controllerCloseSlot = 24
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type wndClassEx struct {
	CbSize     uint32
	Style      uint32
	WndProc    uintptr
	CbClsExtra int32
	CbWndExtra int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type notifyIconData struct {
	CbSize          uint32
	Wnd             uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GuidItem        windows.GUID
	BalloonIcon     uintptr
}

type iconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	MaskBmp  uintptr
	ColorBmp uintptr
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// ── App ─────────────────────────────────────────────────────────────────────

type app struct {
	hwnd     uintptr
	chromium *edge.Chromium
	visible  bool
	idleGen  uint32 // bumps on every show/hide to invalidate stale idle timers
	curIcon  uintptr
	tip      string
	url      string
	shutdown func()
}

var a *app // single instance; the wndproc callback reaches it through this.

// Run installs the tray icon + flyout and blocks on the Win32 message loop
// until the user picks "Quitter" (or the window is destroyed). It must run on
// the process's main goroutine. shutdown is invoked once, on quit, to stop the
// HTTP server cleanly.
func Run(host string, port int, shutdown func()) error {
	runtime.LockOSThread()

	url := fmt.Sprintf("http://localhost:%d", port)
	if port == 80 {
		url = "http://localhost"
	}
	a = &app{url: url, shutdown: shutdown}

	hinst, _, _ := pGetModuleHandleW.Call(0)
	cursor, _, _ := pLoadCursorW.Call(0, idcArrow)
	// Dark brush (COLORREF 0x00BBGGRR for #0d1117) so the window is never
	// see-through, even for the instant before WebView2 paints.
	brush, _, _ := pCreateSolidBrush.Call(0x00171110)
	className := windows.StringToUTF16Ptr("SonarRemoteFlyout")

	wc := wndClassEx{
		CbSize:     uint32(unsafe.Sizeof(wndClassEx{})),
		Style:      csDropShadow | csHRedraw | csVRedraw,
		WndProc:    windows.NewCallback(wndProc),
		Instance:   hinst,
		Cursor:     cursor,
		Background: brush,
		ClassName:  className,
	}
	if ret, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); ret == 0 {
		return fmt.Errorf("RegisterClassEx: %w", err)
	}

	hwnd, _, err := pCreateWindowExW.Call(
		wsExToolWindow|wsExTopmost,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Sonar Remote"))),
		wsPopup,
		0, 0, flyoutW, flyoutH,
		0, 0, hinst, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx: %w", err)
	}
	a.hwnd = hwnd

	// Rounded corners (Windows 11); harmless if unsupported.
	corner := int32(dwmCornerRound)
	pDwmSetWindowAttribute.Call(hwnd, dwmCornerPref, uintptr(unsafe.Pointer(&corner)), 4)

	// WebView2 is created lazily on the first open (see ensureWebview) so that a
	// session where the flyout is never shown — e.g. a whole gaming session —
	// spawns no browser processes and uses no extra RAM.

	// Tray icon.
	a.setIcon(0, false) // neutral placeholder until the first poll
	nid := a.baseNID()
	nid.Flags = nifMessage | nifIcon | nifTip | nifShowTip
	nid.CallbackMessage = wmTray
	nid.Icon = a.curIcon
	copyTip(&nid, "Sonar Remote")
	pShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	nid.Version = notifyIconVersion4
	pShellNotifyIconW.Call(nimSetVersion, uintptr(unsafe.Pointer(&nid)))

	// Global hotkey: Ctrl+Alt+S.
	pRegisterHotKey.Call(hwnd, hotkeyID, modControl|modAlt|modNoRepeat, 'S')

	go a.pollMaster(port)

	// Message loop.
	var m msg
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 { // 0 = WM_QUIT, -1 = error
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	a.removeTray()
	if a.shutdown != nil {
		a.shutdown()
	}
	return nil
}

func wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch message {
	case wmTray:
		switch uint32(lParam & 0xFFFF) {
		case wmLButtonUp, ninSelect, ninKeySelect:
			a.toggle()
		case wmRButtonUp, wmContextMenu:
			a.showMenu()
		}
		return 0

	case wmHotkey:
		if wParam == hotkeyID {
			a.toggle()
		}
		return 0

	case wmActivate:
		// The flyout dismisses itself the moment it loses focus (EarTrumpet feel).
		if (wParam & 0xFFFF) == waInactive {
			a.hide()
		}
		return 0

	case wmSize:
		if a.chromium != nil {
			a.chromium.Resize()
		}
		return 0

	case wmUpdateIcon:
		a.setIcon(int(wParam), lParam != 0)
		a.refreshTray()
		return 0

	case wmDestroyWebview:
		// Fire only if this timer is still the latest and the flyout is closed.
		if uint32(wParam) == a.idleGen && !a.visible {
			a.destroyWebview()
		}
		return 0

	case wmCommand:
		switch wParam & 0xFFFF {
		case idOpen:
			verb := windows.StringToUTF16Ptr("open")
			target := windows.StringToUTF16Ptr(a.url)
			pShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, 1)
		case idQuit:
			pPostQuitMessage.Call(0)
		}
		return 0

	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

// ── Flyout show/hide ────────────────────────────────────────────────────────

func (a *app) toggle() {
	if a.visible {
		a.hide()
	} else {
		a.show()
	}
}

// ensureWebview creates and embeds the WebView2 control the first time the
// flyout is opened. Returns false if the runtime is unavailable.
func (a *app) ensureWebview() bool {
	if a.chromium != nil {
		return true
	}
	c := edge.NewChromium()
	if !c.Embed(a.hwnd) {
		return false
	}
	// Match the app's dark background so the flyout never flashes white.
	if c2 := c.GetController().GetICoreWebView2Controller2(); c2 != nil {
		c2.PutDefaultBackgroundColor(edge.COREWEBVIEW2_COLOR{A: 0xFF, R: 0x0d, G: 0x11, B: 0x17})
	}
	if s, err := c.GetSettings(); err == nil {
		s.PutAreDefaultContextMenusEnabled(false)
		s.PutAreDevToolsEnabled(false)
	}
	a.chromium = c
	c.Resize()
	c.Navigate(a.url + "/flyout")
	return true
}

func (a *app) show() {
	a.idleGen++ // cancel any pending idle-destroy
	if !a.ensureWebview() {
		return
	}
	// Anchor to the bottom-right of the work area, just above the taskbar.
	var wa rect
	pSystemParametersInfo.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&wa)), 0)
	x := wa.Right - flyoutW - margin
	y := wa.Bottom - flyoutH - margin
	// Show the window first, then (re)fit the WebView2 to the now-visible client
	// rect — the controller is embedded while hidden, so its bounds are 0 until
	// we resize against a real, on-screen client area.
	pSetWindowPos.Call(a.hwnd, hwndTopmost, uintptr(x), uintptr(y), flyoutW, flyoutH, swpShowWindow)
	pShowWindow.Call(a.hwnd, swShow)
	if c := a.chromium.GetController(); c != nil {
		c.PutIsVisible(true) // resume rendering + page timers
	}
	a.chromium.Resize()
	pSetForegroundWindow.Call(a.hwnd)
	a.chromium.Focus()
	a.visible = true
}

func (a *app) hide() {
	if !a.visible {
		return
	}
	pShowWindow.Call(a.hwnd, swHide)
	// Tell WebView2 the content is hidden: it stops compositing and Chromium
	// throttles the page, and our JS pauses polling on document.hidden — so a
	// closed flyout costs virtually no CPU.
	if a.chromium != nil {
		if c := a.chromium.GetController(); c != nil {
			c.PutIsVisible(false)
		}
	}
	a.visible = false

	// Schedule a full teardown if the flyout stays closed. The timer only posts
	// to the UI thread (COM lives there); show() bumps idleGen to cancel it.
	a.idleGen++
	gen := a.idleGen
	time.AfterFunc(idleDestroy, func() {
		pPostMessageW.Call(a.hwnd, wmDestroyWebview, uintptr(gen), 0)
	})
}

// destroyWebview closes the WebView2 controller and drops our reference so the
// browser processes exit; ensureWebview recreates it on the next open. Must run
// on the UI thread.
func (a *app) destroyWebview() {
	if a.chromium == nil {
		return
	}
	if c := a.chromium.GetController(); c != nil {
		c.PutIsVisible(false)
		closeController(c)
	}
	a.chromium = nil
}

// controllerRaw mirrors the memory layout of edge.ICoreWebView2Controller (a
// single leading vtable pointer) so we can reach COM methods the wrapper omits.
type controllerRaw struct {
	vtbl *[controllerCloseSlot + 1]edge.ComProc
}

// closeController invokes ICoreWebView2Controller::Close (vtable slot
// controllerCloseSlot); go-webview2 doesn't wrap it. The layout is fixed by the
// pinned module version.
func closeController(c *edge.ICoreWebView2Controller) {
	raw := (*controllerRaw)(unsafe.Pointer(c))
	raw.vtbl[controllerCloseSlot].Call(uintptr(unsafe.Pointer(c)))
}

func (a *app) showMenu() {
	menu, _, _ := pCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer pDestroyMenu.Call(menu)
	pAppendMenuW.Call(menu, mfString, idOpen,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Ouvrir dans le navigateur"))))
	pAppendMenuW.Call(menu, mfSeparator, 0, 0)
	pAppendMenuW.Call(menu, mfString, idQuit,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Quitter"))))

	// Foreground dance so the menu dismisses correctly for a tray owner window.
	pSetForegroundWindow.Call(a.hwnd)
	var pt point
	if p := getCursorPos(); p != nil {
		pt = *p
	}
	cmd, _, _ := pTrackPopupMenu.Call(menu,
		tpmRightButton|tpmReturnCmd|tpmBottomAlign|tpmRightAlign,
		uintptr(pt.X), uintptr(pt.Y), 0, a.hwnd, 0)
	pPostMessageW.Call(a.hwnd, 0 /*WM_NULL*/, 0, 0)
	if cmd != 0 {
		pPostMessageW.Call(a.hwnd, wmCommand, cmd&0xFFFF, 0)
	}
}

var pGetCursorPos = user32.NewProc("GetCursorPos")

func getCursorPos() *point {
	var p point
	if r, _, _ := pGetCursorPos.Call(uintptr(unsafe.Pointer(&p))); r == 0 {
		return nil
	}
	return &p
}

// ── Tray icon (drawn in-process, reflects the master level) ─────────────────

func (a *app) baseNID() notifyIconData {
	return notifyIconData{
		CbSize: uint32(unsafe.Sizeof(notifyIconData{})),
		Wnd:    a.hwnd,
		ID:     1,
	}
}

// refreshTray pushes the current icon + tooltip to the shell.
func (a *app) refreshTray() {
	nid := a.baseNID()
	nid.Flags = nifIcon | nifTip | nifShowTip
	nid.Icon = a.curIcon
	copyTip(&nid, a.tip)
	pShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func (a *app) removeTray() {
	nid := a.baseNID()
	pShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
}

// setIcon builds a fresh HICON for the given master volume (0..100) and mute
// state, replacing the previous one. Called on the UI thread only.
func (a *app) setIcon(volPct int, muted bool) {
	h := buildMeterIcon(volPct, muted)
	if h == 0 {
		return
	}
	if a.curIcon != 0 {
		pDestroyIcon.Call(a.curIcon)
	}
	a.curIcon = h
	if muted {
		a.tip = "Sonar — Muet"
	} else {
		a.tip = fmt.Sprintf("Sonar — Master %d%%", volPct)
	}
}

// buildMeterIcon renders a 5-bar level meter into a 32x32 BGRA icon.
func buildMeterIcon(volPct int, muted bool) uintptr {
	const dim = 32
	var bits unsafe.Pointer
	bmi := bitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:       dim,
		Height:      -dim, // top-down
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
	}
	color, _, _ := pCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&bmi)), dibRGBColors,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if color == 0 || bits == nil {
		return 0
	}
	px := unsafe.Slice((*uint32)(bits), dim*dim)
	for i := range px {
		px[i] = 0 // transparent
	}
	set := func(x, y int, c uint32) {
		if x >= 0 && x < dim && y >= 0 && y < dim {
			px[y*dim+x] = c
		}
	}

	const (
		lit   = 0xFF5B8CFF
		unlit = 0xFF39415A
		gray  = 0xFF5A6072
		redSl = 0xFFFF4D4D
	)
	litCount := (volPct*5 + 50) / 100 // round to nearest of 5
	baseline := 27
	bw, gap := 4, 1
	totalW := 5*bw + 4*gap
	startX := (dim - totalW) / 2
	for i := 0; i < 5; i++ {
		h := 8 + i*4 // 8,12,16,20,24
		x0 := startX + i*(bw+gap)
		var col uint32 = unlit
		if muted {
			col = gray
		} else if i < litCount {
			col = lit
		}
		for y := baseline - h; y <= baseline; y++ {
			for x := x0; x < x0+bw; x++ {
				set(x, y, col)
			}
		}
	}
	if muted {
		for d := 0; d < dim; d++ { // 3px diagonal slash
			set(d, d, redSl)
			set(d+1, d, redSl)
			set(d, d+1, redSl)
		}
	}

	// A 1bpp all-zero mask; the 32bpp alpha channel does the real masking.
	mask, _, _ := pCreateBitmap.Call(dim, dim, 1, 1, 0)
	ii := iconInfo{FIcon: 1, MaskBmp: mask, ColorBmp: color}
	hicon, _, _ := pCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	pDeleteObject.Call(color)
	pDeleteObject.Call(mask)
	return hicon
}

func copyTip(nid *notifyIconData, s string) {
	u := windows.StringToUTF16(s)
	if len(u) > len(nid.Tip) {
		u = u[:len(nid.Tip)]
		u[len(u)-1] = 0
	}
	copy(nid.Tip[:], u)
}

// ── Master-level poller ─────────────────────────────────────────────────────

func (a *app) pollMaster(port int) {
	url := fmt.Sprintf("http://127.0.0.1:%d/api/volumeSettings/classic", port)
	client := &http.Client{Timeout: 2 * time.Second}
	lastVol, lastMuted, first := -1, false, true
	for {
		if vol, muted, ok := fetchMaster(client, url); ok {
			// Only rebuild + push the icon when it would actually change.
			if first || vol != lastVol || muted != lastMuted {
				pPostMessageW.Call(a.hwnd, wmUpdateIcon, uintptr(vol), boolToUintptr(muted))
				lastVol, lastMuted, first = vol, muted, false
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func fetchMaster(client *http.Client, url string) (volPct int, muted, ok bool) {
	resp, err := client.Get(url)
	if err != nil {
		return 0, false, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, false, false
	}
	var body struct {
		Masters struct {
			Classic struct {
				Volume float64 `json:"volume"`
				Muted  bool    `json:"muted"`
			} `json:"classic"`
		} `json:"masters"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, false, false
	}
	v := body.Masters.Classic.Volume
	if v < 0 {
		v = 0
	} else if v > 1 {
		v = 1
	}
	return int(v*100 + 0.5), body.Masters.Classic.Muted, true
}

func boolToUintptr(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}
