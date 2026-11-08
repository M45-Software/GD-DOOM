//go:build integration

package doomruntime

import (
	"bytes"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/music"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
)

func graveyardMap(t *testing.T) (*wad.File, *mapdata.Map) {
	t.Helper()
	source := os.Getenv("GD_GRAVEYARD_WAD")
	if source == "" {
		source = "../../mods/graveyard-shift/graveyard-shift.wad"
	}
	wf, err := wad.OpenFiles(findDOOM1WAD(t), source)
	if err != nil {
		t.Fatal(err)
	}
	name := mapdata.MapName(os.Getenv("GD_GRAVEYARD_MAP"))
	if name == "" {
		name = "E1M1"
	}
	m, err := mapdata.LoadMap(wf, name)
	if err != nil {
		t.Fatal(err)
	}
	return wf, m
}

// Include the shareware tech lamp alongside the shared diagnostic sprite set.
func graveyardSpriteBank(t *testing.T, ts *doomtex.Set) map[string]WallTexture {
	t.Helper()
	sprites := buildBillboardSpritePatchBankForTest(t, ts)
	rgba, w, h, ox, oy, err := ts.BuildPatchRGBA("COLUA0", 0)
	if err != nil {
		t.Fatal(err)
	}
	sprites["COLUA0"] = WallTexture{RGBA: rgba, Width: w, Height: h, OffsetX: ox, OffsetY: oy}
	return sprites
}

