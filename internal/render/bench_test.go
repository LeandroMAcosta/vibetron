package render

import (
	"testing"

	"vibetron/internal/sim"
)

// busyState lays a serpentine trail of up to maxLen cells for each of six
// players in its own sixth of the arena, stamped oldest to newest the way
// the sim would, with the trail cap stretched to cover it so the oldest
// 40% shows the tail fade.
func busyState(size uint8, maxLen int) *sim.State {
	s := sim.NewMatch(7, 6, sim.Config{Size: size, Speed: 1})
	s.Phase = sim.Running
	clear(s.Grid)
	w, h := int(s.W), int(s.H)
	rw, rh := w/3, h/2
	for i := range s.Players {
		x0, y0 := (i%3)*rw+1, (i/3)*rh+1
		p := &s.Players[i]
		n := 0
		put := func(x, y int) {
			if n >= maxLen {
				return
			}
			k := y*w + x
			s.Grid[k], s.Stamp[k] = sim.TrailOf(uint8(i)), uint32(n)
			p.X, p.Y = int16(x), int16(y)
			n++
		}
		for r := 0; y0+r < y0+rh-2; r += 2 {
			for c := 0; c < rw-2; c++ {
				x := x0 + c
				if r%4 == 2 {
					x = x0 + rw - 3 - c
				}
				put(x, y0+r)
			}
			if r%4 == 0 {
				put(x0+rw-3, y0+r+1)
			} else {
				put(x0, y0+r+1)
			}
		}
		p.Steps, p.Alive, p.Boost = uint32(n), true, 60
		p.Bonus = uint16(max(0, n-sim.TrailLen))
	}
	for _, f := range [][2]int{{30, 20}, {60, 40}, {80, 9}} {
		x, y := f[0]*w/140, f[1]*h/80
		s.Grid[(y&^1)*w+x] = sim.Spark
	}
	s.Players[1].BoostTicks = 10
	s.Players[2].JumpTicks = 2
	return s
}

func BenchmarkArenaDraw(b *testing.B) {
	s := busyState(3, 1<<20)
	scr := simScreen(b, int(s.W), (int(s.H)+HUDHeight)/2)
	f := NewFrame(int(s.W), int(s.H)+HUDHeight)
	a := NewArena()
	evs := []sim.Event{
		{Kind: sim.EvCrash, Player: 4, X: 50, Y: 30},
		{Kind: sim.EvPickup, Player: 0, X: 20, Y: 20},
		{Kind: sim.EvBoost, Player: 1, X: 24, Y: 14},
	}
	s.Players[4].Alive = false
	a.Events(s, evs)
	t := 0.0
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t += 1.0 / 60
		if i%30 == 0 {
			s.Tick++
			a.Events(s, evs)
		}
		DrawHUD(f, 0, 0, f.W, s, nil)
		a.Draw(f, 0, HUDHeight, s, float64(i%3)/3, t)
		f.Blit(scr, 0, 0)
	}
	b.StopTimer()
	b.ReportMetric(float64(a.parts.n), "particles")
}

func BenchmarkMenu(b *testing.B) {
	f := NewFrame(110, 64)
	s := busyState(1, 200)
	Title(f, 55, 4, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t := float64(i) / 60
		Backdrop(f, t)
		Title(f, f.W/2, 4, t)
		DrawHUD(f, 5, 0, 100, s, nil)
	}
}
