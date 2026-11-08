//go:build integration

package doomruntime

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
)

// Numeric controls drive real ticcmds, never player coordinates or actor health.
// Every checkpoint writes a playable classic demo and actual renderer screenshot.
// Set GD_GRAVEYARD_PLAYTEST and run this test binary with stdin attached.
type carnivalControl struct {
	Run     bool        `json:"run"`
	Go      *[2]float64 `json:"go"`
	Resume  string      `json:"resume"`
	Ticks   int         `json:"ticks"`
	Forward int         `json:"forward"`
	Side    int         `json:"side"`
	Turn    int         `json:"turn"`
	Buttons int         `json:"buttons"`
	Angle   *float64    `json:"angle"`
	Target  *int        `json:"target"`
	Fight   bool        `json:"fight"`
	Label   string      `json:"label"`
	Reset   string      `json:"reset"`
	Quit    bool        `json:"quit"`
}
type carnivalActor struct {
	ID       int   `json:"id"`
	Type     int16 `json:"type"`
	X, Y     float64
	HP       int
	Distance float64
}
type carnivalPlayDriver struct {
	t            *testing.T
	opts         Options
	m            *mapdata.Map
	fresh        func() *mapdata.Map
	g            *game
	input        chan carnivalControl
	done         bool
	out, attempt string
	commands     *os.File
	tics         []DemoTic
	checkpoint   int
	yawVelocity  float64
	aimTarget    int
}

func (d *carnivalPlayDriver) Update() error {
	if d.done {
		return ebiten.Termination
	}
	return nil
}
func (d *carnivalPlayDriver) Layout(int, int) (int, int) { return 960, 600 }
func (d *carnivalPlayDriver) start(label string) {
	if filepath.Base(label) != label || label == "." {
		d.t.Fatal("invalid attempt name")
	}
	d.attempt = label
	d.tics = nil
	d.checkpoint = 0
	d.yawVelocity = 0
	d.aimTarget = -1
	if d.commands != nil {
		d.commands.Close()
	}
	if err := os.MkdirAll(filepath.Join(d.out, label), 0755); err != nil {
		d.t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(d.out, label, "commands.jsonl"))
	if err != nil {
		d.t.Fatal(err)
	}
	d.commands = f
	profile, _ := json.MarshalIndent(map[string]any{"auto_weapon_switch": d.opts.AutoWeaponSwitch, "skill": 3, "tool_assisted": true, "cheats": false, "turning": "weighted mouse-like acceleration", "run_controls": true}, "", "  ")
	if err := os.WriteFile(filepath.Join(d.out, label, "profile.json"), profile, 0644); err != nil {
		d.t.Fatal(err)
	}
	d.m = d.fresh()
	d.g = newGame(d.m, d.opts)
	d.g.mode = viewWalk
	d.advance(carnivalControl{Ticks: 1})
}
func (d *carnivalPlayDriver) actors(visible bool) []carnivalActor {
	g := d.g
	result := []carnivalActor{}
	for i, th := range g.m.Things {
		if !isMonster(th.Type) || i >= len(g.thingHP) || g.thingHP[i] <= 0 || g.thingCollected[i] {
			continue
		}
		x, y := g.thingPosFixed(i, th)
		dx, dy := float64(x-g.p.x)/fracUnit, float64(y-g.p.y)/fracUnit
		dist := math.Hypot(dx, dy)
		angle := math.Atan2(dy, dx) - float64(g.p.angle)*2*math.Pi/4294967296
		angle = math.Atan2(math.Sin(angle), math.Cos(angle))
		if visible && (dist > 1800 || math.Abs(angle) > math.Pi*.43 || !g.monsterHasLOSPlayerAt(i, th.Type, x, y)) {
			continue
		}
		result = append(result, carnivalActor{i, th.Type, float64(x) / fracUnit, float64(y) / fracUnit, g.thingHP[i], dist})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Distance < result[j].Distance })
	return result
}
func (d *carnivalPlayDriver) steer(degrees float64) int16 {
	angle := uint32(math.Round(degrees / 360 * 4294967296))
	errorDegrees := float64(int32(angle-d.g.p.angle)) * 360 / 4294967296
	// Mouse-like angular inertia: accelerate into the turn, brake as the
	// target approaches, and never snap the camera to another direction.
	wanted := math.Copysign(math.Min(7, math.Sqrt(2*1.2*math.Abs(errorDegrees))), errorDegrees)
	change := wanted - d.yawVelocity
	if change > 1.2 {
		change = 1.2
	}
	if change < -1.2 {
		change = -1.2
	}
	d.yawVelocity += change
	if math.Abs(d.yawVelocity) > math.Abs(errorDegrees) && d.yawVelocity*errorDegrees > 0 {
		d.yawVelocity = errorDegrees
	}
	turn := int16(math.Round(d.yawVelocity*65536/360/256) * 256)
	return turn
}

