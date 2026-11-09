package doomruntime

import (
	"fmt"
	"gddoom/internal/dehacked"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"strings"
)

func init() {
	for _, name := range []dehacked.Action{"BabyMetal", "BossDeath", "BruisAttack", "CyberAttack", "Explode", "Fall", "FatAttack1", "FatAttack2", "FatAttack3", "FatRaise", "Hoof", "KeenDie", "Metal", "PainAttack", "PainDie", "SargAttack", "Scream", "SkelFist", "SkelMissile", "SkelWhoosh", "TroopAttack", "VileAttack", "VileChase", "VileStart", "VileTarget", "XScream"} {
		patchedActions[name] = doomMonsterActionNone
	}
	// Unmodified exact thinkers share the canonical state lookup but do not
	// enter the additional-action dispatcher. Retain their chase action for
	// the footstep states (notably arachnotron RUN1/RUN7).
	for _, name := range []dehacked.Action{"BabyMetal", "Hoof", "Metal"} {
		patchedActions[name] = doomMonsterActionChase
	}
}

func definitionSoundEvent(defs *dehacked.Definitions, id dehacked.SoundID) soundEvent {
	if id == 0 {
		return -1
	}
	s, ok := defs.Sound(id)
	if !ok {
		return -1
	}
	for ev := soundEvent(0); ev < soundEventCount; ev++ {
		if name, ok := soundEventDSName(ev); ok && strings.EqualFold(name, "DS"+s.Name) {
			return ev
		}
	}
	return -1
}
func (g *game) emitDefinitionSound(i int, field string) {
	actor, ok := g.definitionThing(g.m.Things[i].Type)
	if !ok {
		return
	}
	id := actor.PainSound
	switch field {
	case "DeathSound":
		id = actor.DeathSound
	case "SeeSound":
		id = actor.SeeSound
	case "ActiveSound":
		id = actor.ActiveSound
	case "AttackSound":
		id = actor.AttackSound
	}
	if id == 0 {
		return
	}
	// Native sight/death sound families randomize even on a different actor.
	sound, _ := g.opts.DehackedDefinitions.Sound(id)
	group, size := "", 0
	switch strings.ToLower(sound.Name) {
	case "posit1", "posit2", "posit3":
		group, size = "posit", 3
	case "bgsit1", "bgsit2":
		group, size = "bgsit", 2
	case "podth1", "podth2", "podth3":
		group, size = "podth", 3
	case "bgdth1", "bgdth2":
		group, size = "bgdth", 2
	}
	ev := definitionSoundEvent(g.opts.DehackedDefinitions, id)
	if size > 0 && (field == "SeeSound" || field == "DeathSound") {
		class := doomrand.See
		if field == "DeathSound" {
			class = doomrand.Scream
		}
		name := fmt.Sprintf("DS%s%d", group, 1+doomrand.PRandom(class)%size)
		for candidate := soundEvent(0); candidate < soundEventCount; candidate++ {
			if ds, ok := soundEventDSName(candidate); ok && strings.EqualFold(ds, name) {
				ev = candidate
				break
			}
		}
	}
	if ev >= 0 {
		typ := g.m.Things[i].Type
		if (typ == 7 || typ == 16) && (field == "DeathSound" || field == "SeeSound") {
			g.emitSoundEvent(ev)
		} else {
			x, y := g.thingPosFixed(i, g.m.Things[i])
			g.emitSoundEventAt(ev, x, y)
		}
	}
}
func (g *game) definitionMeleeRange(i int) bool {
	tx, ty, _, _, radius, ok := g.monsterAttackTargetPos(i)
	if !ok {
		return false
	}
	x, y := g.thingPosFixed(i, g.m.Things[i])
	return doomApproxDistance(tx-x, ty-y) < monsterMeleeRange-20*fracUnit+radius && g.monsterHasLOSTarget(i, g.m.Things[i].Type, x, y)
}

