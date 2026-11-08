package sessionflow

type Cast struct {
	Actor, State, Tics, Frames, OnMelee int
	Death, Attacking                    bool
	Cue                                 string
}
type castActor struct {
	Name                     string
	See, Melee, Missile, Die int
	SeeSound, DeathSound     string
}
type castStateDef struct {
	Sprite            string
	Frame, Tics, Next int
	Cue               string
}

func startCast() Cast {
	return Cast{State: castActors[0].See, Tics: castStates[castActors[0].See].Tics}
}

func (c Cast) Valid() bool {
	_, ok := castStates[c.State]
	return ok && c.State > 0 && c.Actor >= 0 && c.Actor < len(castActors) && c.Frames >= 0 && c.Frames <= 24 && c.OnMelee >= 0 && c.OnMelee <= 1 && c.Tics >= -1 && c.Tics <= 1000
}
func (c Cast) Name() string {
	if c.Actor < 0 || c.Actor >= len(castActors) {
		return ""
	}
	return castActors[c.Actor].Name
}
func (c Cast) Sprite() (string, byte) { s := castStates[c.State]; return s.Sprite, byte('A' + s.Frame) }

// KillCast responds to a host key event. Recorded ticcmd fire/use does not
// represent F_Responder key events and must never kill a cast actor.
func (f *Finale) KillCast() {
	if f.Stage != FinaleStageCast || f.Cast.Death || !f.Cast.Valid() {
		return
	}
	c := &f.Cast
	a := castActors[c.Actor]
	c.Death, c.Attacking, c.Frames = true, false, 0
	c.State = a.Die
	c.Tics = castStates[c.State].Tics
	c.Cue = a.DeathSound
}
func (c Cast) tick() Cast {
	c.Cue = ""
	c.Tics--
	if c.Tics > 0 {
		return c
	}
	s := castStates[c.State]
	stop := false
	if s.Tics == -1 || s.Next == 0 {
		c.Actor = (c.Actor + 1) % len(castActors)
		a := castActors[c.Actor]
		c.Death = false
		c.State = a.See
		c.Frames = 0
		c.Cue = a.SeeSound
	} else if c.State == 154 {
		stop = true
	} else {
		c.State = s.Next
		c.Frames++
		c.Cue = castStates[c.State].Cue
	}
	a := castActors[c.Actor]
	if !stop && c.Frames == 12 {
		c.Attacking = true
		if c.OnMelee != 0 {
			c.State = a.Melee
		} else {
			c.State = a.Missile
		}
		c.OnMelee ^= 1
		if c.State == 0 {
			if c.OnMelee != 0 {
				c.State = a.Melee
			} else {
				c.State = a.Missile
			}
		}
	}
	if stop || c.Attacking && (c.Frames == 24 || c.State == a.See) {
		c.Attacking = false
		c.Frames = 0
		c.State = a.See
	}
	c.Tics = castStates[c.State].Tics
	if c.Tics == -1 {
		c.Tics = 15
	}
	return c
}
