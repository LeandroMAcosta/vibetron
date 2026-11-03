package bot

import "estela/internal/sim"

const floodCap = 600

const doomWindow = 20

// grid is the per-tick view of the arena a Brain floods over. seen uses a
// generation stamp so a fill never has to clear the whole arena.
type grid struct {
	s        *sim.State
	seen     [sim.W * sim.H]uint32
	gen      uint32
	queue    [floodCap]int32
	doomRing int16 // ring about to become wall, or -1
	doomIn   int   // moves left before that ring closes
}

func cellIndex(x, y int16) int32 { return int32(y)*sim.W + int32(x) }

func inBounds(x, y int16) bool { return x >= 0 && y >= 0 && x < sim.W && y < sim.H }

func depth(x, y int16) int16 { return min(x, y, sim.W-1-x, sim.H-1-y) }

func (g *grid) reset(s *sim.State) {
	g.s = s
	g.doomRing, g.doomIn = -1, 0
	if s.Ring >= sim.MaxRing {
		return
	}
	next := sim.CollapseAfter
	if t := int(s.RunTicks) + 1; t > sim.CollapseAfter {
		next += (t - sim.CollapseAfter + sim.CollapseEvery - 1) / sim.CollapseEvery * sim.CollapseEvery
	}
	if left := next - int(s.RunTicks); left <= doomWindow {
		g.doomRing, g.doomIn = int16(s.Ring), left
	}
}

func (g *grid) doomed(x, y int16) bool { return g.doomRing >= 0 && depth(x, y) <= g.doomRing }

// free reports whether a non-jumping head may enter the cell.
func (g *grid) free(x, y int16) bool {
	c := g.s.At(x, y)
	return c == sim.Empty || c == sim.Spark
}

// open is free and not inside a ring that is about to close; the flood
// treats doomed cells as walls so bots drift inward before a collapse.
func (g *grid) open(x, y int16) bool { return g.free(x, y) && !g.doomed(x, y) }

func (g *grid) begin() {
	g.gen++
	if g.gen == 0 {
		clear(g.seen[:])
		g.gen = 1
	}
}

func (g *grid) mark(x, y int16) {
	if inBounds(x, y) {
		g.seen[cellIndex(x, y)] = g.gen
	}
}

// fill counts cells reachable from (x, y), the start included whatever it
// holds, stopping at floodCap. Call begin and any marks first.
func (g *grid) fill(x, y int16) int {
	if !inBounds(x, y) {
		return 0
	}
	start := cellIndex(x, y)
	g.seen[start] = g.gen
	g.queue[0] = start
	head, tail := 0, 1
	for head < tail && tail < floodCap {
		i := g.queue[head]
		head++
		cx, cy := int16(i%sim.W), int16(i/sim.W)
		for d := sim.Dir(0); d < 4; d++ {
			dx, dy := d.Delta()
			nx, ny := cx+dx, cy+dy
			if !inBounds(nx, ny) {
				continue
			}
			n := cellIndex(nx, ny)
			if g.seen[n] == g.gen || !g.open(nx, ny) {
				continue
			}
			g.seen[n] = g.gen
			g.queue[tail] = n
			tail++
			if tail == floodCap {
				break
			}
		}
	}
	return tail
}
