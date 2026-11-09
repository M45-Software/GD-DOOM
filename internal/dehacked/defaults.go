package dehacked

// Misc patch values use the original DEH representation. In particular 202
// disables and 221 enables same-species infighting.
var vanillaMisc = Misc{
	InitialHealth: 100, InitialBullets: 50, MaxHealth: 100, MaxArmor: 200,
	GreenArmorClass: 1, BlueArmorClass: 2, MaxSoulsphere: 200, SoulsphereHealth: 100,
	MegasphereHealth: 200, GodModeHealth: 100, IDFAArmor: 200, IDFAArmorClass: 2,
	IDKFAArmor: 200, IDKFAArmorClass: 2, BFGCellsPerShot: 40, MonstersInfight: 202,
}

var vanillaCheats = map[string]string{
	"change music": "idmus", "chainsaw": "idchoppers", "god mode": "iddqd",
	"ammo & keys": "idkfa", "ammo": "idfa", "no clipping 1": "idspispopd",
	"no clipping 2": "idclip", "invincibility": "idbeholdv", "berserk": "idbeholds",
	"invisibility": "idbeholdi", "radiation suit": "idbeholdr", "auto-map": "idbeholda",
	"lite-amp goggles": "idbeholdl", "behold menu": "idbehold", "level warp": "idclev",
	"player position": "idmypos", "map cheat": "iddt",
}

// Bits are original 32-bit mobj flags, not this engine's actor classification.
// The upper Boom translucency bit is deliberately kept unsigned.
var thingFlags = map[string]uint32{
	"SPECIAL": 1 << 0, "SOLID": 1 << 1, "SHOOTABLE": 1 << 2,
	"NOSECTOR": 1 << 3, "NOBLOCKMAP": 1 << 4, "AMBUSH": 1 << 5,
	"JUSTHIT": 1 << 6, "JUSTATTACKED": 1 << 7, "SPAWNCEILING": 1 << 8,
	"NOGRAVITY": 1 << 9, "DROPOFF": 1 << 10, "PICKUP": 1 << 11,
	"NOCLIP": 1 << 12, "SLIDE": 1 << 13, "FLOAT": 1 << 14,
	"TELEPORT": 1 << 15, "MISSILE": 1 << 16, "DROPPED": 1 << 17,
	"SHADOW": 1 << 18, "NOBLOOD": 1 << 19, "CORPSE": 1 << 20,
	"INFLOAT": 1 << 21, "COUNTKILL": 1 << 22, "COUNTITEM": 1 << 23,
	"SKULLFLY": 1 << 24, "NOTDMATCH": 1 << 25,
	"TRANSLATION1": 1 << 26, "TRANSLATION2": 1 << 27, "TRANSLUCENT": 1 << 31,
	"TRANSLATION": 1 << 26, "UNUSED1": 1 << 27, "UNUSED4": 1 << 31,
}

type fieldKind uint8

const (
	integerField fieldKind = iota
	stateField
	soundField
	spriteField
	ammoField
	flagsField
	pointerField
	ignoredField
)

type fieldSpec struct {
	member string
	kind   fieldKind
}

var fieldSpecs = map[string]map[string]fieldSpec{
	"thing": {
		"id #": {"EditorNumber", integerField}, "initial frame": {"SpawnState", stateField},
		"hit points": {"SpawnHealth", integerField}, "first moving frame": {"SeeState", stateField},
		"alert sound": {"SeeSound", soundField}, "reaction time": {"ReactionTime", integerField},
		"attack sound": {"AttackSound", soundField}, "injury frame": {"PainState", stateField},
		"pain chance": {"PainChance", integerField}, "pain sound": {"PainSound", soundField},
		"close attack frame": {"MeleeState", stateField}, "far attack frame": {"MissileState", stateField},
		"death frame": {"DeathState", stateField}, "exploding frame": {"XDeathState", stateField},
		"death sound": {"DeathSound", soundField}, "speed": {"Speed", integerField},
		"width": {"Radius", integerField}, "height": {"Height", integerField},
		"mass": {"Mass", integerField}, "missile damage": {"Damage", integerField},
		"action sound": {"ActiveSound", soundField}, "bits": {"Flags", flagsField},
		"respawn frame": {"RaiseState", stateField},
	},
	"frame": {
		"sprite number": {"Sprite", spriteField}, "sprite subnumber": {"Frame", integerField},
		"duration": {"Tics", integerField}, "next frame": {"Next", stateField},
		"unknown 1": {"Misc1", integerField}, "unknown 2": {"Misc2", integerField},
	},
	"pointer": {"codep frame": {"Action", pointerField}},
	"ammo":    {"max ammo": {"Max", integerField}, "per ammo": {"PerPickup", integerField}},
	"weapon": {
		"ammo type": {"Ammo", ammoField},
		// These DEH labels are historically reversed relative to up/down.
		"deselect frame": {"Up", stateField}, "select frame": {"Down", stateField},
		"bobbing frame": {"Ready", stateField}, "shooting frame": {"Attack", stateField},
		"firing frame": {"Flash", stateField},
	},
	"sound": {
		"value": {"Priority", integerField}, "zero 2": {"Pitch", integerField},
		"zero 4": {"", ignoredField},
		"offset": {"", ignoredField}, "zero/one": {"", ignoredField},
		"zero 1": {"", ignoredField}, "zero 3": {"", ignoredField},
		"neg. one 1": {"", ignoredField}, "neg. one 2": {"", ignoredField},
	},
	"sprite": {"offset": {"", ignoredField}},
	"misc": {
		"initial health": {"InitialHealth", integerField}, "initial bullets": {"InitialBullets", integerField},
		"max health": {"MaxHealth", integerField}, "max armor": {"MaxArmor", integerField},
		"green armor class": {"GreenArmorClass", integerField}, "blue armor class": {"BlueArmorClass", integerField},
		"max soulsphere": {"MaxSoulsphere", integerField}, "soulsphere health": {"SoulsphereHealth", integerField},
		"megasphere health": {"MegasphereHealth", integerField}, "god mode health": {"GodModeHealth", integerField},
		"idfa armor": {"IDFAArmor", integerField}, "idfa armor class": {"IDFAArmorClass", integerField},
		"idkfa armor": {"IDKFAArmor", integerField}, "idkfa armor class": {"IDKFAArmorClass", integerField},
		"bfg cells/shot": {"BFGCellsPerShot", integerField}, "monsters infight": {"MonstersInfight", integerField},
	},
}
