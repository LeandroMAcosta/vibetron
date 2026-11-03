package render

import (
	"math"

	"vibetron/internal/sim"
)

// Draw renders the s.W x s.H pixel arena at (ox, oy). alpha in [0,1] is how
// far real time is between the last sim tick and the next; t is seconds and
// drives every effect, so animation stays smooth no matter the sim rate.
func (a *Arena) Draw(f *Frame, ox, oy int, s *sim.State, alpha float64, t float64) {
	a.sync(s)
	dt := t - a.now
	if !a.drawn || dt < 0 || dt > 0.1 {
		dt = 1.0 / 60
	}
	a.now, a.drawn = t, true
	a.shake *= math.Exp(-9 * dt)
	a.parts.update(float32(dt))

	a.paintCells(s, float32(clamp01(alpha)), float32(t))
	for i := uint8(0); i < s.Count; i++ {
		if p := &s.Players[i]; p.Alive {
			a.set(int(p.X), int(p.Y), Players[i].Core)
		}
	}
	a.parts.draw(a.buf, a.w, a.h)
	a.present(f, ox, oy)
}

func (a *Arena) paintCells(s *sim.State, alpha, t float32) {
	var sink [sim.MaxPlayers]uint8
	for i := uint8(0); i < s.Count; i++ {
		if s.Players[i].Alive {
			continue
		}
		k := float32(1)
		if d := a.death[i]; d != 0 {
			k = min(1, (float32(s.Tick-d)+alpha)/deathTicks)
		}
		sink[i] = uint8(k * 0.8 * 255)
	}
	spark := lerp(lilac, white, float64(0.5+0.5*math.Sin(float64(t)*2*math.Pi*1.2)))
	for y := 0; y < a.h; y++ {
		for x := 0; x < a.w; x++ {
			i := y*a.w + x
			c := s.Grid[i]
			if c == sim.Spark {
				a.buf[i] = spark
				continue
			}
			o, ok := c.Owner()
			if !ok {
				a.buf[i] = a.bg[i]
				continue
			}
			col := Players[o].Mid
			if sink[o] > 0 {
				col = mix(col, dimColor, sink[o])
			}
			a.buf[i] = fadeTail(s, col, int16(x), int16(y))
		}
	}
}

// fadeTail sinks the oldest 40% of a trail toward the background, so the
// tail is nearly gone by the time the sim erases it.
func fadeTail(s *sim.State, c RGB, x, y int16) RGB {
	age, max := s.TrailAge(x, y)
	if max <= 0 {
		return c
	}
	r := float32(age) / float32(max)
	if r <= 0.6 {
		return c
	}
	return mix(c, bgColor, uint8(min(1, (r-0.6)/0.4)*0.85*255))
}

func (a *Arena) set(x, y int, c RGB) {
	if uint(x) < uint(a.w) && uint(y) < uint(a.h) {
		a.buf[y*a.w+x] = c
	}
}

// present copies the arena buffer to the frame, shifted by screen shake.
func (a *Arena) present(f *Frame, ox, oy int) {
	sx, sy := 0, 0
	if a.shake > 0.2 {
		sx = int(math.Round(float64(a.rng.s()) * a.shake))
		sy = int(math.Round(float64(a.rng.s()) * a.shake * 0.6))
	}
	for y := 0; y < a.h; y++ {
		fy := oy + y
		if fy < 0 || fy >= f.H {
			continue
		}
		srcY := y - sy
		for x := 0; x < a.w; x++ {
			fx := ox + x
			if fx < 0 || fx >= f.W {
				continue
			}
			c := bgColor
			if srcX := x - sx; srcX >= 0 && srcX < a.w && srcY >= 0 && srcY < a.h {
				c = a.buf[srcY*a.w+srcX]
			}
			f.Pix[fy*f.W+fx] = c
		}
	}
}
