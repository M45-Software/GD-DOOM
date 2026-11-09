package doomruntime

import (
	"fmt"
	"gddoom/internal/dehacked"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/sessionflow"
	"reflect"
	"testing"
)

func TestDehackedAllMonsterStateRoots(t *testing.T) {
	for _, typ := range definitionMonsterTypes {
		t.Run(fmt.Sprint(typ), func(t *testing.T) {
			slot, actor, _ := dehacked.Vanilla().ThingForEditorNumber(int32(typ))
			defs := gameplayDefinitions(t, fmt.Sprintf("Thing %d\nInitial frame = %d\n", slot+1, actor.SpawnState))
			m := predictionTestMap()
			m.Things[0].X = -80
			m.Things = append(m.Things, mapdata.Thing{Type: typ, X: 32, Flags: skillMask})
			g, err := newGameWithDefinitions(m, Options{Headless: true, SkillLevel: 3}, defs)
			if err != nil {
				t.Fatal(err)
			}
			if !g.patchedStateRuntime(typ) || g.thingDoomState[1] != int(actor.SpawnState) {
				t.Fatal("canonical spawn root not installed")
			}
			if g.thingHP[1] != int(actor.SpawnHealth) {
				t.Fatal("canonical spawn health not installed")
			}
		})
	}
	t.Cleanup(doomrand.Clear)
}

func TestDehackedCanonicalStatesPreserveUnmodifiedExactThinkers(t *testing.T) {
	states := stateOverrides(gameplayDefinitions(t, "Thing 25\nBits = 0\n"))
	for state, canonical := range states {
		if legacy, ok := monsterDoomStateDef(state); ok && canonical != legacy {
			t.Fatalf("unmodified state %d changed: canonical=%+v, legacy=%+v", state, canonical, legacy)
		}
	}
}

func TestDehackedFastDemonStateSlots(t *testing.T) {
	defs := gameplayDefinitions(t, "Thing 2\nInitial frame = 174\nFrame 477\nDuration = 3\nFrame 478\nDuration = 1\nFrame 479\nDuration = -1\n[CODEPTR]\nFRAME 477 = NULL\n")
	g := patchedActorGame(t, defs, 3)
	g.opts.FastMonsters = true
	g.boomMomentumDropoff = true
	for state, want := range map[int]int{477: 1, 478: 1, 479: -1, 174: 10} {
		def, ok := g.monsterDoomStateDef(state)
		if !ok || def.tics != want {
			t.Fatalf("fast state %d: tics=%d, want %d", state, def.tics, want)
		}
	}
	// Fast timing belongs to the state slot, independent of its current actor.
	g.setExactDoomMonsterState(1, 3004, 477)
	if g.thingStateTics[1] != 1 {
		t.Fatal("cross-type demon state did not receive fast timing")
	}
}

func TestDehackedDeathFallRaiseAndCheckpointFlags(t *testing.T) {
	defs := gameplayDefinitions(t, "Thing 2\nInitial frame = 174\nHit points = 123\n")
	g := patchedActorGame(t, defs, 3)
	doomrand.Clear()
	g.damageMonster(1, 200)
	flags, _ := g.definitionFlags(1)
	if !g.thingDead[1] || flags&4 != 0 || flags&0x100000 == 0 || flags&2 == 0 {
		t.Fatal("kill flags did not retain solidity until Fall")
	}
	for n := 0; n < 30 && g.thingDefinitionFlags[1]&2 != 0; n++ {
		g.tickDefinitionActor(1)
	}
	flags, _ = g.definitionFlags(1)
	if flags&2 != 0 || g.thingBlocksInSession(1) {
		t.Fatal("A_Fall did not clear solidity")
	}
	sg := &sessionGame{g: g, current: g.m.Name, opts: g.opts}
	sg.opts.NewGameLoader = func(string) (*mapdata.Map, error) { return cloneMapForRestart(g.restartTemplate), nil }
	blob, err := sg.marshalNetplayKeyframe()
	if err != nil {
		t.Fatal(err)
	}
	fresh := &sessionGame{opts: sg.opts}
	if err := fresh.unmarshalNetplayKeyframe(blob); err != nil {
		t.Fatal(err)
	}
	restored, _ := fresh.g.definitionFlags(1)
	if flags != restored || fresh.g.thingBlocksInSession(1) {
		t.Fatal("cold checkpoint lost action-mutated flags")
	}
	fresh.g.startMonsterRaise(1)
	raised, _ := fresh.g.definitionFlags(1)
	if raised&6 != 6 || raised&0x100000 != 0 || fresh.g.thingHP[1] != 123 || fresh.g.thingDoomState[1] != 203 {
		t.Fatal("raise failed to restore definition flags, health and state")
	}
}

