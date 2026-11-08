package doomruntime

import (
	"fmt"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func monsterDoorRetryFixture(special uint16) *game {
	g := &game{m: &mapdata.Map{
		Things:   []mapdata.Thing{{Type: 3004, X: -24, Flags: 7}},
		Vertexes: []mapdata.Vertex{{X: 0, Y: -64}, {X: 0, Y: 64}},
		Linedefs: []mapdata.Linedef{{V1: 0, V2: 1, Special: special, Flags: mlTwoSided, SideNum: [2]int32{0, 1}}},
		Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}},
		Sectors:  []mapdata.Sector{{CeilingHeight: 128}, {CeilingHeight: 0}},
	}, thingCollected: []bool{false}, thingHP: []int{20}, thingDead: []bool{false},
		p: player{x: -128 * fracUnit}, opts: Options{SkillLevel: 3}}
	g.initPhysics()
	g.ensureMonsterAIState()
	return g
}

func TestBoomMonsterDoorUnstickVersionAndRNG(t *testing.T) {
	original := doomrand.Capture()
	t.Cleanup(func() { doomrand.Restore(original) })
	for _, tc := range []struct {
		name          string
		version       byte
		compatibility bool
		boom          bool
		draw          bool
	}{
		{"vanilla", 109, false, false, false},
		{"Boom 2.01", 201, false, true, false},
		{"Boom 2.02 compatibility", 202, true, true, false},
		{"Boom 2.02", 202, false, true, true},
		{"live Boom", 0, false, true, true},
	} {
		for _, success := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/roll-success=%v", tc.name, success), func(t *testing.T) {
				g := monsterDoorRetryFixture(1)
				g.boomMomentumDropoff = tc.boom
				if tc.version != 0 {
					g.opts.DemoScript = &demo.Script{Header: demo.Header{Version: tc.version, BoomCompatibility: tc.compatibility}}
				}
				var before, after doomrand.Snapshot
				for seed := uint32(1); ; seed++ {
					doomrand.ConfigureBoom(seed, true)
					before = doomrand.Capture()
					roll := doomrand.PRandom(doomrand.Trywalk)
					after = doomrand.Capture()
					if (roll&3 != 0) == success {
						break
					}
				}
				doomrand.Restore(before)
				want := true
				if tc.draw {
					want = success
				} else {
					after = before
				}
				if got := g.monsterMoveInDir(0, 3004, monsterDirEast); got != want {
					t.Fatalf("handled door result = %v, want %v", got, want)
				}
				if len(g.doors) == 0 {
					t.Fatal("the door must activate even when the retry roll fails")
				}
				if got := doomrand.Capture(); got != after {
					t.Fatalf("door retry RNG = %+v, want %+v", got, after)
				}
			})
		}
	}
}

func TestBoomUnhandledMonsterSpecialDoesNotDrawDoorRetryRNG(t *testing.T) {
	original := doomrand.Capture()
	t.Cleanup(func() { doomrand.Restore(original) })
	g := monsterDoorRetryFixture(11)
	g.boomMomentumDropoff = true
	doomrand.ConfigureBoom(1993, true)
	before := doomrand.Capture()
	if g.monsterMoveInDir(0, 3004, monsterDirEast) {
		t.Fatal("monster should not activate a switch exit")
	}
	if doomrand.Capture() != before {
		t.Fatal("unhandled special consumed a retry draw")
	}
}
