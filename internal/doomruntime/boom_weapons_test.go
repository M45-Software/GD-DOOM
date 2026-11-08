package doomruntime

import (
	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"
	"reflect"
	"testing"
)

func TestBoomEmptyWeaponWaitsForCommand(t *testing.T) {
	g := newGame(predictionTestMap(), Options{Headless: true, BoomGameOptions: &demo.GameOptions{Seed: 1993, Insurance: true}})
	g.stats.Bullets = 0
	g.stats.Shells = 2
	g.inventory.Weapons[82] = true
	g.weaponState = weaponStatePistolReady
	g.weaponStateTics = 1
	g.weaponPSpriteY = weaponTopY
	before := doomrand.Capture()
	g.fireWeaponStateSequence()
	if g.inventory.PendingWeapon != 0 || g.inventory.ReadyWeapon != weaponPistol || g.weaponState != weaponStatePistolReady || doomrand.Capture() != before {
		t.Fatal("empty Boom weapon invented a switch or an attack")
	}
	g.weaponAttackDown = true
	tc := g.buildOutgoingDemoTic(moveCmd{}, false, true)
	if tc.Buttons != demoButtonAttack|demoButtonChange|byte(8<<3) {
		t.Fatalf("fallback command %#x does not encode direct SSG", tc.Buttons)
	}
	cmd, _, _ := demoTicCommand(tc)
	g.selectCommandWeapon(cmd.weaponSlot)
	if g.inventory.PendingWeapon != weaponSuperShotgun {
		t.Fatal("encoded fallback did not select SSG")
	}
}
func TestBoomCommandSelectionsAreExact(t *testing.T) {
	for _, boom := range []bool{false, true} {
		g := newGame(predictionTestMap(), Options{Headless: true})
		g.boomMomentumDropoff = boom
		g.inventory.Weapons[2001] = true
		g.inventory.Weapons[82] = true
		g.inventory.Weapons[2005] = true
		g.selectCommandWeapon(3)
		want := weaponSuperShotgun
		if boom {
			want = weaponShotgun
		}
		if g.inventory.PendingWeapon != want {
			t.Fatalf("boom=%t shotgun command selected %v", boom, g.inventory.PendingWeapon)
		}
		g.inventory.PendingWeapon = 0
		g.selectCommandWeapon(1)
		want = weaponChainsaw
		if boom {
			want = weaponFist
		}
		if g.inventory.PendingWeapon != want {
			t.Fatal("fist command used the wrong toggle policy")
		}
	}
}
func TestBoomFallbackThresholds(t *testing.T) {
	g := newGame(predictionTestMap(), Options{Headless: true})
	g.inventory.ReadyWeapon = weaponRocketLauncher
	g.inventory.Weapons[82] = true
	g.stats.Bullets = 0
	g.stats.Shells = 2
	if g.fallbackWeapon(true) != weaponSuperShotgun || g.fallbackWeapon(false) != weaponFist {
		t.Fatal("SSG compatibility threshold differs")
	}
	g.stats.Shells = 0
	g.stats.Cells = 40
	g.inventory.Weapons[2006] = true
	if g.fallbackWeapon(true) != weaponBFG || g.fallbackWeapon(false) != weaponFist {
		t.Fatal("BFG compatibility threshold differs")
	}
}
func TestBoomAutomaticSwitchReleaseAndPending(t *testing.T) {
	g := newGame(predictionTestMap(), Options{Headless: true, BoomGameOptions: &demo.GameOptions{Seed: 1993}})
	g.inventory.ReadyWeapon = weaponPistol
	g.stats.Bullets = 0
	g.weaponAttackDown = true
	if slot := g.localWeaponCommand(0, false); slot != 1 {
		t.Fatal("first out-of-ammo input failed to select fallback after release")
	}
	g.inventory.PendingWeapon = weaponFist
	if slot := g.localWeaponCommand(0, true); slot != 0 {
		t.Fatal("pending fallback was emitted repeatedly")
	}
}

