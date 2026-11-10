// Package dehacked provides session-local Doom definitions and transactional
// text DeHackEd/BEX patches. It does not mutate or execute the gameplay engine.
package dehacked

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"maps"
	"reflect"
	"slices"
	"sync"
)

const (
	NumThings  = 137
	NumStates  = 967
	NumSprites = 138
	NumSounds  = 109
	NumWeapons = 9
	NumAmmo    = 4
	NumMusic   = 68
)

type StateID int32
type SoundID int32
type SpriteID int32
type AmmoID int32

const (
	AmmoBullets AmmoID = iota
	AmmoShells
	AmmoCells
	AmmoRockets
	// Four is NUMAMMO, not a usable ammo type in a weapon definition.
	AmmoNone AmmoID = 5
)

// Action names are canonical Doom mnemonics without the A_ prefix. NoAction
// denotes a null code pointer, not the S_NULL state (state number zero).
type Action string

const NoAction Action = "NULL"

// Thing is a mobjinfo slot, indexed from zero internally. DeHackEd's Thing
// numbers start at one; EditorNumber is the independent number used in maps.
// Speed retains Doom's original representation: integer for walking monsters,
// fixed point for missiles. Radius and Height are 16.16 fixed point.
type Thing struct {
	Name                                              string
	EditorNumber                                      int32
	SpawnState                                        StateID
	SpawnHealth                                       int32
	SeeState                                          StateID
	SeeSound                                          SoundID
	ReactionTime                                      int32
	AttackSound                                       SoundID
	PainState                                         StateID
	PainChance                                        int32
	PainSound                                         SoundID
	MeleeState, MissileState, DeathState, XDeathState StateID
	DeathSound                                        SoundID
	Speed, Radius, Height, Mass, Damage               int32
	ActiveSound                                       SoundID
	Flags                                             uint32
	RaiseState                                        StateID
}

// State uses original Doom frame numbers for actors, effects and weapons.
// Frame includes the fullbright bit. Tics == -1 is an infinite duration;
// zero-duration chains and loops are preserved for the eventual runtime.
type State struct {
	Name         string
	Sprite       SpriteID
	Frame, Tics  int32
	Action       Action
	Next         StateID
	Misc1, Misc2 int32
}

type Weapon struct {
	Ammo                           AmmoID
	Up, Down, Ready, Attack, Flash StateID
}

type Ammo struct{ Max, PerPickup int32 }

// Sound contains definition data, never loaded audio or cache pointers.
type Sound struct {
	Name                  string
	Singularity, Priority int32
	Link                  SoundID // -1 means no linked sound
	Pitch, Volume         int32
}

type Misc struct {
	InitialHealth, InitialBullets, MaxHealth, MaxArmor int32
	GreenArmorClass, BlueArmorClass                    int32
	MaxSoulsphere, SoulsphereHealth, MegasphereHealth  int32
	GodModeHealth, IDFAArmor, IDFAArmorClass           int32
	IDKFAArmor, IDKFAArmorClass, BFGCellsPerShot       int32
	MonstersInfight                                    int32 // DEH values: 202 or 221
}

// Snapshot is an owned copy suitable for inspection or later runtime adoption.
// Changes to it never affect the originating Definitions or vanilla defaults.
type Snapshot struct {
	Things           []Thing
	States           []State
	Weapons          []Weapon
	Ammo             []Ammo
	Sounds           []Sound
	Sprites          []string
	Music            []string
	Strings          map[string]string
	TextReplacements map[string]string
	// Defined records explicit writes, even when the value equals the default.
	// Boom uses this distinction for health limits and predefined actor flags.
	Defined map[string]bool
	Cheats  map[string]string
	Pars    map[string]int32 // E1M1 or MAP01; seconds, not tics
	Misc    Misc
}

type Definitions struct {
	data       Snapshot
	digestOnce sync.Once
	digest     string
}

// Vanilla returns a fresh copy of the original Doom 1.9 definitions.
func Vanilla() *Definitions {
	return &Definitions{data: cloneSnapshot(Snapshot{
		Things: vanillaThings[:], States: vanillaStates[:], Weapons: vanillaWeapons[:],
		Ammo: vanillaAmmo[:], Sounds: vanillaSounds[:], Sprites: vanillaSprites[:],
		Music: vanillaMusic[:], Strings: vanillaStrings, Cheats: vanillaCheats,
		Pars: vanillaPars, Misc: vanillaMisc, TextReplacements: map[string]string{}, Defined: map[string]bool{},
	})}
}

func cloneSnapshot(s Snapshot) Snapshot {
	s.Things = slices.Clone(s.Things)
	s.States = slices.Clone(s.States)
	s.Weapons = slices.Clone(s.Weapons)
	s.Ammo = slices.Clone(s.Ammo)
	s.Sounds = slices.Clone(s.Sounds)
	s.Sprites = slices.Clone(s.Sprites)
	s.Music = slices.Clone(s.Music)
	s.Strings = maps.Clone(s.Strings)
	s.TextReplacements = maps.Clone(s.TextReplacements)
	s.Defined = maps.Clone(s.Defined)
	s.Cheats = maps.Clone(s.Cheats)
	s.Pars = maps.Clone(s.Pars)
	return s
}

