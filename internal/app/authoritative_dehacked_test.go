package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"gddoom/internal/dehacked"
	"gddoom/internal/netgame"
	"gddoom/internal/runtimecfg"
)

func liveTestDefinitions(t *testing.T, text string) *dehacked.Definitions {
	t.Helper()
	patch, err := dehacked.Parse("local.deh", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	defs, err := dehacked.Vanilla().Apply(patch)
	if err != nil {
		t.Fatal(err)
	}
	return defs
}

func TestPatchedAuthorityDiscoveryJoinAndMapIdentity(t *testing.T) {
	defs := liveTestDefinitions(t, "Thing 2\nHit points = 123\n")
	same := liveTestDefinitions(t, "# another filename or comments have no effect\nThing 2\nHit points = 123\n")
	wrong := liveTestDefinitions(t, "Thing 2\nHit points = 124\n")
	f := newPatchedAuthorityJoinFixture(t, false, defs)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, local := range []*dehacked.Definitions{nil, wrong} {
		info, err := authorityDiscoverer(f.paths, local)(ctx, f.address)
		if err != nil || info.Compatible || !strings.Contains(info.CompatibilityError, "DeHackEd") {
			t.Fatalf("discovery mismatch: %+v %v", info, err)
		}
		if _, err := authorityJoiner(f.paths, f.file, local)(ctx, runtimecfg.AuthorityJoinRequest{Address: f.address}); err == nil || !strings.Contains(err.Error(), "DeHackEd") {
			t.Fatalf("joined mismatched definitions: %v", err)
		}
	}
	players, _, adds, _ := f.world.state()
	if len(players) != 0 || adds != 0 {
		t.Fatal("mismatch admitted a player")
	}
	info, err := authorityDiscoverer(f.paths, same)(ctx, f.address)
	if err != nil || !info.Compatible {
		t.Fatalf("equivalent definitions rejected: %v", err)
	}
	opts := runtimecfg.Options{DehackedDefinitions: same}
	client, m, err := connectAuthority(ctx, authorityLaunch{Address: f.address, Name: "Patched"}, f.paths, f.file, &opts)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Leave()
	if m.Name != "E1M2" || opts.AuthorityDefinitionDigest != same.Digest() {
		t.Fatal("negotiated identity not installed")
	}
	waitAuthorityJoinCondition(t, func() bool {
		snapshot, ok, err := client.PollSnapshot()
		if err != nil {
			t.Fatal(err)
		}
		return ok && len(snapshot.State) > 0
	})
	// Spectators negotiate the same definitions, and map loader checks retain them.
	result, err := authorityJoiner(f.paths, f.file, same)(ctx, runtimecfg.AuthorityJoinRequest{Address: f.address, Spectator: true})
	if err != nil {
		t.Fatal(err)
	}
	defer joinedClientLeave(t, result.Client)()
	next := f.manifest
	next.Map = "E1M3"
	key, _ := next.Key()
	if _, err := result.MapLoader(netgame.MapChange{Map: next.Map, Compatibility: key}); err != nil {
		t.Fatal(err)
	}
	next.DefinitionDigest = wrong.Digest()
	key, _ = next.Key()
	if _, err := result.MapLoader(netgame.MapChange{Map: next.Map, Compatibility: key}); err == nil {
		t.Fatal("map loader accepted changed definitions")
	}
}
