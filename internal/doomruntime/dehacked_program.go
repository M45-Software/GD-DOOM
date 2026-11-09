package doomruntime

import (
	"fmt"
	"gddoom/internal/dehacked"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

var definitionMonsterTypes = []int16{3004, 9, 64, 66, 67, 65, 3001, 3002, 58, 3005, 3003, 69, 3006, 7, 68, 16, 71, 84, 72, 2035}

// Canonical actor states include death/raise chains and the arch-vile's
// action-selected heal states. Effect/weapon thinkers retain their own runtime.
var definitionStateSet = func() map[int]bool {
	defs := dehacked.Vanilla()
	seen := map[int]bool{}
	var todo []int
	for _, typ := range definitionMonsterTypes {
		_, actor, _ := defs.ThingForEditorNumber(int32(typ))
		todo = append(todo, actorStateRoots(actor)...)
	}
	todo = append(todo, 266, 895)
	for len(todo) > 0 {
		state := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if state == 0 || seen[state] {
			continue
		}
		seen[state] = true
		def, _ := defs.State(dehacked.StateID(state))
		todo = append(todo, int(def.Next))
	}
	return seen
}()

func actorStateRoots(actor dehacked.Thing) []int {
	return []int{int(actor.SpawnState), int(actor.SeeState), int(actor.PainState), int(actor.MeleeState), int(actor.MissileState), int(actor.DeathState), int(actor.XDeathState), int(actor.RaiseState)}
}

func definitionActorTypes(defs *dehacked.Definitions) map[int16]bool {
	if defs == nil {
		return nil
	}
	active := map[int16]bool{}
	for _, typ := range definitionMonsterTypes {
		slot, actor, _ := defs.ThingForEditorNumber(int32(typ))
		for _, field := range []string{"SpawnState", "SeeState", "PainState", "MeleeState", "MissileState", "DeathState", "XDeathState", "RaiseState", "Flags", "SeeSound", "PainSound", "DeathSound", "ActiveSound", "AttackSound"} {
			if defs.FieldWasSet("thing", slot, field) {
				active[typ] = true
			}
		}
		seen := map[int]bool{}
		todo := actorStateRoots(actor)
		// VileChase can enter the original heal chain on any actor.
		for len(todo) > 0 {
			state := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			if state == 0 || seen[state] {
				continue
			}
			seen[state] = true
			for _, field := range []string{"Tics", "Next", "Action", "Sprite", "Frame"} {
				if defs.FieldWasSet("frame", state, field) {
					active[typ] = true
				}
			}
			def, ok := defs.State(dehacked.StateID(state))
			if !ok {
				continue
			}
			todo = append(todo, int(def.Next))
			if def.Action == "VileChase" {
				todo = append(todo, 266)
			}
		}
	}
	return active
}

func (g *game) definitionThing(typ int16) (dehacked.Thing, bool) {
	if g == nil || g.opts.DehackedDefinitions == nil {
		return dehacked.Thing{}, false
	}
	_, actor, ok := g.opts.DehackedDefinitions.ThingForEditorNumber(int32(typ))
	return actor, ok
}

// A separate flags word carries changes made by kill/fall/raise actions across
// checkpoints. Movement's transient flags retain the existing indexed fields.
func (g *game) ensureDefinitionFlags() {
	if g == nil || g.m == nil || len(g.definitionActors) == 0 {
		return
	}
	for len(g.thingDefinitionFlags) < len(g.m.Things) {
		i := len(g.thingDefinitionFlags)
		th := g.m.Things[i]
		flags := uint32(0)
		if actor, ok := g.definitionThing(th.Type); ok {
			flags = actor.Flags
		}
		if int(th.Flags)&thingFlagAmbush != 0 {
			flags |= 0x20
		}
		g.thingDefinitionFlags = append(g.thingDefinitionFlags, flags)
	}
}
func (g *game) definitionFlags(i int) (uint32, bool) {
	if g == nil || g.m == nil || i < 0 || i >= len(g.m.Things) || !g.patchedStateRuntime(g.m.Things[i].Type) {
		return 0, false
	}
	g.ensureDefinitionFlags()
	flags := g.thingDefinitionFlags[i] &^ uint32(0x40|0x80|0x200000|0x1000000)
	if i < len(g.thingJustHit) && g.thingJustHit[i] {
		flags |= 0x40
	}
	if i < len(g.thingJustAtk) && g.thingJustAtk[i] {
		flags |= 0x80
	}
	if i < len(g.thingInFloat) && g.thingInFloat[i] {
		flags |= 0x200000
	}
	if i < len(g.thingSkullFly) && g.thingSkullFly[i] {
		flags |= 0x1000000
	}
	return flags, true
}
func (g *game) definitionBaseFlags(typ int16) (uint32, bool) {
	if !g.patchedStateRuntime(typ) {
		return 0, false
	}
	actor, ok := g.definitionThing(typ)
	return actor.Flags, ok
}
func (g *game) thingTypeIsShootable(typ int16) bool {
	if f, ok := g.definitionBaseFlags(typ); ok {
		return f&4 != 0
	}
	return thingTypeIsShootable(typ)
}
func (g *game) thingTypeNoBlood(typ int16) bool {
	if f, ok := g.definitionBaseFlags(typ); ok {
		return f&0x80000 != 0
	}
	return thingTypeNoBlood(typ)
}
func (g *game) thingCountsKill(typ int16) bool {
	if f, ok := g.definitionBaseFlags(typ); ok {
		return f&0x400000 != 0
	}
	return thingCountsKill(typ)
}
func (g *game) thingCountsItem(typ int16) bool {
	if f, ok := g.definitionBaseFlags(typ); ok {
		return f&0x800000 != 0
	}
	return thingCountsItem(typ)
}
func (g *game) thingSpawnsOnCeiling(typ int16) bool {
	if f, ok := g.definitionBaseFlags(typ); ok {
		return f&0x100 != 0
	}
	return thingSpawnsOnCeiling(typ)
}
func (g *game) monsterCanFloat(typ int16) bool {
	if f, ok := g.definitionBaseFlags(typ); ok {
		return f&0x4000 != 0
	}
	return monsterCanFloat(typ)
}
func (g *game) monsterUsesShadow(typ int16) bool {
	if f, ok := g.definitionBaseFlags(typ); ok {
		return f&0x40000 != 0
	}
	return monsterUsesShadow(typ)
}
func (g *game) monsterUsesExactDoomStateMachine(typ int16) bool {
	return monsterUsesExactDoomStateMachine(typ) || g.patchedStateRuntime(typ)
}

func (g *game) tickDefinitionActor(i int) {
	th := g.m.Things[i]
	g.tickMonsterMomentum(i, th)
	if g.thingCollected[i] {
		return
	}
	if g.thingStateTics[i] != -1 {
		g.thingStateTics[i]--
		if g.thingStateTics[i] == 0 {
			def, ok := g.monsterDoomStateDef(g.thingDoomState[i])
			if !ok {
				g.definitionErr = fmt.Errorf("invalid actor state %d", g.thingDoomState[i])
				return
			}
			g.setExactDoomMonsterState(i, th.Type, def.next)
		}
	} else if g.thingCountsKill(th.Type) {
		g.tickNightmareRespawn(i, th)
	}
}

func (g *game) definitionDeath(i int) {
	typ := g.m.Things[i].Type
	actor, _ := g.definitionThing(typ)
	g.ensureDefinitionFlags()
	flags := g.thingDefinitionFlags[i]
	flags &^= 4 | 0x4000 | 0x1000000
	if typ != 3006 {
		flags &^= 0x200
	}
	flags |= 0x100000 | 0x400
	g.thingDefinitionFlags[i] = flags
	state := actor.DeathState
	if actor.XDeathState != 0 && g.thingHP[i] < -int(actor.SpawnHealth) {
		state = actor.XDeathState
		g.thingXDeath[i] = true
	}
	g.thingDead[i] = true
	g.thingSkullFly[i] = false
	g.thingState[i] = monsterStateDeath
	g.setExactDoomMonsterState(i, typ, int(state))
	// Doom shortens the state AFTER its action has run, including -1 states.
	g.thingStateTics[i] -= (doomrand.PRandom(doomrand.Killtics) & 3)
	if g.thingStateTics[i] < 1 {
		g.thingStateTics[i] = 1
	}
	g.spawnMonsterDrop(i, typ)
	g.bonusFlashTic = max(g.bonusFlashTic, 4)
}

func (g *game) thingTypeUsesBlockmap(typ int16) bool {
	if flags, ok := g.definitionBaseFlags(typ); ok {
		return flags&16 == 0
	}
	return thingTypeUsesBlockmap(typ)
}
func (g *game) thingSpawnsInSession(th mapdata.Thing, skill int, mode string, showNoSkillItems, showAllItems, noMonsters bool) bool {
	if flags, ok := g.definitionBaseFlags(th.Type); ok {
		if mode == gameModeDeathmatch && flags&0x2000000 != 0 {
			return false
		}
		noMonsters = noMonsters && (flags&0x400000 != 0 || th.Type == 3006)
	}
	return thingSpawnsInSession(th, skill, mode, showNoSkillItems, showAllItems, noMonsters)
}

func supportedDefinitionString(key string) bool {
	if len(key) == 6 && key[0] == 'C' && key[1] >= '1' && key[1] <= '6' && key[2:] == "TEXT" {
		return true
	}
	if len(key) == 6 && key[0] == 'E' && key[1] >= '1' && key[1] <= '4' && key[2:] == "TEXT" {
		return true
	}
	var number int
	if n, err := fmt.Sscanf(key, "HUSTR_%d", &number); err == nil && n == 1 && number >= 1 && number <= 32 && key == fmt.Sprintf("HUSTR_%d", number) {
		return true
	}
	return false
}

func (g *game) startDefinitionRaise(i int) {
	actor, _ := g.definitionThing(g.m.Things[i].Type)
	g.setExactDoomMonsterState(i, g.m.Things[i].Type, int(actor.RaiseState))
	g.thingDead[i], g.thingXDeath[i] = false, false
	if g.boomMomentumDropoff {
		g.thingGibbed[i] = false
	}
	g.thingDefinitionFlags[i] = actor.Flags
	g.thingHP[i] = int(actor.SpawnHealth)
	g.initDefinitionActorFlags(i)
	g.thingTargetPlayer[i], g.thingTargetIdx[i] = false, -1
	g.thingAggro[i] = false
	if i < len(g.thingTargetPlayerSlot) {
		g.thingTargetPlayerSlot[i] = 0
	}
}

func (g *game) initDefinitionActorFlags(i int) {
	if i < 0 || i >= len(g.thingDefinitionFlags) || !g.patchedStateRuntime(g.m.Things[i].Type) {
		return
	}
	for _, target := range []*[]bool{&g.thingSkullFly, &g.thingInFloat, &g.thingAmbush} {
		if len(*target) <= i {
			*target = append(*target, make([]bool, i+1-len(*target))...)
		}
	}
	flags := g.thingDefinitionFlags[i]
	g.thingJustHit[i], g.thingJustAtk[i] = flags&0x40 != 0, flags&0x80 != 0
	g.thingSkullFly[i], g.thingInFloat[i] = flags&0x1000000 != 0, flags&0x200000 != 0
	g.thingAmbush[i] = flags&0x20 != 0
}
