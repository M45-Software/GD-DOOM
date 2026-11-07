package doomruntime

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func boomTouchFixture(t *testing.T) *game {
	t.Helper()
	before := doomrand.Capture()
	t.Cleanup(func() { doomrand.Restore(before) })
	m := &mapdata.Map{Name: "MAP01",
		Vertexes: []mapdata.Vertex{{X: 0, Y: -128}, {X: 0, Y: 128}},
		Linedefs: []mapdata.Linedef{{V1: 0, V2: 1, Flags: mlTwoSided, SideNum: [2]int32{0, 1}}},
		Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}},
		Sectors:  []mapdata.Sector{{CeilingHeight: 128}, {CeilingHeight: 128}},
		Things:   []mapdata.Thing{{Type: 1, X: -256, Flags: 7}, {Type: 9, X: -8, Flags: 7}, {Type: 9, X: 8, Flags: 7}},
	}
	return newGame(m, Options{Headless: true, BoomGameOptions: &demo.GameOptions{Seed: 1993, Insurance: true}})
}

func boomTouchOrder(g *game, sector int) []int {
	var out []int
	g.walkBoomSectorTouches(sector, func(i int) { out = append(out, i) }, func() { out = append(out, -g.localSlot) })
	return out
}

func TestBoomSectorOrderSurvivesMovementAndTracksMembership(t *testing.T) {
	g := boomTouchFixture(t)
	if got := boomTouchOrder(g, 0); !reflect.DeepEqual(got, []int{2, 1, -1}) {
		t.Fatal(got)
	}
	g.setThingPosFixed(1, -4*fracUnit, 0)
	if g.thingBlockOrder[1] <= g.thingBlockOrder[2] {
		t.Fatal("fixture did not relink block order")
	}
	if got := boomTouchOrder(g, 1); !reflect.DeepEqual(got, []int{2, 1}) {
		t.Fatalf("movement reordered existing sector nodes: %v", got)
	}
	g.setThingPosFixed(1, -64*fracUnit, 0)
	if got := boomTouchOrder(g, 1); !reflect.DeepEqual(got, []int{2}) {
		t.Fatal(got)
	}
	g.setThingPosFixed(1, -4*fracUnit, 0)
	if got := boomTouchOrder(g, 1); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("new membership did not prepend: %v", got)
	}
	// Staying in sector 0 must retain its old node and ordering.
	if got := boomTouchOrder(g, 0); !reflect.DeepEqual(got, []int{2, 1, -1}) {
		t.Fatal(got)
	}
	g.thingCollected[2] = true
	g.updateThingBlockmapIndex(2)
	spawned := g.appendRuntimeThing(mapdata.Thing{Type: 9, X: 8, Flags: 7}, false)
	if got := boomTouchOrder(g, 1); !reflect.DeepEqual(got, []int{spawned, 1}) {
		t.Fatal(got)
	}
	if !validBoomSectorTouches(captureGameSaveState(g), 2) {
		t.Fatal("removal/spawn left invalid nodes")
	}
}

func TestBoomSectorWalkRestartsAfterRelinking(t *testing.T) {
	g := boomTouchFixture(t)
	var seen []int
	g.walkBoomSectorTouches(1, func(i int) {
		seen = append(seen, i)
		if len(seen) == 1 {
			g.updateBoomActorTouch(i, nil)
			g.updateBoomThingTouch(i)
		}
	}, func() { t.Fatal("unexpected player") })
	if !reflect.DeepEqual(seen, []int{2, 2, 1}) {
		t.Fatalf("walk did not visit the new node: %v", seen)
	}
}

