package doomruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"

	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
	"gddoom/internal/sessionflow"
)

const maxCoopCheckpointBytes = 32 << 20

// A recorded-session checkpoint uses the same owned player/world fields as an
// authority baseline, plus the command stream and campaign state. It does not
// turn network render baselines into rollback checkpoints.
type coopCheckpoint struct {
	SinglePlayer                                                                *authorityReplicaPlayer `json:",omitempty"`
	Single                                                                      bool                    `json:",omitempty"`
	LevelCarryover                                                              *playerLevelCarryover   `json:",omitempty"`
	Version                                                                     int
	Demo                                                                        []byte
	Tick                                                                        int
	Replica                                                                     authorityReplica
	Paused, IntermissionActive, Skip, FinaleActive, FinaleCommercial, WorldDone bool
	Prepared, Exit, Secret                                                      bool
	SoundBodies, Attackers                                                      map[int]int
	Frags                                                                       [4][4]int
	Intermission                                                                intermissionState
	Anims                                                                       [][9]int
	Finale                                                                      sessionFinale
	Next                                                                        mapdata.MapName
	Carryover                                                                   []coopLevelCarryover
	SecretVisited                                                               bool
	nextMap                                                                     *mapdata.Map
}

func (sg *sessionGame) captureCoopCheckpoint() ([]byte, error) {
	g := sg.g
	if !g.coopDemoActive() && !g.singleDemoCheckpointActive() {
		return nil, nil
	}
	stream, err := demo.Format(g.opts.DemoScript)
	if err != nil {
		return nil, err
	}
	c := coopCheckpoint{Version: 2, Demo: stream, Tick: g.demoTick,
		Paused: g.demoPaused, IntermissionActive: g.demoIntermissionActive, Skip: g.demoIntermissionSkip,
		FinaleActive: g.demoFinaleActive, FinaleCommercial: g.demoFinaleCommercial, WorldDone: g.demoWorldDone,
		Prepared: g.coopTransitionPrepared, Exit: g.levelExitRequested, Secret: g.secretLevelExit,
		SoundBodies: maps.Clone(g.demoCoopSoundBodies), Attackers: maps.Clone(g.demoCoopAttackers), Frags: g.demoCoopFrags,
		Intermission: sg.intermission.state, Finale: sg.finale, Carryover: sg.coopCarryover, SecretVisited: sg.secretVisited}
	if g.singleDemoCheckpointActive() {
		c.Version, c.Single, c.LevelCarryover = 3, true, sg.levelCarryover
		p := captureReplicaPlayer(g.captureAuthoritativePlayer())
		for weapon, owned := range p.Inventory.Weapons {
			if !owned {
				delete(p.Inventory.Weapons, weapon)
			}
		}
		if len(p.Inventory.Weapons) == 0 {
			p.Inventory.Weapons = nil
		}
		c.SinglePlayer = &p
	} else {
		a := &Authority{g: g}
		for _, p := range g.authorityPlayers {
			state := *p
			if state.localSlot == g.localSlot {
				state = g.captureAuthoritativePlayer()
			}
			a.players[state.localSlot] = &state
		}
		r, err := a.captureSharedAuthorityReplica()
		if err != nil {
			return nil, err
		}
		// The save codec stores owned weapons only; absent and false both mean
		// unowned. Canonicalize the roster to the same representation.
		for i := range r.Players {
			for weapon, owned := range r.Players[i].Inventory.Weapons {
				if !owned {
					delete(r.Players[i].Inventory.Weapons, weapon)
				}
			}
			if len(r.Players[i].Inventory.Weapons) == 0 {
				r.Players[i].Inventory.Weapons = nil
			}
		}
		for weapon, owned := range r.Game.Inventory.Weapons {
			if !owned {
				delete(r.Game.Inventory.Weapons, weapon)
			}
		}
		if len(r.Game.Inventory.Weapons) == 0 {
			r.Game.Inventory.Weapons = nil
		}
		r.Viewer = byte(g.localSlot)
		if err := validateAuthorityReplica(g, r); err != nil {
			return nil, err
		}
		c.Replica = r
	}
	if sg.intermission.nextMap != nil {
		c.Next = sg.intermission.nextMap.Name
	}
	for _, anim := range c.Intermission.Anims {
		d := anim.def
		c.Anims = append(c.Anims, [9]int{int(d.kind), d.period, d.nanims, d.x, d.y, d.data1, d.data2, anim.nextTic, anim.ctr})
	}
	c.Intermission.Anims = nil
	c.Replica.Game = gameSaveState{} // The binary envelope owns the shared world.
	data, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	if len(data) > maxCoopCheckpointBytes {
		return nil, fmt.Errorf("co-op checkpoint too large")
	}
	return data, nil
}

