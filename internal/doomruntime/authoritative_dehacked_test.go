package doomruntime

import (
	"context"
	"gddoom/internal/runtimecfg"
	"reflect"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestPatchedAuthorityReplicaAndCampaignPreserveDefinitions(t *testing.T) {
	defs := gameplayDefinitions(t, "Thing 2\nHit points = 123\nFrame 174\nDuration = -1\n")
	m := predictionTestMap()
	m.Things[0].X = -80
	m.Things = append(m.Things, mapdata.Thing{Type: 3004, X: 32, Flags: skillMask})
	opts := Options{Headless: true, GameMode: gameModeCoop, SkillLevel: 3, DehackedDefinitions: defs, WADHash: "same WAD content"}
	a, err := NewAuthority(m, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.AddPlayer(1); err != nil {
		t.Fatal(err)
	}
	if err := a.Step(map[byte]demo.Tic{1: {Forward: 5}}); err != nil {
		t.Fatal(err)
	}
	blob, err := a.Snapshot(1)
	if err != nil {
		t.Fatal(err)
	}
	client := newGame(cloneMapForRestart(m), opts)
	if err := client.applyAuthoritySnapshot(blob); err != nil {
		t.Fatal(err)
	}
	if client.thingHP[1] != 123 || !reflect.DeepEqual(client.thingDoomState, a.g.thingDoomState) || !reflect.DeepEqual(client.thingDefinitionFlags, a.g.thingDefinitionFlags) {
		t.Fatal("replication lost patched actor state")
	}
	wrong := opts
	wrong.DehackedDefinitions = gameplayDefinitions(t, "Thing 2\nHit points = 124\n")
	other := newGame(cloneMapForRestart(m), wrong)
	before := doomrand.Capture()
	if err := other.applyAuthoritySnapshot(blob); err == nil {
		t.Fatal("replica accepted mismatched definitions")
	}
	if !reflect.DeepEqual(before, doomrand.Capture()) {
		t.Fatal("rejected replica changed RNG")
	}
	a.g.authorityRules.Ended = true
	next := cloneMapForRestart(m)
	next.Name = "MAP02"
	if err := a.AdvanceMap(next); err != nil {
		t.Fatal(err)
	}
	if a.g.opts.DehackedDefinitions.Digest() != defs.Digest() || a.g.thingHP[1] != 123 {
		t.Fatal("campaign lost definitions")
	}
	blob, err = a.Snapshot(1)
	if err != nil {
		t.Fatal(err)
	}
	nextClient := newGame(cloneMapForRestart(next), opts)
	if err := nextClient.applyAuthoritySnapshot(blob); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(doomrand.Clear)
}

func TestPatchedAuthorityClientRequiresNegotiatedIdentity(t *testing.T) {
	defs := gameplayDefinitions(t, "Thing 2\nHit points = 123\n")
	opts := Options{DehackedDefinitions: defs, AuthorityClient: &fakeAuthorityClient{}}
	if err := ValidateDehackedOptions(opts); err == nil {
		t.Fatal("unnegotiated patched client accepted")
	}
	opts.AuthorityDefinitionDigest = defs.Digest()
	if err := ValidateDehackedOptions(opts); err != nil {
		t.Fatal(err)
	}
	opts.DehackedDefinitions = nil
	if err := ValidateDehackedOptions(opts); err == nil {
		t.Fatal("plain client accepted patched session")
	}
	opts.AuthorityClient = nil
	opts.DehackedDefinitions = defs
	opts.CoopPeers = &speedTestCoopPeers{} // legacy peer input cannot negotiate definitions
	if err := ValidateDehackedOptions(opts); err == nil {
		t.Fatal("patched peer play accepted")
	}
}

func TestPatchedMultiplayerMenuChecksIdentityBeforeInstalling(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	sg.opts.DehackedDefinitions = gameplayDefinitions(t, "Thing 2\nHit points = 123\n")
	result, _ := multiplayerMenuTestResult()
	sg.opts.AuthorityJoin = func(context.Context, runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
		return result, nil
	}
	if !sg.multiplayerMenuAvailable() {
		t.Fatal("patched authoritative menu unavailable")
	}
	before := sg.g
	if err := sg.installAuthorityJoin(result); err == nil || sg.g != before || sg.opts.AuthorityClient != nil {
		t.Fatal("mismatch mutated session")
	}
	result.Manifest.DefinitionDigest = sg.opts.DehackedDefinitions.Digest()
	if err := sg.installAuthorityJoin(result); err != nil {
		t.Fatal(err)
	}
	if sg.g.definitionErr != nil || sg.opts.AuthorityDefinitionDigest != result.Manifest.DefinitionDigest {
		t.Fatal("patched join not installed")
	}
	sg.leaveAuthorityMatch()
}
