package doomruntime

import (
	"fmt"
	"reflect"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
)

func TestPendingRebirthBinaryCheckpointContinuation(t *testing.T) {
	for _, insurance := range []bool{false, true} {
		for _, version := range []int{saveGameVersion, keyframeVersion, 39, 26} {
			t.Run(fmt.Sprintf("insurance=%t/version=%d", insurance, version), func(t *testing.T) {
				g := boomTouchFixture(t)
				g.opts.BoomGameOptions = &demo.GameOptions{Seed: 1993, Insurance: insurance}
				doomrand.ConfigureBoom(1993, insurance)
				doomrand.PRandom(doomrand.Painchance)
				doomrand.SetLogicTic(200)
				g.worldTic, g.stats.Health, g.playerMobjHealth = 200, 0, 0
				g.isDead = true
				g.stepGameplayFromDemoTic(DemoTic{Buttons: demo.ButtonUse})
				if !g.playerReborn {
					t.Fatal("Use did not request rebirth")
				}
				rng := doomrand.Capture()
				state := captureGameSaveState(g)
				blob, err := encodeSnapshot(boomSnapshotMagic(version), saveFile{Version: version, Game: state, RNG: saveRNGState{Extended: rng, MenuIndex: rng.MenuIndex, PlayIndex: rng.PlayIndex}})
				if err != nil {
					t.Fatal(err)
				}
				file, err := decodeSnapshot(blob, boomSnapshotMagic(version))
				if err != nil {
					t.Fatal(err)
				}
				loaded := boomTouchFixture(t)
				loaded.opts = g.opts
				restoreGameSaveState(loaded, file.Game)
				if version == 39 || version == 26 {
					if loaded.playerReborn || !loaded.isDead {
						t.Fatal("old format invented a restart request or lost death")
					}
					if !reflect.DeepEqual(loaded.captureBoomSectorTouches(), state.BoomSectorTouches) {
						t.Fatal("previous sector-list extension lost")
					}
					return
				}
				if !loaded.playerReborn || !loaded.isDead {
					t.Fatal("checkpoint lost pending rebirth")
				}
				doomrand.Restore(rng)
				g.stepGameplayFromDemoTic(DemoTic{})
				want, wantRNG := captureGameSaveState(g), doomrand.Capture()
				restoreSavedRNG(file)
				loaded.stepGameplayFromDemoTic(DemoTic{})
				if got := captureGameSaveState(loaded); !reflect.DeepEqual(got, want) || doomrand.Capture() != wantRNG {
					a, b := reflect.ValueOf(got), reflect.ValueOf(want)
					for i := 0; i < a.NumField(); i++ {
						if !reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
							t.Logf("field %s: got=%v want=%v", a.Type().Field(i).Name, a.Field(i), b.Field(i))
						}
					}
					t.Logf("RNG matches: %t", doomrand.Capture() == wantRNG)
					t.Fatal("rebirth continuation diverged after checkpoint")
				}
				if loaded.isDead || loaded.playerReborn || loaded.worldTic != 1 || loaded.stats.Health != 100 {
					t.Fatal("rebirth did not execute exactly once on the next tic")
				}
			})
		}
	}
}

func TestCheckpointRestoreClearsStalePendingRebirth(t *testing.T) {
	g := boomTouchFixture(t)
	state := captureGameSaveState(g)
	for _, dead := range []bool{false, true} {
		g.playerReborn = true
		state.IsDead = dead
		restoreGameSaveState(g, state)
		if g.playerReborn || g.isDead != dead {
			t.Fatal("restore retained a stale restart request")
		}
	}
}

func TestPendingRebirthAuthorityViewersAndValidation(t *testing.T) {
	a, client, _ := snapshotFixture(t)
	a.players[2].isDead, a.players[2].playerReborn = true, true
	a.players[2].stats.Health, a.players[2].playerMobjHealth = 0, 0
	states, err := a.SnapshotBatch([]byte{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	for _, viewer := range []byte{1, 2} {
		if err := client.applyAuthoritySnapshot(states[viewer]); err != nil {
			t.Fatal(err)
		}
		if client.playerReborn != (viewer == 2) {
			t.Fatal("viewer inherited another player's rebirth request")
		}
	}
	r, err := decodeAuthorityReplica(states[2])
	if err != nil {
		t.Fatal(err)
	}
	r.Game.IsDead = false
	if err := validateAuthorityReplica(client, r); err == nil {
		t.Fatal("accepted pending rebirth for a living player")
	}
}

func TestPendingRebirthDecoderRejectsLivingPlayer(t *testing.T) {
	blob, err := encodeSnapshot(saveGameMagic, saveFile{Version: saveGameVersion, Game: gameSaveState{PlayerReborn: true}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeSnapshot(blob, saveGameMagic); err == nil {
		t.Fatal("accepted a restart request without death")
	}
}

func TestDeadPlayerCheckpointRetainsMovingAttacker(t *testing.T) {
	for _, thing := range []int{-1, 1} {
		for _, version := range []int{saveGameVersion, keyframeVersion, 39, 26} {
			g := boomTouchFixture(t)
			g.isDead, g.stats.Health = true, 0
			g.statusHasAttacker, g.statusAttackerThing = true, thing
			g.statusAttackerX, g.statusAttackerY = 32*fracUnit, 96*fracUnit
			blob, err := encodeSnapshot(boomSnapshotMagic(version), saveFile{Version: version, Game: captureGameSaveState(g)})
			if err != nil {
				t.Fatal(err)
			}
			file, err := decodeSnapshot(blob, boomSnapshotMagic(version))
			if err != nil {
				t.Fatal(err)
			}
			loaded := boomTouchFixture(t)
			restoreGameSaveState(loaded, file.Game)
			if version == 39 || version == 26 {
				if loaded.statusHasAttacker || loaded.statusAttackerThing != -1 {
					t.Fatal("old format invented an attacker")
				}
				continue
			}
			for tic := range 40 {
				if thing >= 0 {
					g.setThingPosFixed(thing, -8*fracUnit, int64(96-tic)*fracUnit)
					loaded.setThingPosFixed(thing, -8*fracUnit, int64(96-tic)*fracUnit)
				}
				g.tickDeadPlayerTurnTowardAttacker()
				loaded.tickDeadPlayerTurnTowardAttacker()
				if g.p.angle != loaded.p.angle {
					t.Fatal("death camera lost the live attacker reference")
				}
			}
		}
	}
}

func TestBoomFinaleDemoInputUsesAttackAndUseEdges(t *testing.T) {
	for _, boom := range []bool{false, true} {
		g := boomTouchFixture(t)
		g.boomMomentumDropoff = boom
		g.weaponAttackDown, g.useButtonDown = false, false
		g.demoFinaleActive, g.demoFinaleCommercial = true, true
		g.opts.DemoScript = &demo.Script{Tics: []demo.Tic{{Buttons: demo.ButtonAttack}, {Buttons: demo.ButtonAttack}, {}, {Buttons: demo.ButtonUse}, {Buttons: demo.ButtonUse}}}
		for tic, edge := range []bool{true, false, false, true, false} {
			if err := g.advanceDemoTic(); err != nil {
				t.Fatal(err)
			}
			if g.demoIntermissionSkip != (boom && edge) {
				t.Fatalf("boom=%t tic=%d: wrong finale acceleration edge", boom, tic)
			}
			g.demoIntermissionSkip = false
		}
	}
}
