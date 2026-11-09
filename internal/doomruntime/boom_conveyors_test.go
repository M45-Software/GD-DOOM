package doomruntime

import (
	"gddoom/internal/mapdata"
	"testing"
)

func boomConveyorGame() *game {
	m := &mapdata.Map{
		Things:   []mapdata.Thing{{Type: 1, X: -64, Flags: 7}, {Type: 1, X: 256, Flags: 7}},
		Vertexes: []mapdata.Vertex{{}, {X: 1024}},
		Linedefs: []mapdata.Linedef{{V1: 0, V2: 1, Special: 253, Tag: 7, SideNum: [2]int32{0, -1}}},
		Sidedefs: []mapdata.Sidedef{{Sector: 0}},
		Sectors:  []mapdata.Sector{{Tag: 7, CeilingHeight: 128, FloorPic: "FLOOR"}},
	}
	return newGame(m, Options{Headless: true})
}

func TestBoomConveyorCarriesVoodooBodyInThinkerOrder(t *testing.T) {
	g := boomConveyorGame()
	if !isVoodooDollType(g.m.Things[0].Type) || g.localPlayerThingIndex != 1 || g.thingCurrentHeight(0, g.m.Things[0]) != playerHeight {
		t.Fatal("duplicate start did not become physical player body")
	}
	start := g.thingX[0]
	g.runGameplayTic(moveCmd{}, false, false)
	if g.thingX[0] != start || g.thingMomX[0] != 3*fracUnit {
		t.Fatalf("carry impulse/movement phase incorrect: x=%d mom=%d", g.thingX[0], g.thingMomX[0])
	}
	g.runGameplayTic(moveCmd{}, false, false)
	if g.thingX[0] != start+3*fracUnit {
		t.Fatalf("voodoo body did not move: x=%d", g.thingX[0])
	}
	key := g.plane3DKeyForSectorCached(0, &g.m.Sectors[0], true)
	if key.offsetX != -64*fracUnit || key.offsetY != 0 {
		t.Fatalf("floor texture phase=%d,%d", key.offsetX, key.offsetY)
	}
}

func TestBoomConveyorRejectsAirborneAndFloatingActors(t *testing.T) {
	g := boomConveyorGame()
	g.p.z = fracUnit
	g.thingZState[0] = fracUnit
	g.tickBoomCarry(g.boomCarries[0])
	if g.p.momx != 0 || g.thingMomX[0] != 0 {
		t.Fatal("conveyor carried airborne actor")
	}
}

func TestBoomConveyorCarriesEveryAuthoritativePlayer(t *testing.T) {
	g := boomConveyorGame()
	first := g.captureAuthoritativePlayer()
	second := first
	second.localSlot = 2
	second.p.x += 64 * fracUnit
	g.authorityPlayers = []*authoritativePlayerState{&first, &second}
	g.tickBoomCarry(g.boomCarries[0])
	if g.p.momx != 3*fracUnit || first.p.momx != 3*fracUnit || second.p.momx != 3*fracUnit {
		t.Fatal("conveyor did not reach every player exactly once")
	}
}

func TestBoomVoodooDamageSharesPlayerHealth(t *testing.T) {
	g := boomConveyorGame()
	main := g.p
	g.damageShootableThing(0, 15)
	if g.stats.Health != 85 || g.p.x != main.x || g.p.y != main.y || g.thingHP[0] != 85 {
		t.Fatal("voodoo damage did not reach player or moved camera body")
	}
}

func TestBoomConveyorAndVoodooSaveRestore(t *testing.T) {
	g := boomConveyorGame()
	for i := 0; i < 5; i++ {
		g.runGameplayTic(moveCmd{}, false, false)
	}
	saved := captureGameSaveState(g)
	data, err := encodeSnapshot(saveGameMagic, saveFile{Version: saveGameVersion, Game: saved})
	if err != nil {
		t.Fatal(err)
	}
	file, err := decodeSnapshot(data, saveGameMagic)
	if err != nil {
		t.Fatal(err)
	}
	saved = file.Game
	restored := boomConveyorGame()
	restoreGameSaveState(restored, saved)
	for i := 0; i < 5; i++ {
		g.runGameplayTic(moveCmd{}, false, false)
		restored.runGameplayTic(moveCmd{}, false, false)
	}
	if g.thingX[0] != restored.thingX[0] || g.thingMomX[0] != restored.thingMomX[0] || g.worldTic != restored.worldTic {
		t.Fatal("restored conveyor/doll diverged")
	}
}

