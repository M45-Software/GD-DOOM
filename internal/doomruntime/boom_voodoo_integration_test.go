//go:build integration

package doomruntime

import (
	"fmt"
	"os"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

// Controlled body-damage calls match dsda_voodoo_lifecycle_probe.inc. This
// tests shared player state and persistent body identity, not netdemo timing.
func TestBoomVoodooLifecycleCapture(t *testing.T) {
	path := os.Getenv("GD_VOODOO_OUT")
	if path == "" {
		t.Skip("run verify_boom_voodoo.py")
	}
	wf, err := wad.Open(os.Getenv("GD_VOODOO_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "MAP01")
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewAuthority(m, Options{SkillLevel: 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []byte{1, 2} {
		if err := a.AddPlayer(id); err != nil {
			t.Fatal(err)
		}
	}
	dolls := dollsForSlot(a.g, 2)
	if len(dolls) != 2 {
		t.Fatal("fixture needs two dolls")
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fmt.Fprintln(f, "event,slot,health,bodyhealth,dead,dolls")
	damage := func(i, n int) { a.g.damageShootableThingFrom(dolls[i], n, false, -1, 0, 0, false) }
	for event := 0; event < 7; event++ {
		switch event {
		case 1:
			damage(0, 30)
		case 2:
			damage(1, 20)
		case 3:
			damage(1, 50)
		case 4:
			damage(0, 70)
		case 5:
			if err := a.spawnAuthoritativePlayer(2, true); err != nil {
				t.Fatal(err)
			}
		case 6:
			damage(1, 30)
		}
		for slot := 1; slot <= 2; slot++ {
			p := a.players[slot]
			dead := 0
			if p.isDead {
				dead = 1
			}
			fmt.Fprintf(f, "%d,%d,%d,%d,%d,", event, slot, p.stats.Health, p.playerMobjHealth, dead)
			for _, idx := range dolls {
				corpse, solid, shootable := 0, 1, 1
				if a.g.thingDead[idx] {
					corpse, solid, shootable = 1, 0, 0
				}
				fmt.Fprintf(f, "%d:%d:%d:%d:%d;", a.g.thingHP[idx], corpse, a.g.thingCurrentHeight(idx, a.g.m.Things[idx]), solid, shootable)
			}
			fmt.Fprintln(f)
		}
	}
}
