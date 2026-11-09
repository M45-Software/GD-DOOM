package doomruntime

import (
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"
	"reflect"
	"strings"
	"testing"
)

func TestDehackedZeroTicChainAndNullRemoval(t *testing.T) {
	defs := gameplayDefinitions(t, "Frame 174\nDuration = 0\nNext frame = 175\nFrame 175\nDuration = 0\nNext frame = 176\nFrame 176\nDuration = 2\nNext frame = 0\n[CODEPTR]\nFRAME 174 = NULL\nFRAME 175 = NULL\nFRAME 176 = NULL\n")
	g := patchedActorGame(t, defs, 3)
	doomrand.Clear()
	g.setExactDoomMonsterState(1, 3004, 174)
	if g.thingDoomState[1] != 176 || g.thingStateTics[1] != 2 {
		t.Fatalf("zero chain stopped at %d/%d", g.thingDoomState[1], g.thingStateTics[1])
	}
	g.tickExactDoomMonster(1, g.m.Things[1])
	if g.thingCollected[1] {
		t.Fatal("removed before duration expired")
	}
	g.tickExactDoomMonster(1, g.m.Things[1])
	if !g.thingCollected[1] || g.thingDoomState[1] != 0 {
		t.Fatal("S_NULL did not remove actor")
	}
	if _, prnd := doomrand.State(); prnd != 0 {
		t.Fatal("NULL actions consumed RNG")
	}
}

func TestDehackedInfiniteSpawnDoesNotRunAction(t *testing.T) {
	g := patchedActorGame(t, gameplayDefinitions(t, "Frame 174\nDuration = -1\n[CODEPTR]\nFRAME 174 = SPosAttack\n"), 3)
	if g.thingStateTics[1] != -1 {
		t.Fatal("infinite spawn duration lost")
	}
	before := doomrand.Capture()
	for range 10 {
		g.tickExactDoomMonster(1, g.m.Things[1])
	}
	if g.thingDoomState[1] != 174 || !reflect.DeepEqual(before, doomrand.Capture()) {
		t.Fatal("spawn action ran or infinite state advanced")
	}
}

func TestDehackedCrossTypeCodePointerUsesActionRNG(t *testing.T) {
	for _, test := range []struct {
		action string
		rolls  int
	}{{"PosAttack", 3}, {"SPosAttack", 9}, {"CPosAttack", 3}} {
		t.Run(test.action, func(t *testing.T) {
			g := patchedActorGame(t, gameplayDefinitions(t, "[CODEPTR]\nFRAME 176 = "+test.action+"\n"), 3)
			g.thingTargetPlayer[1] = true
			g.thingTargetIdx[1] = -1
			// Shoot away from the player, so blood/puff impact RNG cannot obscure
			// the action's own spread/damage stream. Move target beyond trace range.
			g.p.x = -4000 * fracUnit
			doomrand.Clear()
			doomrand.ConfigureBoom(123, false)
			before := doomrand.Capture()
			class := doomrand.Posattack
			if test.action == "SPosAttack" {
				class = doomrand.Sposattack
			}
			if test.action == "CPosAttack" {
				class = doomrand.Cposattack
			}
			for range test.rolls {
				_ = doomrand.PRandom(class)
			}
			expected := doomrand.Capture()
			doomrand.Restore(before)
			g.setExactDoomMonsterState(1, 3004, 176)
			actual := doomrand.Capture()
			if actual.Seeds[class] != expected.Seeds[class] {
				t.Fatalf("%s did not use its own action stream for %d rolls", test.action, test.rolls)
			}
		})
	}
}

