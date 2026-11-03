package sim

type EventKind uint8

const (
	EvCountdown EventKind = iota // Player holds the seconds left
	EvRoundStart
	EvCrash // Cause and By say what the player hit
	EvPickup
	EvJump
	EvBoost
	EvSudden
	EvRoundWon // Player is the winner, or NoPlayer for a draw
	EvMatchWon
)

type Cause uint8

const (
	HitWall Cause = iota
	HitSelf
	HitTrail  // By is the trail's owner
	HitHeadOn // By is the other rider
	HitQuit
)

const NoPlayer = 0xFF

type Event struct {
	Kind   EventKind
	Player uint8
	By     uint8
	Cause  Cause
	X, Y   int16
}

// Step advances one tick, applying inputs in order. Events are appended to
// out so callers can reuse one buffer across frames.
func (s *State) Step(inputs []Input, out []Event) []Event {
	s.Tick++
	switch s.Phase {
	case Countdown:
		s.PhaseTicks--
		if s.PhaseTicks == 0 {
			s.Phase = Running
			out = append(out, Event{Kind: EvRoundStart})
		} else if s.PhaseTicks%s.Rate == 0 {
			out = append(out, Event{Kind: EvCountdown, Player: uint8(s.PhaseTicks / s.Rate)})
		}
		out = s.applyQuits(inputs, out)
	case Running:
		out = s.applyInputs(inputs, out)
		out = s.run(out)
	case RoundOver:
		out = s.applyQuits(inputs, out)
		s.PhaseTicks--
		if s.PhaseTicks > 0 {
			break
		}
		if s.matchWinner() >= 0 {
			s.Phase = MatchOver
			s.Winner = s.matchWinner()
			out = append(out, Event{Kind: EvMatchWon, Player: uint8(s.Winner)})
		} else {
			s.newRound()
		}
	}
	return out
}

func (s *State) applyQuits(inputs []Input, out []Event) []Event {
	for _, in := range inputs {
		if in.Action == Quit && in.Player < s.Count {
			out = s.leave(in.Player, out)
		}
	}
	return out
}

func (s *State) leave(id uint8, out []Event) []Event {
	p := &s.Players[id]
	p.Left = true
	if p.Alive {
		p.Alive = false
		out = append(out, Event{Kind: EvCrash, Player: id, Cause: HitQuit, X: p.X, Y: p.Y})
	}
	return out
}

func (s *State) applyInputs(inputs []Input, out []Event) []Event {
	for _, in := range inputs {
		if in.Player >= s.Count {
			continue
		}
		if in.Action == Quit {
			out = s.leave(in.Player, out)
			continue
		}
		p := &s.Players[in.Player]
		if !p.Alive {
			continue
		}
		switch in.Action {
		case TurnLeft:
			p.enqueue(p.lastDir().TurnLeft())
		case TurnRight:
			p.enqueue(p.lastDir().TurnRight())
		case FaceUp, FaceRight, FaceDown, FaceLeft:
			p.enqueue(Dir(in.Action - FaceUp))
		case Boost:
			if p.BoostTicks == 0 && p.Boost >= BoostCost {
				p.Boost -= BoostCost
				p.BoostTicks = uint16(s.secs(3) / 2)
				out = append(out, Event{Kind: EvBoost, Player: in.Player, X: p.X, Y: p.Y})
			}
		case Jump:
			if p.JumpCooldown == 0 && p.JumpTicks == 0 {
				p.JumpTicks, p.JumpCooldown = JumpDuration, uint16(s.secs(4))
				out = append(out, Event{Kind: EvJump, Player: in.Player, X: p.X, Y: p.Y})
			}
		}
	}
	return out
}

func (p *Player) lastDir() Dir {
	if p.queued > 0 {
		return p.queue[p.queued-1]
	}
	return p.Dir
}

// enqueue keeps up to two pending turns so a fast double tap lands on two
// consecutive ticks instead of the second press overwriting the first.
func (p *Player) enqueue(d Dir) {
	last := p.lastDir()
	if d == last || d == last.Reverse() || p.queued == uint8(len(p.queue)) {
		return
	}
	p.queue[p.queued] = d
	p.queued++
}

func (p *Player) popTurn() {
	if p.queued == 0 {
		return
	}
	p.Dir = p.queue[0]
	p.queue[0] = p.queue[1]
	p.queued--
}

