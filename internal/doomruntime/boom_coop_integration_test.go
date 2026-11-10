//go:build integration

package doomruntime

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/launchcatalog"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
)

func TestBoomCoopDemoCapture(t *testing.T) {
	path := os.Getenv("GD_COOP_OUT")
	if path == "" {
		t.Skip("run verify_boom_coop.py")
	}
	script, err := demo.Load(os.Getenv("GD_COOP_DEMO"))
	if err != nil {
		t.Fatal(err)
	}
	wf, err := wad.Open(os.Getenv("GD_COOP_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	mapName := os.Getenv("GD_COOP_MAP")
	if mapName == "" {
		mapName = fmt.Sprintf("MAP%02d", script.Header.Map)
	}
	m, err := mapdata.LoadMap(wf, mapdata.MapName(mapName))
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{Headless: true, DemoScript: script, DemoQuitOnComplete: true, NewGameLoader: func(name string) (*mapdata.Map, error) { return mapdata.LoadMap(wf, mapdata.MapName(name)) }}
	if os.Getenv("GD_DEHACKED_CAPTURE") == "1" {
		var paths []string
		if value := os.Getenv("GD_DEHACKED_FILES"); value != "" {
			paths = strings.Split(value, ",")
		}
		defs, _, err := launchcatalog.LoadDehacked(wf, paths, os.Getenv("GD_DEHACKED_NO_EMBEDDED") == "1")
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateDehackedDefinitions(defs); err != nil {
			t.Fatal(err)
		}
		opts.DehackedDefinitions = defs
	}
	// Texture dimensions affect simulation even without a renderer: Boom
	// movers can use the shortest adjacent lower or upper texture as a target.
	// Generated geometry-only fixtures deliberately have no texture lumps.
	if _, present := wf.LumpByName("TEXTURE1"); present {
		textures, err := doomtex.LoadFromWAD(wf)
		if err != nil {
			t.Fatal(err)
		}
		opts.WallTextureHeights = make(map[string]int)
		for i, name := range textures.TextureNames() {
			tex, _ := textures.Texture(name)
			opts.WallTextureHeights[name] = tex.Height
			if i == 0 {
				opts.WallTextureHeights["-"] = tex.Height
				opts.WallTextureZeroName = name
			}
		}
	}
	var g *game
	var sg *sessionGame
	campaign := os.Getenv("GD_COOP_CAMPAIGN") != ""
	if campaign {
		rt, meta := NewRuntime(m, opts, func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error) {
			name, err := mapdata.NextMapName(wf, current, secret)
			if err != nil {
				return nil, "", err
			}
			next, err := mapdata.LoadMap(wf, name)
			return next, name, err
		})
		defer meta.Close()
		sg = rt.(*sessionGame)
		g = sg.g
	} else {
		g = newGame(m, opts)
		sg = &sessionGame{g: g, rt: g, opts: opts, current: m.Name}
	}
	start := 0
	if path := os.Getenv("GD_COOP_LOAD_SAVE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fresh := &sessionGame{opts: opts, nextMap: sg.nextMap, gameFactory: sg.gameFactory}
		if script.Header.PlayerCount() > 1 {
			fresh.opts.DemoScript = nil
		}
		if err := fresh.unmarshalSaveGame(data); err != nil {
			t.Fatal(err)
		}
		sg, g = fresh, fresh.g
		original, err := demo.Format(script)
		if err != nil {
			t.Fatal(err)
		}
		restored, err := demo.Format(g.opts.DemoScript)
		if err != nil || !bytes.Equal(original, restored) {
			t.Fatal("diagnostic checkpoint contains a different command stream")
		}
		start = g.demoTick
	}
	dumpCommand := 0
	if value := os.Getenv("GD_COOP_DUMP_COMMAND"); value != "" {
		dumpCommand, err = strconv.Atoi(value)
		if err != nil || dumpCommand <= start {
			t.Fatal("diagnostic save command must follow the replay start")
		}
	}
	checkpoints := map[int]bool{}
	for _, v := range strings.Split(os.Getenv("GD_COOP_CHECKPOINT_TICS"), ",") {
		if v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				t.Fatal(err)
			}
			checkpoints[n] = true
		}
	}
	finaleMode := os.Getenv("GD_COOP_FINALE") != ""
	castKeys := map[int]bool{}
	for _, v := range strings.Split(os.Getenv("GD_COOP_CAST_KEYS"), ",") {
		if v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				t.Fatal(err)
			}
			castKeys[n] = true
		}
	}
	summaryWritten := false
	restored := 0
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := bufio.NewWriter(f)
	defer out.Flush()
	compact := os.Getenv("GD_COOP_COMPACT") != "" && !finaleMode
	lineStateEnabled := os.Getenv("GD_COOP_LINES") != ""
	var lineState []byte
	detailCommand := 0
	if value := os.Getenv("GD_COOP_DETAIL_COMMAND"); value != "" {
		detailCommand, err = strconv.Atoi(value)
		if err != nil || detailCommand < 1 || !compact {
			t.Fatal("GD_COOP_DETAIL_COMMAND requires compact mode and a positive command number")
		}
	}
	var row bytes.Buffer
	rowOut := bufio.NewWriter(&row)
	if campaign {
		fmt.Fprint(out, "demotic,map,")
	}
	if finaleMode {
		fmt.Fprintln(out, "phase,count,stage,accelerated,castnum,caststate,casttics,castframes,melee,attacking,death,players,playindex,seeds")
	} else if compact {
		fmt.Fprintln(out, "tic,state_md5")
	} else {
		fmt.Fprint(out, "tic,players,bodies,items,sectors")
		if lineStateEnabled {
			fmt.Fprint(out, ",line_md5")
		}
		fmt.Fprintln(out, ",playindex,seeds")
	}
	var phases *bufio.Writer
	if path := os.Getenv("GD_COOP_PHASE_OUT"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		phases = bufio.NewWriter(f)
		defer phases.Flush()
		fmt.Fprintln(phases, "demotic,map,phase,count,stage,accelerated,castnum,caststate,casttics,castframes,melee,attacking,death,players,playindex,seeds,wi_screen,wi_bcnt,wi_cnt,wi_state,wi_pause,wi_accelerate,wi_counts,wi_time,wi_par")
	}
	frames := len(script.Frames)
	exportDir := os.Getenv("GD_COOP_EXPORT_SAVES")
	if exportDir != "" {
		if err := os.MkdirAll(exportDir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	lastPhase := -1
	if script.Header.PlayerCount() == 1 {
		frames = len(script.Tics)
	}
	if value := os.Getenv("GD_COOP_CAPTURE_COMMANDS"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit <= start || limit > frames {
			t.Fatal("invalid diagnostic command limit")
		}
		frames = limit
	}
	if dumpCommand > frames {
		t.Fatal("diagnostic save command exceeds the capture limit")
	}
	for tic := start; tic < frames; tic++ {
		oldMap, oldWorld := g.m.Name, g.worldTic
		if campaign {
			for updates := 0; ; updates++ {
				if err := sg.Update(); err != nil {
					t.Fatal(err)
				}
				g = sg.g
				if g.demoTick == tic+1 {
					break
				}
				if updates >= 8 {
					t.Fatal("stalled campaign")
				}
			}
		} else if err := g.advanceDemoTic(); err != nil {
			t.Fatal(err)
		}
		if tic+1 == dumpCommand {
			data, err := sg.marshalSaveGame("diagnostic")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path+".save", data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		if !summaryWritten && g.levelExitRequested && os.Getenv("GD_COOP_SUMMARY") != "" {
			summary := struct {
				Completed    bool             `json:"completed"`
				Map          string           `json:"map"`
				Leveltime    int              `json:"leveltime"`
				TotalKills   int              `json:"total_kills"`
				TotalItems   int              `json:"total_items"`
				TotalSecrets int              `json:"total_secrets"`
				Players      []map[string]int `json:"players"`
			}{Completed: true, Map: string(g.m.Name), Leveltime: g.worldTic, TotalKills: g.levelKillsTotal, TotalItems: g.levelItemsTotal, TotalSecrets: g.secretsTotal}
			for _, p := range coopProbePlayers(g) {
				g.withAuthoritativePlayer(p, func() {
					summary.Players = append(summary.Players, map[string]int{"slot": g.localSlot, "health": g.stats.Health, "armor": g.stats.Armor, "bullets": g.stats.Bullets, "shells": g.stats.Shells, "cells": g.stats.Cells, "rockets": g.stats.Rockets, "kills": g.playerKillCount, "items": g.playerItemCount, "secrets": g.secretsFound})
				})
			}
			data, err := json.Marshal(summary)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(os.Getenv("GD_COOP_SUMMARY"), data, 0600); err != nil {
				t.Fatal(err)
			}
			summaryWritten = true
		}
		if castKeys[tic+1] {
			sg.finale.KillCast()
		}
		if checkpoints[tic] {
			var data []byte
			var err error
			if os.Getenv("GD_COOP_CHECKPOINT_FORMAT") == "keyframe" {
				data, err = sg.marshalNetplayKeyframe()
			} else {
				data, err = sg.marshalSaveGame("co-op")
			}
			if err != nil {
				t.Fatal(err)
			}
			if os.Getenv("GD_COOP_COLD_RESTORE") != "" {
				fresh := &sessionGame{opts: opts, nextMap: sg.nextMap, gameFactory: sg.gameFactory}
				if script.Header.PlayerCount() > 1 {
					fresh.opts.DemoScript = nil
				}
				sg = fresh
			}
			if os.Getenv("GD_COOP_CHECKPOINT_FORMAT") == "keyframe" {
				err = sg.unmarshalNetplayKeyframe(data)
			} else {
				err = sg.unmarshalSaveGame(data)
			}
			if err != nil {
				t.Fatalf("checkpoint %d: %v", tic, err)
			}
			g = sg.g
			restored++
		}
		phase := 0
		if g.m.Name == oldMap && g.worldTic == oldWorld {
			if sg.finale.Active {
				phase = 2
			} else if sg.intermission.state.Active {
				phase = 1
			}
		}
		if exportDir != "" && (g.m.Name != oldMap || phase != lastPhase) {
			data, err := sg.marshalSaveGame("campaign diagnostic")
			if err != nil {
				t.Fatal(err)
			}
			name := fmt.Sprintf("%08d-%s-phase%d.save", tic+1, g.m.Name, phase)
			if err := os.WriteFile(filepath.Join(exportDir, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		lastPhase = phase
		if phases != nil {
			fmt.Fprintf(phases, "%d,%s,%d,", tic+1, g.m.Name, phase)
			writeRecordedPhaseFields(phases, sg, phase, true)
			if compact {
				if err := phases.Flush(); err != nil {
					t.Fatal(err)
				}
			}
		}
		if finaleMode {
			phase := 0
			if g.m.Name == oldMap && g.worldTic == oldWorld {
				if sg.finale.Active {
					phase = 2
				} else if sg.intermission.state.Active {
					phase = 1
				}
			}
			fmt.Fprintf(out, "%d,%s,%d,", tic+1, g.m.Name, phase)
			writeCoopFinaleFields(out, sg, phase)
			continue
		}
		if campaign && g.m.Name == oldMap && g.worldTic == oldWorld {
			continue
		}
		destination := out
		if compact {
			row.Reset()
			rowOut.Reset(&row)
			out = rowOut
		}
		if campaign {
			fmt.Fprintf(out, "%d,%s,", tic+1, g.m.Name)
		}
		fmt.Fprintf(out, "%d,", g.worldTic-1)
		for _, p := range coopProbePlayers(g) {
			g.withAuthoritativePlayer(p, func() {
				if tic+1 == detailCommand {
					fmt.Printf("co-op attacker slot=%d has=%v thing=%d roster_ref=%d cached=(%d,%d) damage=%d\n", g.localSlot, g.statusHasAttacker, g.statusAttackerThing, g.demoCoopAttackers[g.localSlot], g.statusAttackerX, g.statusAttackerY, g.statusDamageCount)
				}
				state := 0
				if g.playerReborn {
					state = 2
				} else if g.isDead {
					state = 1
				}
				pending := 10
				if g.inventory.PendingWeapon != 0 {
					pending = demoTraceWeaponID(g.inventory.PendingWeapon)
				}
				cards := 0
				for i, on := range g.inventory.cardKeys() {
					if on {
						cards |= 1 << i
					}
				}
				for i, on := range g.inventory.SkullKeys {
					if on {
						cards |= 1 << (i + 3)
					}
				}
				fmt.Fprintf(out, "%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d", g.localSlot, g.p.x, g.p.y, g.p.z, g.p.angle, g.p.momx, g.p.momy, g.p.momz, g.stats.Health, g.playerMobjHealth, g.stats.Armor, g.stats.Bullets, g.stats.Shells, g.stats.Cells, g.stats.Rockets, state, g.p.viewHeight, g.playerViewZ, demoTraceWeaponID(g.inventory.ReadyWeapon), pending, weaponDoomStateID(g.weaponState), g.weaponStateTics, boolToInt(g.weaponAttackDown), boolToInt(g.weaponRefire), cards)
				ax, ay := int64(0), int64(0)
				if g.statusHasAttacker {
					ax, ay = coopProbeAttackerPosition(g)
				}
				fmt.Fprintf(out, ":%d:%d;", ax, ay)
			})
		}
		fmt.Fprint(out, ",")
		mobjs := g.demoTraceMobjs()
		var bodyIndices map[int]int
		bodyNumber := 0
		if tic+1 == detailCommand {
			bodyIndices = make(map[int]int)
		}
		for _, m := range mobjs {
			if m.Player != 0 || m.Flags&1 != 0 || os.Getenv("GD_COOP_COMBAT_ONLY") != "" && (m.Type > 42 || m.Type == 41) {
				continue
			}
			if bodyIndices != nil {
				if m.Kind == "monster" {
					bodyIndices[bodyNumber] = m.thingIndex
				}
				bodyNumber++
			}
			owner := m.ownerSlot
			fmt.Fprintf(out, "%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d", m.Type, m.X, m.Y, m.Z, m.Angle, m.MomX, m.MomY, m.MomZ, m.Health, m.Tics, m.State, m.Radius, m.Height, m.FloorZ, m.CeilingZ, owner)
			if m.Kind == "monster" && g.thingCountsKill(g.m.Things[m.thingIndex].Type) {
				idx := m.thingIndex
				tx, ty, _, _, _, _ := g.monsterAttackTargetPos(idx)
				targetSlot := g.monsterTargetPlayerSlot(idx)
				if len(g.authorityPlayers) == 0 && g.thingTargetPlayer[idx] {
					targetSlot = g.localSlot
				}
				fmt.Fprintf(out, ":%d:%d:%d:%d:%d:%d:%d:%d:%d:%d", targetSlot, tx, ty, m.ReactionTime, m.Threshold, m.Movedir, m.Movecount, m.LastLook, boolToInt(g.thingJustHit[idx]), boolToInt(g.thingJustAtk[idx]))
			}
			fmt.Fprint(out, ";")
		}
		fmt.Fprint(out, ",")
		for _, m := range mobjs {
			if m.Flags&1 != 0 {
				fmt.Fprintf(out, "%d:%d:%d:%d;", m.Type, m.X, m.Y, m.Z)
			}
		}
		fmt.Fprint(out, ",")
		for i, s := range g.m.Sectors {
			fmt.Fprintf(out, "%d:%d:%d;", g.sectorFloor[i], g.sectorCeil[i], s.Light)
		}
		if lineStateEnabled {
			if len(lineState) != len(g.lineSpecial)*2 {
				lineState = make([]byte, len(g.lineSpecial)*2)
			}
			for i, special := range g.lineSpecial {
				lineState[i*2], lineState[i*2+1] = byte(special), byte(special>>8)
			}
			fmt.Fprintf(out, ",%x", md5.Sum(lineState))
		}
		r := doomrand.Capture()
		fmt.Fprintf(out, ",%d,", r.PlayIndex)
		for cls, seed := range r.Seeds {
			if doomrand.Class(cls) != doomrand.Misc {
				fmt.Fprintf(out, "%d;", seed)
			}
		}
		fmt.Fprintln(out)
		if compact {
			if err := out.Flush(); err != nil {
				t.Fatal(err)
			}
			out = destination
			if tic+1 == detailCommand {
				if err := os.WriteFile(path+".detail.csv", row.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
				indices, err := json.Marshal(bodyIndices)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path+".bodies.json", indices, 0600); err != nil {
					t.Fatal(err)
				}
				if lineStateEnabled {
					lines, err := json.Marshal(g.lineSpecial)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path+".lines.json", lines, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if campaign {
				fmt.Fprintf(out, "%d,%s,", tic+1, g.m.Name)
			}
			// Hash the complete existing CSV row, including its newline. This
			// reduces campaign trace storage without sampling world state.
			fmt.Fprintf(out, "%d,%x\n", g.worldTic-1, md5.Sum(row.Bytes()))
			if err := out.Flush(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if restored != len(checkpoints) {
		t.Fatal("not all checkpoints restored")
	}
}

func coopProbeAttackerPosition(g *game) (int64, int64) {
	if !g.statusHasAttacker {
		return 0, 0
	}
	// GD_LiveAttacker in the reference observes only current live thinkers.
	// Removed barrels can leave a dangling player->attacker pointer in Boom;
	// its freed coordinates are deliberately outside this comparison profile.
	if g.statusAttackerThing == selfAttackerThing {
		return g.statusAttackerPosition()
	}
	thing := g.statusAttackerThing
	if g.coopDemoActive() {
		if ref := g.demoCoopAttackers[g.localSlot]; ref < 0 {
			return g.statusAttackerPosition()
		} else if ref > 0 {
			thing = ref - 1
		}
	}
	if thing >= 0 && thing < len(g.thingCollected) && g.thingCollected[thing] {
		return 0, 0
	}
	return g.statusAttackerPosition()
}

func TestBoomCoopProbeRemovedAttacker(t *testing.T) {
	g := &game{statusHasAttacker: true, statusAttackerThing: 0, statusAttackerX: 123, statusAttackerY: 456, thingCollected: []bool{false}}
	if x, y := coopProbeAttackerPosition(g); x != 123 || y != 456 {
		t.Fatalf("present attacker = %d,%d", x, y)
	}
	g.thingCollected[0] = true
	if x, y := coopProbeAttackerPosition(g); x != 0 || y != 0 {
		t.Fatalf("removed attacker = %d,%d", x, y)
	}
	g.opts.GameMode = gameModeCoop
	g.opts.DemoScript = &demo.Script{Frames: make([][4]demo.Tic, 1)}
	g.localSlot = 1
	g.demoCoopAttackers = map[int]int{1: 1}
	g.statusAttackerThing = selfAttackerThing
	g.p.x, g.p.y = 789, 987
	if x, y := coopProbeAttackerPosition(g); x != 789 || y != 987 {
		t.Fatalf("self attacker = %d,%d", x, y)
	}
	g.statusHasAttacker = false
	if x, y := coopProbeAttackerPosition(g); x != 0 || y != 0 {
		t.Fatalf("absent attacker retained cached coordinates = %d,%d", x, y)
	}
}

func coopProbePlayers(g *game) []*authoritativePlayerState {
	if len(g.authorityPlayers) != 0 {
		return g.authorityPlayers
	}
	p := g.captureAuthoritativePlayer()
	return []*authoritativePlayerState{&p}
}

func writeCoopFinaleFields(out *bufio.Writer, sg *sessionGame, phase int) {
	writeRecordedPhaseFields(out, sg, phase, false)
}

func writeRecordedPhaseFields(out *bufio.Writer, sg *sessionGame, phase int, intermission bool) {
	g := sg.g
	f := sg.finale
	if phase == 2 {
		fmt.Fprintf(out, "%d,%d,%d,", f.Tic, f.Stage, boolToInt(f.Accelerated))
		if f.Stage == 2 {
			c := f.Cast
			fmt.Fprintf(out, "%d,%d,%d,%d,%d,%d,%d,", c.Actor, c.State, c.Tics, c.Frames, c.OnMelee, boolToInt(c.Attacking), boolToInt(c.Death))
		} else {
			fmt.Fprint(out, "0,0,0,0,0,0,0,")
		}
	} else {
		fmt.Fprint(out, "0,0,0,0,0,0,0,0,0,0,")
	}
	for _, p := range coopProbePlayers(g) {
		g.withAuthoritativePlayer(p, func() {
			pending := 10
			if g.inventory.PendingWeapon != 0 {
				pending = demoTraceWeaponID(g.inventory.PendingWeapon)
			}
			cards := 0
			for i, on := range g.inventory.cardKeys() {
				if on {
					cards |= 1 << i
				}
			}
			for i, on := range g.inventory.SkullKeys {
				if on {
					cards |= 1 << (i + 3)
				}
			}
			fmt.Fprintf(out, "%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d;", g.localSlot, g.stats.Health, g.stats.Armor, g.stats.Bullets, g.stats.Shells, g.stats.Cells, g.stats.Rockets, demoTraceWeaponID(g.inventory.ReadyWeapon), pending, boolToInt(g.weaponAttackDown), boolToInt(g.useButtonDown), cards)
		})
	}
	rng := doomrand.Capture()
	fmt.Fprintf(out, ",%d,", rng.PlayIndex)
	for cls, seed := range rng.Seeds {
		if doomrand.Class(cls) != doomrand.Misc {
			fmt.Fprintf(out, "%d;", seed)
		}
	}
	if intermission {
		if phase != 1 {
			fmt.Fprint(out, ",0,0,0,0,0,0,,0,0")
		} else {
			w := sg.intermission.state
			screen := int(w.Screen)
			if w.Screen == intermissionScreenNoState {
				screen = -1
			}
			fmt.Fprintf(out, ",%d,%d,%d,%d,%d,%d,", screen, w.Bcnt, w.Cnt, w.SPState, w.PauseTics, boolToInt(w.Accelerate))
			if w.Coop {
				for i, on := range w.CoopSlots {
					if on {
						v := w.CoopShow[i]
						fmt.Fprintf(out, "%d:%d:%d:%d;", v[0], v[1], v[2], v[3])
					}
				}
			} else {
				fmt.Fprintf(out, "%d:%d:%d:0;", w.Show.KillsPct, w.Show.ItemsPct, w.Show.SecretsPct)
			}
			if w.Coop {
				fmt.Fprint(out, ",0,0")
			} else {
				fmt.Fprintf(out, ",%d,%d", w.Show.TimeSec, w.Show.ParSec)
			}
		}
	}
	fmt.Fprintln(out)
}
