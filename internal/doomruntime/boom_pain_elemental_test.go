package doomruntime

import (
	"fmt"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestBoomPainSkullBlockedPathDoesNotSpawnOrConsumeRNG(t *testing.T) {
	for _, boom := range []bool{false, true} {
		for _, flags := range []uint16{0, mlTwoSided, mlTwoSided | mlBlocking, mlTwoSided | mlBlockMonsters} {
			t.Run(fmt.Sprintf("boom%t_flags%d", boom, flags), func(t *testing.T) {
				m := &mapdata.Map{Name: "MAP01", Sectors: []mapdata.Sector{{CeilingHeight: 128}, {CeilingHeight: 128}},
					Vertexes: []mapdata.Vertex{{X: 32, Y: -128}, {X: 32, Y: 128}},
					Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}},
					Linedefs: []mapdata.Linedef{{V1: 0, V2: 1, Flags: flags, SideNum: [2]int32{0, 1}}},
					Things:   []mapdata.Thing{{Type: 1, X: -256, Flags: 7}, {Type: 71, Flags: 7}},
				}
				g := newGame(m, Options{Headless: true})
				g.boomMomentumDropoff = boom
				before := doomrand.Capture()
				spawned := g.spawnPainLostSoul(1, 0)
				blocked := boom && flags != mlTwoSided
				if blocked {
					if spawned || len(m.Things) != 2 || doomrand.Capture() != before {
						t.Fatal("blocked path allocated a skull or consumed RNG")
					}
				} else if len(m.Things) != 3 || g.thingDead[2] {
					t.Fatal("allowed path did not spawn its skull")
				}
			})
		}
	}
}

func TestBoomPainSkullRemovesVanillaPopulationLimit(t *testing.T) {
	for _, boom := range []bool{false, true} {
		m := &mapdata.Map{Name: "MAP01", Sectors: []mapdata.Sector{{CeilingHeight: 128}}, Things: []mapdata.Thing{
			{Type: 1, X: -256, Flags: 7}, {Type: 71, Flags: 7},
		}}
		for i := 0; i < 21; i++ {
			m.Things = append(m.Things, mapdata.Thing{Type: 3006, X: 1024, Y: int16(i * 64), Flags: 7})
		}
		g := newGame(m, Options{Headless: true})
		g.boomMomentumDropoff = boom
		g.spawnPainLostSoul(1, 0)
		want := 21
		if boom {
			want++
		}
		if count := g.countActiveThingType(3006); count != want {
			t.Fatalf("boom=%t skulls=%d want=%d", boom, count, want)
		}
	}
}

func TestBoomPainSkullRejectsSpawnAboveCeiling(t *testing.T) {
	m := &mapdata.Map{Name: "MAP01", Sectors: []mapdata.Sector{{CeilingHeight: 128}}, Things: []mapdata.Thing{
		{Type: 1, X: -256, Flags: 7}, {Type: 71, Flags: 7},
	}}
	g := newGame(m, Options{Headless: true})
	g.boomMomentumDropoff = true
	g.setThingSupportState(1, 100*fracUnit, 0, 128*fracUnit)
	if g.spawnPainLostSoul(1, 0) || len(m.Things) != 3 || !g.thingDead[2] {
		t.Fatal("skull spawned above the ceiling was not killed in place")
	}
}
