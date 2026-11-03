package ui

import (
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"github.com/LeandroMAcosta/vibetron/internal/net"
	"github.com/LeandroMAcosta/vibetron/internal/render"
)

type dialResult struct {
	c   *net.Client
	err error
}

type join struct {
	code    string
	auto    bool
	pending chan dialResult
	err     string
}

// newJoin starts dialing right away when the code came from a flag or the
// rooms list, so the player never retypes a code they already chose.
func newJoin(code string) *join {
	return &join{code: strings.ToUpper(code), auto: code != ""}
}

func (j *join) dial(a *App) {
	j.err = ""
	j.pending = make(chan dialResult, 1)
	code, name := j.code, a.Opts.Name
	go func() {
		c, err := net.DialCode(code, name)
		j.pending <- dialResult{c, err}
	}()
}

func (j *join) Key(a *App, ev *tcell.EventKey) Scene {
	if j.pending != nil {
		return j
	}
	switch ev.Key() {
	case tcell.KeyEscape:
		return newMenu()
	case tcell.KeyEnter:
		j.dial(a)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(j.code) > 0 {
			j.code = j.code[:len(j.code)-1]
		}
	case tcell.KeyRune:
		r := unicode.ToUpper(ev.Rune())
		if len(j.code) < 8 && (r == '-' || (r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)))) {
			j.code += string(r)
		}
	}
	return j
}

func (j *join) Update(a *App) Scene {
	if j.auto {
		j.auto = false
		j.dial(a)
	}
	if j.pending == nil {
		return j
	}
	select {
	case r := <-j.pending:
		j.pending = nil
		if r.err != nil {
			j.err = r.err.Error()
			return j
		}
		a.code = j.code
		return a.joinRoom(r.c)
	default:
	}
	return j
}

func (j *join) Draw(a *App, f *render.Frame) {
	render.Backdrop(f, a.now)
	top := (f.H - 60) / 2
	render.Title(f, f.W/2, top+3, a.now)
	centerText(f, top+24, "CÓDIGO DE LA SALA", white, false)
	code := j.code
	if j.pending == nil && int(a.now*2)%2 == 0 {
		code += "_"
	}
	if code == "" {
		code = " "
	}
	centerScaled(f, top+32, code, pulse(accent, a.now), 2)
	switch {
	case j.pending != nil:
		centerText(f, top+50, "CONECTANDO...", white, false)
	case j.err != "":
		centerText(f, top+50, strings.ToUpper(j.err), danger, false)
	}
	centerText(f, f.H-6, "ENTER: UNIRSE   ESC: VOLVER", dim, false)
}
