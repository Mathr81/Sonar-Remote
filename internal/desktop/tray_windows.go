// Package desktop is the Windows tray experience: an icon showing the Sonar
// master level, a native flyout mixer (flyout_windows.go) and a global hotkey,
// all on one Win32 message loop. It draws everything itself, so opening the
// flyout costs no extra process and no extra memory.
package desktop

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"rsc.io/qr"

	"sonar-remote/internal/mdnsx"
	"sonar-remote/internal/sonar"
)

const (
	wmTray    = wmApp + 1 // tray icon callback
	wmState   = wmApp + 2 // posted by the poller when Sonar's state changed
	wmConnect = wmApp + 3 // posted by the tray menu: open the connect panel

	hotkeyID = 1

	idConnect, idRemote, idAutostart, idQuit = 1, 2, 3, 4

	className    = "SonarRemoteFlyout"
	runKey       = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "SonarRemote"
)

// ErrAlreadyRunning is returned by Run when another instance owns the tray.
var ErrAlreadyRunning = errors.New("sonar-remote is already running")

type app struct {
	hwnd  uintptr
	sonar *sonar.Client
	host  string // mDNS name of the web remote, "" if not published
	port  int

	// UI thread only.
	scale            float64 // DPI / 96
	th               theme
	acrylic          bool
	dib, memDC       uintptr
	bits             unsafe.Pointer
	dibW, dibH       int
	textFam, iconFam uintptr
	fText, fBold     uintptr
	fIcon            uintptr
	fmtLeft          uintptr
	fmtCenter        uintptr
	fmtRight         uintptr
	hover, hoverPart int
	drag, focus      int
	kbd, tracking    bool
	hiddenAt         time.Time
	connect          bool     // showing the "connect a device" panel instead of the mixer
	link             string   // LAN address of the web remote, encoded in code
	code             *qr.Code // nil while the machine has no LAN address
	icon             uintptr
	iconKey          string

	visible atomic.Bool

	mu     sync.Mutex
	st     sonar.State
	online bool
	hold   [chatRow + 1]time.Time
	pend   map[string]func() error
	kick   chan struct{} // wakes the writer
	wake   chan struct{} // wakes the poller
}

var a *app // the single instance; wndProc reaches it through this