func (d *carnivalPlayDriver) advance(c carnivalControl) {
	if c.Ticks < 0 || c.Ticks > 2000 || c.Forward < -50 || c.Forward > 50 || c.Side < -50 || c.Side > 50 || c.Turn < -128 || c.Turn > 127 || c.Buttons < 0 || c.Buttons > 127 {
		d.t.Fatal("numeric command out of range")
	}
	for i := 0; i < c.Ticks; i++ {
		if d.g.isDead || d.g.levelExitRequested {
			break
		}
		desiredAngle := c.Angle
		tc := DemoTic{Forward: int8(c.Forward), Side: int8(c.Side), AngleTurn: int16(c.Turn * 256), Buttons: byte(c.Buttons)}

		target := c.Target
		if c.Fight {
			a := d.actors(true)
			if len(a) > 0 {
				idx := a[0].ID
				for _, actor := range a {
					if actor.ID == d.aimTarget {
						idx = actor.ID
						break
					}
				}
				d.aimTarget = idx
				target = &idx
			}
		}
		if target != nil && *target >= 0 && *target < len(d.m.Things) && d.g.thingHP[*target] > 0 {
			x, y := d.g.thingPosFixed(*target, d.m.Things[*target])
			value := math.Atan2(float64(y-d.g.p.y), float64(x-d.g.p.x)) * 180 / math.Pi
			desiredAngle = &value
			tc.Buttons |= demo.ButtonAttack
		}
		if c.Go != nil {
			dx := c.Go[0] - float64(d.g.p.x)/fracUnit
			dy := c.Go[1] - float64(d.g.p.y)/fracUnit
			dist := math.Hypot(dx, dy)
			mx, my := float64(d.g.p.momx)/fracUnit, float64(d.g.p.momy)/fracUnit
			if dist < 5 && math.Hypot(mx, my) < .6 {
				break
			}
			heading := math.Atan2(dy, dx)
			if target == nil {
				value := heading * 180 / math.Pi
				desiredAngle = &value
			}
			if desiredAngle != nil {
				tc.AngleTurn = d.steer(*desiredAngle)
			}
			actual := float64(uint32(int64(d.g.p.angle)+(int64(tc.AngleTurn)<<16))) * 2 * math.Pi / 4294967296
			maxSpeed := 7.0
			if c.Run && dist > 190 {
				maxSpeed = 14
			}
			speed := math.Min(maxSpeed, dist/5)
			// Avoid racing sideways through a corner while the view catches up.
			if target == nil {
				error := math.Abs(math.Atan2(math.Sin(heading-actual), math.Cos(heading-actual)))
				speed *= math.Max(.3, math.Cos(error))
			}
			ax, ay := (speed*math.Cos(heading)-mx)*32, (speed*math.Sin(heading)-my)*32
			f := math.Round(ax*math.Cos(actual) + ay*math.Sin(actual))
			side := math.Round(ax*math.Sin(actual) - ay*math.Cos(actual))
			if f > 50 {
				f = 50
			}
			if f < -50 {
				f = -50
			}
			if side > 50 {
				side = 50
			}
			if side < -50 {
				side = -50
			}
			tc.Forward, tc.Side = int8(f), int8(side)
		}
		if c.Go == nil && desiredAngle != nil {
			tc.AngleTurn = d.steer(*desiredAngle)
		}
		if desiredAngle != nil {
			desired := uint32(math.Round(*desiredAngle / 360 * 4294967296))
			after := uint32(int64(d.g.p.angle) + (int64(tc.AngleTurn) << 16))
			error := math.Abs(float64(int32(desired-after)) * 360 / 4294967296)
			if error > 6 {
				tc.Buttons &^= demo.ButtonAttack | demo.ButtonUse
			}
		}
		d.g.capturePrevState()
		d.g.stepGameplayFromDemoTic(tc)
		d.tics = append(d.tics, tc)
	}
}
func (d *carnivalPlayDriver) state() map[string]any {
	g := d.g
	sector := g.sectorAt(g.p.x, g.p.y)
	return map[string]any{"attempt": d.attempt, "tic": len(d.tics), "seconds": float64(len(d.tics)) / 35,
		"x": float64(g.p.x) / fracUnit, "y": float64(g.p.y) / fracUnit, "z": float64(g.p.z) / fracUnit,
		"angle": float64(g.p.angle) * 360 / 4294967296, "health": g.stats.Health, "armor": g.stats.Armor,
		"bullets": g.stats.Bullets, "shells": g.stats.Shells, "weapon": g.inventory.ReadyWeapon,
		"blue_key": g.inventory.BlueKey, "kills": g.playerKillCount, "total": g.levelKillsTotal,
		"remaining": len(d.actors(false)), "secrets": g.secretsFound, "dead": g.isDead, "exited": g.levelExitRequested,
		"sector": sector, "message": g.useText, "visible": d.actors(true)}
}
func (d *carnivalPlayDriver) save(label string) {
	if label == "" {
		label = "checkpoint"
	}
	if filepath.Base(label) != label {
		d.t.Fatal("invalid checkpoint name")
	}
	d.checkpoint++
	base := filepath.Join(d.out, d.attempt, fmt.Sprintf("%03d-%s", d.checkpoint, label))
	script, err := BuildRecordedDemo(d.m.Name, d.opts, d.tics)
	if err != nil {
		d.t.Fatal(err)
	}
	script.Header.Version = demo.Version109
	if err := SaveDemoScript(filepath.Join(d.out, d.attempt, "session.lmp"), script); err != nil {
		d.t.Fatal(err)
	}
	d.g.syncRenderState()
	d.g.prepareRenderState()
	img := ebiten.NewImage(960, 600)
	d.g.Draw(img)
	f, err := os.Create(base + ".png")
	if err != nil {
		d.t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		d.t.Fatal(err)
	}
	f.Close()
	state := d.state()
	state["screenshot"] = base + ".png"
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		d.t.Fatal(err)
	}
	if err := os.WriteFile(base+".json", data, 0644); err != nil {
		d.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d.out, d.attempt, "latest.json"), data, 0644); err != nil {
		d.t.Fatal(err)
	}
	compact, _ := json.Marshal(state)
	fmt.Println("PLAYTEST " + string(compact))
}
func (d *carnivalPlayDriver) Draw(*ebiten.Image) {
	if d.g == nil {
		label := os.Getenv("GD_GRAVEYARD_ATTEMPT")
		if label == "" {
			label = "attempt-01-hmp"
		}
		d.start(label)
		d.save("spawn")
		return
	}
	select {
	case c := <-d.input:
		if c.Reset != "" {
			d.start(c.Reset)
			d.save("spawn")
			return
		}
		if c.Resume != "" {
			script, err := LoadDemoScript(filepath.Join(d.out, c.Resume, "session.lmp"))
			if err != nil {
				d.t.Fatal(err)
			}
			d.m = d.fresh()
			d.g = newGame(d.m, d.opts)
			d.g.mode = viewWalk
			d.tics = nil
			for _, tc := range script.Tics {
				d.g.capturePrevState()
				d.g.stepGameplayFromDemoTic(tc)
				d.tics = append(d.tics, tc)
			}
			d.save("resumed")
			return
		}
		encoded, _ := json.Marshal(c)
		fmt.Fprintln(d.commands, string(encoded))
		d.commands.Sync()
		d.advance(c)
		d.save(c.Label)
		if c.Quit {
			d.done = true
			d.commands.Close()
		}
	default:
	}
}
func TestGraveyardShiftInteractivePlaytest(t *testing.T) {
	out := os.Getenv("GD_GRAVEYARD_PLAYTEST")
	if out == "" {
		t.Skip("set GD_GRAVEYARD_PLAYTEST; feed numeric JSON commands on stdin")
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	wf, m := graveyardMap(t)
	sourcePath := os.Getenv("GD_GRAVEYARD_WAD")
	if sourcePath == "" {
		sourcePath = "../../mods/graveyard-shift/graveyard-shift.wad"
	}
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "graveyard-shift-playtest.wad"), source, 0644); err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(source))
	os.WriteFile(filepath.Join(out, "wad-sha256.txt"), []byte(digest+"\n"), 0644)
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatal(err)
	}
	walls := buildWallTextureBankForTest(t, ts)
	flats, err := doomtex.LoadFlatsRGBA(wf, 0)
	if err != nil {
		t.Fatal(err)
	}
	indexed, err := doomtex.LoadFlatsIndexed(wf)
	if err != nil {
		t.Fatal(err)
	}
	palette, err := ts.PaletteRGBA(0)
	if err != nil {
		t.Fatal(err)
	}
	lump, _ := wf.LumpByName("COLORMAP")
	cm, err := wf.LumpData(lump)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := doomtex.LoadBoomTables(wf, ts)
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{Width: 960, Height: 600, PlayerSlot: 1, SkillLevel: 3, NoFPS: true, AutoWeaponSwitch: true,
		WallTexBank: walls, FlatBank: flats, FlatBankIndexed: indexed, DoomPaletteRGBA: palette, DoomColorMap: cm, DoomColorMapRows: len(cm) / 256,
		SpritePatchBank: graveyardSpriteBank(t, ts), StatusPatchBank: buildAlphaPatchBankForDump(t, wf, ts),
		WallTextureAnimSequences: tables.Walls, FlatTextureAnimSequences: tables.Flats}
	driver := &carnivalPlayDriver{t: t, opts: opts, m: m, out: out, input: make(chan carnivalControl, 16)}
	driver.fresh = func() *mapdata.Map {
		m, err := mapdata.LoadMap(wf, m.Name)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	go func() {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			var c carnivalControl
			if err := json.Unmarshal(s.Bytes(), &c); err != nil {
				fmt.Println("CONTROL ERROR", err)
				continue
			}
			driver.input <- c
		}
	}()
	ebiten.SetVsyncEnabled(true)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}

