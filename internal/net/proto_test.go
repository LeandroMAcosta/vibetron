package net

import (
	"bufio"
	"bytes"
	"reflect"
	"testing"

	"github.com/LeandroMAcosta/vibetron/internal/sim"
)

func TestMsgRoundTrip(t *testing.T) {
	msgs := []Msg{
		{Type: MsgHello, Version: Version, Name: "Lean"},
		{Type: MsgWelcome, You: 3},
		{Type: MsgReject, Reason: "sala llena"},
		{Type: MsgRoster, Roster: []Seat{{"Ana", false}, {"Bot 2", true}}},
		{Type: MsgStart, Seed: 0xDEADBEEF12345678, Cfg: sim.Config{Size: 3, Speed: 2}, Roster: []Seat{{"Ana", false}}},
		{Type: MsgTick, Tick: 9001, Inputs: []sim.Input{{Player: 0, Action: sim.Jump}, {Player: 5, Action: sim.FaceLeft}}},
		{Type: MsgHash, Tick: 40, Hash: 0xFFEEDDCCBBAA9988},
		{Type: MsgInput, Action: sim.Boost},
		{Type: MsgBye},
	}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	for i := range msgs {
		if err := WriteMsg(w, &msgs[i]); err != nil {
			t.Fatal(err)
		}
	}
	w.Flush()
	r := bufio.NewReader(&buf)
	for _, want := range msgs {
		var got Msg
		if err := ReadMsg(r, &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Inputs) == 0 {
			got.Inputs = nil
		}
		if len(got.Roster) == 0 {
			got.Roster = nil
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	}
}

func TestReadMsgRejectsTruncatedPayload(t *testing.T) {
	r := bufio.NewReader(bytes.NewReader([]byte{0, 3, byte(MsgHash), 0, 1}))
	var m Msg
	if err := ReadMsg(r, &m); err != ErrFrame {
		t.Fatalf("err = %v, want ErrFrame", err)
	}
}
