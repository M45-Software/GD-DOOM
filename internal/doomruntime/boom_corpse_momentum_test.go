package doomruntime

import (
	"fmt"
	"testing"

	"gddoom/internal/mapdata"
)

func TestBoomNonSolidCorpseSlidesThroughActors(t *testing.T) {
	for _, boom := range []bool{false, true} {
		for _, phase := range []int{0, 4} {
			for _, blocker := range []int16{1, 30, 3004} {
				t.Run(fmt.Sprintf("boom%t_phase%d_block%d", boom, phase, blocker), func(t *testing.T) {
					m := &mapdata.Map{Name: "MAP01", Sectors: []mapdata.Sector{{CeilingHeight: 128}}, Things: []mapdata.Thing{
						{Type: 1, X: -256, Flags: 7}, {Type: 3001, Flags: 7}, {Type: blocker, X: 40, Flags: 7},
					}}
					g := newGame(m, Options{Headless: true})
					g.boomMomentumDropoff = boom
					g.thingDead[1], g.thingHP[1] = true, 0
					g.thingState[1], g.thingStatePhase[1] = monsterStateDeath, phase
					if blocker == 1 {
						g.p.x = 40 * fracUnit
					}
					g.setThingMomentum(1, 8*fracUnit, 0, 0)
					g.tickMonsterMomentum(1, m.Things[1])
					x, _ := g.thingPosFixed(1, m.Things[1])
					want := int64(0)
					if boom && phase == 4 {
						want = 8 * fracUnit
					}
					if x != want {
						t.Fatalf("corpse x=%d want=%d", x, want)
					}
				})
			}
		}
	}
}