// Replay the archived inputs through the normal demo player, independently of
// the numeric controller. Check the same final position, inventory and outcome.
func TestGraveyardShiftRecordedSessionReplay(t *testing.T) {
	out := os.Getenv("GD_GRAVEYARD_VERIFY")
	if out == "" {
		t.Skip("set GD_GRAVEYARD_VERIFY to an archived playtest folder")
	}
	wf, err := wad.OpenFiles(findDOOM1WAD(t), filepath.Join(out, "graveyard-shift-playtest.wad"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(out, entry.Name())
		raw, err := os.ReadFile(filepath.Join(path, "latest.json"))
		if err != nil {
			continue
		}
		var expected map[string]any
		if err := json.Unmarshal(raw, &expected); err != nil {
			t.Fatal(err)
		}
		script, err := LoadDemoScript(filepath.Join(path, "session.lmp"))
		if err != nil {
			t.Fatal(err)
		}
		if script.Header.NoMonsters || script.Header.Skill != 2 || script.Header.Episode != 1 || (script.Header.Map != 1 && script.Header.Map != 2) {
			t.Fatalf("unexpected demo header: %+v", script.Header)
		}
		m, err := mapdata.LoadMap(wf, mapdata.MapName(fmt.Sprintf("E%dM%d", script.Header.Episode, script.Header.Map)))
		if err != nil {
			t.Fatal(err)
		}
		g := newGame(m, Options{Headless: true, PlayerSlot: 1, SkillLevel: 3, DemoScript: script})
		var profile struct {
			Auto bool `json:"auto_weapon_switch"`
		}
		data, err := os.ReadFile(filepath.Join(path, "profile.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &profile); err != nil {
			t.Fatal(err)
		}
		if !profile.Auto {
			g.autoWeaponSwitch = false
			g.opts.AutoWeaponSwitch = false
			t.Logf("%s: legacy recorder profile; standard client playback will differ", entry.Name())
		}
		for range script.Tics {
			if err := g.advanceDemoTic(); err != nil {
				t.Fatalf("%s: %v", entry.Name(), err)
			}
		}
		got := map[string]any{"x": float64(g.p.x) / fracUnit, "y": float64(g.p.y) / fracUnit, "z": float64(g.p.z) / fracUnit,
			"health": g.stats.Health, "armor": g.stats.Armor, "bullets": g.stats.Bullets, "shells": g.stats.Shells,
			"kills": g.playerKillCount, "secrets": g.secretsFound, "dead": g.isDead, "exited": g.levelExitRequested, "tic": len(script.Tics)}
		for key, actual := range got {
			switch v := actual.(type) {
			case int:
				if float64(v) != expected[key].(float64) {
					t.Fatalf("%s %s=%v, want %v", entry.Name(), key, actual, expected[key])
				}
			case float64:
				if v != expected[key].(float64) {
					t.Fatalf("%s %s=%v, want %v", entry.Name(), key, actual, expected[key])
				}
			case bool:
				if v != expected[key].(bool) {
					t.Fatalf("%s %s=%v, want %v", entry.Name(), key, actual, expected[key])
				}
			}
		}
		t.Logf("%s: %d tics, kills=%d, secrets=%d, exited=%t, dead=%t; exact replay", entry.Name(), len(script.Tics), g.playerKillCount, g.secretsFound, g.levelExitRequested, g.isDead)
		actors := (&carnivalPlayDriver{g: g}).actors(false)
		if len(actors) > 0 {
			data, _ := json.Marshal(actors)
			t.Logf("%s remaining actors: %s", entry.Name(), data)
		}
		checked++
	}
	if checked == 0 {
		t.Fatalf("verified only %d sessions", checked)
	}
}
