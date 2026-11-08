package doomruntime

import (
	"fmt"
	"testing"

	"gddoom/internal/mapdata"
)

func failedWalkFixture(special uint16, boom bool) *game {
	return &game{m: &mapdata.Map{
		Linedefs: []mapdata.Linedef{{Special: special, Tag: 7}},
		Sectors:  []mapdata.Sector{{Tag: 7, CeilingHeight: 128}},
	}, lineSpecial: []uint16{special}, sectorFloor: []int64{0}, sectorCeil: []int64{128 * fracUnit},
		boomMomentumDropoff: boom,
		lines: []physLine{{idx: 0, x1: 0, y1: 64 * fracUnit, x2: 0, y2: -64 * fracUnit,
			dy: -128 * fracUnit, slope: slopeVertical, bbox: [4]int64{64 * fracUnit, -64 * fracUnit, 0, 0}}}}
}

func TestBoomFailedLegacyWalkOnceRetainsTrigger(t *testing.T) {
	for _, special := range []uint16{36, 109, 10} {
		for _, boom := range []bool{false, true} {
			t.Run(fmt.Sprintf("special=%d/Boom=%v", special, boom), func(t *testing.T) {
				g := failedWalkFixture(special, boom)
				switch special {
				case 36:
					g.floors = map[int]*floorThinker{0: {direction: -1}}
				case 109:
					g.doors = map[int]*doorThinker{0: {sector: 0, direction: 1}}
				case 10:
					g.plats = map[int]*platThinker{0: {status: platStatusWaiting}}
				}
				g.checkWalkSpecialLinesForActorWithCandidates(-8*fracUnit, 0, 8*fracUnit, 0, -1, true, []int{0})
				want := uint16(0)
				if boom {
					want = special
				}
				if g.lineSpecial[0] != want {
					t.Fatalf("busy W1 special = %d, want %d", g.lineSpecial[0], want)
				}
			})
		}
	}
}

func TestBoomHandledEmptyLegacyWalkOnceStillConsumesTrigger(t *testing.T) {
	for _, special := range []uint16{12, 13, 17, 35, 54, 104} {
		t.Run(fmt.Sprint(special), func(t *testing.T) {
			g := failedWalkFixture(special, true)
			g.m.Sectors[0].Tag = 8 // No matching target; the event still reports success.
			g.checkWalkSpecialLinesForActorWithCandidates(-8*fracUnit, 0, 8*fracUnit, 0, -1, true, []int{0})
			if g.lineSpecial[0] != 0 {
				t.Fatal("handled lighting/platform-stop event retained its W1 trigger")
			}
		})
	}
}

func TestWalkExitsRetainLineWhileSwitchExitsConsumeIt(t *testing.T) {
	for _, special := range []uint16{52, 124, 11, 51, 197, 198} {
		t.Run(fmt.Sprint(special), func(t *testing.T) {
			g := failedWalkFixture(special, true)
			info := mapdata.LookupLineSpecial(special)
			if !g.handleExitSpecial(0, special, info.Trigger) || !g.levelExitRequested {
				t.Fatal("exit was not queued")
			}
			want := uint16(0)
			if info.Trigger == mapdata.TriggerWalk {
				want = special
			}
			if g.lineSpecial[0] != want {
				t.Fatalf("exit special = %d, want %d", g.lineSpecial[0], want)
			}
		})
	}
}
