package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestThingThinkerIndexTracksLiveEligibilityAndInsertedBodies(t *testing.T) {
	g := &game{m: &mapdata.Map{Things: []mapdata.Thing{{Type: 3001}, {Type: 2007}, {Type: 3004}}},
		thingThinkerOrder: []int64{30, 10, 20}, thingCollected: make([]bool, 3),
		thingMomX: make([]int64, 3), thingMomY: make([]int64, 3), thingMomZ: make([]int64, 3)}
	check := func(after int64, key int, order int64) {
		t.Helper()
		got, ok := g.nextThingThinkerAfter(after)
		if !ok || got.key != key || got.order != order {
			t.Fatalf("next after %d = %+v/%v, want key=%d order=%d", after, got, ok, key, order)
		}
	}
	check(0, 2, 20) // A stationary pickup does not initially have a thinker.
	g.thingMomX[1] = fracUnit
	check(0, 1, 10) // A conveyor can activate it after the index was built.
	g.thingCollected[2] = true
	check(10, 0, 30)
	// Rebirth can append a detached body with its historical order rather
	// than the new object's index order. It must participate in this tic.
	g.m.Things = append(g.m.Things, mapdata.Thing{Type: detachedPlayerBodyType})
	g.thingThinkerOrder = append(g.thingThinkerOrder, 25)
	g.thingCollected = append(g.thingCollected, false)
	check(10, 3, 25)
	check(25, 0, 30)
}

func TestThingThinkerIndexPreservesFallbackAndTieOrder(t *testing.T) {
	g := &game{m: &mapdata.Map{Things: []mapdata.Thing{{Type: 3001}, {Type: 3004}, {Type: 3002}}},
		thingThinkerOrder: []int64{7, 7, 0}}
	if got, ok := g.nextThingThinkerAfter(0); !ok || got.key != 2 || got.order != 3 {
		t.Fatalf("fallback thinker = %+v/%v", got, ok)
	}
	if got, ok := g.nextThingThinkerAfter(3); !ok || got.key != 0 || got.order != 7 {
		t.Fatalf("equal-order thinker = %+v/%v", got, ok)
	}
}

func TestThingThinkerIndexRefreshesAfterItemRespawn(t *testing.T) {
	original := doomrand.Capture()
	t.Cleanup(func() { doomrand.Restore(original) })
	g, _, _ := authoritativeTargetsTestGame()
	g.opts.GameMode = gameModeDeathmatch
	g.authorityRules = &authorityRulesState{}
	g.m.Things[0] = mapdata.Thing{Type: 2007, X: 32, Flags: 7}
	g.thingSpawnPoint = append([]mapdata.Thing(nil), g.m.Things...)
	g.thingThinkerOrder = []int64{4}
	g.thingCollected[0] = true
	g.nextThingThinkerAfter(0) // Cache the old order while the pickup is absent.
	g.worldTic, g.nextThinkerOrder = 100, 10
	g.authorityItemRespawns = map[int]uint64{0: 99}
	g.tickAuthoritativeItemRespawns()
	g.thingMomX[0] = fracUnit
	got, ok := g.nextThingThinkerAfter(0)
	if !ok || got.order != g.thingThinkerOrder[0] || got.order <= 4 {
		t.Fatalf("respawned thinker = %+v/%v, actual order %d", got, ok, g.thingThinkerOrder[0])
	}
}
