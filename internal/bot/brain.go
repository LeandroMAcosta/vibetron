// Package bot drives empty seats. A Brain reads the host's State once per
// tick and picks at most one action: survive first, then play to its
// personality.
package bot

import "github.com/LeandroMAcosta/vibetron/internal/sim"

// Personality knobs, all drawn from the seed in New.
type traits struct {
	aggression  float32 // pull toward the point just ahead of a nearby rival
	greed       float32 // pull toward sparks, and how far it looks for them
	boostRate   float32 // chance per open-road tick of a cruise boost
	comfort     int     // reachable cells above which space stops mattering
	turnPenalty float32
	jitter      float32
}

type Brain struct {
	id     uint8
	rng    uint64
	traits traits
	g      grid
	threat [sim.MaxPlayers * 4]threat
	nThr   int
}

type threat struct {
	x, y   int16
	weight float32
}

func New(id uint8, seed uint64) *Brain {
	b := &Brain{id: id, rng: seed ^ uint64(id+1)*0xD1B54A32D192ED03}
	b.traits = traits{
		aggression:  b.unit(),
		greed:       b.unit(),
		boostRate:   0.01 + 0.07*b.unit(),
		comfort:     250 + int(250*b.unit()),
		turnPenalty: 3 + 5*b.unit(),
		jitter:      2 + 5*b.unit(),
	}
	return b
}

// next is splitmix64, kept per Brain so seeded tests replay exactly.
func (b *Brain) next() uint64 {
	b.rng += 0x9E3779B97F4A7C15
	z := b.rng
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func (b *Brain) unit() float32 { return float32(b.next()>>40) / (1 << 24) }

// Think returns the action for this tick, or false to keep going straight.
func (b *Brain) Think(s *sim.State) (sim.Action, bool) {
	if s.Phase != sim.Running || b.id >= s.Count {
		return 0, false
	}
	p := &s.Players[b.id]
	if !p.Alive {
		return 0, false
	}
	b.g.reset(s)
	if p.Jumping() {
		return b.midJump(p)
	}
	b.scanThreats(s)

	steps := 1
	if p.Boosting() {
		steps = 2
	}
	a := b.findAims(s, p)
	var cands [3]candidate
	dirs := [3]sim.Dir{p.Dir, p.Dir.TurnLeft(), p.Dir.TurnRight()}
	roomiest := -1
	for i, d := range dirs {
		cands[i] = b.evaluate(p, d, steps)
		roomiest = max(roomiest, cands[i].space)
	}
	if roomiest <= 2 && b.jumpSpace(p) > roomiest {
		return sim.Jump, true
	}
	tight := roomiest < b.traits.comfort
	best := 0
	for i := range cands {
		b.score(&cands[i], i > 0, tight, &a)
		if cands[i].score > cands[best].score {
			best = i
		}
	}
	if best > 0 {
		return sim.FaceUp + sim.Action(dirs[best]), true
	}
	if !tight && b.wantsBoost(s, p, &a) {
		return sim.Boost, true
	}
	return 0, false
}

// midJump keeps the planned line; it only steers to dodge a wall, the one
// thing a jump cannot pass.
func (b *Brain) midJump(p *sim.Player) (sim.Action, bool) {
	dx, dy := p.Dir.Delta()
	if b.g.s.At(p.X+dx, p.Y+dy) != sim.Wall {
		return 0, false
	}
	for _, d := range [2]sim.Dir{p.Dir.TurnLeft(), p.Dir.TurnRight()} {
		dx, dy = d.Delta()
		if b.g.s.At(p.X+dx, p.Y+dy) != sim.Wall {
			return sim.FaceUp + sim.Action(d), true
		}
	}
	return 0, false
}

// jumpSpace is the room left after a jump straight ahead from now, or 0
// when the jump would end in a wall.
func (b *Brain) jumpSpace(p *sim.Player) int {
	if p.JumpCooldown > 0 {
		return 0
	}
	cells := 0
	for k := 0; k < sim.JumpDuration; k++ {
		cells++
		if int(p.BoostTicks) > k {
			cells++
		}
	}
	dx, dy := p.Dir.Delta()
	x, y := p.X, p.Y
	for k := 0; k < cells; k++ {
		x, y = x+dx, y+dy
		if b.g.s.At(x, y) == sim.Wall {
			return 0
		}
	}
	b.g.begin()
	b.g.mark(p.X, p.Y)
	return b.g.fill(x, y) - 1
}

// scanThreats lists the cells rival heads can enter this tick, since two
// heads landing on one cell both crash. Straight ahead is the likeliest.
func (b *Brain) scanThreats(s *sim.State) {
	b.nThr = 0
	for j := uint8(0); j < s.Count; j++ {
		o := &s.Players[j]
		if j == b.id || !o.Alive {
			continue
		}
		dx, dy := o.Dir.Delta()
		b.addThreat(o.X+dx, o.Y+dy, 3)
		if o.Boosting() {
			b.addThreat(o.X+2*dx, o.Y+2*dy, 2)
		}
		for _, d := range [2]sim.Dir{o.Dir.TurnLeft(), o.Dir.TurnRight()} {
			dx, dy = d.Delta()
			b.addThreat(o.X+dx, o.Y+dy, 1)
		}
	}
}

func (b *Brain) addThreat(x, y int16, w float32) {
	if b.g.free(x, y) {
		b.threat[b.nThr] = threat{x, y, w}
		b.nThr++
	}
}

func (b *Brain) threatAt(x, y int16) float32 {
	var w float32
	for i := 0; i < b.nThr; i++ {
		if t := &b.threat[i]; t.x == x && t.y == y {
			w += t.weight
		}
	}
	return w
}