func TestDehackedKeenFlagsControlSpawnAndDamage(t *testing.T) {
	defs := gameplayDefinitions(t, "Thing 25\nBits = 0\nHit points = 10000\nMass = 1000\nSpeed = 12\nFrame 763\nDuration = 2\nNext frame = 764\n[CODEPTR]\nFRAME 764 = VileChase\n")
	m := predictionTestMap()
	m.Sectors[0].FloorHeight = -64
	m.Things = append(m.Things, mapdata.Thing{Type: 72, X: 32, Flags: skillMask})
	g, err := newGameWithDefinitions(m, Options{Headless: true, SkillLevel: 4, NoMonsters: true}, defs)
	if err != nil {
		t.Fatal(err)
	}
	if g.thingCollected[1] || g.levelKillsTotal != 0 || g.thingBlocksInSession(1) {
		t.Fatal("cleared COUNTKILL/SOLID flags ignored")
	}
	z, floor, _ := g.thingSupportState(1, g.m.Things[1])
	if z != floor {
		t.Fatal("cleared SPAWNCEILING still spawned Keen on ceiling")
	}
	g.damageShootableThing(1, 100)
	if g.thingHP[1] != 10000 {
		t.Fatal("cleared SHOOTABLE flag ignored")
	}
	t.Cleanup(doomrand.Clear)
}

func TestDehackedKeenFlagsControlSkullAndProjectileCollision(t *testing.T) {
	for _, solid := range []bool{false, true} {
		bits := 0
		if solid {
			bits = 2
		}
		defs := gameplayDefinitions(t, fmt.Sprintf("Thing 25\nBits = %d\n", bits))
		m := predictionTestMap()
		m.Things[0].X = -80
		m.Things = append(m.Things, mapdata.Thing{Type: 72, Flags: skillMask}, mapdata.Thing{Type: 3006, X: 64, Flags: skillMask})
		g, err := newGameWithDefinitions(m, Options{Headless: true, SkillLevel: 3}, defs)
		if err != nil {
			t.Fatal(err)
		}
		if _, hit := g.lostSoulChargeTargetAt(2, g.m.Things[2], 0, 0, 0); hit != solid {
			t.Fatalf("skull charge hit=%v, solidity=%v", hit, solid)
		}
		if probe := g.probeSkullFlyMove(2, 3006, 0, 0); probe.hitTarget != solid {
			t.Fatalf("skull move hit=%v, solidity=%v", probe.hitTarget, solid)
		}
		if _, hit := g.projectileThingHitAtPosition(projectile{sourceThing: 2, radius: 6 * fracUnit, height: 8 * fracUnit}, 0, 0, 0); hit != solid {
			t.Fatalf("projectile hit=%v, solidity=%v", hit, solid)
		}
	}
	t.Cleanup(doomrand.Clear)
}

