// Package sim is the deterministic game core. It uses integer math and a
// seeded PRNG only, so every peer that applies the same inputs in the same
// order reaches the same State. Step is the only mutator.
package sim

const (
	MaxPlayers  = 6
	WinsToMatch = 5

	TrailLen   = 70 // cells a trail keeps before its tail starts erasing
	SparkBonus = 25 // extra trail cells per spark picked up

	BoostMax     = 100
	BoostCost    = 40
	JumpDuration = 3
	maxSparks    = 3
)

// Size and Speed are the host's choices. Config stores indices into these
// tables, so a config read off the wire can only name a size that exists.
type Size struct {
	Name string
	W, H int16
}

type Speed struct {
	Name string
	Rate uint8 // ticks per second
}

var Sizes = [...]Size{{"Chica", 60, 34}, {"Mediana", 70, 40}, {"Grande", 100, 56}, {"Enorme", 140, 80}}
var Speeds = [...]Speed{{"Tranqui", 10}, {"Normal", 15}, {"Rápida", 20}, {"Furiosa", 25}}

type Config struct{ Size, Speed uint8 }

var DefaultConfig = Config{Size: 1, Speed: 1}

func (c Config) Valid() bool  { return int(c.Size) < len(Sizes) && int(c.Speed) < len(Speeds) }
func (c Config) W() int16     { return Sizes[c.Size].W }
func (c Config) H() int16     { return Sizes[c.Size].H }
func (c Config) Rate() uint16 { return uint16(Speeds[c.Speed].Rate) }

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
	BoostTicks   uint16
	JumpTicks    uint8
	JumpCooldown uint16
	Wins         uint8
	Steps        uint32 // cells painted this round; Stamp records it per cell
	Bonus        uint16 // trail cells earned from sparks this round
	trail        []int32
	tail         int
}

// Boosting and Jumping are what renderers and bots read.
func (p *Player) Boosting() bool { return p.BoostTicks > 0 }
func (p *Player) Jumping() bool  { return p.JumpTicks > 0 }

// TrailCells is how many cells the trail covers right now.
func (p *Player) TrailCells() int { return len(p.trail) - p.tail }

type State struct {
	Cfg        Config
	W, H       int16
	Rate       uint16
	Tick       uint32
	Phase      Phase
	PhaseTicks uint16
	RunTicks   uint32
	Round      uint8
	Sparks     uint8
	Sudden     bool
	suddenAt   uint32
	Winner     int8 // last round or match winner, -1 for a draw
	Count      uint8
	Players    [MaxPlayers]Player
	Grid       []Cell
	Stamp      []uint32
	rng        uint64
}

// spawns are per-mille positions so every arena size keeps the same layout.
var spawns = [MaxPlayers]struct {
	fx, fy int32
	dir    Dir
}{
	{120, 214, Right}, {870, 786, Left}, {870, 214, Down},
	{120, 786, Up}, {350, 107, Down}, {640, 893, Up},
}

// NewMatch builds the first round of a match for count players.
func NewMatch(seed uint64, count uint8, cfg Config) *State {
	if count > MaxPlayers {
		count = MaxPlayers
	}
	if !cfg.Valid() {
		cfg = DefaultConfig
	}
	w, h := cfg.W(), cfg.H()
	s := &State{Cfg: cfg, W: w, H: h, Rate: cfg.Rate(), Count: count, Winner: -1,
		rng: seed ^ 0x9E3779B97F4A7C15, Grid: make([]Cell, int(w)*int(h)), Stamp: make([]uint32, int(w)*int(h))}
	s.newRound()
	return s
}

// secs converts seconds to ticks at this match's speed.
func (s *State) secs(n uint32) uint32 { return n * uint32(s.Rate) }

func (s *State) newRound() {
	clear(s.Grid)
	s.Phase, s.PhaseTicks = Countdown, uint16(s.secs(3))
	s.RunTicks, s.Sparks, s.Sudden = 0, 0, false
	s.Round++
	for i := uint8(0); i < s.Count; i++ {
		p := &s.Players[i]
		sp := spawns[i]
		x, y := int16(int32(s.W)*sp.fx/1000), int16(int32(s.H)*sp.fy/1000)
		*p = Player{X: x, Y: y, Dir: sp.dir, Alive: !p.Left, Left: p.Left, Boost: BoostMax, Wins: p.Wins, trail: p.trail[:0]}
		if p.Alive {
			s.paint(i)
		}
	}
}

func (s *State) idx(x, y int16) int { return int(y)*int(s.W) + int(x) }

// At returns the cell at (x, y); anything outside the arena is a wall.
func (s *State) At(x, y int16) Cell {
	if x < 0 || y < 0 || x >= s.W || y >= s.H {
		return Wall
	}
	return s.Grid[s.idx(x, y)]
}

// MaxTrail is how long player p's trail may grow right now. Once sudden
// death starts trails stop shrinking, so a long round has to end.
func (s *State) MaxTrail(p uint8) int {
	n := TrailLen + int(s.Players[p].Bonus)
	if s.Sudden {
		n += int(s.RunTicks - s.suddenAt)
	}
	return n
}

// TrailAge says how far back from the head the trail cell at (x, y) is, in
// cells, so renderers can fade the tail before it erases.
func (s *State) TrailAge(x, y int16) (age, max int) {
	o, ok := s.At(x, y).Owner()
	if !ok {
		return 0, 0
	}
	return int(s.Players[o].Steps - 1 - s.Stamp[s.idx(x, y)]), s.MaxTrail(o)
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
