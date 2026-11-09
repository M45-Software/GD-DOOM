package doomruntime

import (
	"reflect"
	"testing"

	"gddoom/internal/dehacked"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func gameplayDefinitions(t *testing.T, text string) *dehacked.Definitions {
	t.Helper()
	patch, err := dehacked.Parse("gameplay.deh", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	defs, err := dehacked.Vanilla().Apply(patch)
	if err != nil {
		t.Fatal(err)
	}
	return defs
}

func patchedActorGame(t *testing.T, defs *dehacked.Definitions, skill int) *game {
	t.Helper()
	t.Cleanup(doomrand.Clear)
	m := predictionTestMap()
	m.Things[0].X = -80
	m.Things = append(m.Things, mapdata.Thing{Type: 3004, X: 32, Flags: skillMask})
	g, err := newGameWithDefinitions(m, Options{Headless: true, SkillLevel: skill}, defs)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestDehackedActorSpawnMovementDamageAndRaise(t *testing.T) {
	defs := gameplayDefinitions(t, "Thing 2\nHit points = 123\nReaction time = 17\nPain chance = 256\nSpeed = 3\nWidth = 786432\nHeight = 2621440\nMass = 200\n")
	digest := defs.Digest()
	g := patchedActorGame(t, defs, 3)
	th := g.m.Things[1]
	if g.thingHP[1] != 123 || g.thingReactionTics[1] != 17 {
		t.Fatalf("spawn health/reaction = %d/%d", g.thingHP[1], g.thingReactionTics[1])
	}
	if g.thingCurrentRadius(1, th) != 12*fracUnit || g.thingCurrentHeight(1, th) != 40*fracUnit {
		t.Fatal("patched bounds not used")
	}
	x, y := g.thingPosFixed(1, th)
	if !g.monsterMoveInDir(1, th.Type, monsterDirEast) {
		t.Fatal("patched actor could not move")
	}
	nx, ny := g.thingPosFixed(1, th)
	if nx != x+3*fracUnit || ny != y {
		t.Fatalf("move delta = %d,%d", nx-x, ny-y)
	}
	// Thrust uses C's signed numerator and the patched mass. Compare with a
	// vanilla actor subjected to the same hit, whose mass is 100.
	base := patchedActorGame(t, nil, 3)
	base.applyMonsterDamageThrust(1, 10, false, -1, 0, 0, true, 0, false, 123)
	g.setThingPosFixed(1, x, y)
	g.applyMonsterDamageThrust(1, 10, false, -1, 0, 0, true, 0, false, 123)
	if g.thingMomX[1] == 0 || g.thingMomX[1] != base.thingMomX[1]/2 {
		t.Fatalf("mass did not halve thrust: patched=%d vanilla=%d", g.thingMomX[1], base.thingMomX[1])
	}
	g.damageMonsterFrom(1, 1, false, -1, 0, 0, false)
	if g.thingHP[1] != 122 || !g.thingJustHit[1] {
		t.Fatal("patched health/pain not used by damage")
	}
	g.damageMonsterFrom(1, 200, false, -1, 0, 0, false)
	if !g.thingDead[1] || g.thingXDeath[1] {
		t.Fatal("xdeath must use patched spawn health threshold")
	}
	if g.thingCurrentHeight(1, th) != 10*fracUnit {
		t.Fatal("corpse height must quarter patched height")
	}
	g.startMonsterRaise(1)
	if g.thingHP[1] != 123 || g.thingCurrentHeight(1, th) != 40*fracUnit {
		t.Fatal("resurrection did not restore patched health/height")
	}
	if defs.Digest() != digest {
		t.Fatal("gameplay mutated definitions")
	}
	// Sessions never modify the shared baseline or each other.
	if base.thingHP[1] != 20 || base.thingCurrentHeight(1, th) != 56*fracUnit {
		t.Fatal("patch leaked into unpatched game")
	}
}

func TestDehackedNightmareReactionOverride(t *testing.T) {
	g := patchedActorGame(t, gameplayDefinitions(t, "Thing 2\nReaction time = 17\n"), 5)
	if g.thingReactionTics[1] != 0 {
		t.Fatal("nightmare must override patched spawn reaction")
	}
}

func TestDehackedGameplayRejectsUnsupportedBeforeRNGChanges(t *testing.T) {
	for _, text := range []string{
		"Frame 281\nDuration = 5\n", "Thing 2\nBits = SPECIAL\n",
		"Thing 2\nID # = 3004\n", "Thing 1\nHit points = 123\n",
		"Thing 2\nMass = 0\n", "Thing 2\nSpeed = -1\n",
		"Thing 2\nPain chance = 257\n", "Thing 31\nPain chance = 256\n", "[STRINGS]\nGOTARMOR = changed\n",
	} {
		t.Run(text, func(t *testing.T) {
			defs := gameplayDefinitions(t, text)
			doomrand.Clear()
			_ = doomrand.PRandom(doomrand.Painchance)
			before := doomrand.Capture()
			g, err := newGameWithDefinitions(predictionTestMap(), Options{Headless: true}, defs)
			if err == nil || g != nil {
				t.Fatal("unsupported patch accepted")
			}
			if !reflect.DeepEqual(before, doomrand.Capture()) {
				t.Fatal("failed validation altered RNG")
			}
		})
	}
	t.Cleanup(doomrand.Clear)
}

func TestDehackedVanillaDefinitionsPreserveSpawnAndRNG(t *testing.T) {
	a := patchedActorGame(t, nil, 3)
	rng := doomrand.Capture()
	b := patchedActorGame(t, dehacked.Vanilla(), 3)
	if !reflect.DeepEqual(rng, doomrand.Capture()) || !reflect.DeepEqual(captureGameSaveState(a), captureGameSaveState(b)) {
		t.Fatal("vanilla definitions changed baseline state or RNG")
	}
}

func TestDehackedRadiusChangesWallCollision(t *testing.T) {
	base := patchedActorGame(t, nil, 3)
	wide := patchedActorGame(t, gameplayDefinitions(t, "Thing 2\nWidth = 2621440\n"), 3)
	if !base.probeMonsterMove(1, 3004, 90*fracUnit, 0).ok {
		t.Fatal("baseline actor should fit beside wall")
	}
	if wide.probeMonsterMove(1, 3004, 90*fracUnit, 0).ok {
		t.Fatal("wide actor should hit wall")
	}
}

func TestDehackedZeroPainChanceStillConsumesNativeRoll(t *testing.T) {
	g := patchedActorGame(t, gameplayDefinitions(t, "Thing 2\nPain chance = 0\n"), 3)
	g.stats.Health = 0
	doomrand.Clear()
	g.damageMonsterFrom(1, 1, false, -1, 0, 0, false)
	if g.thingJustHit[1] {
		t.Fatal("zero chance entered pain")
	}
	if _, prnd := doomrand.State(); prnd != 1 {
		t.Fatalf("pain RNG=%d, want one consumed roll even at zero chance", prnd)
	}
}

func TestDehackedBarrelAndZeroBounds(t *testing.T) {
	defs := gameplayDefinitions(t, "Thing 31\nHit points = 70\nWidth = 0\nHeight = 0\n")
	m := predictionTestMap()
	m.Things = append(m.Things, mapdata.Thing{Type: 2035, X: 64, Flags: skillMask})
	g, err := newGameWithDefinitions(m, Options{Headless: true, SkillLevel: 3}, defs)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(doomrand.Clear)
	if g.thingHP[1] != 70 || g.thingCurrentRadius(1, m.Things[1]) != 0 || g.thingCurrentHeight(1, m.Things[1]) != 0 {
		t.Fatal("barrel properties or explicit zero bounds were ignored")
	}
}

func TestDehackedNightmareRespawnRestoresPatchedHealth(t *testing.T) {
	g := patchedActorGame(t, gameplayDefinitions(t, "Thing 2\nHit points = 123\nReaction time = 17\n"), 3)
	g.opts.RespawnMonsters = true
	g.thingDead[1], g.thingHP[1] = true, 0
	g.thingMoveCount[1] = 12 * doomTicsPerSecond
	g.worldTic = 1
	doomrand.Clear()
	for attempt := 0; attempt < 256 && len(g.m.Things) == 2; attempt++ {
		g.tickNightmareRespawn(1, g.m.Things[1])
	}
	if len(g.m.Things) != 3 || !g.thingCollected[1] {
		t.Fatal("nightmare respawn did not create replacement")
	}
	if g.thingHP[2] != 123 || g.thingReactionTics[2] != 18 {
		t.Fatalf("respawn health/reaction=%d/%d, want patched health and Doom's fixed 18-tic delay", g.thingHP[2], g.thingReactionTics[2])
	}
}

func TestDehackedZeroSpeedStillAllowsSuccessfulTryWalk(t *testing.T) {
	g := patchedActorGame(t, gameplayDefinitions(t, "Thing 2\nSpeed = 0\n"), 3)
	x, y := g.thingPosFixed(1, g.m.Things[1])
	doomrand.Clear()
	if !g.monsterTryWalk(1, 3004, monsterDirEast) {
		t.Fatal("stationary actor should successfully probe its current position")
	}
	nx, ny := g.thingPosFixed(1, g.m.Things[1])
	if nx != x || ny != y {
		t.Fatal("zero-speed actor moved")
	}
	if _, prnd := doomrand.State(); prnd != 1 {
		t.Fatalf("trywalk RNG=%d, want one successful-walk roll", prnd)
	}
}

func TestDehackedCeilingSpawnUsesPatchedHeight(t *testing.T) {
	defs := gameplayDefinitions(t, "Thing 25\nHeight = 1310720\n")
	m := predictionTestMap()
	m.Things = append(m.Things, mapdata.Thing{Type: 72, X: 64, Flags: skillMask})
	g, err := newGameWithDefinitions(m, Options{Headless: true, SkillLevel: 3}, defs)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(doomrand.Clear)
	z, _, ceil := g.thingSupportState(1, g.m.Things[1])
	if z != ceil-20*fracUnit {
		t.Fatalf("ceiling spawn z=%d, want %d", z, ceil-20*fracUnit)
	}
}