func (d *Definitions) Snapshot() Snapshot { return cloneSnapshot(d.data) }

// FieldWasSet uses parser section names (thing/frame/weapon/ammo/sound/misc),
// zero-based table indices, and the exported definition field name. Code-pointer
// writes are recorded as frame/INDEX/Action. Scalars such as misc use index 0.
func (d *Definitions) FieldWasSet(section string, index int, member string) bool {
	return d.data.Defined[fieldKey(section, index, member)]
}

func fieldKey(section string, index int, member string) string {
	return fmt.Sprintf("%s/%d/%s", section, index, member)
}

// VanillaSourceHashes returns provenance for the compiled canonical tables.
func VanillaSourceHashes() map[string]string { return maps.Clone(vanillaSourceSHA256) }

func (d *Definitions) Thing(index int) (Thing, bool) {
	if index < 0 || index >= len(d.data.Things) {
		return Thing{}, false
	}
	return d.data.Things[index], true
}

func (d *Definitions) ThingByNumber(number int) (Thing, bool) { return d.Thing(number - 1) }

func (d *Definitions) ThingForEditorNumber(number int32) (int, Thing, bool) {
	for i, thing := range d.data.Things {
		if thing.EditorNumber == number && number >= 0 {
			return i, thing, true
		}
	}
	return 0, Thing{}, false
}

func (d *Definitions) State(id StateID) (State, bool) {
	if id < 0 || int(id) >= len(d.data.States) {
		return State{}, false
	}
	return d.data.States[id], true
}

func (d *Definitions) Weapon(index int) (Weapon, bool) {
	if index < 0 || index >= len(d.data.Weapons) {
		return Weapon{}, false
	}
	return d.data.Weapons[index], true
}
func (d *Definitions) Ammo(id AmmoID) (Ammo, bool) {
	if id < 0 || int(id) >= len(d.data.Ammo) {
		return Ammo{}, false
	}
	return d.data.Ammo[id], true
}
func (d *Definitions) Sound(id SoundID) (Sound, bool) {
	if id < 0 || int(id) >= len(d.data.Sounds) {
		return Sound{}, false
	}
	return d.data.Sounds[id], true
}
func (d *Definitions) Sprite(id SpriteID) (string, bool) {
	if id < 0 || int(id) >= len(d.data.Sprites) {
		return "", false
	}
	return d.data.Sprites[id], true
}
func (d *Definitions) Music(index int) (string, bool) {
	if index < 0 || index >= len(d.data.Music) {
		return "", false
	}
	return d.data.Music[index], true
}
func (d *Definitions) String(key string) (string, bool) {
	v, ok := d.data.Strings[upper(key)]
	return v, ok
}
func (d *Definitions) Cheat(key string) (string, bool) {
	v, ok := d.data.Cheats[lower(key)]
	return v, ok
}
func (d *Definitions) Par(mapName string) (int32, bool) {
	v, ok := d.data.Pars[upper(mapName)]
	return v, ok
}
func (d *Definitions) Misc() Misc { return d.data.Misc }

// Digest identifies all resolved definition data. Encoding is length-delimited,
// sorts map keys, and preserves arbitrary legacy string bytes (including invalid
// UTF-8), so comments and patch filenames do not change the definition identity.
// Nil identifies an unpatched runtime with an empty digest. Immutable results
// are cached for per-tick authority baselines and concurrent discovery callers.
func (d *Definitions) Digest() string {
	if d == nil {
		return ""
	}
	d.digestOnce.Do(func() {
		h := sha256.New()
		h.Write([]byte("gddoom-dehacked-definitions-v1\x00"))
		digestValue(h, reflect.ValueOf(d.data))
		d.digest = hex.EncodeToString(h.Sum(nil))
	})
	return d.digest
}

func digestValue(h hash.Hash, v reflect.Value) {
	number := func(n uint64) { var b [8]byte; binary.LittleEndian.PutUint64(b[:], n); h.Write(b[:]) }
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			digestValue(h, v.Field(i))
		}
	case reflect.Slice:
		number(uint64(v.Len()))
		for i := 0; i < v.Len(); i++ {
			digestValue(h, v.Index(i))
		}
	case reflect.Map:
		keys := v.MapKeys()
		slices.SortFunc(keys, func(a, b reflect.Value) int {
			if a.String() < b.String() {
				return -1
			}
			if a.String() > b.String() {
				return 1
			}
			return 0
		})
		number(uint64(len(keys)))
		for _, k := range keys {
			digestValue(h, k)
			digestValue(h, v.MapIndex(k))
		}
	case reflect.String:
		s := v.String()
		number(uint64(len(s)))
		h.Write([]byte(s))
	case reflect.Int32:
		number(uint64(v.Int()))
	case reflect.Uint32:
		number(v.Uint())
	case reflect.Bool:
		if v.Bool() {
			number(1)
		} else {
			number(0)
		}
	default:
		panic("unsupported definition digest field: " + v.Type().String())
	}
}
