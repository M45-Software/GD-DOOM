package doomruntime

// Boom's Check_Sides rejects a blocked PE-to-skull path before P_SpawnMobj
// can allocate a thinker or consume RNG. Match its inclusive bounding boxes
// and point-side test, including the original infinite-line approximation.
func (g *game) painSkullPathBlocked(sx, sy, x, y int64) bool {
	left, right := min64(sx, x), max64(sx, x)
	bottom, top := min64(sy, y), max64(sy, y)
	g.validCount++
	visit := func(line int) bool {
		if line < 0 || line >= len(g.physForLine) {
			return true
		}
		pi := g.physForLine[line]
		if pi < 0 || pi >= len(g.lines) || g.lineValid[pi] == g.validCount {
			return true
		}
		g.lineValid[pi] = g.validCount
		ld := g.lines[pi]
		if ld.flags&mlTwoSided != 0 && ld.flags&(mlBlocking|mlBlockMonsters) == 0 {
			return true
		}
		if left > ld.bbox[2] || right < ld.bbox[3] || top < ld.bbox[1] || bottom > ld.bbox[0] {
			return true
		}
		return g.pointOnLineSide(sx, sy, ld) == g.pointOnLineSide(x, y, ld)
	}
	if g.m.BlockMap != nil && g.bmapWidth > 0 && g.bmapHeight > 0 {
		xl, xh := int((left-g.bmapOriginX)>>(fracBits+7)), int((right-g.bmapOriginX)>>(fracBits+7))
		yl, yh := int((bottom-g.bmapOriginY)>>(fracBits+7)), int((top-g.bmapOriginY)>>(fracBits+7))
		for bx := xl; bx <= xh; bx++ {
			for by := yl; by <= yh; by++ {
				if !g.blockLinesIterator(bx, by, visit) {
					return true
				}
			}
		}
		return false
	}
	for line := range g.physForLine {
		if !visit(line) {
			return true
		}
	}
	return false
}