func decodeCoopCheckpoint(data []byte) (*coopCheckpoint, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if len(data) > maxCoopCheckpointBytes {
		return nil, fmt.Errorf("co-op checkpoint too large")
	}
	c := new(coopCheckpoint)
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(c); err != nil {
		return nil, fmt.Errorf("co-op checkpoint: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("trailing co-op checkpoint data")
	}
	if c.Version < 1 || c.Version > 3 {
		return nil, fmt.Errorf("unsupported co-op checkpoint version")
	}
	if c.Finale.Stage == 2 && !c.Finale.Cast.Valid() {
		return nil, fmt.Errorf("checkpoint lacks a valid cast actor")
	}
	return c, nil
}

func (c *coopCheckpoint) validate(g *game, file saveFile) error {
	script := g.opts.DemoScript
	frames := 0
	if script != nil {
		frames = len(script.Frames)
		if c.Single {
			frames = len(script.Tics)
		}
	}
	if script == nil || c.Tick < 0 || c.Tick > frames {
		return fmt.Errorf("invalid campaign checkpoint cursor")
	}
	if c.Single {
		if c.Version < 3 || !g.singleDemoCheckpointActive() || len(c.Replica.Players) != 0 || len(c.Carryover) != 0 || len(c.SoundBodies) != 0 || len(c.Attackers) != 0 {
			return fmt.Errorf("invalid single-player campaign checkpoint")
		}
		savedInventory := file.Game.Inventory
		if len(savedInventory.Weapons) == 0 {
			savedInventory.Weapons = nil
		}
		p := c.SinglePlayer
		if p == nil {
			return fmt.Errorf("single-player checkpoint missing player activation")
		}
		badIdentity := p.LocalSlot != int(script.Header.ConsolePlayer)+1 || p.LocalPlayerThingIndex < -1 || p.LocalPlayerThingIndex >= len(file.Game.Things)
		badWeapon := p.Inventory.ReadyWeapon < int(weaponFist) || p.Inventory.ReadyWeapon > int(weaponChainsaw) || int(p.WeaponState) != file.Game.WeaponState || int(p.WeaponFlashState) != file.Game.WeaponFlashState || p.WeaponAttackDown != file.Game.WeaponAttackDown || p.WeaponRefire != file.Game.WeaponRefire
		badBody := p.P != file.Game.Player || p.Stats != file.Game.Stats || p.PlayerMobjHealth != file.Game.PlayerMobjHealth || p.IsDead != file.Game.IsDead || p.PlayerReborn != file.Game.PlayerReborn
		badAttacker := p.StatusHasAttacker && (p.StatusAttackerThing < selfAttackerThing || p.StatusAttackerThing >= len(file.Game.Things))
		if badIdentity || badWeapon || badBody || badAttacker || !equalAuthorityInventory(p.Inventory, savedInventory) {
			return fmt.Errorf("single-player checkpoint activation does not match saved world")
		}
	} else if !g.coopDemoActive() || len(c.Replica.Players) != script.Header.PlayerCount() {
		return fmt.Errorf("invalid co-op checkpoint roster or cursor")
	}
	for _, p := range c.Replica.Players {
		if p.LocalSlot < 1 || p.LocalSlot > 4 || !script.Header.PlayerInGame[p.LocalSlot-1] {
			return fmt.Errorf("checkpoint player is absent from demo")
		}
	}
	if !c.Single {
		c.Replica.Game = file.Game
		if len(c.Replica.Game.Inventory.Weapons) == 0 {
			c.Replica.Game.Inventory.Weapons = nil
		}
		if err := validateAuthorityReplica(g, c.Replica); err != nil {

			return err
		}
	}
	n := len(file.Game.Things)
	for sector, body := range c.SoundBodies {
		if sector < 0 || sector >= len(g.m.Sectors) || body < 0 || body >= n || file.Game.Things[body].Type != detachedPlayerBodyType {
			return fmt.Errorf("invalid retained sound body")
		}
	}
	for slot, ref := range c.Attackers {
		if slot < 1 || slot > 4 || !script.Header.PlayerInGame[slot-1] || ref == 0 || ref > n || ref < -4 {
			return fmt.Errorf("invalid co-op attacker reference")
		}
		if ref > 0 && file.Game.Things[ref-1].Type != detachedPlayerBodyType {
			return fmt.Errorf("invalid retained attacker body")
		}
		if ref < 0 && !script.Header.PlayerInGame[-ref-1] {
			return fmt.Errorf("inactive co-op attacker")
		}
	}
	if !c.Single && c.Replica.Viewer != byte(script.Header.ConsolePlayer+1) || c.IntermissionActive && c.FinaleActive || c.FinaleActive != c.Finale.Active || c.FinaleCommercial && !c.FinaleActive && !c.WorldDone || c.Finale.Stage > 2 {
		return fmt.Errorf("inconsistent co-op campaign phase")
	}
	if c.Single && (c.IntermissionActive || c.FinaleCommercial || c.WorldDone) && c.LevelCarryover == nil {
		return fmt.Errorf("single-player campaign checkpoint missing carryover")
	}
	if !c.Single && (c.LevelCarryover != nil || c.SinglePlayer != nil) {
		return fmt.Errorf("co-op checkpoint has single-player carryover")
	}
	carrySlots := [4]bool{}
	for _, carry := range c.Carryover {
		if carry.Slot < 1 || carry.Slot > 4 || !script.Header.PlayerInGame[carry.Slot-1] || carrySlots[carry.Slot-1] {
			return fmt.Errorf("invalid campaign carryover roster")
		}
		carrySlots[carry.Slot-1] = true
	}
	if !c.Single && (c.IntermissionActive || c.FinaleCommercial || c.WorldDone) && len(c.Carryover) != len(c.Replica.Players) {
		return fmt.Errorf("campaign checkpoint missing player carryover")
	}
	if c.Intermission.TerminalEpisode {
		if _, ok := sessionflow.StartFinale(file.Current, false); !ok || !g.boomMomentumDropoff || c.Secret {
			return fmt.Errorf("invalid episode-victory checkpoint")
		}
	}
	if len(c.Anims) > 64 || len(c.Carryover) > 4 || c.Intermission.Tic < 0 || c.Finale.Tic < 0 {
		return fmt.Errorf("invalid campaign checkpoint")
	}
	if c.Next != "" {
		if len(c.Next) > 8 {
			return fmt.Errorf("invalid next map")
		}
		var err error
		c.nextMap, err = g.opts.NewGameLoader(string(c.Next))
		if err != nil {
			return err
		}
		if c.nextMap == nil || c.nextMap.Name != c.Next {
			return fmt.Errorf("missing next map")
		}
	}
	if (c.WorldDone || c.IntermissionActive || c.FinaleCommercial) && c.nextMap == nil && !c.Intermission.TerminalEpisode {
		return fmt.Errorf("campaign checkpoint missing next map")
	}
	for _, a := range c.Anims {
		if a[0] < 0 || a[0] > 2 || a[1] <= 0 || a[2] < 1 || a[2] > 16 {
			return fmt.Errorf("invalid intermission animation")
		}
	}
	return nil
}

func (c *coopCheckpoint) restoreGame(g *game) {
	if c.Single {
		g.applyAuthoritativePlayer(restoreReplicaPlayer(*c.SinglePlayer))
	}
	if !c.Single {
		g.applyValidatedAuthorityReplica(c.Replica)
		// Network baselines reset interpolation for rendering; checkpoints retain
		// the exact player activation fields, including the previous frame pose.
		for _, p := range c.Replica.Players {
			if p.LocalSlot == int(c.Replica.Viewer) {
				g.applyAuthoritativePlayer(restoreReplicaPlayer(p))
				break
			}
		}
		g.State = c.Replica.Game.View
	}
	g.demoTick, c.Replica.Game = c.Tick, gameSaveState{}
	g.demoPaused, g.demoIntermissionActive, g.demoIntermissionSkip = c.Paused, c.IntermissionActive, c.Skip
	g.demoFinaleActive, g.demoFinaleCommercial, g.demoWorldDone = c.FinaleActive, c.FinaleCommercial, c.WorldDone
	g.coopTransitionPrepared, g.levelExitRequested, g.secretLevelExit = c.Prepared, c.Exit, c.Secret
	g.demoCoopSoundBodies, g.demoCoopAttackers, g.demoCoopFrags = maps.Clone(c.SoundBodies), maps.Clone(c.Attackers), c.Frags
}

func (c *coopCheckpoint) restoreSession(sg *sessionGame) {
	sg.levelCarryover = c.LevelCarryover
	sg.intermission = sessionIntermission{state: c.Intermission, nextMap: c.nextMap}
	for _, a := range c.Anims {
		sg.intermission.state.Anims = append(sg.intermission.state.Anims, intermissionAnimState{intermissionAnimDef{intermissionAnimKind(a[0]), a[1], a[2], a[3], a[4], a[5], a[6]}, a[7], a[8]})
	}
	sg.finale, sg.coopCarryover, sg.secretVisited = c.Finale, c.Carryover, c.SecretVisited
}

// Single-player Boom recordings share the campaign attachment without creating
// an authority roster or enabling co-op spawning and collision rules.
func (g *game) singleDemoCheckpointActive() bool {
	s := g.opts.DemoScript
	return s != nil && g.boomMomentumDropoff && g.opts.GameMode == gameModeSingle && s.Header.PlayerCount() == 1 && len(s.Tics) > 0
}
