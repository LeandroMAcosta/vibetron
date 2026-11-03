package render

import "estela/internal/sim"

// HUDHeight is the pixel height DrawHUD uses: two rows of headroom for
// accent marks (Á in ÁMBAR), five of text and one for the boost bar.
const HUDHeight = 8

var hudBg = RGB{6, 4, 16}

// DrawHUD draws one slot per player across w pixels: a jump-ready chip,
// the name, win pips and a boost bar. Players out of the round are dimmed.
func DrawHUD(f *Frame, x, y, w int, s *sim.State, names []string) {
	for yy := y; yy < y+HUDHeight; yy++ {
		for xx := x; xx < x+w; xx++ {
			f.Set(xx, yy, hudBg)
		}
	}
	n := int(s.Count)
	if n == 0 {
		return
	}
	slot := w / n
	for i := 0; i < n; i++ {
		p := &s.Players[i]
		pal := Players[i]
		dim := 256
		if !p.Alive {
			dim = 90
		}
		core, mid, glow := scale(pal.Core, dim), scale(pal.Mid, dim), scale(pal.Glow, dim)
		sx, ty := x+i*slot, y+2

		ready := 1 - float64(p.JumpCooldown)/sim.JumpCooldown
		lit := int(ready * 5)
		for r := 0; r < 5; r++ {
			c := glow
			switch {
			case ready >= 1:
				c = core
			case 4-r < lit:
				c = mid
			}
			f.Set(sx, ty+r, c)
		}

		for r := 0; r < sim.WinsToMatch && r < 5; r++ {
			c := scale(glow, 140)
			if r < int(p.Wins) {
				c = core
			}
			f.Set(sx+slot-2, ty+4-r, c)
		}

		name := pal.Name
		if i < len(names) && names[i] != "" {
			name = names[i]
		}
		DrawText(f, sx+2, ty, fit(name, slot-5), mid, false)

		barW := slot - 1
		fill := int(p.Boost) * barW / sim.BoostMax
		for b := 0; b < barW; b++ {
			c := scale(glow, 120)
			if b < fill {
				c = mid
				if b == fill-1 || p.Boosting() {
					c = core
				}
			}
			f.Set(sx+b, ty+5, c)
		}
	}
}

// fit cuts s to the longest rune prefix whose small-font width is <= w.
func fit(s string, w int) string {
	fn := &fonts[0]
	acc := 0
	for i, r := range s {
		g, _ := fn.lookup(r)
		next := acc + int(g.w)
		if i > 0 {
			next++
		}
		if next > w {
			return s[:i]
		}
		acc = next
	}
	return s
}
