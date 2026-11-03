// Package render draws ESTELA into an RGB pixel framebuffer and blits it to
// the terminal as half-block cells, two pixels per cell.
package render

import "github.com/gdamore/tcell/v2"

type RGB struct{ R, G, B uint8 }

// Frame is a pixel buffer. H is even so every terminal cell maps to exactly
// one top and one bottom pixel.
type Frame struct {
	W, H int
	Pix  []RGB
}

const halfBlock = "▀"

func NewFrame(w, h int) *Frame {
	h += h & 1
	return &Frame{W: w, H: h, Pix: make([]RGB, w*h)}
}

func (f *Frame) Clear(c RGB) {
	for i := range f.Pix {
		f.Pix[i] = c
	}
}

func (f *Frame) Set(x, y int, c RGB) {
	if uint(x) < uint(f.W) && uint(y) < uint(f.H) {
		f.Pix[y*f.W+x] = c
	}
}

// Add saturates per channel, so overlapping particles burn toward white
// instead of wrapping.
func (f *Frame) Add(x, y int, c RGB) {
	if uint(x) < uint(f.W) && uint(y) < uint(f.H) {
		p := &f.Pix[y*f.W+x]
		*p = add(*p, c)
	}
}

func (f *Frame) Blend(x, y int, c RGB, a uint8) {
	if uint(x) < uint(f.W) && uint(y) < uint(f.H) {
		p := &f.Pix[y*f.W+x]
		*p = mix(*p, c, a)
	}
}

// Blit writes the frame at cell (ox, oy). Put with a constant string is used
// instead of SetContent, which allocates a rune slice and string per cell.
func (f *Frame) Blit(s tcell.Screen, ox, oy int) {
	sw, sh := s.Size()
	for cy := 0; cy < f.H/2; cy++ {
		y := oy + cy
		if y < 0 || y >= sh {
			continue
		}
		top := f.Pix[2*cy*f.W : (2*cy+1)*f.W]
		bot := f.Pix[(2*cy+1)*f.W : (2*cy+2)*f.W]
		for cx := 0; cx < f.W; cx++ {
			x := ox + cx
			if x < 0 || x >= sw {
				continue
			}
			st := tcell.StyleDefault.Foreground(color(top[cx])).Background(color(bot[cx]))
			s.Put(x, y, halfBlock, st)
		}
	}
}

func color(c RGB) tcell.Color {
	return tcell.NewRGBColor(int32(c.R), int32(c.G), int32(c.B))
}

func add(a, b RGB) RGB {
	return RGB{sat(int(a.R) + int(b.R)), sat(int(a.G) + int(b.G)), sat(int(a.B) + int(b.B))}
}

func sat(v int) uint8 {
	if v > 255 {
		return 255
	}
	if v < 0 {
		return 0
	}
	return uint8(v)
}

func mix(a, b RGB, t uint8) RGB {
	k, j := int(t), 255-int(t)
	return RGB{
		uint8((int(a.R)*j + int(b.R)*k) / 255),
		uint8((int(a.G)*j + int(b.G)*k) / 255),
		uint8((int(a.B)*j + int(b.B)*k) / 255),
	}
}

// scale multiplies by k/256.
func scale(c RGB, k int) RGB {
	if k < 0 {
		k = 0
	}
	return RGB{sat(int(c.R) * k >> 8), sat(int(c.G) * k >> 8), sat(int(c.B) * k >> 8)}
}

func lerp(a, b RGB, t float64) RGB {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	return mix(a, b, uint8(t*255))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
