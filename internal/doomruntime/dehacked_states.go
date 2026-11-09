package doomruntime

import (
	"fmt"
	"gddoom/internal/dehacked"
	"gddoom/internal/mapdata"
)

var patchedActions = map[dehacked.Action]doomMonsterAction{
	"NULL": doomMonsterActionNone, "Look": doomMonsterActionLook,
	"Chase": doomMonsterActionChase, "FaceTarget": doomMonsterActionFaceTarget,
	"PosAttack": doomMonsterActionPosAttack, "SPosAttack": doomMonsterActionSPosAttack,
	"CPosAttack": doomMonsterActionCPosAttack, "CPosRefire": doomMonsterActionCPosRefire,
	"Pain": doomMonsterActionPain, "SkullAttack": doomMonsterActionSkullAttack,
	"HeadAttack": doomMonsterActionHeadAttack, "BspiAttack": doomMonsterActionBspiAttack,
	"SpidRefire": doomMonsterActionBspiRefire,
}

func validatePatchedState(index int, member string, states []dehacked.State) error {
	if _, ok := monsterDoomStateDef(index); !ok {
		return fmt.Errorf("DeHackEd gameplay does not yet support Frame %d", index)
	}
	switch member {
	case "Tics", "Next", "Action":
	default:
		return fmt.Errorf("DeHackEd gameplay does not yet support frame/%d/%s", index, member)
	}
	state := states[index]
	if state.Tics < -1 {
		return fmt.Errorf("DeHackEd Frame %d duration below -1 is not supported", index)
	}
	if state.Next != 0 {
		if _, ok := monsterDoomStateDef(int(state.Next)); !ok {
			return fmt.Errorf("DeHackEd Frame %d points to unsupported Frame %d", index, state.Next)
		}
	}
	if _, ok := patchedActions[state.Action]; !ok {
		return fmt.Errorf("DeHackEd Frame %d action %s is not supported yet", index, state.Action)
	}
	return nil
}

func validateZeroStateChains(defs *dehacked.Definitions) error {
	for start := 0; start < dehacked.NumStates; start++ {
		if _, ok := monsterDoomStateDef(start); !ok {
			continue
		}
		seen := map[int]bool{}
		for state := start; state != 0; {
			def, _ := defs.State(dehacked.StateID(state))
			if def.Tics != 0 {
				break
			}
			if seen[state] {
				return fmt.Errorf("DeHackEd zero-duration state cycle at Frame %d", state)
			}
			seen[state] = true
			state = int(def.Next)
		}
	}
	return nil
}

// A_SkullAttack can use another actor's info.damage on impact. Until damage
// definitions and zero-damage hit semantics are integrated, reject such cross-
// type assignments, including indirect links and changed entry-state roots.
func validateReachableStateActions(defs *dehacked.Definitions) error {
	for _, typ := range []int16{3004, 9, 65, 68, 3005, 3006} {
		_, actor, _ := defs.ThingForEditorNumber(int32(typ))
		todo := []int{int(actor.SpawnState), int(actor.SeeState), int(actor.PainState), int(actor.MissileState)}
		seen := map[int]bool{}
		for len(todo) > 0 {
			state := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			if state == 0 || seen[state] {
				continue
			}
			seen[state] = true
			if _, ok := monsterDoomStateDef(state); !ok {
				return fmt.Errorf("unsupported reachable state %d for actor %d", state, typ)
			}
			def, _ := defs.State(dehacked.StateID(state))
			if def.Action == "SkullAttack" && typ != 3006 {
				return fmt.Errorf("cross-type SkullAttack requires actor damage integration (actor %d, Frame %d)", typ, state)
			}
			todo = append(todo, int(def.Next))
		}
	}
	return nil
}

func stateOverrides(defs *dehacked.Definitions) map[int]doomMonsterStateDef {
	if defs == nil {
		return nil
	}
	var out map[int]doomMonsterStateDef
	for index := 0; index < dehacked.NumStates; index++ {
		if !defs.FieldWasSet("frame", index, "Tics") && !defs.FieldWasSet("frame", index, "Next") && !defs.FieldWasSet("frame", index, "Action") {
			continue
		}
		s, _ := defs.State(dehacked.StateID(index))
		if out == nil {
			out = make(map[int]doomMonsterStateDef)
		}
		out[index] = doomMonsterStateDef{tics: int(s.Tics), next: int(s.Next), action: patchedActions[s.Action]}
	}
	return out
}

