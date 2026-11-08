//go:build integration

package doomruntime

import (
	"fmt"
	"testing"
)

// Follow the pistol-start route with real collision, pickups, key locks and
// door traces. Monsters are disabled here; recorded combat runs are separate.
func TestGraveyardMidwayProgression(t *testing.T) {
	t.Setenv("GD_GRAVEYARD_MAP", "E1M2")
	for _, skill := range []int{1, 3, 4} {
		t.Run(fmt.Sprint(skill), func(t *testing.T) {
			_, m := graveyardMap(t)
			g := newGame(m, Options{Headless: true, NoMonsters: true, SkillLevel: skill, PlayerSlot: 1})
			walk := func(x, y int) {
				t.Helper()
				tx, ty := int64(x)*fracUnit, int64(y)*fracUnit
				for n := 0; g.p.x != tx || g.p.y != ty; n++ {
					if n > 2500 {
						t.Fatal("route did not converge")
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
						t.Fatalf("blocked at %.0f,%.0f,z=%.0f toward %d,%d", float64(g.p.x)/fracUnit, float64(g.p.y)/fracUnit, float64(g.p.z)/fracUnit, x, y)
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
			use := func(angle float64) {
				t.Helper()
				g.p.angle = uint32(angle / 360 * 4294967296)
				g.useLines()
				for i := 0; i < 160; i++ {
					g.tickDoors()
				}
			}
			closed := func(angle float64, special uint16) {
				t.Helper()
				g.p.angle = uint32(angle / 360 * 4294967296)
				line, tr := g.peekUseTargetLine()
				if tr != useTraceSpecial || line < 0 || m.Linedefs[line].Special != special {
					t.Fatalf("missing key lock %d: line %d trace %v", special, line, tr)
				}
				use(angle)
				back := m.Sidedefs[m.Linedefs[line].SideNum[1]].Sector
				if g.sectorCeil[back] > 0 {
					t.Fatal("locked gate opened without its token")
				}
			}
			walk(544, 224)
			walk(864, 640)
			walk(1248, 640)
			closed(0, 32)
			walk(864, 640)
			walk(416, 640)
			walk(32, 704)
			use(180)
			walk(-352, 704)
			walk(-416, 1376)
			walk(-544, 1504)
			walk(-800, 1504)
			use(90)
			walk(-800, 1824)
			if !g.inventory.BlueKey {
				t.Fatal("blue token not picked up")
			}
			walk(-800, 2016)
			walk(160, 2016)
			walk(160, 1888)
			use(270)
			walk(160, 1696)
			walk(864, 1696)
			closed(90, 33)
			walk(416, 1696)
			walk(416, 640)
			walk(864, 640)
			walk(1248, 640)
			use(0)
			walk(1440, 640)
			walk(1440, 96)
			walk(2880, 96)
			walk(2880, 864)
			walk(2592, 864)
			walk(2592, 1056)
			walk(2848, 1056)
			walk(2848, 1120)
			if !g.inventory.RedKey {
				t.Fatal("red token not picked up")
			}
			walk(2592, 1056)
			walk(2592, 1344)
			walk(1952, 1344)
			walk(1952, 1504)
			use(90)
			walk(1952, 1760)
			walk(1760, 1760)
			walk(1760, 1888)
			walk(1312, 1888)
			walk(1312, 2464)
			walk(1760, 2464)
			walk(1760, 2528)
			use(90)
			if !g.levelExitRequested {
				t.Fatal("exit did not activate")
			}
		})
	}
}
