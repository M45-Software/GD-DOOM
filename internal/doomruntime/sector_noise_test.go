package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestSectorNoiseTopologyRetainsMovingOpeningsAndSoundBlocks(t *testing.T) {
	g := &game{m: &mapdata.Map{
		Sectors:  []mapdata.Sector{{CeilingHeight: 128}, {}, {CeilingHeight: 128}},
		Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}, {Sector: 1}, {Sector: 2}},
	}, sectorSoundTarget: make([]bool, 3)}
	g.initPhysics()
	g.lines = []physLine{{sideNum0: 0, sideNum1: 1, flags: mlTwoSided},
		{sideNum0: 2, sideNum1: 3, flags: mlTwoSided | lineSoundBlock}}
	check := func(want [3]bool) {
		t.Helper()
		clear(g.sectorSoundTarget)
		g.propagateSectorNoise(0, 0, []int{-1, -1, -1})
		for i, value := range want {
			if g.sectorSoundTarget[i] != value {
				t.Fatalf("sector %d noise = %v, want %v", i, g.sectorSoundTarget, want)
			}
		}
	}
	check([3]bool{true, false, false})
	g.sectorCeil[1] = 128 * fracUnit
	check([3]bool{true, true, true})
	g.lines[0].flags |= lineSoundBlock
	check([3]bool{true, true, false})
	g.sectorCeil[1] = 0
	check([3]bool{true, false, false})
}
