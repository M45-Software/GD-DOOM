package doomruntime

type boomScrollVector struct{ x, y int64 }
type boomCarryThinker struct {
	order  int64
	sector int
	vector boomScrollVector
}

func (g *game) boomActorTouchesSector(x, y, radius int64, sec int) bool {
	if g.sectorAt(x, y) == sec {
		return true
	}
	box := [4]int64{y + radius, y - radius, x + radius, x - radius}
	if sec >= len(g.sectorLineAdj) {
		return false
	}
	for _, edge := range g.sectorLineAdj[sec] {
		if edge.line < 0 || edge.line >= len(g.physForLine) {
			continue
		}
		idx := g.physForLine[edge.line]
		if idx < 0 || idx >= len(g.lines) {
			continue
		}
		line := g.lines[idx]
		if box[2] <= line.bbox[3] || box[3] >= line.bbox[2] || box[0] <= line.bbox[1] || box[1] >= line.bbox[0] {
			continue
		}
		if g.boxOnLineSide(box, line) == -1 {
			return true
		}
	}
	return false
}

func (g *game) tickBoomCarry(c boomCarryThinker) {
	floor := g.sectorFloor[c.sector]
	water := int64(-1 << 31)
	if control := g.heightControl(c.sector); control >= 0 && g.sectorFloor[control] > floor {
		water = g.sectorFloor[control]
	}
	carryPlayer := func() {
		if !g.noClip && (g.p.z <= floor || g.p.z < water) && g.boomActorTouchesSector(g.p.x, g.p.y, playerRadius, c.sector) {
			g.p.momx = boomFixed(g.p.momx + c.vector.x)
			g.p.momy = boomFixed(g.p.momy + c.vector.y)
		}
	}
	if len(g.authorityPlayers) > 0 {
		for _, state := range g.authorityPlayers {
			g.withAuthoritativePlayer(state, carryPlayer)
		}
	} else {
		carryPlayer()
	}
	for i, th := range g.m.Things {
		if !thingTypeUsesBlockmap(th.Type) || g.thingCollected[i] {
			continue
		}
		x, y := g.thingPosFixed(i, th)
		z, _, _ := g.thingSupportState(i, th)
		if ((!monsterCanFloat(th.Type) && !thingSpawnsOnCeiling(th.Type) && z <= floor) || z < water) && g.boomActorTouchesSector(x, y, g.thingCurrentRadius(i, th), c.sector) {
			g.thingMomX[i] = boomFixed(g.thingMomX[i] + c.vector.x)
			g.thingMomY[i] = boomFixed(g.thingMomY[i] + c.vector.y)
		}
	}
	// Missiles have no gravity, but Boom carries them when submerged too.
	if water > int64(-1<<31) {
		for i := range g.projectiles {
			p := &g.projectiles[i]
			if p.z < water && g.boomActorTouchesSector(p.x, p.y, p.radius, c.sector) {
				p.vx = boomFixed(p.vx + c.vector.x)
				p.vy = boomFixed(p.vy + c.vector.y)
			}
		}
	}

	for i := range g.projectileImpacts {
		fx := &g.projectileImpacts[i]
		if fx.z < water && g.boomActorTouchesSector(fx.x, fx.y, demoTraceProjectileImpactRadius(fx.kind, fx.sourceType), c.sector) {
			fx.momx = boomFixed(fx.momx + c.vector.x)
			fx.momy = boomFixed(fx.momy + c.vector.y)
		}
	}
	for i := range g.hitscanPuffs {
		p := &g.hitscanPuffs[i]
		if ((p.kind == hitscanFxBlood && p.z <= floor) || p.z < water) && g.boomActorTouchesSector(p.x, p.y, 20*fracUnit, c.sector) {
			p.momx = boomFixed(p.momx + c.vector.x)
			p.momy = boomFixed(p.momy + c.vector.y)
		}
	}
}

// Earlier starts become physical actors; the last start retains the camera.
// Editor numbers stay unchanged in restartTemplate. Internal actor types are
// serialized with the ordinary thing arrays, including momentum and health.
const voodooThingType int16 = -32001

func voodooPlayerSlot(typ int16) int {
	slot := int(voodooThingType) - int(typ) + 1
	if slot < 1 || slot > 4 {
		return 0
	}
	return slot
}

func voodooTypeForSlot(slot int) int16 { return voodooThingType - int16(slot-1) }
func isVoodooDollType(typ int16) bool  { return voodooPlayerSlot(typ) != 0 }

// A doll shares its owner's inventory and player state, never the world
// thinker's current anchor. A disconnected owner's bodies cannot act.
func (g *game) withVoodooOwner(i int, action func()) {
	if i < 0 || i >= len(g.m.Things) || g.thingCollected[i] {
		return
	}
	slot := voodooPlayerSlot(g.m.Things[i].Type)
	if g.authorityRules != nil {
		if owner := g.authoritativePlayerForSlot(slot); owner != nil {
			g.withAuthoritativePlayer(owner, action)
		}
	} else if slot == g.localSlot {
		action()
	}
}

