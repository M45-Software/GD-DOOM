//go:build integration

package doomruntime

import (
	"bufio"
	"fmt"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
	"os"
	"testing"
)

func TestBoomRebuiltBlockmapCapture(t *testing.T) {
	path := os.Getenv("GD_BLOCKMAP_GO_OUT")
	if path == "" {
		t.Skip("run native blockmap comparison")
	}
	wf, err := wad.Open(os.Getenv("GD_BLOCKMAP_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, mapdata.MapName(os.Getenv("GD_BLOCKMAP_MAP")))
	if err != nil {
		t.Fatal(err)
	}
	bm := buildBoomBlockmap(m)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	out := bufio.NewWriter(file)
	fmt.Fprintf(out, "origin,%d,%d,%d,%d\n", bm.OriginX, bm.OriginY, bm.Width, bm.Height)
	for i, cell := range bm.Cells {
		fmt.Fprintf(out, "%d,", i)
		for _, line := range cell {
			fmt.Fprintf(out, "%d;", uint16(line))
		}
		fmt.Fprintln(out)
	}
	if err := out.Flush(); err != nil {
		t.Fatal(err)
	}
}
