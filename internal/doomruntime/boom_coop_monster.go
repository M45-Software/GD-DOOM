package doomruntime

// A_Chase checks the other live players when its retained target is hidden
// and the retaliation threshold has expired. Acquiring even the same slot
// returns immediately, without a chase move or its RNG draws this frame.
func (g *game) retargetHiddenCoopPlayer(i int, typ int16, x, y int64) bool {
	return g.coopDemoNetgame() && i >= 0 && i < len(g.thingThreshold) && g.thingThreshold[i] == 0 && !g.monsterHasLOSTarget(i, typ, x, y) && g.monsterLookForPlayer(i, true, x, y)
}

// Classic co-op assigns non-player kills to a live retained enemy's owner,
// falling back to the first active slot. This includes infighting and crushers.
func (g *game) creditCoopUnownedKill(i int) {
	slot := 0
	if i >= 0 && i < len(g.boomLastEnemy) {
		ref := g.boomLastEnemy[i]
		if ref < 0 {
			if p := g.authoritativePlayerForSlot(-ref); p != nil {
				health := p.playerMobjHealth
				if p.localSlot == g.localSlot {
					health = g.playerMobjHealth
				}
				if health > 0 {
					slot = p.localSlot
				}
			}
		} else if ref > 0 && ref <= len(g.m.Things) && g.thingHP[ref-1] > 0 {
			slot = voodooPlayerSlot(g.m.Things[ref-1].Type)
		}
	}
	if slot == 0 && len(g.authorityPlayers) > 0 {
		slot = g.authorityPlayers[0].localSlot
	}
	if p := g.authoritativePlayerForSlot(slot); p != nil {
		g.withAuthoritativePlayer(p, func() { g.playerKillCount++ })
	}
}
