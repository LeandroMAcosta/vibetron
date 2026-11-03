package render

type glyph struct {
	w, h uint8
	rows [7]uint8 // bit (w-1-x) lit
	ok   bool
}

type font struct {
	ascii      [128]glyph
	inv, invQ  glyph // ¡ and ¿
	accentSize int
}

var fonts [2]font

func init() {
	for i, src := range [2]map[rune]string{smallGlyphs, bigGlyphs} {
		f := &fonts[i]
		f.accentSize = i
		for r, s := range src {
			g := parseGlyph(s)
			switch {
			case r < 128:
				f.ascii[r] = g
			case r == '¡':
				f.inv = g
			case r == '¿':
				f.invQ = g
			}
		}
	}
}

func parseGlyph(s string) glyph {
	var g glyph
	x := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ':
			g.h++
			x = 0
			continue
		case '#':
			g.rows[g.h] |= 1 << x
		}
		x++
		if uint8(x) > g.w {
			g.w = uint8(x)
		}
	}
	g.h++
	g.ok = true
	for y := uint8(0); y < g.h; y++ {
		g.rows[y] = reverseBits(g.rows[y], g.w)
	}
	return g
}

func reverseBits(v, w uint8) uint8 {
	var out uint8
	for x := uint8(0); x < w; x++ {
		if v&(1<<x) != 0 {
			out |= 1 << (w - 1 - x)
		}
	}
	return out
}

func (f *font) lookup(r rune) (*glyph, accent) {
	r, ac := baseRune(r)
	switch {
	case r < 128 && r >= 0 && f.ascii[r].ok:
		return &f.ascii[r], ac
	case r == '¡':
		return &f.inv, ac
	case r == '¿':
		return &f.invQ, ac
	}
	return &f.ascii['?'], noAccent
}

func pickFont(big bool) *font {
	if big {
		return &fonts[1]
	}
	return &fonts[0]
}

// DrawText draws s with its glyph tops at y and returns the width drawn.
// Accent marks rise two or three pixels above y.
func DrawText(f *Frame, x, y int, s string, c RGB, big bool) int {
	return drawText(f, x, y, s, c, pickFont(big), 1)
}

func TextWidth(s string, big bool) int {
	return textWidth(s, pickFont(big), 1)
}

// DrawTextScaled draws the big font with each pixel as a scale x scale block.
func DrawTextScaled(f *Frame, x, y int, s string, c RGB, scale int) {
	if scale < 1 {
		scale = 1
	}
	drawText(f, x, y, s, c, &fonts[1], scale)
}

// TextWidthScaled is TextWidth for DrawTextScaled.
func TextWidthScaled(s string, scale int) int {
	if scale < 1 {
		scale = 1
	}
	return textWidth(s, &fonts[1], scale)
}

func textWidth(s string, fn *font, k int) int {
	w, n := 0, 0
	for _, r := range s {
		g, _ := fn.lookup(r)
		w += int(g.w) * k
		n++
	}
	if n > 1 {
		w += (n - 1) * k
	}
	return w
}

func drawText(f *Frame, x, y int, s string, c RGB, fn *font, k int) int {
	start := x
	for i, r := range s {
		if i > 0 {
			x += k
		}
		g, ac := fn.lookup(r)
		for gy := 0; gy < int(g.h); gy++ {
			row := g.rows[gy]
			for gx := 0; gx < int(g.w); gx++ {
				if row&(1<<(int(g.w)-1-gx)) != 0 {
					block(f, x+gx*k, y+gy*k, k, c)
				}
			}
		}
		for _, d := range accentDots[fn.accentSize][ac] {
			block(f, x+int(d.x)*k, y+int(d.y)*k, k, c)
		}
		x += int(g.w) * k
	}
	return x - start
}

func block(f *Frame, x, y, k int, c RGB) {
	for dy := 0; dy < k; dy++ {
		for dx := 0; dx < k; dx++ {
			f.Set(x+dx, y+dy, c)
		}
	}
}
