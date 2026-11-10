package netgame

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestCompatibilityIncludesOrderedContentRulesAndSimulation(t *testing.T) {
	m := CompatibilityManifest{Simulation: SimulationVersion, WADHashes: []string{fmt.Sprintf("%x", sha256.Sum256([]byte("base"))), fmt.Sprintf("%x", sha256.Sum256([]byte("addon")))}, Map: "E1M1", Mode: "coop", Skill: 3, RespawnDelayTics: 35}
	key, err := m.Key()
	if err != nil {
		t.Fatal(err)
	}
	variants := []CompatibilityManifest{m, m, m, m, m}
	variants[0].WADHashes = []string{m.WADHashes[1], m.WADHashes[0]}
	variants[1].Simulation = "different"
	variants[2].FriendlyFire = true
	variants[3].Map = "E1M2"
	variants[4].Skill = 4
	for _, v := range variants {
		other, err := v.Key()
		if err != nil {
			t.Fatal(err)
		}
		if key == other {
			t.Fatalf("incompatible manifest matched: %+v", v)
		}
	}
}

func TestDefinitionIdentityBindsHandshakeAndMapIndependentContent(t *testing.T) {
	m := discoveryTestManifest()
	plain, _ := m.Key()
	plainContent, _ := m.ContentKey()
	m.DefinitionDigest = fmt.Sprintf("%x", sha256.Sum256([]byte("patched definitions")))
	patched, _ := m.Key()
	content, _ := m.ContentKey()
	if patched == plain || content == plainContent {
		t.Fatal("patch identity omitted")
	}
	m.Map = "MAP02"
	nextContent, _ := m.ContentKey()
	if nextContent != content {
		t.Fatal("map transition changed definition content identity")
	}
	for _, bad := range []string{"bad", string(make([]byte, 64)), "ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF"} {
		m.DefinitionDigest = bad
		if _, err := m.Key(); err == nil {
			t.Fatal("invalid definition digest accepted")
		}
	}
}
