package render

import (
	"testing"

	"estela/internal/sim"
)

// busyState fills the arena with long serpentine trails for six players,
// three collapse rings and a few sparks: a late, crowded round.
func busyState() *sim.State {
	s := sim.NewMatch(7, 6)
	s.Phase, s.Ring = sim.Running, 3
	for y := 0; y < sim.H; y++ {
		for x := 0; x < sim.W; x++ {
			c := sim.Empty
			ring := min(x, y, sim.W-1-x, sim.H-1-y)
			switch {
			case ring < 3:
				c = sim.Wall
			case y%3 != 0 || x%7 == 0:
				c = sim.TrailOf(uint8((x/16 + y/19*2) % sim.MaxPlayers))
			}
			s.Grid[y*sim.W+x] = c
		}
	}
	for _, i := range []int{20*sim.W + 30, 40*sim.W + 60, 9*sim.W + 80} {
		s.Grid[i] = sim.Spark
	}
	for i := range s.Players {
		p := &s.Players[i]
		p.X, p.Y, p.Alive, p.Boost = int16(10+i*14), int16(6+i*8), true, 60
	}
	s.Players[1].BoostTicks = 10
	s.Players[2].JumpTicks = 2
	return s
}

func BenchmarkArenaDraw(b *testing.B) {
	scr := simScreen(b, 100, 30)
	f := NewFrame(100, 60)
	s := busyState()
	a := NewArena()
	crash := []sim.Event{
		{Kind: sim.EvCrash, Player: 4, X: 50, Y: 30},
		{Kind: sim.EvPickup, Player: 0, X: 20, Y: 20},
		{Kind: sim.EvBoost, Player: 1, X: 24, Y: 14},
	}
	s.Players[4].Alive = false
	a.Events(s, crash)
	t := 0.0
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t += 1.0 / 60
		if i%30 == 0 {
			s.Tick++
			s.Players[0].X = int16(10 + (i/30)%60)
			a.Events(s, crash)
		}
		a.Draw(f, 0, 2, s, float64(i%3)/3, t)
		f.Blit(scr, 0, 0)
	}
	b.StopTimer()
	b.ReportMetric(float64(a.parts.n), "particles")
}

func BenchmarkMenu(b *testing.B) {
	f := NewFrame(110, 64)
	s := busyState()
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
