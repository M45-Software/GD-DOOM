package doomruntime

import (
	"fmt"
	"testing"

	"gddoom/internal/mapdata"
)

func TestBoomBlockmapDelimiterDoesNotCreatePhantomSlideWall(t *testing.T) {
	for _, boom := range []bool{false, true} {
		for _, realZero := range []bool{false, true} {
			t.Run(fmt.Sprintf("Boom=%v/real-line-zero=%v", boom, realZero), func(t *testing.T) {
				cell := []int16{0}
				if realZero {
					cell = append(cell, 0)
				}
				g := &game{m: &mapdata.Map{
					BlockMap: &mapdata.BlockMap{Cells: [][]int16{cell}},
					Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}},
					Sectors:  []mapdata.Sector{{CeilingHeight: 128}, {CeilingHeight: 128}},
				}, boomMomentumDropoff: boom, bmapWidth: 1, bmapHeight: 1,
					bmapOriginX: -64 * fracUnit, bmapOriginY: 960 * fracUnit,
					physForLine: []int{0}, sectorFloor: []int64{0, 0}, sectorCeil: []int64{128 * fracUnit, 128 * fracUnit},
					p: player{x: -8 * fracUnit, y: 1000 * fracUnit, z: -128 * fracUnit},
					lines: []physLine{{idx: 0, x1: 0, y1: 0, x2: 0, y2: 16 * fracUnit,
						dy: 16 * fracUnit, slope: slopeVertical, flags: mlTwoSided, sideNum0: 0, sideNum1: 1}}}
				// The short trace crosses the extension of distant linedef zero.
				// Only the vanilla delimiter bug, or an explicit real entry, can
				// make that line a slide candidate in this block.
				_, _, blocked := g.firstBlockingIntercept(-8*fracUnit, 1000*fracUnit, 8*fracUnit, 1000*fracUnit)
				if want := !boom || realZero; blocked != want {
					t.Fatalf("phantom slide wall = %v, want %v", blocked, want)
				}
			})
		}
	}
}
