package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestBoomUsePassThroughActivatesSwitchesInOrder(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		boom, pass, locked, wall bool
		first, second            bool
	}{
		{"Boom pass", true, true, false, false, true, true},
		{"no flag", true, false, false, false, true, false},
		{"vanilla ignores flag", false, true, false, false, true, false},
		{"failed activation still passes", true, true, true, false, false, true},
		{"closed ordinary wall stops ray", true, true, false, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &mapdata.Map{
				Vertexes: []mapdata.Vertex{{X: 16, Y: 8}, {X: 16, Y: -8}, {X: 32, Y: 8}, {X: 32, Y: -8}},
				Sectors:  []mapdata.Sector{{Tag: 1}, {Tag: 2}, {CeilingHeight: 128}},
				Sidedefs: []mapdata.Sidedef{{Sector: 2}, {Sector: 2}},
				Linedefs: []mapdata.Linedef{
					{V1: 0, V2: 1, Special: 112, Tag: 1, SideNum: [2]int32{0, -1}},
					{V1: 2, V2: 3, Special: 112, Tag: 2, SideNum: [2]int32{1, -1}},
				},
			}
			if tc.pass {
				m.Linedefs[0].Flags = mlPassUse
			}
			if tc.locked {
				m.Linedefs[0].Special = 133
			}
			if tc.wall {
				m.Vertexes = append(m.Vertexes, mapdata.Vertex{X: 24, Y: 8}, mapdata.Vertex{X: 24, Y: -8})
				m.Linedefs = append(m.Linedefs, mapdata.Linedef{V1: 4, V2: 5, SideNum: [2]int32{0, -1}})
			}
			g := newGame(m, Options{Headless: true})
			g.p.x, g.p.y, g.p.angle = 0, 0, 0
			g.boomMomentumDropoff = tc.boom
			if line, result := g.peekUseTargetLine(); line != 0 || result != useTraceSpecial || len(g.doors) != 0 {
				t.Fatalf("preview line=%d result=%v doors=%v", line, result, g.doors)
			}
			g.useLines()
			if (g.activeDoorThinker(0) != nil) != tc.first || (g.activeDoorThinker(1) != nil) != tc.second {
				t.Fatalf("doors=%v, want activation first=%v second=%v", g.doors, tc.first, tc.second)
			}
		})
	}
}

func TestPassUseFlagEnablesBoomExceptInVanillaDemo(t *testing.T) {
	m := &mapdata.Map{Linedefs: []mapdata.Linedef{{Flags: mlPassUse}}}
	if !boomMomentumDropoffEnabled(m, Options{}) {
		t.Fatal("use-through flag alone did not enable Boom map behavior")
	}
	if boomMomentumDropoffEnabled(m, Options{DemoScript: &DemoScript{Header: DemoHeader{Version: 109}}}) {
		t.Fatal("use-through flag changed vanilla demo compatibility")
	}
}

func TestCollectUseLineInterceptsPreservesInsertionOrderOnTie(t *testing.T) {
	g := &game{
		lines: []physLine{
			{idx: 10, x1: 64 * fracUnit, y1: -8 * fracUnit, x2: 64 * fracUnit, y2: 8 * fracUnit, dx: 0, dy: 16 * fracUnit},
			{idx: 11, x1: 64 * fracUnit, y1: -8 * fracUnit, x2: 64 * fracUnit, y2: 8 * fracUnit, dx: 0, dy: 16 * fracUnit},
		},
	}
	intercepts := g.collectUseLineIntercepts(0, 0, 128*fracUnit, 0)
	if len(intercepts) != 2 {
		t.Fatalf("intercepts=%d want=2", len(intercepts))
	}
	if intercepts[0].line != 10 {
		t.Fatalf("first intercept line=%d want=10", intercepts[0].line)
	}
	if intercepts[1].line != 11 {
		t.Fatalf("second intercept line=%d want=11", intercepts[1].line)
	}
}

func TestPeekUseTargetLine_IgnoresInterceptsBeyondUseRangeLikeDoom(t *testing.T) {
	g := &game{
		lines:       []physLine{{idx: 0, x1: 66 * fracUnit, y1: -8 * fracUnit, x2: 66 * fracUnit, y2: 8 * fracUnit, dx: 0, dy: 16 * fracUnit, sideNum1: 1}},
		physForLine: []int{0},
		lineSpecial: []uint16{31},
		p:           player{angle: 0},
	}
	if got, tr := g.peekUseTargetLine(); got != -1 || tr != useTraceNone {
		t.Fatalf("peekUseTargetLine beyond range=(%d,%v) want (-1,%v)", got, tr, useTraceNone)
	}
}

func TestPeekUseTargetLine_UsesSpecialWithinUseRange(t *testing.T) {
	g := &game{
		lines:       []physLine{{idx: 0, x1: 63 * fracUnit, y1: -8 * fracUnit, x2: 63 * fracUnit, y2: 8 * fracUnit, dx: 0, dy: 16 * fracUnit, sideNum1: 1}},
		physForLine: []int{0},
		lineSpecial: []uint16{31},
		p:           player{angle: 0},
	}
	if got, tr := g.peekUseTargetLine(); got != 0 || tr != useTraceSpecial {
		t.Fatalf("peekUseTargetLine within range=(%d,%v) want (0,%v)", got, tr, useTraceSpecial)
	}
}
