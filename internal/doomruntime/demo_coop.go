package doomruntime

import (
	"github.com/hajimehoshi/ebiten/v2"
)

func (g *game) coopDemoActive() bool {
	return g.opts.DemoScript != nil && g.opts.GameMode == gameModeCoop && len(g.opts.DemoScript.Frames) > 0
}

// Classic demo playback enables netgame rules from the second slot bit,
// even when a sparse recording contains other active players.
func (g *game) coopDemoNetgame() bool {
	return g.coopDemoActive() && g.opts.DemoScript.Header.PlayerInGame[1]
}

// The recorded roster is present during map spawning, unlike late network
// joins. Numbered starts and dolls retain their original thinker/block order.
func (g *game) initCoopDemoPlayers() {
	if !g.coopDemoActive() {
		return
	}
	selected := g.captureAuthoritativePlayer()
	g.authorityPlayers = make([]*authoritativePlayerState, 0, 4)
	g.remotePlayers = make(map[int]*remotePlayer)
	g.authorityRules = &authorityRulesState{Config: AuthorityRules{FriendlyFire: true}}
	for slot, on := range g.opts.DemoScript.Header.PlayerInGame {
		if !on {
			continue
		}
		g.applyAuthoritativePlayer(selected)
		g.localSlot = slot + 1
		g.p, _, _, g.localPlayerThingIndex = spawnPlayer(g.m, slot+1)
		g.playerBlockOrder = int64(g.localPlayerThingIndex + 1)
		g.refreshPlayerSubsectorCache(g.p.x, g.p.y)
		if sec := g.playerSector(); sec >= 0 {
			g.p.floorz, g.p.ceilz = g.sectorFloor[sec], g.sectorCeil[sec]
			g.p.z = g.p.floorz
		}
		g.initPlayerState()
		g.clearWeaponOverlay()
		g.bringUpWeapon()
		g.initStatusFaceState()
		g.playerMobjState, g.playerMobjTics = 149, -1
		g.playerViewZ = g.p.z + g.p.viewHeight
		g.prevPX, g.prevPY, g.prevAngle, g.prevPrevAngle = g.p.x, g.p.y, g.p.angle, g.p.angle
		state := g.captureAuthoritativePlayer()
		g.authorityPlayers = append(g.authorityPlayers, &state)
		if slot+1 != selected.localSlot {
			g.remotePlayers[slot+1] = &remotePlayer{slot: slot + 1, p: state.p}
		}
		g.authorityRules.Scores[slot+1].Generation = 1
	}
	g.applyAuthoritativePlayer(selected)
	for _, p := range g.authorityPlayers {
		if p.localSlot == selected.localSlot {
			g.applyAuthoritativePlayer(*p)
		}
	}
}

func (g *game) advanceCoopDemoTic() error {
	script := g.opts.DemoScript
	if g.demoIntermissionActive || g.demoFinaleActive {
		return g.updateCoopDemoIntermission()
	}
	if g.demoTick >= len(script.Frames) || g.opts.DemoStopAfterTics > 0 && g.demoTick >= g.opts.DemoStopAfterTics {
		return g.finishCoopDemo()
	}
	frame := script.Frames[g.demoTick]
	g.demoTick++
	commands := make(map[int]DemoTic, len(g.authorityPlayers))
	for _, p := range g.authorityPlayers {
		if p.playerReborn {
			g.withAuthoritativePlayer(p, g.respawnCoopDemoPlayer)
		}
		tc := frame[p.localSlot-1]
		commands[p.localSlot] = tc
		g.applyRecordedDemoPause(tc)
	}
	if !g.demoPaused {
		if err := g.runAuthoritativePlayersTic(g.authorityPlayers, commands); err != nil {
			return err
		}
	}
	g.tickDelayedSounds()
	g.flushSoundEvents()
	for _, p := range g.authorityPlayers {
		if p.localSlot == g.localSlot {
			if g.demoPaused {
				g.withAuthoritativePlayer(p, g.tickPausedDemoStatusWidgets)
			} else {
				g.withAuthoritativePlayer(p, g.tickStatusWidgets)
			}
		}
		if rp := g.remotePlayers[p.localSlot]; rp != nil {
			rp.p = p.p
		}
	}
	g.tickDelayedSwitchReverts()
	g.discoverLinesAroundPlayer()
	g.State.SetCamera(float64(g.p.x)/fracUnit, float64(g.p.y)/fracUnit)
	g.writeDemoTraceTic(g.demoTick - 1)
	g.demoTraceInitialWritten = true
	return nil
}

func (g *game) finishCoopDemo() error {
	if g.demoTrace != nil {
		g.demoTrace.Close()
		g.demoTrace = nil
	}
	g.reportDemoBench(g.opts.DemoScript)
	return ebiten.Termination
}
