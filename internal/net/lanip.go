package net

import (
	"errors"
	stdnet "net"
	"net/netip"
)

var ErrNoLAN = errors.New("No se encontró una red local")

// LANAddr returns the IPv4 of the default-route interface. Connecting a UDP
// socket sends nothing; it only makes the kernel pick a source address,
// which skips VPN and container bridges that may come first in Interfaces.
func LANAddr() (netip.Addr, error) {
	if c, err := stdnet.Dial("udp4", "8.8.8.8:80"); err == nil {
		defer c.Close()
		if ap, err := netip.ParseAddrPort(c.LocalAddr().String()); err == nil {
			if ip := ap.Addr().Unmap(); ip.Is4() && !ip.IsLoopback() && !ip.IsUnspecified() {
				return ip, nil
			}
		}
	}
	ifaces, err := stdnet.Interfaces()
	if err != nil {
		return netip.Addr{}, ErrNoLAN
	}
	for _, iface := range ifaces {
		if iface.Flags&stdnet.FlagUp == 0 || iface.Flags&stdnet.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			pfx, err := netip.ParsePrefix(a.String())
			if err != nil {
				continue
			}
			if ip := pfx.Addr().Unmap(); ip.Is4() && ip.IsPrivate() {
				return ip, nil
			}
		}
	}
	return netip.Addr{}, ErrNoLAN
}
