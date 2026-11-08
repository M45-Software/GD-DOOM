package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestBoomHeightClipNonSolidMoversBesideOverlappingPlayer(t *testing.T) {
	for _, tc := range []struct {
		name  string
		boom  bool
		typ   int16
		dead  bool
		phase int
		flags uint16
		ceil  int64
	}{
		{"vanilla corpse", false, 66, true, 5, mlTwoSided, 128},
		{"Boom solid death frame", true, 66, true, 1, mlTwoSided, 128},
		{"Boom non-solid corpse", true, 66, true, 5, mlTwoSided, 64},
		{"Boom corpse at blocking line", true, 66, true, 5, mlTwoSided | mlBlockMonsters, 128},
		{"vanilla pickup", false, 2046, false, 0, mlTwoSided, 128},
		{"Boom non-solid pickup", true, 2046, false, 0, mlTwoSided, 64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &game{m: &mapdata.Map{
				Sectors:  []mapdata.Sector{{CeilingHeight: 128}, {CeilingHeight: 64}},
				Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}},
				Things:   []mapdata.Thing{{Type: tc.typ, X: 64, Y: 32, Flags: skillMask}},
			}, opts: Options{SkillLevel: 3}, stats: playerStats{Health: 100}, playerMobjHealth: 100,
				thingHP: []int{-6}, thingDead: []bool{tc.dead}, thingCollected: []bool{false},
				boomMomentumDropoff: tc.boom}
			g.initPhysics()
			g.ensureMonsterAIState()
			g.p = player{x: 64 * fracUnit, y: 32 * fracUnit, ceilz: 128 * fracUnit}
			g.thingState[0], g.thingStatePhase[0] = monsterStateDeath, tc.phase
			g.setThingSupportState(0, 0, 0, 128*fracUnit)
			g.lines = []physLine{{idx: 0, x1: 80 * fracUnit, y1: 64 * fracUnit,
				x2: 80 * fracUnit, y2: -64 * fracUnit, dy: -128 * fracUnit,
				slope: slopeVertical, sideNum0: 0, sideNum1: 1, flags: tc.flags,
				bbox: [4]int64{64 * fracUnit, -64 * fracUnit, 80 * fracUnit, 80 * fracUnit}}}
			if !g.heightClipThing(0, g.m.Things[0]) {
				t.Fatal("corpse should fit the opening")
			}
			if _, _, ceil := g.thingSupportState(0, g.m.Things[0]); ceil != tc.ceil*fracUnit {
				t.Fatalf("corpse ceiling = %d, want %d", ceil, tc.ceil*fracUnit)
			}
		})
	}
}
