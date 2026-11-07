package doomruntime

import "slices"

// Boom retains sector membership nodes when an actor moves within a sector.
// Blocklinks, by contrast, move to the head after every successful XY move.
// A crusher must walk the former order to assign damage/RNG draws correctly.
// Only blockmapped bodies need these lists: P_CheckSector excludes NOBLOCKMAP
// effects and missiles. Conveyor effects are handled separately.
type boomTouchNode struct {
	sector, actor int // actor >= 0 is a thing; -1..-4 are player slots.
}

type boomSectorTouchState struct{ Sector, Actor int }

func (g *game) initBoomSectorTouches() {
	g.boomTouchSectors = nil
	g.boomTouchActors = nil
	if !g.boomMomentumDropoff || g.m == nil {
		return
	}
	g.boomTouchSectors = make([][]*boomTouchNode, len(g.m.Sectors))
	g.boomTouchActors = make([][]*boomTouchNode, len(g.m.Things)+4)
	// P_SpawnMapThing links the player at its place in the THINGS stream.
	for i := range g.m.Things {
		if i == g.localPlayerThingIndex {
			g.updateBoomPlayerTouch()
		} else {
			g.updateBoomThingTouch(i)
		}
	}
	if g.localPlayerThingIndex < 0 {
		g.updateBoomPlayerTouch()
	}
}

func (g *game) updateBoomThingTouch(i int) {
	if g.boomTouchSectors == nil || i < 0 || i >= len(g.m.Things) {
		return
	}
	th := g.m.Things[i]
	if !thingTypeUsesBlockmap(th.Type) || (i < len(g.thingCollected) && g.thingCollected[i]) || playerSlotFromThingType(th.Type) != 0 {
		g.updateBoomActorTouch(i, nil)
		return
	}
	x, y := g.thingPosFixed(i, th)
	g.updateBoomActorTouch(i, g.boomTouchedSectors(x, y, g.thingCurrentRadius(i, th)))
}

func (g *game) updateBoomPlayerTouch() {
	if g.boomTouchSectors != nil && !g.predictionMovement && g.localSlot >= 1 && g.localSlot <= 4 {
		g.updateBoomActorTouch(-g.localSlot, g.boomTouchedSectors(g.p.x, g.p.y, playerRadius))
	}
}

func (g *game) updateBoomActorTouch(actor int, sectors []int) {
	index := actor + 4
	if index < 0 {
		return
	}
	for len(g.boomTouchActors) <= index {
		g.boomTouchActors = append(g.boomTouchActors, nil)
	}
	old := g.boomTouchActors[index]
	keep := old[:0]
	for _, node := range old {
		if slices.Contains(sectors, node.sector) {
			keep = append(keep, node)
		} else {
			list := g.boomTouchSectors[node.sector]
			if at := slices.Index(list, node); at >= 0 {
				g.boomTouchSectors[node.sector] = slices.Delete(list, at, at+1)
			}
		}
	}
	for _, sector := range sectors {
		found := false
		for _, node := range keep {
			if node.sector == sector {
				found = true
				break
			}
		}
		if !found {
			node := &boomTouchNode{sector: sector, actor: actor}
			keep = append(keep, node)
			list := g.boomTouchSectors[sector]
			g.boomTouchSectors[sector] = append(list, nil)
			copy(g.boomTouchSectors[sector][1:], list)
			g.boomTouchSectors[sector][0] = node
		}
	}
	g.boomTouchActors[index] = keep
}

