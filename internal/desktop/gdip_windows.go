package desktop

import (
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Thin wrappers over the GDI+ flat API: anti-aliased shapes and text with
// per-pixel alpha, which plain GDI cannot do and the acrylic backdrop needs.

var (
	gdiplus = windows.NewLazySystemDLL("gdiplus.dll")

	pGdiplusStartup        = gdiplus.NewProc("GdiplusStartup")
	pGdipBitmapFromScan0   = gdiplus.NewProc("GdipCreateBitmapFromScan0")
	pGdipImageGraphics     = gdiplus.NewProc("GdipGetImageGraphicsContext")
	pGdipDisposeImage      = gdiplus.NewProc("GdipDisposeImage")
	pGdipDeleteGraphics    = gdiplus.NewProc("GdipDeleteGraphics")
	pGdipSetSmoothingMode  = gdiplus.NewProc("GdipSetSmoothingMode")
	pGdipSetTextHint       = gdiplus.NewProc("GdipSetTextRenderingHint")
	pGdipSetPixelOffset    = gdiplus.NewProc("GdipSetPixelOffsetMode")
	pGdipGraphicsClear     = gdiplus.NewProc("GdipGraphicsClear")
	pGdipCreateSolidFill   = gdiplus.NewProc("GdipCreateSolidFill")
	pGdipDeleteBrush       = gdiplus.NewProc("GdipDeleteBrush")
	pGdipCreatePen1        = gdiplus.NewProc("GdipCreatePen1")
	pGdipSetPenLineCap     = gdiplus.NewProc("GdipSetPenLineCap197819")
	pGdipDeletePen         = gdiplus.NewProc("GdipDeletePen")
	pGdipFillRectangleI    = gdiplus.NewProc("GdipFillRectangleI")
	pGdipFillEllipseI      = gdiplus.NewProc("GdipFillEllipseI")
	pGdipDrawLineI         = gdiplus.NewProc("GdipDrawLineI")
	pGdipCreatePath        = gdiplus.NewProc("GdipCreatePath")
	pGdipAddPathArcI       = gdiplus.NewProc("GdipAddPathArcI")
	pGdipClosePathFigure   = gdiplus.NewProc("GdipClosePathFigure")
	pGdipFillPath          = gdiplus.NewProc("GdipFillPath")
	pGdipDrawPath          = gdiplus.NewProc("GdipDrawPath")
	pGdipDeletePath        = gdiplus.NewProc("GdipDeletePath")
	pGdipCreateFontFamily  = gdiplus.NewProc("GdipCreateFontFamilyFromName")
	pGdipCreateFont        = gdiplus.NewProc("GdipCreateFont")
	pGdipDeleteFont        = gdiplus.NewProc("GdipDeleteFont")
	pGdipCreateStringFmt   = gdiplus.NewProc("GdipCreateStringFormat")
	pGdipSetStringAlign    = gdiplus.NewProc("GdipSetStringFormatAlign")
	pGdipSetStringLineAlgn = gdiplus.NewProc("GdipSetStringFormatLineAlign")
	pGdipSetStringTrimming = gdiplus.NewProc("GdipSetStringFormatTrimming")
	pGdipDrawString        = gdiplus.NewProc("GdipDrawString")
)

const (
	pixelFormat32bppPARGB            = 0x000E200B
	alignNear, alignCenter, alignFar = 0, 1, 2
)

func gdipStart() {
	in := struct {
		Version  uint32
		Callback uintptr
		NoThread int32
		NoCodecs int32
	}{Version: 1}
	var token uintptr
	pGdiplusStartup.Call(uintptr(unsafe.Pointer(&token)), uintptr(unsafe.Pointer(&in)), 0)
}

// f32 passes a float32 argument through a syscall: Go mirrors the first four
// integer registers into XMM0-3, and stack slots carry the raw bits.
func f32(v float64) uintptr { return uintptr(math.Float32bits(float32(v))) }

// fontFamily returns the first installed family among names (0 if none).
func fontFamily(names ...string) uintptr {
	for _, n := range names {
		var fam uintptr
		if st, _, _ := pGdipCreateFontFamily.Call(uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(n))), 0, uintptr(unsafe.Pointer(&fam))); st == 0 {
			return fam
		}
	}
	return 0
}

func newFont(family uintptr, px float64, bold bool) uintptr {
	style := uintptr(0)
	if bold {
		style = 1
	}
	var f uintptr
	pGdipCreateFont.Call(family, f32(px), style, 2 /*UnitPixel*/, uintptr(unsafe.Pointer(&f)))
	return f
}

