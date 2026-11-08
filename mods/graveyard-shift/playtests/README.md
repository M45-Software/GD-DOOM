# Map 1 recorded playtests

The default recording is now **detailed-showcase/attempt-07-detailed/session.lmp**:
a completed Hurt Me Plenty run through the detailed circular-carousel version.
It uses weighted turns and running, with no cheats, and finishes in **2:38.40**
with **36/36 kills, 3/3 secrets, 75 health and 153 armor**. All monsters were dead
at **2:17.20**, before a short walk around and onto the carousel.

GD-DOOM's normal demo player reproduced the exact final position, inventory and
outcome. Chocolate Doom 3.0.0 independently reported 36/36 kills and 3/3 secrets.
These are tool-assisted timings, not human completion-time estimates.

The earlier **attempt-04-classic-clear** remains available against its original
pinned WAD. It completes the earlier layout in 3:00.11 with all kills and secrets.
The `showcase/` folder contains two intermediate weighted-control experiments;
attempt 06 intentionally stops after collecting the blue key and is not a full run.

## Watch

From the GD-DOOM repository root:

```bash
./mods/graveyard-shift/playtests/watch.sh
# Watch the failed classic-settings attempt:
./mods/graveyard-shift/playtests/watch.sh attempt-03-classic
```

Demo viewing defaults to modern mode at 1920×1080 with full-resolution detail.
You can override the display settings after the attempt name, for example:

```bash
./mods/graveyard-shift/playtests/watch.sh attempt-07-detailed -width=2560 -height=1440
```

The same recorded inputs are used at every rendering resolution.

The script selects the recording’s matching pinned `graveyard-shift-playtest.wad` and bundled
`DOOM1.WAD`, so future map edits do not silently change the recording's content.
`wad-sha256.txt` identifies the exact add-on used. The base IWAD is not copied
into this folder. Original DOOM restricts add-ons with a shareware IWAD; the
Chocolate reference replay used a private temporary combined IWAD, which is not
distributed here.

## Attempts

| Attempt | Outcome | Recording profile |
| --- | --- | --- |
| 01 | Died at 0:22.8 with 12 kills | Experimental, auto weapon switch disabled |
| 02 | Completed with all kills and secrets; continuation includes the whole input stream | Experimental, auto weapon switch disabled |
| 03 | Died at 0:38.57 with 17 kills, without secrets | Standard classic settings; normal demo playback verified |
| 04 | Completed at 3:00.11 with all kills and secrets | Standard classic settings; normal playback and Chocolate Doom verified |
| 07 | Detailed map completed at 2:38.40 with all kills and secrets | Weighted turns and running; normal playback and Chocolate Doom verified |

The early experimental demos use GD-DOOM's v1.10 header and require their saved
`profile.json` to reproduce the weapon-switch setting. Standard client playback
assumes automatic weapon switching and will differ. They are retained as raw
experiments, not the recommended recordings to watch. Attempt 02's
`attempt-02-direct-shotgun` folder is the initial segment; `attempt-02-continuation`
contains the full run from spawn.

## What the controls actually did

The agent issued numeric commands for movement, turning, firing, using doors,
and weapon selection. The waypoint controller computes ordinary forward/strafe
inputs from position and momentum; the aiming helper uses line-of-sight actor
telemetry. Every executed tic is stored in a regular DOOM `.lmp` input stream.
There were no teleports, noclip, invulnerability, health grants, monster removal,
or inventory/key injection. Physics, reloads, enemy AI, damage, pickups and
specials ran normally. This is tool-assisted testing, not a human skill benchmark.

Each checkpoint retains a real rendered PNG, numeric state JSON and the current
demo. `commands.jsonl` contains the higher-level control requests. The screenshots
include the world and HUD; this diagnostic harness does not load the held-weapon
sprite bank, so the gun itself is absent from these PNGs. Normal demo playback
in GD-DOOM displays the weapon normally.

## Earlier layout review

`cleared-map-contact-sheet.png` groups four checkpoints from attempt 04, all taken
after the remaining-monster count reached zero. The full run also includes a
walk back through the yard, booth, stairs, balcony, entrance and courtyard after
clearing the level.

Findings from the earlier pinned layout (attempts 01–04):

1. **The first shot wakes a large pursuit.** Shooting in the plaza can pull enemies
   into the security office. The pistol-first attempt died, while a direct route
   to the shotgun was much safer. Consider a stronger visual cue toward security
   and less simultaneous early pursuit if the opener should be forgiving.
2. **The staff corridor punishes rushing.** Charging into imps and the pinky before
   the shotgun reloads cost substantial health. Clearing the approach before
   advancing worked better; keep adequate maneuvering room around that encounter.
3. **The right-hand lamps can snag movement.** The northern lamp near (1056,1248)
   stopped a pass along the right edge; the southern one near (1056,288) also
   interrupted a straight crossing. The detailed revision removes these floor lamps.
4. **The power-switch ambush is easy to leave behind.** The two original staff
   guards remained in their room after the balcony return, requiring a cleanup
   trip. In attempt 04, a wandering zombie also joined them. Consider sending this
   group into the player's return route for a clearer payoff.
5. **The carousel provides useful cover.** Moving around it worked; standing in
   crossfire cost health. The cleared shots show the canopy and chasing lights,
   although the central column needed a more recognizable ride silhouette. The detailed
   revision replaces it with a round platform, striped canopy and wooden horses.

Each recording uses a pinned WAD. The detailed revision is archived separately,
so the earlier observations and demos still refer to exactly the layout tested.

## Detailed cleared-map views

`detailed-showcase/attempt-07-detailed/084-cleared-carousel-composition.png` shows
the courtyard with all monsters defeated. Checkpoint 079 views the horses from
the deck, and 080 looks back across the ride. The tour confirms that the round
platform is accessible with ordinary movement and its 16-unit steps.

## Reproduce the checks

```bash
GD_GRAVEYARD_VERIFY="$PWD/mods/graveyard-shift/playtests/detailed-showcase" \
  go test -tags=integration ./internal/doomruntime \
  -run '^TestGraveyardShiftRecordedSessionReplay$' -count=1 -v
```

`verification/gddoom-replay.txt` contains the exact replay checks. The legacy
profiles are explicitly applied for experimental recordings. The classic
recordings use the normal demo playback policy.
`verification/chocolate-doom-stats.txt` is the independent reference engine's
level-completion report. `session-summary.json` contains outcomes and demo hashes.

To run another numeric session, build the integration-test executable and launch
`TestGraveyardShiftInteractivePlaytest` with `GD_GRAVEYARD_PLAYTEST` set to a new
output folder, a unique `GD_GRAVEYARD_ATTEMPT` name and stdin attached. Each JSON
line can contain `ticks`, `forward`, `side`, `turn`, `buttons`, `angle`, `go: [x,y]`,
`fight`, `target`, `run`, `label`, and `quit`. `go` and `fight` are tool assistance; their
outputs are encoded as regular demo tics. Use a virtual display for PNG capture.
