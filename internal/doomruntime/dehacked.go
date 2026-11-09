package doomruntime

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"gddoom/internal/dehacked"
	"gddoom/internal/mapdata"
)

// actorDefinition is an owned, validated subset of an immutable patch set.
// Unwritten fields keep the existing engine behavior, including compatibility
// quirks which are not represented by the original C definition tables.
type actorDefinition map[string]int32

// newGameWithDefinitions validates before map initialization and RNG changes.
// Launchers install the same immutable set in Options so rebuilds and campaign
// transitions retain it. Network sessions require further compatibility work.
func newGameWithDefinitions(m *mapdata.Map, opts Options, defs *dehacked.Definitions) (*game, error) {
	actors, err := compileActorDefinitions(defs)
	if err != nil {
		return nil, err
	}
	opts.DehackedDefinitions = defs
	if err := validateDehackedNetworkOptions(opts); err != nil {
		return nil, err
	}
	return newGameWithActorDefinitions(m, opts, true, actors), nil
}

func compileActorDefinitions(defs *dehacked.Definitions) (map[int16]actorDefinition, error) {
	if defs == nil {
		return nil, nil
	}
	actual, vanilla := defs.Snapshot(), dehacked.Vanilla().Snapshot()
	if len(actual.Things) != len(vanilla.Things) {
		return nil, fmt.Errorf("DeHackEd gameplay requires vanilla-based definitions")
	}
	actors := make(map[int16]actorDefinition)
	keys := make([]string, 0, len(actual.Defined))
	for key := range actual.Defined {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		parts := strings.Split(key, "/")
		if len(parts) != 3 || (parts[0] != "thing" && parts[0] != "frame") {
			return nil, fmt.Errorf("DeHackEd gameplay does not yet support %s", key)
		}
		slot, err := strconv.Atoi(parts[1])
		if err != nil || slot < 0 || slot >= dehacked.NumStates {
			return nil, fmt.Errorf("invalid DeHackEd gameplay field %s", key)
		}
		if parts[0] == "frame" {
			if err := validatePatchedState(slot, parts[2], actual.States); err != nil {
				return nil, err
			}
			reflect.ValueOf(&actual.States[slot]).Elem().FieldByName(parts[2]).Set(reflect.ValueOf(vanilla.States[slot]).FieldByName(parts[2]))
			continue
		}
		if slot >= len(vanilla.Things) {
			return nil, fmt.Errorf("invalid Thing slot %d", slot)
		}

		typ := int16(vanilla.Things[slot].EditorNumber)
		if !isMonster(typ) && !isBarrelThingType(typ) {
			return nil, fmt.Errorf("DeHackEd gameplay does not yet support Thing %d", slot+1)
		}
		member := parts[2]
		if isBarrelThingType(typ) && (member == "PainChance" || member == "Speed") {
			return nil, fmt.Errorf("DeHackEd gameplay does not yet support %s for barrels", member)
		}
		switch member {
		case "SpawnHealth", "ReactionTime", "PainChance", "Speed", "Radius", "Height", "Mass":
		case "SpawnState", "SeeState", "PainState", "MissileState":
			if !monsterUsesExactDoomStateMachine(typ) {
				return nil, fmt.Errorf("DeHackEd state roots are not supported for Thing %d yet", slot+1)
			}
		default:
			return nil, fmt.Errorf("DeHackEd gameplay does not yet support %s", key)
		}
		value := int32(reflect.ValueOf(actual.Things[slot]).FieldByName(member).Int())
		valid := value >= 0
		switch member {
		case "SpawnHealth", "Mass":
			valid = value > 0
		case "PainChance":
			valid = value >= 0 && value <= 256
		case "SpawnState", "SeeState", "PainState", "MissileState":
			state := int(value)
			_, valid = monsterDoomStateDef(state)
			if value == 0 && member != "SpawnState" {
				valid = true
			}
		case "Speed":
			valid = value >= 0 && value <= 32767
		}
		if !valid {
			return nil, fmt.Errorf("DeHackEd gameplay: %s value %d is outside the currently supported range", key, value)
		}
		if actors[typ] == nil {
			actors[typ] = make(actorDefinition)
		}
		actors[typ][member] = value
		reflect.ValueOf(&actual.Things[slot]).Elem().FieldByName(member).Set(reflect.ValueOf(vanilla.Things[slot]).FieldByName(member))
	}
	// Also check the resolved data, rather than trusting only explicit-write keys.
	actual.Defined, vanilla.Defined = nil, nil
	if !reflect.DeepEqual(actual, vanilla) {
		return nil, fmt.Errorf("DeHackEd gameplay definitions contain unsupported changes")
	}
	if err := validateReachableStateActions(defs); err != nil {
		return nil, err
	}
	if err := validateZeroStateChains(defs); err != nil {
		return nil, err
	}
	return actors, nil
}