func TestDehackedVileChaseUsesCallerSpeedVileRadiusAndCanonicalHeal(t *testing.T) {
	defs := gameplayDefinitions(t, "Thing 2\nInitial frame = 174\nThing 25\nBits = 0\nSpeed = 12\n[CODEPTR]\nFRAME 764 = VileChase\n")
	m := predictionTestMap()
	m.Things[0].X = -80
	// Trial x=50-12=38: outside corpse+Keen radii (36), within
	// corpse+arch-vile radii (40), as required by A_VileChase.
	m.Things = append(m.Things, mapdata.Thing{Type: 3004, Flags: skillMask}, mapdata.Thing{Type: 72, X: 50, Flags: skillMask})
	g, err := newGameWithDefinitions(m, Options{Headless: true, SkillLevel: 3}, defs)
	if err != nil {
		t.Fatal(err)
	}
	g.damageMonster(1, 21)
	for n := 0; n < 80 && g.thingStateTics[1] != -1; n++ {
		g.tickDefinitionActor(1)
	}
	if g.thingStateTics[1] != -1 {
		t.Fatal("corpse did not reach terminal state")
	}
	g.thingMoveDir[2] = monsterDirWest
	g.setExactDoomMonsterState(2, 72, 764)
	if g.thingDoomState[2] != 266 || g.thingDoomState[1] != 203 || g.thingHP[1] != 20 {
		t.Fatalf("cross-type heal/raise states=%d/%d health=%d", g.thingDoomState[2], g.thingDoomState[1], g.thingHP[1])
	}
	// The heal chain's next state is the compiled arch-vile run state, not
	// the Keen's zero see-state root.
	if def, _ := g.monsterDoomStateDef(268); def.next != 243 {
		t.Fatal("heal chain redirected through source see-state")
	}
	t.Cleanup(doomrand.Clear)
}

func TestDehackedPresentationStringsAndIsolation(t *testing.T) {
	defs := gameplayDefinitions(t, "[STRINGS]\nHUSTR_30 = MAP30: God Machine\nC4TEXT = Patched finale.\\nNext line.\n")
	if err := ValidateDehackedDefinitions(defs); err != nil {
		t.Fatal(err)
	}
	opts := Options{DehackedDefinitions: defs, MusicPlayerCatalog: []runtimecfg.MusicPlayerWAD{{Episodes: []runtimecfg.MusicPlayerEpisode{{Tracks: []runtimecfg.MusicPlayerTrack{{MapName: "MAP30", Label: "Original"}}}}}}}
	next := applyDefinitionPresentationOptions(opts)
	if next.MusicPlayerCatalog[0].Episodes[0].Tracks[0].Label != "MAP30: God Machine" || opts.MusicPlayerCatalog[0].Episodes[0].Tracks[0].Label != "Original" {
		t.Fatal("patched title missing or options mutated")
	}
	state, _ := sessionflow.StartCommercialFinale("MAP30", false)
	state = (&sessionGame{opts: opts}).applyDefinitionFinaleText(state)
	if state.Text != "Patched finale.\nNext line." {
		t.Fatalf("finale=%q", state.Text)
	}
}

func TestPlayerStartsIgnoreSkillFilterForVoodooBodies(t *testing.T) {
	m := predictionTestMap()
	m.Things = []mapdata.Thing{{Type: 1, X: 32, Flags: 2}, {Type: 1, X: -80, Flags: 7}}
	g := newGame(m, Options{Headless: true, SkillLevel: 4})
	if !isVoodooDollType(g.m.Things[0].Type) || g.thingCollected[0] || g.thingHP[0] != 100 {
		t.Fatal("player start/doll filtered by ordinary actor skill flags")
	}
	baseline := doomrand.Capture()
	noSkill := cloneMapForRestart(m)
	noSkill.Things[0].Flags = 7
	newGame(noSkill, Options{Headless: true, SkillLevel: 4})
	if !reflect.DeepEqual(baseline, doomrand.Capture()) {
		t.Fatal("player skill bits changed spawn RNG")
	}
	t.Cleanup(doomrand.Clear)
}