func TestBoomSectorOrderBinaryCheckpointsAndLegacyDefaults(t *testing.T) {
	for _, version := range []int{saveGameVersion, keyframeVersion, 38, 25} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			g := boomTouchFixture(t)
			g.setThingPosFixed(1, -64*fracUnit, 0)
			g.setThingPosFixed(1, -4*fracUnit, 0)
			g.projectileImpacts = []projectileImpact{{kind: projectileFatShot, momx: 12345, momy: -6789, tics: 20}}
			g.doors = map[int]*doorThinker{0: {sector: 0, lightLine: 1}}
			state := captureGameSaveState(g)
			magic := boomSnapshotMagic(version)
			blob, err := encodeSnapshot(magic, saveFile{Version: version, Game: state})
			if err != nil {
				t.Fatal(err)
			}
			file, err := decodeSnapshot(blob, magic)
			if err != nil {
				t.Fatal(err)
			}
			if file.Game.Doors[0].LightLine != 1 {
				t.Fatal("previous door-lighting extension lost")
			}
			if version == 38 || version == 25 {
				if file.Game.BoomSectorTouches != nil || file.Game.ProjectileImpacts[0].MomX != 0 {
					t.Fatal("legacy format invented new state")
				}
				return
			}
			loaded := boomTouchFixture(t)
			restoreGameSaveState(loaded, file.Game)
			if !reflect.DeepEqual(g.captureBoomSectorTouches(), loaded.captureBoomSectorTouches()) || loaded.projectileImpacts[0].momx != 12345 || loaded.projectileImpacts[0].momy != -6789 {
				t.Fatal("checkpoint lost order/momentum")
			}
			g.setThingPosFixed(2, -64*fracUnit, 0)
			if !reflect.DeepEqual(boomTouchOrder(loaded, 1), []int{1, 2}) {
				t.Fatal("restored nodes alias the original world")
			}
			loaded.setThingPosFixed(2, -64*fracUnit, 0)
			if !reflect.DeepEqual(g.captureBoomSectorTouches(), loaded.captureBoomSectorTouches()) {
				t.Fatal("membership diverged after continuation")
			}
		})
	}
	// Magic must distinguish historical save 26 from current keyframe 26.
	for _, magic := range [][]byte{saveGameMagic, keyframeMagic} {
		blob, err := encodeSnapshot(magic, saveFile{Version: 26, Game: gameSaveState{WeaponPose: weaponPresentationPose{Valid: true}}})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(blob, magic)
		if err != nil {
			t.Fatal(err)
		}
		if file.Game.WeaponPose.Valid != bytes.Equal(magic, keyframeMagic) {
			t.Fatal("overlapping format versions confused")
		}
	}
}

func TestBoomSectorOrderAuthorityBaselineAndValidation(t *testing.T) {
	a, client, _ := snapshotFixture(t)
	a.g.boomMomentumDropoff = true
	a.g.initBoomSectorTouches()
	for _, p := range a.g.authorityPlayers {
		a.g.withAuthoritativePlayer(p, func() { a.g.updateBoomPlayerTouch() })
	}
	data, err := a.Snapshot(2)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.applyAuthoritySnapshot(data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.g.captureBoomSectorTouches(), client.captureBoomSectorTouches()) {
		t.Fatal("authority baseline lost sector order")
	}
	original := a.g.captureBoomSectorTouches()
	client.updateBoomActorTouch(-2, nil)
	if !reflect.DeepEqual(original, a.g.captureBoomSectorTouches()) {
		t.Fatal("baseline aliases server nodes")
	}
	r, err := decodeAuthorityReplica(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]boomSectorTouchState{
		{{Sector: -1, Actor: -1}}, {{Sector: len(a.g.m.Sectors), Actor: -1}},
		{{Sector: 0, Actor: -5}}, {{Sector: 0, Actor: len(a.g.m.Things)}},
		{{Sector: 0, Actor: -1}, {Sector: 0, Actor: -1}},
	} {
		r.Game.BoomSectorTouches = bad
		if err := validateAuthorityReplica(client, r); err == nil {
			t.Fatalf("accepted malformed membership %v", bad)
		}
	}
	a.RemovePlayer(2)
	for _, node := range a.g.captureBoomSectorTouches() {
		if node.Actor == -2 {
			t.Fatal("departed player remains in sector list")
		}
	}
}
