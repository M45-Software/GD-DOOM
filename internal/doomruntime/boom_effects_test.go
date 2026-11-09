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
func TestMovingEffectCrossesSilentTeleport(t *testing.T) {
	for _, special := range []uint16{207, 208, 243, 244, 262, 263, 264, 265, 266, 267, 268, 269} {
		g, source, _ := teleportFixtureGame(t)
		g.lineSpecial[source] = special
		g.setThingPosFixed(1, -704*fracUnit, 0)
		p := hitscanPuff{x: -449 * fracUnit, y: 64 * fracUnit, z: 13 * fracUnit,
			momx: 7 * fracUnit, momz: fracUnit, floorz: 0, ceilz: 256 * fracUnit,
			angle: doomAng90, kind: hitscanFxBlood, tics: 8, state: 90}
		g.tickHitscanPuff(&p)
		if p.x <= 0 || p.momz != 0 { // Blood's ordinary Z thinker applies gravity after teleport.
			t.Fatalf("special %d did not teleport blood with normal Z movement: %+v", special, p)
		}
		if len(g.hitscanPuffs) != 0 || g.effectMover != nil {
			t.Fatalf("special %d left fog or a scoped mover", special)
		}
	}
}

func TestEffectTeleportAngleSnapshotCompatibility(t *testing.T) {
	for _, keyframe := range []bool{false, true} {
		magic, version := saveGameMagic, saveGameVersion
		if keyframe {
			magic, version = keyframeMagic, keyframeVersion
		}
		for _, old := range []bool{false, true} {
			v := version
			if old {
				v = 43
				if keyframe {
					v = 30
				}
			}
			file := saveFile{Version: v, Game: gameSaveState{HitscanPuffs: captureHitscanPuffs([]hitscanPuff{{angle: doomAng90, kind: hitscanFxBlood, momx: 1, order: 2}})}}
			blob, err := encodeSnapshot(magic, file)
			if err != nil {
				t.Fatal(err)
			}
			got, err := decodeSnapshot(blob, magic)
			if err != nil {
				t.Fatal(err)
			}
			p := restoreHitscanPuffs(got.Game.HitscanPuffs)[0]
			want := uint32(doomAng90)
			if old {
				want = 0
			}
			if p.angle != want || p.momx != 1 || p.order != 2 {
				t.Fatalf("keyframe=%v old=%v: %+v", keyframe, old, p)
			}
		}
	}
}

func TestMovingBloodTeleportsAndTicksNewFog(t *testing.T) {
	g := newGame(&mapdata.Map{
		Name:     "MAP01",
		Things:   []mapdata.Thing{{Type: 1, X: -100}, {Type: 14, X: 100, Y: 100, Angle: 315, Flags: 7}},
		Vertexes: []mapdata.Vertex{{X: -128, Y: 4}, {X: 128, Y: 4}},
		Linedefs: []mapdata.Linedef{{V1: 0, V2: 1, Flags: mlTwoSided, Special: 97, Tag: 7, SideNum: [2]int32{0, 0}}},
		Sidedefs: []mapdata.Sidedef{{Sector: 0}}, Sectors: []mapdata.Sector{{CeilingHeight: 216, Tag: 7}},
	}, Options{SkillLevel: 3})
	g.hitscanPuffs = []hitscanPuff{{kind: hitscanFxBlood, momx: 180746, momy: 1016236,
		momz: -fracUnit, z: 32 * fracUnit, ceilz: 216 * fracUnit, state: 90, tics: 8, order: g.allocThinkerOrder()}}
	g.tickHitscanPuffs()
	if len(g.hitscanPuffs) != 3 {
		t.Fatalf("effects=%+v", g.hitscanPuffs)
	}
	p := g.hitscanPuffs[0]
	if p.x != 100*fracUnit+90373 || p.y != 100*fracUnit+508118 || p.z != 0 || p.momx != 0 || p.momy != 0 || p.momz != 0 || p.angle != degToAngle(315) {
		t.Fatalf("teleported blood=%+v", p)
	}
	for _, fog := range g.hitscanPuffs[1:] {
		if fog.kind != hitscanFxTeleport || fog.tics != 5 {
			t.Fatalf("new fog did not advance this tic: %+v", fog)
		}
	}
	if g.effectMover != nil {
		t.Fatal("effect mover escaped its movement scope")
	}
}

func TestMovingImpactKeepsDoomProjectileTypeTriggerExclusions(t *testing.T) {
	for _, tc := range []struct {
		kind    projectileKind
		source  int16
		allowed bool
	}{
		{projectileRocket, 16, false}, {projectilePlayerPlasma, 0, false}, {projectileBFGBall, 0, false},
		{projectileFireball, 3001, false}, {projectileBaronBall, 69, false}, {projectilePlasmaBall, 3005, false},
		{projectilePlasmaBall, 68, true}, {projectileTracer, 66, true}, {projectileFatShot, 67, true},
	} {
		g, _, _ := teleportFixtureGame(t)
		g.setThingPosFixed(1, -704*fracUnit, 0)
		fx := projectileImpact{kind: tc.kind, sourceType: tc.source,
			x: -449 * fracUnit, y: 64 * fracUnit, z: 13 * fracUnit, momx: 2 * fracUnit,
			ceilz: 256 * fracUnit, tics: 10, phaseTics: 10}
		g.advanceProjectileImpactTic(&fx)
		if (fx.x > 0) != tc.allowed {
			t.Fatalf("kind=%d source=%d teleported=%v allowed=%v", tc.kind, tc.source, fx.x > 0, tc.allowed)
		}
	}
}
