package doomruntime

// Effects are non-solid but still run P_XYMovement/P_TryMove. A blocked split
// step clears momentum without discarding the local remainder of that move.
func (g *game) moveBoomEffectXY(x, y *int64, z int64, momx, momy, floorz, ceilz *int64, radius, height int64) {
	for _, momentum := range []*int64{momx, momy} {
		if *momentum > doomMaxMove {
			*momentum = doomMaxMove
		}
		if *momentum < -doomMaxMove {
			*momentum = -doomMaxMove
		}
	}
	xmove, ymove := *momx, *momy
	for xmove != 0 || ymove != 0 {
		var nx, ny int64
		if doomProjectileShouldSplitMove(xmove, ymove) {
			nx, ny = boomFixed(*x+xmove/2), boomFixed(*y+ymove/2)
			xmove, ymove = xmove>>1, ymove>>1
		} else {
			nx, ny = boomFixed(*x+xmove), boomFixed(*y+ymove)
			xmove, ymove = 0, 0
		}
		floor, ceil, drop, ok := g.checkPositionForActorWithThingPolicy(nx, ny, radius, true, -1, true, g.boomMomentumDropoff)
		if ok && ceil-floor >= height && ceil-z >= height && floor-z <= stepHeight && (g.boomMomentumDropoff || floor-drop <= stepHeight) {
			*x, *y, *floorz, *ceilz = nx, ny, floor, ceil
		} else {
			*momx, *momy = 0, 0
		}
	}
	if z <= *floorz {
		if abs(*momx) < stopSpeed && abs(*momy) < stopSpeed {
			*momx, *momy = 0, 0
		} else {
			*momx, *momy = fixedMul(*momx, friction), fixedMul(*momy, friction)
		}
	}
}
