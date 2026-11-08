package doomruntime

import (
	"encoding/json"
	"reflect"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestCoopCheckpointFullRosterContinuation(t *testing.T) {
	for _, keyframe := range []bool{false, true} {
		t.Run(map[bool]string{false: "save", true: "keyframe"}[keyframe], func(t *testing.T) {
			g := coopPlaybackTestGame()
			g.m.Name = "MAP01"
			g.restartTemplate.Name = "MAP01"
			template := cloneMapForRestart(g.restartTemplate)
			opts := g.opts
			opts.NewGameLoader = func(string) (*mapdata.Map, error) { return cloneMapForRestart(template), nil }
			g.opts = opts
			sg := &sessionGame{g: g, rt: g, opts: opts, current: g.m.Name}
			g.authorityPlayers[0].stats.Shells = 17
			g.authorityPlayers[0].inventory.BlueKey = true
			p := g.authorityPlayers[1]
			g.withAuthoritativePlayer(p, func() { g.isDead = true; g.playerReborn = true; g.stats.Health = 0; g.respawnCoopDemoPlayer() })
			if err := g.advanceDemoTic(); err != nil {
				t.Fatal(err)
			}
			oldBody := len(g.m.Things) - 1
			g.demoCoopSoundBodies = map[int]int{0: oldBody}
			g.demoCoopAttackers = map[int]int{1: oldBody + 1, 2: -1}
			g.withAuthoritativePlayer(g.authorityPlayers[0], func() { g.isDead = true; g.playerReborn = true; g.stats.Health = 0 })
			if !validBoomSectorTouches(captureGameSaveState(g), len(g.m.Sectors)) {
				t.Fatalf("invalid source touches: %+v things %+v", g.captureBoomSectorTouches(), g.m.Things)
			}
			before := doomrand.Capture()
			var data []byte
			var err error
			if keyframe {
				data, err = sg.marshalNetplayKeyframe()
			} else {
				data, err = sg.marshalSaveGame("co-op")
			}
			if err != nil {
				t.Fatal(err)
			}
			restoreOpts := opts
			restoreOpts.DemoScript = nil
			restored := &sessionGame{opts: restoreOpts}
			if keyframe {
				err = restored.unmarshalNetplayKeyframe(data)
			} else {
				err = restored.unmarshalSaveGame(data)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, doomrand.Capture()) {
				t.Fatal("restore changed RNG")
			}
			want, err := sg.marshalSaveGame("co-op")
			if err != nil {
				t.Fatal(err)
			}
			got, err := restored.marshalSaveGame("co-op")
			if err != nil {
				t.Fatal(err)
			}
			if string(want) != string(got) {
				t.Fatal("full roster checkpoint differs after restore")
			}
			if err := g.advanceDemoTic(); err != nil {
				t.Fatal(err)
			}
			want, err = sg.marshalSaveGame("co-op")
			if err != nil {
				t.Fatal(err)
			}
			rng := doomrand.Capture()
			doomrand.Restore(before)
			if err := restored.g.advanceDemoTic(); err != nil {
				t.Fatal(err)
			}
			got, err = restored.marshalSaveGame("co-op")
			if err != nil {
				t.Fatal(err)
			}
			if string(want) != string(got) || !reflect.DeepEqual(rng, doomrand.Capture()) {
				t.Fatal("restored co-op continuation diverged")
			}
		})
	}
}

func TestCoopCheckpointRejectsInvalidRosterWithoutMutation(t *testing.T) {
	g := coopPlaybackTestGame()
	g.m.Name = "MAP01"
	g.restartTemplate.Name = "MAP01"
	template := cloneMapForRestart(g.restartTemplate)
	opts := g.opts
	opts.NewGameLoader = func(string) (*mapdata.Map, error) { return cloneMapForRestart(template), nil }
	g.opts = opts
	sg := &sessionGame{g: g, rt: g, opts: opts, current: g.m.Name}
	data, err := sg.marshalSaveGame("valid")
	if err != nil {
		t.Fatal(err)
	}
	original, err := decodeSnapshot(data, saveGameMagic)
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]func(*coopCheckpoint){
		"cursor":           func(c *coopCheckpoint) { c.Tick = -1 },
		"missing player":   func(c *coopCheckpoint) { c.Replica.Players = c.Replica.Players[:1] },
		"duplicate player": func(c *coopCheckpoint) { c.Replica.Players[1] = c.Replica.Players[0] },
		"attacker":         func(c *coopCheckpoint) { c.Attackers = map[int]int{1: 999999} },
		"sound body":       func(c *coopCheckpoint) { c.SoundBodies = map[int]int{0: 999999} },
		"phase":            func(c *coopCheckpoint) { c.WorldDone = true },
		"carryover":        func(c *coopCheckpoint) { c.Carryover = []coopLevelCarryover{{Slot: 5}} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			c, err := decodeCoopCheckpoint(original.CoopCheckpoint)
			if err != nil {
				t.Fatal(err)
			}
			mutate(c)
			file := original
			file.CoopCheckpoint, err = json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := encodeSnapshot(saveGameMagic, file)
			if err != nil {
				t.Fatal(err)
			}
			before := doomrand.Capture()
			state := captureLiveRuntimeRoundTripState(g)
			players, err := sg.captureCoopCheckpoint()
			if err != nil {
				t.Fatal(err)
			}
			if err := sg.unmarshalSaveGame(encoded); err == nil {
				t.Fatal("accepted invalid checkpoint")
			}
			after, err := sg.captureCoopCheckpoint()
			if err != nil {
				t.Fatal(err)
			}
			if sg.g != g || string(players) != string(after) || !reflect.DeepEqual(state, captureLiveRuntimeRoundTripState(g)) || !reflect.DeepEqual(before, doomrand.Capture()) {
				t.Fatal("invalid checkpoint changed active session")
			}
		})
	}
}

