package demo

import (
	"bytes"
	"testing"
)

func boomDemoBytes() []byte {
	b := make([]byte, 109)
	b[0] = VersionBoom202
	copy(b[1:7], []byte{0x1d, 'B', 'o', 'o', 'm', 0xe6})
	b[8] = 3
	b[9] = 1
	b[10] = 1
	b[77] = 1
	b[21] = 1
	return append(b, 50, 0, 128, ButtonUse, Marker)
}

func TestBoomDemoHeaderAndCommands(t *testing.T) {
	for _, version := range []byte{VersionBoom201, VersionBoom202} {
		b := boomDemoBytes()
		b[0] = version
		s, err := Parse(b)
		if err != nil {
			t.Fatal(err)
		}
		if s.Header.Version != version || s.Header.Skill != 3 || !s.Header.NoMonsters || s.Header.Map != 1 || !s.Header.PlayerInGame[0] || len(s.Tics) != 1 || s.Tics[0].Forward != 50 || s.Tics[0].AngleTurn != -32768 || s.Tics[0].Buttons != ButtonUse {
			t.Fatalf("incorrect Boom header/command: %+v", s)
		}
	}
}

func TestBoomDemoRejectsMalformedAndIncompleteFrames(t *testing.T) {
	for _, edit := range []func([]byte) []byte{
		func(b []byte) []byte { return b[:108] },
		func(b []byte) []byte { b[1] = 0; return b },
		func(b []byte) []byte { b[78] = 1; return b },
		func(b []byte) []byte { b[81] = 1; return b },
		func(b []byte) []byte { b[77] = 0; return b },
		func(b []byte) []byte { return b[:len(b)-1] },
	} {
		if _, err := Parse(edit(boomDemoBytes())); err == nil {
			t.Fatal("invalid Boom stream accepted")
		}
	}
}

func TestBoomCoopCommandsAreGroupedByWorldTic(t *testing.T) {
	b := boomDemoBytes()[:109]
	b[77], b[78], b[80], b[12] = 1, 1, 1, 1
	b = append(b, 10, 1, 2, 3, 20, 4, 5, 6, 30, 7, 8, 9, 11, 2, 3, 4, 21, 5, 6, 7, 31, 8, 9, 10, Marker)
	s, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Frames) != 2 || len(s.Tics) != 2 || s.Tics[1].Forward != 21 || s.Frames[0][0].Forward != 10 || s.Frames[0][3].Forward != 30 || s.Frames[0][2] != (Tic{}) {
		t.Fatalf("incorrect sparse frame grouping: %+v", s)
	}
	roundtrip, err := Format(s)
	if err != nil || !bytes.Equal(roundtrip, b) {
		t.Fatalf("multiplayer format lost frames: %v", err)
	}
	for _, end := range []int{113, 117, 118, 120} {
		if _, err := Parse(append(append([]byte{}, b[:end]...), Marker)); err == nil {
			t.Fatal("accepted partial multiplayer frame")
		}
	}
}
