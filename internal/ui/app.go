// Package ui runs the terminal loop and the scene state machine that joins
// input, the network client and the renderer.
package ui

import (
	"time"

	"github.com/gdamore/tcell/v2"

	"estela/internal/bot"
	"estela/internal/net"
	"estela/internal/render"
	"estela/internal/sim"
)

const (
	MinCols = sim.W
	MinRows = (sim.H + render.HUDHeight) / 2
	fps     = 60
)

// Scene is one screen of the game. Key and Update return the scene to show
// next, which is how every transition happens.
type Scene interface {
	Key(a *App, ev *tcell.EventKey) Scene
	Update(a *App) Scene
	Draw(a *App, f *render.Frame)
}

type Options struct {
	Name string
	Host bool
	Solo bool
	Demo bool
	Join string
}

type App struct {
	Opts   Options
	screen tcell.Screen
	frame  *render.Frame
	host   *net.Host
	client *net.Client
	code   string // room code to show in the lobby
	start  time.Time
	now    float64
	quit   bool
}

// Run owns the terminal until the player quits.
func Run(opts Options) error {
	s, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := s.Init(); err != nil {
		return err
	}
	defer s.Fini()
	s.HideCursor()
	a := &App{Opts: opts, screen: s, start: time.Now()}
	defer a.leave()
	a.loop(a.firstScene())
	return nil
}

func (a *App) firstScene() Scene {
	switch {
	case a.Opts.Join != "":
		return newJoin(a.Opts.Join)
	case a.Opts.Demo:
		return a.hostRoom(sim.MaxPlayers-1, true)
	case a.Opts.Solo:
		return a.hostRoom(3, true)
	case a.Opts.Host:
		return a.hostRoom(0, false)
	}
	return newMenu()
}

func (a *App) loop(scene Scene) {
	events := make(chan tcell.Event, 64)
	go func() {
		for {
			ev := a.screen.PollEvent()
			if ev == nil {
				return
			}
			events <- ev
		}
	}()
	tick := time.NewTicker(time.Second / fps)
	defer tick.Stop()
	for !a.quit {
		select {
		case ev := <-events:
			switch ev := ev.(type) {
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyCtrlC {
					return
				}
				scene = scene.Key(a, ev)
			case *tcell.EventResize:
				a.screen.Sync()
			}
		case <-tick.C:
			a.now = time.Since(a.start).Seconds()
			scene = scene.Update(a)
			a.draw(scene)
		}
	}
}

func (a *App) draw(scene Scene) {
	cols, rows := a.screen.Size()
	if a.frame == nil || a.frame.W != cols || a.frame.H != rows*2 {
		a.frame = render.NewFrame(cols, rows*2)
	}
	if cols < MinCols || rows < MinRows {
		drawTooSmall(a, a.frame, cols, rows)
	} else {
		scene.Draw(a, a.frame)
	}
	a.frame.Blit(a.screen, 0, 0)
	a.screen.Show()
}

// hostRoom starts a local host with bots and joins it over loopback, so the
// player at this keyboard goes through the same path as everyone else.
func (a *App) hostRoom(bots int, autoStart bool) Scene {
	a.leave()
	h, err := net.NewHost(net.HostConfig{NewBrain: func(id uint8, seed uint64) net.Brain {
		return bot.New(id, seed)
	}})
	if err != nil {
		return menuWithError("No se pudo crear la sala: " + err.Error())
	}
	h.SetBots(bots)
	c, err := net.Dial(h.LoopbackAddr(), a.Opts.Name)
	if err != nil {
		h.Close()
		return menuWithError("No se pudo crear la sala: " + err.Error())
	}
	a.host, a.client, a.code = h, c, h.Code()
	l := newLobby()
	l.autoStart = autoStart
	return l
}

func (a *App) joinRoom(c *net.Client) Scene {
	code := a.code
	a.leave()
	a.client, a.code = c, code
	return newLobby()
}

// leave tears down any connection; safe to call when there is none.
func (a *App) leave() {
	if a.client != nil {
		a.client.Close()
		a.client = nil
	}
	if a.host != nil {
		a.host.Close()
		a.host = nil
	}
}
