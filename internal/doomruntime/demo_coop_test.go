package doomruntime

import (
	"errors"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"github.com/hajimehoshi/ebiten/v2"
)

func coopPlaybackTestGame() *game {
	m := cloneMapForRestart(boomConveyorGame().restartTemplate)
	m.Things[1].Y = 128
	m.Things = append(m.Things, mapdata.Thing{Type: 2, X: 128, Y: 256, Flags: 7}, mapdata.Thing{Type: 3, X: 512, Y: 256, Flags: 7})
	options := demo.GameOptions{Seed: 1993, Insurance: true, PlayerBobbing: true, AllowPushers: true}
	script := &DemoScript{Header: DemoHeader{Version: 202, Skill: 2, Episode: 1, Map: 1, ConsolePlayer: 1, PlayerInGame: [4]bool{true, true}, BoomOptions: options.Bytes()}, Frames: make([][4]demo.Tic, 3), Tics: make([]demo.Tic, 3)}
	return newGame(m, Options{Headless: true, DemoScript: script})
}

func TestCoopDemoMapOrderSpawnsOnlyRecordedSlots(t *testing.T) {
	g := coopPlaybackTestGame()
	rng := doomrand.New()
	rng.ConfigureBoom(1993, true)
	for range 3 {
		rng.PRandom(doomrand.Lastlook)
	}
	if doomrand.Capture().Seeds[doomrand.Lastlook] != rng.Capture().Seeds[doomrand.Lastlook] {
		t.Fatal("initial player bodies consumed wrong spawn RNG")
	}
	if g.localSlot != 2 || len(g.authorityPlayers) != 2 || len(g.boomVoodooBodies) != 1 || voodooPlayerSlot(g.m.Things[0].Type) != 1 || len(g.remotePlayers) != 1 {
		t.Fatal("recorded roster did not own its starts, dolls, and camera")
	}
	g.opts.DemoScript.Frames[0][0].Forward = 25
	before := g.authorityPlayers[0].p.x
	if err := g.advanceDemoTic(); err != nil {
		t.Fatal(err)
	}
	if g.worldTic != 1 || g.authorityPlayers[0].p.x <= before || g.localSlot != 2 {
		t.Fatal("frame did not advance the shared world once while keeping selected viewer")
	}
}

func TestCoopDemoPauseFromNonConsoleSlot(t *testing.T) {
	g := coopPlaybackTestGame()
	g.opts.DemoScript.Frames[0][0].Buttons = demo.ButtonSpecial | 1
	g.opts.DemoScript.Frames[1][0].Buttons = demo.ButtonSpecial | 1
	if err := g.advanceDemoTic(); err != nil || g.worldTic != 0 {
		t.Fatal("non-console pause did not stop world")
	}
	if err := g.advanceDemoTic(); err != nil || g.worldTic != 1 {
		t.Fatal("unpause did not resume shared world")
	}
	if err := g.advanceDemoTic(); err != nil {
		t.Fatal(err)
	}
	if err := g.advanceDemoTic(); !errors.Is(err, ebiten.Termination) {
		t.Fatal("recorded frame stream did not terminate")
	}
}

