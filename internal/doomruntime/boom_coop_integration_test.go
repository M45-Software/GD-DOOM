//go:build integration

package doomruntime

import (
	"bufio"
	"fmt"
	"os"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func TestBoomCoopDemoCapture(t *testing.T) {
	path := os.Getenv("GD_COOP_OUT")
	if path == "" {
		t.Skip("run verify_boom_coop.py")
	}
	script, err := demo.Load(os.Getenv("GD_COOP_DEMO"))
	if err != nil {
		t.Fatal(err)
	}
	wf, err := wad.Open(os.Getenv("GD_COOP_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "MAP01")
	if err != nil {
		t.Fatal(err)
	}
	g := newGame(m, Options{Headless: true, DemoScript: script})
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := bufio.NewWriter(f)
	defer out.Flush()
	fmt.Fprintln(out, "tic,players,bodies,items,sectors,playindex,seeds")
	for tic := range script.Frames {
		if err := g.advanceDemoTic(); err != nil {
			t.Fatalf("tic %d: %v", tic, err)
		}
		fmt.Fprintf(out, "%d,", g.worldTic-1)
		for _, p := range g.authorityPlayers {
			g.withAuthoritativePlayer(p, func() {
				state := 0
				if g.playerReborn {
					state = 2
				} else if g.isDead {
					state = 1
				}
				pending := 10
				if g.inventory.PendingWeapon != 0 {
					pending = demoTraceWeaponID(g.inventory.PendingWeapon)
				}
				cards := 0
				for i, on := range g.inventory.cardKeys() {
					if on {
						cards |= 1 << i
					}
				}
				for i, on := range g.inventory.SkullKeys {
					if on {
						cards |= 1 << (i + 3)
					}
				}
				fmt.Fprintf(out, "%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d", g.localSlot, g.p.x, g.p.y, g.p.z, g.p.angle, g.p.momx, g.p.momy, g.p.momz, g.stats.Health, g.playerMobjHealth, g.stats.Armor, g.stats.Bullets, g.stats.Shells, g.stats.Cells, g.stats.Rockets, state, g.p.viewHeight, g.playerViewZ, demoTraceWeaponID(g.inventory.ReadyWeapon), pending, weaponDoomStateID(g.weaponState), g.weaponStateTics, boolToInt(g.weaponAttackDown), boolToInt(g.weaponRefire), cards)
				ax, ay := int64(0), int64(0)
				if g.statusHasAttacker {
					ax, ay = g.statusAttackerPosition()
				}
				fmt.Fprintf(out, ":%d:%d;", ax, ay)
			})
		}
		fmt.Fprint(out, ",")
		for _, m := range g.demoTraceMobjs() {
			if m.Player != 0 || m.Flags&1 != 0 {
				continue
			}
			owner := m.ownerSlot
			fmt.Fprintf(out, "%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d", m.Type, m.X, m.Y, m.Z, m.Angle, m.MomX, m.MomY, m.MomZ, m.Health, m.Tics, m.State, m.Radius, m.Height, m.FloorZ, m.CeilingZ, owner)
			if m.Kind == "monster" {
				idx := m.thingIndex
				tx, ty, _, _, _, _ := g.monsterAttackTargetPos(idx)
				fmt.Fprintf(out, ":%d:%d:%d:%d:%d:%d:%d:%d:%d:%d", g.monsterTargetPlayerSlot(idx), tx, ty, m.ReactionTime, m.Threshold, m.Movedir, m.Movecount, m.LastLook, boolToInt(g.thingJustHit[idx]), boolToInt(g.thingJustAtk[idx]))
			}
			fmt.Fprint(out, ";")
		}
		fmt.Fprint(out, ",")
		for _, m := range g.demoTraceMobjs() {
			if m.Flags&1 != 0 {
				fmt.Fprintf(out, "%d:%d:%d:%d;", m.Type, m.X, m.Y, m.Z)
			}
		}
		fmt.Fprint(out, ",")
		for i, s := range g.m.Sectors {
			fmt.Fprintf(out, "%d:%d:%d;", g.sectorFloor[i], g.sectorCeil[i], s.Light)
		}
		r := doomrand.Capture()
		fmt.Fprintf(out, ",%d,", r.PlayIndex)
		for cls, seed := range r.Seeds {
			if doomrand.Class(cls) != doomrand.Misc {
				fmt.Fprintf(out, "%d;", seed)
			}
		}
		fmt.Fprintln(out)
	}
}