func (g *game) actorProperty(typ int16, field string) (int32, bool) {
	if g == nil {
		return 0, false
	}
	value, ok := g.actorDefinitions[typ][field]
	return value, ok
}

func (g *game) monsterSpawnHealth(typ int16) int {
	if n, ok := g.actorProperty(typ, "SpawnHealth"); ok {
		return int(n)
	}
	return monsterSpawnHealth(typ)
}
func (g *game) shootableThingSpawnHealth(typ int16) int {
	if n, ok := g.actorProperty(typ, "SpawnHealth"); ok {
		return int(n)
	}
	return shootableThingSpawnHealth(typ)
}
func (g *game) demoTraceSpawnReactionTime(typ int16) int {
	if n, ok := g.actorProperty(typ, "ReactionTime"); ok {
		return int(n)
	}
	return demoTraceSpawnReactionTime(typ)
}
func (g *game) monsterPainChance(typ int16) int {
	if n, ok := g.actorProperty(typ, "PainChance"); ok {
		return int(n)
	}
	return monsterPainChance(typ)
}
func (g *game) thingTypeMass(typ int16) int {
	if n, ok := g.actorProperty(typ, "Mass"); ok {
		return int(n)
	}
	return thingTypeMass(typ)
}
func (g *game) monsterMoveStep(typ int16, fast bool) int64 {
	if n, ok := g.actorProperty(typ, "Speed"); ok {
		return int64(n) * fracUnit
	}
	return monsterMoveStep(typ, fast)
}
func (g *game) monsterRadius(typ int16) int64 {
	if n, ok := g.actorProperty(typ, "Radius"); ok {
		return int64(n)
	}
	return monsterRadius(typ)
}
func (g *game) monsterHeight(typ int16) int64 {
	if n, ok := g.actorProperty(typ, "Height"); ok {
		return int64(n)
	}
	return monsterHeight(typ)
}
func (g *game) thingTypeRadius(typ int16) int64 {
	if n, ok := g.actorProperty(typ, "Radius"); ok {
		return int64(n)
	}
	return thingTypeRadius(typ)
}
func (g *game) demoTraceThingBounds(typ int16) (int64, int64) {
	radius, height := demoTraceThingBounds(typ)
	if n, ok := g.actorProperty(typ, "Radius"); ok {
		radius = int64(n)
	}
	if n, ok := g.actorProperty(typ, "Height"); ok {
		height = int64(n)
	}
	return radius, height
}

// ValidateDehackedDefinitions checks the runtime subset before launching.
func ValidateDehackedDefinitions(defs *dehacked.Definitions) error {
	_, err := compileActorDefinitions(defs)
	return err
}

func definitionDigest(opts Options) string {
	if opts.DehackedDefinitions == nil {
		return ""
	}
	return opts.DehackedDefinitions.Digest()
}

// ValidateDehackedOptions also prevents patched sessions from entering network
// protocols which do not yet negotiate a resolved-definition identity.
func ValidateDehackedOptions(opts Options) error {
	if err := ValidateDehackedDefinitions(opts.DehackedDefinitions); err != nil {
		return err
	}
	return validateDehackedNetworkOptions(opts)
}
func validateDehackedNetworkOptions(opts Options) error {
	if opts.DehackedDefinitions != nil && (opts.CoopPeers != nil || opts.LiveTicSource != nil || opts.LiveTicSink != nil || opts.AuthorityClient != nil) {
		return fmt.Errorf("DeHackEd network sessions are not supported yet")
	}
	return nil
}
