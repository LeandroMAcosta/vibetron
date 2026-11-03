package bot

import "estela/internal/sim"

const unsafe = -1

// candidate is one of straight, left or right for this tick.
type candidate struct {
	space      int // reachable cells after the move, or unsafe
	endX, endY int16
	threat     float32
	score      float32
}

// aim holds the points a personality steers toward once survival is easy.
type aim struct {
	headX, headY   int16
	spark, cut     bool
	sparkX, sparkY int16
	cutX, cutY     int16
	rivalDist      int16 // to the nearest rival head
}

func (b *Brain) evaluate(p *sim.Player, d sim.Dir, steps int) candidate {
	c := candidate{space: unsafe}
	dx, dy := d.Delta()
	x, y := p.X, p.Y
	for k := 0; k < steps; k++ {
		x, y = x+dx, y+dy
		if !b.g.free(x, y) {
			return c
		}
		c.threat += b.threatAt(x, y)
	}
	b.g.begin()
	b.g.mark(p.X, p.Y)
	if steps == 2 {
		b.g.mark(x-dx, y-dy)
	}
	c.space = b.g.fill(x, y)
	c.endX, c.endY = x, y
	return c
}

func (b *Brain) score(c *candidate, turned, tight bool, a *aim) {
	if c.space == unsafe {
		c.score = -1e9
		return
	}
	t := &b.traits
	c.score = float32(min(c.space, t.comfort))
	if c.space >= t.comfort {
		if a.spark {
			c.score += 12 * t.greed * float32(manhattan(a.headX, a.headY, a.sparkX, a.sparkY)-manhattan(c.endX, c.endY, a.sparkX, a.sparkY))
		}
		if a.cut {
			c.score += 12 * t.aggression * float32(manhattan(a.headX, a.headY, a.cutX, a.cutY)-manhattan(c.endX, c.endY, a.cutX, a.cutY))
		}
	}
	if turned {
		c.score -= t.turnPenalty
	}
	headOn := float32(15)
	if tight {
		headOn = 40
	}
	c.score -= c.threat * headOn
	c.score += t.jitter * b.unit()
}

// findAims finds the nearest spark within the bot's greed radius and the cell a
// few moves ahead of the nearest rival, for aggressive bots to cut across.
func (b *Brain) findAims(s *sim.State, p *sim.Player) aim {
	a := aim{headX: p.X, headY: p.Y, rivalDist: 1 << 14}
	t := &b.traits
	if t.greed > 0.15 {
		r := 3 + int16(15*t.greed)
		bestD := int16(1 << 14)
		for y := max(p.Y-r, 0); y <= min(p.Y+r, s.H-1); y++ {
			for x := max(p.X-r, 0); x <= min(p.X+r, s.W-1); x++ {
				if s.At(x, y) != sim.Spark {
					continue
				}
				if d := manhattan(p.X, p.Y, x, y); d < bestD {
					bestD, a.spark, a.sparkX, a.sparkY = d, true, x, y
				}
			}
		}
	}
	var rival *sim.Player
	for j := uint8(0); j < s.Count; j++ {
		o := &s.Players[j]
		if j == b.id || !o.Alive {
			continue
		}
		if d := manhattan(p.X, p.Y, o.X, o.Y); d < a.rivalDist {
			a.rivalDist, rival = d, o
		}
	}
	if rival != nil && t.aggression > 0.25 && a.rivalDist <= 6+int16(14*t.aggression) {
		lead := int16(4)
		if rival.Boosting() {
			lead = 8
		}
		dx, dy := rival.Dir.Delta()
		a.cut = true
		a.cutX = min(max(rival.X+lead*dx, 0), s.W-1)
		a.cutY = min(max(rival.Y+lead*dy, 0), s.H-1)
	}
	return a
}

// wantsBoost fires on a long clear road, or to snatch a spark dead ahead.
func (b *Brain) wantsBoost(s *sim.State, p *sim.Player, a *aim) bool {
	if p.Boosting() || p.Boost < sim.BoostCost {
		return false
	}
	dx, dy := p.Dir.Delta()
	run := int16(0)
	for x, y := p.X+dx, p.Y+dy; run < 24 && b.g.free(x, y); x, y = x+dx, y+dy {
		run++
		if a.spark && x == a.sparkX && y == a.sparkY && run <= 12 && b.traits.greed > 0.4 {
			return true
		}
	}
	return run >= 18 && a.rivalDist > 10 && b.unit() < b.traits.boostRate
}

func manhattan(ax, ay, bx, by int16) int16 { return abs16(ax-bx) + abs16(ay-by) }

func abs16(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
}