func TestDehackedStateRootsAndCanonicalSprite(t *testing.T) {
	g := patchedActorGame(t, gameplayDefinitions(t, "Thing 2\nInitial frame = 207\nFirst moving frame = 208\nInjury frame = 218\nFar attack frame = 215\nFrame 207\nDuration = 2\n"), 3)
	if g.thingDoomState[1] != 207 || g.thingStateTics[1] < 1 || g.thingStateTics[1] > 2 {
		t.Fatal("patched initial state did not spawn/randomize")
	}
	prefix, _ := g.monsterStateSpritePrefix(1, g.m.Things[1])
	if prefix != "SPOS" || g.monsterFrameLetter(1, g.m.Things[1], 0) != 'A' {
		t.Fatal("state sprite metadata did not follow state redirect")
	}
	if g.monsterDoomSeeState(3004) != 208 || g.monsterDoomPainState(3004) != 218 || g.monsterDoomMissileState(3004) != 215 {
		t.Fatal("state roots ignored")
	}
}

func TestDehackedStateValidationAndActionBudget(t *testing.T) {
	for _, patch := range []string{
		"Frame 174\nDuration = 0\nNext frame = 174\n",
		"Frame 174\nDuration = -2\n", "Frame 174\nNext frame = 763\n",
		"Frame 174\nSprite number = 1\n", "[CODEPTR]\nFRAME 174 = VileChase\n",
		"Thing 25\nInitial frame = 174\n",
	} {
		if err := ValidateDehackedDefinitions(gameplayDefinitions(t, patch)); err == nil {
			t.Fatalf("accepted unsupported/unsafe patch %q", patch)
		}
	}
	// Action-driven recursion must also terminate even though neither frame
	// has zero duration. Look repeatedly chooses a patched Look see state.
	g := patchedActorGame(t, gameplayDefinitions(t, "Thing 2\nFirst moving frame = 174\n"), 3)
	g.thingTargetPlayer[1] = true
	g.thingTargetIdx[1] = -1
	g.setThingWorldAngle(1, degToAngle(180))
	g.sectorSoundTarget = []bool{true}
	g.setExactDoomMonsterState(1, 3004, 174)
	if g.definitionErr == nil || !strings.Contains(g.definitionErr.Error(), "budget") {
		t.Fatal("recursive action transitions were not bounded")
	}
}

func TestDehackedSaveIdentityAndLevelRebuild(t *testing.T) {
	defs := gameplayDefinitions(t, "Thing 2\nHit points = 123\nFrame 174\nDuration = -1\n")
	m := predictionTestMap()
	m.Things = append(m.Things, mapdata.Thing{Type: 3004, X: 32, Flags: skillMask})
	opts := Options{Headless: true, SkillLevel: 3, DehackedDefinitions: defs, NewGameLoader: func(string) (*mapdata.Map, error) { return cloneMapForRestart(m), nil }}
	sg := &sessionGame{opts: opts, current: m.Name}
	sg.g = sg.buildGame(cloneMapForRestart(m), opts)
	sg.g.setExactDoomMonsterState(1, 3004, 174)
	data, err := sg.marshalSaveGame("patched")
	if err != nil {
		t.Fatal(err)
	}
	restored := &sessionGame{opts: opts, current: m.Name}
	if err := restored.unmarshalSaveGame(data); err != nil {
		t.Fatal(err)
	}
	if restored.g.thingHP[1] != 123 || restored.g.thingDoomState[1] != 174 || restored.g.thingStateTics[1] != -1 {
		t.Fatal("patched state/definitions lost on cold restore")
	}
	wrong := opts
	wrong.DehackedDefinitions = gameplayDefinitions(t, "Thing 2\nHit points = 124\nFrame 174\nDuration = -1\n")
	mismatch := &sessionGame{opts: wrong}
	before := doomrand.Capture()
	if err := mismatch.unmarshalSaveGame(data); err == nil || !strings.Contains(err.Error(), "definitions") {
		t.Fatal("accepted different patch set")
	}
	if !reflect.DeepEqual(before, doomrand.Capture()) || mismatch.g != nil {
		t.Fatal("rejected restore changed live state")
	}
	unpatched := &sessionGame{opts: Options{NewGameLoader: opts.NewGameLoader}}
	if err := unpatched.unmarshalSaveGame(data); err == nil {
		t.Fatal("accepted patched save without definitions")
	}
	// Campaign/rebirth constructors carry the immutable definitions in Options.
	next := newGameWithRNG(cloneMapForRestart(m), opts, false)
	if next.thingHP[1] != 123 || next.thingStateTics[1] != -1 {
		t.Fatal("level rebuild lost definitions")
	}
	t.Cleanup(doomrand.Clear)
}

