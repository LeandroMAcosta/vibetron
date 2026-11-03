package render

import (
	"math"

	"estela/internal/sim"
)

const (
	aw, ah = sim.W, sim.H
	cells  = aw * ah

	dissolveSpeed = 95 // px/s, so the farthest trail cell goes in about a second
	dissolveEdge  = 4  // px of white-hot band at the front
)

// dissolve tracks one dead player's trail being eaten outward from the crash.
type dissolve struct {
	active      bool
	x, y        float32
	start       float64
	prevR, maxR float32
}

// Arena is the renderer's memory of the match: what the sim does not keep
// but the eye needs (previous head positions, cell birth times, effects).
type Arena struct {
	rng   rng
	now   float64
	drawn bool
	round uint8
	tick  uint32

	prevX, prevY [sim.MaxPlayers]int16
	curX, curY   [sim.MaxPlayers]int16
	emitAcc      [sim.MaxPlayers]float32
	charge       [sim.MaxPlayers]float32
	melt         [sim.MaxPlayers]dissolve

	grid   [cells]sim.Cell
	birth  [cells]float32
	jitter [cells]uint8
	emit   [cells]RGB
	glow   glowBuf
	buf    [cells]RGB
	bg     [cells]RGB
	gridW  [cells]uint8
	vig    [cells]uint16

	parts pool

	shake, flash, collapse float64
	flashColor             RGB
}

func NewArena() *Arena {
	a := &Arena{rng: 0x5EED1E57A}
	a.bake()
	return a
}

// Reset starts a new round: effects that belong to the old trails go away.
func (a *Arena) Reset() {
	a.parts.clear()
	a.melt = [sim.MaxPlayers]dissolve{}
	a.charge = [sim.MaxPlayers]float32{}
	a.grid = [cells]sim.Cell{}
	a.shake, a.collapse = 0, 0
}

// bake precomputes everything that never changes: background, grid weights,
// vignette with scanlines, and per-cell noise for the dissolve edge.
func (a *Arena) bake() {
	var h rng = 0xC0FFEE
	for y := 0; y < ah; y++ {
		for x := 0; x < aw; x++ {
			i := y*aw + x
			a.bg[i] = lerp(bgTop, bgBottom, float64(y)/ah)
			switch {
			case x%10 == 0 && y%10 == 0:
				a.gridW[i] = 3
			case x%10 == 0 || y%10 == 0:
				a.gridW[i] = 2
			case x%5 == 0 && y%5 == 0:
				a.gridW[i] = 1
			}
			dx := (float64(x) - (aw-1)/2.0) / (aw / 2.0)
			dy := (float64(y) - (ah-1)/2.0) / (ah / 2.0)
			r2 := dx*dx + dy*dy
			v := 1 - 0.30*r2*r2
			if y&1 == 1 {
				v *= 0.86
			}
			a.vig[i] = uint16(clamp01(v) * 256)
			a.jitter[i] = uint8(h.next())
		}
	}
}

// sync captures head positions when the sim tick advances and resets on a
// new round, which the sim signals only by bumping Round.
func (a *Arena) sync(s *sim.State) {
	if s.Round != a.round {
		a.round = s.Round
		a.Reset()
		for i := range s.Players {
			p := &s.Players[i]
			a.prevX[i], a.prevY[i], a.curX[i], a.curY[i] = p.X, p.Y, p.X, p.Y
		}
		a.tick = s.Tick
	}
	if s.Tick == a.tick {
		return
	}
	a.tick = s.Tick
	for i := range s.Players {
		p := &s.Players[i]
		a.prevX[i], a.prevY[i] = a.curX[i], a.curY[i]
		a.curX[i], a.curY[i] = p.X, p.Y
		if abs(int(p.X-a.prevX[i])) > 3 || abs(int(p.Y-a.prevY[i])) > 3 {
			a.prevX[i], a.prevY[i] = p.X, p.Y
		}
	}
}

// Events turns one tick's sim events into effects.
func (a *Arena) Events(s *sim.State, evs []sim.Event) {
	a.sync(s)
	for _, e := range evs {
		x, y := clampf(float32(e.X), 0, aw-1), clampf(float32(e.Y), 0, ah-1)
		switch e.Kind {
		case sim.EvCrash:
			a.crash(e.Player, x, y)
		case sim.EvPickup:
			a.burst(x, y, 28, 30, 0.7, 2.5, sparkGold)
			a.ring(x, y, 16, 22, 0.45, white)
			if e.Player < sim.MaxPlayers {
				a.charge[e.Player] = 1
			}
		case sim.EvBoost:
			if e.Player < sim.MaxPlayers {
				a.puff(s, e.Player, x, y)
			}
		case sim.EvJump:
			if e.Player < sim.MaxPlayers {
				a.ring(x, y, 20, 14, 0.5, scale(Players[e.Player].Mid, 200))
			}
		case sim.EvCollapse:
			a.collapseRing(int(e.Player) - 1)
		case sim.EvRoundWon:
			a.flash, a.flashColor = 0.75, white
			if e.Player < sim.MaxPlayers {
				a.flashColor = lerp(Players[e.Player].Mid, white, 0.5)
			}
		case sim.EvRoundStart:
			a.flash, a.flashColor = 0.35, RGB{180, 200, 255}
		case sim.EvMatchWon:
			a.flash = 1
		}
	}
}

func (a *Arena) crash(p uint8, x, y float32) {
	if p >= sim.MaxPlayers {
		return
	}
	pal := Players[p]
	a.burst(x, y, 140, 70, 1.1, 2.2, pal.Mid)
	a.burst(x, y, 50, 45, 0.6, 3, pal.Core)
	a.burst(x, y, 24, 110, 0.35, 1.5, white)
	a.shake = math.Max(a.shake, 3.5)
	a.flash, a.flashColor = math.Max(a.flash, 0.3), lerp(pal.Mid, white, 0.6)
	a.melt[p] = dissolve{active: true, x: x, y: y, start: a.now, maxR: 130}
}

func (a *Arena) puff(s *sim.State, p uint8, x, y float32) {
	dx, dy := s.Players[p].Dir.Delta()
	pal := Players[p]
	for i := 0; i < 22; i++ {
		v := 20 + 40*a.rng.f()
		a.parts.spawn(x, y, -float32(dx)*v+a.rng.s()*14, -float32(dy)*v+a.rng.s()*14, 0.5+0.3*a.rng.f(), 3, pal.Core)
	}
}

func (a *Arena) collapseRing(r int) {
	if r < 0 || r >= ah/2 {
		return
	}
	a.collapse = 1
	a.shake = math.Max(a.shake, 1.5)
	per := 2 * ((aw - 2*r) + (ah - 2*r))
	for i := 0; i < per; i += 3 {
		x, y := perimeter(r, i)
		cx, cy := float32(aw)/2-float32(x), float32(ah)/2-float32(y)
		n := float32(math.Hypot(float64(cx), float64(cy))) + 1
		v := 6 + 10*a.rng.f()
		a.parts.spawn(float32(x), float32(y), cx/n*v, cy/n*v, 0.6+0.5*a.rng.f(), 1.5, wallHot)
	}
}

// perimeter walks ring r clockwise from its top-left corner.
func perimeter(r, i int) (int, int) {
	w, h := aw-2*r-1, ah-2*r-1
	switch {
	case i < w:
		return r + i, r
	case i < w+h:
		return r + w, r + i - w
	case i < 2*w+h:
		return r + w - (i - w - h), r + h
	}
	return r, r + h - (i - 2*w - h)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
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
