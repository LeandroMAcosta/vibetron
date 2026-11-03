package net

import (
	"bufio"
	"errors"
	"fmt"
	stdnet "net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"estela/internal/sim"
)

// Brain picks a bot's action for the next tick from the host's own state.
type Brain interface {
	Think(s *sim.State) (sim.Action, bool)
}

type HostConfig struct {
	TickInterval time.Duration // 0 follows the speed the host picked
	LobbyDelay   time.Duration // pause on the final screen before the lobby; default 4s
	NewBrain     func(id uint8, seed uint64) Brain
	Port         int // 0 tries BasePort..BasePort+7
}

const (
	handshakeTimeout = 5 * time.Second
	writeTimeout     = 5 * time.Second
	peerBuffer       = 512
)

var ErrHostClosed = errors.New("La sala está cerrada")

// Host owns the room. All room state lives in the run goroutine; every
// other goroutine talks to it by posting closures on cmds.
type Host struct {
	cfg     HostConfig
	ln      stdnet.Listener
	port    uint16
	code    string
	cmds    chan func(*room)
	done    chan struct{}
	once    sync.Once
	wg      sync.WaitGroup
	desyncs atomic.Int64
	lobby   atomic.Pointer[lobbyInfo] // nil during a match; read by the beacon
}

type lobbyInfo struct{ humans, seats int }

func NewHost(cfg HostConfig) (*Host, error) {
	if cfg.LobbyDelay <= 0 {
		cfg.LobbyDelay = 4 * time.Second
	}
	ports := []int{cfg.Port}
	if cfg.Port == 0 {
		ports = ports[:0]
		for off := 0; off < portSlots; off++ {
			ports = append(ports, BasePort+off)
		}
	} else if cfg.Port < BasePort || cfg.Port >= BasePort+portSlots {
		return nil, ErrPort
	}
	var ln stdnet.Listener
	var err error
	for _, p := range ports {
		if ln, err = stdnet.Listen("tcp", fmt.Sprintf(":%d", p)); err == nil {
			cfg.Port = p
			break
		}
	}
	if ln == nil {
		return nil, fmt.Errorf("No hay puertos libres (%d-%d): %w", ports[0], ports[len(ports)-1], err)
	}
	ip, err := LANAddr()
	if err != nil {
		ip = netip.AddrFrom4([4]byte{127, 0, 0, 1})
	}
	h := &Host{cfg: cfg, ln: ln, port: uint16(cfg.Port), cmds: make(chan func(*room)), done: make(chan struct{})}
	h.code, _ = Encode(netip.AddrPortFrom(ip, h.port))
	r := &room{h: h, cfg: sim.DefaultConfig}
	r.publish()
	h.wg.Add(3)
	go h.run(r)
	go h.accept()
	go h.beacon()
	return h, nil
}

func (h *Host) Code() string { return h.code }

func (h *Host) LoopbackAddr() netip.AddrPort {
	return netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), h.port)
}

func (h *Host) SetBots(n int) { h.post(func(r *room) { r.setBots(n) }) }

// SetConfig picks the arena size and speed for the next match.
func (h *Host) SetConfig(c sim.Config) { h.post(func(r *room) { r.setConfig(c) }) }

func (h *Host) Config() sim.Config {
	c, _ := call(h, func(r *room) sim.Config { return r.cfg })
	return c
}

func (h *Host) Bots() int {
	n, _ := call(h, func(r *room) int { return r.effectiveBots() })
	return n
}

func (h *Host) Start() error {
	err, ok := call(h, func(r *room) error { return r.start() })
	if !ok {
		return ErrHostClosed
	}
	return err
}

// Desyncs counts hash mismatches reported by clients in the current match.
func (h *Host) Desyncs() int { return int(h.desyncs.Load()) }

func (h *Host) Close() {
	h.once.Do(func() {
		close(h.done)
		h.ln.Close()
	})
	h.wg.Wait()
}

// post queues f for the room goroutine; false once the host is closed.
func (h *Host) post(f func(*room)) bool {
	select {
	case h.cmds <- f:
		return true
	case <-h.done:
		return false
	}
}

func call[T any](h *Host, f func(*room) T) (T, bool) {
	reply := make(chan T, 1)
	var zero T
	if !h.post(func(r *room) { reply <- f(r) }) {
		return zero, false
	}
	select {
	case v := <-reply:
		return v, true
	case <-h.done:
		return zero, false
	}
}

func (h *Host) run(r *room) {
	defer h.wg.Done()
	for {
		var tick, back <-chan time.Time
		if r.ticker != nil {
			tick = r.ticker.C
		}
		if r.backTimer != nil {
			back = r.backTimer.C
		}
		select {
		case f := <-h.cmds:
			f(r)
		case <-tick:
			r.step()
		case <-back:
			r.toLobby()
		case <-h.done:
			r.shutdown()
			return
		}
	}
}

func (h *Host) accept() {
	defer h.wg.Done()
	for {
		c, err := h.ln.Accept()
		if err != nil {
			select {
			case <-h.done:
				return
			default:
				time.Sleep(10 * time.Millisecond)
				continue
			}
		}
		go h.serve(c)
	}
}

func (h *Host) serve(c stdnet.Conn) {
	if tc, ok := c.(*stdnet.TCPConn); ok {
		tc.SetNoDelay(true)
	}
	br, bw := bufio.NewReader(c), bufio.NewWriter(c)
	c.SetReadDeadline(time.Now().Add(handshakeTimeout))
	var m Msg
	if err := ReadMsg(br, &m); err != nil || m.Type != MsgHello {
		c.Close()
		return
	}
	p := &peer{out: make(chan *Msg, peerBuffer), name: cleanName(m.Name)}
	go p.write(c, bw)
	if m.Version != Version {
		p.out <- &Msg{Type: MsgReject, Reason: "Versión distinta"}
		close(p.out)
		return
	}
	c.SetReadDeadline(time.Time{})
	if !h.post(func(r *room) { r.join(p) }) {
		close(p.out)
		return
	}
	for {
		var m Msg
		if err := ReadMsg(br, &m); err != nil {
			break
		}
		if m.Type == MsgInput {
			a := m.Action
			if !h.post(func(r *room) { r.input(p, a) }) {
				return
			}
			continue
		}
		if m.Type == MsgHash {
			tick, sum := m.Tick, m.Hash
			if !h.post(func(r *room) { r.hash(p, tick, sum) }) {
				return
			}
			continue
		}
		break
	}
	h.post(func(r *room) { r.leave(p) })
}

// peer is one connected human. Only the room goroutine touches its fields;
// the writer goroutine only drains out.
type peer struct {
	out    chan *Msg
	name   string
	queue  []sim.Action
	gone   bool
	closed bool
}

// write flushes whenever the queue drains, so a tick batch costs one
// syscall. After a write error it keeps draining until the room closes out,
// which keeps the room's non-blocking sends from ever stalling.
func (p *peer) write(c stdnet.Conn, w *bufio.Writer) {
	for m := range p.out {
		c.SetWriteDeadline(time.Now().Add(writeTimeout))
		if WriteMsg(w, m) != nil {
			break
		}
		if len(p.out) == 0 && w.Flush() != nil {
			break
		}
	}
	c.Close()
	for range p.out {
	}
}

func cleanName(s string) string {
	s = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == unicode.ReplacementChar || r == '|' {
			return -1
		}
		return r
	}, s))
	if rs := []rune(s); len(rs) > MaxName {
		s = strings.TrimSpace(string(rs[:MaxName]))
	}
	return s
}
