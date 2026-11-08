package doomruntime

import (
	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
)

func (g *game) boomDoorUnstickEnabled() bool {
	if !g.boomMomentumDropoff {
		return false
	}
	if script := g.opts.DemoScript; script != nil {
		return script.Header.Version == demo.VersionBoom202 && !script.Header.BoomCompatibility
	}
	return true
}

func (g *game) activateGeneralizedDoor(line int, info mapdata.LineSpecialInfo, isPlayer bool) bool {
	targets, err := g.m.DoorTargetSectors(line)
	if err != nil {
		return false
	}
	gen := info.Door.Generalized
	if gen.Locked {
		if !isPlayer || !info.Door.CanActivate(g.inventory.keys()) {
			return false
		}
		filtered := targets[:0]
		for _, sec := range targets {
			if !g.ceilingSlotBusy(sec) {
				filtered = append(filtered, sec)
			}
		}
		targets = filtered
	}
	if !g.activateDoorSectorsConfigured(targets, info.Door.Action, true, gen) {
		return false
	}
	if !info.Door.UsesTag {
		for _, sec := range targets {
			g.doors[sec].lightLine = g.boomDoorLightLine(line)
		}
	}
	return true
}
