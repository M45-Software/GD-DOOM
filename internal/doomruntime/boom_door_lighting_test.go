package doomruntime

import (
	"fmt"
	"reflect"
	"testing"
)

func lightingDoorGame(special uint16) *game {
	g := sharedDoorGame(special)
	g.sectorCeil[0] = 0
	g.m.Sectors[0].CeilingHeight = 0
	g.m.Sectors[0].Light = 96
	g.m.Sectors[1].Light = 224
	g.stats.Health = 100
	g.p.x = 64 * fracUnit
	g.p.sector = 1
	g.inventory = lockInventory(63)
	g.initSectorPlaneLevelCache()
	return g
}

func TestBoomDoorLightingCompletionAndRetrigger(t *testing.T) {
	for _, special := range []uint16{1, 26, 27, 28, 31, 32, 33, 34, 117, 118, 0x3c06, 0x3c07, 0x3c26, 0x3c86, 0x3806, 0x3827} {
		t.Run(fmt.Sprint(special), func(t *testing.T) {
			g := lightingDoorGame(special)
			if !g.useSpecialLineForActor(0, 0, true) || g.doors[0].lightLine != 1 {
				t.Fatal("manual door did not retain its lighting line")
			}
			d := g.doors[0]
			for g.sectorCeil[0]+d.speed <= d.topHeight {
				g.tickDoor(0, d)
				if g.m.Sectors[0].Light != 96 {
					t.Fatal("light changed before pastdest")
				}
			}
			g.tickDoor(0, d)
			if g.m.Sectors[0].Light != 224 || g.sectorLightLevelCached(0) != 224 {
				t.Fatal("opening completion did not brighten world/render cache")
			}
			// The close endpoint takes the minimum of current and neighbor
			// levels, including a neighbor whose light changed while open.
			g.m.Sectors[1].Light = 48
			d.direction = -1
			for g.sectorCeil[0]-d.speed >= g.sectorFloor[0] {
				g.tickDoor(0, d)
				if g.m.Sectors[0].Light != 224 {
					t.Fatal("light changed during closing")
				}
			}
			g.tickDoor(0, d)
			if g.m.Sectors[0].Light != 48 {
				t.Fatal("closing completion did not use current neighboring light")
			}
		})
	}
}

func TestBoomDoorLightingNeighborRules(t *testing.T) {
	g := lightingDoorGame(1)
	// Self-referencing lines do not count as neighbors. An isolated tagged
	// sector opens to zero; its own light is only included in the dark bound.
	g.m.Linedefs[0].SideNum[0] = 1
	g.m.Sectors[1].Tag = 7
	g.m.Sectors[0].Tag = 0
	d := &doorThinker{sector: 0, topHeight: 124 * fracUnit, lightLine: 1}
	g.finishBoomDoorLighting(d, true)
	if g.m.Sectors[1].Light != 0 {
		t.Fatal("self-reference counted as a light source")
	}
	g.m.Sectors[1].Light = 96
	g.finishBoomDoorLighting(d, false)
	if g.m.Sectors[1].Light != 96 {
		t.Fatal("isolated closing sector lost its current light")
	}
	// A zero movement range suppresses the endpoint lighting call.
	d.topHeight = g.sectorFloor[0]
	g.finishBoomDoorLighting(d, true)
	if g.m.Sectors[1].Light != 96 {
		t.Fatal("zero-range door changed lights")
	}
}

func TestBoomDoorLightingCompatibilityAndTaggedDoors(t *testing.T) {
	g := lightingDoorGame(1)
	if !boomMomentumDropoffEnabled(g.m, Options{}) {
		t.Fatal("tagged manual door did not enable Boom map rules")
	}
	if boomMomentumDropoffEnabled(g.m, Options{DemoScript: &DemoScript{Header: DemoHeader{Version: 109}}}) {
		t.Fatal("vanilla demo enabled Boom door lighting")
	}
	g.boomMomentumDropoff = false
	g.evVerticalDoor(0, true)
	if g.doors[0].lightLine != 0 {
		t.Fatal("vanilla manual door acquired lighting")
	}
	for _, special := range []uint16{29, 0x3c02, 0x3802} {
		g := lightingDoorGame(special)
		if !g.useSpecialLineForActor(0, 0, true) || g.doors[0].lightLine != 0 {
			t.Fatalf("tag-operated door %d acquired lighting", special)
		}
	}
}

func TestBoomDoorLightingCheckpoints(t *testing.T) {
	for _, version := range []int{saveGameVersion, keyframeVersion, 37, 24} {
		g := lightingDoorGame(1)
		g.evVerticalDoor(0, true)
		g.tickDoors()
		// Preserve lighting on a displaced thinker as well as the owner.
		g.extraDoors = []*doorThinker{{order: 999, sector: 0, lightLine: 1, topHeight: 124 * fracUnit}}
		state := captureGameSaveState(g)
		state.Player.ViewGroundState = -1
		magic := saveGameMagic
		if version <= keyframeVersion {
			magic = keyframeMagic
		}
		data, err := encodeSnapshot(magic, saveFile{Version: version, Game: state})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(data, magic)
		if err != nil {
			t.Fatal(err)
		}
		if file.Game.Player.ViewGroundState != -1 {
			t.Fatal("previous-format camera state was lost")
		}
		if version == 37 || version == 24 {
			if file.Game.Doors[0].LightLine != 0 || file.Game.ExtraDoors[0].LightLine != 0 {
				t.Fatal("older format invented lighting state")
			}
			continue
		}
		if !reflect.DeepEqual(file.Game.Doors, state.Doors) || !reflect.DeepEqual(file.Game.ExtraDoors, state.ExtraDoors) {
			t.Fatal("binary checkpoint lost door lighting references")
		}
		loaded := lightingDoorGame(1)
		restoreGameSaveState(loaded, file.Game)
		g.extraDoors, loaded.extraDoors = nil, nil
		for range 280 {
			g.tickDoors()
			loaded.tickDoors()
			if g.m.Sectors[0].Light != loaded.m.Sectors[0].Light {
				t.Fatal("door lighting diverged after binary checkpoint")
			}
		}
	}
}

func TestBoomDoorLightingInvalidReferences(t *testing.T) {
	for _, pairs := range [][]int{{0}, {0, 0}, {0, -1}, {1, 1}, {0, 1, 0, 1}} {
		if restoreDoorLightingLines(map[int]doorThinkerSaveState{0: {}}, pairs) == nil {
			t.Fatalf("accepted malformed lighting references %v", pairs)
		}
	}
	if validDoorLightingState(gameSaveState{Doors: map[int]doorThinkerSaveState{0: {LightLine: 2}}}, 1) {
		t.Fatal("accepted a line outside the loaded map")
	}
	a, client, _ := snapshotFixture(t)
	a.g.doors = map[int]*doorThinker{0: {order: a.g.allocThinkerOrder(), sector: 0, lightLine: 1}}
	a.g.extraDoors = []*doorThinker{{order: a.g.allocThinkerOrder(), sector: 0, lightLine: 1}}
	data, err := a.Snapshot(2)
	if err != nil {
		t.Fatal(err)
	}
	if err = client.applyAuthoritySnapshot(data); err != nil {
		t.Fatal(err)
	}
	if client.doors[0].lightLine != 1 || client.extraDoors[0].lightLine != 1 {
		t.Fatal("authority snapshot lost lighting references")
	}
}
