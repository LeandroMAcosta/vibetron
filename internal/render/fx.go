package render

import "math"

var sinTab [256]float32

func init() {
	for i := range sinTab {
		sinTab[i] = float32(math.Sin(float64(i) * 2 * math.Pi / 256))
	}
}

// sin1 is a table sine taking turns (1.0 = full cycle), for per-pixel waves.
func sin1(turns float32) float32 {
	return sinTab[uint8(int32(turns*256))]
}

// present copies the arena buffer to the frame with screen shake, the
// round flash and the CRT vignette. The vignette is indexed by destination
// so the screen edges stay dark while the picture shakes under them.
func (a *Arena) present(f *Frame, ox, oy int) {
	sx, sy := 0, 0
	if a.shake > 0.2 {
		sx = int(math.Round(float64(a.rng.s()) * a.shake))
		sy = int(math.Round(float64(a.rng.s()) * a.shake * 0.6))
	}
	fl := int(a.flash * 180)
	flash := scale(a.flashColor, fl)
	for y := 0; y < ah; y++ {
		fy := oy + y
		if fy < 0 || fy >= f.H {
			continue
		}
		srcY := y - sy
		for x := 0; x < aw; x++ {
			fx := ox + x
			if fx < 0 || fx >= f.W {
				continue
			}
			c := black
			if srcX := x - sx; srcX >= 0 && srcX < aw && srcY >= 0 && srcY < ah {
				c = a.buf[srcY*aw+srcX]
			}
			if fl > 0 {
				c = add(c, flash)
			}
			f.Pix[fy*f.W+fx] = scale(c, int(a.vig[y*aw+x]))
		}
	}
}

func decay(v, rate, dt float64) float64 {
	return v * math.Exp(-rate*dt)
}
