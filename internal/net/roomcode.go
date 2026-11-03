package net

import (
	"errors"
	"net/netip"
	"strings"
)

// crockford is Crockford's base32 alphabet: no I, L, O or U, so a code read
// aloud or typed from a screen has no look-alike letters.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

const (
	portSlots = 8
	codeLen   = 7
)

var (
	ErrCode = errors.New("Código inválido")
	ErrPort = errors.New("Puerto fuera de rango")
	ErrIPv4 = errors.New("Se necesita una dirección IPv4")
)

// Encode packs an IPv4 address and a port in BasePort..BasePort+7 into 35
// bits, shown as seven Crockford characters "XXX-XXXX".
func Encode(addr netip.AddrPort) (string, error) {
	ip := addr.Addr().Unmap()
	if !ip.Is4() {
		return "", ErrIPv4
	}
	off := int(addr.Port()) - BasePort
	if off < 0 || off >= portSlots {
		return "", ErrPort
	}
	b := ip.As4()
	v := uint64(b[0])<<24 | uint64(b[1])<<16 | uint64(b[2])<<8 | uint64(b[3])
	v = v<<3 | uint64(off)
	var out [codeLen + 1]byte
	for i := codeLen - 1; i >= 0; i-- {
		out[i] = crockford[v&31]
		v >>= 5
	}
	return string(out[:3]) + "-" + string(out[3:codeLen]), nil
}

// Decode accepts what people actually type: any case, spaces or dashes
// anywhere, and I/L/O in place of 1/1/0.
func Decode(code string) (netip.AddrPort, error) {
	var v uint64
	n := 0
	for _, r := range strings.ToUpper(code) {
		switch r {
		case ' ', '-', '\t':
			continue
		case 'I', 'L':
			r = '1'
		case 'O':
			r = '0'
		}
		d := strings.IndexRune(crockford, r)
		if d < 0 || n == codeLen {
			return netip.AddrPort{}, ErrCode
		}
		v = v<<5 | uint64(d)
		n++
	}
	if n != codeLen {
		return netip.AddrPort{}, ErrCode
	}
	off := uint16(v & 7)
	v >>= 3
	ip := netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
	return netip.AddrPortFrom(ip, BasePort+off), nil
}