func (g *game) runAdditionalDefinitionAction(i int, typ int16, state, depth int) bool {
	def, _ := g.opts.DehackedDefinitions.State(dehacked.StateID(state))
	x, y := g.thingPosFixed(i, g.m.Things[i])
	face := func() {
		tx, ty, _, _, _, ok := g.monsterAttackTargetPos(i)
		if ok {
			g.faceMonsterToward(i, x, y, tx, ty)
		}
	}
	chase := func() {
		tx, ty, _, _, _, _ := g.monsterTargetPos(i)
		g.runExactDoomMonsterChaseAction(i, typ, x, y, tx, ty, doomApproxDistance(tx-x, ty-y), depth)
	}
	switch def.Action {
	case "VileChase":
		if !g.archvileTryRaiseCorpse(i) {
			chase()
		}
	case "Hoof", "Metal", "BabyMetal":
		event := soundEventMetal
		if def.Action == "Hoof" {
			event = soundEventHoof
		}
		g.emitSoundEventAt(event, x, y)
		chase()
	case "Fall":
		g.thingDefinitionFlags[i] &^= 2
	case "Scream":
		g.emitDefinitionSound(i, "DeathSound")
	case "XScream":
		g.emitSoundEventAt(soundEventMonsterRaise, x, y) // sfx_slop
	case "KeenDie":
		g.thingDefinitionFlags[i] &^= 2
		for j, other := range g.m.Things {
			if j != i && other.Type == typ && !g.thingCollected[j] && g.thingHP[j] > 0 {
				return true
			}
		}
		g.activateTaggedDoor(666, mapdata.DoorOpen)
	case "BossDeath":
		g.handleBossDeath(i, typ)
	case "VileStart":
		g.emitSoundEventAt(soundEventMonsterAttackArchvile, x, y)
	case "VileTarget":
		face()
		g.spawnArchVileFire(i)
	case "VileAttack":
		face()
		g.archVileBlast(i, x, y)
	case "SkelWhoosh":
		face()
		g.emitSoundEventAt(soundEventSkelSwing, x, y)
	case "SkelFist":
		face()
		if g.definitionMeleeRange(i) {
			g.emitSoundEventAt(soundEventSkelPunch, x, y)
			g.damageMonsterTarget(i, monsterMeleeDamage(66), "Monster hit you", x, y)
		}
	case "SkelMissile":
		face()
		g.spawnMonsterProjectile(i, 66)
	case "CyberAttack":
		face()
		g.spawnMonsterProjectile(i, 16)
	case "TroopAttack", "BruisAttack", "SargAttack":
		face()
		attackType := int16(3001)
		if def.Action == "BruisAttack" {
			attackType = 3003
		}
		if def.Action == "SargAttack" {
			attackType = 3002
		}
		if g.definitionMeleeRange(i) {
			if attackType != 3002 {
				g.emitSoundEventAt(soundEventMonsterAttackClaw, x, y)
			}
			g.damageMonsterTarget(i, monsterMeleeDamage(attackType), "Monster hit you", x, y)
		} else if attackType != 3002 {
			g.spawnMonsterProjectile(i, attackType)
		}
	case "FatRaise":
		face()
		g.emitSoundEventAt(soundEventMonsterAttackMancubus, x, y)
	case "FatAttack1", "FatAttack2", "FatAttack3":
		old := g.thingAttackPhase[i]
		phase := 1
		if def.Action == "FatAttack2" {
			phase = 4
		}
		if def.Action == "FatAttack3" {
			phase = 7
		}
		g.thingAttackPhase[i] = phase
		g.monsterAttack(i, 67, 0)
		g.thingAttackPhase[i] = old
	case "PainAttack":
		face()
		g.spawnPainLostSoul(i, g.thingWorldAngle(i, g.m.Things[i]))
	case "PainDie":
		g.thingDefinitionFlags[i] &^= 2
		angle := g.thingWorldAngle(i, g.m.Things[i])
		for _, degrees := range []int{90, 180, 270} {
			g.spawnPainLostSoul(i, angle+degToAngle(int16(degrees)))
		}
	case "Explode":
		g.radiusAttackFromThing(i, 128)
	default:
		return false
	}
	return true
}