func (g *game) rebuildVoodooBodies() {
	g.boomVoodooBodies = nil
	for i, th := range g.m.Things {
		if isVoodooDollType(th.Type) {
			g.boomVoodooBodies = append(g.boomVoodooBodies, i)
		}
	}
}

func (g *game) initVoodooDolls() {
	if g.opts.GameMode != gameModeSingle {
		return
	}
	for i, th := range g.m.Things {
		if i != g.localPlayerThingIndex && playerSlotFromThingType(th.Type) == g.localSlot {
			g.m.Things[i].Type = voodooTypeForSlot(g.localSlot)
		}
	}
}

func (g *game) damageVoodooDoll(i, amount int, x, y, z int64, hasInflictor bool, sourceThing int, sourcePlayer bool) {
	source := g.authorityDamageOwner
	if sourceThing >= 0 {
		source = authorityPlayerIdentity{}
		if sourceThing < len(g.m.Things) {
			if owner := voodooPlayerSlot(g.m.Things[sourceThing].Type); owner != 0 {
				source = authorityPlayerIdentity{Slot: owner, Generation: g.authorityPlayerGeneration(owner)}
			}
		}
	} else if source.Slot == 0 && sourcePlayer {
		source = authorityPlayerIdentity{Slot: g.localSlot, Generation: g.authorityPlayerGeneration(g.localSlot)}
	}
	slot := voodooPlayerSlot(g.m.Things[i].Type)
	if !g.authorityPlayerDamageAllowed(source.Slot, slot) {
		return
	}
	g.withVoodooOwner(i, func() {
		if g.thingHP[i] <= 0 {
			return
		}
		main, health, state, tics := g.p, g.playerMobjHealth, g.playerMobjState, g.playerMobjTics
		previousSource := g.authorityDamageSource
		if g.authorityRules != nil {
			g.authorityDamageSource = 0
			if g.authoritativeSourceIdentityValid(source) {
				g.authorityDamageSource = source.Slot
			}
		}
		th := g.m.Things[i]
		px, py := g.thingPosFixed(i, th)
		pz, floor, ceil := g.thingSupportState(i, th)
		g.p = player{x: px, y: py, z: pz, floorz: floor, ceilz: ceil, momx: g.thingMomX[i], momy: g.thingMomY[i], momz: g.thingMomZ[i], subsector: -1, sector: -1, viewHeight: playerViewHeight}
		g.playerMobjHealth = g.thingHP[i]
		g.damagePlayerBody(amount, "Ouch!", x, y, hasInflictor, sourceThing, z, i)
		g.thingHP[i] = g.playerMobjHealth
		g.setThingMomentum(i, g.p.momx, g.p.momy, g.p.momz)
		g.thingDead[i] = g.thingHP[i] <= 0
		g.p, g.playerMobjHealth, g.playerMobjState, g.playerMobjTics = main, health, state, tics
		g.authorityDamageSource = previousSource
	})
}

// Player mobj states have no AI. Dolls only run pain/death animation after
// their physical thinker; the shared player's weapon/camera stays separate.
func (g *game) tickVoodooState(i int) {
	if g.thingGibbed[i] || g.thingStateTics[i] < 0 {
		return
	}
	if g.thingStateTics[i] > 0 {
		g.thingStateTics[i]--
	}
	if g.thingStateTics[i] != 0 {
		return
	}
	state := g.thingDoomState[i]
	switch {
	case state == 156:
		g.thingDoomState[i], g.thingStateTics[i] = 157, 4
		x, y := g.thingPosFixed(i, g.m.Things[i])
		g.emitSoundEventAt(soundEventPain, x, y)
	case state == 157:
		g.thingDoomState[i], g.thingStateTics[i] = 149, -1
	case state >= 158 && state < 164:
		g.thingDoomState[i], g.thingStateTics[i] = state+1, 10
		if state+1 == 164 {
			g.thingStateTics[i] = -1
		}
	case state >= 165 && state < 173:
		g.thingDoomState[i], g.thingStateTics[i] = state+1, 5
		if state+1 == 173 {
			g.thingStateTics[i] = -1
		}
	default:
		g.thingDoomState[i], g.thingStateTics[i] = 149, -1
	}
}

func (g *game) voodooFrameLetter(i int) byte {
	state := g.thingDoomState[i]
	switch {
	case state == 156 || state == 157:
		return 'G'
	case state >= 158 && state <= 164:
		return 'H' + byte(state-158)
	case state >= 165 && state <= 173:
		return 'O' + byte(state-165)
	default:
		return 'A'
	}
}

// Player state and the controlled mobj can disagree after a doll dies. The
// corpse flag belongs to the struck body; the camera body can remain solid.
func (g *game) playerBodyDead() bool {
	return g.isDead && g.playerMobjHealth <= 0
}

func (g *game) playerOwnsVoodooBody() bool {
	for _, i := range g.boomVoodooBodies {
		if voodooPlayerSlot(g.m.Things[i].Type) == g.localSlot && !g.thingCollected[i] {
			return true
		}
	}
	return false
}
