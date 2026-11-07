package doomruntime

import (
	"reflect"
	"testing"

	"gddoom/internal/mapdata"
)

func voodooAuthority(t *testing.T) *Authority {
	t.Helper()
	m := cloneMapForRestart(boomConveyorGame().restartTemplate)
	m.Things = nil
	for slot := 1; slot <= 4; slot++ {
		m.Things = append(m.Things, mapdata.Thing{Type: int16(slot), X: int16(slot * 128), Flags: 7})
		if slot == 2 {
			m.Things = append(m.Things, mapdata.Thing{Type: 2, X: 640, Flags: 7})
		}
		m.Things = append(m.Things, mapdata.Thing{Type: int16(slot), X: int16(slot * 128), Y: 1024, Flags: 7})
	}
	m.Things = append(m.Things, mapdata.Thing{Type: 2007, X: 258, Flags: 7})
	a, err := NewAuthority(m, Options{SkillLevel: 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []byte{2, 1, 4, 3} {
		if err := a.AddPlayer(id); err != nil {
			t.Fatal(err)
		}
	}
	return a
}

func dollsForSlot(g *game, slot int) []int {
	var dolls []int
	for i, th := range g.m.Things {
		if voodooPlayerSlot(th.Type) == slot && !g.thingCollected[i] {
			dolls = append(dolls, i)
		}
	}
	return dolls
}

func TestAuthorityVoodooOwnershipAndIndependentBodies(t *testing.T) {
	a := voodooAuthority(t)
	g := a.g
	for slot := 1; slot <= 4; slot++ {
		if a.players[slot].p.y != 1024*fracUnit || len(dollsForSlot(g, slot)) == 0 {
			t.Fatal("camera did not use last numbered start")
		}
	}
	dolls := dollsForSlot(g, 2)
	camera := a.players[2].p
	// Environmental damage belongs to slot 2 although slot 1 is the anchor.
	g.damageShootableThingFrom(dolls[0], 30, false, -1, 0, 0, false)
	g.damageShootableThingFrom(dolls[1], 20, false, -1, 0, 0, false)
	if a.players[2].stats.Health != 50 || g.thingHP[dolls[0]] != 70 || g.thingHP[dolls[1]] != 80 || a.players[2].playerMobjHealth != 100 || a.players[2].p != camera || a.players[1].stats.Health != 100 {
		t.Fatal("doll damage mixed player ownership or body health")
	}
	// Shared health can reach zero before any particular body dies.
	g.damageShootableThingFrom(dolls[1], 50, false, -1, 0, 0, false)
	if a.players[2].stats.Health != 0 || a.players[2].isDead || g.thingHP[dolls[1]] != 30 {
		t.Fatal("shared zero health incorrectly killed a surviving body")
	}
	g.damageShootableThingFrom(dolls[0], 70, false, -1, 0, 0, false)
	if !a.players[2].isDead || !g.thingDead[dolls[0]] || g.thingDead[dolls[1]] || a.MatchState().Scores[1].Deaths != 1 || g.thingCurrentHeight(dolls[0], g.m.Things[dolls[0]]) != 14*fracUnit {
		t.Fatal("doll death did not kill only its owner and body")
	}
	if err := a.spawnAuthoritativePlayer(2, true); err != nil {
		t.Fatal(err)
	}
	if a.players[2].isDead || a.players[2].stats.Health != 100 || g.thingHP[dolls[1]] != 30 || !g.thingDead[dolls[0]] {
		t.Fatal("respawn replaced persistent dolls")
	}
	g.damageShootableThingFrom(dolls[1], 30, false, -1, 0, 0, false)
	if !a.players[2].isDead || a.players[2].stats.Health != 70 || a.MatchState().Scores[1].Deaths != 2 {
		t.Fatal("old doll did not retain its health and owner across respawn")
	}
}

func TestAuthorityVoodooFriendlyFireAndStaleSource(t *testing.T) {
	a := voodooAuthority(t)
	g := a.g
	doll := dollsForSlot(g, 2)[0]
	g.damageShootableThing(doll, 10)
	if g.thingHP[doll] != 100 {
		t.Fatal("doll bypassed friendly-fire protection")
	}
	g.authorityRules.Config.FriendlyFire = true
	g.damageShootableThing(doll, 10)
	if a.players[2].stats.Health != 90 || a.players[1].stats.Health != 100 {
		t.Fatal("enabled friendly fire hit wrong owner")
	}
	// A stale missile retains its owner's protection policy, without credit.
	g.authorityRules.Config.FriendlyFire = false
	g.withProjectileSource(1, g.authorityPlayerGeneration(1)-1, func(bool) { g.damageShootableThingFrom(doll, 20, false, -1, 0, 0, false) })
	if g.thingHP[doll] != 90 {
		t.Fatal("stale source bypassed friendly-fire policy")
	}
}

func TestAuthorityVoodooPickupUsesOwner(t *testing.T) {
	a := voodooAuthority(t)
	g := a.g
	doll := dollsForSlot(g, 2)[0]
	a.players[2].stats.Bullets = 1
	g.setThingMomentum(doll, 2*fracUnit, 0, 0)
	g.tickThingThinker(doll, g.m.Things[doll])
	if a.players[2].stats.Bullets != 11 || a.players[1].stats.Bullets != 50 {
		t.Fatalf("pickup went to wrong inventory: owner=%d anchor=%d", a.players[2].stats.Bullets, a.players[1].stats.Bullets)
	}
}

func TestAuthorityVoodooReplicationAndRejoin(t *testing.T) {
	a := voodooAuthority(t)
	g := a.g
	old := dollsForSlot(g, 2)
	g.damageShootableThingFrom(old[0], 17, false, -1, 0, 0, false)
	for _, viewer := range []byte{1, 2, 3, 4} {
		data, err := a.Snapshot(viewer)
		if err != nil {
			t.Fatal(err)
		}
		client := newGame(cloneMapForRestart(g.restartTemplate), Options{Headless: true, GameMode: gameModeCoop, SkillLevel: 3})
		if err := client.applyAuthoritySnapshot(data); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(g.m.Things, client.m.Things) || !reflect.DeepEqual(g.thingHP, client.thingHP) || !reflect.DeepEqual(g.boomVoodooBodies, client.boomVoodooBodies) || client.localSlot != int(viewer) {
			t.Fatal("viewer lost doll identity, health, or cache")
		}
	}
	a.RemovePlayer(2)
	for _, idx := range old {
		if !g.thingCollected[idx] {
			t.Fatal("disconnect left active doll")
		}
	}
	if err := a.AddPlayer(2); err != nil {
		t.Fatal(err)
	}
	for _, idx := range dollsForSlot(g, 2) {
		if idx <= old[len(old)-1] || g.thingHP[idx] != 100 {
			t.Fatal("rejoin reused retired body identity")
		}
	}
	g.damageShootableThingFrom(old[0], 100, false, -1, 0, 0, false)
	if a.players[2].stats.Health != 100 {
		t.Fatal("retired doll damaged replacement player")
	}
	if err := a.Step(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Snapshot(2); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorityVoodooWorldCodecsPreserveBodies(t *testing.T) {
	a := voodooAuthority(t)
	g := a.g
	dolls := dollsForSlot(g, 2)
	g.damageShootableThingFrom(dolls[0], 200, false, -1, 0, 0, false)
	g.setThingMomentum(dolls[1], 3*fracUnit, fracUnit, 0)
	for _, format := range []struct {
		magic   []byte
		version int
	}{{saveGameMagic, saveGameVersion}, {keyframeMagic, keyframeVersion}} {
		t.Run(string(format.magic), func(t *testing.T) {
			saved := captureGameSaveState(g)
			raw, err := encodeSnapshot(format.magic, saveFile{Version: format.version, Game: saved})
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeSnapshot(raw, format.magic)
			if err != nil {
				t.Fatal(err)
			}
			restored := newGame(cloneMapForRestart(g.restartTemplate), Options{Headless: true, GameMode: gameModeCoop, SkillLevel: 3})
			restoreGameSaveState(restored, decoded.Game)
			if !reflect.DeepEqual(g.m.Things, restored.m.Things) || !reflect.DeepEqual(g.thingHP, restored.thingHP) || !reflect.DeepEqual(g.thingDoomState, restored.thingDoomState) || !reflect.DeepEqual(g.thingStateTics, restored.thingStateTics) || !reflect.DeepEqual(g.thingMomX, restored.thingMomX) || !reflect.DeepEqual(g.boomVoodooBodies, restored.boomVoodooBodies) {
				t.Fatal("world codec lost owner/body state")
			}
			// World save formats do not contain the authority roster. Bind an owned
			// copy explicitly to test deterministic physical continuation only.
			for _, p := range g.authorityPlayers {
				copy := *p
				restored.authorityPlayers = append(restored.authorityPlayers, &copy)
			}
			restored.authorityRules = g.authorityRules
			for range 12 {
				restored.tickThingThinker(dolls[0], restored.m.Things[dolls[0]])
				g.tickThingThinker(dolls[0], g.m.Things[dolls[0]])
			}
			if restored.thingDoomState[dolls[0]] != g.thingDoomState[dolls[0]] || restored.thingStateTics[dolls[0]] != g.thingStateTics[dolls[0]] {
				t.Fatal("restored death thinker diverged")
			}
		})
	}
}

func TestAuthorityVoodooDeathKeepsCameraBodySolid(t *testing.T) {
	a := voodooAuthority(t)
	g := a.g
	doll := dollsForSlot(g, 2)[0]
	g.damageShootableThingFrom(doll, 100, false, -1, 0, 0, false)
	body, dead, _ := g.authoritativePlayerBody(a.players[2])
	if dead || !a.players[2].isDead || !g.actorBlockedByThings(body.x, body.y, playerRadius, -1, false) {
		t.Fatal("doll death cleared the controlled body's solidity")
	}
	g.withAuthoritativePlayer(a.players[2], func() {
		if g.playerMobjHeight() != playerHeight {
			t.Fatal("doll death shrank camera body")
		}
		g.damagePlayer(10, "after doll death")
	})
	if a.players[2].playerMobjHealth != 90 {
		t.Fatal("living camera body stopped receiving damage")
	}
}

func TestAuthorityVoodooRejectsOrphanSnapshot(t *testing.T) {
	a := voodooAuthority(t)
	a.RemovePlayer(4)
	data, err := a.Snapshot(1)
	if err != nil {
		t.Fatal(err)
	}
	r, err := decodeAuthorityReplica(data)
	if err != nil {
		t.Fatal(err)
	}
	idx := dollsForSlot(a.g, 2)[0]
	r.Game.Things[idx].Type = voodooTypeForSlot(4)
	if err := validateAuthorityReplica(a.g, r); err == nil {
		t.Fatal("accepted live doll belonging to inactive slot")
	}
}

func TestVoodooLegacySinglePlayerOwnerRestoredFromStart(t *testing.T) {
	m := cloneMapForRestart(boomConveyorGame().restartTemplate)
	for i := range m.Things {
		m.Things[i].Type = 2
	}
	g := newGame(m, Options{Headless: true, PlayerSlot: 2})
	saved := captureGameSaveState(g)
	saved.Things[0].Type = voodooThingType // historical encoding for any local slot
	restored := newGame(cloneMapForRestart(g.restartTemplate), Options{Headless: true, PlayerSlot: 2})
	restoreGameSaveState(restored, saved)
	restored.damageShootableThing(0, 10)
	if restored.stats.Health != 90 || voodooPlayerSlot(restored.m.Things[0].Type) != 2 {
		t.Fatal("historical doll lost its local owner")
	}
}

func TestAuthorityVoodooFrictionArraysGrowOnJoinAndRejoin(t *testing.T) {
	a := voodooAuthority(t)
	g := a.g
	g.m.Linedefs[0].Special = 223
	g.initBoomEnvironment()
	a.RemovePlayer(2)
	if err := a.AddPlayer(2); err != nil {
		t.Fatal(err)
	}
	g.tickBoomFriction(g.boomFrictions[0])
	if len(g.thingFriction) != len(g.m.Things) || len(g.thingMoveFactor) != len(g.m.Things) {
		t.Fatal("joining dolls lost friction state")
	}
	if _, err := a.Snapshot(2); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorityVoodooCrusherRoutesSharedDamage(t *testing.T) {
	a := voodooAuthority(t)
	g := a.g
	g.worldTic = 1
	g.setSectorCeilingHeightWithCrush(0, 48*fracUnit, true)
	// Slot 2 has two dolls and one controlled body in this sector.
	if a.players[2].stats.Health != 70 || a.players[1].stats.Health != 80 {
		t.Fatalf("crusher shared damage owner2=%d owner1=%d", a.players[2].stats.Health, a.players[1].stats.Health)
	}
	for _, idx := range dollsForSlot(g, 2) {
		if g.thingHP[idx] != 90 {
			t.Fatal("crusher did not damage independent doll body")
		}
	}
}

func TestAuthorityVoodooTelefragProtectionAndOwnerBody(t *testing.T) {
	a := voodooAuthority(t)
	g := a.g
	doll := dollsForSlot(g, 2)[0]
	x, y := g.thingPosFixed(doll, g.m.Things[doll])
	if g.teleportStompDestinationThings(x, y, playerRadius, -1, true, 0, 0) || g.thingHP[doll] != 100 {
		t.Fatal("protected teammate doll did not block telefrag")
	}
	g.authorityRules.Config.FriendlyFire = true
	if !g.teleportStompDestinationThings(x, y, playerRadius, -1, true, 0, 0) || !a.players[2].isDead {
		t.Fatal("telefrag did not kill doll owner")
	}
	b := voodooAuthority(t)
	g = b.g
	doll = dollsForSlot(g, 2)[0]
	body := b.players[2].p
	if !g.teleportStompDestinationThings(body.x, body.y, playerRadius, doll, false, 0, 0) || !b.players[2].isDead || b.players[1].isDead {
		t.Fatal("doll did not telefrag its own separate camera body")
	}
}
