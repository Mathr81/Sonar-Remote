package desktop

import "testing"

func TestSliderValue(t *testing.T) {
	for _, c := range []struct {
		x    int
		want float64
	}{{100, 0}, {150, 0.5}, {200, 1}, {40, 0}, {900, 1}} { // outside the rail clamps
		if got := sliderValue(c.x, 100, 200); got != c.want {
			t.Errorf("sliderValue(%d) = %v, want %v", c.x, got, c.want)
		}
	}
}

func TestBalanceSnapsToCentre(t *testing.T) {
	for _, c := range []struct{ pos, want float64 }{{0, -1}, {1, 1}, {0.5, 0}, {0.51, 0}, {0.75, 0.5}} {
		if got := balanceFrom(c.pos); got != c.want {
			t.Errorf("balanceFrom(%v) = %v, want %v", c.pos, got, c.want)
		}
	}
}

func TestHitFindsRowsAndParts(t *testing.T) {
	a := &app{scale: 1.5}
	y := a.px(rowTop(4) + rowH/2) // Aux
	for _, c := range []struct {
		x, y, row, part int
	}{
		{a.px(iconX + 5), y, 4, partIcon},
		{a.px(railX0 + 20), y, 4, partSlider},
		{a.px(nameX + 5), y, 4, partNone},
		{a.px(railX0 + 20), a.px(rowTop(chatRow) + 5), chatRow, partSlider},
		{a.px(iconX + 5), a.px(rowTop(chatRow) + 5), chatRow, partNone}, // ChatMix has no mute
		{10, a.px(padY+nChan*rowH) + 2, -1, partNone},                   // separator gap
	} {
		if row, part := a.hit(c.x, c.y); row != c.row || part != c.part {
			t.Errorf("hit(%d,%d) = %d,%d, want %d,%d", c.x, c.y, row, part, c.row, c.part)
		}
	}
}
