// Package net carries the lockstep protocol: clients send actions, the host
// orders them into numbered ticks, and every peer steps the same sim.
package net

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"

	"estela/internal/sim"
)

const (
	Version  = 2
	BasePort = 47470
	MaxName  = 10
	maxFrame = 1 << 12
)

type MsgType uint8

const (
	MsgHello MsgType = iota + 1
	MsgWelcome
	MsgReject
	MsgRoster
	MsgStart
	MsgTick
	MsgHash
	MsgInput
	MsgBye
)

// Msg is a flat union: each MsgType uses the fields its comment names.
type Msg struct {
	Type    MsgType
	Name    string      // Hello
	Version uint8       // Hello
	You     uint8       // Welcome
	Reason  string      // Reject
	Roster  []Seat      // Roster, Start
	Seed    uint64      // Start
	Cfg     sim.Config  // Roster, Start
	Tick    uint32      // Tick, Hash
	Inputs  []sim.Input // Tick
	Hash    uint64      // Hash
	Action  sim.Action  // Input
}

// Seat is one player slot; its index is the sim player id.
type Seat struct {
	Name string
	Bot  bool
}

var ErrFrame = errors.New("net: malformed frame")

// WriteMsg encodes m as [u16 length][type][payload].
func WriteMsg(w *bufio.Writer, m *Msg) error {
	var b [maxFrame]byte
	p := append(b[:2], byte(m.Type))
	switch m.Type {
	case MsgHello:
		p = append(p, m.Version)
		p = putStr(p, m.Name)
	case MsgWelcome:
		p = append(p, m.You)
	case MsgReject:
		p = putStr(p, m.Reason)
	case MsgRoster, MsgStart:
		p = binary.BigEndian.AppendUint64(p, m.Seed)
		p = append(p, m.Cfg.Size, m.Cfg.Speed, byte(len(m.Roster)))
		for _, s := range m.Roster {
			p = putStr(p, s.Name)
			p = append(p, bool8(s.Bot))
		}
	case MsgTick:
		p = binary.BigEndian.AppendUint32(p, m.Tick)
		p = append(p, byte(len(m.Inputs)))
		for _, in := range m.Inputs {
			p = append(p, in.Player, byte(in.Action))
		}
	case MsgHash:
		p = binary.BigEndian.AppendUint32(p, m.Tick)
		p = binary.BigEndian.AppendUint64(p, m.Hash)
	case MsgInput:
		p = append(p, byte(m.Action))
	}
	binary.BigEndian.PutUint16(p, uint16(len(p)-2))
	_, err := w.Write(p)
	return err
}

// ReadMsg decodes one frame. It never trusts lengths from the wire.
func ReadMsg(r *bufio.Reader, m *Msg) error {
	var hdr [2]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return err
	}
	n := int(binary.BigEndian.Uint16(hdr[:]))
	if n < 1 || n > maxFrame {
		return ErrFrame
	}
	var b [maxFrame]byte
	if _, err := io.ReadFull(r, b[:n]); err != nil {
		return err
	}
	d := decoder{b: b[1:n]}
	*m = Msg{Type: MsgType(b[0]), Inputs: m.Inputs[:0], Roster: m.Roster[:0]}
	switch m.Type {
	case MsgHello:
		m.Version = d.u8()
		m.Name = d.str()
	case MsgWelcome:
		m.You = d.u8()
	case MsgReject:
		m.Reason = d.str()
	case MsgRoster, MsgStart:
		m.Seed = d.u64()
		m.Cfg = sim.Config{Size: d.u8(), Speed: d.u8()}
		if !d.bad && !m.Cfg.Valid() {
			return ErrFrame
		}
		count := int(d.u8())
		if count > sim.MaxPlayers {
			return ErrFrame
		}
		for i := 0; i < count; i++ {
			m.Roster = append(m.Roster, Seat{Name: d.str(), Bot: d.u8() == 1})
		}
	case MsgTick:
		m.Tick = d.u32()
		count := int(d.u8())
		for i := 0; i < count; i++ {
			m.Inputs = append(m.Inputs, sim.Input{Player: d.u8(), Action: sim.Action(d.u8())})
		}
	case MsgHash:
		m.Tick = d.u32()
		m.Hash = d.u64()
	case MsgInput:
		m.Action = sim.Action(d.u8())
	case MsgBye:
	default:
		return ErrFrame
	}
	if d.bad {
		return ErrFrame
	}
	return nil
}

func putStr(p []byte, s string) []byte {
	if len(s) > 255 {
		s = s[:255]
	}
	p = append(p, byte(len(s)))
	return append(p, s...)
}

func bool8(v bool) byte {
	if v {
		return 1
	}
	return 0
}

// decoder reads fixed fields and flags any read past the end instead of
// panicking, so one bad frame cannot crash a peer.
type decoder struct {
	b   []byte
	bad bool
}

func (d *decoder) take(n int) []byte {
	if d.bad || len(d.b) < n {
		d.bad = true
		return make([]byte, n)
	}
	v := d.b[:n]
	d.b = d.b[n:]
	return v
}

func (d *decoder) u8() uint8   { return d.take(1)[0] }
func (d *decoder) u32() uint32 { return binary.BigEndian.Uint32(d.take(4)) }
func (d *decoder) u64() uint64 { return binary.BigEndian.Uint64(d.take(8)) }
func (d *decoder) str() string { return string(d.take(int(d.u8()))) }
