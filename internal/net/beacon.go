package net

import (
	"context"
	"fmt"
	stdnet "net"
	"os"
	"slices"
	"strings"
	"time"

	"estela/internal/sim"
)

const (
	beaconPort   = BasePort + 100
	beaconMagic  = "ESTELA1"
	beaconEvery  = time.Second
	roomLifetime = 3 * time.Second
)

// Room is one open lobby heard on the local network.
type Room struct {
	Code, Name string
	Players    string // "humans/seats"
	Seen       time.Time
}

// beacon announces the lobby by UDP broadcast. Many Wi-Fi networks drop
// broadcast, so every error is ignored: the room code works without it.
func (h *Host) beacon() {
	defer h.wg.Done()
	name := beaconName()
	lc := stdnet.ListenConfig{Control: broadcastControl}
	pc, err := lc.ListenPacket(context.Background(), "udp4", ":0")
	if err != nil {
		return
	}
	defer pc.Close()
	dst := &stdnet.UDPAddr{IP: stdnet.IPv4bcast, Port: beaconPort}
	t := time.NewTicker(beaconEvery)
	defer t.Stop()
	for {
		if info := h.lobby.Load(); info != nil {
			msg := fmt.Sprintf("%s|%s|%s|%d/%d", beaconMagic, h.code, name, info.humans, info.seats)
			pc.WriteTo([]byte(msg), dst)
		}
		select {
		case <-t.C:
		case <-h.done:
			return
		}
	}
}

func beaconName() string {
	name, _ := os.Hostname()
	name, _, _ = strings.Cut(name, ".")
	if name = cleanName(name); name == "" {
		name = "Sala"
	}
	return name
}

// Browse lists rooms heard in the last few seconds, about once a second,
// until ctx ends. If the port cannot be bound it still emits empty lists.
func Browse(ctx context.Context) <-chan []Room {
	out := make(chan []Room, 1)
	heard := make(chan Room, 16)
	lc := stdnet.ListenConfig{Control: reuseControl}
	pc, err := lc.ListenPacket(ctx, "udp4", fmt.Sprintf(":%d", beaconPort))
	if err == nil {
		go listenBeacons(pc, heard)
		go func() {
			<-ctx.Done()
			pc.Close()
		}()
	}
	go func() {
		defer close(out)
		rooms := map[string]Room{}
		t := time.NewTicker(beaconEvery)
		defer t.Stop()
		for {
			select {
			case r := <-heard:
				rooms[r.Code] = r
				continue
			case <-t.C:
			case <-ctx.Done():
				return
			}
			list := make([]Room, 0, len(rooms))
			for code, r := range rooms {
				if time.Since(r.Seen) > roomLifetime {
					delete(rooms, code)
					continue
				}
				list = append(list, r)
			}
			slices.SortFunc(list, func(a, b Room) int { return strings.Compare(a.Code, b.Code) })
			select {
			case out <- list:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func listenBeacons(pc stdnet.PacketConn, heard chan<- Room) {
	var buf [512]byte
	for {
		n, _, err := pc.ReadFrom(buf[:])
		if err != nil {
			return
		}
		if r, ok := parseBeacon(string(buf[:n])); ok {
			select {
			case heard <- r:
			default:
			}
		}
	}
}

func parseBeacon(s string) (Room, bool) {
	parts := strings.Split(s, "|")
	if len(parts) != 4 || parts[0] != beaconMagic {
		return Room{}, false
	}
	addr, err := Decode(parts[1])
	if err != nil {
		return Room{}, false
	}
	code, _ := Encode(addr)
	var humans, seats int
	if _, err := fmt.Sscanf(parts[3], "%d/%d", &humans, &seats); err != nil || humans < 0 || seats < humans || seats > sim.MaxPlayers {
		return Room{}, false
	}
	return Room{Code: code, Name: cleanName(parts[2]), Players: fmt.Sprintf("%d/%d", humans, seats), Seen: time.Now()}, true
}