// P_CreateSecNodeList uses every intersected line's front and back sectors,
// even for one-sided walls, followed by the center point's subsector sector.
func (g *game) boomTouchedSectors(x, y, radius int64) []int {
	sectors := make([]int, 0, 4)
	add := func(sec int) {
		if sec >= 0 && sec < len(g.m.Sectors) && !slices.Contains(sectors, sec) {
			sectors = append(sectors, sec)
		}
	}
	box := [4]int64{y + radius, y - radius, x + radius, x - radius}
	visit := func(idx int) bool {
		if idx < 0 || idx >= len(g.physForLine) {
			return true
		}
		idx = g.physForLine[idx]
		if idx < 0 || idx >= len(g.lines) {
			return true
		}
		line := g.lines[idx]
		if box[2] <= line.bbox[3] || box[3] >= line.bbox[2] || box[0] <= line.bbox[1] || box[1] >= line.bbox[0] || g.boxOnLineSide(box, line) != -1 {
			return true
		}
		front, back := g.physLineSectors(line)
		add(front)
		add(back)
		return true
	}
	if g.m.BlockMap != nil && g.bmapWidth > 0 && g.bmapHeight > 0 {
		for bx := int((box[3] - g.bmapOriginX) >> (fracBits + 7)); bx <= int((box[2]-g.bmapOriginX)>>(fracBits+7)); bx++ {
			for by := int((box[1] - g.bmapOriginY) >> (fracBits + 7)); by <= int((box[0]-g.bmapOriginY)>>(fracBits+7)); by++ {
				g.blockLinesIterator(bx, by, visit)
			}
		}
	} else {
		for _, line := range g.lines {
			visit(line.idx)
		}
	}
	add(g.sectorAt(x, y))
	return sectors
}

func (g *game) walkBoomSectorTouches(sec int, thing func(int), player func()) {
	if g.boomTouchSectors == nil {
		g.initBoomSectorTouches()
	}
	// Like P_CheckSector, restart from the list head after each callback.
	// A skull can move/relink during height clipping; new nodes start unvisited.
	visited := make(map[*boomTouchNode]bool)
	for {
		var next *boomTouchNode
		for _, node := range g.boomTouchSectors[sec] {
			if !visited[node] {
				next = node
				break
			}
		}
		if next == nil {
			return
		}
		visited[next] = true
		if next.actor >= 0 {
			i := next.actor
			if i < len(g.m.Things) && !(i < len(g.thingCollected) && g.thingCollected[i]) {
				thing(i)
			}
		} else if g.authorityPlayers != nil {
			for _, state := range g.authorityPlayers {
				if state.localSlot == -next.actor {
					g.withAuthoritativePlayer(state, player)
					break
				}
			}
		} else if -next.actor == g.localSlot {
			player()
		}
	}
}

func (g *game) captureBoomSectorTouches() []boomSectorTouchState {
	if g.boomTouchSectors == nil {
		return nil
	}
	out := make([]boomSectorTouchState, 0)
	for sector, nodes := range g.boomTouchSectors {
		for _, node := range nodes {
			out = append(out, boomSectorTouchState{Sector: sector, Actor: node.actor})
		}
	}
	return out
}

func validBoomSectorTouches(s gameSaveState, sectors int) bool {
	if s.BoomSectorTouches == nil {
		return true
	} // Previous formats.
	seen := make(map[boomSectorTouchState]bool, len(s.BoomSectorTouches))
	actors := make(map[int]bool)
	for _, node := range s.BoomSectorTouches {
		if node.Sector < 0 || node.Sector >= sectors || node.Actor < -4 || node.Actor >= len(s.Things) || seen[node] {
			return false
		}
		if node.Actor >= 0 && (!thingTypeUsesBlockmap(s.Things[node.Actor].Type) || playerSlotFromThingType(s.Things[node.Actor].Type) != 0 || node.Actor < len(s.ThingCollected) && s.ThingCollected[node.Actor]) {
			return false
		}
		seen[node] = true
		actors[node.Actor] = true
	}
	for i, th := range s.Things {
		if thingTypeUsesBlockmap(th.Type) && !(i < len(s.ThingCollected) && s.ThingCollected[i]) && !actors[i] {
			return false
		}
	}
	return true
}

func (g *game) restoreBoomSectorTouches(state []boomSectorTouchState) {
	if state == nil {
		// Previous formats never stored sector-list order. Reconstruct an
		// initial order; exact historical crusher ordering cannot be recovered.
		g.initBoomSectorTouches()
		return
	}
	g.boomTouchSectors = make([][]*boomTouchNode, len(g.m.Sectors))
	g.boomTouchActors = make([][]*boomTouchNode, len(g.m.Things)+4)
	for _, saved := range state {
		node := &boomTouchNode{sector: saved.Sector, actor: saved.Actor}
		g.boomTouchSectors[node.sector] = append(g.boomTouchSectors[node.sector], node)
		g.boomTouchActors[node.actor+4] = append(g.boomTouchActors[node.actor+4], node)
	}
}
