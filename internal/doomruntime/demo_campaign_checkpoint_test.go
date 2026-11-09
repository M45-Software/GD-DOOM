package doomruntime

import (
	"encoding/json"
	"reflect"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func singleCampaignCheckpointSession(t *testing.T, phase string) *sessionGame {
	t.Helper()
	base := predictionTestMap()
	base.Name = "MAP06"
	next := cloneMapForRestart(base)
	next.Name = "MAP07"
	settings := demo.GameOptions{Seed: 1993, Insurance: true, NoMonsters: true, MonstersRemember: true}
	script := &DemoScript{Header: DemoHeader{Version: 202, Skill: 2, Episode: 1, Map: 6, NoMonsters: true, PlayerInGame: [4]bool{true}, BoomOptions: settings.Bytes()}, Tics: make([]DemoTic, 200)}
	opts := Options{Headless: true, DemoScript: script, NewGameLoader: func(name string) (*mapdata.Map, error) {
		m := base
		if name == "MAP07" {
			m = next
		}
		return cloneMapForRestart(m), nil
	}}
	rt, _ := NewRuntime(base, opts, func(mapdata.MapName, bool) (*mapdata.Map, mapdata.MapName, error) {
		return cloneMapForRestart(next), next.Name, nil
	})
	sg := rt.(*sessionGame)
	sg.g.demoTick = 3
	sg.g.worldTic = 123
	sg.g.stats.Health = 73
	sg.g.useButtonDown = false
	sg.g.currentMoveCmd = moveCmd{forward: 50, side: -25}
	sg.g.playerMobjState, sg.g.playerMobjTics = 150, 2
	sg.g.inventory.Weapons[2001] = true
	sg.g.inventory.BlueKey = true
	if phase != "level" {
		sg.g.levelExitRequested = true
		sg.startIntermission(next, next.Name, false)
		if err := sg.g.updateDemoIntermission(script); err != nil {
			t.Fatal(err)
		}
		sg.intermission.state.Bcnt = 7
		sg.intermission.state.Tic = 7
		sg.intermission.state.Show.KillsPct = 17
		if phase == "finale" || phase == "worlddone" {
			if !sg.startCommercialFinale() {
				t.Fatal("missing finale")
			}
			sg.finale.Tic = 27
			sg.finale.Accelerated = true
		}
		if phase == "worlddone" {
			sg.g.demoWorldDone = true
		}
	}
	return sg
}

func TestSingleCampaignColdLoadRebirthAndLivingViewRecovery(t *testing.T) {
	for _, dead := range []bool{false, true} {
		for _, keyframe := range []bool{false, true} {
			name := map[bool]string{false: "living", true: "dead"}[dead] +
				map[bool]string{false: "/save", true: "/keyframe"}[keyframe]
			t.Run(name, func(t *testing.T) {
				sg := singleCampaignCheckpointSession(t, "level")
				g := sg.g
				g.p.deltaViewHeight = -5 * fracUnit / 8
				g.weaponAttackDown, g.useButtonDown = false, false
				g.inventory.ReadyWeapon = weaponShotgun
				g.stats.Shells = 17
				if dead {
					g.stats.Health, g.isDead = 0, true
				}
				g.syncPlayerMobjHealth()
				next, err := sg.opts.NewGameLoader("MAP07")
				if err != nil {
					t.Fatal(err)
				}
				sg.startIntermission(next, next.Name, false)
				var data []byte
				if keyframe {
					data, err = sg.marshalNetplayKeyframe()
				} else {
					data, err = sg.marshalSaveGame("transition")
				}
				if err != nil {
					t.Fatal(err)
				}
				opts := sg.opts
				opts.DemoScript = nil
				restored := &sessionGame{opts: opts, nextMap: sg.nextMap}
				if keyframe {
					err = restored.unmarshalNetplayKeyframe(data)
				} else {
					err = restored.unmarshalSaveGame(data)
				}
				if err != nil {
					t.Fatal(err)
				}
				restored.finishIntermission()
				g = restored.g
				if dead {
					if g.stats.Health != 100 || g.stats.Bullets != 50 || g.stats.Shells != 0 ||
						g.inventory.ReadyWeapon != weaponPistol || !g.weaponAttackDown || !g.useButtonDown || g.p.deltaViewHeight != 0 {
						t.Fatal("dead player did not rebirth with pistol defaults and held-button suppression")
					}
				} else if g.stats.Health != 73 || g.stats.Shells != 17 ||
					g.inventory.ReadyWeapon != weaponShotgun || g.weaponAttackDown || g.useButtonDown || g.p.deltaViewHeight != -5*fracUnit/8 {
					t.Fatal("living player lost carried inventory, latches or view recovery")
				}
			})
		}
	}
}

func TestSingleCampaignColdCheckpointContinuation(t *testing.T) {
	for _, phase := range []string{"level", "intermission", "finale", "worlddone"} {
		for _, keyframe := range []bool{false, true} {
			t.Run(phase+map[bool]string{false: "/save", true: "/keyframe"}[keyframe], func(t *testing.T) {
				sg := singleCampaignCheckpointSession(t, phase)
				var data []byte
				var err error
				if keyframe {
					data, err = sg.marshalNetplayKeyframe()
				} else {
					data, err = sg.marshalSaveGame("campaign")
				}
				if err != nil {
					t.Fatal(err)
				}
				before := doomrand.Capture()
				opts := sg.opts
				opts.DemoScript = nil
				restored := &sessionGame{opts: opts, nextMap: sg.nextMap}
				if keyframe {
					err = restored.unmarshalNetplayKeyframe(data)
				} else {
					err = restored.unmarshalSaveGame(data)
				}
				if err != nil {
					t.Fatal(err)
				}
				if restored.g.opts.GameMode != gameModeSingle || len(restored.g.authorityPlayers) != 0 || restored.g.demoTick != sg.g.demoTick || !reflect.DeepEqual(before, doomrand.Capture()) {
					t.Fatal("restore changed single-player rules, cursor or RNG")
				}
				want, err := sg.marshalSaveGame("campaign")
				if err != nil {
					t.Fatal(err)
				}
				got, err := restored.marshalSaveGame("campaign")
				if err != nil {
					t.Fatal(err)
				}
				if string(want) != string(got) {

					t.Fatal("cold checkpoint lost campaign state")
				}
				// Continue independent sessions from the same RNG state, across a queued
				// next-map load as well as interior intermission/finale commands.
				for n := 0; n < 12; n++ {
					if err := sg.Update(); err != nil {
						t.Fatal(err)
					}
				}
				want, err = sg.marshalSaveGame("campaign")
				if err != nil {
					t.Fatal(err)
				}
				after := doomrand.Capture()
				doomrand.Restore(before)
				for n := 0; n < 12; n++ {
					if err := restored.Update(); err != nil {
						t.Fatal(err)
					}
				}
				got, err = restored.marshalSaveGame("campaign")
				if err != nil {
					t.Fatal(err)
				}
				if string(want) != string(got) || !reflect.DeepEqual(after, doomrand.Capture()) {
					t.Fatal("cold continuation diverged")
				}
			})
		}
	}
}

func TestSingleCampaignRejectsMalformedCheckpointWithoutMutation(t *testing.T) {
	sg := singleCampaignCheckpointSession(t, "intermission")
	data, err := sg.marshalSaveGame("valid")
	if err != nil {
		t.Fatal(err)
	}
	original, err := decodeSnapshot(data, saveGameMagic)
	if err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*coopCheckpoint){
		"missing activation": func(c *coopCheckpoint) { c.SinglePlayer = nil },
		"activation health":  func(c *coopCheckpoint) { c.SinglePlayer.PlayerMobjHealth++ },
		"activation attacker": func(c *coopCheckpoint) {
			c.SinglePlayer.StatusHasAttacker = true
			c.SinglePlayer.StatusAttackerThing = 9999
		},
		"missing carryover": func(c *coopCheckpoint) { c.LevelCarryover = nil },
		"cursor":            func(c *coopCheckpoint) { c.Tick = 201 },
		"wrong mode":        func(c *coopCheckpoint) { c.Single = false },
		"co-op carryover":   func(c *coopCheckpoint) { c.Carryover = []coopLevelCarryover{{Slot: 1}} },
		"old attachment":    func(c *coopCheckpoint) { c.Version = 2 },
	} {
		t.Run(name, func(t *testing.T) {
			file := original
			c, err := decodeCoopCheckpoint(file.CoopCheckpoint)
			if err != nil {
				t.Fatal(err)
			}
			change(c)
			file.CoopCheckpoint, err = json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			bad, err := encodeSnapshot(saveGameMagic, file)
			if err != nil {
				t.Fatal(err)
			}
			before := doomrand.Capture()
			g := sg.g
			if err := sg.unmarshalSaveGame(bad); err == nil {
				t.Fatal("accepted malformed checkpoint")
			}
			if sg.g != g || !reflect.DeepEqual(before, doomrand.Capture()) {
				t.Fatal("rejected restore changed session")
			}
		})
	}
}

