package doomruntime

import (
	"fmt"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestBoomConveyorCarriesGroundBloodAndSubmergedEffects(t *testing.T) {
	for _, submerged := range []bool{false, true} {
		for _, kind := range []uint8{hitscanFxBlood, hitscanFxPuff, hitscanFxSmoke, hitscanFxTeleport, hitscanFxBFGExtra} {
			t.Run(fmt.Sprintf("water=%t/kind=%d", submerged, kind), func(t *testing.T) {
				g := boomTouchFixture(t)
				if submerged {
					g.boomHeightControl[0] = 1
					g.sectorFloor[1] = 32 * fracUnit
				}
				g.hitscanPuffs = []hitscanPuff{{kind: kind, x: -128 * fracUnit}}
				g.projectileImpacts = []projectileImpact{{kind: projectileFatShot, x: -128 * fracUnit}}
				g.tickBoomCarry(boomCarryThinker{sector: 0, vector: boomScrollVector{x: fracUnit, y: -fracUnit}})
				want := int64(0)
				if submerged || kind == hitscanFxBlood {
					want = fracUnit
				}
				if p := g.hitscanPuffs[0]; p.momx != want || p.momy != -want {
					t.Fatalf("puff momentum=(%d,%d) want=(%d,%d)", p.momx, p.momy, want, -want)
				}
				impact := int64(0)
				if submerged {
					impact = fracUnit
				}
				if p := g.projectileImpacts[0]; p.momx != impact || p.momy != -impact {
					t.Fatal("impact carry eligibility differs")
				}
			})
		}
	}
}

func TestBoomBloodSplitMoveRetainsSuccessfulHalfStep(t *testing.T) {
	g := boomTouchFixture(t)
	// A one-sided wall at x=0 permits the first half-step, then rejects the
	// remainder when the effect's 20-unit radius reaches it.
	g.lines[0].sideNum1 = -1
	p := hitscanPuff{kind: hitscanFxBlood, x: -40 * fracUnit, momx: 30 * fracUnit,
		z: 32 * fracUnit, ceilz: 128 * fracUnit, tics: 8, state: 90}
	g.tickHitscanPuff(&p)
	if p.x != -25*fracUnit || p.momx != 0 {
		t.Fatalf("blood x=%d momx=%d", p.x, p.momx)
	}
}

func TestBoomExplodingBarrelKeepsSolidityUntilGibbed(t *testing.T) {
	for phase := 0; phase < 5; phase++ {
		g := boomTouchFixture(t)
		i := g.appendRuntimeThing(mapdata.Thing{Type: barrelThingType, X: -40, Flags: 7}, false)
		g.thingDead[i], g.thingHP[i] = true, 0
		g.thingState[i], g.thingStatePhase[i] = monsterStateDeath, phase
		if !g.monsterCorpseBlocksMovement(i) {
			t.Fatal("exploding barrel incorrectly became non-solid")
		}
		g.setThingMomentum(i, 8*fracUnit, 0, 0)
		g.tickMonsterMomentum(i, g.m.Things[i])
		x, _ := g.thingPosFixed(i, g.m.Things[i])
		if x != -40*fracUnit {
			t.Fatalf("phase %d: barrel passed through actor", phase)
		}
		g.thingGibbed[i], g.thingState[i] = true, monsterStateGibs
		if g.monsterCorpseBlocksMovement(i) {
			t.Fatal("gibbed barrel remained solid")
		}
	}
}

func TestBoomDoorLowestCeilingExcludesSelfReferences(t *testing.T) {
	for _, boom := range []bool{false, true} {
		g := boomTouchFixture(t)
		g.boomMomentumDropoff = boom
		g.m.Sidedefs[1].Sector = 0
		want := int64(128 * fracUnit)
		if boom {
			want = 32000 * fracUnit
		}
		if got := g.lowestSurroundingCeiling(0); got != want {
			t.Fatalf("boom=%t target=%d want=%d", boom, got, want)
		}
		g.m.Sidedefs[1].Sector = 1
		g.sectorCeil[1] = 32767 * fracUnit
		if !boom {
			want = g.sectorCeil[1]
		}
		if got := g.lowestSurroundingCeiling(0); got != want {
			t.Fatal("lowest-ceiling sentinel differs")
		}
	}
}

func TestArachnotronFlightUsesFiveTicStates(t *testing.T) {
	before := doomrand.Capture()
	defer doomrand.Restore(before)
	for _, source := range []int16{68, 3005} {
		g := &game{}
		base := projectileFlightStateTics(projectilePlasmaBall, source)
		want := 4
		if source == 68 {
			want = 5
		}
		if base != want {
			t.Fatalf("source %d: spawn tics=%d want=%d", source, base, want)
		}
		p := projectile{kind: projectilePlasmaBall, sourceType: source, frameTics: base}
		for range want - 1 {
			if g.tickProjectileAnim(&p) {
				t.Fatal("flight frame changed early")
			}
		}
		if !g.tickProjectileAnim(&p) || p.frame != 1 || p.frameTics != want {
			t.Fatal("flight frame did not reload source-specific duration")
		}
	}
}

func TestBoomCarriedVileFireUsesItsFullHeight(t *testing.T) {
	g := boomTouchFixture(t)
	g.sectorCeil[1] = 32 * fracUnit
	fx := projectileImpact{kind: projectileArchVileFire, x: -40 * fracUnit, z: 24 * fracUnit,
		momx: 30 * fracUnit, ceilz: 128 * fracUnit, tics: 100, phaseTics: 100}
	g.advanceProjectileImpactTic(&fx)
	if fx.x != -25*fracUnit || fx.momx != 0 {
		t.Fatalf("fire passed a ceiling with only eight units of clearance: %+v", fx)
	}
}
