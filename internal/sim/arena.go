package sim

import "hash/fnv"

// paint lays player i's trail on its current cell and remembers it in the
// trail queue so the tail can be erased later.
func (s *State) paint(i uint8) {
	p := &s.Players[i]
	k := s.idx(p.X, p.Y)
	s.Grid[k] = TrailOf(i)
	s.Stamp[k] = p.Steps
	p.Steps++
	if p.tail > 1024 && p.tail*2 > len(p.trail) {
		p.trail = append(p.trail[:0], p.trail[p.tail:]...)
		p.tail = 0
	}
	p.trail = append(p.trail, int32(k))
}

// retract erases up to n cells from the tail of player i's trail.
func (s *State) retract(i uint8, n int) {
	p := &s.Players[i]
	for ; n > 0 && p.tail < len(p.trail); n-- {
		k := p.trail[p.tail]
		p.tail++
		if s.Grid[k] == TrailOf(i) {
			s.Grid[k] = Empty
		}
	}
}

// spawnSpark drops a pickup on a random empty cell every few seconds.
func (s *State) spawnSpark() {
	if s.RunTicks%s.secs(3) != 0 || s.Sparks >= maxSparks {
		return
	}
	for try := 0; try < 16; try++ {
		n := s.next()
		x, y := int16(n%uint64(s.W)), int16((n>>16)%uint64(s.H))
		if s.Grid[s.idx(x, y)] == Empty && !s.nearHead(x, y) {
			s.Grid[s.idx(x, y)] = Spark
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
	var b [32]byte
	put := func(n int, v uint64) {
		for i := 0; i < n; i++ {
			b[i] = byte(v >> (8 * i))
		}
		h.Write(b[:n])
	}
	put(4, uint64(s.Tick))
	put(4, uint64(s.RunTicks))
	put(4, uint64(s.Phase)|uint64(s.PhaseTicks)<<8|uint64(s.Sparks)<<24)
	put(8, s.rng)
	for i := uint8(0); i < s.Count; i++ {
		p := &s.Players[i]
		put(8, uint64(uint16(p.X))|uint64(uint16(p.Y))<<16|uint64(p.Dir)<<32|uint64(p.queued)<<40|uint64(bool8(p.Alive))<<48|uint64(bool8(p.Left))<<56)
		put(8, uint64(p.Boost)|uint64(p.BoostTicks)<<8|uint64(p.JumpTicks)<<24|uint64(p.JumpCooldown)<<32|uint64(p.Wins)<<48)
		put(8, uint64(p.Steps)|uint64(p.Bonus)<<32|uint64(p.TrailCells())<<48)
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
