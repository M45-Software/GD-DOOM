//go:build integration

package doomruntime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
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
	if campaign {
		fmt.Fprint(out, "demotic,map,")
	}
	if finaleMode {
		fmt.Fprintln(out, "phase,count,stage,accelerated,castnum,caststate,casttics,castframes,melee,attacking,death,players,playindex,seeds")
	} else {
		fmt.Fprintln(out, "tic,players,bodies,items,sectors,playindex,seeds")
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
	if script.Header.PlayerCount() == 1 {
		frames = len(script.Tics)
	}
	for tic := 0; tic < frames; tic++ {
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
				fresh.opts.DemoScript = nil
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
		if phases != nil {
			fmt.Fprintf(phases, "%d,%s,%d,", tic+1, g.m.Name, phase)
			writeRecordedPhaseFields(phases, sg, phase, true)
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
		if campaign {
			fmt.Fprintf(out, "%d,%s,", tic+1, g.m.Name)
		}
		fmt.Fprintf(out, "%d,", g.worldTic-1)
		for _, p := range coopProbePlayers(g) {
			g.withAuthoritativePlayer(p, func() {
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
					ax, ay = g.statusAttackerPosition()
				}
				fmt.Fprintf(out, ":%d:%d;", ax, ay)
			})
		}
		fmt.Fprint(out, ",")
		for _, m := range g.demoTraceMobjs() {
			if m.Player != 0 || m.Flags&1 != 0 || os.Getenv("GD_COOP_COMBAT_ONLY") != "" && (m.Type > 42 || m.Type == 41) {
				continue
			}
			owner := m.ownerSlot
			fmt.Fprintf(out, "%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d", m.Type, m.X, m.Y, m.Z, m.Angle, m.MomX, m.MomY, m.MomZ, m.Health, m.Tics, m.State, m.Radius, m.Height, m.FloorZ, m.CeilingZ, owner)
			if m.Kind == "monster" && thingCountsKill(g.m.Things[m.thingIndex].Type) {
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
		for _, m := range g.demoTraceMobjs() {
			if m.Flags&1 != 0 {
				fmt.Fprintf(out, "%d:%d:%d:%d;", m.Type, m.X, m.Y, m.Z)
			}
		}
		fmt.Fprint(out, ",")
		for i, s := range g.m.Sectors {
			fmt.Fprintf(out, "%d:%d:%d;", g.sectorFloor[i], g.sectorCeil[i], s.Light)
		}
		r := doomrand.Capture()
		fmt.Fprintf(out, ",%d,", r.PlayIndex)
		for cls, seed := range r.Seeds {
			if doomrand.Class(cls) != doomrand.Misc {
				fmt.Fprintf(out, "%d;", seed)
			}
		}
		fmt.Fprintln(out)
	}
	if restored != len(checkpoints) {
		t.Fatal("not all checkpoints restored")
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