// Run shows the tray icon and blocks on the message loop until the user quits.
// It must be called from the main goroutine.
func Run(client *sonar.Client, host string, port int) error {
	runtime.LockOSThread()

	if _, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(`Local\SonarRemote`)); err == windows.ERROR_ALREADY_EXISTS {
		return ErrAlreadyRunning
	}
	pSetProcessDpiAwarenessC.Call(dpiPerMonitorV2)
	gdipStart()

	a = &app{
		sonar: client, host: host, port: port,
		hover: -1, drag: -1, focus: -1,
		pend: map[string]func() error{},
		kick: make(chan struct{}, 1), wake: make(chan struct{}, 1),
	}
	a.textFam = fontFamily("Segoe UI Variable Text", "Segoe UI Variable", "Segoe UI")
	a.iconFam = fontFamily("Segoe Fluent Icons", "Segoe MDL2 Assets")
	a.fmtLeft, a.fmtCenter, a.fmtRight = newStringFormat(alignNear), newStringFormat(alignCenter), newStringFormat(alignFar)

	hinst := windows.Handle(0)
	windows.GetModuleHandleEx(0, nil, &hinst)
	cursor, _, _ := pLoadCursorW.Call(0, idcArrow)
	class := windows.StringToUTF16Ptr(className)
	wc := wndClassEx{
		WndProc:   windows.NewCallback(wndProc),
		Instance:  uintptr(hinst),
		Cursor:    cursor,
		ClassName: class,
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return fmt.Errorf("RegisterClassEx: %w", err)
	}
	hwnd, _, err := pCreateWindowExW.Call(
		wsExToolWindow|wsExTopmost,
		uintptr(unsafe.Pointer(class)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Sonar Remote"))),
		wsPopup, 0, 0, 0, 0, 0, 0, uintptr(hinst), 0) // sized on show
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx: %w", err)
	}
	a.hwnd = hwnd

	// Windows 11 chrome: rounded corners, and the acrylic backdrop where the
	// build supports it (22H2+). Older systems get a flat themed background.
	corner := int32(dwmCornerRound)
	pDwmSetWindowAttribute.Call(hwnd, dwmaCornerPreference, uintptr(unsafe.Pointer(&corner)), 4)
	backdrop := int32(dwmBackdropAcrylic)
	if hr, _, _ := pDwmSetWindowAttribute.Call(hwnd, dwmaSystemBackdropType, uintptr(unsafe.Pointer(&backdrop)), 4); hr == 0 {
		a.acrylic = true
		sheet := [4]int32{-1, -1, -1, -1}
		pDwmExtendFrameIntoClientArea.Call(hwnd, uintptr(unsafe.Pointer(&sheet)))
	}

	a.syncIcon()
	pRegisterHotKey.Call(hwnd, hotkeyID, modControl|modAlt|modNoRepeat, 'S')
	go a.poller()
	go a.writer()
	time.AfterFunc(10*time.Second, trimMemory) // once startup has settled

	var m msg
	for {
		r, _, err := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 { // 0 = WM_QUIT, -1 = error
			log.Printf("message loop ended: GetMessage=%d (%v)", int32(r), err)
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	nid := a.nid()
	pShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
	return nil
}

func wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	// Client coordinates packed in lParam are signed 16-bit.
	x, y := int(int16(lParam)), int(int16(lParam>>16))
	switch message {
	case wmTray:
		switch lParam & 0xFFFF {
		case ninSelect, ninKeySelect:
			// Clicking the icon while the flyout is open first deactivates
			// (hides) it; don't reopen it on the same click.
			if time.Since(a.hiddenAt) > 300*time.Millisecond {
				a.show(false)
			}
		case wmContextMenu:
			a.showMenu()
		}
	case wmHotkey:
		if a.visible.Load() {
			a.hide()
		} else {
			a.show(false)
		}
	case wmActivate:
		if wParam&0xFFFF == 0 { // WA_INACTIVE: dismiss like a system flyout
			a.hide()
		}
	case wmState:
		a.stateChanged()
	case wmConnect:
		a.show(true)
	case wmPaint:
		a.onPaint()
	case wmEraseBkgnd:
		return 1
	case wmLButtonDown:
		a.onMouseDown(x, y)
	case wmMouseMove:
		a.onMouseMove(x, y)
	case wmLButtonUp:
		pReleaseCapture.Call()
	case wmCaptureChanged:
		a.drag = -1
		a.redraw()
	case wmMouseLeave:
		a.tracking, a.hover, a.hoverPart = false, -1, partNone
		a.redraw()
	case wmMouseWheel:
		pt := point{int32(x), int32(y)} // wheel coordinates are screen-relative
		pScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
		a.onWheel(int(pt.X), int(pt.Y), int(int16(wParam>>16))/120)
	case wmKeyDown:
		a.onKey(wParam)
	case wmDestroy:
		log.Print("window destroyed")
		pPostQuitMessage.Call(0)
	default:
		r, _, _ := pDefWindowProcW.Call(hwnd, message, wParam, lParam)
		return r
	}
	return 0
}

// ── Flyout window ───────────────────────────────────────────────────────────

// url returns the web remote's address on the given host.
func (a *app) url(host string) string {
	if a.port == 80 {
		return "http://" + host
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(a.port))
}

// show opens the flyout on the mixer, or on the panel that tells another
// device how to reach the web remote.
func (a *app) show(connect bool) {
	a.connect, a.code = connect, nil
	if ip := mdnsx.LANAddr(); connect && ip != "" {
		// The IP rather than the .local name: it works even where mDNS doesn't.
		a.link = a.url(ip)
		a.code, _ = qr.Encode(a.link, qr.M)
	}
	dpi, _, _ := pGetDpiForWindow.Call(a.hwnd)
	if s := float64(dpi) / 96; s != a.scale {
		a.scale = s
		for _, f := range []uintptr{a.fText, a.fBold, a.fIcon} {
			pGdipDeleteFont.Call(f)
		}
		a.fText, a.fBold = newFont(a.textFam, 14*s, false), newFont(a.textFam, 14*s, true)
		a.fIcon = newFont(a.iconFam, 16*s, false)
	}
	a.th = loadTheme()
	dark := int32(1)
	if a.th.light {
		dark = 0
	}
	pDwmSetWindowAttribute.Call(a.hwnd, dwmaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&dark)), 4)

	// shortcut: anchored to the primary work area's bottom-right corner, which
	// is where the tray is unless the taskbar sits on a secondary monitor.
	var wa rect
	pSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&wa)), 0)
	w, h, m := a.px(winW), a.px(winH), a.px(12)
	a.hover, a.drag, a.kbd = -1, -1, false
	a.visible.Store(true)
	pSetWindowPos.Call(a.hwnd, hwndTopmost, uintptr(int(wa.Right)-w-m), uintptr(int(wa.Bottom)-h-m),
		uintptr(w), uintptr(h), swpShowWindow)
	pSetForegroundWindow.Call(a.hwnd)
	a.redraw()
	select { // refresh now rather than at the next slow tick
	case a.wake <- struct{}{}:
	default:
	}
}

