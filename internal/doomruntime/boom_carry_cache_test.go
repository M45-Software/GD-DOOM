package doomruntime

import "testing"

func TestBoomConveyorCachedCenterKeepsBoundaryTouches(t *testing.T) {
	g := boomTouchFixture(t)
	for _, x := range []int64{-256, -40, -20, -1, 0, 1, 20, 40, 256} {
		g.setThingPosFixed(1, x*fracUnit, 0)
		th := g.m.Things[1]
		for sector := range g.m.Sectors {
			for _, radius := range []int64{0, 20 * fracUnit} {
				// Compare against a fresh BSP query, including actors whose
				// center belongs to a different sector than their touching edge.
				want := g.boomActorTouchesSector(x*fracUnit, 0, radius, sector)
				got := g.boomActorTouchesSectorWithCenter(x*fracUnit, 0, radius, sector, g.thingSectorCached(1, th))
				if got != want {
					t.Fatalf("x=%d sector=%d radius=%d cached=%t fresh=%t", x, sector, radius, got, want)
				}
			}
			g.thingMomX[1], g.thingMomY[1] = 0, 0
			z, _, _ := g.thingSupportState(1, th)
			want := z <= g.sectorFloor[sector] && g.boomActorTouchesSector(x*fracUnit, 0, g.thingCurrentRadius(1, th), sector)
			g.tickBoomCarry(boomCarryThinker{sector: sector, vector: boomScrollVector{x: fracUnit}})
			if got := g.thingMomX[1] != 0; got != want {
				t.Fatalf("x=%d sector=%d carried=%t want=%t", x, sector, got, want)
			}
		}
	}
}
