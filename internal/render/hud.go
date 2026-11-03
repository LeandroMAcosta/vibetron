package render

import "vibetron/internal/sim"

// HUDHeight is the pixel height DrawHUD uses: two rows of headroom for
// accent marks (Á in ÁMBAR), five of text and one for the boost bar.
const HUDHeight = 8

// chipMinSlot is the narrowest slot that still fits the jump chip next to
// two initials; narrower slots show jump-ready by lighting the name.
const chipMinSlot = 14

// DrawHUD draws one slot per player across w pixels: a jump-ready chip,
// the name, win pips and a boost bar. Players out of the round are dimmed.
func DrawHUD(f *Frame, x, y, w int, s *sim.State, names []string) {
	for yy := y; yy < y+HUDHeight; yy++ {
		for xx := x; xx < x+w; xx++ {
			f.Set(xx, yy, bgColor)
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
		core, mid, off := pal.Core, pal.Mid, mix(pal.Mid, dimColor, 200)
		if !p.Alive {
			core, mid = mix(core, dimColor, 190), mix(mid, dimColor, 190)
		}
		sx, ty := x+i*slot, y+2

		ready := 1.0
		if cd := 4 * float64(s.Rate); cd > 0 {
			ready = 1 - float64(p.JumpCooldown)/cd
		}
		nameX, nameC := sx, mid
		if slot >= chipMinSlot {
			lit := int(ready * 5)
			for r := 0; r < 5; r++ {
				c := off
				switch {
				case ready >= 1:
					c = core
				case 4-r < lit:
					c = mid
				}
				f.Set(sx, ty+r, c)
			}
			nameX = sx + 2
		} else if ready >= 1 {
			nameC = core
		}

		for r := 0; r < sim.WinsToMatch && r < 5; r++ {
			c := off
			if r < int(p.Wins) {
				c = core
			}
			f.Set(sx+slot-2, ty+4-r, c)
		}

		name := pal.Name
		if i < len(names) && names[i] != "" {
			name = names[i]
		}
		DrawText(f, nameX, ty, fit(name, sx+slot-3-nameX), nameC, false)

		barW := slot - 1
		fill := int(p.Boost) * barW / sim.BoostMax
		for b := 0; b < barW; b++ {
			c := off
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
