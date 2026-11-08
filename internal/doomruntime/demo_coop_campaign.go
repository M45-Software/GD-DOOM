package doomruntime

// Carry only inventory and shared health across a map load. Physical bodies,
// pointers and per-map statistics belong to the old level, not the new roster.
type coopLevelCarryover struct {
	Slot   int
	Player playerLevelCarryover
	Reborn bool
}

func (sg *sessionGame) captureCoopCarryover() {
	sg.coopCarryover = nil
	if sg.g == nil || !sg.g.coopDemoActive() {
		return
	}
	for _, p := range sg.g.authorityPlayers {
		sg.g.withAuthoritativePlayer(p, func() {
			sg.coopCarryover = append(sg.coopCarryover, coopLevelCarryover{sg.g.localSlot, sg.g.captureLevelCarryover(), sg.g.isDead || sg.g.playerReborn || sg.g.stats.Health <= 0})
		})
	}
}

func (sg *sessionGame) applyCoopCarryover(next, old *game) bool {
	if next == nil || !next.coopDemoActive() || len(sg.coopCarryover) == 0 {
		return false
	}
	for _, carry := range sg.coopCarryover {
		p := next.authoritativePlayerForSlot(carry.Slot)
		if p == nil {
			continue
		}
		next.withAuthoritativePlayer(p, func() {
			if !carry.Reborn {
				next.applyLevelCarryover(carry.Player)
			}
			next.bringUpWeapon()
			if prior := old.authoritativePlayerForSlot(carry.Slot); prior != nil && !carry.Reborn {
				next.weaponAttackDown, next.useButtonDown = prior.weaponAttackDown, prior.useButtonDown
			}
			for i, th := range next.m.Things {
				if voodooPlayerSlot(th.Type) == carry.Slot {
					next.thingHP[i] = next.stats.Health
				}
			}
		})
	}
	sg.coopCarryover = nil
	sg.levelCarryover = nil
	return true
}

func (g *game) updateCoopDemoIntermission() error {
	// G_DoCompleted finishes every player on the first intermission command,
	// after the last live level frame has already been observed.
	if !g.coopTransitionPrepared {
		for _, p := range g.authorityPlayers {
			g.withAuthoritativePlayer(p, g.finishLevelForCarryover)
		}
		g.coopTransitionPrepared = true
	}
	script := g.opts.DemoScript
	if g.demoTick >= len(script.Frames) {
		return g.finishCoopDemo()
	}
	frame := script.Frames[g.demoTick]
	g.demoTick++
	g.demoIntermissionSkip = false
	for _, p := range g.authorityPlayers {
		g.withAuthoritativePlayer(p, func() {
			tc := frame[g.localSlot-1]
			g.applyRecordedDemoPause(tc)
			_, use, attack := demoTicCommand(tc)
			g.demoIntermissionSkip = g.demoIntermissionSkip || attack && !g.weaponAttackDown || use && !g.useButtonDown
			g.weaponAttackDown, g.useButtonDown = attack, use
		})
	}
	return nil
}

func initCoopIntermission(g *game, state *intermissionState) {
	if g == nil || !g.coopDemoNetgame() {
		return
	}
	state.Coop = true
	for _, p := range g.authorityPlayers {
		slot := p.localSlot - 1
		state.CoopSlots[slot] = true
		counts := [3]int{p.playerKillCount, p.playerItemCount, p.secretsFound}
		totals := [3]int{g.levelKillsTotal, g.levelItemsTotal, g.secretsTotal}
		for n, count := range counts {
			target, limit := 100, 0
			if totals[n] > 0 {
				target = count * 100 / totals[n]
				limit = target
			} else if n == 0 {
				target = max(1, count) * 100
				limit = count * 100
			}
			state.CoopTarget[slot][n], state.CoopLimit[slot][n] = target, limit
		}
		frags := 0
		for other, n := range g.demoCoopFrags[slot] {
			if other == slot {
				frags -= n
			} else {
				frags += n
			}
		}
		state.CoopTarget[slot][3], state.CoopLimit[slot][3] = frags, frags
	}
	sum := 0
	for _, target := range state.CoopTarget {
		sum += target[3]
	}
	state.CoopDoFrags = sum != 0
}

func updateCoopIntermissionStats(state intermissionState) (intermissionState, []soundEvent) {
	var sounds []soundEvent
	if state.Accelerate && state.SPState != 10 {
		state.Accelerate = false
		state.CoopShow = state.CoopTarget
		state.SPState = 10
		sounds = append(sounds, soundEventIntermissionDone)
	}
	switch state.SPState {
	case 2, 4, 6, 8:
		column := state.SPState/2 - 1
		increment := 2
		if column == 3 {
			increment = 1
		}
		if state.Bcnt&3 == 0 {
			sounds = append(sounds, soundEventShootPistol)
		}
		ticking := false
		for slot, on := range state.CoopSlots {
			if !on {
				continue
			}
			state.CoopShow[slot][column] += increment
			if state.CoopShow[slot][column] >= state.CoopLimit[slot][column] {
				state.CoopShow[slot][column] = state.CoopTarget[slot][column]
			} else {
				ticking = true
			}
		}
		if !ticking {
			sounds = append(sounds, soundEventIntermissionDone)
			state.SPState++
			if column == 2 && !state.CoopDoFrags {
				state.SPState += 2
			}
		}
	case 10:
		if state.Accelerate {
			state.Accelerate = false
			sounds = append(sounds, soundEventShotgunClose)
			if state.Commercial {
				state = initIntermissionNoState(state)
			} else {
				state = initIntermissionShowNextLoc(state)
			}
		}
	default:
		if state.SPState&1 != 0 {
			state.PauseTics--
			if state.PauseTics <= 0 {
				state.SPState++
				state.PauseTics = doomTicsPerSecond
			}
		}
	}
	// Existing renderers can still display the selected player's tally.
	for slot, on := range state.CoopSlots {
		if on {
			state.Show.KillsPct, state.Show.ItemsPct, state.Show.SecretsPct = state.CoopShow[slot][0], state.CoopShow[slot][1], state.CoopShow[slot][2]
			break
		}
	}
	return state, sounds
}
