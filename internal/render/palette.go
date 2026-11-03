package render

import "estela/internal/sim"

// Palette is one player's neon: Core is the white-hot tint, Mid the trail
// body, Glow the halo. Mid and Glow sit on xterm 6x6x6 cube levels
// (0, 95, 135, 175, 215, 255) so hues hold when tcell drops to 256 colors.
type Palette struct {
	Core, Mid, Glow RGB
	Name            string
}

var Players = [sim.MaxPlayers]Palette{
	{RGB{215, 255, 255}, RGB{0, 215, 255}, RGB{0, 95, 135}, "Cian"},
	{RGB{255, 215, 255}, RGB{255, 0, 215}, RGB{135, 0, 95}, "Magenta"},
	{RGB{255, 255, 215}, RGB{255, 175, 0}, RGB{135, 95, 0}, "Ámbar"},
	{RGB{215, 255, 175}, RGB{135, 255, 0}, RGB{0, 95, 0}, "Lima"},
	{RGB{235, 215, 255}, RGB{175, 95, 255}, RGB{95, 0, 175}, "Violeta"},
	{RGB{255, 235, 215}, RGB{255, 135, 95}, RGB{135, 55, 0}, "Coral"},
}

var (
	white     = RGB{255, 255, 255}
	black     = RGB{}
	wallHot   = RGB{255, 40, 80}
	wallCold  = RGB{95, 0, 35}
	sparkGold = RGB{255, 235, 135}
	bgTop     = RGB{8, 6, 22}
	bgBottom  = RGB{14, 4, 28}
	gridLine  = RGB{30, 8, 64}
)
