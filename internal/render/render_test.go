package render

import (
	"testing"

	"estela/internal/sim"
	"github.com/gdamore/tcell/v2"
)

func simScreen(t testing.TB, w, h int) tcell.SimulationScreen {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	s.SetSize(w, h)
	return s
}

func TestBlitMapsTopAndBottomPixelsToOneCell(t *testing.T) {
	scr := simScreen(t, 20, 10)
	f := NewFrame(4, 4)
	top, bot := RGB{10, 200, 30}, RGB{250, 5, 120}
	f.Set(2, 2, top)
	f.Set(2, 3, bot)
	f.Blit(scr, 3, 1)

	str, st, _ := scr.Get(3+2, 1+1)
	if str != "▀" {
		t.Fatalf("cell = %q, want half block", str)
	}
	fg, bg, _ := st.Decompose()
	if want := tcell.NewRGBColor(10, 200, 30); fg != want {
		t.Errorf("fg = %v, want top pixel %v", fg, want)
	}
	if want := tcell.NewRGBColor(250, 5, 120); bg != want {
		t.Errorf("bg = %v, want bottom pixel %v", bg, want)
	}
}

func TestBlitClipsToScreen(t *testing.T) {
	scr := simScreen(t, 3, 2)
	f := NewFrame(10, 10)
	f.Blit(scr, -2, -1)
	f.Blit(scr, 2, 1)
}

func TestTextWidth(t *testing.T) {
	cases := []struct {
		s    string
		big  bool
		want int
	}{
		{"", false, 0},
		{"A", false, 3},
		{"AB", false, 7},
		{"M", false, 5},
		{"a.", false, 5},
		{"A", true, 5},
		{"ESTELA", true, 6*5 + 5},
		{"Á", false, 3},
		{"ñ", true, 5},
		{"¡HOLA!", true, 2 + 5 + 5 + 5 + 5 + 2 + 5},
	}
	for _, c := range cases {
		if got := TextWidth(c.s, c.big); got != c.want {
			t.Errorf("TextWidth(%q, %v) = %d, want %d", c.s, c.big, got, c.want)
		}
	}
}

func TestDrawTextReturnsTextWidth(t *testing.T) {
	f := NewFrame(120, 20)
	for _, s := range []string{"ÁMBAR", "¿Qué pasó? 100%", "jugadores: 6"} {
		for _, big := range []bool{false, true} {
			if got, want := DrawText(f, 0, 5, s, white, big), TextWidth(s, big); got != want {
				t.Errorf("DrawText(%q, %v) = %d, TextWidth = %d", s, big, got, want)
			}
		}
	}
}

func TestLowercaseDrawsLikeUppercase(t *testing.T) {
	a, b := NewFrame(40, 12), NewFrame(40, 12)
	DrawText(a, 1, 3, "estela ñú", white, false)
	DrawText(b, 1, 3, "ESTELA ÑÚ", white, false)
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			t.Fatalf("pixel %d differs", i)
		}
	}
}

func TestAccentDrawsAboveLetter(t *testing.T) {
	plain, acc := NewFrame(10, 12), NewFrame(10, 12)
	DrawText(plain, 1, 4, "E", white, true)
	DrawText(acc, 1, 4, "É", white, true)
	above := 0
	for y := 0; y < 4; y++ {
		for x := 0; x < 10; x++ {
			if acc.Pix[y*10+x] != black {
				above++
			}
			if plain.Pix[y*10+x] != black {
				t.Fatalf("plain E drew above its top")
			}
		}
	}
	if above == 0 {
		t.Fatal("É drew no accent above the letter")
	}
}

func TestEveryGlyphIsRectangular(t *testing.T) {
	for i, src := range [2]map[rune]string{smallGlyphs, bigGlyphs} {
		wantH := [2]int{5, 7}[i]
		for r, s := range src {
			g := parseGlyph(s)
			if int(g.h) != wantH {
				t.Errorf("font %d %q has %d rows, want %d", i, r, g.h, wantH)
			}
			rowW := 0
			for j := 0; j <= len(s); j++ {
				if j == len(s) || s[j] == ' ' {
					if rowW != int(g.w) {
						t.Errorf("font %d %q has a row %d wide, glyph is %d", i, r, rowW, g.w)
					}
					rowW = 0
					continue
				}
				rowW++
			}
		}
	}
}

func TestArenaSurvivesRoundsAndOffGridCrash(t *testing.T) {
	a := NewArena()
	s := sim.NewMatch(1, 6)
	f := NewFrame(sim.W, sim.H)
	var evs []sim.Event
	for i := 0; i < 400; i++ {
		evs = s.Step(nil, evs[:0])
		a.Events(s, evs)
		a.Draw(f, 0, 0, s, 0.5, float64(i)/20)
	}
	a.Events(s, []sim.Event{{Kind: sim.EvCrash, Player: 0, X: -1, Y: sim.H}})
	a.Draw(f, 0, 0, s, 0, 21)
}
