package sim

import "testing"

var big = Config{Size: 2, Speed: 2} // 100x56 at 20 ticks/s

// running returns a state past the countdown with players placed by hand.
func running(t *testing.T, players ...Player) *State {
	t.Helper()
	s := NewMatch(1, uint8(len(players)), big)
	for s.Phase != Running {
		s.Step(nil, nil)
	}
	clear(s.Grid)
	for i, p := range players {
		p.Alive, p.Boost = true, BoostMax
		s.Players[i] = p
		s.paint(uint8(i))
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

func crashOf(evs []Event, player uint8) (Event, bool) {
	for _, e := range evs {
		if e.Kind == EvCrash && e.Player == player {
			return e, true
		}
	}
	return Event{}, false
}

func TestConfigSetsArenaAndRate(t *testing.T) {
	for si := range Sizes {
		for vi := range Speeds {
			cfg := Config{Size: uint8(si), Speed: uint8(vi)}
			s := NewMatch(3, 6, cfg)
			if s.W != Sizes[si].W || s.H != Sizes[si].H || len(s.Grid) != int(s.W)*int(s.H) || s.Rate != uint16(Speeds[vi].Rate) {
				t.Fatalf("config %+v built %dx%d at %d", cfg, s.W, s.H, s.Rate)
			}
			for i := 0; i < 6; i++ {
				p := s.Players[i]
				if p.X <= 0 || p.Y <= 0 || p.X >= s.W-1 || p.Y >= s.H-1 {
					t.Fatalf("spawn %d off the arena in %+v: %d,%d", i, cfg, p.X, p.Y)
				}
			}
		}
	}
	if (Config{Size: 9}).Valid() || NewMatch(1, 2, Config{Size: 9}).Cfg != DefaultConfig {
		t.Fatal("invalid config must fall back to the default")
	}
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

func TestTrailKeepsItsLength(t *testing.T) {
	s := running(t, Player{X: 2, Y: 10, Dir: Right}, Player{X: 2, Y: 40, Dir: Right})
	for i := 0; i < TrailLen+10; i++ {
		s.Step(nil, nil)
	}
	if n := s.Players[0].TrailCells(); n != TrailLen {
		t.Fatalf("trail covers %d cells, want %d", n, TrailLen)
	}
	if s.At(2, 10) != Empty || s.At(s.Players[0].X, 10) != TrailOf(0) {
		t.Fatal("tail should erase while the head keeps painting")
	}
	if age, max := s.TrailAge(s.Players[0].X-1, 10); age != 1 || max != TrailLen {
		t.Fatalf("TrailAge = %d/%d, want 1/%d", age, max, TrailLen)
	}
}

func TestWallCrashEndsRound(t *testing.T) {
	s := running(t, Player{X: s0W() - 1, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left})
	evs := s.Step(nil, nil)
	e, ok := crashOf(evs, 0)
	if !ok || e.Cause != HitWall || s.Players[0].Alive {
		t.Fatalf("expected wall crash, got %+v", e)
	}
	if s.Phase != RoundOver || s.Winner != 1 || s.Players[1].Wins != 1 {
		t.Fatalf("phase %v winner %d wins %d", s.Phase, s.Winner, s.Players[1].Wins)
	}
}

func s0W() int16 { return big.W() }

func TestTrailCrashNamesTheOwner(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left}, Player{X: 80, Y: 5, Dir: Down})
	s.Grid[s.idx(11, 10)] = TrailOf(1)
	evs := s.Step(nil, nil)
	if e, _ := crashOf(evs, 0); e.Cause != HitTrail || e.By != 1 {
		t.Fatalf("crash = %+v, want trail of player 1", e)
	}
	if s.Phase != Running {
		t.Fatal("two players remain, round should continue")
	}
}

func TestOwnTrailIsHitSelf(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left}, Player{X: 80, Y: 5, Dir: Down})
	s.Step(nil, nil)
	s.Step(nil, nil)
	s.Step([]Input{{Player: 0, Action: FaceDown}}, nil)
	s.Step([]Input{{Player: 0, Action: FaceLeft}}, nil)
	evs := s.Step([]Input{{Player: 0, Action: FaceUp}}, nil)
	if e, ok := crashOf(evs, 0); !ok || e.Cause != HitSelf {
		t.Fatalf("crash = %+v, want own trail", e)
	}
}

func TestHeadOnSameCellCrashesBoth(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right}, Player{X: 12, Y: 10, Dir: Left})
	evs := s.Step(nil, nil)
	if e, _ := crashOf(evs, 0); e.Cause != HitHeadOn || e.By != 1 || s.Players[1].Alive {
		t.Fatalf("both should crash head-on, got %+v", e)
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
	s.Grid[s.idx(12, 10)] = TrailOf(1)
	s.Step([]Input{{Player: 0, Action: Jump}}, nil)
	s.Step(nil, nil)
	s.Step(nil, nil)
	p := s.Players[0]
	if !p.Alive || p.X != 13 {
		t.Fatalf("alive=%v x=%d, want alive at 13", p.Alive, p.X)
	}
	if s.At(11, 10) != Empty || s.At(12, 10) != TrailOf(1) {
		t.Fatal("jump must leave no trail and keep the trail it crossed")
	}
	s.Step(nil, nil)
	if s.At(14, 10) != TrailOf(0) {
		t.Fatal("trail should resume after landing")
	}
}

