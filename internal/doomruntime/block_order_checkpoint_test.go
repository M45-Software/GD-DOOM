package doomruntime

import (
	"reflect"
	"testing"

	"gddoom/internal/mapdata"
)

func TestColdSnapshotPreservesCollisionLinkOrder(t *testing.T) {
	m := &mapdata.Map{Name: "MAP01", Sectors: []mapdata.Sector{{CeilingHeight: 128}},
		BlockMap: &mapdata.BlockMap{Width: 1, Height: 1, Cells: [][]int16{{}}},
		Things:   []mapdata.Thing{{Type: 1, X: 256}, {Type: 3001, X: 32, Y: 32, Flags: 7}, {Type: 3001, X: 64, Y: 32, Flags: 7}}}
	g := newGame(cloneMapForRestart(m), Options{Headless: true})
	// Relinking an actor changes its position in the collision list even
	// when it stays inside the same block. Map/thinker order cannot recover it.
	g.setThingPosFixed(1, 32*fracUnit, 32*fracUnit)
	order := func(g *game) []int {
		var result []int
		g.walkActorBlockCell(0, func(i int) { result = append(result, i) }, func() {})
		return result
	}
	want := order(g)
	if !reflect.DeepEqual(want, []int{1, 2}) {
		t.Fatalf("fixture order=%v", want)
	}
	for _, keyframe := range []bool{false, true} {
		magic, version := saveGameMagic, saveGameVersion
		if keyframe {
			magic, version = keyframeMagic, keyframeVersion
		}
		data, err := encodeSnapshot(magic, saveFile{Version: version, Game: captureGameSaveState(g)})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(data, magic)
		if err != nil {
			t.Fatal(err)
		}
		fresh := newGame(cloneMapForRestart(m), Options{Headless: true})
		restoreGameSaveState(fresh, file.Game)
		if got := order(fresh); !reflect.DeepEqual(got, want) {
			t.Fatalf("keyframe=%v order=%v want=%v", keyframe, got, want)
		}
		if fresh.nextBlockmapOrder != g.nextBlockmapOrder {
			t.Fatal("collision-link counter changed")
		}
	}
}
