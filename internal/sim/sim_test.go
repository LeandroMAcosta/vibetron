package sim

import "testing"

// running returns a state past the countdown with players placed by hand.
func running(t *testing.T, players ...Player) *State {
	t.Helper()
	s := NewMatch(1, uint8(len(players)))
	for s.Phase != Running {
		s.Step(nil, nil)
	}
	s.Grid = [W * H]Cell{}
	for i, p := range players {
		p.Alive, p.Boost = true, BoostMax
		s.Players[i] = p
		s.Grid[idx(p.X, p.Y)] = TrailOf(uint8(i))
	}
	return s
}

func hasEvent(evs []Event, k EventKind, player uint8) bool {
	for _, e := range evs {
		if e.Kind == k && e.Player == player {
			return true
		}
	}
	return false
}

func TestMovesAndLeavesTrail(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left})
	s.Step(nil, nil)
	if p := s.Players[0]; p.X != 11 || p.Y != 10 {
		t.Fatalf("head at %d,%d, want 11,10", p.X, p.Y)
	}
	if s.At(10, 10) != TrailOf(0) || s.At(11, 10) != TrailOf(0) {
		t.Fatal("trail not painted")
	}
}

func TestWallCrashEndsRound(t *testing.T) {
	s := running(t, Player{X: W - 1, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left})
	evs := s.Step(nil, nil)
	if s.Players[0].Alive || !hasEvent(evs, EvCrash, 0) {
		t.Fatal("expected wall crash")
	}
	if s.Phase != RoundOver || s.Winner != 1 || s.Players[1].Wins != 1 {
		t.Fatalf("phase %v winner %d wins %d", s.Phase, s.Winner, s.Players[1].Wins)
	}
}

func TestTrailCrash(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left}, Player{X: 80, Y: 5, Dir: Down})
	s.Grid[idx(11, 10)] = TrailOf(1)
	s.Step(nil, nil)
	if s.Players[0].Alive {
		t.Fatal("expected crash into trail")
	}
	if s.Phase != Running {
		t.Fatal("two players remain, round should continue")
	}
}

func TestHeadOnSameCellCrashesBoth(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right}, Player{X: 12, Y: 10, Dir: Left})
	evs := s.Step(nil, nil)
	if s.Players[0].Alive || s.Players[1].Alive {
		t.Fatal("both should crash")
	}
	if s.Winner != -1 || !hasEvent(evs, EvRoundWon, NoPlayer) {
		t.Fatal("expected a draw")
	}
}

func TestHeadSwapCrashesBoth(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right}, Player{X: 11, Y: 10, Dir: Left})
	s.Step(nil, nil)
	if s.Players[0].Alive || s.Players[1].Alive {
		t.Fatal("both should crash when swapping cells")
	}
}

func TestJumpPassesOverTrailWithoutPainting(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left})
	s.Grid[idx(12, 10)] = TrailOf(1)
	s.Step([]Input{{0, Jump}}, nil)
	s.Step(nil, nil)
	s.Step(nil, nil)
	p := s.Players[0]
	if !p.Alive || p.X != 13 {
		t.Fatalf("alive=%v x=%d, want alive at 13", p.Alive, p.X)
	}
	if s.At(11, 10) != Empty || s.At(12, 10) != TrailOf(1) {
		t.Fatal("jump must leave no trail and keep the trail it crossed")
	}
	if p.JumpCooldown == 0 {
		t.Fatal("cooldown should be running")
	}
	s.Step(nil, nil)
	if s.At(14, 10) != TrailOf(0) {
		t.Fatal("trail should resume after landing")
	}
}

func TestBoostMovesTwoAndChecksEachCell(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left}, Player{X: 10, Y: 30, Dir: Right})
	s.Step([]Input{{0, Boost}}, nil)
	if s.Players[0].X != 12 || s.At(11, 10) != TrailOf(0) {
		t.Fatalf("boost x=%d, want 12 with both cells painted", s.Players[0].X)
	}
	s.Grid[idx(12, 30)] = TrailOf(1)
	s.Step([]Input{{2, Boost}}, nil)
	if s.Players[2].Alive {
		t.Fatal("boost must not skip over a trail cell")
	}
}

