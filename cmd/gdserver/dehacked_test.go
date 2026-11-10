package main

import (
	"context"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gddoom/internal/dehacked"
	"gddoom/internal/netgame"
)

func TestRunPatchedCoopLoadsAndNegotiatesDefinitions(t *testing.T) {
	for _, skip := range []bool{false, true} {
		name := "embedded-and-external"
		if skip {
			name = "skip-embedded"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			embedded := []byte("Thing 2\nSpeed = 3\n")
			data := make([]byte, 12+len(embedded)+16)
			copy(data, "PWAD")
			binary.LittleEndian.PutUint32(data[4:], 1)
			binary.LittleEndian.PutUint32(data[8:], uint32(12+len(embedded)))
			copy(data[12:], embedded)
			binary.LittleEndian.PutUint32(data[12+len(embedded):], 12)
			binary.LittleEndian.PutUint32(data[16+len(embedded):], uint32(len(embedded)))
			copy(data[20+len(embedded):], "DEHACKED")
			addon := filepath.Join(dir, "embedded.wad")
			first := filepath.Join(dir, "first.deh")
			second := filepath.Join(dir, "second.deh")
			for path, contents := range map[string][]byte{addon: data, first: []byte("Thing 2\nHit points = 122\n"), second: []byte("INCLUDE leaf.deh\n"), filepath.Join(dir, "leaf.deh"): []byte("Thing 2\nHit points = 123\n")} {
				if err := os.WriteFile(path, contents, 0600); err != nil {
					t.Fatal(err)
				}
			}
			text := "Thing 2\nHit points = 123\n"
			if !skip {
				text = string(embedded) + text
			}
			patch, err := dehacked.Parse("expected.deh", []byte(text))
			if err != nil {
				t.Fatal(err)
			}
			defs, err := dehacked.Vanilla().Apply(patch)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var output serverOutput
			done := make(chan error, 1)
			args := []string{"-listen", "127.0.0.1:0", "-web-listen", "127.0.0.1:0", "-wad", filepath.Join("..", "..", "DOOM1.WAD"), "-file", addon, "-deh", first, "-deh", second, "-mode", "coop", "-no-monsters"}
			if skip {
				args = append(args, "-nodeh")
			}
			go func() { done <- run(ctx, args, &output, io.Discard) }()
			t.Cleanup(func() {
				cancel()
				select {
				case err := <-done:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(3 * time.Second):
					t.Error("server did not stop")
				}
			})
			address, web := "", ""
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				for _, line := range strings.Split(output.value(), "\n") {
					if strings.HasPrefix(line, "gdserver: coop on ") {
						address = strings.Split(strings.TrimPrefix(line, "gdserver: coop on "), ",")[0]
					}
					if strings.HasPrefix(line, "websocket: ") {
						web = strings.TrimPrefix(line, "websocket: ")
					}
				}
				if address != "" && web != "" {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if address == "" || web == "" {
				t.Fatalf("server startup: %s", output.value())
			}
			transport, err := netgame.OpenTCP(ctx, address)
			if err != nil {
				t.Fatal(err)
			}
			info, err := netgame.QueryServer(ctx, transport)
			if err != nil {
				t.Fatal(err)
			}
			if info.Manifest.DefinitionDigest != defs.Digest() {
				t.Fatal("server did not resolve embedded/external/include patch order")
			}
			key, _ := info.Manifest.Key()
			wrong := info.Manifest
			wrong.DefinitionDigest = ""
			badKey, _ := wrong.Key()
			if client, err := netgame.DialTCP(ctx, address, netgame.Hello{Compatibility: badKey, Name: "wrong"}); err == nil {
				client.Close()
				t.Fatal("plain client admitted to patched game")
			}
			native, err := netgame.DialTCP(ctx, address, netgame.Hello{Compatibility: key, Name: "native"})
			if err != nil {
				t.Fatal(err)
			}
			defer native.Close()
			browser, err := netgame.DialWebSocket(ctx, web, netgame.Hello{Compatibility: key, Name: "browser"})
			if err != nil {
				t.Fatal(err)
			}
			defer browser.Close()
			if native.Welcome().PlayerID == browser.Welcome().PlayerID {
				t.Fatal("players share body")
			}
			for _, client := range []*netgame.Client{native, browser} {
				got := false
				deadline = time.Now().Add(2 * time.Second)
				for time.Now().Before(deadline) {
					s, ok, err := client.PollSnapshot()
					if err != nil {
						t.Fatal(err)
					}
					if ok && len(s.State) > 0 {
						got = true
						break
					}
					time.Sleep(time.Millisecond)
				}
				if !got {
					t.Fatal("patched co-op baseline missing")
				}
			}
		})
	}
}