func TestCoopCampaignCarriesEveryLivingPlayerAndRebirthsDeadPlayers(t *testing.T) {
	old := coopPlaybackTestGame()
	sg := &sessionGame{g: old}
	old.withAuthoritativePlayer(old.authorityPlayers[0], func() { old.stats.Health = 47; old.stats.Shells = 17; old.inventory.BlueKey = true })
	old.withAuthoritativePlayer(old.authorityPlayers[1], func() {
		old.stats.Health = 0
		old.isDead = true
		old.inventory.Weapons[2001] = true
		old.stats.Shells = 99
	})
	sg.captureCoopCarryover()
	next := newGame(cloneMapForRestart(old.restartTemplate), old.opts)
	if !sg.applyCoopCarryover(next, old) {
		t.Fatal("did not apply roster carryover")
	}
	a, b := next.authorityPlayers[0], next.authorityPlayers[1]
	if a.stats.Health != 47 || a.stats.Shells != 17 || a.inventory.BlueKey || b.stats.Health != 100 || b.stats.Shells != 0 || b.inventory.Weapons[2001] || b.isDead {
		t.Fatal("wrong inventory or rebirth across map load")
	}
	if next.thingHP[0] != 47 {
		t.Fatal("new doll did not inherit owner health")
	}
}

func TestCoopCheckpointCampaignScreens(t *testing.T) {
	for _, phase := range []string{"intermission", "finale", "worlddone"} {
		t.Run(phase, func(t *testing.T) {
			g := coopPlaybackTestGame()
			g.m.Name = "MAP01"
			g.restartTemplate.Name = "MAP01"
			template := cloneMapForRestart(g.restartTemplate)
			opts := g.opts
			opts.NewGameLoader = func(name string) (*mapdata.Map, error) {
				m := cloneMapForRestart(template)
				m.Name = mapdata.MapName(name)
				return m, nil
			}
			g.opts = opts
			sg := &sessionGame{g: g, rt: g, opts: opts, current: g.m.Name}
			next, _ := opts.NewGameLoader("MAP02")
			sg.startIntermission(next, "MAP02", true)
			if err := g.advanceDemoTic(); err != nil {
				t.Fatal(err)
			}
			sg.intermission.state.Anims = []intermissionAnimState{{intermissionAnimDef{kind: intermissionAnimAlways, period: 11, nanims: 3, x: 50, y: 20}, 17, 2}}
			sg.intermission.state.CoopShow[0][0] = 42
			if phase != "intermission" {
				sg.intermission.state.Active = false
				g.demoIntermissionActive = false
				g.demoFinaleActive = true
				g.demoFinaleCommercial = true
				sg.finale = sessionFinale{Active: true, Commercial: true, Boom: true, Accelerated: true, Tic: 89, MapName: "MAP15", Text: "story"}
			}
			if phase == "worlddone" {
				g.demoWorldDone = true
			}
			data, err := sg.marshalNetplayKeyframe()
			if err != nil {
				t.Fatal(err)
			}
			before := doomrand.Capture()
			opts.DemoScript = nil
			loaded := &sessionGame{opts: opts}
			if err := loaded.unmarshalNetplayKeyframe(data); err != nil {
				t.Fatal(err)
			}
			got, err := loaded.marshalNetplayKeyframe()
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != string(got) || !reflect.DeepEqual(before, doomrand.Capture()) {
				t.Fatal("campaign checkpoint lost screen, carryover, animation, roster, or RNG state")
			}
		})
	}
}