func (s *State) run(out []Event) []Event {
	s.RunTicks++
	for i := uint8(0); i < s.Count; i++ {
		if p := &s.Players[i]; p.Alive {
			p.popTurn()
		}
	}
	out = s.moveAll(false, out)
	out = s.moveAll(true, out)
	refill := s.Rate / 5
	for i := uint8(0); i < s.Count; i++ {
		p := &s.Players[i]
		if p.BoostTicks > 0 {
			p.BoostTicks--
		} else if p.Boost < BoostMax && s.RunTicks%uint32(refill) == 0 {
			p.Boost++
		}
		if p.JumpTicks > 0 {
			p.JumpTicks--
		}
		if p.JumpCooldown > 0 {
			p.JumpCooldown--
		}
		if !p.Alive {
			s.retract(i, 2)
		}
	}
	if !s.Sudden && s.RunTicks >= s.secs(60) {
		s.Sudden, s.suddenAt = true, s.RunTicks
		out = append(out, Event{Kind: EvSudden})
	}
	s.spawnSpark()
	return s.checkRoundEnd(out)
}

// moveAll moves every alive player one cell at once. The boost pass moves
// only boosting players, giving them two cells per tick.
func (s *State) moveAll(boostPass bool, out []Event) []Event {
	var tx, ty [MaxPlayers]int16
	var moving, crashed [MaxPlayers]bool
	var cause [MaxPlayers]Cause
	var by [MaxPlayers]uint8
	for i := uint8(0); i < s.Count; i++ {
		p := &s.Players[i]
		if !p.Alive || (boostPass && !p.Boosting()) {
			continue
		}
		dx, dy := p.Dir.Delta()
		tx[i], ty[i], moving[i] = p.X+dx, p.Y+dy, true
		c := s.At(tx[i], ty[i])
		switch o, isTrail := c.Owner(); {
		case c == Wall:
			crashed[i], cause[i] = true, HitWall
		case isTrail && !p.Jumping():
			crashed[i], cause[i], by[i] = true, HitTrail, o
			if o == i {
				cause[i] = HitSelf
			}
		}
	}
	for i := uint8(0); i < s.Count; i++ {
		for j := i + 1; j < s.Count; j++ {
			if moving[i] && moving[j] && tx[i] == tx[j] && ty[i] == ty[j] {
				crashed[i], crashed[j] = true, true
				cause[i], cause[j], by[i], by[j] = HitHeadOn, HitHeadOn, j, i
			}
		}
	}
	for i := uint8(0); i < s.Count; i++ {
		if !moving[i] {
			continue
		}
		p := &s.Players[i]
		if crashed[i] {
			p.Alive = false
			out = append(out, Event{Kind: EvCrash, Player: i, Cause: cause[i], By: by[i], X: tx[i], Y: ty[i]})
			continue
		}
		p.X, p.Y = tx[i], ty[i]
		cell := &s.Grid[s.idx(p.X, p.Y)]
		if *cell == Spark {
			s.Sparks--
			p.Boost, p.JumpCooldown = BoostMax, 0
			p.Bonus += SparkBonus
			out = append(out, Event{Kind: EvPickup, Player: i, X: p.X, Y: p.Y})
			*cell = Empty
		}
		if !p.Jumping() && *cell == Empty {
			s.paint(i)
		}
		s.retract(i, p.TrailCells()-s.MaxTrail(i))
	}
	return out
}

func (s *State) checkRoundEnd(out []Event) []Event {
	alive := s.Alive()
	if alive > 1 || (alive == 1 && s.Count == 1) {
		return out
	}
	s.Phase, s.PhaseTicks, s.Winner = RoundOver, uint16(s.secs(2)), -1
	for i := uint8(0); i < s.Count; i++ {
		if p := &s.Players[i]; p.Alive {
			p.Wins++
			s.Winner = int8(i)
		}
	}
	w := uint8(NoPlayer)
	if s.Winner >= 0 {
		w = uint8(s.Winner)
	}
	return append(out, Event{Kind: EvRoundWon, Player: w})
}

func (s *State) matchWinner() int8 {
	for i := uint8(0); i < s.Count; i++ {
		if s.Players[i].Wins >= WinsToMatch {
			return int8(i)
		}
	}
	present := 0
	last := int8(-1)
	for i := uint8(0); i < s.Count; i++ {
		if !s.Players[i].Left {
			present++
			last = int8(i)
		}
	}
	if present <= 1 && s.Count > 1 {
		return last
	}
	return -1
}