func newStringFormat(align uintptr) uintptr {
	var f uintptr
	pGdipCreateStringFmt.Call(0, 0, uintptr(unsafe.Pointer(&f)))
	pGdipSetStringAlign.Call(f, align)
	pGdipSetStringLineAlgn.Call(f, alignCenter)
	pGdipSetStringTrimming.Call(f, 3 /*EllipsisCharacter*/)
	return f
}

// gfx is a GDI+ Graphics handle. Colours are 0xAARRGGBB, coordinates pixels.
type gfx uintptr

// withGraphics runs draw against a 32-bit premultiplied-alpha pixel buffer.
func withGraphics(bits unsafe.Pointer, w, h int, draw func(gfx)) {
	var bmp, g uintptr
	pGdipBitmapFromScan0.Call(uintptr(w), uintptr(h), uintptr(w*4), pixelFormat32bppPARGB,
		uintptr(bits), uintptr(unsafe.Pointer(&bmp)))
	pGdipImageGraphics.Call(bmp, uintptr(unsafe.Pointer(&g)))
	pGdipSetSmoothingMode.Call(g, 4) // AntiAlias
	pGdipSetTextHint.Call(g, 3)      // AntiAliasGridFit (ClearType breaks on alpha)
	pGdipSetPixelOffset.Call(g, 4)   // Half
	draw(gfx(g))
	pGdipDeleteGraphics.Call(g)
	pGdipDisposeImage.Call(bmp)
}

func (g gfx) clear(c uint32) { pGdipGraphicsClear.Call(uintptr(g), uintptr(c)) }

func (g gfx) brush(c uint32, use func(b uintptr)) {
	var b uintptr
	pGdipCreateSolidFill.Call(uintptr(c), uintptr(unsafe.Pointer(&b)))
	use(b)
	pGdipDeleteBrush.Call(b)
}

func (g gfx) pen(c uint32, width float64, use func(p uintptr)) {
	var p uintptr
	pGdipCreatePen1.Call(uintptr(c), f32(width), 2 /*UnitPixel*/, uintptr(unsafe.Pointer(&p)))
	pGdipSetPenLineCap.Call(p, 2, 2, 2) // round caps
	use(p)
	pGdipDeletePen.Call(p)
}

func (g gfx) fillRect(c uint32, x, y, w, h int) {
	g.brush(c, func(b uintptr) {
		pGdipFillRectangleI.Call(uintptr(g), b, uintptr(x), uintptr(y), uintptr(w), uintptr(h))
	})
}

func (g gfx) fillCircle(c uint32, cx, cy, r int) {
	g.brush(c, func(b uintptr) {
		pGdipFillEllipseI.Call(uintptr(g), b, uintptr(cx-r), uintptr(cy-r), uintptr(2*r), uintptr(2*r))
	})
}

func (g gfx) line(c uint32, width float64, x0, y0, x1, y1 int) {
	g.pen(c, width, func(p uintptr) {
		pGdipDrawLineI.Call(uintptr(g), p, uintptr(x0), uintptr(y0), uintptr(x1), uintptr(y1))
	})
}

func roundPath(x, y, w, h, r int) uintptr {
	r = min(r, w/2, h/2)
	var p uintptr
	pGdipCreatePath.Call(0, uintptr(unsafe.Pointer(&p)))
	d := uintptr(2 * r)
	arc := func(ax, ay int, start float64) {
		pGdipAddPathArcI.Call(p, uintptr(ax), uintptr(ay), d, d, f32(start), f32(90))
	}
	arc(x, y, 180)
	arc(x+w-2*r, y, 270)
	arc(x+w-2*r, y+h-2*r, 0)
	arc(x, y+h-2*r, 90)
	pGdipClosePathFigure.Call(p)
	return p
}

func (g gfx) fillRound(c uint32, x, y, w, h, r int) {
	if w < 2 || h < 2 {
		return
	}
	path := roundPath(x, y, w, h, r)
	g.brush(c, func(b uintptr) { pGdipFillPath.Call(uintptr(g), b, path) })
	pGdipDeletePath.Call(path)
}

func (g gfx) strokeRound(c uint32, width float64, x, y, w, h, r int) {
	path := roundPath(x, y, w, h, r)
	g.pen(c, width, func(p uintptr) { pGdipDrawPath.Call(uintptr(g), p, path) })
	pGdipDeletePath.Call(path)
}

func (g gfx) text(s string, font uintptr, c uint32, x, y, w, h int, format uintptr) {
	u := windows.StringToUTF16(s)
	box := [4]float32{float32(x), float32(y), float32(w), float32(h)}
	g.brush(c, func(b uintptr) {
		pGdipDrawString.Call(uintptr(g), uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), font,
			uintptr(unsafe.Pointer(&box)), format, b)
	})
}
