package doomruntime

import (
	"gddoom/internal/mapdata"
	"testing"
)

func TestBoomFailedLegacyTeleportKeepsOneShotLine(t *testing.T) {
	for _, boom := range []bool{false, true} {
		g, source, _ := teleportFixtureGame(t)
		g.boomMomentumDropoff = boom
		g.lineSpecial[source] = 125
		g.m.Linedefs[source].Tag = 999 // No destination: this attempt must fail.
		oldX, y := int64(-449*fracUnit), int64(64*fracUnit)
		g.setThingPosFixed(1, oldX, y)
		g.checkWalkSpecialLinesForActorWithCandidates(oldX, y, -447*fracUnit, y, 1, false, []int{source})
		want := uint16(0)
		if boom {
			want = 125
		}
		if g.lineSpecial[source] != want {
			t.Fatalf("boom=%t consumed failed teleport: %d", boom, g.lineSpecial[source])
		}
	}
}

func TestBoomLivingDollCanExitAfterCameraDeath(t *testing.T) {
	g := newGame(predictionTestMap(), Options{Headless: true})
	g.boomMomentumDropoff = true
	g.isDead = true
	g.stats.Health = 0
	g.m.Things[0].Type = voodooTypeForSlot(g.localSlot)
	g.thingCollected[0] = false
	g.thingHP[0] = 73
	g.lineSpecial = []uint16{52}
	g.lines = []physLine{{idx: 0, x1: 0, y1: 100, x2: 0, y2: -100, dx: 0, dy: -200, slope: slopeVertical, sideNum0: -1, sideNum1: -1}}
	g.checkWalkSpecialLinesForActor(-10, 0, 10, 0, 0, false)
	if !g.levelExitRequested || g.lineSpecial[0] != 52 {
		t.Fatal("living doll did not activate exit for dead owner")
	}
}

func TestBoomRestingArchVileFireSkipsVerticalClamp(t *testing.T) {
	g := &game{}
	fx := projectileImpact{kind: projectileArchVileFire, z: 0, floorz: 0, ceilz: 8 * fracUnit, tics: 20, phaseTics: 2}
	if !g.advanceProjectileImpactTic(&fx) || fx.z != 0 {
		t.Fatal("resting fire was pushed below the floor")
	}
	fx.z = fracUnit
	if !g.advanceProjectileImpactTic(&fx) || fx.z != -8*fracUnit {
		t.Fatal("airborne fire did not run vertical clipping")
	}
}

func TestBoomZeroDenominatorIntermissionCounters(t *testing.T) {
	g := &game{m: &mapdata.Map{}, boomMomentumDropoff: true}
	stats := collectIntermissionStats(g, "MAP01", "MAP02")
	if stats.KillsPct != 100 || stats.ItemsPct != 100 || stats.SecretsPct != 100 || stats.SecretsTotal != 0 {
		t.Fatalf("zero-denominator stats=%+v", stats)
	}
	state := intermissionState{Active: true, SPState: 2, Target: stats, Show: intermissionStats{KillsPct: -1, ItemsPct: -1, SecretsPct: -1}}
	state, _, _ = tickVanillaIntermission(state, false)
	if state.Show.KillsPct != 100 || state.SPState != 3 {
		t.Fatal("zero kills counted to 100 instead of snapping after its first increment")
	}
	state.SPState = 4
	state, _, _ = tickVanillaIntermission(state, false)
	if state.Show.ItemsPct != 100 || state.SPState != 5 {
		t.Fatal("zero items used the display percentage as its counting limit")
	}
}

func TestBoomDollSlideUsesLivingBodyHeightAfterOwnerDies(t *testing.T) {
	m := &mapdata.Map{Name: "MAP01", Things: []mapdata.Thing{{Type: 1, X: 0, Y: 0}, {Type: 1, X: 512, Y: 512}},
		Vertexes: []mapdata.Vertex{{X: -128, Y: 32}, {X: 128, Y: 32}},
		Linedefs: []mapdata.Linedef{{V1: 0, V2: 1, Flags: mlTwoSided, SideNum: [2]int32{0, 1}}},
		Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}},
		Sectors:  []mapdata.Sector{{CeilingHeight: 128}, {CeilingHeight: 24}}}
	g := newGame(m, Options{Headless: true})
	g.boomMomentumDropoff = true
	g.isDead = true
	g.stats.Health = 0
	g.playerMobjHealth = 0
	g.thingHP[0] = 73
	g.setThingPosFixed(0, 0, 0)
	g.setThingSupportState(0, 0, 0, 128*fracUnit)
	g.setThingMomentum(0, 0, 64*fracUnit, 0)
	before := g.p
	g.slideVoodooDoll(0, g.m.Things[0])
	if g.thingMomY[0] != 0 {
		t.Fatalf("living doll slid using dead owner's short height: momy=%d", g.thingMomY[0])
	}
	if g.p != before || g.playerMobjHealth != 0 || !g.isDead {
		t.Fatal("doll slide replaced the camera's body state")
	}
}
