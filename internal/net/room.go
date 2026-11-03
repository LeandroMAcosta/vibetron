package net

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/LeandroMAcosta/vibetron/internal/sim"
)

const (
	maxQueued  = 4
	hashEvery  = 20
	minPlayers = 2
)

var (
	ErrInMatch    = errors.New("La partida ya empezó")
	ErrFewPlayers = errors.New("Se necesitan al menos 2 jugadores")
)

// room is the host's state, owned by Host.run. peers holds humans in join
// order; their index is their seat, and bots take the seats after them.
type room struct {
	h         *Host
	peers     []*peer
	bots      int
	cfg       sim.Config
	match     *match
	ticker    *time.Ticker
	backTimer *time.Timer
}

type match struct {
	state  *sim.State
	seats  []*peer // nil for a bot seat
	brains []Brain
	inputs []sim.Input
	evs    []sim.Event
	hashes map[uint32]uint64
	got    int // hashes received, for tests
}

var botNames = [...]string{"Neón", "Chispa", "Rayo", "Brasa", "Cometa", "Pulsar"}

func (r *room) effectiveBots() int { return min(r.bots, sim.MaxPlayers-len(r.peers)) }

func (r *room) roster() []Seat {
	seats := make([]Seat, 0, sim.MaxPlayers)
	for _, p := range r.peers {
		seats = append(seats, Seat{Name: p.name})
	}
	for i := range r.effectiveBots() {
		seats = append(seats, Seat{Name: botNames[i%len(botNames)], Bot: true})
	}
	return seats
}

func (r *room) publish() {
	if r.match != nil {
		r.h.lobby.Store(nil)
		return
	}
	r.h.lobby.Store(&lobbyInfo{humans: len(r.peers), seats: len(r.peers) + r.effectiveBots()})
}

// broadcastRoster resends each peer its seat first, since a lobby departure
// shifts everyone after it down by one.
func (r *room) broadcastRoster() {
	rs := &Msg{Type: MsgRoster, Roster: r.roster(), Cfg: r.cfg}
	for i, p := range r.peers {
		r.send(p, &Msg{Type: MsgWelcome, You: uint8(i)})
		r.send(p, rs)
	}
	r.publish()
}

func (r *room) send(p *peer, m *Msg) {
	if p.closed {
		return
	}
	select {
	case p.out <- m:
	default:
		r.hangup(p)
	}
}

// hangup closes the peer's queue; its writer then closes the conn and its
// reader reports leave. Safe to call more than once.
func (r *room) hangup(p *peer) {
	if !p.closed {
		p.closed = true
		close(p.out)
	}
}

func (r *room) reject(p *peer, reason string) {
	r.send(p, &Msg{Type: MsgReject, Reason: reason})
	r.hangup(p)
}

func (r *room) join(p *peer) {
	switch {
	case r.match != nil:
		r.reject(p, "Partida en curso")
		return
	case len(r.peers) >= sim.MaxPlayers:
		r.reject(p, "Sala llena")
		return
	}
	if p.name == "" {
		p.name = fmt.Sprintf("Jugador %d", len(r.peers)+1)
	}
	r.peers = append(r.peers, p)
	r.broadcastRoster()
}

func (r *room) leave(p *peer) {
	r.hangup(p)
	if r.match != nil {
		p.gone = true
		return
	}
	for i, q := range r.peers {
		if q == p {
			r.peers = append(r.peers[:i], r.peers[i+1:]...)
			r.broadcastRoster()
			return
		}
	}
}

func (r *room) setBots(n int) {
	if r.match != nil {
		return
	}
	r.bots = max(0, min(n, sim.MaxPlayers))
	r.broadcastRoster()
}

func (r *room) setConfig(c sim.Config) {
	if r.match != nil || !c.Valid() {
		return
	}
	r.cfg = c
	r.broadcastRoster()
}

func (r *room) start() error {
	if r.match != nil {
		return ErrInMatch
	}
	roster := r.roster()
	if len(roster) < minPlayers {
		return ErrFewPlayers
	}
	var b [8]byte
	rand.Read(b[:])
	seed := binary.LittleEndian.Uint64(b[:])
	m := &match{
		state:  sim.NewMatch(seed, uint8(len(roster)), r.cfg),
		seats:  make([]*peer, len(roster)),
		brains: make([]Brain, len(roster)),
		hashes: map[uint32]uint64{},
	}
	copy(m.seats, r.peers)
	for id := len(r.peers); id < len(roster); id++ {
		if r.h.cfg.NewBrain != nil {
			m.brains[id] = r.h.cfg.NewBrain(uint8(id), seed)
		}
	}
	for _, p := range r.peers {
		p.queue = p.queue[:0]
	}
	r.match = m
	r.h.desyncs.Store(0)
	start := &Msg{Type: MsgStart, Seed: seed, Roster: roster, Cfg: r.cfg}
	for _, p := range r.peers {
		r.send(p, start)
	}
	every := r.h.cfg.TickInterval
	if every <= 0 {
		every = time.Second / time.Duration(r.cfg.Rate())
	}
	r.ticker = time.NewTicker(every)
	r.publish()
	return nil
}

func (r *room) input(p *peer, a sim.Action) {
	if a > sim.Quit {
		r.hangup(p)
		return
	}
	if r.match == nil || r.backTimer != nil || p.closed || len(p.queue) >= maxQueued {
		return
	}
	p.queue = append(p.queue, a)
}

func (r *room) hash(p *peer, tick uint32, sum uint64) {
	if r.match == nil || p.closed {
		return
	}
	want, ok := r.match.hashes[tick]
	if !ok {
		return
	}
	r.match.got++
	if want != sum {
		r.h.desyncs.Add(1)
	}
}

func (r *room) step() {
	m := r.match
	in := m.inputs[:0]
	for id, p := range m.seats {
		player := uint8(id)
		switch {
		case p == nil:
			if b := m.brains[id]; b != nil {
				if a, ok := b.Think(m.state); ok {
					in = append(in, sim.Input{Player: player, Action: a})
				}
			}
		case p.gone:
			if !m.state.Players[id].Left {
				in = append(in, sim.Input{Player: player, Action: sim.Quit})
			}
		default:
			for _, a := range p.queue {
				in = append(in, sim.Input{Player: player, Action: a})
			}
			p.queue = p.queue[:0]
		}
	}
	m.inputs = in
	m.evs = m.state.Step(in, m.evs[:0])
	n := m.state.Tick
	tick := &Msg{Type: MsgTick, Tick: n, Inputs: append([]sim.Input(nil), in...)}
	for _, p := range r.peers {
		r.send(p, tick)
	}
	if n%hashEvery == 0 {
		m.hashes[n] = m.state.Hash()
	}
	if m.state.Phase == sim.MatchOver {
		r.ticker.Stop()
		r.ticker = nil
		r.backTimer = time.NewTimer(r.h.cfg.LobbyDelay)
	}
}

func (r *room) toLobby() {
	r.backTimer = nil
	r.match = nil
	kept := r.peers[:0]
	for _, p := range r.peers {
		if !p.gone {
			kept = append(kept, p)
		}
	}
	clear(r.peers[len(kept):])
	r.peers = kept
	r.broadcastRoster()
}

func (r *room) shutdown() {
	if r.ticker != nil {
		r.ticker.Stop()
	}
	if r.backTimer != nil {
		r.backTimer.Stop()
	}
	for _, p := range r.peers {
		r.hangup(p)
	}
	r.h.lobby.Store(nil)
}
