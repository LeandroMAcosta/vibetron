package render

import (
	"math"

	"estela/internal/sim"
)

const glowGain = 860 // thin 1px lines lose most of their energy to the blur

// Draw renders the arena at (ox, oy). alpha in [0,1] is how far real time is
// between the last sim tick and the next; t is seconds and drives every
// effect, so animation stays smooth no matter the sim rate.
func (a *Arena) Draw(f *Frame, ox, oy int, s *sim.State, alpha float64, t float64) {
	a.sync(s)
	dt := t - a.now
	if !a.drawn || dt < 0 || dt > 0.1 {
		dt = 1.0 / 60
	}
	a.now, a.drawn = t, true
	a.shake = decay(a.shake, 9, dt)
	a.flash = decay(a.flash, 4, dt)
	a.collapse = decay(a.collapse, 2.5, dt)
	for i := range a.charge {
		a.charge[i] = float32(decay(float64(a.charge[i]), 2, dt))
	}
	a.parts.update(float32(dt))
	a.embers(s, alpha, float32(dt))

	// Ambient waves advance at 15 Hz: they are slow enough to look smooth,
	// and tcell then resends the whole arena on only one frame in four.
	a.paintCells(s, float32(math.Floor(t*15)/15), float32(t))
	a.glow.blur(&a.emit)
	a.glow.addGlow(&a.buf, &a.emit, glowGain)
	a.paintSparks(s, float32(t))
	a.paintHeads(s, alpha, float32(t))
	a.parts.draw(a.buf[:], aw, ah)
	a.present(f, ox, oy)
}

// paintCells writes the base color of every cell into buf and its light
// into emit, which the glow pass then spreads.
func (a *Arena) paintCells(s *sim.State, t, now float32) {
	var meltR [sim.MaxPlayers]float32
	for p := range a.melt {
		if m := &a.melt[p]; m.active {
			meltR[p] = float32(float64(now)-m.start) * dissolveSpeed
		}
	}
	// The grid breathes within a narrow band so that, under tcell's 256-color
	// fallback, lines stay on one palette entry instead of flickering.
	gridPulse := 0.85 + 0.1*sin1(t*0.15)
	stripeShift := int(t * 12)
	wallK := 0.5 + 0.5*float32(a.collapse)
	for y := 0; y < ah; y++ {
		for x := 0; x < aw; x++ {
			i := y*aw + x
			c := s.Grid[i]
			if c != a.grid[i] {
				a.grid[i], a.birth[i] = c, t
			}
			switch {
			case c == sim.Empty || c == sim.Spark:
				col := a.bg[i]
				if gw := a.gridW[i]; gw > 0 {
					wave := gridPulse + 0.15*sin1(t*0.4-float32(x+y)*0.012)
					col = add(col, scale(gridLine, int(float32(gw)*wave*110)))
				}
				a.buf[i], a.emit[i] = col, black
			case c == sim.Wall:
				ring := min(x, y, aw-1-x, ah-1-y)
				pulse := 0.6 + 0.4*sin1(t*0.8-float32(ring)*0.07)
				k := clampf(pulse*0.75+wallK*0.5, 0, 1)
				col := lerp(wallCold, wallHot, float64(k))
				if (x+y+stripeShift)&3 < 2 {
					col = scale(col, 120)
				}
				a.buf[i], a.emit[i] = col, scale(wallHot, int(k*150))
			default:
				o, _ := c.Owner()
				a.paintTrail(s, i, x, y, o, t, meltR[o])
			}
		}
	}
	for p := range a.melt {
		if m := &a.melt[p]; m.active {
			m.prevR = meltR[p]
			if meltR[p] > m.maxR+dissolveEdge {
				m.active = false
			}
		}
	}
}

func (a *Arena) paintTrail(s *sim.State, i, x, y int, o uint8, t, r float32) {
	pal := &Players[o]
	if !s.Players[o].Alive {
		m := &a.melt[o]
		ghost := scale(pal.Glow, 170)
		if !m.active {
			a.buf[i], a.emit[i] = ghost, scale(pal.Glow, 90)
			return
		}
		dx, dy := float32(x)-m.x, float32(y)-m.y
		d := float32(math.Sqrt(float64(dx*dx+dy*dy))) + float32(a.jitter[i])/32
		switch {
		case d <= r-dissolveEdge:
			a.buf[i], a.emit[i] = ghost, scale(pal.Glow, 90)
			return
		case d <= r:
			a.buf[i], a.emit[i] = white, pal.Core
			if d > m.prevR && a.jitter[i]&3 == 0 {
				n := d + 0.01
				a.parts.spawn(float32(x), float32(y), dx/n*18+a.rng.s()*8, dy/n*18+a.rng.s()*8, 0.5+0.4*a.rng.f(), 2, pal.Mid)
			}
			return
		}
	}
	hot := float32(math.Exp(float64(-(t - a.birth[i]) * 2.2)))
	flow := 0.88 + 0.12*sin1(t*0.9-float32(x+y)*0.045)
	col := scale(lerp(pal.Mid, pal.Core, float64(hot*0.9)), int(flow*256))
	a.buf[i], a.emit[i] = col, add(pal.Mid, scale(pal.Core, int(hot*120)))
}

