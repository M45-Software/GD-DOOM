package doomruntime

import (
	"fmt"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func recordedBarrelFixture(t *testing.T, health int) (*game, *authoritativePlayerState, *authoritativePlayerState, int) {
	t.Helper()
	before := doomrand.Capture()
	t.Cleanup(func() { doomrand.Restore(before) })
	g, victim, shooter := authoritativeTargetsTestGame()
	g.opts.GameMode = gameModeCoop
	g.opts.DemoScript = &demo.Script{Header: demo.Header{Version: 202, PlayerInGame: [4]bool{true, true}}, Frames: make([][4]demo.Tic, 1)}
	g.boomMomentumDropoff = true
	g.authorityRules = &authorityRulesState{Config: AuthorityRules{FriendlyFire: true}}
	g.authorityRules.Scores[1].Generation, g.authorityRules.Scores[2].Generation = 1, 1
	g.m.Things[0].Type, g.thingHP[0] = barrelThingType, 20
	g.setThingPosFixed(0, 96*fracUnit, 0)
	g.p.x, g.p.y = 0, 0
	g.stats.Health, g.playerMobjHealth = health, health
	*victim = g.captureAuthoritativePlayer()
	shooter.p.x = -512 * fracUnit
	cyber := g.appendRuntimeThing(mapdata.Thing{Type: 16, X: 512}, false)
	g.thingHP[cyber] = 4000
	return g, victim, shooter, cyber
}

func TestRecordedBarrelLethalFirstHitHasNoShooter(t *testing.T) {
	for _, health := range []int{200, 40} {
		t.Run(fmt.Sprint(health), func(t *testing.T) {
			g, victim, _, cyber := recordedBarrelFixture(t, health)
			g.damageBarrelFrom(0, 30, false, cyber, 0, 0, false, 0, false)
			g.radiusAttackFromThing(0, 128)
			if victim.statusHasAttacker || victim.p.momx >= 0 {
				t.Fatal("unattributed explosion must retain thrust without an attacker")
			}
			if health == 200 && victim.stats.Health != 152 {
				t.Fatalf("blast health = %d, want 152", victim.stats.Health)
			}
			// Doom stores a positive diagonal count and subtracts it when
			// calculating the player's net frag total.
			if health == 40 && (!victim.isDead || g.demoCoopFrags[0][0] != 1) {
				t.Fatal("unattributed lethal blast must record the Doom self-frag")
			}
		})
	}
}

func TestRecordedBarrelRetainsFirstNonlethalTarget(t *testing.T) {
	g, victim, shooter, cyber := recordedBarrelFixture(t, 200)
	g.withAuthoritativePlayer(shooter, func() { g.damageBarrelFrom(0, 5, true, -1, 0, 0, false, 0, false) })
	g.damageBarrelFrom(0, 1, false, cyber, 0, 0, false, 0, false)
	g.damageBarrelFrom(0, 30, false, cyber, 0, 0, false, 0, false)
	g.radiusAttackFromThing(0, 128)
	if !victim.statusHasAttacker || g.demoCoopAttackers[1] != -2 || victim.stats.Health != 152 {
		t.Fatal("later hits replaced the barrel's retained player target")
	}
}

func TestIgnoredSplashDoesNotReplaceExistingAttacker(t *testing.T) {
	g, victim, _, cyber := recordedBarrelFixture(t, 200)
	g.invulnerable = true
	g.statusHasAttacker, g.statusAttackerThing = true, -1
	g.statusAttackerX, g.statusAttackerY = 123, 456
	*victim = g.captureAuthoritativePlayer()
	g.radiusAttackAt(96*fracUnit, 0, 0, 42*fracUnit, 0, 128, "blast", false, cyber)
	if victim.stats.Health != 200 || victim.statusAttackerX != 123 || victim.statusAttackerY != 456 {
		t.Fatal("ignored splash changed health or attacker identity")
	}
	if victim.p.momx >= 0 {
		t.Fatal("invulnerability must still permit explosion thrust")
	}
}
