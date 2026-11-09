package launchcatalog

import (
	"encoding/binary"
	"gddoom/internal/wad"
	"os"
	"path/filepath"
	"testing"
)

func patchTestWAD(t *testing.T, patches ...string) *wad.File {
	t.Helper()
	payload := make([]byte, 12)
	directory := []byte{}
	for _, patch := range patches {
		entry := make([]byte, 16)
		binary.LittleEndian.PutUint32(entry, uint32(len(payload)))
		binary.LittleEndian.PutUint32(entry[4:], uint32(len(patch)))
		copy(entry[8:], "DEHACKED")
		directory = append(directory, entry...)
		payload = append(payload, []byte(patch)...)
	}
	copy(payload, "PWAD")
	binary.LittleEndian.PutUint32(payload[4:], uint32(len(patches)))
	binary.LittleEndian.PutUint32(payload[8:], uint32(len(payload)))
	file, err := wad.OpenData("patches.wad", append(payload, directory...))
	if err != nil {
		t.Fatal(err)
	}
	return file
}
func TestDehackedStackOrderAndRelativeIncludes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.deh")
	include := filepath.Join(dir, "nested.deh")
	if err := os.WriteFile(path, []byte("Thing 2\nHit points = 70\nINCLUDE nested.deh\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(include, []byte("Thing 2\nHit points = 80\n"), 0600); err != nil {
		t.Fatal(err)
	}
	stack := wad.Merge(patchTestWAD(t, "Thing 2\nHit points = 40\n"), patchTestWAD(t, "Thing 2\nHit points = 50\n", "Thing 2\nHit points = 60\n"))
	defs, _, err := LoadDehacked(stack, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	actor, _ := defs.ThingByNumber(2)
	if actor.SpawnHealth != 60 {
		t.Fatal("did not apply every embedded lump in directory/stack order")
	}
	defs, _, err = LoadDehacked(stack, []string{path}, false)
	if err != nil {
		t.Fatal(err)
	}
	actor, _ = defs.ThingByNumber(2)
	if actor.SpawnHealth != 80 {
		t.Fatal("external patch/include did not override embedded lumps")
	}
}
func TestDehackedEmbeddedSkipAndTransactionalErrors(t *testing.T) {
	invalid := patchTestWAD(t, "unsupported garbage\n")
	if defs, _, err := LoadDehacked(invalid, nil, false); err == nil || defs != nil {
		t.Fatal("invalid embedded patch accepted")
	}
	if defs, _, err := LoadDehacked(invalid, nil, true); err != nil || defs != nil {
		t.Fatal("nodeh did not skip embedded content")
	}
	embedded := patchTestWAD(t, "INCLUDE forbidden.deh\n")
	if defs, _, err := LoadDehacked(embedded, nil, false); err == nil || defs != nil {
		t.Fatal("embedded INCLUDE received filesystem access")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "valid.deh")
	if err := os.WriteFile(path, []byte("Thing 2\nHit points = 90\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if defs, _, err := LoadDehacked(nil, []string{path, filepath.Join(dir, "missing.deh")}, false); err == nil || defs != nil {
		t.Fatal("failed patch stack leaked partial definitions")
	}
}
func TestDehackedRepeatedAndCommaPaths(t *testing.T) {
	var paths DehackedPaths
	_ = paths.Set(" a.deh, b.bex ")
	_ = paths.Set("c.deh")
	if paths.String() != "a.deh,b.bex,c.deh" {
		t.Fatalf("paths=%v", paths)
	}
}
