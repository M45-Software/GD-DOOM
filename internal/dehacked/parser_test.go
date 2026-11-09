package dehacked

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func mustPatch(t *testing.T, text string) *Patch {
	t.Helper()
	p, err := Parse("test.deh", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mustApply(t *testing.T, d *Definitions, patches ...*Patch) *Definitions {
	t.Helper()
	next, err := d.Apply(patches...)
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func TestVanillaCanonicalDefinitionsAndIndexDomains(t *testing.T) {
	d := Vanilla()
	s := d.Snapshot()
	for name, got := range map[string]int{"things": len(s.Things), "states": len(s.States), "sprites": len(s.Sprites), "sounds": len(s.Sounds), "weapons": len(s.Weapons), "ammo": len(s.Ammo)} {
		want := map[string]int{"things": 137, "states": 967, "sprites": 138, "sounds": 109, "weapons": 9, "ammo": 4}[name]
		if got != want {
			t.Fatalf("%s=%d want=%d", name, got, want)
		}
	}
	keen, ok := d.ThingByNumber(25)
	if !ok || keen.Name != "MT_KEEN" || keen.EditorNumber != 72 || keen.SpawnHealth != 100 || keen.SpawnState != 763 {
		t.Fatalf("Keen=%+v", keen)
	}
	index, fromEditor, ok := d.ThingForEditorNumber(72)
	if !ok || index != 24 || fromEditor != keen {
		t.Fatal("Thing index, patch number and editor number were conflated")
	}
	if _, ok := d.ThingByNumber(0); ok {
		t.Fatal("Thing 0 is not a valid patch number")
	}
	if _, _, ok := d.ThingForEditorNumber(-1); ok {
		t.Fatal("-1 is not a spawnable editor number")
	}
	state, _ := d.State(763)
	if state.Name != "S_KEENSTND" || state.Sprite != 52 || state.Tics != -1 || state.Action != NoAction {
		t.Fatalf("Keen state=%+v", state)
	}
	if s.Ammo[2] != (Ammo{300, 20}) || s.Weapons[6].Ammo != 2 || s.Weapons[6].Attack != 84 {
		t.Fatalf("weapon/ammo defaults=%+v/%+v", s.Weapons[6], s.Ammo)
	}
	if s.Sounds[86].Name != "chgun" || s.Sounds[86].Link != 1 || s.Sounds[86].Pitch != 150 {
		t.Fatalf("linked sound=%+v", s.Sounds[86])
	}
	if s.Pars["E1M1"] != 30 || s.Pars["MAP32"] != 30 || s.Misc.BFGCellsPerShot != 40 {
		t.Fatal("par/misc defaults differ from Doom")
	}
}

func TestCanonicalTableReferencesAndReadOnlyAccessors(t *testing.T) {
	d := Vanilla()
	s := d.Snapshot()
	for i, thing := range s.Things {
		for _, id := range []StateID{thing.SpawnState, thing.SeeState, thing.PainState, thing.MeleeState, thing.MissileState, thing.DeathState, thing.XDeathState, thing.RaiseState} {
			if _, ok := d.State(id); !ok {
				t.Fatalf("thing %d references invalid state %d", i, id)
			}
		}
		for _, id := range []SoundID{thing.SeeSound, thing.AttackSound, thing.PainSound, thing.DeathSound, thing.ActiveSound} {
			if _, ok := d.Sound(id); !ok {
				t.Fatalf("thing %d references invalid sound %d", i, id)
			}
		}
	}
	for i, state := range s.States {
		if _, ok := d.State(state.Next); !ok {
			t.Fatalf("state %d has invalid next state", i)
		}
		if _, ok := d.Sprite(state.Sprite); !ok {
			t.Fatalf("state %d has invalid sprite", i)
		}
		if _, ok := findAction(string(state.Action)); !ok {
			t.Fatalf("state %d has an unregistered action", i)
		}
	}
	for i, w := range s.Weapons {
		got, ok := d.Weapon(i)
		if !ok || got != w {
			t.Fatal("weapon getter lost canonical data")
		}
		if _, ok := d.Ammo(w.Ammo); !ok && w.Ammo != AmmoNone {
			t.Fatalf("weapon %d has invalid ammo", i)
		}
		for _, id := range []StateID{w.Up, w.Down, w.Ready, w.Attack, w.Flash} {
			if _, ok := d.State(id); !ok {
				t.Fatalf("weapon %d has invalid state", i)
			}
		}
	}
	if _, ok := d.Ammo(AmmoNone); ok {
		t.Fatal("no-ammo weapon sentinel is not an ammo slot")
	}
	if _, ok := d.State(-1); ok {
		t.Fatal("negative state accepted")
	}
	if _, ok := d.Sound(NumSounds); ok {
		t.Fatal("out-of-range sound accepted")
	}
	if _, ok := d.Weapon(NumWeapons); ok {
		t.Fatal("out-of-range weapon accepted")
	}
	if _, ok := d.Sprite(NumSprites); ok {
		t.Fatal("out-of-range sprite accepted")
	}
	if _, ok := d.Music(NumMusic); ok {
		t.Fatal("out-of-range music accepted")
	}
	if value, ok := d.String("presskey"); !ok || value != "press a key." {
		t.Fatal("canonical string lookup failed")
	}
	if value, ok := d.Cheat("God mode"); !ok || value != "iddqd" {
		t.Fatal("canonical cheat lookup failed")
	}
	if value, ok := d.Par("map01"); !ok || value != 30 {
		t.Fatal("canonical par lookup failed")
	}
	if d.Misc().InitialHealth != 100 {
		t.Fatal("misc getter lost canonical data")
	}
	before := d.Digest()
	sfx, _ := d.Sound(1)
	sfx.Priority = 1
	if d.Digest() != before {
		t.Fatal("a getter escaped mutable definition data")
	}
	hashes := VanillaSourceHashes()
	hashes["info.c"] = "mutated"
	if VanillaSourceHashes()["info.c"] == "mutated" {
		t.Fatal("provenance escaped mutable data")
	}
}

func TestSignedNumericWordEncodings(t *testing.T) {
	for text, want := range map[string]int32{"-1": -1, "0xffffffff": -1, "4294967295": -1, "0x80000000": -2147483648, "2147483648": -2147483648, "010": 8} {
		d := mustApply(t, Vanilla(), mustPatch(t, "Frame 174\nUnknown 1 = "+text+"\n"))
		state, _ := d.State(174)
		if state.Misc1 != want {
			t.Fatalf("%s resolved to %d, want %d", text, state.Misc1, want)
		}
	}
}

func TestSnapshotsAndAppliedPatchesCannotMutateSharedDefinitions(t *testing.T) {
	base := Vanilla()
	original := base.Digest()
	s := base.Snapshot()
	s.Things[24].SpawnHealth = 9
	s.States[763].Tics = 2
	s.Strings["HUSTR_1"] = "wrong"
	s.Pars["MAP01"] = 1
	if base.Digest() != original || Vanilla().Digest() != original {
		t.Fatal("snapshot escaped mutable defaults")
	}
	patch := mustPatch(t, "Thing 25\nHit points = 10000\n")
	next := mustApply(t, base, patch)
	keen, _ := next.ThingByNumber(25)
	if keen.SpawnHealth != 10000 || base.Digest() != original || Vanilla().Digest() != original {
		t.Fatal("patch application mutated its base")
	}
	if _, err := base.Apply(patch, nil); err == nil || base.Digest() != original {
		t.Fatal("failed application was not transactional")
	}
	if next.Digest() != mustApply(t, Vanilla(), patch).Digest() {
		t.Fatal("resolved digest is not deterministic")
	}
	if base.Digest() != mustApply(t, base, mustPatch(t, "# no changes\n")).Digest() {
		t.Fatal("comments changed resolved definitions")
	}
	if _, err := (*Definitions)(nil).Apply(patch); err == nil {
		t.Fatal("nil base accepted")
	}
}

func TestExplicitDefaultOverridesSurviveCompositionAndCopies(t *testing.T) {
	base := Vanilla()
	d := mustApply(t, base, mustPatch(t, "Thing 25\nBits = 0\nMisc 0\nMax Health = 100\n"))
	if !d.FieldWasSet("thing", 24, "Flags") || !d.FieldWasSet("misc", 0, "MaxHealth") || base.FieldWasSet("misc", 0, "MaxHealth") {
		t.Fatal("explicit override provenance was lost")
	}
	next := mustApply(t, d, mustPatch(t, "[CODEPTR]\nFRAME 174 = Look\n"))
	if !next.FieldWasSet("misc", 0, "MaxHealth") || !next.FieldWasSet("frame", 174, "Action") {
		t.Fatal("composition lost writes")
	}
	s := next.Snapshot()
	s.Defined["misc/0/MaxHealth"] = false
	if !next.FieldWasSet("misc", 0, "MaxHealth") {
		t.Fatal("snapshot escaped definition write tracking")
	}
	if mustApply(t, base, mustPatch(t, "Misc 0\nMax Health = 100\n")).Digest() == base.Digest() {
		t.Fatal("digest conflated an explicit compatibility override with no patch")
	}
}

func TestSunlustKeenPatchDefinitions(t *testing.T) {
	text := "Patch File for DeHackEd v3.0\nDoom version = 21\nPatch format = 6\n\nThing 25 (Commander Keen)\nBits = 0\nPain sound = 0\nHit points = 10000\nMass = 1000\nDeath sound = 0\nSpeed = 12\n"
	for frame := 763; frame <= 773; frame++ {
		text += fmt.Sprintf("\nFrame %d\nDuration = 2\n", frame)
	}
	text += "\nFrame 763\nNext frame = 764\nFrame 773\nNext frame = 764\n[CODEPTR]\n"
	for frame := 764; frame <= 773; frame++ {
		text += fmt.Sprintf("FRAME %d = VileChase\n", frame)
	}
	patch := mustPatch(t, strings.ReplaceAll(text, "\n", "\r\n"))
	d := mustApply(t, Vanilla(), patch)
	keen, _ := d.ThingByNumber(25)
	if keen.Flags != 0 || keen.SpawnHealth != 10000 || keen.Mass != 1000 || keen.Speed != 12 || keen.PainSound != 0 || keen.DeathSound != 0 {
		t.Fatalf("patched Keen=%+v", keen)
	}
	for frame := StateID(763); frame <= 773; frame++ {
		s, _ := d.State(frame)
		if s.Tics != 2 || frame > 763 && s.Action != "VileChase" {
			t.Fatalf("patched state %d=%+v", frame, s)
		}
	}
	end, _ := d.State(773)
	if end.Next != 764 {
		t.Fatal("patch loop was not preserved")
	}
	if len(patch.Diagnostics()) != 0 {
		t.Fatalf("unexpected diagnostics: %v", patch.Diagnostics())
	}
}

func TestPointerCopiesCompiledActionAcrossFramesAndPatchFiles(t *testing.T) {
	first := mustPatch(t, "[CODEPTR]\nFRAME 174 = VileChase\n")
	second := mustPatch(t, "Pointer 9999 (Frame 763)\nCodep Frame = 174\nPointer 0 (Frame 764)\nCodep Frame = 0\n")
	d := mustApply(t, Vanilla(), first, second)
	source, _ := d.State(174)
	copied, _ := d.State(763)
	null, _ := d.State(764)
	if source.Action != "VileChase" || copied.Action != "Look" || null.Action != NoAction {
		t.Fatalf("actions=%s,%s,%s", source.Action, copied.Action, null.Action)
	}
	separate := mustApply(t, mustApply(t, Vanilla(), first), second)
	if separate.Digest() != d.Digest() {
		t.Fatal("separate patch applications changed original-pointer lookup")
	}
}

func TestClassicFieldsAndBoomBits(t *testing.T) {
	d := mustApply(t, Vanilla(), mustPatch(t, `Thing 2
ID # = 12345
Initial frame = 0
First moving frame = 176
Alert sound = 1
Reaction time = -1
Attack sound = 2
Injury frame = 187
Pain chance = 17
Pain sound = 3
Close attack frame = 4
Far attack frame = 5
Death frame = 6
Exploding frame = 7
Death sound = 8
Speed = 655360
Width = 1310720
Height = 3670016
Missile damage = 9
Action sound = 10
Bits = SOLID + SHOOTABLE | TRANSLUCENT
Respawn frame = 11
Frame 174
Sprite number = 52
Sprite subnumber = 32770
Duration = 0
Next frame = 174
Unknown 1 = -1
Unknown 2 = 0x1234
Weapon 6
Ammo type = 5
Deselect frame = 1
Select frame = 2
Bobbing frame = 3
Shooting frame = 4
Firing frame = 5
Ammo 2
Max ammo = 999
Per ammo = 33
Misc 0
Initial Health = 123
BFG Cells/Shot = 20
Monsters Infight = 221
Sound 1
Value = 77
Zero 2 = 150
Zero 4 = 12
`))
	s := d.Snapshot()
	thing := s.Things[1]
	if thing.EditorNumber != 12345 || thing.SpawnState != 0 || thing.ReactionTime != -1 || thing.Flags != 0x80000006 || thing.RaiseState != 11 || thing.Speed != 655360 || thing.Damage != 9 {
		t.Fatalf("thing=%+v", thing)
	}
	if s.States[174].Sprite != 52 || s.States[174].Frame != 32770 || s.States[174].Misc1 != -1 || s.States[174].Misc2 != 0x1234 || s.States[174].Tics != 0 || s.States[174].Next != 174 {
		t.Fatalf("state=%+v", s.States[174])
	}
	if s.Weapons[6] != (Weapon{5, 1, 2, 3, 4, 5}) {
		t.Fatalf("weapon=%+v", s.Weapons[6])
	}
	if s.Ammo[2] != (Ammo{999, 33}) || s.Misc.InitialHealth != 123 || s.Misc.BFGCellsPerShot != 20 || s.Misc.MonstersInfight != 221 {
		t.Fatal("ammo/misc patch not applied")
	}
	if s.Sounds[1].Priority != 77 || s.Sounds[1].Pitch != 150 || s.Sounds[1].Volume != -1 {
		t.Fatal("Boom sound fields not applied")
	}
	for _, value := range []string{"0x80000006", "2147483654", "-2147483642"} {
		d := mustApply(t, Vanilla(), mustPatch(t, "Thing 2\nBits = "+value+"\n"))
		thing, _ := d.Thing(1)
		if thing.Flags != 0x80000006 {
			t.Fatalf("Bits %s became %x", value, thing.Flags)
		}
	}
}

func TestBexStringsParsAndLiteralByteCountedText(t *testing.T) {
	text := `[STRINGS]
HUSTR_1 = one \n\
    two\Nthree

[PARS]
par 1 23
par 2 3 77
`
	text += "Text 4 4\r\nTROOABCD\r\nText 6 5\npistollaser\nText 4 3\ne1m1abc\nText 4 4\nA\n#B\xff\n=C\n"
	d := mustApply(t, Vanilla(), mustPatch(t, text))
	s := d.Snapshot()
	if s.Strings["HUSTR_1"] != "one \ntwo\nthree" {
		t.Fatalf("continued string=%q", s.Strings["HUSTR_1"])
	}
	if s.Pars["MAP01"] != 23 || s.Pars["E2M3"] != 77 {
		t.Fatal("BEX par units or indexing wrong")
	}
	if s.Sprites[0] != "ABCD" || s.Sounds[1].Name != "laser" || s.Music[1] != "abc" {
		t.Fatalf("text resource renames=%q/%q/%q", s.Sprites[0], s.Sounds[1].Name, s.Music[1])
	}
	if s.TextReplacements["A\n#B"] != "\xff\n=C" {
		t.Fatalf("Text payload interpreted as lines or UTF-8: %q", s.TextReplacements)
	}
	if d.Digest() == mustApply(t, Vanilla(), mustPatch(t, "Text 4 4\nA\n#B\xfe\n=C")).Digest() {
		t.Fatal("digest lost legacy high bytes")
	}
}

func TestTextCRLFCountsAndOneShotResourceRenames(t *testing.T) {
	first := mustPatch(t, "Text 4 4\nTROOABCD\nText 6 6\npistollaser1\n")
	second := mustPatch(t, "Text 4 4\nTROOEFGH\nText 6 6\npistollaser2\nText 4 3\nA\r\nBCX\r\nY")
	s := mustApply(t, Vanilla(), first, second).Snapshot()
	if s.Sprites[0] != "ABCD" || s.Sounds[1].Name != "laser1" || s.TextReplacements["A\nBC"] != "X\nY" {
		t.Fatal("one-shot resource rename or CR removal incorrect")
	}
}

func TestResourceRenameCannotEscapeOneShotTrackingAsLiteralText(t *testing.T) {
	first := mustApply(t, Vanilla(), mustPatch(t, "Text 4 4\nTROOABCD\nText 6 6\npistollaser1\n"))
	before := first.Digest()
	next := mustApply(t, first, mustPatch(t, "Text 4 4\nTROOEFGH\nText 6 6\npistollaser2\n"))
	if next.Digest() != before {
		t.Fatal("ignored repeat resource rename changed resolved definitions")
	}
	longer := mustApply(t, Vanilla(), mustPatch(t, "Text 4 6\ne1m1newmus"))
	if name, _ := longer.Music(1); name != "newmus" {
		t.Fatalf("longer replacement was truncated: %q", name)
	}
	// The dummy music entry must not steal an unkeyed empty-string replacement.
	empty := mustApply(t, Vanilla(), mustPatch(t, "Text 0 1\nx"))
	if empty.Snapshot().TextReplacements[""] != "x" {
		t.Fatal("empty literal mistaken for a resource rename")
	}
}

func TestNotextLegacySkipStopsAfterSpaceLeadingLine(t *testing.T) {
	resolver := func(_, _ string) (Source, error) {
		return Source{"child", []byte("Text 4 4\nTROOABCD\n\nFrame 174\nDuration = 3\n\n # boundary\nThing 25\nHit points = 999\n")}, nil
	}
	p, err := ParseWithOptions("root", []byte("INCLUDE NOTEXT child\n\n[STRINGS]\nHUSTR_1 = Parent\n"), ParseOptions{ResolveInclude: resolver})
	if err != nil {
		t.Fatal(err)
	}
	d := mustApply(t, Vanilla(), p)
	state, _ := d.State(174)
	keen, _ := d.ThingByNumber(25)
	text, _ := d.String("HUSTR_1")
	if state.Tics != 10 || keen.SpawnHealth != 999 || text != "Parent" || len(p.Diagnostics()) != 1 {
		t.Fatal("NOTEXT skipping did not resume at the native boundary")
	}
}

func TestIncludesAreExplicitOrderedAndBounded(t *testing.T) {
	root := []byte("Thing 25\nHit points = 1\nINCLUDE NOTEXT \"child.deh\"\nThing 25\nMass = 500\n")
	resolver := func(parent, name string) (Source, error) {
		if parent != "root" || name != "child.deh" {
			t.Fatalf("resolver arguments=%q,%q", parent, name)
		}
		return Source{"child", []byte("Thing 25\nHit points = 2\nText 4 4\nTROOABCD\n[STRINGS]\nHUSTR_1 = Child\n")}, nil
	}
	p, err := ParseWithOptions("root", root, ParseOptions{ResolveInclude: resolver})
	if err != nil {
		t.Fatal(err)
	}
	s := mustApply(t, Vanilla(), p).Snapshot()
	if s.Things[24].SpawnHealth != 2 || s.Things[24].Mass != 500 || s.Sprites[0] != "TROO" || s.Strings["HUSTR_1"] != Vanilla().data.Strings["HUSTR_1"] {
		t.Fatal("include ordering or NOTEXT scope wrong")
	}
	if len(p.Sources()) != 2 {
		t.Fatal("included bytes missing from provenance")
	}
	identities := p.Sources()
	identities[0].Name = "mutated"
	if p.Sources()[0].Name != "root" {
		t.Fatal("source identities not copied")
	}
	if _, err := Parse("root", root); err == nil {
		t.Fatal("include accessed filesystem without a resolver")
	}
	cycle := func(_, name string) (Source, error) { return Source{"root", root}, nil }
	if _, err := ParseWithOptions("root", root, ParseOptions{ResolveInclude: cycle}); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle error=%v", err)
	}
	if _, err := ParseWithOptions("root", root, ParseOptions{ResolveInclude: resolver, MaxIncludeDepth: 1}); err == nil {
		t.Fatal("depth limit ignored")
	}
	if _, err := ParseWithOptions("root", root, ParseOptions{ResolveInclude: resolver, MaxBytes: len(root) + 1}); err == nil {
		t.Fatal("aggregate byte limit ignored")
	}
}

func TestRejectedPatchesHaveLocationsAndNoPartialDefinitions(t *testing.T) {
	for _, text := range []string{
		"Thing 0\n", "Thing 138\n", "Frame 967\n", "Thing 1\nInitial frame = -1\n",
		"Thing 25\nHit points = 3\nHit points = 4294967296\n", "Weapon 0\nAmmo type = 4\n",
		"Sound 109\n", "Ammo 4\n", "Frame 0\nSprite number = 138\n",
		"Thing 25\nBits = FRIEND\n", "Thing 25\nBits = 4294967296\n",
		"[CODEPTR]\nFRAME 763 = Spawn\n", "[CODEPTR]\nFRAME -1 = NULL\n",
		"[CODEPTR]\nFRAME 763 = A_VileChase\n",
		"Pointer 0 (Frame 967)\n", "Pointer 0 (Frame 0)\nCodep Frame = 967\n",
		"Frame 0\nArgs1 = 2\n", "[HELPER]\nType = 1\n", "Thing 25\nMBF21 Bits = 1\n",
		"Text 3 3\nshort", "Text -1 0\n", "Text 2147483647 2147483647\n",
		"[STRINGS]\nHUSTR_1 = unterminated\\", "Patch format = 5\n", "Doom version = 16\n",
		"[PARS]\npar 4 1 90\n", "[PARS]\npar 33 90\n", "bad header\n", "\x00binary",
	} {
		t.Run(fmt.Sprintf("%q", text), func(t *testing.T) {
			p, err := Parse("bad.deh", []byte(text))
			if err == nil || p != nil || !strings.Contains(err.Error(), "bad.deh:") {
				t.Fatalf("patch=%v err=%v", p, err)
			}
		})
	}
	if _, err := ParseWithOptions("small", []byte("Frame 0\nDuration = 1\nNext frame = 0\n"), ParseOptions{MaxChanges: 1}); err == nil {
		t.Fatal("change limit ignored")
	}
	if _, err := ParseWithOptions("small", []byte("Sprite 0\nOffset = 1\nOffset = 2\n"), ParseOptions{MaxChanges: 1}); err == nil {
		t.Fatal("diagnostic limit ignored")
	}
	resolve := func(_, _ string) (Source, error) { return Source{"child", nil}, nil }
	if _, err := ParseWithOptions("small", []byte("INCLUDE child"), ParseOptions{MaxChanges: 1, ResolveInclude: resolve}); err == nil {
		t.Fatal("source-count limit ignored")
	}
}

func TestDiagnosticsAndCheatBytesArePreserved(t *testing.T) {
	p := mustPatch(t, "Sound 1\nOffset = 1234\nSprite 0\nOffset = 5678\n[STRINGS]\nPORT_EXTENSION = kept\nCheat 0\nGod mode = newcode\xffpadding\n")
	if len(p.Diagnostics()) != 3 {
		t.Fatalf("diagnostics=%v", p.Diagnostics())
	}
	diags := p.Diagnostics()
	diags[0].Message = "mutated"
	if p.Diagnostics()[0].Message == "mutated" {
		t.Fatal("diagnostics escaped")
	}
	s := mustApply(t, Vanilla(), p).Snapshot()
	if s.Strings["PORT_EXTENSION"] != "kept" || s.Cheats["god mode"] != "newcode" {
		t.Fatal("unknown string or padded cheat lost")
	}
}

func TestPatchCanBeAppliedConcurrentlyWithoutSharedMutation(t *testing.T) {
	base := Vanilla()
	p := mustPatch(t, "Thing 25\nHit points = 77\n")
	expected := mustApply(t, base, p).Digest()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			d, err := base.Apply(p)
			if err != nil {
				t.Error(err)
				return
			}
			if d.Digest() != expected {
				t.Error("concurrent patch differs")
			}
		})
	}
	wg.Wait()
	keen, _ := base.ThingByNumber(25)
	if keen.SpawnHealth != 100 {
		t.Fatal("concurrent application mutated base")
	}
}

func FuzzParseApply(f *testing.F) {
	for _, text := range []string{"", "Thing 25\nHit points = 123\n", "Text 4 4\nTROOABCD", "[CODEPTR]\nFRAME 763 = VileChase\n", "[STRINGS]\nHUSTR_1 = value\n"} {
		f.Add([]byte(text))
	}
	base := Vanilla()
	original := base.Digest()
	f.Fuzz(func(t *testing.T, data []byte) {
		patch, err := ParseWithOptions("fuzz", data, ParseOptions{MaxBytes: 16384, MaxChanges: 100, MaxIncludeDepth: 4})
		if err != nil {
			return
		}
		next, err := base.Apply(patch)
		if err != nil {
			t.Fatal(err)
		}
		if base.Digest() != original {
			t.Fatal("base mutated")
		}
		if !reflect.DeepEqual(next.Snapshot(), mustApply(t, base, patch).Snapshot()) {
			t.Fatal("nondeterministic application")
		}
	})
}
