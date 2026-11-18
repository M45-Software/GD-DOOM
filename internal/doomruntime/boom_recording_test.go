package doomruntime

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func TestBoomRecordedEncounterReplaysEveryTic(t *testing.T) {
	m := predictionTestMap()
	m.Linedefs[0].Flags |= mlPassUse
	m.Things = append(m.Things,
		mapdata.Thing{X: 64, Type: 3001, Flags: 7, Angle: 180},
		mapdata.Thing{X: 12, Type: 2004, Flags: 7})
	inputs := make([]NativeMeshInput, 90)
	for i := range inputs {
		inputs[i].Fire = true
	}
	inputs[0].Forward = 1
	checkBoomEncounterRecordings(t, m, t.TempDir(), inputs, weaponPlasma, demo.GameOptions{Insurance: true, WeaponRecoil: true, Seed: 0x80706050})
	m.Linedefs[0].Flags &^= mlPassUse
	t.Run("vanilla", func(t *testing.T) {
		checkEncounterRecording(t, m, filepath.Join(t.TempDir(), "vanilla.lmp"), inputs, weaponPlasma, nil, demo.Version110)
	})
}

func TestSpeedOfDoomRecordedEncounter(t *testing.T) {
	path := os.Getenv("GD_BOOM_RECORD_SOD_WAD")
	if path == "" {
		t.Skip("set GD_BOOM_RECORD_SOD_WAD to the merged Doom II and Speed of Doom WAD")
	}
	wf, err := wad.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "MAP22")
	if err != nil {
		t.Fatal(err)
	}
	inputs := make([]NativeMeshInput, 180)
	for i := range inputs {
		inputs[i].Fire = true
		if i < 24 {
			inputs[i].Forward = 1
		}
		if i >= 75 && i < 78 {
			inputs[i].Turn = 1
		}
	}
	out := os.Getenv("GD_BOOM_RECORD_OUT")
	if out == "" {
		out = t.TempDir()
	}
	checkBoomEncounterRecordings(t, m, out, inputs, weaponChaingun, demo.GameOptions{
		Insurance: true, WeaponRecoil: true, VariableFriction: true, AllowPushers: true,
		PlayerBobbing: true, MonstersRemember: true, Seed: 0x80706050,
	})
}

func checkBoomEncounterRecordings(t *testing.T, m *mapdata.Map, out string, inputs []NativeMeshInput, wantWeapon weaponID, custom demo.GameOptions) {
	t.Helper()
	t.Run("automatic", func(t *testing.T) {
		checkEncounterRecording(t, m, filepath.Join(out, "automatic.lmp"), inputs, wantWeapon, nil, demo.VersionBoom202)
	})
	t.Run("custom", func(t *testing.T) {
		custom.Raw[47] = 0xa5
		checkEncounterRecording(t, m, filepath.Join(out, "custom.lmp"), inputs, wantWeapon, &custom, demo.VersionBoom202)
	})
}

func checkEncounterRecording(t *testing.T, m *mapdata.Map, path string, inputs []NativeMeshInput, wantWeapon weaponID, settings *demo.GameOptions, version byte) {
	t.Helper()
	t.Cleanup(doomrand.Clear)
	opts := Options{Headless: true, SkillLevel: 3, RecordDemoPath: path, BoomGameOptions: settings}
	live := NewNativeCampaign(NewNativeMeshGame(cloneMapForRestart(m), opts), opts, nil)
	type frame struct {
		checksum uint32
		view     int64
		actors   []demoTraceMobj
		rng      doomrand.Snapshot
	}
	snapshot := func(g *game) frame {
		return frame{g.SimChecksum(), g.playerViewZ, g.demoTraceMobjs(), doomrand.Capture()}
	}
	initialHP := append([]int(nil), live.Game.g.thingHP...)
	frames := make([]frame, 0, len(inputs))
	switched := false
	for _, input := range inputs {
		if err := live.Tick(input, false); err != nil {
			t.Fatal(err)
		}
		switched = switched || live.Game.g.inventory.ReadyWeapon == wantWeapon
		frames = append(frames, snapshot(live.Game.g))
	}
	if !switched {
		t.Fatalf("live recording never switched to collected weapon %d", wantWeapon)
	}
	damaged := 0
	for i, hp := range initialHP {
		if isMonster(m.Things[i].Type) && hp > 0 && live.Game.g.thingHP[i] < hp {
			damaged++
		}
	}
	if damaged == 0 {
		t.Fatal("live recording never damaged a monster")
	}
	live.session.freezeDemoRecord()
	if _, err := live.FlushDemoRecording(); err != nil {
		t.Fatal(err)
	}
	script, err := demo.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	episode, slot, _ := demo.MapSlot(m.Name)
	if episode == 0 && version == demo.VersionBoom202 {
		episode = 1
	}
	if script.Header.Version != version || int(script.Header.Episode) != episode || int(script.Header.Map) != slot || len(script.Tics) != len(frames) {
		t.Fatalf("unexpected recording header or command count: %+v, %d commands", script.Header, len(script.Tics))
	}
	if version == demo.VersionBoom202 {
		want := demo.GameOptions{MonstersRemember: true, AllowPushers: true, VariableFriction: true, PlayerBobbing: true, Seed: doomrand.DefaultBoomSeed}
		if settings != nil {
			want = *settings
		}
		if script.Header.BoomOptions != want.Bytes() {
			t.Fatalf("recorded game options %x, want %x", script.Header.BoomOptions, want.Bytes())
		}
	}
	playback := Options{Headless: true, DemoScript: script}
	replay := NewNativeCampaign(NewNativeMeshGame(cloneMapForRestart(m), playback), playback, nil)
	for i, want := range frames {
		if err := replay.Tick(NativeMeshInput{}, false); err != nil {
			t.Fatal(err)
		}
		if got := snapshot(replay.Game.g); !reflect.DeepEqual(got, want) {
			t.Fatalf("command %d: checksum %x/%x view %d/%d actors equal=%t RNG equal=%t", i+1, got.checksum, want.checksum, got.view, want.view, reflect.DeepEqual(got.actors, want.actors), got.rng == want.rng)
		}
	}
}

func TestUnrecordedWeaponPickupUsesPreference(t *testing.T) {
	t.Cleanup(doomrand.Clear)
	for _, enabled := range []bool{false, true} {
		g := newGame(predictionTestMap(), Options{Headless: true, AutoWeaponSwitch: enabled})
		g.applyPickup(2004, false)
		if got := g.inventory.PendingWeapon == weaponPlasma; got != enabled {
			t.Fatalf("auto switch %t: queued plasma %t", enabled, got)
		}
	}
}
