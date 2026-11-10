package doomruntime

import (
	"reflect"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestBoomCoopChaseFindsAnotherPlayerAfterLosingSight(t *testing.T) {
	g, first, second := authoritativeTargetsTestGame()
	g.boomMomentumDropoff = true
	g.opts.GameMode = gameModeCoop
	g.opts.DemoScript = &DemoScript{Header: DemoHeader{Version: 202, PlayerInGame: [4]bool{true, true}}, Frames: make([][4]demo.Tic, 1)}
	// The retained first player is beyond a wall; the second is visible on
	// the monster's side. Retaliation must expire before A_Chase switches.
	g.lines = []physLine{{idx: 0, x1: 128 * fracUnit, y1: -128 * fracUnit, x2: 128 * fracUnit, y2: 128 * fracUnit, dy: 256 * fracUnit, flags: 1}}
	g.m.Linedefs = []mapdata.Linedef{{Flags: 1}}
	g.withAuthoritativePlayer(first, func() { g.setMonsterTargetPlayer(0) })
	g.thingLastLook[0] = 0
	g.thingThreshold[0] = 1
	g.thingMoveCount[0] = 9
	before := doomrand.Capture()
	if g.retargetHiddenCoopPlayer(0, 3002, 32*fracUnit, 0) {
		t.Fatal("retargeted during retaliation")
	}
	g.thingThreshold[0] = 0
	if !g.retargetHiddenCoopPlayer(0, 3002, 32*fracUnit, 0) || g.monsterTargetPlayerSlot(0) != second.localSlot {
		t.Fatal("did not reacquire the other visible player")
	}
	if g.thingMoveCount[0] != 9 || !reflect.DeepEqual(before, doomrand.Capture()) {
		t.Fatal("reacquisition moved or drew chase RNG")
	}
	if g.localSlot != 1 || g.p.x != first.p.x {
		t.Fatal("target search replaced the selected player")
	}
}

func TestBoomCoopDeadPlayerReacquireAdvancesFrameWithoutChaseMove(t *testing.T) {
	for _, typ := range []int16{69, 66} {
		g, first, second := authoritativeTargetsTestGame()
		g.m.Things[0].Type = typ
		g.boomMomentumDropoff = true
		g.opts.GameMode = gameModeCoop
		g.withAuthoritativePlayer(second, func() {
			g.setMonsterTargetPlayer(0)
			g.isDead, g.stats.Health, g.playerMobjHealth = true, 0, -10
		})
		g.thingState[0], g.thingStatePhase[0], g.thingStateTics[0] = monsterStateSee, 2, 1
		g.thingLastLook[0], g.thingThreshold[0], g.thingReactionTics[0] = 0, 0, 0
		g.thingMoveCount[0], g.thingJustAtk[0] = 9, true
		x, y := g.thingPosFixed(0, g.m.Things[0])
		before := doomrand.Capture()
		g.tickGenericMonsterState(0, g.m.Things[0])
		if g.monsterTargetPlayerSlot(0) != first.localSlot {
			t.Fatalf("actor %d did not reacquire the surviving player", typ)
		}
		if g.thingStatePhase[0] != 3 || g.thingStateTics[0] != g.monsterSeeStateTicsForPhase(0, typ) {
			t.Fatalf("actor %d lost its newly-entered walk frame", typ)
		}
		nx, ny := g.thingPosFixed(0, g.m.Things[0])
		if nx != x || ny != y || g.thingMoveCount[0] != 9 || !g.thingJustAtk[0] || !reflect.DeepEqual(before, doomrand.Capture()) {
			t.Fatalf("actor %d continued chasing after reacquisition", typ)
		}
	}
	t.Cleanup(doomrand.Clear)
}

func TestBoomCoopDeadPlayerReacquireOnRunEntryPreservesJustAttacked(t *testing.T) {
	for _, justAttacked := range []bool{false, true} {
		g, first, second := authoritativeTargetsTestGame()
		g.m.Things[0].Type = 66
		g.boomMomentumDropoff = true
		g.withAuthoritativePlayer(second, func() {
			g.setMonsterTargetPlayer(0)
			g.isDead, g.stats.Health, g.playerMobjHealth = true, 0, -10
		})
		g.thingState[0], g.thingStatePhase[0], g.thingStateTics[0] = monsterStateSee, 0, 2
		g.thingResumeChaseNow[0] = true // RUN1 entered after an attack or raise.
		g.thingLastLook[0], g.thingThreshold[0], g.thingReactionTics[0] = 0, 0, 0
		g.thingMoveCount[0], g.thingJustAtk[0] = 10, justAttacked
		x, y := g.thingPosFixed(0, g.m.Things[0])
		before := doomrand.Capture()
		g.tickGenericMonsterState(0, g.m.Things[0])
		nx, ny := g.thingPosFixed(0, g.m.Things[0])
		if g.monsterTargetPlayerSlot(0) != first.localSlot || g.thingStatePhase[0] != 0 || g.thingStateTics[0] != 2 {
			t.Fatal("run entry did not retain its frame after player reacquisition")
		}
		if nx != x || ny != y || g.thingMoveCount[0] != 10 || g.thingJustAtk[0] != justAttacked || !reflect.DeepEqual(before, doomrand.Capture()) {
			t.Fatal("run entry performed a second chase after reacquiring the surviving player")
		}
	}
	t.Cleanup(doomrand.Clear)
}

func TestBoomLoweringPlatCreatesHeadroomForInitiallyTrappedPlayer(t *testing.T) {
	for _, boom := range []bool{false, true} {
		t.Run(map[bool]string{false: "vanilla", true: "boom"}[boom], func(t *testing.T) {
			g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{FloorHeight: 128, CeilingHeight: 128}}}, sectorFloor: []int64{128 * fracUnit}, sectorCeil: []int64{128 * fracUnit}, localSlot: 1, localPlayerThingIndex: -1, stats: playerStats{Health: 100}, playerMobjHealth: 100, p: player{z: 128 * fracUnit, floorz: 128 * fracUnit, ceilz: 128 * fracUnit, sector: 0, subsector: -1}, boomMomentumDropoff: boom}
			g.initPhysics()
			pt := &platThinker{sector: 0, typ: platTypeDownWaitUpStay, status: platStatusDown, speed: 4 * fracUnit, low: 0, high: 128 * fracUnit}
			g.tickPlat(0, pt)
			want := int64(128 * fracUnit)
			if boom {
				want = 124 * fracUnit
			}
			if g.sectorFloor[0] != want {
				t.Fatalf("lowering floor=%d want %d", g.sectorFloor[0], want)
			}
			// The final overshoot check retains Doom's rejected-destination behavior.
			g.sectorFloor[0] = 128 * fracUnit
			g.p.z = 128 * fracUnit
			g.p.floorz = 128 * fracUnit
			pt.low = 127 * fracUnit
			g.tickPlat(0, pt)
			if g.sectorFloor[0] != 128*fracUnit || pt.status != platStatusWaiting {
				t.Fatal("final destination no longer performs its obstruction check")
			}
		})
	}
}