func TestSingleCampaignEmptyWeaponMapAndUseLatch(t *testing.T) {
	sg := singleCampaignCheckpointSession(t, "level")
	sg.g.inventory.Weapons[2001] = false
	sg.g.useButtonDown = false
	data, err := sg.marshalSaveGame("empty inventory")
	if err != nil {
		t.Fatal(err)
	}
	opts := sg.opts
	opts.DemoScript = nil
	restored := &sessionGame{opts: opts, nextMap: sg.nextMap}
	if err := restored.unmarshalSaveGame(data); err != nil {
		t.Fatal(err)
	}
	if restored.g.useButtonDown || restored.g.currentMoveCmd != sg.g.currentMoveCmd || restored.g.playerMobjState != sg.g.playerMobjState || restored.g.playerMobjTics != sg.g.playerMobjTics {
		t.Fatal("restore lost local player activation fields")
	}
}

func TestSingleCampaignRestoresSpawnedThingFrictionAndSelfAttacker(t *testing.T) {
	sg := singleCampaignCheckpointSession(t, "level")
	g := sg.g
	g.thingFriction = make([]int64, len(g.m.Things))
	g.thingMoveFactor = make([]int64, len(g.m.Things))
	idx := g.appendRuntimeThing(mapdata.Thing{Type: 2007, X: 64, Flags: 7}, true)
	g.thingFriction[idx] = 59392
	g.thingMoveFactor[idx] = 2048
	g.statusHasAttacker = true
	g.statusAttackerThing = selfAttackerThing
	data, err := sg.marshalSaveGame("spawned friction")
	if err != nil {
		t.Fatal(err)
	}
	opts := sg.opts
	opts.DemoScript = nil
	restored := &sessionGame{opts: opts, nextMap: sg.nextMap}
	if err := restored.unmarshalSaveGame(data); err != nil {
		t.Fatal(err)
	}
	if len(restored.g.thingFriction) != len(restored.g.m.Things) || restored.g.thingFriction[idx] != 59392 || restored.g.thingMoveFactor[idx] != 2048 {
		t.Fatal("restore lost spawned actor friction")
	}
	restored.g.p.x += 17 * fracUnit
	x, y := restored.g.statusAttackerPosition()
	if x != restored.g.p.x || y != restored.g.p.y {
		t.Fatal("saved self attacker no longer follows its physical body")
	}
}
