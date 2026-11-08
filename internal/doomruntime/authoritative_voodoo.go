package doomruntime

// Join creates bodies only for an active co-op slot. A respawn keeps these
// bodies and their independent health. A later rejoin appends new identities:
// missiles and monster targets must never start referring to replacement dolls.
func (a *Authority) spawnVoodooDolls(slot int) {
	g := a.g
	if g.opts.GameMode != gameModeCoop {
		return
	}
	source := g.restartTemplate
	if source == nil {
		source = g.m
	}
	starts := collectPlayerStarts(source)
	last := -1
	for i, p := range starts {
		if p.slot == slot {
			last = i
		}
	}
	if last < 0 {
		return
	}
	for i, p := range starts {
		if p.slot != slot || i == last {
			continue
		}
		th := source.Things[p.index]
		th.Type = voodooTypeForSlot(slot)
		idx := g.appendRuntimeThing(th, false)
		g.ensureMonsterAIState()
		g.thingHP[idx] = 100
		g.thingDoomState[idx], g.thingStateTics[idx] = 149, -1
		if g.authorityPlayerGeneration(slot) == 1 {
			g.thingThinkerOrder[idx] = int64(p.index + 1)
			g.thingThinkerIndexDirty = true
		}
		// Friction arrays are optional until the first friction/pusher thinker.
		if len(g.thingFriction) > 0 {
			for len(g.thingFriction) < len(g.m.Things) {
				g.thingFriction = append(g.thingFriction, friction)
				g.thingMoveFactor = append(g.thingMoveFactor, boomMoveFactor)
			}
		}
		g.updateBoomThingTouch(idx)
	}
	g.rebuildVoodooBodies()
}

func (a *Authority) removeVoodooDolls(slot int) {
	g := a.g
	for i, th := range g.m.Things {
		if voodooPlayerSlot(th.Type) != slot || g.thingCollected[i] {
			continue
		}
		g.thingCollected[i] = true
		g.setThingMomentum(i, 0, 0, 0)
		g.updateThingBlockmapIndex(i)
		g.updateBoomThingTouch(i)
	}
}