func TestCoopCheckpointCastDeathRestoresWithoutRunningWorldActions(t *testing.T) {
	g := coopPlaybackTestGame()
	g.m.Name = "MAP30"
	g.restartTemplate.Name = "MAP30"
	template := cloneMapForRestart(g.restartTemplate)
	opts := g.opts
	opts.NewGameLoader = func(name string) (*mapdata.Map, error) {
		m := cloneMapForRestart(template)
		m.Name = mapdata.MapName(name)
		return m, nil
	}
	g.opts = opts
	sg := &sessionGame{g: g, rt: g, opts: opts, current: g.m.Name}
	next, _ := opts.NewGameLoader("MAP31")
	sg.startIntermission(next, "MAP31", false)
	if !sg.startCommercialFinale() {
		t.Fatal("no MAP30 finale")
	}
	sg.tickFinaleAdvance(true)
	sg.tickFinaleAdvance(true)
	sg.finale.KillCast()
	for range 12 {
		sg.tickFinaleAdvance(false)
	}
	if !sg.finale.Cast.Death || sg.finale.Stage != 2 {
		t.Fatal("fixture did not reach cast death")
	}
	data, err := sg.marshalSaveGame("cast")
	if err != nil {
		t.Fatal(err)
	}
	rng := doomrand.Capture()
	opts.DemoScript = nil
	loaded := &sessionGame{opts: opts}
	if err := loaded.unmarshalSaveGame(data); err != nil {
		t.Fatal(err)
	}
	again, err := loaded.marshalSaveGame("cast")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(again) || !reflect.DeepEqual(rng, doomrand.Capture()) {
		t.Fatal("cast restore changed owned state or RNG")
	}
	for range 100 {
		sg.tickFinaleAdvance(false)
		loaded.tickFinaleAdvance(false)
	}
	if sg.finale != loaded.finale || !reflect.DeepEqual(rng, doomrand.Capture()) {
		t.Fatal("cast continuation diverged or consumed world RNG")
	}
	file, err := decodeSnapshot(data, saveGameMagic)
	if err != nil {
		t.Fatal(err)
	}
	c, err := decodeCoopCheckpoint(file.CoopCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	c.Finale.Cast.State = 999999
	file.CoopCheckpoint, _ = json.Marshal(c)
	bad, _ := encodeSnapshot(saveGameMagic, file)
	old := loaded.g
	if err := loaded.unmarshalSaveGame(bad); err == nil || loaded.g != old || !reflect.DeepEqual(rng, doomrand.Capture()) {
		t.Fatal("invalid cast checkpoint was accepted or mutated session")
	}
}
