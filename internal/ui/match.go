package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"estela/internal/bot"
	"estela/internal/net"
	"estela/internal/render"
	"estela/internal/sim"
)

const (
	hashEvery    = 20
	tickSeconds  = 1.0 / sim.TickRate
	layoutHeight = sim.H + render.HUDHeight
)

type match struct {
	state    *sim.State
	arena    *render.Arena
	names    []string
	evs      []sim.Event
	lastTick float64
	pilot    *bot.Brain // drives this seat in demo mode
}

func newMatch(a *App, start net.Msg) *match {
	m := &match{
		state: sim.NewMatch(start.Seed, uint8(len(start.Roster))),
		arena: render.NewArena(),
		evs:   make([]sim.Event, 0, 64),
	}
	for _, s := range start.Roster {
		m.names = append(m.names, strings.ToUpper(s.Name))
	}
	if a.Opts.Demo {
		m.pilot = bot.New(a.client.You(), start.Seed)
	}
	m.lastTick = a.now
	return m
}

var keyActions = map[tcell.Key]sim.Action{
	tcell.KeyUp: sim.FaceUp, tcell.KeyRight: sim.FaceRight,
	tcell.KeyDown: sim.FaceDown, tcell.KeyLeft: sim.FaceLeft,
	tcell.KeyEnter: sim.Jump,
}

var runeActions = map[rune]sim.Action{
	'w': sim.FaceUp, 'd': sim.FaceRight, 's': sim.FaceDown, 'a': sim.FaceLeft,
	' ': sim.Boost, 'j': sim.Jump, 'x': sim.Jump,
}

func (m *match) Key(a *App, ev *tcell.EventKey) Scene {
	if ev.Key() == tcell.KeyEscape || (ev.Key() == tcell.KeyRune && unicode.ToLower(ev.Rune()) == 'q') {
		a.client.Send(sim.Quit)
		a.leave()
		return newMenu()
	}
	act, ok := keyActions[ev.Key()]
	if ev.Key() == tcell.KeyRune {
		act, ok = runeActions[unicode.ToLower(ev.Rune())]
	}
	if ok && m.pilot == nil {
		a.client.Send(act)
	}
	return m
}

func (m *match) Update(a *App) Scene {
	for {
		select {
		case msg := <-a.client.Msgs():
			switch msg.Type {
			case net.MsgTick:
				m.step(a, msg)
			case net.MsgRoster:
				l := newLobby()
				l.roster = append(l.roster, msg.Roster...)
				l.autoStart = a.Opts.Demo
				return l
			case net.MsgBye:
				a.leave()
				return menuWithError("Se cortó la conexión con la sala")
			}
		default:
			return m
		}
	}
}

func (m *match) step(a *App, msg net.Msg) {
	if msg.Tick != m.state.Tick+1 {
		return
	}
	wasRound := m.state.Round
	m.evs = m.state.Step(msg.Inputs, m.evs[:0])
	if m.state.Round != wasRound {
		m.arena.Reset()
	}
	m.arena.Events(m.state, m.evs)
	m.lastTick = a.now
	if m.state.Tick%hashEvery == 0 {
		a.client.SendHash(m.state.Tick, m.state.Hash())
	}
	if m.pilot != nil {
		if act, ok := m.pilot.Think(m.state); ok {
			a.client.Send(act)
		}
	}
}

func (m *match) Draw(a *App, f *render.Frame) {
	render.Backdrop(f, a.now)
	ox := (f.W - sim.W) / 2
	oy := (f.H - layoutHeight) / 2 &^ 1
	render.DrawHUD(f, ox, oy, sim.W, m.state, m.names)
	alpha := (a.now - m.lastTick) / tickSeconds
	if alpha > 1 {
		alpha = 1
	}
	m.arena.Draw(f, ox, oy+render.HUDHeight, m.state, alpha, a.now)
	m.drawOverlay(a, f, oy+render.HUDHeight)
}

func (m *match) drawOverlay(a *App, f *render.Frame, top int) {
	s := m.state
	mid := top + sim.H/2
	switch s.Phase {
	case sim.Countdown:
		n := int(s.PhaseTicks)/sim.TickRate + 1
		frac := float64(int(s.PhaseTicks)%sim.TickRate) / sim.TickRate
		centerText(f, mid-16, fmt.Sprintf("RONDA %d", s.Round), white, true)
		centerScaled(f, mid-6, fmt.Sprint(n), pulse(accent, a.now), 2+int(frac*3))
		if s.Round == 1 {
			centerText(f, mid+16, "FLECHAS: GIRAR   ESPACIO: IMPULSO   J: SALTO", white, false)
		}
	case sim.Running:
		if !s.Players[a.client.You()].Alive {
			centerText(f, top+sim.H-7, "ELIMINADO", dim, false)
		}
	case sim.RoundOver:
		if s.Winner < 0 {
			centerScaled(f, mid-5, "¡EMPATE!", white, 2)
		} else {
			centerScaled(f, mid-5, "¡GANA "+m.names[s.Winner]+"!", render.Players[s.Winner].Core, 2)
		}
	case sim.MatchOver:
		c := white
		if s.Winner >= 0 {
			c = render.Players[s.Winner].Core
			centerScaled(f, mid-12, m.names[s.Winner], pulse(c, a.now), 3)
		}
		centerText(f, mid+10, "GANA LA PARTIDA", c, true)
		centerText(f, mid+20, "VOLVIENDO A LA SALA...", dim, false)
	}
}
