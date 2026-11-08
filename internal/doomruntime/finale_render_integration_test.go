//go:build integration

package doomruntime

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gddoom/internal/render/doomtex"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/sessionflow"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestFinaleRenderCPUAndNative(t *testing.T) {
	out := os.Getenv("GD_FINALE_RENDER_OUT")
	if out == "" {
		t.Skip("set GD_FINALE_RENDER_OUT")
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		opts := Options{Headless: true, SpritePatchBank: map[string]WallTexture{}, IntermissionPatchBank: map[string]WallTexture{}, MessageFontBank: map[rune]WallTexture{}}
		for _, path := range []string{"/usr/share/games/doom/freedoom2.wad", "/usr/share/games/doom/freedoom1.wad"} {
			wf, err := wad.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			set, err := doomtex.LoadFromWAD(wf)
			if err != nil {
				t.Fatal(err)
			}
			for _, l := range wf.Lumps {
				sprite := false
				for _, prefix := range []string{"POSS", "SPOS", "CPOS", "TROO", "SARG", "SKUL", "HEAD", "BOS2", "BOSS", "BSPI", "PAIN", "SKEL", "FATT", "VILE", "SPID", "CYBR", "PLAY"} {
					if strings.HasPrefix(l.Name, prefix) && (len(l.Name) == 6 || len(l.Name) == 8) {
						sprite = true
					}
				}
				artwork := l.Name == "BOSSBACK" || l.Name == "PFUB1" || l.Name == "PFUB2" || l.Name == "CREDIT" || strings.HasPrefix(l.Name, "END") || strings.HasPrefix(l.Name, "STCFN")
				if !sprite && !artwork {
					continue
				}
				rgba, w, h, x, y, err := set.BuildPatchRGBA(l.Name, 0)
				if err != nil {
					continue
				}
				tex := WallTexture{RGBA: rgba, Width: w, Height: h, OffsetX: x, OffsetY: y}
				if sprite {
					opts.SpritePatchBank[l.Name] = tex
				}
				if artwork {
					opts.IntermissionPatchBank[l.Name] = tex
				}
				for ch := rune(33); ch <= 95; ch++ {
					if l.Name == fmt.Sprintf("STCFN%03d", ch) {
						opts.MessageFontBank[ch] = tex
					}
				}
			}
		}
		sg := &sessionGame{opts: opts, g: &game{opts: opts}}
		f, _ := sessionflow.StartCommercialFinale("MAP30", false)
		f.Boom = true
		f.Accelerated = true
		f, _ = sessionflow.TickFinale(f, true)
		scenes := map[string]sessionFinale{"cast-walk": f}
		for range 48 {
			f, _ = sessionflow.TickFinale(f, false)
		}
		scenes["cast-attack"] = f
		f.KillCast()
		for range 12 {
			f, _ = sessionflow.TickFinale(f, false)
		}
		scenes["cast-death"] = f
		for f.Cast.Actor != 16 {
			f.KillCast()
			old := f.Cast.Actor
			for f.Cast.Actor == old {
				f, _ = sessionflow.TickFinale(f, false)
			}
		}
		scenes["cast-hero"] = f
		for _, tic := range []int{0, 230, 550, 870, 1130, 1210} {
			f, _ := sessionflow.StartFinale("E3M8", false)
			f.Boom = true
			f.Stage = sessionflow.FinaleStagePicture
			f.Tic = tic
			scenes[fmt.Sprintf("bunny-%04d", tic)] = f
		}
		for name, f := range scenes {
			sg.finale = f
			want := ebiten.NewImage(320, 200)
			want.Fill(color.Black)
			sg.drawFinaleContents(want, 1, 0, 0)
			var patches []levelmesh.Patch
			sg.nativePatches = &patches
			sg.drawFinaleContents(nil, 1, 0, 0)
			sg.nativePatches = nil
			got := ebiten.NewImage(320, 200)
			got.Fill(color.Black)
			for _, p := range patches {
				img := ebiten.NewImage(p.Texture.Width, p.Texture.Height)
				img.WritePixels(p.Texture.RGBA)
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(p.W/float64(p.Texture.Width), p.H/float64(p.Texture.Height))
				op.GeoM.Translate(p.X, p.Y)
				got.DrawImage(img, op)
			}
			a, b := make([]byte, 320*200*4), make([]byte, 320*200*4)
			got.ReadPixels(a)
			want.ReadPixels(b)
			if !slices.Equal(a, b) {
				t.Fatalf("%s native/CPU pixels differ", name)
			}
			file, err := os.Create(filepath.Join(out, name+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(file, &image.RGBA{Pix: b, Stride: 320 * 4, Rect: image.Rect(0, 0, 320, 200)}); err != nil {
				t.Fatal(err)
			}
			file.Close()
			t.Logf("%s: %d patch commands, identical pixels", name, len(patches))
		}
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}
