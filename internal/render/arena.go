package render

import (
	"math"

	"estela/internal/sim"
)

// deathTicks is how long a dead rider's leftover trail takes to sink to dim.
const deathTicks = 6

// Arena is the renderer's memory of the match: what the sim does not keep
// but the eye needs (when each rider died, particles, shake).
type Arena struct {
	rng   rng
	now   float64
	drawn bool
	round uint8

	w, h  int
	buf   []RGB
	bg    []RGB
	death [sim.MaxPlayers]uint32 // sim tick of the crash; 0 means long dead

	parts pool
	shake float64
}

func NewArena() *Arena {
	return &Arena{rng: 0x5EED1E57A}
}

// Reset starts a new round: effects that belong to the old trails go away.
func (a *Arena) Reset() {
	a.parts.clear()
	a.death = [sim.MaxPlayers]uint32{}
	a.shake = 0
}

// sync sizes the buffers to this match's arena and resets on a new round,
// which the sim signals only by bumping Round.
func (a *Arena) sync(s *sim.State) {
	if w, h := int(s.W), int(s.H); w != a.w || h != a.h {
		a.w, a.h = w, h
		a.buf = make([]RGB, w*h)
		a.bg = make([]RGB, w*h)
		a.bake()
		a.round = s.Round
		a.Reset()
	}
	if s.Round != a.round {
		a.round = s.Round
		a.Reset()
	}
}

// bake lays the dot grid: one dim pixel every 4x4, on the lower half of a
// cell. A dot every 2 pixels read as texture, not as a quiet floor.
func (a *Arena) bake() {
	dot := lerp(bgColor, dimColor, 0.7)
	for y := 0; y < a.h; y++ {
		for x := 0; x < a.w; x++ {
			c := bgColor
			if x%4 == 0 && y%4 == 1 {
				c = dot
			}
			a.bg[y*a.w+x] = c
		}
	}
}

// Events turns one tick's sim events into effects.
func (a *Arena) Events(s *sim.State, evs []sim.Event) {
	a.sync(s)
	for _, e := range evs {
		x, y := clampf(float32(e.X), 0, float32(a.w-1)), clampf(float32(e.Y), 0, float32(a.h-1))
		switch e.Kind {
		case sim.EvCrash:
			a.crash(s, e.Player, x, y)
		case sim.EvPickup:
			a.burst(x, y, 28, 30, 0.7, 2.5, sparkGold)
			a.ring(x, y, 16, 22, 0.45, white)
		case sim.EvBoost:
			if e.Player < sim.MaxPlayers {
				a.puff(s, e.Player, x, y)
			}
		case sim.EvJump:
			if e.Player < sim.MaxPlayers {
				a.ring(x, y, 20, 14, 0.5, Players[e.Player].Core)
			}
		}
	}
}

func (a *Arena) crash(s *sim.State, p uint8, x, y float32) {
	if p >= sim.MaxPlayers {
		return
	}
	a.death[p] = max(s.Tick, 1)
	pal := Players[p]
	a.burst(x, y, 140, 70, 1.1, 2.2, pal.Mid)
	a.burst(x, y, 50, 45, 0.6, 3, pal.Core)
	a.burst(x, y, 24, 110, 0.35, 1.5, white)
	a.shake = math.Max(a.shake, 3.5)
}

func (a *Arena) puff(s *sim.State, p uint8, x, y float32) {
	dx, dy := s.Players[p].Dir.Delta()
	pal := Players[p]
	for i := 0; i < 22; i++ {
		v := 20 + 40*a.rng.f()
		a.parts.spawn(x, y, -float32(dx)*v+a.rng.s()*14, -float32(dy)*v+a.rng.s()*14, 0.5+0.3*a.rng.f(), 3, pal.Core)
	}
}

func clampf(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
