package render

// glowBuf holds a separable 5-tap binomial blur (1 4 6 4 1) of the emission
// layer. Each pass sums to 16, so two passes fit a uint16 per channel.
type glowBuf struct {
	tmp [cells][3]uint16
	out [cells][3]uint16
}

func (g *glowBuf) blur(src *[cells]RGB) {
	for y := 0; y < ah; y++ {
		row := y * aw
		for x := 0; x < aw; x++ {
			var r, gg, b uint16
			for k := -2; k <= 2; k++ {
				xx := x + k
				if xx < 0 || xx >= aw {
					continue
				}
				c := src[row+xx]
				w := binom[k+2]
				r += uint16(c.R) * w
				gg += uint16(c.G) * w
				b += uint16(c.B) * w
			}
			g.tmp[row+x] = [3]uint16{r, gg, b}
		}
	}
	for y := 0; y < ah; y++ {
		for x := 0; x < aw; x++ {
			var r, gg, b uint16
			for k := -2; k <= 2; k++ {
				yy := y + k
				if yy < 0 || yy >= ah {
					continue
				}
				c := g.tmp[yy*aw+x]
				w := binom[k+2]
				r += c[0] * w
				gg += c[1] * w
				b += c[2] * w
			}
			g.out[y*aw+x] = [3]uint16{r, gg, b}
		}
	}
}

var binom = [5]uint16{1, 4, 6, 4, 1}

// addGlow adds the halo scaled by gain/256 on top of the blur's own /256.
// Emitting cells take a quarter of it: full gain would bleach every trail
// to white, and the halo should sit around the line, not on it.
func (g *glowBuf) addGlow(dst, emit *[cells]RGB, gain int) {
	for i := range dst {
		o := g.out[i]
		d := &dst[i]
		k := gain
		if emit[i] != black {
			k >>= 2
		}
		d.R = sat(int(d.R) + int(o[0])*k>>16)
		d.G = sat(int(d.G) + int(o[1])*k>>16)
		d.B = sat(int(d.B) + int(o[2])*k>>16)
	}
}