func TestBoomVoodooCrossesPlayerOnlyGeneralizedFloorTrigger(t *testing.T) {
	g := boomConveyorGame()
	// The belt lies in a control sector. This two-sided, self-referencing
	// trigger crosses its path and raises the same floor by 24 map units.
	g.m.Linedefs[0].Flags = mlTwoSided
	g.m.Linedefs[0].SideNum = [2]int32{0, 0}
	g.m.Vertexes = append(g.m.Vertexes, mapdata.Vertex{X: 0, Y: 128}, mapdata.Vertex{X: 0, Y: -128})
	const special = 0x6000 | 64 | (6 << 7)
	g.m.Linedefs = append(g.m.Linedefs, mapdata.Linedef{V1: 2, V2: 3, Flags: mlTwoSided, SideNum: [2]int32{0, 0}, Tag: 7, Special: special})
	g.initPhysics()
	g.physForLine = []int{0, 1}
	for i := 0; i < 80; i++ {
		g.runGameplayTic(moveCmd{}, false, false)
	}
	if g.lineSpecial[1] != 0 || g.sectorFloor[0] != 24*fracUnit {
		t.Fatalf("voodoo crossing did not run player-only W1: special=%d floor=%d dollx=%d", g.lineSpecial[1], g.sectorFloor[0], g.thingX[0])
	}
	if g.p.x == g.thingX[0] {
		t.Fatal("voodoo movement replaced main player position")
	}
}

func TestVoodooSelfDamageRetainsLiveControlledAttacker(t *testing.T) {
	g := boomConveyorGame()
	g.damageVoodooDoll(0, 10000, g.p.x, g.p.y, g.p.z, true, -1, true)
	if !g.isDead || g.statusAttackerThing != selfAttackerThing {
		t.Fatalf("dead=%v attacker=%d", g.isDead, g.statusAttackerThing)
	}
	g.setPlayerPosFixed(100*fracUnit, 200*fracUnit)
	g.p.angle = doomAng270
	g.tickDeadPlayerTurnTowardAttacker()
	if g.p.angle != doomAng270 {
		t.Fatal("self attacker rotated death camera")
	}
	x, y := g.statusAttackerPosition()
	if x != g.p.x || y != g.p.y {
		t.Fatalf("attacker=(%d,%d) player=(%d,%d)", x, y, g.p.x, g.p.y)
	}
}

func TestVoodooSlideKeepsSplitMoveRemainder(t *testing.T) {
	g := newGame(&mapdata.Map{Name: "MAP01",
		Things:   []mapdata.Thing{{Type: 1, X: -20}, {Type: 1, X: -512}},
		Vertexes: []mapdata.Vertex{{Y: 256}, {Y: -256}},
		Linedefs: []mapdata.Linedef{{V1: 0, V2: 1, SideNum: [2]int32{0, -1}}},
		Sidedefs: []mapdata.Sidedef{{Sector: 0}}, Sectors: []mapdata.Sector{{CeilingHeight: 128}},
	}, Options{Headless: true})
	g.setThingMomentum(0, 30*fracUnit, 10*fracUnit, 0)
	g.tickMonsterMomentum(0, g.m.Things[0])
	// The first slide reaches the wall and retains its tangential momentum.
	// The still-pending half-step hits again and slides along that wall too.
	_, y := g.thingPosFixed(0, g.m.Things[0])
	if y <= 15*fracUnit || g.thingMomX[0] != 0 {
		t.Fatalf("split remainder discarded: y=%d momentum=(%d,%d)", y, g.thingMomX[0], g.thingMomY[0])
	}
}

func TestBoomConveyorCarriesFallenNoGravityMonsterCorpses(t *testing.T) {
	for _, typ := range []int16{3005, 71, 72, 3006} {
		for _, dead := range []bool{false, true} {
			g := boomTouchFixture(t)
			i := g.appendRuntimeThing(mapdata.Thing{Type: typ, X: -128, Flags: 7}, false)
			g.ensureMonsterAIState()
			g.thingDead[i] = dead
			if dead {
				g.thingHP[i] = -1
			}
			g.setThingSupportState(i, 0, 0, 128*fracUnit)
			g.tickBoomCarry(boomCarryThinker{sector: 0, vector: boomScrollVector{x: fracUnit}})
			want := int64(0)
			if dead && typ != 3006 {
				want = fracUnit
			}
			if g.thingMomX[i] != want {
				t.Fatalf("type=%d dead=%v carry=%d want=%d", typ, dead, g.thingMomX[i], want)
			}
		}
	}
}
