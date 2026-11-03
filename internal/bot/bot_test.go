package bot

import (
	"testing"

	"estela/internal/sim"
)

// arena returns a running two-player state with an empty grid and the
// given players placed by hand.
func arena(t testing.TB, players ...sim.Player) *sim.State {
	t.Helper()
	s := sim.NewMatch(1, uint8(len(players)), testCfg)
	for s.Phase != sim.Running {
		s.Step(nil, nil)
	}
	clear(s.Grid)
	for i, p := range players {
		p.Alive, p.Boost = true, sim.BoostMax
		s.Players[i] = p
		s.Grid[at(s, p.X, p.Y)] = sim.TrailOf(uint8(i))
	}
	return s
}

func TestTurnsAwayFromWall(t *testing.T) {
	for seed := uint64(0); seed < 20; seed++ {
		s := arena(t, sim.Player{X: testCfg.W() - 1, Y: 20, Dir: sim.Right}, sim.Player{X: 10, Y: 40, Dir: sim.Left})
		a, ok := New(0, seed).Think(s)
		if !ok || (a != sim.FaceUp && a != sim.FaceDown) {
			t.Fatalf("seed %d: got %v %v, want a turn off the wall", seed, a, ok)
		}
	}
}

func TestPrefersOpenSideOverPocket(t *testing.T) {
	s := arena(t, sim.Player{X: 50, Y: 20, Dir: sim.Right}, sim.Player{X: 10, Y: 50, Dir: sim.Left})
	// Up leads into a six-cell pocket, down into the open arena.
	block := func(x, y int16) { s.Grid[at(s, x, y)] = sim.TrailOf(1) }
	for x := int16(47); x <= 51; x++ {
		block(x, 17)
	}
	for y := int16(17); y <= 23; y++ {
		block(51, y)
	}
	for y := int16(17); y <= 20; y++ {
		block(47, y)
	}
	block(48, 20)
	block(49, 20)
	for seed := uint64(0); seed < 20; seed++ {
		a, ok := New(0, seed).Think(s)
		if !ok || a != sim.FaceDown {
			t.Fatalf("seed %d: got %v %v, want FaceDown out of the pocket", seed, a, ok)
		}
	}
}

func TestJumpsOverTrapWhenBoxedIn(t *testing.T) {
	s := arena(t, sim.Player{X: 50, Y: 20, Dir: sim.Right}, sim.Player{X: 10, Y: 50, Dir: sim.Left})
	for y := int16(19); y <= 21; y++ {
		s.Grid[at(s, 51, y)] = sim.TrailOf(1)
	}
	s.Grid[at(s, 50, 19)] = sim.TrailOf(1)
	s.Grid[at(s, 50, 21)] = sim.TrailOf(1)
	a, ok := New(0, 7).Think(s)
	if !ok || a != sim.Jump {
		t.Fatalf("got %v %v, want Jump", a, ok)
	}
	s.Step([]sim.Input{{Player: 0, Action: a}}, nil)
	for s.Players[0].Jumping() {
		s.Step(nil, nil)
	}
	if !s.Players[0].Alive {
		t.Fatal("bot died during its own jump")
	}
}

func TestAvoidsHeadOn(t *testing.T) {
	// The rival's next cell is the one straight ahead of the bot.
	s := arena(t, sim.Player{X: 50, Y: 20, Dir: sim.Right}, sim.Player{X: 52, Y: 20, Dir: sim.Left})
	for seed := uint64(0); seed < 20; seed++ {
		a, ok := New(0, seed).Think(s)
		if !ok || (a != sim.FaceUp && a != sim.FaceDown) {
			t.Fatalf("seed %d: got %v %v, want a turn away from the head-on", seed, a, ok)
		}
	}
}

func TestIdleOutsideRunning(t *testing.T) {
	s := sim.NewMatch(1, 2, testCfg)
	if _, ok := New(0, 1).Think(s); ok {
		t.Fatal("acted during countdown")
	}
}

func TestPersonalitiesDiffer(t *testing.T) {
	a, b := New(0, 1).traits, New(1, 1).traits
	if a == b {
		t.Fatal("two seats share a personality")
	}
	if New(2, 9).traits != New(2, 9).traits {
		t.Fatal("same seed gave different personalities")
	}
}

// midRound returns a six-bot state a few hundred ticks into a round, with
// trails on the board so the flood fill does real work.
func midRound() (*sim.State, *Brain) {
	s := sim.NewMatch(3, seats, testCfg)
	var minds [seats]*Brain
	for i := range minds {
		minds[i] = New(uint8(i), uint64(i))
	}
	var inputs []sim.Input
	for s.Phase != sim.MatchOver && (s.Phase != sim.Running || s.RunTicks < 300) {
		inputs = inputs[:0]
		for i, m := range minds {
			if a, ok := m.Think(s); ok {
				inputs = append(inputs, sim.Input{Player: uint8(i), Action: a})
			}
		}
		s.Step(inputs, nil)
	}
	for i := range minds {
		if s.Players[i].Alive {
			return s, minds[i]
		}
	}
	return s, minds[0]
}

func TestThinkDoesNotAllocate(t *testing.T) {
	s, b := midRound()
	if n := testing.AllocsPerRun(1000, func() { b.Think(s) }); n != 0 {
		t.Fatalf("Think allocated %v times per call", n)
	}
}

func BenchmarkThink(bm *testing.B) {
	s, b := midRound()
	bm.ReportAllocs()
	for bm.Loop() {
		b.Think(s)
	}
}

// testCfg is the 100x56 arena the hand-placed scenarios were drawn for.
var testCfg = sim.Config{Size: 2, Speed: 2}

func at(s *sim.State, x, y int16) int { return int(y)*int(s.W) + int(x) }
