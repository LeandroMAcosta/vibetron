package bot

import "github.com/LeandroMAcosta/vibetron/internal/sim"

const floodCap = 600

// grid is the per-tick view of the arena a Brain floods over. seen uses a
// generation stamp so a fill never has to clear the whole arena.
type grid struct {
	s     *sim.State
	seen  []uint32
	gen   uint32
	queue [floodCap]int32
}

func (g *grid) cellIndex(x, y int16) int32 { return int32(y)*int32(g.s.W) + int32(x) }

func (g *grid) inBounds(x, y int16) bool { return x >= 0 && y >= 0 && x < g.s.W && y < g.s.H }

// reset points the grid at this tick's state; seen only grows when the host
// picks a bigger arena, so Think stays allocation free within a match.
func (g *grid) reset(s *sim.State) {
	g.s = s
	if n := len(s.Grid); len(g.seen) < n {
		g.seen = make([]uint32, n)
		g.gen = 0
	}
}

// free reports whether a non-jumping head may enter the cell.
func (g *grid) free(x, y int16) bool {
	c := g.s.At(x, y)
	return c == sim.Empty || c == sim.Spark
}

func (g *grid) begin() {
	g.gen++
	if g.gen == 0 {
		clear(g.seen)
		g.gen = 1
	}
}

func (g *grid) mark(x, y int16) {
	if g.inBounds(x, y) {
		g.seen[g.cellIndex(x, y)] = g.gen
	}
}

// fill counts cells reachable from (x, y), the start included whatever it
// holds, stopping at floodCap. Call begin and any marks first.
func (g *grid) fill(x, y int16) int {
	if !g.inBounds(x, y) {
		return 0
	}
	w := int32(g.s.W)
	start := g.cellIndex(x, y)
	g.seen[start] = g.gen
	g.queue[0] = start
	head, tail := 0, 1
	for head < tail && tail < floodCap {
		i := g.queue[head]
		head++
		cx, cy := int16(i%w), int16(i/w)
		for d := sim.Dir(0); d < 4; d++ {
			dx, dy := d.Delta()
			nx, ny := cx+dx, cy+dy
			if !g.inBounds(nx, ny) {
				continue
			}
			n := g.cellIndex(nx, ny)
			if g.seen[n] == g.gen || !g.free(nx, ny) {
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
