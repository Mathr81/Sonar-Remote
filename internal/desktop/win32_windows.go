package desktop

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32  = windows.NewLazySystemDLL("user32.dll")
	gdi32   = windows.NewLazySystemDLL("gdi32.dll")
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	dwmapi  = windows.NewLazySystemDLL("dwmapi.dll")

	pRegisterClassExW        = user32.NewProc("RegisterClassExW")
	pCreateWindowExW         = user32.NewProc("CreateWindowExW")
	pDefWindowProcW          = user32.NewProc("DefWindowProcW")
	pGetMessageW             = user32.NewProc("GetMessageW")
	pTranslateMessage        = user32.NewProc("TranslateMessage")
	pDispatchMessageW        = user32.NewProc("DispatchMessageW")
	pPostQuitMessage         = user32.NewProc("PostQuitMessage")
	pPostMessageW            = user32.NewProc("PostMessageW")
	pShowWindow              = user32.NewProc("ShowWindow")
	pSetForegroundWindow     = user32.NewProc("SetForegroundWindow")
	pSetWindowPos            = user32.NewProc("SetWindowPos")
	pLoadCursorW             = user32.NewProc("LoadCursorW")
	pRegisterHotKey          = user32.NewProc("RegisterHotKey")
	pCreatePopupMenu         = user32.NewProc("CreatePopupMenu")
	pAppendMenuW             = user32.NewProc("AppendMenuW")
	pTrackPopupMenu          = user32.NewProc("TrackPopupMenu")
	pDestroyMenu             = user32.NewProc("DestroyMenu")
	pSystemParametersInfoW   = user32.NewProc("SystemParametersInfoW")
	pCreateIconIndirect      = user32.NewProc("CreateIconIndirect")
	pDestroyIcon             = user32.NewProc("DestroyIcon")
	pGetCursorPos            = user32.NewProc("GetCursorPos")
	pScreenToClient          = user32.NewProc("ScreenToClient")
	pInvalidateRect          = user32.NewProc("InvalidateRect")
	pBeginPaint              = user32.NewProc("BeginPaint")
	pEndPaint                = user32.NewProc("EndPaint")
	pSetCapture              = user32.NewProc("SetCapture")
	pReleaseCapture          = user32.NewProc("ReleaseCapture")
	pTrackMouseEvent         = user32.NewProc("TrackMouseEvent")
	pGetDpiForWindow         = user32.NewProc("GetDpiForWindow")
	pGetSystemMetricsForDpi  = user32.NewProc("GetSystemMetricsForDpi")
	pSetProcessDpiAwarenessC = user32.NewProc("SetProcessDpiAwarenessContext")

	pCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	pCreateBitmap       = gdi32.NewProc("CreateBitmap")
	pCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	pSelectObject       = gdi32.NewProc("SelectObject")
	pDeleteObject       = gdi32.NewProc("DeleteObject")
	pBitBlt             = gdi32.NewProc("BitBlt")

	pShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	pShellExecuteW    = shell32.NewProc("ShellExecuteW")

	pDwmSetWindowAttribute        = dwmapi.NewProc("DwmSetWindowAttribute")
	pDwmExtendFrameIntoClientArea = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
)

const (
	wmDestroy        = 0x0002
	wmActivate       = 0x0006
	wmPaint          = 0x000F
	wmEraseBkgnd     = 0x0014
	wmContextMenu    = 0x007B
	wmKeyDown        = 0x0100
	wmMouseMove      = 0x0200
	wmLButtonDown    = 0x0201
	wmLButtonUp      = 0x0202
	wmRButtonUp      = 0x0205
	wmMouseWheel     = 0x020A
	wmCaptureChanged = 0x0215
	wmMouseLeave     = 0x02A3
	wmHotkey         = 0x0312
	wmUser           = 0x0400
	wmApp            = 0x8000

	ninSelect    = wmUser + 0
	ninKeySelect = wmUser + 1

	vkTab, vkEscape, vkSpace           = 0x09, 0x1B, 0x20
	vkLeft, vkUp, vkRight, vkDown, vkM = 0x25, 0x26, 0x27, 0x28, 0x4D

	wsPopup        = 0x80000000
	wsExToolWindow = 0x00000080
	wsExTopmost    = 0x00000008

	swHide        = 0
	swpShowWindow = 0x0040
	hwndTopmost   = ^uintptr(0)

	spiGetWorkArea = 0x0030
	smCxSmIcon     = 49
	idcArrow       = 32512
	srcCopy        = 0x00CC0020
	tmeLeave       = 0x0002

	tpmRightButton = 0x0002
	tpmRightAlign  = 0x0008
	tpmBottomAlign = 0x0020
	tpmReturnCmd   = 0x0100
	mfString       = 0x0000
	mfChecked      = 0x0008
	mfSeparator    = 0x0800

	modAlt, modControl, modNoRepeat = 0x0001, 0x0002, 0x4000

	nimAdd, nimModify, nimDelete, nimSetVersion = 0, 1, 2, 4
	nifMessage, nifIcon, nifTip, nifShowTip     = 0x01, 0x02, 0x04, 0x80
	notifyIconVersion4                          = 4

	dwmaUseImmersiveDarkMode = 20
	dwmaCornerPreference     = 33
	dwmaSystemBackdropType   = 38
	dwmCornerRound           = 2
	dwmBackdropAcrylic       = 3 // DWMSBT_TRANSIENTWINDOW

	dpiPerMonitorV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
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

type paintStruct struct {
	Hdc      uintptr
	Erase    int32
	Paint    rect
	Restore  int32
	IncUpd   int32
	Reserved [32]byte
}

type trackMouseEvent struct {
	CbSize    uint32
	Flags     uint32
	Hwnd      uintptr
	HoverTime uint32
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

// newDIB creates a top-down 32-bit bitmap and returns it with its pixel memory.
func newDIB(w, h int) (bmp uintptr, bits unsafe.Pointer) {
	bmi := bitmapInfoHeader{
		Size:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:    int32(w),
		Height:   int32(-h),
		Planes:   1,
		BitCount: 32,
	}
	bmp, _, _ = pCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&bmi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	return bmp, bits
}