// paintSparks draws pickups as twinkling four-point stars over the glow.
func (a *Arena) paintSparks(s *sim.State, t float32) {
	for i, c := range s.Grid {
		if c != sim.Spark {
			continue
		}
		x, y := i%aw, i/aw
		tw := 0.5 + 0.5*sin1(t*1.6+float32(a.jitter[i])/97)
		a.buf[i] = lerp(sparkGold, white, float64(tw))
		arm := scale(sparkGold, int(80+150*tw))
		far := scale(sparkGold, int(60*tw))
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			addAt(&a.buf, x+d[0], y+d[1], arm)
			addAt(&a.buf, x+2*d[0], y+2*d[1], far)
		}
		halo := scale(RGB{120, 90, 20}, int(160*tw))
		for _, d := range [4][2]int{{1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
			addAt(&a.buf, x+d[0], y+d[1], halo)
		}
	}
}

// paintHeads draws each live rider at its interpolated sub-pixel position:
// a soft pulsing halo, a white core, a boost streak and a jump shadow.
func (a *Arena) paintHeads(s *sim.State, alpha float64, t float32) {
	al := float32(clamp01(alpha))
	for i := uint8(0); i < s.Count; i++ {
		p := &s.Players[i]
		if !p.Alive {
			continue
		}
		pal := &Players[i]
		hx := float32(a.prevX[i]) + float32(a.curX[i]-a.prevX[i])*al
		hy := float32(a.prevY[i]) + float32(a.curY[i]-a.prevY[i])*al
		if p.Jumping() {
			for dx := -1; dx <= 1; dx++ {
				mulAt(&a.buf, int(hx+0.5)+dx, int(hy+0.5)+1, 90)
			}
			hy -= 2
		}
		if p.Boosting() {
			ddx, ddy := p.Dir.Delta()
			for k := 1; k <= 7; k++ {
				c := scale(lerp(pal.Core, pal.Mid, float64(k)/7), 256-k*32)
				addAt(&a.buf, int(hx+0.5)-int(ddx)*k, int(hy+0.5)-int(ddy)*k, c)
			}
		}
		pulse := 0.65 + 0.35*sin1(t*2.2+float32(i)*0.17)
		ringR := 2.2 + 0.8*sin1(t*1.4+float32(i)*0.3)
		charge := a.charge[i]
		cx, cy := int(hx+0.5), int(hy+0.5)
		for dy := -4; dy <= 4; dy++ {
			for dx := -4; dx <= 4; dx++ {
				fx, fy := float32(cx+dx)-hx, float32(cy+dy)-hy
				d := float32(math.Sqrt(float64(fx*fx + fy*fy)))
				if d > 4 {
					continue
				}
				fall := 1 - d/4
				k := fall * fall * pulse * 170
				ring := 1 - absf(d-ringR)*1.6
				if ring > 0 {
					k += ring * 70 * pulse
				}
				c := scale(pal.Mid, int(k))
				if charge > 0.05 {
					c = add(c, scale(sparkGold, int(charge*fall*200)))
				}
				addAt(&a.buf, cx+dx, cy+dy, c)
			}
		}
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			addAt(&a.buf, cx+d[0], cy+d[1], scale(pal.Core, 150))
		}
		setAt(&a.buf, cx, cy, white)
	}
}

// embers trail a faint stream of sparks off every live head.
func (a *Arena) embers(s *sim.State, alpha float64, dt float32) {
	if s.Phase != sim.Running {
		return
	}
	al := float32(clamp01(alpha))
	for i := uint8(0); i < s.Count; i++ {
		p := &s.Players[i]
		if !p.Alive {
			continue
		}
		rate := float32(14)
		if p.Boosting() {
			rate = 70
		}
		a.emitAcc[i] += rate * dt
		dx, dy := p.Dir.Delta()
		hx := float32(a.prevX[i]) + float32(a.curX[i]-a.prevX[i])*al
		hy := float32(a.prevY[i]) + float32(a.curY[i]-a.prevY[i])*al
		for a.emitAcc[i] >= 1 {
			a.emitAcc[i]--
			v := 6 + 10*a.rng.f()
			a.parts.spawn(hx, hy, -float32(dx)*v+a.rng.s()*7, -float32(dy)*v+a.rng.s()*7, 0.25+0.3*a.rng.f(), 4, Players[i].Mid)
		}
	}
}

func addAt(b *[cells]RGB, x, y int, c RGB) {
	if uint(x) < aw && uint(y) < ah {
		b[y*aw+x] = add(b[y*aw+x], c)
	}
}

func setAt(b *[cells]RGB, x, y int, c RGB) {
	if uint(x) < aw && uint(y) < ah {
		b[y*aw+x] = c
	}
}

func mulAt(b *[cells]RGB, x, y, k int) {
	if uint(x) < aw && uint(y) < ah {
		b[y*aw+x] = scale(b[y*aw+x], k)
	}
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
