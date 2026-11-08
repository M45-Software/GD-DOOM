package doomruntime

import "gddoom/internal/runtimecfg"

// Boom encodes the selected weapon itself, including SSG (slot 9). The old
// Doom demo path resolves the fist/saw and shotgun/SSG toggles in P_PlayerThink.
func commandWeapon(slot int) weaponID {
	switch slot {
	case 1:
		return weaponFist
	case 2:
		return weaponPistol
	case 3:
		return weaponShotgun
	case 4:
		return weaponChaingun
	case 5:
		return weaponRocketLauncher
	case 6:
		return weaponPlasma
	case 7:
		return weaponBFG
	case 8:
		return weaponChainsaw
	case 9:
		return weaponSuperShotgun
	}
	return 0
}
func (g *game) selectCommandWeapon(slot int) {
	if !g.boomMomentumDropoff {
		g.selectWeaponSlot(((slot - 1) & 7) + 1)
		return
	}
	if next := commandWeapon(slot); next != 0 {
		g.queueWeaponSwitch(next)
	}
}

// Vanilla simulation retains the default order and its historical ammo
// thresholds. Custom preferences apply only to Boom local command generation.
func (g *game) fallbackWeapon(boom bool) weaponID {
	shells, cells := 3, 41
	order := runtimecfg.DefaultWeaponPreferences()
	if boom {
		shells, cells = 2, 40
		order = g.opts.WeaponPreferences.Normalized()
	}
	// Zero is the unpowered fist, after the ranked berserk fist.
	for _, slot := range append(order[:], 0) {
		w := commandWeapon(slot)
		usable := false
		switch slot {
		case 0:
			w, usable = weaponFist, true
		case 1:
			usable = g.inventory.Strength
		case 2, 4:
			usable = g.stats.Bullets >= 1
		case 3:
			usable = g.stats.Shells >= 1
		case 9:
			usable = g.stats.Shells >= shells
		case 5:
			usable = g.stats.Rockets >= 1
		case 6:
			usable = g.stats.Cells >= 1
		case 7:
			usable = g.stats.Cells >= cells
		case 8:
			usable = true
		}
		if w != g.inventory.ReadyWeapon && usable && g.weaponOwned(w) {
			return w
		}
	}
	return g.inventory.ReadyWeapon
}

func (g *game) weaponPreferred(first, second weaponID) bool {
	for _, slot := range g.opts.WeaponPreferences.Normalized() {
		w := commandWeapon(slot)
		if w == first {
			return true
		}
		if w == second {
			return false
		}
	}
	return false
}

// This is input policy, not simulation: demos and received network commands
// must never invent a switch when an empty weapon attempts to fire.
func (g *game) localWeaponCommand(slot int, fire bool) int {
	if !g.boomMomentumDropoff {
		return slot
	}
	if !g.weaponAttackDown {
		g.boomAutoSwitchDone = false
	}
	if !g.isDead && g.weaponAttackDown && !g.canFireSelectedWeapon() && (!g.boomAutoSwitchDone || fire && g.inventory.PendingWeapon == 0) {
		g.boomAutoSwitchDone = true
		return demoTraceWeaponID(g.fallbackWeapon(true)) + 1
	}
	next := commandWeapon(slot)
	if slot == 1 && g.weaponOwned(weaponChainsaw) && g.inventory.ReadyWeapon != weaponChainsaw &&
		(g.inventory.ReadyWeapon == weaponFist || !g.inventory.Strength || g.weaponPreferred(weaponChainsaw, weaponFist)) {
		next = weaponChainsaw
	}
	if slot == 3 && g.weaponOwned(weaponSuperShotgun) && (!g.weaponOwned(weaponShotgun) || g.inventory.ReadyWeapon == weaponShotgun ||
		(g.inventory.ReadyWeapon != weaponSuperShotgun && g.weaponPreferred(weaponSuperShotgun, weaponShotgun))) {
		next = weaponSuperShotgun
	}
	if next != 0 {
		return demoTraceWeaponID(next) + 1
	}
	return slot
}