func (a *app) hide() {
	if !a.visible.Swap(false) {
		return
	}
	a.hiddenAt = time.Now()
	pShowWindow.Call(a.hwnd, swHide)
	trimMemory()
}

// trimMemory hands the process's physical pages back to Windows. The app then
// idles on the few it touches to poll; the rest fault back in on the next open.
func trimMemory() {
	windows.SetProcessWorkingSetSizeEx(windows.CurrentProcess(), ^uintptr(0), ^uintptr(0), 0)
}

func (a *app) onPaint() {
	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(a.hwnd, uintptr(unsafe.Pointer(&ps)))
	w, h := a.px(winW), a.px(winH)
	if w != a.dibW || h != a.dibH {
		if a.memDC == 0 {
			a.memDC, _, _ = pCreateCompatibleDC.Call(0)
		}
		old := a.dib
		a.dib, a.bits = newDIB(w, h)
		a.dibW, a.dibH = w, h
		pSelectObject.Call(a.memDC, a.dib)
		pDeleteObject.Call(old)
	}
	withGraphics(a.bits, w, h, a.paint)
	pBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), a.memDC, 0, 0, srcCopy)
	pEndPaint.Call(a.hwnd, uintptr(unsafe.Pointer(&ps)))
}

// stateChanged refreshes whatever shows the mixer state. UI thread only.
func (a *app) stateChanged() {
	a.syncIcon()
	if a.visible.Load() {
		a.redraw()
	}
}

// ── Sonar I/O (off the UI thread) ───────────────────────────────────────────

// poller mirrors Sonar's state: every second while the flyout is open, and
// lazily otherwise, just to keep the tray icon honest.
func (a *app) poller() {
	for {
		st, err := a.sonar.State()
		a.mu.Lock()
		changed := a.online != (err == nil)
		a.online = err == nil
		if err == nil {
			now := time.Now()
			for i := range st.Levels {
				if now.Before(a.hold[i]) {
					st.Levels[i] = a.st.Levels[i]
				}
			}
			if now.Before(a.hold[chatRow]) {
				st.ChatMix = a.st.ChatMix
			}
			changed = changed || st != a.st
			a.st = st
		}
		a.mu.Unlock()
		if changed {
			pPostMessageW.Call(a.hwnd, wmState, 0, 0)
		}
		every := 5 * time.Second
		if a.visible.Load() {
			every = time.Second
		}
		select {
		case <-a.wake:
		case <-time.After(every):
		}
	}
}

// send queues a write, replacing any pending one with the same key so a drag
// only ever sends its latest position.
func (a *app) send(key string, fn func() error) {
	a.mu.Lock()
	a.pend[key] = fn
	a.mu.Unlock()
	select {
	case a.kick <- struct{}{}:
	default:
	}
}

func (a *app) writer() {
	for range a.kick {
		a.mu.Lock()
		batch := a.pend
		a.pend = map[string]func() error{}
		a.mu.Unlock()
		for _, fn := range batch {
			_ = fn() // a lost write is corrected by the next poll
		}
	}
}

// ── Tray icon and menu ──────────────────────────────────────────────────────

func (a *app) nid() notifyIconData {
	n := notifyIconData{Wnd: a.hwnd, ID: 1}
	n.CbSize = uint32(unsafe.Sizeof(n))
	return n
}

