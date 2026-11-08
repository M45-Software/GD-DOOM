package doomruntime

import "testing"

func TestMonsterNewChaseDirUsesCurrentRetaliationTarget(t *testing.T) {
	for _, playerTarget := range []bool{false, true} {
		g := boomTouchFixture(t)
		// A failed skull-fly probe can change the target from the actor
		// west of us to a player or monster north of us, before A_Chase
		// invokes P_NewChaseDir. Its earlier coordinates are stale.
		g.setThingPosFixed(1, -8*fracUnit, 0)
		g.setThingPosFixed(2, -8*fracUnit, 128*fracUnit)
		g.thingMoveDir[1] = monsterDirNoDir
		g.thingTargetPlayer[1] = playerTarget
		g.thingTargetIdx[1] = 2
		if playerTarget {
			g.setThingPosFixed(2, 1024*fracUnit, 0)
			g.setPlayerPosFixed(-8*fracUnit, 128*fracUnit)
			g.thingTargetIdx[1] = -1
		}
		g.monsterPickNewChaseDir(1, g.m.Things[1].Type, -256*fracUnit, 0)
		x, y := g.thingPosFixed(1, g.m.Things[1])
		if x != -8*fracUnit || y != 8*fracUnit || g.thingMoveDir[1] != monsterDirNorth {
			t.Fatalf("player target=%t: position=(%d,%d) direction=%d", playerTarget, x, y, g.thingMoveDir[1])
		}
	}
}