func TestDehackedNativeSaveAndNetworkBoundary(t *testing.T) {
	defs := gameplayDefinitions(t, "Thing 2\nHit points = 123\nFrame 174\nDuration = -1\n")
	m := predictionTestMap()
	m.Things = append(m.Things, mapdata.Thing{Type: 3004, X: 32, Flags: skillMask})
	opts := Options{Headless: true, SkillLevel: 3, DehackedDefinitions: defs, NewGameLoader: func(string) (*mapdata.Map, error) { return cloneMapForRestart(m), nil }}
	c := NewNativeCampaign(NewNativeMeshGame(cloneMapForRestart(m), opts), opts, nil)
	blob, err := c.CaptureKeyframe()
	if err != nil {
		t.Fatal(err)
	}
	same := NewNativeCampaign(NewNativeMeshGame(cloneMapForRestart(m), opts), opts, nil)
	if err := same.LoadKeyframe(blob); err != nil {
		t.Fatal(err)
	}
	if same.Game.g.thingHP[1] != 123 || same.Game.g.thingStateTics[1] != -1 {
		t.Fatal("native keyframe lost definitions")
	}
	plain := opts
	plain.DehackedDefinitions = nil
	unpatched := NewNativeCampaign(NewNativeMeshGame(cloneMapForRestart(m), plain), plain, nil)
	before := doomrand.Capture()
	if err := unpatched.LoadKeyframe(blob); err == nil {
		t.Fatal("native host accepted mismatched definitions")
	}
	if !reflect.DeepEqual(before, doomrand.Capture()) {
		t.Fatal("rejected native load altered RNG")
	}
	if _, err := NewAuthority(m, opts); err == nil {
		t.Fatal("patched authority accepted without definition negotiation")
	}
	sg := &sessionGame{opts: opts}
	if sg.multiplayerMenuAvailable() {
		t.Fatal("patched session exposed multiplayer")
	}
	sg.beginAuthorityJoin()
	if !strings.Contains(sg.multiplayer.status, "DEHACKED") {
		t.Fatal("patched menu join did not explain limitation")
	}
	if err := sg.installAuthorityJoin(runtimecfg.AuthorityJoinResult{}); err == nil || !strings.Contains(err.Error(), "DeHackEd") {
		t.Fatal("patched direct join not rejected")
	}
	t.Cleanup(doomrand.Clear)
}

func TestDehackedCrossTypeSkullRequiresDamageIntegration(t *testing.T) {
	for _, patch := range []string{
		"[CODEPTR]\nFRAME 185 = SkullAttack\n",
		"Thing 2\nInitial frame = 589\n",
		"Frame 186\nNext frame = 590\n",
	} {
		if err := ValidateDehackedDefinitions(gameplayDefinitions(t, patch)); err == nil || !strings.Contains(err.Error(), "SkullAttack") {
			t.Fatalf("unsafe cross-type charge accepted: %v", err)
		}
	}
}

func TestDehackedPublicConstructorsRejectBeforeInitialization(t *testing.T) {
	defs := gameplayDefinitions(t, "[CODEPTR]\nFRAME 763 = VileChase\n")
	opts := Options{Headless: true, DehackedDefinitions: defs}
	before := doomrand.Capture()
	rt, meta := NewRuntime(predictionTestMap(), opts, nil)
	if meta.Err() == nil || rt.Update() == nil {
		t.Fatal("runtime constructor did not surface unsupported definitions")
	}
	meta.Close()
	native := NewNativeMeshGame(predictionTestMap(), opts)
	if native.g.definitionErr == nil || native.Frame(1).Message == "" {
		t.Fatal("native constructor did not surface unsupported definitions")
	}
	native.Tick(NativeMeshInput{})
	if !reflect.DeepEqual(before, doomrand.Capture()) {
		t.Fatal("rejected constructors consumed RNG")
	}
}
