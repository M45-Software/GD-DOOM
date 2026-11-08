# Graveyard Shift

**Map 1 — Admission Is Mandatory** is a playable first version of an original
carnival episode for GD-DOOM. Spooky attractions, cheerful stripes, and UAC
corporate signage surround a ticket plaza, staff service yard, raised control
booth, balcony shortcut, and carousel courtyard finale. Chasing marquee bulbs and rippling fountain water
use classic DOOM texture animations.

## Play

The included WAD targets **GD-DOOM with this repository's bundled shareware
`DOOM1.WAD`**. Monsters, weapons, sprites and sounds come from that IWAD; the map,
carnival textures, sky, signs, title art and music are original. Freedoom is not
required. No Boom features, scripts, jumping, or engine changes are used. The entrance
marquee uses the three-frame `GSTFONT` animation, kiosk/carousel trim uses
`SLADRIP`, and the harmless fountain uses the four-frame `FWATER` animation.
All advance at the classic eight-tic rate; no `ANIMATED` lump is required.

From the repository root:

```bash
./mods/graveyard-shift/play.sh
# Or:
go run . -wad DOOM1.WAD -file mods/graveyard-shift/graveyard-shift.wad -map E1M1
# Ultra-Violence:
./mods/graveyard-shift/play.sh -skill=4
```

In the browser build, load `DOOM1.WAD` as the base and `graveyard-shift.wad` as
an add-on, then start Episode 1. The map replaces **E1M1**.

This is the first map, not the completed episode. The normal exit currently
continues to the base IWAD's E1M2. Combat balance and first-visit completion time
still need human playtesting.

The geometry, nodes, actions, patches, flats and MUS score use classic DOOM
formats. The original DOOM executables restrict add-ons with the shareware IWAD;
GD-DOOM supports this combination. For another IWAD, rebuild the texture tables
against that IWAD with `--iwad`; the distributed WAD targets the bundled one.

## Files to explore

- `graveyard-shift.wad` — ready-to-load map, graphics and music.
- `assets/textures/*.png` — original PNGs for each wall, sign, sky and floor.
- `assets/title.png` — title-screen artwork.
- `assets/admission-jingle.json` — editable lyrics, syllables, notes and chord progression.
- `assets/admission-jingle-lyrics.txt` — the imaginary sung words.
- `assets/admission-jingle-score.txt` — syllable-by-syllable melody and rhythm guide.
- `previews/admission-jingle.wav` — volume-normalized OPL preview using the shareware instrument bank.
- `previews/admission-metal.wav` — the current distorted-guitar, bass and drum arrangement.
- `assets/admission-waltz.mus` — current lyric-led instrumental score (historical filename).
- `previews/map-overview.png` — annotated layout; reveals secrets and placements.
- `previews/animated-textures.gif` — looping preview of the chasing bulbs and water.
- `previews/texture-atlas.png` — contact sheet of the texture PNGs.
- `previews/01-arrival.png` through `11-balcony-to-booth.png` — GD-DOOM renders, including stair/sky checks.
- `map-manifest.json` — difficulty counts, sector tags, action lines and landmarks.
- `build.py` — deterministic geometry, BSP, blockmap, art and music source.

Signs leave 19 pixels of horizontal padding on each side of a 128-pixel panel,
keeping lettering clear of the decorative border and bulbs.

## The park jingle

“Come and Join Our Happy Crowd” is an instrumental melody written to eight
seven-syllable lyric lines. Each line takes two bars in 3/4 at 112 BPM. The
distorted lead follows each imaginary syllable and leaves a breath after each
rhyme, resolving to A minor. Short power-chord chugs, picked bass, kick, snare
and cymbals give the 26-second loop a metal pulse; a quiet calliope echoes the rhymes. The words are a composition guide; no vocals are embedded.

Edit `assets/admission-jingle.json` to revise the words, pitches or chords, then
rebuild. `--keep-art` retains the texture PNGs while regenerating the music.
An already-running game retains its loaded music; relaunch/reload the add-on to
hear the rebuilt score.

Outdoor skies share a 320-unit plane so elevated views cannot cut across the
control booth or stairwell. Exterior signs use the existing walls’ lower textures at eye level, with normal
sidedef offsets and no inset overlays. Closed sectors behind those boundary walls
split the upper background from the sign on one wall plane. Wall materials share
a vertical origin, while stair risers keep visible edge trim.

## Route and balance

Explore the ticket plaza, take the security shotgun, cross the staff yard, climb
to the control booth, collect the blue pass, and restore admission power. Drop
from the balcony into the plaza and open the blue admission gate. Entering the
welcome courtyard opens two maintenance closets around the carousel. The final
switch leads toward the midway.

