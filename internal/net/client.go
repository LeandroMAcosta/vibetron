package net

import (
	"bufio"
	"errors"
	stdnet "net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"estela/internal/sim"
)

const dialTimeout = 3 * time.Second

var (
	ErrDial      = errors.New("No se pudo conectar a la sala")
	ErrHandshake = errors.New("La sala respondió algo inesperado")
)

// Client is one player's connection to a Host.
type Client struct {
	conn stdnet.Conn
	mu   sync.Mutex
	w    *bufio.Writer
	you  atomic.Uint32
	msgs chan Msg
	done chan struct{}
	once sync.Once
}

func DialCode(code, name string) (*Client, error) {
	addr, err := Decode(code)
	if err != nil {
		return nil, err
	}
	return Dial(addr, name)
}

// Dial joins the room at addr. A Reject comes back as an error carrying the
// host's reason.
func Dial(addr netip.AddrPort, name string) (*Client, error) {
	conn, err := stdnet.DialTimeout("tcp", addr.String(), dialTimeout)
	if err != nil {
		return nil, ErrDial
	}
	if tc, ok := conn.(*stdnet.TCPConn); ok {
		tc.SetNoDelay(true)
	}
	c := &Client{conn: conn, w: bufio.NewWriter(conn), msgs: make(chan Msg, 256), done: make(chan struct{})}
	br := bufio.NewReader(conn)
	conn.SetDeadline(time.Now().Add(handshakeTimeout))
	var m Msg
	err = WriteMsg(c.w, &Msg{Type: MsgHello, Version: Version, Name: name})
	if err == nil {
		err = c.w.Flush()
	}
	if err == nil {
		err = ReadMsg(br, &m)
	}
	switch {
	case err != nil:
		conn.Close()
		return nil, ErrDial
	case m.Type == MsgReject:
		conn.Close()
		return nil, errors.New(cleanReason(m.Reason))
	case m.Type != MsgWelcome:
		conn.Close()
		return nil, ErrHandshake
	}
	conn.SetDeadline(time.Time{})
	c.you.Store(uint32(m.You))
	go c.read(br)
	return c, nil
}

// You is this client's seat; it can change in the lobby when someone
// before it leaves, and is always current by the time a Roster arrives.
func (c *Client) You() uint8 { return uint8(c.you.Load()) }

// Msgs delivers Roster, Start and Tick in order, then one MsgBye and closes.
func (c *Client) Msgs() <-chan Msg { return c.msgs }

func (c *Client) Send(a sim.Action) { c.write(&Msg{Type: MsgInput, Action: a}) }

func (c *Client) SendHash(tick uint32, h uint64) {
	c.write(&Msg{Type: MsgHash, Tick: tick, Hash: h})
}

func (c *Client) Close() {
	c.once.Do(func() {
		c.write(&Msg{Type: MsgBye})
		close(c.done)
		c.conn.Close()
	})
}

// write errors are dropped on purpose: a dead link surfaces as MsgBye from
// the reader, which is the one place the UI watches for it.
func (c *Client) write(m *Msg) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if WriteMsg(c.w, m) == nil {
		c.w.Flush()
	}
}

func (c *Client) read(br *bufio.Reader) {
	defer close(c.msgs)
	seats := 0
	for {
		var m Msg
		if ReadMsg(br, &m) != nil || !validFromHost(&m, &seats) {
			break
		}
		if m.Type == MsgWelcome {
			c.you.Store(uint32(m.You))
			continue
		}
		select {
		case c.msgs <- m:
		case <-c.done:
			return
		}
	}
	c.conn.Close()
	select {
	case c.msgs <- Msg{Type: MsgBye}:
	case <-c.done:
	}
}

// validFromHost rejects anything a well-behaved host never sends, so the UI
// can index the roster with any Tick input it receives.
func validFromHost(m *Msg, seats *int) bool {
	switch m.Type {
	case MsgWelcome:
		return int(m.You) < sim.MaxPlayers
	case MsgRoster:
		for i := range m.Roster {
			m.Roster[i].Name = cleanName(m.Roster[i].Name)
		}
		return true
	case MsgStart:
		for i := range m.Roster {
			m.Roster[i].Name = cleanName(m.Roster[i].Name)
		}
		*seats = len(m.Roster)
		return *seats > 0
	case MsgTick:
		for _, in := range m.Inputs {
			if int(in.Player) >= *seats || in.Action > sim.Quit {
				return false
			}
		}
		return true
	}
	return false
}

func cleanReason(s string) string {
	s = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s))
	if s == "" {
		return "La sala rechazó la conexión"
	}
	return s
}