func TestCoopDemoRespawnRetainsBodyAndPointerIdentity(t *testing.T) {
	g := coopPlaybackTestGame()
	owner := g.authorityPlayers[1]
	originalOrder := owner.thinkerOrder(g.m)
	owner.stats.Health = 0
	owner.playerReborn = true
	owner.isDead = true
	owner.playerMobjState, owner.playerMobjTics = 152, 2
	owner.inventory.BlueKey = true
	g.thingHP[0] = 63
	before := doomrand.Capture()
	g.withAuthoritativePlayer(owner, func() {
		g.setMonsterTargetPlayer(0) // Use the doll's retained target pointer too.
		g.respawnCoopDemoPlayer()
	})
	want := doomrand.New()
	want.Restore(before)
	for range 2 {
		want.PRandom(doomrand.Lastlook)
	} // Fog, then new player.
	if doomrand.Capture().Seeds[doomrand.Lastlook] != want.Capture().Seeds[doomrand.Lastlook] {
		t.Fatal("detaching old body consumed spawn RNG")
	}
	old := -1
	for i, th := range g.m.Things {
		if th.Type == detachedPlayerBodyType {
			old = i
		}
	}
	if old < 0 || g.thingHP[old] != 100 || g.thingThinkerOrder[old] != originalOrder || g.thingBlocksInSession(old) || owner.isDead || owner.stats.Health != 100 || owner.inventory.BlueKey || g.thingHP[0] != 63 {
		t.Fatal("respawn changed persistent body state or rebirth inventory")
	}
	if g.thingTargetPlayer[0] || g.thingTargetIdx[0] != old {
		t.Fatal("target pointer followed replacement player")
	}
	g.damageShootableThingFrom(old, 10, false, -1, 0, 0, false)
	if g.thingHP[old] != 90 || owner.stats.Health != 100 {
		t.Fatal("detached body still damaged shared player")
	}
	x := g.thingX[old]
	g.setThingMomentum(old, fracUnit, 0, 0)
	g.tickThingThinker(old, g.m.Things[old])
	if g.thingX[old] != x+fracUnit {
		t.Fatal("non-solid former camera collided with its replacement")
	}
}

func TestAuthoritativePlayerSearchRetainsStopSlot(t *testing.T) {
	g, _, second := authoritativeTargetsTestGame()
	g.boomMomentumDropoff = true
	g.thingLastLook[0] = 1
	second.stats.Health = 0
	second.isDead = true
	second.playerMobjHealth = 0
	if g.monsterLookForPlayer(0, true, 32*fracUnit, 0) || g.thingLastLook[0] != 0 {
		t.Fatal("search wrapped past original stop slot")
	}
	if !g.monsterLookForPlayer(0, true, 32*fracUnit, 0) || g.monsterTargetPlayerSlot(0) != 1 {
		t.Fatal("next look did not restart from retained cursor")
	}
}

func TestBoomSightBoundsRejectDistantHorizontalEquality(t *testing.T) {
	g := &game{m: &mapdata.Map{Linedefs: []mapdata.Linedef{{}}, SubSectors: []mapdata.SubSector{{SegCount: 1}}, Segs: []mapdata.Seg{{}}},
		lines: []physLine{{x1: -128 * fracUnit, y1: -128 * fracUnit, x2: 128 * fracUnit, y2: -128 * fracUnit, dx: 256 * fracUnit, flags: 1, bbox: [4]int64{-128 * fracUnit, -128 * fracUnit, 128 * fracUnit, -128 * fracUnit}}}, lineValid: make([]int, 1)}
	for _, boom := range []bool{false, true} {
		g.boomMomentumDropoff = boom
		g.validCount++
		sight := losTrace{trace: divline{x: 0, y: -48 * fracUnit, dx: -128 * fracUnit, dy: 96 * fracUnit}, t2x: -128 * fracUnit, t2y: 48 * fracUnit}
		if got := g.crossSubsectorLOS(0, &sight); got != boom {
			t.Fatalf("boom=%v sight=%v", boom, got)
		}
	}
}

func TestCoopDemoCampaignInputUsesWholeRoster(t *testing.T) {
	g := coopPlaybackTestGame()
	g.demoIntermissionActive = true
	g.opts.DemoScript.Frames[1][0].Buttons = 1
	if err := g.advanceDemoTic(); err != nil {
		t.Fatal(err)
	}
	if err := g.advanceDemoTic(); err != nil {
		t.Fatal(err)
	}
	if g.demoTick != 2 || !g.demoIntermissionSkip || !g.coopTransitionPrepared {
		t.Fatal("intermission ignored non-console input")
	}
}
