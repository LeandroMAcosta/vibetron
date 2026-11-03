package ui

import (
	"fmt"
	"math"
	"strings"

	"vibetron/internal/render"
)

var (
	white  = render.RGB{R: 245, G: 245, B: 255}
	dim    = render.RGB{R: 120, G: 110, B: 170}
	accent = render.RGB{R: 255, G: 70, B: 200}
	danger = render.RGB{R: 255, G: 80, B: 90}
)

var shadow = render.RGB{R: 8, G: 4, B: 20}

// centerText draws s centered at y with a drop shadow, wrapping words when
// the line is wider than the frame. It returns the height it used.
func centerText(f *render.Frame, y int, s string, c render.RGB, big bool) int {
	lineH := 6
	if big {
		lineH = 9
	}
	h := 0
	for _, line := range wrap(s, f.W-4, big) {
		x := (f.W - render.TextWidth(line, big)) / 2
		render.DrawText(f, x+1, y+h+1, line, shadow, big)
		render.DrawText(f, x, y+h, line, c, big)
		h += lineH
	}
	return h
}

func wrap(s string, width int, big bool) []string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(s) {
		next := w
		if line != "" {
			next = line + " " + w
		}
		if line != "" && render.TextWidth(next, big) > width {
			lines = append(lines, line)
			next = w
		}
		line = next
	}
	return append(lines, line)
}

func centerScaled(f *render.Frame, y int, s string, c render.RGB, scale int) {
	w := render.TextWidthScaled(s, scale)
	for scale > 1 && w > f.W-4 {
		scale--
		w = render.TextWidthScaled(s, scale)
	}
	render.DrawTextScaled(f, (f.W-w)/2+1, y+1, s, shadow, scale)
	render.DrawTextScaled(f, (f.W-w)/2, y, s, c, scale)
}

// pulse returns c brightened and dimmed over time, for selected items.
func pulse(c render.RGB, t float64) render.RGB {
	k := 0.75 + 0.25*math.Sin(t*6)
	return render.RGB{R: uint8(float64(c.R) * k), G: uint8(float64(c.G) * k), B: uint8(float64(c.B) * k)}
}

func drawTooSmall(a *App, f *render.Frame, cols, rows, needCols, needRows int) {
	render.Backdrop(f, a.now)
	y := f.H/2 - 8
	if y < 0 {
		y = 0
	}
	y += centerText(f, y, "AGRANDÁ LA TERMINAL", accent, false) + 3
	y += centerText(f, y, fmt.Sprintf("MÍNIMO %dX%d", needCols, needRows), white, false) + 1
	centerText(f, y, fmt.Sprintf("AHORA %dX%d", cols, rows), dim, false)
}
