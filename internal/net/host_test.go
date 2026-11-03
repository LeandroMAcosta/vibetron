package net

import (
	"bufio"
	"math/rand/v2"
	stdnet "net"
	"testing"
	"time"

	"estela/internal/sim"
)

type randomBrain struct{ rng *rand.Rand }

func (b *randomBrain) Think(*sim.State) (sim.Action, bool) {
	switch b.rng.IntN(10) {
	case 0:
		return sim.TurnLeft, true
	case 1:
		return sim.TurnRight, true
	}
	return 0, false
}

func newBrain(id uint8, seed uint64) Brain {
	return &randomBrain{rand.New(rand.NewPCG(seed, uint64(id)))}
}

func newTestHost(t *testing.T, bots int) *Host {
	t.Helper()
	h, err := NewHost(HostConfig{TickInterval: time.Millisecond, LobbyDelay: time.Second, NewBrain: newBrain})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.SetBots(bots)
	return h
}

func dial(t *testing.T, h *Host, name string) *Client {
	t.Helper()
	c, err := Dial(h.LoopbackAddr(), name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

// await reads c until a message of type want arrives.
func await(t *testing.T, c *Client, want MsgType, accept func(Msg) bool) Msg {
	t.Helper()
	timeout := time.After(30 * time.Second)
	for {
		select {
		case m := <-c.Msgs():
			if m.Type == want && (accept == nil || accept(m)) {
				return m
			}
			if m.Type == MsgBye {
				t.Fatalf("disconnected while waiting for %d", want)
			}
		case <-timeout:
			t.Fatalf("timed out waiting for %d", want)
		}
	}
}

func seats(n int) func(Msg) bool { return func(m Msg) bool { return len(m.Roster) == n } }

// play runs a client's own sim from Start until MatchOver, sending hashes
// and a few inputs like a UI would.
func play(c *Client, quitAt uint32) (*sim.State, int, error) {
	var s *sim.State
	hashes := 0
	rng := rand.New(rand.NewPCG(uint64(c.You()), 7))
	for m := range c.Msgs() {
		switch m.Type {
		case MsgStart:
			s = sim.NewMatch(m.Seed, uint8(len(m.Roster)), m.Cfg)
		case MsgTick:
			s.Step(m.Inputs, nil)
			if s.Tick != m.Tick {
				return s, hashes, errTick
			}
			if s.Tick%hashEvery == 0 {
				c.SendHash(s.Tick, s.Hash())
				hashes++
			}
			if quitAt != 0 && s.Tick == quitAt {
				c.Close()
				return s, hashes, nil
			}
			if s.Phase == sim.MatchOver {
				return s, hashes, nil
			}
			if rng.IntN(15) == 0 {
				c.Send(sim.Action(rng.IntN(int(sim.Jump) + 1)))
			}
		}
	}
	return s, hashes, errBye
}

var (
	errTick = stdErr("tick out of order")
	errBye  = stdErr("disconnected mid-match")
)

type stdErr string

func (e stdErr) Error() string { return string(e) }

type played struct {
	s      *sim.State
	hashes int
	err    error
}

func TestLockstepMatch(t *testing.T) {
	h := newTestHost(t, 3)
	clients := []*Client{dial(t, h, "Ana"), dial(t, h, "Beto")}
	for _, c := range clients {
		await(t, c, MsgRoster, seats(5))
	}
	if h.Bots() != 3 || clients[0].You() != 0 || clients[1].You() != 1 {
		t.Fatalf("bots=%d you=%d,%d", h.Bots(), clients[0].You(), clients[1].You())
	}
	want := sim.Config{Size: 0, Speed: 3}
	h.SetConfig(want)
	for _, c := range clients {
		if m := await(t, c, MsgRoster, seats(5)); m.Cfg != want {
			t.Fatalf("roster config = %+v, want %+v", m.Cfg, want)
		}
	}
	if h.Config() != want {
		t.Fatalf("host config = %+v", h.Config())
	}
	if err := h.Start(); err != nil {
		t.Fatal(err)
	}
	results := make(chan played, len(clients))
	for _, c := range clients {
		go func() {
			s, n, err := play(c, 0)
			results <- played{s, n, err}
		}()
	}
	var final []played
	for range clients {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		final = append(final, r)
	}
	wantHashes := final[0].hashes + final[1].hashes
	var hostHash uint64
	var hostTick uint32
	deadline := time.Now().Add(10 * time.Second)
	for {
		got, _ := call(h, func(r *room) int {
			if r.match == nil {
				return -1
			}
			hostHash, hostTick = r.match.state.Hash(), r.match.state.Tick
			return r.match.got
		})
		if got == wantHashes {
			break
		}
		if got < 0 || time.Now().After(deadline) {
			t.Fatalf("host checked %d of %d hashes before returning to lobby", got, wantHashes)
		}
		time.Sleep(time.Millisecond)
	}
	for i, r := range final {
		if r.s.Tick != hostTick || r.s.Hash() != hostHash {
			t.Errorf("client %d ended at tick %d hash %x; host tick %d hash %x", i, r.s.Tick, r.s.Hash(), hostTick, hostHash)
		}
	}
	t.Logf("host tick %d, %d hashes checked", hostTick, wantHashes)
	if d := h.Desyncs(); d != 0 {
		t.Fatalf("Desyncs() = %d", d)
	}
	for _, c := range clients {
		await(t, c, MsgRoster, seats(5))
	}
}

func TestJoinDuringMatchIsRejected(t *testing.T) {
	h := newTestHost(t, 1)
	c := dial(t, h, "Ana")
	await(t, c, MsgRoster, seats(2))
	if err := h.Start(); err != nil {
		t.Fatal(err)
	}
	await(t, c, MsgStart, nil)
	if _, err := Dial(h.LoopbackAddr(), "Tarde"); err == nil || err.Error() != "Partida en curso" {
		t.Fatalf("Dial during match err = %v", err)
	}
}

func TestDisconnectMidMatchLeavesPlayerDead(t *testing.T) {
	h := newTestHost(t, 0)
	stay, quit := dial(t, h, "Ana"), dial(t, h, "Beto")
	await(t, stay, MsgRoster, seats(2))
	await(t, quit, MsgRoster, seats(2))
	if err := h.Start(); err != nil {
		t.Fatal(err)
	}
	go play(quit, 70)
	s, _, err := play(stay, 0)
	if err != nil {
		t.Fatal(err)
	}
	if p := s.Players[1]; !p.Left || p.Alive {
		t.Fatalf("quitter left=%v alive=%v", p.Left, p.Alive)
	}
	await(t, stay, MsgRoster, seats(1))
}

func TestLobbyDepartureShiftsSeats(t *testing.T) {
	h := newTestHost(t, 0)
	a, b := dial(t, h, "Ana"), dial(t, h, "Beto")
	await(t, b, MsgRoster, seats(2))
	a.Close()
	m := await(t, b, MsgRoster, seats(1))
	if b.You() != 0 || m.Roster[0].Name != "Beto" {
		t.Fatalf("you=%d roster=%v", b.You(), m.Roster)
	}
	if err := h.Start(); err != ErrFewPlayers {
		t.Fatalf("Start with one seat err = %v", err)
	}
}

func TestRejects(t *testing.T) {
	h := newTestHost(t, 0)
	for i := range sim.MaxPlayers {
		dial(t, h, "P"+string(rune('0'+i)))
	}
	if _, err := Dial(h.LoopbackAddr(), "Siete"); err == nil || err.Error() != "Sala llena" {
		t.Fatalf("seventh Dial err = %v", err)
	}
	conn, err := stdnet.Dial("tcp", h.LoopbackAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	w := bufio.NewWriter(conn)
	WriteMsg(w, &Msg{Type: MsgHello, Version: Version + 1, Name: "Viejo"})
	w.Flush()
	var m Msg
	if err := ReadMsg(bufio.NewReader(conn), &m); err != nil || m.Type != MsgReject || m.Reason != "Versión distinta" {
		t.Fatalf("version mismatch reply = %+v, %v", m, err)
	}
}

func TestNameIsCleanedAndTruncated(t *testing.T) {
	h := newTestHost(t, 1)
	c := dial(t, h, "  Ñandú\x07 con un nombre larguísimo  ")
	m := await(t, c, MsgRoster, seats(2))
	if got := m.Roster[0].Name; got != "Ñandú con" {
		t.Fatalf("name = %q", got)
	}
}
