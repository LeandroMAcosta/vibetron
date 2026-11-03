package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"github.com/LeandroMAcosta/vibetron/internal/bot"
	"github.com/LeandroMAcosta/vibetron/internal/net"
	"github.com/LeandroMAcosta/vibetron/internal/render"
	"github.com/LeandroMAcosta/vibetron/internal/sim"
)

const (
	hashEvery   = 20
	feedSeconds = 3.0
)

type notice struct {
	text string
	col  render.RGB
	at   float64
}

type match struct {
	state    *sim.State
	arena    *render.Arena
	names    []string
	evs      []sim.Event
	feed     []notice
	lastTick float64
	pilot    *bot.Brain // drives this seat in demo mode
}

func newMatch(a *App, start net.Msg) *match {
	m := &match{
		state: sim.NewMatch(start.Seed, uint8(len(start.Roster)), start.Cfg),
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

func (m *match) minSize() (int, int) { return arenaTerminal(m.state.Cfg) }

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
				l.cfg = msg.Cfg
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
		m.feed = m.feed[:0]
	}
	m.arena.Events(m.state, m.evs)
	for _, e := range m.evs {
		if text := m.announce(e); text != "" {
			col := white
			if e.Kind == sim.EvCrash {
				col = render.Players[e.Player].Core
			}
			m.feed = append(m.feed, notice{text, col, a.now})
		}
	}
	if len(m.feed) > 2 {
		m.feed = m.feed[len(m.feed)-2:]
	}
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

// announce words an event for the feed under the arena; short, because the
// smallest arena is only 60 pixels wide.
func (m *match) announce(e sim.Event) string {
	switch e.Kind {
	case sim.EvSudden:
		return "¡MUERTE SÚBITA!"
	case sim.EvCrash:
		who := m.names[e.Player]
		switch e.Cause {
		case sim.HitWall:
			return who + " SE ESTRELLÓ"
		case sim.HitSelf:
			return who + " SE CHOCÓ SOLO"
		case sim.HitTrail:
			return m.names[e.By] + " CERRÓ A " + who
		case sim.HitHeadOn:
			if e.Player < e.By {
				return who + " Y " + m.names[e.By] + " DE FRENTE"
			}
		case sim.HitQuit:
			return who + " SE FUE"
		}
	}
	return ""
}

func (m *match) Draw(a *App, f *render.Frame) {
	s := m.state
	render.Backdrop(f, a.now)
	ox := (f.W - int(s.W)) / 2
	oy := (f.H - int(s.H) - render.HUDHeight) / 2 &^ 1
	render.DrawHUD(f, ox, oy, int(s.W), s, m.names)
	alpha := (a.now - m.lastTick) * float64(s.Rate)
	if alpha > 1 {
		alpha = 1
	}
	top := oy + render.HUDHeight
	m.arena.Draw(f, ox, top, s, alpha, a.now)
	m.drawOverlay(a, f, top)
	m.drawFeed(a, f, top+int(s.H))
}

func (m *match) drawFeed(a *App, f *render.Frame, bottom int) {
	y := bottom - 7
	for i := len(m.feed) - 1; i >= 0; i-- {
		n := m.feed[i]
		if a.now-n.at > feedSeconds {
			continue
		}
		y -= centerText(f, y, n.text, n.col, false)
	}
}

func (m *match) drawOverlay(a *App, f *render.Frame, top int) {
	s := m.state
	h := int(s.H)
	mid := top + h/2
	big := max(1, h/16)
	switch s.Phase {
	case sim.Countdown:
		n := int(s.PhaseTicks)/int(s.Rate) + 1
		frac := float64(int(s.PhaseTicks)%int(s.Rate)) / float64(s.Rate)
		centerText(f, mid-h/4-4, fmt.Sprintf("RONDA %d", s.Round), white, true)
		centerScaled(f, mid-3*big, fmt.Sprint(n), pulse(accent, a.now), big+int(frac*float64(big)))
		if s.Round == 1 && h >= 50 {
			centerText(f, mid+14, "FLECHAS: GIRAR", white, false)
			centerText(f, mid+20, "ESPACIO: IMPULSO   J: SALTO", white, false)
		}
	case sim.Running:
		if !s.Players[a.client.You()].Alive {
			centerText(f, top+2, "ELIMINADO", dim, false)
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
			centerScaled(f, mid-4-3*big, m.names[s.Winner], pulse(c, a.now), big+1)
		}
		centerText(f, mid+6, "GANA LA PARTIDA", c, true)
		centerText(f, mid+15, "VOLVIENDO A LA SALA...", dim, false)
	}
}
