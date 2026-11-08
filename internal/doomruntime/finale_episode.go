package doomruntime

import "gddoom/internal/sessionflow"

func (sg *sessionGame) boomEpisodeEnd() bool {
	if sg.g == nil || !sg.g.boomMomentumDropoff || sg.g.secretLevelExit {
		return false
	}
	_, ok := sessionflow.StartFinale(sg.current, false)
	return ok
}

// Boom shows the final map's statistics before queuing ga_victory. The
// following command starts F_Ticker, preserving the same roster and latches.
func (sg *sessionGame) startQueuedEpisodeFinale() bool {
	if sg.g == nil || !sg.g.demoWorldDone || !sg.intermission.state.TerminalEpisode {
		return false
	}
	sg.g.demoWorldDone = false
	sg.startEpisodeFinale(sg.current, false)
	sg.intermission = sessionIntermission{}
	return true
}