func TestBoomPreferenceInputAndAuthoritativeCommands(t *testing.T) {
	prefs, err := runtimecfg.ParseWeaponPreferences("fist,shotgun,bfg")
	if err != nil {
		t.Fatal(err)
	}
	g := newGame(predictionTestMap(), Options{Headless: true, WeaponPreferences: prefs, BoomGameOptions: &demo.GameOptions{Seed: 1993}})
	g.inventory.Weapons[2001], g.inventory.Weapons[82], g.inventory.Weapons[2005] = true, true, true
	g.inventory.Weapons[2004], g.inventory.Weapons[2006] = true, true
	g.stats.Bullets, g.stats.Shells, g.stats.Cells = 0, 2, 40
	g.inventory.ReadyWeapon = weaponPistol
	if got := g.localWeaponCommand(3, false); got != 3 {
		t.Fatalf("preferred shotgun key emitted %d", got)
	}
	g.inventory.Strength = true
	if got := g.localWeaponCommand(1, false); got != 1 {
		t.Fatalf("berserk fist key emitted %d", got)
	}
	g.weaponAttackDown = true
	if got := g.localWeaponCommand(0, true); got != 1 {
		t.Fatalf("berserk fallback emitted %d", got)
	}
	g.inventory.Strength = false
	g.boomAutoSwitchDone = false
	if got := g.localWeaponCommand(0, true); got != 3 {
		t.Fatalf("unpowered fallback emitted %d", got)
	}
	// Vanilla fallback is simulation policy and never uses the local order.
	if got := g.fallbackWeapon(false); got != weaponPlasma {
		t.Fatalf("vanilla fallback=%v", got)
	}
	// Recorded and received command codes are direct weapon identities, even
	// when the local preference and toggle key would choose another weapon.
	g.inventory.PendingWeapon = 0
	g.selectCommandWeapon(9)
	if g.inventory.PendingWeapon != weaponSuperShotgun {
		t.Fatal("direct SSG command was reinterpreted")
	}
	g.inventory.PendingWeapon = 0
	g.selectCommandWeapon(8)
	if g.inventory.PendingWeapon != weaponChainsaw {
		t.Fatal("direct chainsaw command was reinterpreted")
	}
}

func TestBoomPreferencesDoNotChangeRecordedPlayback(t *testing.T) {
	custom, _ := runtimecfg.ParseWeaponPreferences("fist,shotgun,bfg")
	var want []liveRuntimeRoundTripState
	for pass, prefs := range []runtimecfg.WeaponPreferences{{}, custom} {
		g := newGame(predictionTestMap(), Options{Headless: true, WeaponPreferences: prefs, BoomGameOptions: &demo.GameOptions{Seed: 1993, Insurance: true, NoMonsters: true}})
		g.inventory.Weapons[2001], g.inventory.Weapons[82], g.inventory.Weapons[2005] = true, true, true
		g.stats.Bullets, g.stats.Shells = 0, 2
		g.inventory.Strength = true
		for tic := 0; tic < 240; tic++ {
			buttons := byte(demoButtonAttack)
			if tic == 60 {
				buttons = demoButtonChange | byte(2<<3)
			}
			if tic == 120 {
				buttons = demoButtonChange | byte(7<<3)
			}
			g.stepGameplayFromDemoTic(DemoTic{Buttons: buttons})
			state := captureLiveRuntimeRoundTripState(g)
			if pass == 0 {
				want = append(want, state)
			} else if !reflect.DeepEqual(state, want[tic]) {
				t.Fatalf("local preferences changed recorded simulation at tic %d", tic)
			}
		}
	}
}

func TestBoomKeyframeKeepsReceivingPlayersPreferences(t *testing.T) {
	sourcePrefs, _ := runtimecfg.ParseWeaponPreferences("bfg")
	localPrefs, _ := runtimecfg.ParseWeaponPreferences("shotgun")
	base := predictionTestMap()
	sg := &sessionGame{current: base.Name, currentTemplate: cloneMapForRestart(base), opts: Options{Headless: true, WeaponPreferences: sourcePrefs, BoomGameOptions: &demo.GameOptions{Seed: 1993}}}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g
	sg.g.inventory.Weapons[2001], sg.g.inventory.Weapons[2006] = true, true
	sg.g.stats.Bullets, sg.g.stats.Shells, sg.g.stats.Cells = 0, 2, 40
	data, err := sg.marshalNetplayKeyframe()
	if err != nil {
		t.Fatal(err)
	}
	loaded := &sessionGame{opts: Options{Headless: true, WeaponPreferences: localPrefs, NewGameLoader: func(string) (*mapdata.Map, error) { return cloneMapForRestart(base), nil }}}
	if err := loaded.unmarshalNetplayKeyframe(data); err != nil {
		t.Fatal(err)
	}
	if loaded.opts.WeaponPreferences != localPrefs || loaded.g.opts.WeaponPreferences != localPrefs {
		t.Fatal("keyframe overwrote receiving player's preferences")
	}
	if got := loaded.g.fallbackWeapon(true); got != weaponShotgun {
		t.Fatalf("restored local fallback=%v", got)
	}
}
