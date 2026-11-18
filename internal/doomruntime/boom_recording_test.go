package doomruntime

import (
	"path/filepath"
	"reflect"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestBoomRecordedEncounterReplaysEveryTic(t *testing.T) {
	t.Cleanup(doomrand.Clear)
	for _, explicit := range []bool{false, true} {
		name := "map-detected"
		if explicit {
			name = "explicit-options"
		}
		t.Run(name, func(t *testing.T) {
			m := predictionTestMap()
			m.Linedefs[0].Flags |= mlPassUse
			m.Things = append(m.Things, mapdata.Thing{X: 64, Type: 3001, Flags: 7, Angle: 180})
			path := filepath.Join(t.TempDir(), "encounter.lmp")
			opts := Options{Headless: true, SkillLevel: 3, RecordDemoPath: path}
			want := demo.GameOptions{MonstersRemember: true, AllowPushers: true, VariableFriction: true, PlayerBobbing: true, Seed: doomrand.DefaultBoomSeed}
			if explicit {
				want = demo.GameOptions{Insurance: true, WeaponRecoil: true, Seed: 0x80706050}
				want.Raw[47] = 0xa5
				opts.BoomGameOptions = &want
			}
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
			initialHP := live.Game.g.thingHP[1]
			var frames []frame
			for tic := 0; tic < 90; tic++ {
				if err := live.Tick(NativeMeshInput{Fire: true}, false); err != nil {
					t.Fatal(err)
				}
				frames = append(frames, snapshot(live.Game.g))
			}
			if initialHP <= 0 || live.Game.g.thingHP[1] >= initialHP {
				t.Fatal("recording never damaged the imp")
			}
			live.session.freezeDemoRecord()
			if _, err := live.FlushDemoRecording(); err != nil {
				t.Fatal(err)
			}
			script, err := demo.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if script.Header.Version != demo.VersionBoom202 || script.Header.Episode != 1 || script.Header.BoomOptions != want.Bytes() || len(script.Tics) != len(frames) {
				t.Fatalf("bad recording header or count: %+v, %d commands", script.Header, len(script.Tics))
			}
			playback := Options{Headless: true, DemoScript: script}
			replay := NewNativeCampaign(NewNativeMeshGame(cloneMapForRestart(m), playback), playback, nil)
			for i, want := range frames {
				if err := replay.Tick(NativeMeshInput{}, false); err != nil {
					t.Fatal(err)
				}
				got := snapshot(replay.Game.g)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("tic %d: checksum %d/%d view %d/%d actors equal=%t RNG equal=%t", i, got.checksum, want.checksum, got.view, want.view, reflect.DeepEqual(got.actors, want.actors), got.rng == want.rng)
				}
			}
		})
	}
}