func (g *game) monsterDoomStateDef(state int) (doomMonsterStateDef, bool) {
	if def, ok := g.patchedStates[state]; ok {
		return def, true
	}
	return monsterDoomStateDef(state)
}
func (g *game) monsterInitialDoomState(typ int16) int {
	if value, ok := g.actorProperty(typ, "SpawnState"); ok {
		return int(value)
	}
	return monsterInitialDoomState(typ)
}
func (g *game) monsterDoomSeeState(typ int16) int {
	if value, ok := g.actorProperty(typ, "SeeState"); ok {
		return int(value)
	}
	return monsterDoomSeeState(typ)
}
func (g *game) monsterDoomPainState(typ int16) int {
	if value, ok := g.actorProperty(typ, "PainState"); ok {
		return int(value)
	}
	return monsterDoomPainState(typ)
}
func (g *game) monsterDoomMissileState(typ int16) int {
	if value, ok := g.actorProperty(typ, "MissileState"); ok {
		return int(value)
	}
	return monsterDoomMissileState(typ)
}
func (g *game) hasPatchedStateRoots(typ int16) bool {
	for _, field := range []string{"SpawnState", "SeeState", "PainState", "MissileState"} {
		if _, ok := g.actorProperty(typ, field); ok {
			return true
		}
	}
	return false
}
func (g *game) patchedStateRuntime(typ int16) bool {
	return g != nil && g.opts.DehackedDefinitions != nil && monsterUsesExactDoomStateMachine(typ) && (len(g.patchedStates) > 0 || g.hasPatchedStateRoots(typ))
}
func (g *game) dehackedSpawnInfo(typ int16, info demoTraceThingInfo) demoTraceThingInfo {
	if g.patchedStateRuntime(typ) {
		if state, ok := g.monsterDoomStateDef(g.monsterInitialDoomState(typ)); ok {
			info.spawnTics = state.tics
		}
	}
	return info
}

// P_SetMobjState executes actions on entry and follows zero-tic states in the
// same tic. Spawn initialization merely installs the initial state, without
// invoking its action. Nested action transitions share a bounded work budget.
func (g *game) setPatchedMonsterState(i int, typ int16, state, depth int) {
	if i < 0 || i >= len(g.thingDoomState) || i >= len(g.thingStateTics) {
		return
	}
	if depth == 0 {
		g.stateTransitionCount = 0
	}
	for {
		g.stateTransitionCount++
		if g.stateTransitionCount > dehacked.NumStates {
			g.definitionErr = fmt.Errorf("DeHackEd state transition budget exceeded for actor %d", i)
			return
		}
		if state == 0 {
			g.thingDoomState[i] = 0
			g.thingCollected[i] = true
			g.updateBoomThingTouch(i)
			g.updateThingBlockmapIndex(i)
			return
		}
		def, ok := g.monsterDoomStateDef(state)
		if !ok {
			g.definitionErr = fmt.Errorf("unsupported DeHackEd runtime state %d", state)
			return
		}
		g.thingDoomState[i], g.thingStateTics[i] = state, def.tics
		g.syncExactDoomMonsterCompatState(i, typ)
		g.runExactDoomMonsterAction(i, typ, state, def.action, depth)
		if g.definitionErr != nil || g.thingCollected[i] || g.thingStateTics[i] != 0 {
			return
		}
		current, ok := g.monsterDoomStateDef(g.thingDoomState[i])
		if !ok {
			return
		}
		state = current.next
	}
}

func (g *game) runPatchedAttackAction(i int, typ int16, action doomMonsterAction) bool {
	attackType := int16(0)
	switch action {
	case doomMonsterActionPosAttack:
		attackType = 3004
	case doomMonsterActionSPosAttack:
		attackType = 9
	case doomMonsterActionCPosAttack:
		attackType = 65
	case doomMonsterActionSkullAttack:
		attackType = 3006
	case doomMonsterActionBspiAttack:
		attackType = 68
	default:
		return false
	}
	x, y := g.thingPosFixed(i, g.m.Things[i])
	tx, ty, _, _, _, ok := g.monsterAttackTargetPos(i)
	if !ok {
		return true
	}
	g.faceMonsterToward(i, x, y, tx, ty)
	switch attackType {
	case 3004, 9, 65:
		pellets := 1
		sound := soundEventShootPistol
		if attackType == 9 {
			pellets = 3
		}
		if attackType != 3004 {
			sound = soundEventShootShotgun
		}
		g.emitSoundEventAt(sound, x, y)
		g.monsterHitscanAttack(i, attackType, x, y, pellets)
	case 3006:
		g.startLostSoulCharge(i)
	case 68:
		g.spawnMonsterProjectile(i, 68)
	}
	return true
}

func (g *game) patchedVisualState(i int, th mapdata.Thing) (dehacked.State, bool) {
	if !g.patchedStateRuntime(th.Type) || i < 0 || i >= len(g.thingDoomState) || g.thingDoomState[i] < 1 || g.thingDead[i] || g.thingState[i] == monsterStateRaise {
		return dehacked.State{}, false
	}
	return g.opts.DehackedDefinitions.State(dehacked.StateID(g.thingDoomState[i]))
}
func (g *game) monsterStateSpritePrefix(i int, th mapdata.Thing) (string, bool) {
	if state, ok := g.patchedVisualState(i, th); ok {
		return g.opts.DehackedDefinitions.Sprite(state.Sprite)
	}
	return monsterSpritePrefix(th.Type)
}
