package doomruntime

import "testing"

func TestBoomShortestTextureExcludesIndexZero(t *testing.T) {
	for _, placeholder := range []string{"-", "ZERO", ""} {
		g := boomFloorGame(0)
		g.opts.WallTextureZeroName = "ZERO"
		g.opts.WallTextureHeights = map[string]int{"-": 8, "ZERO": 8, "": 8, "TALL": 256}
		g.m.Sidedefs[0].Bottom, g.m.Sidedefs[1].Bottom = placeholder, "TALL"
		g.m.Sidedefs[0].Top, g.m.Sidedefs[1].Top = placeholder, "TALL"
		if got := g.boomShortestLowerTexture(1); got != 256*fracUnit {
			t.Fatalf("placeholder %q: lower texture height=%d", placeholder, got)
		}
		if got := g.boomShortestUpperTexture(1); got != 256*fracUnit {
			t.Fatalf("placeholder %q: upper texture height=%d", placeholder, got)
		}
		g.m.Sidedefs[1].Bottom, g.m.Sidedefs[1].Top = placeholder, placeholder
		if g.boomShortestLowerTexture(1) != 32000*fracUnit || g.boomShortestUpperTexture(1) != 32000*fracUnit {
			t.Fatalf("placeholder %q: missing textures changed the 32000-unit sentinel", placeholder)
		}
	}
}

func TestRaiseToTextureBoomTargetAndVanillaOverflow(t *testing.T) {
	for _, boom := range []bool{false, true} {
		g := boomFloorGame(0)
		g.boomMomentumDropoff = boom
		g.opts.WallTextureHeights = map[string]int{"-": 8, "TALL": 256}
		g.m.Sidedefs[0].Bottom, g.m.Sidedefs[1].Bottom = "-", "TALL"
		g.sectorFloor[1] = 31900 * fracUnit
		want := int64(31908 * fracUnit)
		if boom {
			want = 32000 * fracUnit
		}
		if got := g.raiseToTextureHeight(1); got != want {
			t.Fatalf("boom=%t target=%d want=%d", boom, got, want)
		}
		g.m.Sidedefs[0].Bottom, g.m.Sidedefs[1].Bottom = "MISSING", "MISSING"
		want = int64(int32(g.sectorFloor[1] + 2147483647))
		if boom {
			want = 32000 * fracUnit
		}
		if got := g.raiseToTextureHeight(1); got != want {
			t.Fatalf("boom=%t missing-texture target=%d want=%d", boom, got, want)
		}
	}
}
