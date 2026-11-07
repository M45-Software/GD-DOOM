package doomruntime

import "fmt"

func (g *game) boomDoorLightLine(line int) int {
	if !g.boomMomentumDropoff || line < 0 || line >= len(g.m.Linedefs) || g.m.Linedefs[line].Tag == 0 {
		return 0
	}
	return line + 1
}

// Boom 2.02 changes lights on pastdest, including a blocked final snap.
// MBF's gradual, per-tic interpolation is deliberately not used here.
func (g *game) finishBoomDoorLighting(d *doorThinker, opening bool) {
	if d.lightLine <= 0 || d.lightLine > len(g.m.Linedefs) || d.topHeight == g.sectorFloor[d.sector] {
		return
	}
	for _, sec := range g.taggedSectorsForLine(d.lightLine - 1) {
		bright, dark := int16(0), g.m.Sectors[sec].Light
		for _, line := range g.m.Linedefs {
			front, frontOK := g.sectorIndexFromSidedef(line.SideNum[0])
			back, backOK := g.sectorIndexFromSidedef(line.SideNum[1])
			if !frontOK || !backOK || front == back {
				continue
			}
			other := -1
			if front == sec {
				other = back
			} else if back == sec {
				other = front
			}
			if other < 0 {
				continue
			}
			light := g.m.Sectors[other].Light
			if light > bright {
				bright = light
			}
			if light < dark {
				dark = light
			}
		}
		g.m.Sectors[sec].Light = dark
		if opening {
			g.m.Sectors[sec].Light = bright
		}
	}
	g.refreshSectorPlaneCacheLighting()
}

// Keep the original binary door records readable; append sparse line references
// in the new snapshot extension, for both owned and displaced door thinkers.
func doorLightingLines(doors map[int]doorThinkerSaveState) []int {
	var result []int
	for _, key := range sortedSaveIntKeys(doors) {
		if line := doors[key].LightLine; line != 0 {
			result = append(result, key, line)
		}
	}
	return result
}

func restoreDoorLightingLines(doors map[int]doorThinkerSaveState, pairs []int) error {
	if len(pairs)%2 != 0 {
		return fmt.Errorf("invalid door lighting references")
	}
	for i := 0; i < len(pairs); i += 2 {
		key, line := pairs[i], pairs[i+1]
		d, ok := doors[key]
		if !ok || line <= 0 || d.LightLine != 0 {
			return fmt.Errorf("invalid door lighting reference")
		}
		d.LightLine = line
		doors[key] = d
	}
	return nil
}

func validDoorLightingState(s gameSaveState, lines int) bool {
	for _, doors := range []map[int]doorThinkerSaveState{s.Doors, s.ExtraDoors} {
		for _, d := range doors {
			if d.LightLine < 0 || d.LightLine > lines {
				return false
			}
		}
	}
	return true
}
