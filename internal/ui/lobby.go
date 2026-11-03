package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"estela/internal/net"
	"estela/internal/render"
	"estela/internal/sim"
)

type lobby struct {
	roster    []net.Seat
	cfg       sim.Config
	autoStart bool
	err       string
}

func newLobby() *lobby { return &lobby{cfg: sim.DefaultConfig} }

func (l *lobby) Key(a *App, ev *tcell.EventKey) Scene {
	switch ev.Key() {
	case tcell.KeyEscape:
		a.leave()
		return newMenu()
	case tcell.KeyEnter:
		l.start(a)
	case tcell.KeyRight:
		l.bots(a, 1)
	case tcell.KeyLeft:
		l.bots(a, -1)
	case tcell.KeyRune:
		switch unicode.ToLower(ev.Rune()) {
		case '+', '=':
			l.bots(a, 1)
		case '-', '_':
			l.bots(a, -1)
		case 't':
			l.configure(a, func(c *sim.Config) { c.Size = (c.Size + 1) % uint8(len(sim.Sizes)) })
		case 'v':
			l.configure(a, func(c *sim.Config) { c.Speed = (c.Speed + 1) % uint8(len(sim.Speeds)) })
		case 'q':
			a.leave()
			return newMenu()
		}
	}
	return l
}

func (l *lobby) bots(a *App, delta int) {
	if a.host != nil {
		a.host.SetBots(a.host.Bots() + delta)
	}
}

// configure changes the host's arena. It updates l.cfg right away so quick
// repeated presses build on each other instead of on a roster still in flight.
func (l *lobby) configure(a *App, change func(*sim.Config)) {
	if a.host == nil {
		return
	}
	change(&l.cfg)
	a.host.SetConfig(l.cfg)
}

func (l *lobby) start(a *App) {
	if a.host == nil {
		return
	}
	if err := a.host.Start(); err != nil {
		l.err = err.Error()
	}
}

func (l *lobby) Update(a *App) Scene {
	if l.autoStart && len(l.roster) >= 2 {
		l.autoStart = false
		l.start(a)
	}
	for {
		select {
		case m := <-a.client.Msgs():
			switch m.Type {
			case net.MsgRoster:
				l.roster = append(l.roster[:0], m.Roster...)
				l.cfg = m.Cfg
				l.err = ""
			case net.MsgStart:
				return newMatch(a, m)
			case net.MsgBye:
				a.leave()
				return menuWithError("Se cortó la conexión con la sala")
			}
		default:
			return l
		}
	}
}

func (l *lobby) Draw(a *App, f *render.Frame) {
	render.Backdrop(f, a.now)
	top := (f.H - 60) / 2
	if a.host != nil {
		centerText(f, top+2, "COMPARTÍ ESTE CÓDIGO", white, false)
	} else {
		centerText(f, top+2, "SALA", white, false)
	}
	if a.code != "" {
		centerScaled(f, top+9, a.code, pulse(accent, a.now), 2)
	}
	for i, s := range l.roster {
		pal := render.Players[i%sim.MaxPlayers]
		x, y := f.W/2-46+(i/3)*48, top+27+(i%3)*6
		c := pal.Mid
		if uint8(i) == a.client.You() {
			c = pulse(pal.Core, a.now)
		}
		for dy := 0; dy < 5; dy++ {
			for dx := 0; dx < 5; dx++ {
				if !s.Bot || dx == 0 || dy == 0 || dx == 4 || dy == 4 {
					f.Set(x+dx, y+dy, pal.Core)
				}
			}
		}
		render.DrawText(f, x+8, y, strings.ToUpper(s.Name), c, false)
	}
	size, speed := sim.Sizes[l.cfg.Size], sim.Speeds[l.cfg.Speed]
	cols, rows := arenaTerminal(l.cfg)
	if tc, tr := a.screen.Size(); tc < cols || tr < rows {
		centerText(f, f.H-18, fmt.Sprintf("AGRANDÁ A %dX%d PARA JUGAR", cols, rows), danger, false)
	} else {
		centerText(f, f.H-18, fmt.Sprintf("%s %dX%d - %s", strings.ToUpper(size.Name), size.W, size.H, strings.ToUpper(speed.Name)), accent, false)
	}
	switch {
	case l.err != "":
		centerText(f, f.H-12, strings.ToUpper(l.err), danger, false)
	case a.host != nil:
		centerText(f, f.H-12, "ENTER: JUGAR   +/-: BOTS", dim, false)
		centerText(f, f.H-6, "T: TAMAÑO   V: VELOCIDAD", dim, false)
		return
	default:
		centerText(f, f.H-12, "ESPERANDO QUE EMPIECE...", dim, false)
	}
	centerText(f, f.H-6, "ESC: SALIR", dim, false)
}

// arenaTerminal is the terminal size a match on cfg needs: one column per
// pixel and two pixel rows per terminal row, plus the HUD.
func arenaTerminal(c sim.Config) (cols, rows int) {
	return int(c.W()), (int(c.H()) + render.HUDHeight + 1) / 2
}
