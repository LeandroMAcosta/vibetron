// Package sim is the deterministic game core. It uses integer math and a
// seeded PRNG only, so every peer that applies the same inputs in the same
// order reaches the same State. Step is the only mutator.
package sim

const (
	W          = 100
	H          = 56
	MaxPlayers = 6
	TickRate   = 20

	WinsToMatch = 5

	countdownTicks = 3 * TickRate
	roundOverTicks = 2 * TickRate
	collapseAfter  = 30 * TickRate
	collapseEvery  = 2 * TickRate
	maxRing        = 20
	sparkEvery     = 5 * TickRate
	maxSparks      = 3

	BoostMax      = 100
	BoostCost     = 40
	boostDuration = 30
	boostRefill   = 4
	JumpDuration  = 3
	JumpCooldown  = 4 * TickRate
)

// Cell holds what occupies one arena pixel.
type Cell uint8

const (
	Empty Cell = 0
	Spark Cell = 0xFE
	Wall  Cell = 0xFF
)

// TrailOf returns the trail cell for player p.
func TrailOf(p uint8) Cell { return Cell(p + 1) }

// Owner reports which player left this trail.
func (c Cell) Owner() (uint8, bool) {
	if c == Empty || c > MaxPlayers {
		return 0, false
	}
	return uint8(c - 1), true
}

type Dir uint8

const (
	Up Dir = iota
	Right
	Down
	Left
)

var dirDX = [4]int16{0, 1, 0, -1}
var dirDY = [4]int16{-1, 0, 1, 0}

func (d Dir) Delta() (int16, int16) { return dirDX[d&3], dirDY[d&3] }
func (d Dir) Reverse() Dir          { return (d + 2) & 3 }
func (d Dir) TurnLeft() Dir         { return (d + 3) & 3 }
func (d Dir) TurnRight() Dir        { return (d + 1) & 3 }

type Action uint8

const (
	TurnLeft Action = iota
	TurnRight
	FaceUp
	FaceRight
	FaceDown
	FaceLeft
	Boost
	Jump
	Quit
)

type Input struct {
	Player uint8
	Action Action
}

type Phase uint8

const (
	Countdown Phase = iota
	Running
	RoundOver
	MatchOver
)

type Player struct {
	X, Y         int16
	Dir          Dir
	queue        [2]Dir
	queued       uint8
	Alive        bool
	Left         bool
	Boost        uint8
	BoostTicks   uint8
	JumpTicks    uint8
	JumpCooldown uint8
	Wins         uint8
}

// Boosting and Jumping are what renderers and bots read.
func (p *Player) Boosting() bool { return p.BoostTicks > 0 }
func (p *Player) Jumping() bool  { return p.JumpTicks > 0 }

type State struct {
	Tick       uint32
	Phase      Phase
	PhaseTicks uint16
	RunTicks   uint16
	Round      uint8
	Ring       uint8
	Sparks     uint8
	Winner     int8 // last round or match winner, -1 for a draw
	Count      uint8
	Players    [MaxPlayers]Player
	Grid       [W * H]Cell
	rng        uint64
}

type spawn struct {
	x, y int16
	dir  Dir
}

var spawns = [MaxPlayers]spawn{
	{12, 12, Right}, {87, 43, Left}, {87, 12, Down},
	{12, 43, Up}, {35, 6, Down}, {64, 49, Up},
}

// NewMatch builds the first round of a match for count players.
func NewMatch(seed uint64, count uint8) *State {
	if count > MaxPlayers {
		count = MaxPlayers
	}
	s := &State{Count: count, rng: seed ^ 0x9E3779B97F4A7C15, Winner: -1}
	s.newRound()
	return s
}

func (s *State) newRound() {
	s.Grid = [W * H]Cell{}
	s.Phase, s.PhaseTicks = Countdown, countdownTicks
	s.RunTicks, s.Ring, s.Sparks = 0, 0, 0
	s.Round++
	for i := uint8(0); i < s.Count; i++ {
		p := &s.Players[i]
		sp := spawns[i]
		*p = Player{X: sp.x, Y: sp.y, Dir: sp.dir, Alive: !p.Left, Left: p.Left, Boost: BoostMax, Wins: p.Wins}
		if p.Alive {
			s.Grid[idx(sp.x, sp.y)] = TrailOf(i)
		}
	}
}

func idx(x, y int16) int { return int(y)*W + int(x) }

// At returns the cell at (x, y); anything outside the arena is a wall.
func (s *State) At(x, y int16) Cell {
	if x < 0 || y < 0 || x >= W || y >= H {
		return Wall
	}
	return s.Grid[idx(x, y)]
}

// Alive counts players still in the round.
func (s *State) Alive() int {
	n := 0
	for i := uint8(0); i < s.Count; i++ {
		if s.Players[i].Alive {
			n++
		}
	}
	return n
}

// next is splitmix64: tiny, fast and identical on every platform.
func (s *State) next() uint64 {
	s.rng += 0x9E3779B97F4A7C15
	z := s.rng
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