// syncIcon redraws the tray icon and tooltip when the master level, mute state
// or taskbar theme changed since the last call.
func (a *app) syncIcon() {
	a.mu.Lock()
	master, online := a.st.Levels[0], a.online
	a.mu.Unlock()
	pct, light := int(master.Volume*100+0.5), systemLight()
	tip := fmt.Sprintf("Sonar : %d %%", pct)
	switch {
	case !online:
		tip, pct = "Sonar est introuvable", 0
	case master.Muted:
		tip = "Sonar : muet"
	}
	key := fmt.Sprint(tip, light)
	if key == a.iconKey {
		return
	}
	first := a.icon == 0
	a.iconKey = key

	dpi, _, _ := pGetDpiForWindow.Call(a.hwnd)
	size, _, _ := pGetSystemMetricsForDpi.Call(smCxSmIcon, dpi)
	old := a.icon
	a.icon = meterIcon(int(size), pct, master.Muted || !online, light)

	n := a.nid()
	n.Flags = nifMessage | nifIcon | nifTip | nifShowTip
	n.CallbackMessage = wmTray
	n.Icon = a.icon
	copy(n.Tip[:len(n.Tip)-1], windows.StringToUTF16(tip))
	if first {
		pShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&n)))
		n.Version = notifyIconVersion4
		pShellNotifyIconW.Call(nimSetVersion, uintptr(unsafe.Pointer(&n)))
	} else {
		pShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&n)))
		pDestroyIcon.Call(old)
	}
}

// meterIcon draws five rising bars, lit up to the master level, in the
// taskbar's foreground colour. off dims them all and strikes them through.
func meterIcon(size, pct int, off, light bool) uintptr {
	color, bits := newDIB(size, size)
	if color == 0 {
		return 0
	}
	px := unsafe.Slice((*uint32)(bits), size*size)
	// Premultiplied BGRA: full and 35 % opacity of the foreground colour.
	lit, dim := uint32(0xFFFFFFFF), uint32(0x59595959)
	if light {
		lit, dim = 0xFF000000, 0x59000000
	}
	gap := max(1, size/12)
	bw := (size - 4*gap) / 5
	x0 := (size - 5*bw - 4*gap) / 2
	litBars := (pct*5 + 50) / 100
	for i := 0; i < 5; i++ {
		c := dim
		if !off && i < litBars {
			c = lit
		}
		h := (size - 2) * (i + 3) / 7
		for y := size - 1 - h; y < size-1; y++ {
			for x := x0 + i*(bw+gap); x < x0+i*(bw+gap)+bw; x++ {
				px[y*size+x] = c
			}
		}
	}
	if off {
		for d := 1; d < size-1; d++ { // diagonal strike, two pixels thick
			px[d*size+d], px[d*size+d-1] = 0xFFE5484D, 0xFFE5484D
		}
	}
	mask, _, _ := pCreateBitmap.Call(uintptr(size), uintptr(size), 1, 1, 0)
	ii := iconInfo{FIcon: 1, MaskBmp: mask, ColorBmp: color}
	icon, _, _ := pCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	pDeleteObject.Call(color)
	pDeleteObject.Call(mask)
	return icon
}

func (a *app) showMenu() {
	menu, _, _ := pCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer pDestroyMenu.Call(menu)
	add := func(flags, id uintptr, label string) {
		pAppendMenuW.Call(menu, flags, id, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(label))))
	}
	auto := uintptr(mfString)
	if autostart() {
		auto |= mfChecked
	}
	add(mfString, idConnect, "Connecter un iPad ou un téléphone")
	add(mfString, idRemote, "Ouvrir la télécommande web ici")
	add(auto, idAutostart, "Lancer au démarrage de Windows")
	add(mfSeparator, 0, "")
	add(mfString, idQuit, "Quitter")

	// A tray menu only dismisses properly if its owner is the foreground window.
	pSetForegroundWindow.Call(a.hwnd)
	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	cmd, _, _ := pTrackPopupMenu.Call(menu, tpmRightButton|tpmReturnCmd|tpmBottomAlign|tpmRightAlign,
		uintptr(pt.X), uintptr(pt.Y), 0, a.hwnd, 0)
	switch cmd {
	case idConnect:
		// Posted, not called: the menu closing still has activation changes
		// queued, and they would dismiss a flyout shown right now.
		pPostMessageW.Call(a.hwnd, wmConnect, 0, 0)
	case idRemote:
		pShellExecuteW.Call(0, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("open"))),
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(a.url("localhost")))), 0, 0, 1)
	case idAutostart:
		setAutostart(auto&mfChecked == 0)
	case idQuit:
		pPostQuitMessage.Call(0)
	}
}

// ── Launch at logon (HKCU Run key, no admin needed) ─────────────────────────

func autostart() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(runValueName)
	return err == nil
}

// setAutostart registers this executable with the flags it is running with.
func setAutostart(on bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	if !on {
		k.DeleteValue(runValueName)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := syscall.EscapeArg(exe)
	for _, arg := range os.Args[1:] {
		cmd += " " + syscall.EscapeArg(arg)
	}
	k.SetStringValue(runValueName, cmd)
}
