package bot

import (
	"slices"
	"sync"
	"testing"

	"estela/internal/sim"
)

const (
	matches  = 50
	seats    = 6
	tickCap  = 200_000
	turnOdds = 10 // the random baseline turns on one tick in turnOdds
)

type thinker interface {
	Think(s *sim.State) (sim.Action, bool)
}

// randomTurner is the baseline: it never looks at the board.
type randomTurner struct{ rng uint64 }

func (r *randomTurner) Think(s *sim.State) (sim.Action, bool) {
	r.rng = r.rng*6364136223846793005 + 1442695040888963407
	n := r.rng >> 33
	if s.Phase != sim.Running || n%turnOdds != 0 {
		return 0, false
	}
	return sim.TurnLeft + sim.Action(n/turnOdds%2), true
}

type matchResult struct {
	rounds []int
	ticks  int
	over   bool
}

func playMatch(seed uint64, seat func(id uint8, seed uint64) thinker) matchResult {
	s := sim.NewMatch(seed, seats)
	var minds [seats]thinker
	for i := range minds {
		minds[i] = seat(uint8(i), seed*31+uint64(i))
	}
	var inputs []sim.Input
	var events []sim.Event
	var r matchResult
	for r.ticks = 0; r.ticks < tickCap && s.Phase != sim.MatchOver; r.ticks++ {
		inputs = inputs[:0]
		for i, m := range minds {
			if a, ok := m.Think(s); ok {
				inputs = append(inputs, sim.Input{Player: uint8(i), Action: a})
			}
		}
		was := s.Phase
		events = s.Step(inputs, events[:0])
		if was == sim.Running && s.Phase == sim.RoundOver {
			r.rounds = append(r.rounds, int(s.RunTicks))
		}
	}
	r.over = s.Phase == sim.MatchOver
	return r
}

func playAll(seat func(id uint8, seed uint64) thinker) []matchResult {
	out := make([]matchResult, matches)
	var wg sync.WaitGroup
	for m := range out {
		wg.Go(func() { out[m] = playMatch(uint64(m+1), seat) })
	}
	wg.Wait()
	return out
}

func medianRound(rs []matchResult) int {
	var all []int
	for _, r := range rs {
		all = append(all, r.rounds...)
	}
	slices.Sort(all)
	return all[len(all)/2]
}

func TestBotsOutlastRandomTurners(t *testing.T) {
	bots := playAll(func(id uint8, seed uint64) thinker { return New(id, seed) })
	random := playAll(func(_ uint8, seed uint64) thinker { return &randomTurner{rng: seed} })
	for i, r := range bots {
		if !r.over {
			t.Fatalf("bot match %d did not finish within %d ticks", i, tickCap)
		}
	}
	botMed, randMed := medianRound(bots), medianRound(random)
	t.Logf("median round length: bots %d ticks, random turners %d ticks", botMed, randMed)
	if botMed < 3*randMed {
		t.Fatalf("bots median %d is not clearly above random %d", botMed, randMed)
	}
}