func TestBoostMovesTwoAndChecksEachCell(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left}, Player{X: 10, Y: 30, Dir: Right})
	s.Step([]Input{{Player: 0, Action: Boost}}, nil)
	if s.Players[0].X != 12 || s.At(11, 10) != TrailOf(0) {
		t.Fatalf("boost x=%d, want 12 with both cells painted", s.Players[0].X)
	}
	s.Grid[s.idx(12, 30)] = TrailOf(1)
	s.Step([]Input{{Player: 2, Action: Boost}}, nil)
	if s.Players[2].Alive {
		t.Fatal("boost must not skip over a trail cell")
	}
}

func TestQueuedTurnsAndNoReverse(t *testing.T) {
	s := running(t, Player{X: 20, Y: 20, Dir: Right}, Player{X: 50, Y: 40, Dir: Left})
	s.Step([]Input{{Player: 0, Action: FaceLeft}}, nil)
	if s.Players[0].Dir != Right {
		t.Fatal("reverse must be ignored")
	}
	s.Step([]Input{{Player: 0, Action: FaceUp}, {Player: 0, Action: FaceLeft}}, nil)
	if s.Players[0].Dir != Up {
		t.Fatal("first queued turn should apply")
	}
	s.Step(nil, nil)
	if s.Players[0].Dir != Left {
		t.Fatal("second queued turn should apply on the next tick")
	}
}

func TestSparkPickupRefillsAndGrowsTrail(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left})
	s.Grid[s.idx(11, 10)] = Spark
	s.Sparks = 1
	s.Players[0].Boost, s.Players[0].JumpCooldown = 0, 50
	evs := s.Step(nil, nil)
	p := s.Players[0]
	if !hasEvent(evs, EvPickup, 0) || p.Boost != BoostMax || p.JumpCooldown != 0 || s.Sparks != 0 {
		t.Fatalf("pickup failed: boost=%d cd=%d", p.Boost, p.JumpCooldown)
	}
	if s.MaxTrail(0) != TrailLen+SparkBonus {
		t.Fatalf("max trail %d, want %d", s.MaxTrail(0), TrailLen+SparkBonus)
	}
}

func TestDeadTrailRetracts(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left}, Player{X: 80, Y: 5, Dir: Down})
	for i := 0; i < 20; i++ {
		s.Step(nil, nil)
	}
	s.Step([]Input{{Player: 0, Action: Quit}}, nil)
	for i := 0; i < 11; i++ {
		s.Step(nil, nil)
	}
	if n := s.Players[0].TrailCells(); n != 0 {
		t.Fatalf("dead trail still covers %d cells", n)
	}
}

func TestSuddenDeathStopsShrinking(t *testing.T) {
	s := running(t, Player{X: 2, Y: 10, Dir: Right}, Player{X: 2, Y: 40, Dir: Right})
	s.RunTicks = s.secs(60) - 1
	evs := s.Step(nil, nil)
	if !s.Sudden || !hasEvent(evs, EvSudden, 0) {
		t.Fatal("expected sudden death at 60 s")
	}
	before := s.MaxTrail(0)
	s.Step(nil, nil)
	if s.MaxTrail(0) != before+1 {
		t.Fatal("during sudden death the allowed trail grows every tick")
	}
}

func TestQuitAfterCrashStillLeaves(t *testing.T) {
	s := running(t, Player{X: s0W() - 1, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left}, Player{X: 50, Y: 20, Dir: Left})
	s.Step(nil, nil)
	s.Step([]Input{{Player: 0, Action: Quit}}, nil)
	if !s.Players[0].Left {
		t.Fatal("a crashed player who quits must be marked Left")
	}
}

func TestQuitRemovesPlayerAndEndsMatch(t *testing.T) {
	s := running(t, Player{X: 10, Y: 10, Dir: Right}, Player{X: 50, Y: 40, Dir: Left})
	s.Step([]Input{{Player: 1, Action: Quit}}, nil)
	for s.Phase == RoundOver {
		s.Step(nil, nil)
	}
	if s.Phase != MatchOver || s.Winner != 0 {
		t.Fatalf("phase %v winner %d", s.Phase, s.Winner)
	}
}

func TestMatchEndsAtWinsToMatch(t *testing.T) {
	s := NewMatch(7, 2, DefaultConfig)
	for s.Phase != MatchOver && s.Tick < 100000 {
		var in []Input
		if s.Phase == Running && s.RunTicks == 3 {
			in = []Input{{Player: 1, Action: TurnLeft}}
		}
		s.Step(in, nil)
	}
	if s.Phase != MatchOver || s.Players[s.Winner].Wins != WinsToMatch {
		t.Fatalf("phase %v winner %d", s.Phase, s.Winner)
	}
}

func TestDeterminism(t *testing.T) {
	for _, cfg := range []Config{DefaultConfig, {Size: 3, Speed: 3}, {Size: 0, Speed: 0}} {
		a, b := NewMatch(42, 6, cfg), NewMatch(42, 6, cfg)
		script := NewMatch(99, 1, cfg)
		actions := []Action{TurnLeft, TurnRight, Boost, Jump}
		for tick := 0; tick < 3000; tick++ {
			var in []Input
			if n := script.next(); n%3 == 0 {
				in = append(in, Input{Player: uint8(n>>8) % 6, Action: actions[(n>>16)%4]})
			}
			a.Step(in, nil)
			b.Step(in, nil)
			if a.Hash() != b.Hash() {
				t.Fatalf("%+v: desync at tick %d", cfg, tick)
			}
		}
	}
}
