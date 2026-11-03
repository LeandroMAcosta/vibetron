package ui

import (
	"context"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"estela/internal/net"
	"estela/internal/render"
	"estela/internal/sim"
)

type menuItem uint8

const (
	itemSolo menuItem = iota
	itemHost
	itemJoin
	itemName
	itemQuit
	itemRoom // rooms found on the network follow the fixed items
)

var menuLabels = [...]string{"JUGAR SOLO", "CREAR SALA", "UNIRSE", "NOMBRE", "SALIR"}

type menu struct {
	sel     int
	rooms   []net.Room
	roomsCh <-chan []net.Room
	stop    context.CancelFunc
	editing bool
	err     string
}

func newMenu() *menu {
	ctx, cancel := context.WithCancel(context.Background())
	return &menu{roomsCh: net.Browse(ctx), stop: cancel}
}

func menuWithError(msg string) *menu {
	m := newMenu()
	m.err = msg
	return m
}

func (m *menu) count() int { return int(itemRoom) + len(m.rooms) }

func (m *menu) item() menuItem {
	if m.sel >= int(itemRoom) {
		return itemRoom
	}
	return menuItem(m.sel)
}

func (m *menu) Key(a *App, ev *tcell.EventKey) Scene {
	if m.editing {
		m.editName(a, ev)
		return m
	}
	switch ev.Key() {
	case tcell.KeyUp:
		m.sel = (m.sel + m.count() - 1) % m.count()
	case tcell.KeyDown, tcell.KeyTab:
		m.sel = (m.sel + 1) % m.count()
	case tcell.KeyEnter:
		return m.choose(a)
	case tcell.KeyEscape:
		a.quit = true
	case tcell.KeyRune:
		switch unicode.ToLower(ev.Rune()) {
		case 'q':
			a.quit = true
		case 'w', 'k':
			m.sel = (m.sel + m.count() - 1) % m.count()
		case 's', 'j':
			m.sel = (m.sel + 1) % m.count()
		}
	}
	return m
}

func (m *menu) choose(a *App) Scene {
	m.err = ""
	switch m.item() {
	case itemSolo:
		m.stop()
		return a.hostRoom(3, true)
	case itemHost:
		m.stop()
		return a.hostRoom(0, false)
	case itemJoin:
		m.stop()
		return newJoin("")
	case itemName:
		m.editing = true
	case itemQuit:
		a.quit = true
	case itemRoom:
		m.stop()
		return newJoin(m.rooms[m.sel-int(itemRoom)].Code)
	}
	return m
}

func (m *menu) editName(a *App, ev *tcell.EventKey) {
	name := []rune(a.Opts.Name)
	switch ev.Key() {
	case tcell.KeyEnter, tcell.KeyEscape:
		if strings.TrimSpace(a.Opts.Name) == "" {
			a.Opts.Name = "JUGADOR"
		}
		m.editing = false
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(name) > 0 {
			a.Opts.Name = string(name[:len(name)-1])
		}
	case tcell.KeyRune:
		r := unicode.ToUpper(ev.Rune())
		if len(name) < net.MaxName && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-') {
			a.Opts.Name += string(r)
		}
	}
}

func (m *menu) Update(a *App) Scene {
	select {
	case rooms := <-m.roomsCh:
		m.rooms = rooms
		if m.sel >= m.count() {
			m.sel = m.count() - 1
		}
	default:
	}
	return m
}

func (m *menu) Draw(a *App, f *render.Frame) {
	render.Backdrop(f, a.now)
	top := (f.H - 60) / 2
	render.Title(f, f.W/2, top+3, a.now)
	y := top + 24
	for i := 0; i < m.count(); i++ {
		label := m.label(a, i)
		c := dim
		if i == m.sel {
			c = pulse(render.Players[i%sim.MaxPlayers].Core, a.now)
			label = "> " + label + " <"
		}
		centerText(f, y, label, c, false)
		y += 6
	}
	hint := "ENTER: ELEGIR   Q: SALIR"
	if m.editing {
		hint = "ESCRIBÍ TU NOMBRE"
	}
	if m.err != "" {
		centerText(f, f.H-12, strings.ToUpper(m.err), danger, false)
	}
	centerText(f, f.H-6, hint, dim, false)
}

func (m *menu) label(a *App, i int) string {
	if i >= int(itemRoom) {
		r := m.rooms[i-int(itemRoom)]
		return "SALA DE " + strings.ToUpper(r.Name) + " " + r.Players
	}
	if menuItem(i) == itemName {
		name := a.Opts.Name
		if m.editing && int(a.now*2)%2 == 0 {
			name += "_"
		}
		return "NOMBRE: " + strings.ToUpper(name)
	}
	return menuLabels[i]
}
