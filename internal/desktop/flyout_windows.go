package desktop

import (
	"strconv"
	"time"
	"unsafe"

	"golang.org/x/sys/windows/registry"

	"sonar-remote/internal/sonar"
)

// The flyout is one custom-drawn window: six channel rows (mute button, name,
// slider, value) and a ChatMix row. Layout constants are in 96-DPI units and
// scaled by px(); everything else is in device pixels.
const (
	nChan   = len(sonar.Channels)
	chatRow = nChan // row index of the ChatMix slider

	winW, padY, rowH, sepH = 360, 8, 44, 9
	winH                   = padY + nChan*rowH + sepH + rowH + padY
	iconX, iconSz          = 10, 36
	nameX                  = 56
	railX0, railX1         = 124, 296
	valueW, valueR         = 40, 344

	partNone, partIcon, partSlider = 0, 1, 2

	// How long a value the user just set wins over what Sonar reports back.
	holdFor = 1500 * time.Millisecond
)

var (
	names = [nChan]string{"Master", "Jeu", "Chat", "Média", "Aux", "Micro"}
	// Segoe Fluent Icons: Volume, Game, Message, Audio, Equalizer, Microphone.
	glyphs = [nChan]string{"", "", "", "", "", ""}
)

type theme struct {
	light                                                          bool
	bg, text, dim, rail, railOff, thumb, edge, hover, line, accent uint32
}

func regDWORD(path, name string, def uint64) uint64 {
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err != nil {
		return def
	}
	defer k.Close()
	if v, _, err := k.GetIntegerValue(name); err == nil {
		return v
	}
	return def
}

// systemLight reports whether the taskbar and shell flyouts use the light theme.
func systemLight() bool {
	return regDWORD(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, "SystemUsesLightTheme", 0) == 1
}

