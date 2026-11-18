//go:build integration

package app

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"gddoom/internal/doomsession"
)

// One iteration replays the full active fight. Exclude map loading, rendering,
// traces, cold restores and intermission while retaining ordinary session updates.
func BenchmarkSpeedOfDoomMAP32(b *testing.B) {
	iwad, dir := os.Getenv("GD_BENCH_IWAD"), os.Getenv("GD_BENCH_DIR")
	if iwad == "" || dir == "" {
		b.Skip("set GD_BENCH_IWAD and GD_BENCH_DIR as in docs/boom-support.md")
	}
	pwad := filepath.Join(dir, "SODfinal.wad")
	recording := filepath.Join(dir, "sd32-917.lmp")
	for path, want := range map[string]string{
		iwad:      "10d67824b11025ddd9198e8cfc87ca335ee6e2d3e63af4180fa9b8a471893255",
		pwad:      "f985edf85ebe5ed3c8ce72efcdb2ad2e891f3c957ed5981cfd172d3937bf74cc",
		recording: "f4806dd992fc53ddb3baf11cb8a43d8b27979532dbea55764f6ee8f54868175d",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
			b.Fatalf("%s: got SHA-256 %s, want %s", path, got, want)
		}
	}
	cfg := renderBuildConfig{width: 320, height: 200, importTextures: true, startInMap: true, selectedMap: "MAP32", mapExplicit: true, pwadPaths: []string{pwad}, demoPath: recording, noEmbeddedDeh: true}
	const activeTics = 19520
	var total, slowestWindow, worstP99 time.Duration
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		bundle, err := buildRenderBundle(iwad, cfg, io.Discard)
		if err != nil {
			b.Fatal(err)
		}
		s := doomsession.New(bundle.m, bundle.opts, bundle.nextMap)
		samples := make([]time.Duration, activeTics)
		// The first update may perform boot work. Start at the first playable
		// tic and retain all subsequent simulation intervals.
		b.StartTimer()
		prev := time.Now()
		for i := 0; i < activeTics; {
			if err := s.Update(); err != nil {
				s.Close()
				b.Fatal(err)
			}
			now := time.Now()
			tic := s.CurrentWorldTic()
			if tic == i+1 {
				samples[i] = now.Sub(prev)
				i++
			} else if tic != 0 || i != 0 {
				s.Close()
				b.Fatalf("world tic %d after %d measured tics", tic, i)
			}
			prev = now
		}
		b.StopTimer()
		s.Close()
		var window time.Duration
		for i, d := range samples {
			total += d
			window += d
			if i >= 35 {
				window -= samples[i-35]
			}
			if i >= 34 {
				slowestWindow = max(slowestWindow, window)
			}
		}
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		worstP99 = max(worstP99, samples[(activeTics*99+99)/100-1])
	}
	b.ReportMetric(float64(activeTics*b.N)/total.Seconds(), "tics/s")
	b.ReportMetric(float64(slowestWindow)/1e6, "max-ms/35tics")
	b.ReportMetric(float64(worstP99)/1e6, "p99-ms/tic")
}