| Difficulty | Enemies |
| --- | ---: |
| Easy (skills 1–2) | 20 |
| Hurt Me Plenty | 36 |
| Ultra-Violence / Nightmare | 52 |

Three optional secrets reward exploration with armor, supplies, and a chaingun.
The main route provides a guaranteed shotgun and ammunition without secrets.
Four co-op starts and four deathmatch starts are placed; multiplayer balance has
not been playtested.

## Episode pacing

The episode has **four maps**. Map 1 is cute, short and sweet: a welcoming ticket
plaza, a glimpse behind the cheerful facade, and one open circular carousel with
wooden horses and monsters beneath its striped canopy. Its existing route and
enemy counts stay small; added detail improves composition rather than length.

Each later map introduces a different part of the park and grows in size and
playtime. These are design targets, not measured times; Maps 2–4 are not built yet.

| Map | Theme and reserved attractions | First-visit target |
| --- | --- | --- |
| 1 — Admission Is Mandatory | Tickets, fountain, carousel; “Your fun is mandatory.” | 3–5 minutes |
| 2 — Midway Massacre | Shooting gallery, rigged prize booths, bumper cars; impossible performance quotas | 6–9 minutes |
| 3 — Tunnel of Loathe | Ghost train, funhouse, tunnel ride; a compulsory corporate team-building excursion | 10–14 minutes |
| 4 — The Happiest Place in Hell | Roller coaster, Ferris wheel, backstage power plant and executive finale | 15–20 minutes |

Save those later attractions for their own reveals. Map 1 should establish the
park’s friendly colors, eerie after-hours atmosphere and petty corporate cruelty
without showing everything the episode has to offer.

## Recorded playtests

See [playtests/README.md](playtests/README.md) for the successful classic demo,
failed attempts, cleared-map screenshots and observed layout/balance issues.
The current recording completes the detailed map in 2:38.40 with all 36 kills and all three secrets,
and was verified in GD-DOOM and Chocolate Doom. The current run includes weighted turns, running and a cleared-carousel tour.
The runs use numeric movement and aiming assistance; their times do not measure human difficulty.

## Rebuild

Requires Python 3, Pillow, DejaVu Sans Bold and ZDBSP (normal classic nodes).
Install `zdbsp` with your system package manager, or pass `--node-builder /path/to/zdbsp`.
Run from anywhere:

```bash
python3 /path/to/GD-DOOM/mods/graveyard-shift/build.py
```

To retain hand-edited texture PNGs while rebuilding the WAD:

```bash
python3 /path/to/GD-DOOM/mods/graveyard-shift/build.py --keep-art
```

To target a different base IWAD, supply `--iwad /path/to/base.wad`. This replaces
the generated WAD in this folder; rebuild with the default to return to shareware.
The builder includes the base IWAD's texture definitions and patch-name metadata,
without bundling its graphic, sound or sprite data.

## Validation

The integration tests walk the main route with collision, gravity, item pickups,
use traces, the blue-key lock and the ambush triggers on skills 1, 3 and 4. They
also check all three secret-door actions, parse the original MUS score, and verify
the classic animation ranges, distinct frames, palette-safe blue water, runtime
frame selection, continuous door/marquee texture alignment, and high outdoor sky planes.
Monsters are disabled for route tests; these checks do not establish combat balance.

```bash
go test -tags=integration ./internal/doomruntime \
  -run 'TestGraveyardShift(Progression|SecretDoors|Animations|TextureAlignment)$' -count=1

GD_GRAVEYARD_CAPTURE="$PWD/mods/graveyard-shift/previews" \
  go test -tags=integration ./internal/doomruntime \
  -run TestGraveyardShiftRender -count=1

go run ./cmd/mapaudit mods/graveyard-shift/graveyard-shift.wad
```

The rendering test needs a display and should run alone. Layout validation also
checks thing clearances and route connectivity when rebuilding. Six main camera
positions were also rendered for 140 tics each in Chocolate Doom 3.0.0 using a
private temporary combination of the bundled shareware IWAD and this add-on.
Those reference checks passed without renderer-limit failures. The current
5,544-tic showcase also replayed completely with Chocolate Doom rendering enabled,
with 36/36 kills and 3/3 secrets in its independent statistics report. This verifies
that recorded route and camera path; it does not establish every possible view
or human combat balance.

The original map, art, music and generator follow the repository's GPL-2.0
license. The base game is not included in this mod folder.
