package doomruntime

import (
	"fmt"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestOrdinaryTeleportDoesNotRequireDestinationHeadroom(t *testing.T) {
	original := doomrand.Capture()
	t.Cleanup(func() { doomrand.Restore(original) })
	for _, boom := range []bool{false, true} {
		for _, isPlayer := range []bool{false, true} {
			for _, clearance := range []int64{0, 52 * fracUnit} {
				t.Run(fmt.Sprintf("boom=%t/player=%t/clearance=%d", boom, isPlayer, clearance/fracUnit), func(t *testing.T) {
					g := newDoorTimingGame(0)
					g.boomMomentumDropoff = boom
					g.m.Linedefs[0].Special, g.m.Linedefs[0].Tag = 97, 7
					g.m.Sectors[0].Tag, g.m.Sectors[1].Tag = 7, 0
					g.sectorFloor[0] = 36 * fracUnit
					g.sectorCeil[0] = g.sectorFloor[0] + clearance
					g.m.Things = []mapdata.Thing{{X: -128, Angle: 90, Type: 14}, {X: 256, Y: 128, Type: 64}}
					g.p.x, g.p.y = 1000*fracUnit, 1000*fracUnit
					g.stats.Health = 100
					if !g.activateTeleportLine(0, 0, *mapdata.LookupLineSpecial(97).Teleport, 1, isPlayer) {
						t.Fatal("teleport rejected a valid destination subsector because of its height")
					}
					x, y, z := g.p.x, g.p.y, g.p.z
					if !isPlayer {
						x, y = g.thingPosFixed(1, g.m.Things[1])
						z, _, _ = g.thingSupportState(1, g.m.Things[1])
					}
					if x != -128*fracUnit || y != 0 || z != 36*fracUnit || len(g.hitscanPuffs) != 2 {
						t.Fatalf("destination=(%d,%d,%d) fog=%d", x, y, z, len(g.hitscanPuffs))
					}
				})
			}
		}
	}
}