func TestBoomDollResidualMomentumUsesOwnerCommand(t *testing.T) {
	for _, moving := range []bool{false, true} {
		t.Run(map[bool]string{false: "neutral_owner", true: "moving_owner"}[moving], func(t *testing.T) {
			g := coopPlaybackTestGame()
			owner := g.authorityPlayers[0]
			g.currentMoveCmd = moveCmd{forward: 50}
			if moving {
				owner.currentMoveCmd = moveCmd{forward: 50}
				g.currentMoveCmd = moveCmd{}
			}
			g.thingMomY[0] = 142
			g.tickMonsterMomentum(0, g.m.Things[0])
			want := int64(0)
			if moving {
				want = fixedMul(142, friction)
			}
			if g.thingMomY[0] != want {
				t.Fatalf("residual momentum=%d want %d", g.thingMomY[0], want)
			}
		})
	}
}

func TestBoomFrozenCoopViewUsesSharedGroundState(t *testing.T) {
	g := coopPlaybackTestGame()
	g.withAuthoritativePlayer(g.authorityPlayers[0], func() { g.p.z = 10 * fracUnit; g.p.floorz = 0; g.p.momz = 0 })
	g.withAuthoritativePlayer(g.authorityPlayers[1], func() {
		g.p.reactionTime = 10
		g.p.viewGroundState = 1
		g.p.viewHeight = 35 * fracUnit
		g.p.deltaViewHeight = fracUnit
	})
	if err := g.advanceDemoTic(); err != nil {
		t.Fatal(err)
	}
	second := g.authorityPlayers[1]
	if second.p.viewHeight != 35*fracUnit || second.p.viewGroundState != -1 {
		t.Fatal("frozen second player did not inherit the airborne first player's ground flag")
	}
}

func TestCoopEnvironmentKillsCreditRetainedEnemyOrFirstSlot(t *testing.T) {
	g := coopPlaybackTestGame()
	// The selected console is slot two. Environmental kills default to slot one.
	g.creditCoopUnownedKill(0)
	if g.authorityPlayers[0].playerKillCount != 1 || g.playerKillCount != 0 {
		t.Fatal("environmental kill followed viewer instead of first active slot")
	}
	g.setMonsterTargetPlayer(0)
	g.rememberBoomEnemy(0)
	g.creditCoopUnownedKill(0)
	if g.playerKillCount != 1 {
		t.Fatal("retained live player did not receive environmental kill credit")
	}
}

func TestWeaponPickupAfterEmptyInventoryCheckpoint(t *testing.T) {
	g := coopPlaybackTestGame()
	// Restored inventory canonically omits unowned weapon entries.
	g.inventory = restorePlayerInventorySaveState(capturePlayerInventorySaveState(playerInventory{ReadyWeapon: weaponPistol}))
	if g.inventory.Weapons != nil {
		t.Fatal("fixture does not have empty restored weapon inventory")
	}
	if _, _, ok := g.applyPickup(2003, false); !ok || !g.inventory.Weapons[2003] || g.stats.Rockets != 2 {
		t.Fatal("rocket launcher pickup failed after restoration")
	}
}

func TestBoomIsolatedHighestFloorIgnoresSelfReferences(t *testing.T) {
	g := newGame(predictionTestMap(), Options{Headless: true})
	g.boomMomentumDropoff = true
	g.m.Sidedefs = []mapdata.Sidedef{{Sector: 0}, {Sector: 0}}
	g.m.Linedefs = []mapdata.Linedef{{Flags: mlTwoSided, SideNum: [2]int32{0, 1}}}
	if got := g.findHighestFloorSurrounding(0); got != -32000*fracUnit {
		t.Fatalf("isolated floor target=%d", got)
	}
}

func TestCampaignCarryoverInitializesDollHealth(t *testing.T) {
	g := coopPlaybackTestGame()
	g.withAuthoritativePlayer(g.authorityPlayers[0], func() {
		carry := g.captureLevelCarryover()
		carry.Stats.Health = 73
		g.applyLevelCarryover(carry)
		for i, th := range g.m.Things {
			if voodooPlayerSlot(th.Type) == g.localSlot && g.thingHP[i] != 73 {
				t.Fatal("doll spawned with fresh health instead of carried health")
			}
		}
	})
}
