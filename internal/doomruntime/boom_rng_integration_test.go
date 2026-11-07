//go:build integration

package doomruntime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestBoomRNGVectorsCapture(t *testing.T) {
	path := os.Getenv("GD_RNG_VECTORS")
	if path == "" {
		t.Skip("run verify_boom_rng.py")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name         string
		Seed         uint32
		Insurance    bool
		Tic          uint32
		Class, Count int
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(os.Getenv("GD_RNG_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := bufio.NewWriter(f)
	defer out.Flush()
	fmt.Fprintln(out, "name,draw,value,playindex,menuindex,seeds")
	for _, c := range cases {
		doomrand.ConfigureBoom(c.Seed, c.Insurance)
		doomrand.SetLogicTic(c.Tic)
		for i := 0; i < c.Count; i++ {
			cls := c.Class
			if cls < 0 {
				cls = (i*17 + 13) % doomrand.ClassCount
			}
			value := doomrand.PRandom(doomrand.Class(cls))
			s := doomrand.Capture()
			fmt.Fprintf(out, "%s,%d,%d,%d,%d,", c.Name, i, value, s.PlayIndex, s.MenuIndex)
			for _, seed := range s.Seeds {
				fmt.Fprintf(out, "%d;", seed)
			}
			fmt.Fprintln(out)
		}
	}
	doomrand.Clear()
}
func TestBoomRNGDemoCapture(t *testing.T) {
	path := os.Getenv("GD_RNG_DEMO")
	if path == "" {
		t.Skip("run verify_boom_rng.py")
	}
	script, err := demo.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	wf, err := wad.Open(os.Getenv("GD_RNG_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	fullMap := os.Getenv("GD_RNG_FULL_MAP") == "1"
	rebirth := os.Getenv("GD_RNG_REBIRTH") == "1"
	extended := fullMap || rebirth
	mapName := mapdata.MapName("MAP01")
	if extended {
		mapName = mapdata.MapName(fmt.Sprintf("MAP%02d", script.Header.Map))
	}
	m, err := mapdata.LoadMap(wf, mapName)
	if err != nil {
		t.Fatal(err)
	}
	g := newGame(m, Options{Headless: true, DemoScript: script})
	f, err := os.Create(os.Getenv("GD_RNG_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := bufio.NewWriter(f)
	defer out.Flush()
	fmt.Fprint(out, "tic,x,y,z,angle,momx,momy,momz,health,armor,ammo,viewz,playindex,mobjs,seeds")
	if extended {
		fmt.Fprint(out, ",shells,cells,rockets,ready,pending,weaponstate,weapontics,flash,flashtics,attackdown,refire,kills,secrets,sectors")
	}
	if rebirth {
		fmt.Fprint(out, ",leveltime,playerstate,viewheight,deltaviewheight,reloads,logictic")
	}
	fmt.Fprintln(out)
	checkpoints := make(map[int]bool)
	for _, value := range strings.Split(os.Getenv("GD_RNG_CHECKPOINT_TICS"), ",") {
		if value == "" {
			continue
		}
		tic, err := strconv.Atoi(value)
		if err != nil || tic < 0 || tic >= len(script.Tics) {
			t.Fatalf("invalid checkpoint tic %q", value)
		}
		checkpoints[tic] = true
	}
	appliedCheckpoints := make([]int, 0)
	previousLevelTime, reloads := 0, 0
	for tic := range script.Tics {
		if err := g.advanceDemoTic(); err != nil {
			t.Fatal(err)
		}
		if checkpoints[tic] {
			magic, version := saveGameMagic, saveGameVersion
			if os.Getenv("GD_RNG_CHECKPOINT_FORMAT") == "keyframe" {
				magic, version = keyframeMagic, keyframeVersion
			}
			state := captureGameSaveState(g)
			if !validBoomSectorTouches(state, len(g.m.Sectors)) {
				t.Fatalf("invalid membership before checkpoint %d", tic)
			}
			rng := doomrand.Capture()
			blob, err := encodeSnapshot(magic, saveFile{Version: version, Game: state, RNG: saveRNGState{Extended: rng, MenuIndex: rng.MenuIndex, PlayIndex: rng.PlayIndex}})
			if err != nil {
				t.Fatal(err)
			}
			file, err := decodeSnapshot(blob, magic)
			if err != nil {
				t.Fatal(err)
			}
			loaded := newGame(cloneMapForRestart(g.restartTemplate), g.opts)
			restoreGameSaveState(loaded, file.Game)
			loaded.inheritDemoPlayback(g)
			restoreSavedRNG(file)
			g = loaded
			appliedCheckpoints = append(appliedCheckpoints, tic)
		}
		if tic == 0 && os.Getenv("GD_RNG_CRUSHED_CORPSE") != "" {
			// Controlled crushed-corpse setup; subsequent resurrection is
			// performed exclusively by the live arch-vile thinker.
			for i, th := range g.m.Things {
				if th.Type == 9 {
					g.damageMonster(i, 200)
					g.thingGibbed[i] = true
					g.thingMomX[i], g.thingMomY[i] = 0, 0
					g.thingState[i], g.thingStatePhase[i], g.thingStateTics[i] = monsterStateGibs, 0, -1
				}
			}
		}
		s := doomrand.Capture()
		fmt.Fprintf(out, "%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,", tic, g.p.x, g.p.y, g.p.z, g.p.angle, g.p.momx, g.p.momy, g.p.momz, g.stats.Health, g.stats.Armor, g.stats.Bullets, g.playerViewZ, s.PlayIndex)
		for _, m := range g.demoTraceMobjs() {
			if os.Getenv("GD_RNG_COMBAT_ONLY") != "" && (m.Kind == "thing" || m.Kind == "pickup" || m.Kind == "barrel" && !fullMap) {
				continue
			}
			if m.Player != 0 || m.Flags&1 != 0 {
				continue
			}
			fmt.Fprintf(out, "%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d", m.Type, m.X, m.Y, m.Z, m.Angle, m.MomX, m.MomY, m.MomZ, m.Health, m.Tics, m.State)
			if os.Getenv("GD_RNG_COMBAT_ONLY") != "" {
				fmt.Fprintf(out, ":%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d", m.Target, m.TargetType, m.Tracer, m.TracerType, m.Movedir, m.Movecount, m.ReactionTime, m.Threshold, m.LastLook, m.Radius, m.Height, m.FloorZ, m.CeilingZ)
			}
			fmt.Fprint(out, ";")
		}
		fmt.Fprint(out, ",")
		for cls, seed := range s.Seeds {
			if doomrand.Class(cls) != doomrand.Misc {
				fmt.Fprintf(out, "%d;", seed)
			}
		}
		if extended {
			pending := 10
			if g.inventory.PendingWeapon != 0 {
				pending = demoTraceWeaponID(g.inventory.PendingWeapon)
			}
			fmt.Fprintf(out, ",%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,", g.stats.Shells, g.stats.Cells, g.stats.Rockets,
				demoTraceWeaponID(g.inventory.ReadyWeapon), pending, weaponDoomStateID(g.weaponState), g.weaponStateTics,
				weaponDoomStateID(g.weaponFlashState), g.weaponFlashTics, boolToInt(g.weaponAttackDown), boolToInt(g.weaponRefire), g.playerKillCount, g.secretsFound)
			for i, sector := range g.m.Sectors {
				fmt.Fprintf(out, "%d:%d:%d;", g.sectorFloor[i], g.sectorCeil[i], sector.Light)
			}
		}
		if rebirth {
			if g.worldTic < previousLevelTime {
				reloads++
			}
			state := 0
			if g.playerReborn {
				state = 2
			} else if g.isDead {
				state = 1
			}
			fmt.Fprintf(out, ",%d,%d,%d,%d,%d,%d", g.worldTic, state, g.p.viewHeight, g.p.deltaViewHeight, reloads, s.LogicTic)
			previousLevelTime = g.worldTic
		}
		fmt.Fprintln(out)
		if fullMap && g.levelExitRequested {
			break
		}
	}
	if path := os.Getenv("GD_RNG_CHECKPOINT_LOG"); path != "" {
		data, err := json.Marshal(appliedCheckpoints)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if fullMap {
		summary := map[string]any{"completed": g.levelExitRequested, "leveltime": g.worldTic, "kills": g.playerKillCount,
			"total_kills": g.levelKillsTotal, "secrets": g.secretsFound, "total_secrets": g.secretsTotal, "health": g.stats.Health}
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(os.Getenv("GD_RNG_SUMMARY"), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBoomRNGLiftsCapture(t *testing.T) {
	path := os.Getenv("GD_RNG_LIFTS")
	if path == "" {
		t.Skip("run verify_boom_rng.py")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name                      string
		Seed                      uint32
		Insurance                 bool
		Special                   uint16
		Draws, Tics, Stop, Resume int
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	wf, err := wad.Open(os.Getenv("GD_RNG_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(os.Getenv("GD_RNG_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := bufio.NewWriter(f)
	defer out.Flush()
	fmt.Fprintln(out, "name,tic,floor,status,count,low,high,playindex,seeds")
	for _, c := range cases {
		m, err := mapdata.LoadMap(wf, "MAP01")
		if err != nil {
			t.Fatal(err)
		}
		var raw [64]byte
		raw[0], raw[1], raw[3], raw[5] = 1, 1, 1, 1
		raw[10] = byte(c.Seed >> 24)
		raw[11] = byte(c.Seed >> 16)
		raw[12] = byte(c.Seed >> 8)
		raw[13] = byte(c.Seed)
		if c.Insurance {
			raw[9] = 1
		}
		settings := demo.DecodeGameOptions(raw)
		g := newGame(m, Options{Headless: true, SkillLevel: 3, BoomGameOptions: &settings})
		m.Linedefs[2].Special = c.Special
		g.lineSpecial[2] = c.Special
		for i := 0; i < c.Draws; i++ {
			doomrand.PRandom(doomrand.See)
		}
		for tic := 0; tic <= c.Tics; tic++ {
			if tic == 0 || tic == c.Resume {
				if c.Special == 87 {
					g.checkWalkSpecialLinesForActorWithCandidates(-129*fracUnit, 0, -127*fracUnit, 0, -1, true, []int{2})
				} else {
					g.useSpecialLineForActorIndex(2, 0, true, -1)
				}
			}
			if tic == c.Stop {
				g.stopTaggedPlats(7)
			}
			if tic > 0 {
				g.worldTic = tic
				for _, pt := range g.plats {
					g.tickPlat(pt.sector, pt)
				}
			}
			status, count := -1, 0
			var low, high int64
			if pt := g.plats[1]; pt != nil {
				status = int(pt.status)
				count = pt.count
				low = pt.low
				high = pt.high
			}
			s := doomrand.Capture()
			fmt.Fprintf(out, "%s,%d,%d,%d,%d,%d,%d,%d,", c.Name, tic, g.sectorFloor[1], status, count, low, high, s.PlayIndex)
			for cls, seed := range s.Seeds {
				if doomrand.Class(cls) != doomrand.Misc {
					fmt.Fprintf(out, "%d;", seed)
				}
			}
			fmt.Fprintln(out)
			if tic > 0 {
				doomrand.AdvanceLogicTic()
			}
			g.soundQueue = g.soundQueue[:0]
			g.soundQueueOrigin = g.soundQueueOrigin[:0]
		}
	}
}
