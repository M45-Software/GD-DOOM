package roomhost

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/dehacked"
	"gddoom/internal/lobby"
)

func TestEmbeddedPatchIdentitySurvivesCatalogUploadAndRoomJSON(t *testing.T) {
	dir := t.TempDir()
	patchText := []byte("Thing 2\nHit points = 123\n")
	data := make([]byte, 12+len(patchText)+16)
	copy(data, "PWAD")
	binary.LittleEndian.PutUint32(data[4:], 1)
	binary.LittleEndian.PutUint32(data[8:], uint32(12+len(patchText)))
	copy(data[12:], patchText)
	binary.LittleEndian.PutUint32(data[12+len(patchText):], 12)
	binary.LittleEndian.PutUint32(data[16+len(patchText):], uint32(len(patchText)))
	copy(data[20+len(patchText):], "DEHACKED")
	addon := filepath.Join(dir, "patch.wad")
	if err := os.WriteFile(addon, data, 0600); err != nil {
		t.Fatal(err)
	}
	base, err := filepath.Abs(filepath.Join("..", "..", "DOOM1.WAD"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := filepath.Join(dir, "catalog.json")
	contents, _ := json.Marshal(Catalog{Packs: []CatalogPack{{ID: "patched", Name: "Patched", WADs: []string{base, addon}}}})
	if err := os.WriteFile(catalog, contents, 0600); err != nil {
		t.Fatal(err)
	}
	packs, err := LoadCatalog(catalog)
	if err != nil {
		t.Fatal(err)
	}
	uploaded, err := inspectUploadPaths("patched", "Patched", []string{base, addon})
	if err != nil {
		t.Fatal(err)
	}
	patch, err := dehacked.Parse("expected.deh", patchText)
	if err != nil {
		t.Fatal(err)
	}
	defs, err := dehacked.Vanilla().Apply(patch)
	if err != nil {
		t.Fatal(err)
	}
	for _, pack := range []lobby.Pack{packs[0].Pack, uploaded.Pack} {
		if pack.DefinitionDigest != defs.Digest() {
			t.Fatal("content inspection lost patch identity")
		}
		manifest, err := lobby.ValidateSettings(lobby.Settings{PackID: pack.ID, Map: pack.Maps[0], Mode: "coop", Skill: 3, PlayerLimit: 2}, pack)
		if err != nil {
			t.Fatal(err)
		}
		room := lobby.Room{Manifest: manifest}
		wire, err := json.Marshal(room)
		if err != nil {
			t.Fatal(err)
		}
		var restored lobby.Room
		if err := json.Unmarshal(wire, &restored); err != nil {
			t.Fatal(err)
		}
		before, _ := manifest.Key()
		after, _ := restored.Manifest.Key()
		if before != after || restored.Manifest.DefinitionDigest != defs.Digest() {
			t.Fatal("lobby JSON lost patched compatibility identity")
		}
	}
}