// Movement follows the actual route in eight-unit steps with collision, stairs,
// gravity, pickups, use traces and crossed-line actions enabled. No noclip,
// injected keys, or direct activation of the progression linedefs is used.
func TestGraveyardShiftProgression(t *testing.T) {
	for _, skill := range []int{1, 3, 4} {
		t.Run(fmt.Sprint(skill), func(t *testing.T) {
			wf, m := graveyardMap(t)
			g := newGame(m, Options{Headless: true, NoMonsters: true, SkillLevel: skill, PlayerSlot: 1})
			walk := func(x, y int) {
				t.Helper()
				tx, ty := int64(x)*fracUnit, int64(y)*fracUnit
				for steps := 0; g.p.x != tx || g.p.y != ty; steps++ {
					if steps > 2000 {
						t.Fatal("movement did not converge")
					}
					dx, dy := tx-g.p.x, ty-g.p.y
					if dx > 8*fracUnit {
						dx = 8 * fracUnit
					}
					if dx < -8*fracUnit {
						dx = -8 * fracUnit
					}
					if dy > 8*fracUnit {
						dy = 8 * fracUnit
					}
					if dy < -8*fracUnit {
						dy = -8 * fracUnit
					}
					if !g.tryMove(g.p.x+dx, g.p.y+dy) {
						t.Fatalf("blocked at %.1f,%.1f,z=%.1f on route to %d,%d", float64(g.p.x)/fracUnit, float64(g.p.y)/fracUnit, float64(g.p.z)/fracUnit, x, y)
					}
					g.zMovement()
					g.processThingPickups()
					g.tickDoors()
				}
				for i := 0; i < 20; i++ {
					g.zMovement()
					g.tickDoors()
				}
			}
			use := func(degrees float64) {
				t.Helper()
				g.p.angle = uint32(degrees / 360 * 4294967296)
				g.useLines()
				for i := 0; i < 160; i++ {
					g.tickDoors()
				}
			}
			// Gate rejects the player before the staff loop has supplied a blue pass.
			walk(352, 480)
			walk(1056, 480)
			walk(1120, 768)
			use(0)
			if g.useText != "You need a blue key to open this door" {
				t.Fatalf("gate did not reject missing key: %q", g.useText)
			}
			// Guaranteed shotgun, staff yard, 16-unit risers, and the real key pickup.
			walk(1056, 480)
			walk(352, 480)
			walk(-96, 480)
			walk(-96, 576)
			use(180)
			walk(-256, 576)
			walk(-544, 544)
			if !g.inventory.Weapons[2001] {
				t.Fatal("main-route shotgun was not picked up")
			}
			walk(-608, 736)
			walk(-608, 1088)
			walk(-608, 1248)
			walk(-608, 1504)
			walk(-672, 1504)
			walk(-672, 1760)
			walk(-224, 1760)
			walk(288, 1760)
			walk(544, 1760)
			walk(544, 1888)
			if !g.inventory.BlueKey {
				t.Fatal("blue pass was not picked up")
			}
			walk(672, 1792)
			use(0)
			for i, s := range m.Sectors {
				if s.Tag == 20 && g.sectorCeil[i]-g.sectorFloor[i] < 56*fracUnit {
					t.Fatalf("power switch failed to open sector %d", i)
				}
			}
			// Opened shutter, balcony drop, and keyed main admission.
			walk(544, 1792)
			walk(544, 1632)
			walk(544, 1536)
			walk(608, 1440)
			walk(608, 1312)
			if g.p.z != 0 {
				t.Fatalf("balcony did not drop to plaza: z=%d", g.p.z)
			}
			walk(864, 1312)
			walk(1088, 1312)
			walk(1088, 768)
			walk(1120, 768)
			use(0)
			walk(1280, 768)
			walk(1408, 768)
			for i, s := range m.Sectors {
				if s.Tag == 30 && g.sectorCeil[i]-g.sectorFloor[i] < 56*fracUnit {
					t.Fatalf("final ambush failed to open sector %d", i)
				}
			}
			walk(1408, 1216)
			walk(2112, 1216)
			walk(2112, 896)
			walk(2656, 896)
			use(0)
			if !g.levelExitRequested {
				t.Fatalf("exit did not activate: %q", g.useText)
			}
			secrets := 0
			for _, s := range m.Sectors {
				if s.Special == 9 {
					secrets++
				}
			}
			if secrets != 3 {
				t.Fatalf("secrets=%d, want 3", secrets)
			}
			lump, ok := wf.LumpByName("D_E1M1")
			if !ok {
				t.Fatal("missing music")
			}
			raw, err := wf.LumpData(lump)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = music.ParseMUS(raw); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Each optional reward must be accessible through the intended classic action.
func TestGraveyardShiftSecretDoors(t *testing.T) {
	tests := []struct {
		name    string
		x, y    int
		angle   float64
		shoot   bool
		tag     int
		special uint16
	}{
		{"garden", 960, 224, 270, false, 0, 31},
		{"snack cache", -800, 512, 180, false, 0, 31},
		{"shooting gallery", -1040, 1792, 180, true, 40, 46},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, m := graveyardMap(t)
			g := newGame(m, Options{Headless: true, NoMonsters: true, SkillLevel: 3, PlayerSlot: 1})
			g.setPlayerPosFixed(int64(tc.x)*fracUnit, int64(tc.y)*fracUnit)
			g.p.angle = uint32(tc.angle / 360 * 4294967296)
			line, tr := g.peekUseTargetLine()
			if tr != useTraceSpecial || line < 0 {
				t.Fatalf("no usable secret line: %d %v", line, tr)
			}
			if m.Linedefs[line].Special != tc.special {
				t.Fatalf("secret line special=%d", m.Linedefs[line].Special)
			}
			var targets []int
			if tc.shoot {
				g.shootSpecialLine(line, true)
				for i, s := range m.Sectors {
					if int(s.Tag) == tc.tag {
						targets = append(targets, i)
					}
				}
			} else {
				targets, _ = m.DoorTargetSectors(line)
				g.useLines()
			}
			if len(targets) == 0 {
				t.Fatal("secret door has no target")
			}
			for i := 0; i < 160; i++ {
				g.tickDoors()
			}
			for _, sec := range targets {
				if g.sectorCeil[sec]-g.sectorFloor[sec] < 56*fracUnit {
					t.Fatalf("secret door sector %d remained closed", sec)
				}
			}
		})
	}
}

// Verify actual classic texture-table ranges, distinct palette-indexed frames,
// and runtime frame selection. No custom ANIMATED or SWITCHES lump is needed.
func TestGraveyardShiftAnimations(t *testing.T) {
	wf, m := graveyardMap(t)
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := doomtex.LoadBoomTables(wf, ts)
	if err != nil {
		t.Fatal(err)
	}
	flats, err := doomtex.LoadFlatsIndexed(wf)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ANIMATED", "SWITCHES"} {
		if _, ok := wf.LumpByName(name); ok {
			t.Fatalf("unexpected extension %s", name)
		}
	}
	g := newGame(m, Options{Headless: true, NoMonsters: true, SkillLevel: 3,
		WallTextureAnimSequences: tables.Walls, FlatTextureAnimSequences: tables.Flats})
	cycles := [][]string{{"GSTFONT1", "GSTFONT2", "GSTFONT3"}, {"SLADRIP1", "SLADRIP2", "SLADRIP3"}, {"FWATER1", "FWATER2", "FWATER3", "FWATER4"}}
	for _, cycle := range cycles {
		isFlat := cycle[0] == "FWATER1"
		seq := tables.Walls[cycle[0]]
		if isFlat {
			seq = tables.Flats[cycle[0]]
		}
		if !reflect.DeepEqual(seq, cycle) {
			t.Fatalf("%s sequence %v, want %v", cycle[0], seq, cycle)
		}
		var prev []byte
		for i, name := range cycle {
			var pix []byte
			if isFlat {
				pix = flats[name]
			} else {
				pix, _, _, err = ts.BuildTextureRGBA(name, 0)
				if err != nil {
					t.Fatal(err)
				}
			}
			if len(pix) == 0 || (i > 0 && bytes.Equal(prev, pix)) {
				t.Fatalf("%s has missing or duplicate art", name)
			}
			if isFlat {
				palette, err := ts.PaletteRGBA(0)
				if err != nil {
					t.Fatal(err)
				}
				var red, blue int
				for _, index := range pix {
					red += int(palette[int(index)*4])
					blue += int(palette[int(index)*4+2])
				}
				if blue-red < len(pix)*40 {
					t.Fatalf("%s lost its blue color in the stock palette", name)
				}
			}
			prev = pix
			g.worldTic = i*8 + 1
			got := g.resolveAnimatedWallName(cycle[0])
			if isFlat {
				got = g.resolveAnimatedFlatName(cycle[0])
			}
			if got != name {
				t.Fatalf("%s at tic %d: got %s, want %s", cycle[0], g.worldTic, got, name)
			}
		}
	}
}

// Door posters must form one continuous face, and the reverse marquee face
// needs its own offsets rather than showing the front face's chunks backward.
func TestGraveyardShiftTextureAlignment(t *testing.T) {
	_, m := graveyardMap(t)
	gateFaces := map[int][]int{}
	marquee := 0
	for _, line := range m.Linedefs {
		if line.SideNum[0] < 0 {
			continue
		}
		front := m.Sidedefs[line.SideNum[0]]
		if front.Top == "GDOOR" && m.Sectors[front.Sector].CeilingHeight != 128 {
			t.Fatal("security sign would repeat vertically")
		}
		if front.Top == "GBLUE" {
			a, b := m.Vertexes[line.V1], m.Vertexes[line.V2]
			if a.X != b.X {
				t.Fatal("admission face is not vertical")
			}
			gateFaces[int(a.X)] = append(gateFaces[int(a.X)], int(front.TextureOffset))
			if m.Sectors[front.Sector].CeilingHeight != 128 {
				t.Fatal("gate poster would repeat vertically")
			}
		}
		if front.Mid == "GSTFONT1" && line.SideNum[1] >= 0 {
			back := m.Sidedefs[line.SideNum[1]]
			if int(front.TextureOffset)+int(back.TextureOffset) != 192 {
				t.Fatalf("marquee front/back offsets %d,%d", front.TextureOffset, back.TextureOffset)
			}
			marquee++
		}
	}
	if len(gateFaces) != 2 {
		t.Fatalf("gate has %d faces, want 2", len(gateFaces))
	}
	for x, offsets := range gateFaces {
		sort.Ints(offsets)
		if !reflect.DeepEqual(offsets, []int{0, 64, 128, 192}) {
			t.Fatalf("gate face x=%d offsets=%v", x, offsets)
		}
	}
	if marquee != 4 {
		t.Fatalf("marquee segments=%d, want 4", marquee)
	}
}

// Elevated outdoor views must not encounter a low sky plane across the
// control room or stairwell. Covered rooms and the canopy keep real ceilings.
func TestGraveyardShiftExteriorSky(t *testing.T) {
	_, m := graveyardMap(t)
	for i, sector := range m.Sectors {
		if sector.CeilingPic == "F_SKY1" && sector.CeilingHeight < 320 {
			t.Fatalf("outdoor sector %d has low sky at %d", i, sector.CeilingHeight)
		}
	}
}

// Optional visual QA runs in the real software renderer with the actual IWAD
// sprites, original mod textures and palette. Run alone: it owns Ebiten's loop.
func TestGraveyardShiftRender(t *testing.T) {
	out := os.Getenv("GD_GRAVEYARD_CAPTURE")
	if out == "" {
		t.Skip("set GD_GRAVEYARD_CAPTURE to retain gameplay PNGs")
	}
	wf, m := graveyardMap(t)
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatal(err)
	}
	walls := buildWallTextureBankForTest(t, ts)
	tables, err := doomtex.LoadBoomTables(wf, ts)
	if err != nil {
		t.Fatal(err)
	}
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
	colormap, err := wf.LumpData(lump)
	if err != nil {
		t.Fatal(err)
	}
	sprites := graveyardSpriteBank(t, ts)
	for _, key := range []string{"GSTRIPE", "GTEAL", "GARCH", "GBLUE", "SKY1"} {
		if _, ok := walls[key]; !ok {
			t.Fatalf("missing wall %s", key)
		}
	}
	for _, key := range []string{"GCHECK", "GPAVING", "GGRATE"} {
		if _, ok := flats[key]; !ok {
			t.Fatalf("missing flat %s", key)
		}
	}
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		scenes := []struct {
			name   string
			x, y   int
			z, yaw float64
		}{
			{"01-arrival", 416, -240, 0, 90}, {"02-ticket-plaza", 608, 448, 0, 90},
			{"03-staff-yard", -608, 1408, 0, 120}, {"04-control-booth", 352, 1728, 128, 30},
			{"05-balcony", 608, 1440, 128, 270}, {"06-welcome-courtyard", 1440, 640, 0, 30},
			{"07-fountain", 608, 672, 16, 90}, {"08-customer-feedback", 2112, 1088, 0, 90},
			{"09-stairs-down", -96, 1760, 48, 180}, {"10-yard-to-booth", -608, 1760, 0, 0},
			{"11-balcony-to-booth", 608, 1440, 128, 90},
			{"12-kiosk-roof", 704, 800, 0, 45},
			{"13-sprite-light", 1088, 704, 0, 240},
            {"14-security-entrance", 192, 704, 0, 160},
            {"15-security-room", -352, 416, 0, 140},
		}
		if m.Name == "E1M2" {
			scenes = []struct {
				name   string
				x, y   int
				z, yaw float64
			}{
				{"map02-01-arrival", 544, -160, 0, 90},
				{"map02-02-midway", 448, 576, 0, 90},
				{"map02-03-gallery", -320, 960, 0, 180},
				{"map02-04-service", -480, 1856, 0, 180},
				{"map02-05-bumper-arena", 1504, 640, 0, 30},
				{"map02-06-cars", 2080, 896, 0, 270},
				{"map02-07-prize-hall", 1184, 1888, 0, 90},
				{"map02-08-assessment-sign", -576, 672, 0, 207},
				{"map02-09-blue-pass", 1056, 640, 0, 0},
				{"map02-10-red-pass", 896, 1504, 0, 90},
				{"map02-11-prize-sign", 896, 2368, 0, 90},
				{"map02-12-next-shift", 1344, 2368, 0, 90},
				{"map02-13-midway-light", 832, 1504, 0, 330},
				{"map02-14-arena-light", 2912, 512, 0, 255},
                {"map02-15-prize-exit", 1728, 2336, 0, 90},
			}
		}
		for _, scene := range scenes {
			g := newGame(m, Options{Width: 960, Height: 600, SkillLevel: 3, PlayerSlot: 1, NoFPS: true, NoMonsters: os.Getenv("GD_GRAVEYARD_HIDE_MONSTERS") != "",
				Headless: true, SourcePortMode: true, SourcePortSectorLighting: true,
				WallTexBank: walls, FlatBank: flats, FlatBankIndexed: indexed, SpritePatchBank: sprites,
				WallTextureAnimSequences: tables.Walls, FlatTextureAnimSequences: tables.Flats,
				DoomPaletteRGBA: palette, DoomColorMap: colormap, DoomColorMapRows: len(colormap) / 256})
			g.mode = viewWalk
			g.setPlayerPosFixed(int64(scene.x)*fracUnit, int64(scene.y)*fracUnit)
			g.p.z = int64(scene.z * fracUnit)
			g.playerViewZ = int64((scene.z + 41) * fracUnit)
			g.p.angle = uint32(scene.yaw / 360 * math.Pow(2, 32))
			g.syncRenderState()
			g.prepareRenderState()
			img := ebiten.NewImage(960, 600)
			g.Draw(img)
			f, err := os.Create(filepath.Join(out, scene.name+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, img); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
		}
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}
