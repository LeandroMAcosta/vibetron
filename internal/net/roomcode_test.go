package net

import (
	"math/rand/v2"
	"net/netip"
	"testing"
)

func TestRoomCodeRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	ips := []netip.Addr{netip.MustParseAddr("0.0.0.0"), netip.MustParseAddr("255.255.255.255")}
	for range 500 {
		var b [4]byte
		for i := range b {
			b[i] = byte(rng.Uint32())
		}
		ips = append(ips, netip.AddrFrom4(b))
	}
	for _, ip := range ips {
		for off := range uint16(portSlots) {
			want := netip.AddrPortFrom(ip, BasePort+off)
			code, err := Encode(want)
			if err != nil {
				t.Fatal(err)
			}
			if len(code) != 8 || code[3] != '-' {
				t.Fatalf("code %q is not XXX-XXXX", code)
			}
			got, err := Decode(code)
			if err != nil || got != want {
				t.Fatalf("Decode(%q) = %v, %v; want %v", code, got, err, want)
			}
		}
	}
}

func TestDecodeIsForgiving(t *testing.T) {
	want := netip.MustParseAddrPort("192.168.1.10:47473")
	code, _ := Encode(want)
	lower := []byte(code)
	for i := range lower {
		if lower[i] >= 'A' && lower[i] <= 'Z' {
			lower[i] += 'a' - 'A'
		}
	}
	for _, in := range []string{code, string(lower), " " + code[:3] + " " + code[4:] + " ", code[:3] + code[4:], "-" + code[:2] + "-" + code[2:]} {
		if got, err := Decode(in); err != nil || got != want {
			t.Errorf("Decode(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	ref, _ := Decode("1A0-B1C0")
	for _, alias := range []string{"IA0-B1C0", "lAO-BIco", "La0-bLCO"} {
		if got, err := Decode(alias); err != nil || got != ref {
			t.Errorf("Decode(%q) = %v, %v; want %v", alias, got, err, ref)
		}
	}
	for _, bad := range []string{"", "ABC-DEF", "ABC-DEFGH", "ABC-DEFU", "ABC*DEFG"} {
		if _, err := Decode(bad); err == nil || err.Error() != "Código inválido" {
			t.Errorf("Decode(%q) err = %v, want Código inválido", bad, err)
		}
	}
}

func TestEncodeRejectsOutOfRange(t *testing.T) {
	for _, ap := range []string{"10.0.0.1:47469", "10.0.0.1:47478", "[::1]:47470"} {
		if _, err := Encode(netip.MustParseAddrPort(ap)); err == nil {
			t.Errorf("Encode(%s) succeeded", ap)
		}
	}
}

func TestParseBeacon(t *testing.T) {
	code, _ := Encode(netip.MustParseAddrPort("10.0.0.7:47471"))
	r, ok := parseBeacon("VIBETRON1|" + code + "|mac\x01book|2/5")
	if !ok || r.Code != code || r.Name != "macbook" || r.Players != "2/5" {
		t.Fatalf("parseBeacon = %+v, %v", r, ok)
	}
	for _, bad := range []string{"VIBETRON2|" + code + "|x|1/2", "VIBETRON1|nope|x|1/2", "VIBETRON1|" + code + "|x|3/2", "VIBETRON1|" + code + "|x"} {
		if _, ok := parseBeacon(bad); ok {
			t.Errorf("parseBeacon(%q) accepted", bad)
		}
	}
}
