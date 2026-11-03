package render

import "github.com/LeandroMAcosta/vibetron/internal/sim"

// Palette is one player's color: Mid is the flat trail color, Core the
// head, Mid lightened 55% toward white.
type Palette struct {
	Core, Mid RGB
	Name      string
}

var Players = neon([sim.MaxPlayers]RGB{
	{0x2e, 0xf2, 0xff}, {0xff, 0x3f, 0xb4}, {0xff, 0xd2, 0x3f},
	{0x7c, 0xff, 0x6b}, {0xb0, 0x7c, 0xff}, {0xff, 0x8a, 0x4c},
}, [sim.MaxPlayers]string{"Cian", "Magenta", "Ámbar", "Lima", "Violeta", "Coral"})

func neon(cols [sim.MaxPlayers]RGB, names [sim.MaxPlayers]string) [sim.MaxPlayers]Palette {
	var p [sim.MaxPlayers]Palette
	for i, c := range cols {
		p[i] = Palette{Core: lerp(c, white, 0.55), Mid: c, Name: names[i]}
	}
	return p
}

var (
	white     = RGB{255, 255, 255}
	black     = RGB{}
	bgColor   = RGB{0x07, 0x07, 0x0d}
	dimColor  = RGB{0x26, 0x24, 0x38}
	sparkGold = RGB{255, 235, 135}
	lilac     = RGB{200, 185, 255}
)
