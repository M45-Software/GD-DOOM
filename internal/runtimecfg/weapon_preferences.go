package runtimecfg

import (
	"fmt"
	"strings"
)

// WeaponPreferences ranks Doom command slots, most preferred first. Slot 1
// ranks the berserk fist; the ordinary fist is always the final fallback.
// The zero value uses Boom's default order.
type WeaponPreferences [9]int

func DefaultWeaponPreferences() WeaponPreferences {
	return WeaponPreferences{6, 9, 4, 3, 2, 8, 5, 7, 1}
}

// Normalized fills omitted entries with the default order and drops invalid
// or repeated slots, including values supplied by embedded runtime hosts.
func (p WeaponPreferences) Normalized() WeaponPreferences {
	var result WeaponPreferences
	var seen [10]bool
	n := 0
	for _, order := range []WeaponPreferences{p, DefaultWeaponPreferences()} {
		for _, slot := range order {
			if slot >= 1 && slot <= 9 && !seen[slot] {
				result[n], seen[slot] = slot, true
				n++
			}
		}
	}
	return result
}

// ParseWeaponPreferences accepts a comma-separated priority list. Omitted
// weapons follow the explicitly ranked weapons in the default Boom order.
func ParseWeaponPreferences(value string) (WeaponPreferences, error) {
	var p WeaponPreferences
	names := map[string]int{"fist": 1, "pistol": 2, "shotgun": 3, "chaingun": 4,
		"rocket": 5, "plasma": 6, "bfg": 7, "chainsaw": 8, "ssg": 9}
	if strings.TrimSpace(value) == "" {
		return p.Normalized(), nil
	}
	var seen [10]bool
	for i, name := range strings.Split(value, ",") {
		name = strings.ToLower(strings.TrimSpace(name))
		slot, ok := names[name]
		if !ok {
			return WeaponPreferences{}, fmt.Errorf("unknown weapon %q (use fist,pistol,shotgun,chaingun,rocket,plasma,bfg,chainsaw,ssg)", name)
		}
		if seen[slot] {
			return WeaponPreferences{}, fmt.Errorf("duplicate weapon %q", name)
		}
		seen[slot] = true
		p[i] = slot
	}
	return p.Normalized(), nil
}
