package doomruntime

import (
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

// A former camera body loses its player pointer at G_DoReborn. It stays in
// the original thinker position and can never damage a replacement owner.
const detachedPlayerBodyType int16 = -32010

func isPlayerBodyThingType(typ int16) bool {
	return isVoodooDollType(typ) || typ == detachedPlayerBodyType
}

func (g *game) detachedPlayerBodySolid(i int) bool {
	if g.thingStatePhase[i] != 0 || g.thingGibbed[i] {
		return false
	}
	if !g.thingDead[i] {
		return true
	}
	state := g.thingDoomState[i]
	return state >= 158 && state < 160 || state >= 165 && state < 167
}

func (g *game) damageDetachedPlayerBody(i, damage int, sourcePlayer bool, sourceThing int, x, y int64, inflictor bool, z int64, hasZ bool) {
	if g.thingCollected[i] || g.thingHP[i] <= 0 {
		return
	}
	g.applyMonsterDamageThrust(i, damage, sourcePlayer, sourceThing, x, y, inflictor, z, hasZ, g.thingHP[i])
	g.thingHP[i] -= damage
	if g.thingHP[i] <= 0 {
		state, tics := 158, 10
		if g.thingHP[i] < -100 {
			state, tics = 165, 5
			g.thingXDeath[i] = true
		}
		g.thingDead[i] = true
		g.thingDoomState[i], g.thingStateTics[i] = state, max(1, tics-(doomrand.PRandom(doomrand.Killtics)&3))
	} else if doomrand.PRandom(doomrand.Painchance) < 255 {
		g.thingDoomState[i], g.thingStateTics[i] = 156, 4
	}
}

func nextPlayerBodyState(state int) (int, int) {
	switch {
	case state >= 150 && state <= 153:
		return 150 + (state-149)%4, 4
	case state == 154:
		return 149, -1
	case state == 155:
		return 154, 12
	case state == 156:
		return 157, 4
	case state == 157:
		return 149, -1
	case state >= 158 && state < 164:
		if state == 163 {
			return 164, -1
		}
		return state + 1, 10
	case state >= 165 && state < 173:
		if state == 172 {
			return 173, -1
		}
		return state + 1, 5
	default:
		return 149, -1
	}
}

func (g *game) tickCoopPlayerMobjState() {
	if g.playerMobjTics < 0 {
		return
	}
	if g.playerMobjTics > 0 {
		g.playerMobjTics--
	}
	if g.playerMobjTics != 0 {
		return
	}
	state, tics := nextPlayerBodyState(g.playerMobjState)
	g.setPlayerMobjState(state, tics)
	if state == 157 {
		g.emitSoundEvent(soundEventPain)
	}
}

func (g *game) detachCoopCameraBody() int {
	nextThinker, nextBlock := g.nextThinkerOrder, g.nextBlockmapOrder
	idx := g.appendRuntimeThingWithSpawnRNG(mapdata.Thing{Type: detachedPlayerBodyType}, false, false)
	g.nextThinkerOrder, g.nextBlockmapOrder = nextThinker, nextBlock
	g.ensureMonsterAIState()
	g.setThingPosFixed(idx, g.p.x, g.p.y)
	g.setThingSupportState(idx, g.p.z, g.p.floorz, g.p.ceilz)
	g.setThingMomentum(idx, g.p.momx, g.p.momy, g.p.momz)
	g.thingHP[idx], g.thingDead[idx] = g.playerMobjHealth, g.playerBodyDead()
	g.thingGibbed[idx] = g.p.corpseGibbed
	g.thingAngleState[idx] = g.p.angle
	g.thingDoomState[idx], g.thingStateTics[idx] = g.playerMobjState, g.playerMobjTics
	// G_CheckSpot sets MF_SOLID for probing, then clears it on the old body.
	g.thingStatePhase[idx] = 1
	order := g.authorityPlayerThinkerOrder
	if order == 0 {
		order = int64(g.localPlayerThingIndex + 1)
	}
	g.thingThinkerOrder[idx], g.thingBlockOrder[idx] = order, g.playerBlockOrder
	g.thingThinkerIndexDirty = true
	g.updateThingBlockmapIndex(idx)
	if g.boomTouchSectors != nil {
		for len(g.boomTouchActors) <= idx+4 {
			g.boomTouchActors = append(g.boomTouchActors, nil)
		}
		// Position setup linked the new thing once. Replace that link with the
		// original camera nodes so crushers visit the retained body exactly once.
		g.updateBoomActorTouch(idx, nil)
		nodes := g.boomTouchActors[4-g.localSlot]
		for _, node := range nodes {
			node.actor = idx
		}
		g.boomTouchActors[idx+4] = nodes
		g.boomTouchActors[4-g.localSlot] = nil
	}
	for victim, ref := range g.demoCoopAttackers {
		if ref == -g.localSlot {
			g.demoCoopAttackers[victim] = idx + 1
		}
	}
	for i, owner := range g.thingTargetPlayerSlot {
		if owner == g.localSlot && g.thingTargetPlayer[i] {
			g.setMonsterTargetThing(i, idx)
		}
	}
	for i, ref := range g.boomLastEnemy {
		if ref == -g.localSlot {
			g.boomLastEnemy[i] = idx + 1
		}
	}
	for i := range g.projectiles {
		p := &g.projectiles[i]
		if p.tracerPlayerSlot == g.localSlot {
			p.tracerPlayer = false
			p.tracerPlayerSlot = 0
			p.tracerThingTarget = idx + 1
		}
	}
	for i := range g.projectileImpacts {
		p := &g.projectileImpacts[i]
		if p.fireTargetPlayerSlot == g.localSlot {
			p.fireTargetPlayer = false
			p.fireTargetPlayerSlot = 0
			p.fireTargetThing = idx + 1
		}
	}
	for sec, owner := range g.sectorSoundPlayerSlot {
		if owner == g.localSlot {
			if g.demoCoopSoundBodies == nil {
				g.demoCoopSoundBodies = make(map[int]int)
			}
			g.demoCoopSoundBodies[sec] = idx
			g.sectorSoundPlayerSlot[sec] = 0
		}
	}

	return idx
}

func (g *game) respawnCoopDemoPlayer() {
	slot := g.localSlot
	a := Authority{g: g}
	starts := a.authoritativeSpawnStarts(byte(slot))
	if len(starts) == 0 {
		return
	}
	start := starts[0]
	fits := false
	for _, candidate := range starts {
		_, _, _, ok := g.checkPositionForActorWithPickupTouch(candidate.x, candidate.y, playerRadius, false, -1, false, false, false)
		if ok {
			start, fits = candidate, true
			break
		}
	}
	g.detachCoopCameraBody()
	if fits {
		angle := start.angle
		x := start.x + 20*doomFineCosine(angle)
		y := start.y + 20*doomFineSineAtAngle(angle)
		g.spawnTeleportFog(x, y, g.thingFloorZ(start.x, start.y))
		g.emitSoundEventAt(soundEventTeleport, x, y)
	}
	kills, items, secrets := g.playerKillCount, g.playerItemCount, g.secretsFound
	g.applyAuthoritativePlayer(authoritativePlayerState{localSlot: slot, localPlayerThingIndex: -1, autoWeaponSwitch: g.opts.AutoWeaponSwitch,
		playerBlockOrder: g.allocBlockmapOrder(), authorityPlayerThinkerOrder: g.allocThinkerOrder(),
		p: player{x: start.x, y: start.y, angle: start.angle, subsector: -1, sector: -1, viewHeight: playerViewHeight}})
	g.refreshPlayerSubsectorCache(start.x, start.y)
	if sec := g.playerSector(); sec >= 0 {
		g.p.floorz, g.p.ceilz = g.sectorFloor[sec], g.sectorCeil[sec]
		g.p.z = g.p.floorz
	}
	g.initPlayerState()
	g.clearWeaponOverlay()
	g.bringUpWeapon()
	g.initStatusFaceState()
	g.playerKillCount, g.playerItemCount, g.secretsFound = kills, items, secrets
	g.playerMobjState, g.playerMobjTics = 149, -1
	g.playerViewZ = g.p.z + g.p.viewHeight
	_ = doomrand.PRandom(doomrand.Lastlook) & 3
	g.updateBoomPlayerTouch()
	score := &g.authorityRules.Scores[slot]
	score.Generation++
	score.DeathRecorded = false
}

// A negative self reference survives a moving player's own splash damage.
const selfAttackerThing = -2

func (g *game) statusAttackerPosition() (int64, int64) {
	if g.statusAttackerThing == selfAttackerThing {
		return g.p.x, g.p.y
	}
	if g.coopDemoActive() {
		if ref := g.demoCoopAttackers[g.localSlot]; ref < 0 {
			if state := g.authoritativePlayerForSlot(-ref); state != nil {
				body, _, _ := g.authoritativePlayerBody(state)
				return body.x, body.y
			}
		} else if ref > 0 && ref <= len(g.m.Things) {
			return g.thingPosFixed(ref-1, g.m.Things[ref-1])
		}
	}
	if g.m != nil && g.statusAttackerThing >= 0 && g.statusAttackerThing < len(g.m.Things) {
		return g.thingPosFixed(g.statusAttackerThing, g.m.Things[g.statusAttackerThing])
	}
	return g.statusAttackerX, g.statusAttackerY
}
