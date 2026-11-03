package sim

import "hash/fnv"

// collapse closes the arena one ring at a time once a round drags on.
func (s *State) collapse(out []Event) []Event {
	if s.RunTicks < CollapseAfter || (s.RunTicks-CollapseAfter)%CollapseEvery != 0 || s.Ring >= MaxRing {
		return out
	}
	r := int16(s.Ring)
	s.Ring++
	for x := r; x < W-r; x++ {
		s.wall(x, r)
		s.wall(x, H-1-r)
	}
	for y := r; y < H-r; y++ {
		s.wall(r, y)
		s.wall(W-1-r, y)
	}
	out = append(out, Event{Kind: EvCollapse, Player: s.Ring})
	for i := uint8(0); i < s.Count; i++ {
		p := &s.Players[i]
		if p.Alive && s.Grid[idx(p.X, p.Y)] == Wall {
			p.Alive = false
			out = append(out, Event{Kind: EvCrash, Player: i, X: p.X, Y: p.Y})
		}
	}
	return out
}

func (s *State) wall(x, y int16) {
	c := &s.Grid[idx(x, y)]
	if *c == Spark {
		s.Sparks--
	}
	*c = Wall
}

// spawnSpark drops a pickup on a random empty cell every few seconds.
func (s *State) spawnSpark() {
	if s.RunTicks%sparkEvery != 0 || s.Sparks >= maxSparks {
		return
	}
	for try := 0; try < 16; try++ {
		n := s.next()
		x, y := int16(n%W), int16((n>>16)%H)
		if s.Grid[idx(x, y)] == Empty && !s.nearHead(x, y) {
			s.Grid[idx(x, y)] = Spark
			s.Sparks++
			return
		}
	}
}

func (s *State) nearHead(x, y int16) bool {
	for i := uint8(0); i < s.Count; i++ {
		p := &s.Players[i]
		if p.Alive && abs16(p.X-x) < 4 && abs16(p.Y-y) < 4 {
			return true
		}
	}
	return false
}

func abs16(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
}

// Hash fingerprints everything that affects future ticks, so peers can
// detect a desync by comparing it.
func (s *State) Hash() uint64 {
	h := fnv.New64a()
	var b [24]byte
	b[0], b[1], b[2], b[3] = byte(s.Tick), byte(s.Tick>>8), byte(s.Tick>>16), byte(s.Tick>>24)
	b[4], b[5], b[6], b[7] = byte(s.Phase), byte(s.PhaseTicks), byte(s.Ring), byte(s.Sparks)
	for i := 0; i < 8; i++ {
		b[8+i] = byte(s.rng >> (8 * i))
	}
	h.Write(b[:16])
	for i := uint8(0); i < s.Count; i++ {
		p := &s.Players[i]
		b = [24]byte{byte(p.X), byte(p.X >> 8), byte(p.Y), byte(p.Y >> 8), byte(p.Dir), p.queued,
			byte(p.queue[0]), byte(p.queue[1]), bool8(p.Alive), bool8(p.Left), p.Boost, p.BoostTicks,
			p.JumpTicks, p.JumpCooldown, p.Wins}
		h.Write(b[:15])
	}
	grid := make([]byte, len(s.Grid))
	for i, c := range s.Grid {
		grid[i] = byte(c)
	}
	h.Write(grid)
	return h.Sum64()
}

func bool8(v bool) byte {
	if v {
		return 1
	}
	return 0
}
