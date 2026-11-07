package doomruntime

func (g *game) monsterTargetPlayerSlot(i int) int {
	if g == nil || i < 0 || i >= len(g.thingTargetPlayer) || !g.thingTargetPlayer[i] || i >= len(g.thingTargetPlayerSlot) {
		return 0
	}
	return g.thingTargetPlayerSlot[i]
}

// A slot can be reused after disconnect or respawn. Actor target pointers and
// retained noises refer to the departing body, never its replacement.
func (g *game) clearAuthoritativePlayerTargets(slot int) {
	if g == nil || slot < 1 || slot > 4 {
		return
	}
	for i, target := range g.thingTargetPlayerSlot {
		if target == slot {
			g.clearMonsterTargetState(i)
		}
	}
	for sector, target := range g.sectorSoundPlayerSlot {
		if target == slot {
			g.sectorSoundPlayerSlot[sector] = 0
			if sector < len(g.sectorSoundTarget) {
				g.sectorSoundTarget[sector] = false
			}
		}
	}
	for i := range g.projectileImpacts {
		if g.projectileImpacts[i].fireTargetPlayerSlot == slot {
			g.projectileImpacts[i].fireTargetPlayerSlot = 0
		}
	}
}

// authoritativeMonsterPlayer resolves the monster's retained player identity.
// A departed or unbound slot is not the current view player. Legacy worlds use
// the original single-player path and do not consult this identity.
func (g *game) authoritativeMonsterPlayer(i int) *authoritativePlayerState {
	if g == nil || len(g.authorityPlayers) == 0 || i < 0 ||
		i >= len(g.thingTargetPlayer) || !g.thingTargetPlayer[i] || i >= len(g.thingTargetPlayerSlot) {
		return nil
	}
	return g.authoritativePlayerForSlot(g.thingTargetPlayerSlot[i])
}

func (g *game) authoritativeMonsterPlayerPos(i int) (x, y, z, height, radius int64, ok bool) {
	state := g.authoritativeMonsterPlayer(i)
	if state == nil {
		return 0, 0, 0, 0, 0, false
	}
	body, dead, _ := g.authoritativePlayerBody(state)
	height = playerHeight
	if dead {
		height >>= 2
	}
	return body.x, body.y, body.z, height, playerRadius, true
}

// P_LookForPlayers retains both its stop slot and count of active slots,
// including dead ones. Skipping that stop changes a failed search into a
// direct reacquire, omitting the nested A_Look see-sound RNG draw.
func (g *game) authoritativeMonsterLookForPlayer(i int, allAround bool, tx, ty int64) bool {
	look := 0
	if i < len(g.thingLastLook) {
		look = g.thingLastLook[i] & 3
	}
	stop := (look - 1) & 3
	limit := 2
	if g.boomRememberEnabled() {
		limit = 4
	}
	tested := 0
	for {
		if state := g.authoritativePlayerForSlot(look + 1); state != nil {
			if tested == limit || look == stop {
				g.thingLastLook[i] = look
				return g.recallBoomEnemy(i)
			}
			tested++
			health := state.stats.Health
			if state.localSlot == g.localSlot {
				health = g.stats.Health
			}
			if health > 0 {
				visible := false
				g.withAuthoritativePlayer(state, func() {
					visible = g.monsterHasLOSPlayerAt(i, g.m.Things[i].Type, tx, ty)
					if visible && !allAround {
						angle := doomPointToAngle2(tx, ty, g.p.x, g.p.y) - g.thingWorldAngle(i, g.m.Things[i])
						visible = !(angle > doomAng90 && angle < doomAng270 && doomApproxDistance(g.p.x-tx, g.p.y-ty) > monsterMeleeRange)
					}
					if visible {
						g.setMonsterTargetPlayer(i)
					}
				})
				if visible {
					g.thingLastLook[i] = look
					return true
				}
			}
		}
		look = (look + 1) & 3
	}
}

func (g *game) authoritativeMonsterSoundTarget(i, sector int, tx, ty int64) (bool, bool) {
	if sector < 0 || sector >= len(g.sectorSoundPlayerSlot) {
		return false, false
	}
	if g.sectorSoundPlayerSlot[sector] == 0 && g.coopDemoActive() {
		if idx, ok := g.demoCoopSoundBodies[sector]; ok && g.thingHP[idx] > 0 && !g.thingCollected[idx] {
			g.setMonsterTargetThing(i, idx)
			wake := !g.thingAmbush[i] || g.monsterHasLOSTarget(i, g.m.Things[i].Type, tx, ty)
			return true, wake
		}
	}
	state := g.authoritativePlayerForSlot(g.sectorSoundPlayerSlot[sector])
	if state == nil {
		return false, false
	}
	if _, dead, _ := g.authoritativePlayerBody(state); dead {
		return false, false
	}
	wake := true
	g.withAuthoritativePlayer(state, func() {
		g.setMonsterTargetPlayer(i)
		if i < len(g.thingAmbush) && g.thingAmbush[i] {
			wake = g.monsterHasLOSPlayerAt(i, g.m.Things[i].Type, tx, ty)
		}
	})
	return true, wake
}