func TestQueuedTurnsAndNoReverse(t *testing.T) {
	s := running(t, Player{X: 20, Y: 20, Dir: Right}, Player{X: 50, Y: 40, Dir: Left})
	s.Step([]Input{{0, FaceLeft}}, nil)
	if s.Players[0].Dir != Right {
		t.Fatal("reverse must be ignored")
	}
	s.Step([]Input{{0, FaceUp}, {0, FaceLeft}}, nil)
	if s.Players[0].Dir != Up {
		t.Fatal("first queued turn should apply")
	}
	s.Step(nil, nil)
	if s.Players[0].Dir != Left {
		t.Fatal("second queued turn should apply on the next tick")
	}
}

func TestSparkPickupRefills(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right, JumpCooldown: 50}, Player{X: 50, Y: 40, Dir: Left})
	s.Grid[idx(11, 10)] = Spark
	s.Sparks = 1
	s.Players[0].Boost, s.Players[0].JumpCooldown = 0, 50
	evs := s.Step(nil, nil)
	p := s.Players[0]
	if !hasEvent(evs, EvPickup, 0) || p.Boost != BoostMax || p.JumpCooldown != 0 || s.Sparks != 0 {
		t.Fatalf("pickup failed: boost=%d cd=%d", p.Boost, p.JumpCooldown)
	}
}

func TestCollapseWallsAndCrushes(t *testing.T) {
	s := running(t, Player{X: 0, Y: 30, Dir: Down}, Player{X: 50, Y: 20, Dir: Up}, Player{X: 50, Y: 40, Dir: Down})
	s.RunTicks = CollapseAfter - 1
	evs := s.Step(nil, nil)
	if s.Ring != 1 || !hasEvent(evs, EvCollapse, 1) || s.At(0, 0) != Wall || s.At(W-1, H-1) != Wall {
		t.Fatal("expected first collapse ring")
	}
	if s.Players[0].Alive {
		t.Fatal("player on the ring must be crushed")
	}
}

func TestQuitRemovesPlayerAndEndsMatch(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left})
	s.Step([]Input{{1, Quit}}, nil)
	for s.Phase == RoundOver {
		s.Step(nil, nil)
	}
	if s.Phase != MatchOver || s.Winner != 0 {
		t.Fatalf("phase %v winner %d", s.Phase, s.Winner)
	}
}

func TestMatchEndsAtWinsToMatch(t *testing.T) {
	s := NewMatch(7, 2)
	for s.Phase != MatchOver && s.Tick < 100000 {
		var in []Input
		if s.Phase == Running && s.RunTicks == 3 {
			in = []Input{{1, TurnLeft}}
		}
		s.Step(in, nil)
	}
	if s.Phase != MatchOver || s.Players[s.Winner].Wins != WinsToMatch {
		t.Fatalf("phase %v winner %d", s.Phase, s.Winner)
	}
}

func TestDeterminism(t *testing.T) {
	a, b := NewMatch(42, 6), NewMatch(42, 6)
	script := NewMatch(99, 1)
	actions := []Action{TurnLeft, TurnRight, Boost, Jump}
	for tick := 0; tick < 2000; tick++ {
		var in []Input
		if n := script.next(); n%3 == 0 {
			in = append(in, Input{uint8(n>>8) % 6, actions[(n>>16)%4]})
		}
		a.Step(in, nil)
		b.Step(in, nil)
		if a.Hash() != b.Hash() {
			t.Fatalf("desync at tick %d", tick)
		}
	}
}

func TestQuitAfterCrashStillLeaves(t *testing.T) {
	s := running(t, Player{X: W - 1, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left}, Player{X: 50, Y: 20, Dir: Left})
	s.Step(nil, nil)
	s.Step([]Input{{Player: 0, Action: Quit}}, nil)
	if !s.Players[0].Left {
		t.Fatal("a crashed player who quits must be marked Left")
	}
}