// loadTheme mirrors the Windows 11 control colours for the current theme and
// accent, so the flyout looks like the system's own.
func loadTheme() theme {
	t := theme{
		bg: 0xFF2C2C2C, text: 0xFFFFFFFF, dim: 0xC5FFFFFF, rail: 0x8BFFFFFF, railOff: 0x28FFFFFF,
		thumb: 0xFF454545, edge: 0x33000000, hover: 0x0FFFFFFF, line: 0x15FFFFFF, accent: 0xFF4CC2FF,
	}
	shade := 1 // AccentPalette entry: Light2 on dark
	if systemLight() {
		t = theme{
			light: true,
			bg:    0xFFF3F3F3, text: 0xE4000000, dim: 0x9E000000, rail: 0x72000000, railOff: 0x37000000,
			thumb: 0xFFFFFFFF, edge: 0x30000000, hover: 0x0C000000, line: 0x12000000, accent: 0xFF005FB8,
		}
		shade = 4 // Dark1 on light
	}
	// AccentPalette is 8 RGBA entries: Light3, Light2, Light1, Accent, Dark1...
	if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\Accent`, registry.QUERY_VALUE); err == nil {
		if p, _, err := k.GetBinaryValue("AccentPalette"); err == nil && len(p) >= 32 {
			c := p[shade*4:]
			t.accent = 0xFF000000 | uint32(c[0])<<16 | uint32(c[1])<<8 | uint32(c[2])
		}
		k.Close()
	}
	return t
}

func (a *app) px(d int) int { return int(float64(d)*a.scale + 0.5) }

func rowTop(i int) int {
	if i == chatRow {
		return padY + nChan*rowH + sepH
	}
	return padY + i*rowH
}

// hit returns the row and part under a client-area point (row -1 if none).
func (a *app) hit(x, y int) (row, part int) {
	if a.connect {
		return -1, partNone
	}
	for i := 0; i <= chatRow; i++ {
		top := a.px(rowTop(i))
		if y < top || y >= top+a.px(rowH) {
			continue
		}
		switch {
		case i != chatRow && x >= a.px(iconX) && x < a.px(iconX+iconSz):
			return i, partIcon
		case x >= a.px(railX0-12) && x <= a.px(railX1+12):
			return i, partSlider
		}
		return i, partNone
	}
	return -1, partNone
}

// sliderValue maps a pointer x to 0..1 along a rail spanning [x0, x1].
func sliderValue(x, x0, x1 int) float64 {
	return min(1, max(0, float64(x-x0)/float64(x1-x0)))
}

// balanceFrom converts a 0..1 slider position to a ChatMix balance (-1..1),
// snapping to the centre so "no bias" is easy to hit.
func balanceFrom(v float64) float64 {
	b := v*2 - 1
	if b > -0.04 && b < 0.04 {
		return 0
	}
	return b
}

// value returns row i's slider position, 0..1.
func (a *app) value(i int) float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	if i == chatRow {
		return (a.st.ChatMix + 1) / 2
	}
	return a.st.Levels[i].Volume
}

// setValue applies a slider position locally at once and queues the write.
func (a *app) setValue(i int, v float64) {
	v = min(1, max(0, v))
	a.mu.Lock()
	a.hold[i] = time.Now().Add(holdFor)
	if i == chatRow {
		b := balanceFrom(v)
		a.st.ChatMix = b
		a.mu.Unlock()
		a.send("mix", func() error { return a.sonar.SetChatMix(b) })
	} else {
		a.st.Levels[i].Volume = v
		a.mu.Unlock()
		id := sonar.Channels[i]
		a.send("vol"+id, func() error { return a.sonar.SetVolume(id, v) })
	}
	a.stateChanged()
}

func (a *app) toggleMute(i int) {
	if i == chatRow {
		return
	}
	a.mu.Lock()
	a.hold[i] = time.Now().Add(holdFor)
	m := !a.st.Levels[i].Muted
	a.st.Levels[i].Muted = m
	a.mu.Unlock()
	id := sonar.Channels[i]
	a.send("mute"+id, func() error { return a.sonar.SetMute(id, m) })
	a.stateChanged()
}

func (a *app) redraw() { pInvalidateRect.Call(a.hwnd, 0, 0) }

// ── Input ───────────────────────────────────────────────────────────────────

func (a *app) dragTo(x int) {
	a.setValue(a.drag, sliderValue(x, a.px(railX0), a.px(railX1)))
}

func (a *app) onMouseDown(x, y int) {
	a.kbd = false
	switch row, part := a.hit(x, y); part {
	case partIcon:
		a.toggleMute(row)
	case partSlider:
		a.drag = row
		pSetCapture.Call(a.hwnd)
		a.dragTo(x)
	}
}

func (a *app) onMouseMove(x, y int) {
	if a.drag >= 0 {
		a.dragTo(x)
		return
	}
	if !a.tracking {
		tme := trackMouseEvent{Flags: tmeLeave, Hwnd: a.hwnd}
		tme.CbSize = uint32(unsafe.Sizeof(tme))
		pTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
		a.tracking = true
	}
	if row, part := a.hit(x, y); row != a.hover || part != a.hoverPart {
		a.hover, a.hoverPart = row, part
		a.redraw()
	}
}

func (a *app) onWheel(x, y, notches int) {
	if row, _ := a.hit(x, y); row >= 0 {
		a.setValue(row, a.value(row)+0.02*float64(notches))
	}
}

func (a *app) onKey(vk uintptr) {
	if a.connect {
		if vk == vkEscape {
			a.hide()
		}
		return
	}
	if a.focus < 0 || !a.kbd { // first key press only reveals the focus ring
		a.focus, a.kbd = max(a.focus, 0), true
		if vk != vkEscape {
			a.redraw()
			return
		}
	}
	switch vk {
	case vkEscape:
		a.hide()
	case vkDown, vkTab:
		a.focus = (a.focus + 1) % (chatRow + 1)
	case vkUp:
		a.focus = (a.focus + chatRow) % (chatRow + 1)
	case vkLeft:
		a.setValue(a.focus, a.value(a.focus)-0.02)
	case vkRight:
		a.setValue(a.focus, a.value(a.focus)+0.02)
	case vkSpace, vkM:
		a.toggleMute(a.focus)
	}
	a.redraw()
}

// ── Painting ────────────────────────────────────────────────────────────────

func (a *app) paint(g gfx) {
	a.mu.Lock()
	st, online := a.st, a.online
	a.mu.Unlock()
	t := a.th

	if a.acrylic {
		g.clear(0) // fully transparent: the system backdrop shows through
	} else {
		g.clear(t.bg)
	}
	w, h := a.px(winW), a.px(winH)
	if a.connect {
		a.paintConnect(g)
		return
	}
	if !online {
		g.text("Sonar est introuvable", a.fBold, t.text, 0, h/2-a.px(34), w, a.px(24), a.fmtCenter)
		g.text("Lance SteelSeries GG : le mixer se reconnecte tout seul.", a.fText, t.dim,
			a.px(48), h/2-a.px(8), w-a.px(96), a.px(44), a.fmtCenter)
		return
	}

	for i, l := range st.Levels {
		top, cy := a.px(rowTop(i)), a.px(rowTop(i))+a.px(rowH)/2
		a.paintFocus(g, i)
		ink := t.text
		if l.Muted {
			ink = t.dim
		}
		iy := cy - a.px(iconSz)/2
		if a.hover == i && a.hoverPart == partIcon {
			g.fillRound(t.hover, a.px(iconX), iy, a.px(iconSz), a.px(iconSz), a.px(4))
		}
		g.text(glyphs[i], a.fIcon, ink, a.px(iconX), iy+a.px(1), a.px(iconSz), a.px(iconSz), a.fmtCenter)
		if l.Muted {
			cx := a.px(iconX + iconSz/2)
			g.line(t.text, a.scale*1.5, cx-a.px(8), cy-a.px(8), cx+a.px(8), cy+a.px(8))
		}
		g.text(names[i], a.fText, ink, a.px(nameX), top, a.px(railX0-nameX-10), a.px(rowH), a.fmtLeft)
		a.paintSlider(g, i, cy, 0, l.Volume, l.Muted)
		g.text(strconv.Itoa(int(l.Volume*100+0.5)), a.fText, t.dim,
			a.px(valueR-valueW), top, a.px(valueW), a.px(rowH), a.fmtRight)
	}

	g.fillRect(t.line, a.px(16), a.px(padY+nChan*rowH+sepH/2), w-a.px(32), max(1, a.px(1)))

	top, cy := a.px(rowTop(chatRow)), a.px(rowTop(chatRow))+a.px(rowH)/2
	ink := t.text
	if !st.HasChatMix {
		ink = t.dim
	}
	a.paintFocus(g, chatRow)
	// Game on the left, chat on the right: the two ends of the balance.
	g.text(glyphs[1], a.fIcon, t.dim, a.px(iconX), top, a.px(iconSz), a.px(rowH), a.fmtCenter)
	g.text("ChatMix", a.fText, ink, a.px(nameX), top, a.px(railX0-nameX-10), a.px(rowH), a.fmtLeft)
	g.fillRect(t.rail, a.px((railX0+railX1)/2), cy-a.px(6), max(1, a.px(1)), a.px(12))
	a.paintSlider(g, chatRow, cy, 0.5, (st.ChatMix+1)/2, !st.HasChatMix)
	g.text(glyphs[2], a.fIcon, t.dim, a.px(valueR-iconSz+4), top, a.px(iconSz), a.px(rowH), a.fmtCenter)
}

// paintConnect draws the QR code of the web remote's LAN address, with the
// addresses spelled out underneath for devices that can't scan it.
func (a *app) paintConnect(g gfx) {
	t, w, h := a.th, a.px(winW), a.px(winH)
	if a.code == nil {
		g.text("Aucun réseau local", a.fBold, t.text, 0, h/2-a.px(34), w, a.px(24), a.fmtCenter)
		g.text("Connecte ce PC au Wi-Fi ou au réseau, puis rouvre ce panneau.", a.fText, t.dim,
			a.px(48), h/2-a.px(8), w-a.px(96), a.px(44), a.fmtCenter)
		return
	}
	// Whole-pixel modules keep the code sharp; 4 modules of white margin all
	// round is what scanners expect. Always dark on white, whatever the theme.
	n := a.code.Size
	box := a.px(208)
	m := max(1, box/(n+8))
	side := m * (n + 8)
	x0, y0 := (w-side)/2, a.px(16)+(box-side)/2
	g.fillRound(0xFFFFFFFF, x0, y0, side, side, a.px(8))
	g.brush(0xFF000000, func(b uintptr) {
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				if a.code.Black(x, y) {
					pGdipFillRectangleI.Call(uintptr(g), b, uintptr(x0+(x+4)*m), uintptr(y0+(y+4)*m), uintptr(m), uintptr(m))
				}
			}
		}
	})
	y := a.px(16) + box + a.px(8)
	g.text(a.link, a.fBold, t.text, 0, y, w, a.px(24), a.fmtCenter)
	hint := "Scanne ce code depuis le même réseau."
	if a.host != "" {
		hint = "ou " + a.url(a.host+".local") + "\n" + hint
	}
	g.text(hint, a.fText, t.dim, a.px(24), y+a.px(26), w-a.px(48), a.px(60), a.fmtCenter)
}

func (a *app) paintFocus(g gfx, i int) {
	if a.kbd && a.focus == i {
		g.strokeRound(a.th.text, a.scale*2, a.px(4), a.px(rowTop(i))+a.px(3), a.px(winW-8), a.px(rowH-6), a.px(6))
	}
}

// paintSlider draws a rail filled between two 0..1 positions with the thumb at
// the second one. off greys it out (muted channel, ChatMix unavailable).
func (a *app) paintSlider(g gfx, i, cy int, from, to float64, off bool) {
	t := a.th
	x0, x1, rh := a.px(railX0), a.px(railX1), a.px(4)
	at := func(v float64) int { return x0 + int(v*float64(x1-x0)+0.5) }
	rail, fill := t.rail, t.accent
	if off {
		rail, fill = t.railOff, t.dim
	}
	g.fillRound(rail, x0, cy-rh/2, x1-x0, rh, rh/2)
	if lo, hi := at(min(from, to)), at(max(from, to)); !off {
		g.fillRound(fill, lo, cy-rh/2, hi-lo, rh, rh/2)
	}
	tx, dot := at(to), 5.0
	switch {
	case a.drag == i:
		dot = 4.5
	case a.hover == i && a.hoverPart == partSlider:
		dot = 6.5
	}
	g.fillCircle(t.edge, tx, cy, a.px(10)+1)
	g.fillCircle(t.thumb, tx, cy, a.px(10))
	g.fillCircle(fill, tx, cy, int(dot*a.scale+0.5))
}
