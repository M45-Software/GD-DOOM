# Boom map support

GD-DOOM implements an automatic subset of Boom map features. Load a
classic-format PWAD with `-file`; no compatibility flag is required. The
recorded paths below are verified; complete Boom compatibility is not claimed.

## Implemented

- Boom weapon commands and ammo-exhaustion timing: fallback selection belongs
  to live command generation; playback honors recorded weapon choices exactly,
  including direct fist/shotgun/SSG codes. Default fallback thresholds and
  lowering/raising/refire flags follow DSDA. Vanilla demos keep their immediate
  simulation-side fallback selection. BFG flashes retain their original phase.
- Boom 2.01/2.02 demo game options: seeded random streams, demo-insurance
  class separation/tic shuffle, monster memory, friction/pusher gates, weapon
  recoil and player bobbing, plus respawn/fast/no-monsters settings. Vanilla
  demos keep their original table RNG. Full seeded state and options survive
  saves/keyframes; demo and authority map transitions continue their streams.

- Boom corpse movement: after the death animation clears solidity, a sliding
  corpse can pass through solid players, monsters and decorations. Walls still
  block it. Early solid death frames and vanilla demos keep actor blocking.
- Boom Pain Elementals: no vanilla 21-skull population cap; the PE-to-skull
  path rejects one-sided, blocking and monster-blocking lines before allocating
  a skull or consuming RNG. Spawned skulls outside their sector's vertical
  bounds are killed in place. Vanilla demos retain the original spawn rules.

- Boom variable friction (223, sector bit 256): controller-derived mud/ice
  friction, inertia, speed-dependent mud acceleration, overlapping-floor
  selection, one-tic friction thinker state and icy wall bounces. Normal maps
  keep their original movement path; vanilla demos do not enable Boom friction.
- Wind (224), currents (225) and point push/pull sources (226, things 5001/5002),
  gated by sector bit 512. Forces add in thinker order, follow real ground and
  height-transfer water boundaries, and honor noclip. Point forces use Boom's
  approximate distance/range, cached source position, last-spawned source and
  visibility checks, including height-sector separation. Under Boom rules
  these affect player bodies and voodoo dolls; ordinary monsters are immune.
  MBF's broader actor eligibility and inverse-square point forces are separate.

- Custom `ANIMATED` wall/flat ranges and per-range tic speeds; custom `SWITCHES`
  pairs, episode filtering and table-order selection. Supplied lumps replace
  defaults, including terminator-only tables. Repeat buttons restore one slot
  after 35 tics; pressing an active button does not extend its timer. Resources
  load in the normal, native, headless demo and dedicated-server paths.

- Boom translucency (260): tag-zero local application, tagged transfers,
  named 65,536-byte filters, supplied `TRANMAP` overrides and the generated
  default 66% source filter. Source lighting is applied before table lookup.
  Masked holes stay clear; actors and opaque/translucent walls paint in depth
  order, including crossing walls. CPU and GPU use the same palette lookup.
  RGB shading in Source Port is quantized for this indexed effect. The setup
  references rebuild on load; no additional simulation/save change is needed.
- Height transfers (242): camera-dependent fake floors/ceilings, deep-water
  plane textures, independent model plane lights, model scrolling offsets and
  sky exceptions. The inclusive waterline and one-16.16-unit plane separation
  follow DSDA's `R_FakeFlat`. Moving control heights update every frame;
  collision, actor support heights and saved physical geometry stay real.
- Height-sector sprites clip at the fake floor/ceiling, including partially
  submerged actors and actors completely separated from the camera. Normal
  play uses the average fake floor/ceiling light with classic sprite light
  rows; Boom 201/202 demos retain their original sector-light behavior.
- Named `C_START`/`C_END` colormaps on 242 control sidedefs select bottom,
  middle and top view regions. Selection uses strict height comparisons,
  distinct from the inclusive fake-floor geometry rule. World surfaces,
  sprites, skies and weapons use the active table; CPU/GPU and weapon caches
  update when it changes. Incomplete tables fall back to normal rendering.
- Per-sector sky transfers (271/272): upper sidedef texture, Doom BAM horizontal
  offsets, vertical offsets, optional flipping, sky floors and several skies
  visible together. Tall skies retain their declared height. CPU/GPU share
  per-sky coordinate lookups; Source Port honors its lighting settings.
- Floor/ceiling light transfers (213/261): planes read the control sector's
  current light, including glow/flash effects, while walls retain their own
  sector lighting. Map references are rebuilt on load; restored light caches
  are refreshed. These static features do not change simulation/save formats.

- XNOD and zlib-compressed ZNOD data in the `NODES` lump, including fractional
  16.16 split vertices, 32-bit seg/subsector indices and BSP child references.
  Classic nodes use the same widened traversal. Malformed counts, references,
  cyclic trees and oversized decompression are rejected.
- Silent thing teleports (207–208), switch-activated thing exits (209–210),
  line-to-line teleports (243–244),
  reversed line exits (262–263), monster-only reversed/normal line exits
  (264–267), and monster-only thing exits (268–269). They preserve relative
  height and vertical momentum, rotate facing/horizontal momentum, and retain
  reaction time without teleport fog or teleport sounds. Switch variants retain
  their normal switch animation. Line exits preserve fractional position,
  select the appropriate side with Boom's fixed-point nudges, and measure Z
  above the higher exit floor. First eligible exit order and blocked-exit
  rejection follow DSDA. Normal teleport freeze timing follows `P_PlayerThink`.
- Generalized ceilings (`0x4000`–`0x5fff`): all eight triggers, four speeds,
  eight targets, crushing and trigger/numeric model texture/type changes.
  Changes apply when the mover finishes and survive saves and replication.
- Generalized lifts (`0x3400`–`0x37ff`): all triggers, speeds, waits and targets,
  including perpetual motion, stop/resume and independent floor/ceiling slots.
  Manual lifts retain the activating line's tag when stopped and restored.
- All generalized locked doors (`0x3800`–`0x3bff`): separate cards/skulls,
  either variant of a color, any key, all three colors, and all six keys.
  All eight triggers, four speeds and raise/open variants are supported.
  Generalized doors use an independent ceiling slot.
- Remaining legacy Boom movers: elevators 227–238, lower-to-nearest floors
  219–222, ceiling targets 199–206, texture/type-only trigger and numeric
  changes (78, 153–154, 189–190, 239–241), and instant toggles 211–212.
  Legacy donut, silent-crusher, combined-mover and repeatable-stair variants
  are also decoded. Elevators reserve both plane slots; ordinary Boom floor
  and ceiling movers can run independently, preserving vanilla demo gating.
- Generalized stairs (`0x3000`–`0x33ff`): four speeds and step sizes, both
  directions, texture filtering, monster permissions, all eight triggers,
  ordered chain selection, whole-chain retrigger locks and alternating direction.
- Generalized crushers (`0x2f80`–`0x2fff`): four speeds, monster permissions,
  normal/silent variants, slowdown/recovery while crushing, and tagged stop/resume.
  Boom complevel 9 intentionally leaves walk-over variants inactive, matching
  DSDA's preservation of the original missing walk-dispatch branch.
- Boom manual-door/mover interactions: repeatable legacy manual doors use the
  ceiling slot independently of floor-only movers; generalized manual doors
  reject an occupied ceiling slot. One-shot legacy doors retain displaced
  thinkers and reproduce actual slot replacement/clearing rather than deleting
  the older controller. Overlapping controllers survive saves and replication.
- Moving-sector sounds: eight-tic movement-loop refreshes, floor/elevator
  completion sounds, platform start/stop/reversal sounds, legacy silent-crusher
  endpoint sounds and fully silent generalized crushers/toggle platforms. Sector
  identity and bounding-box sound origins survive sound-event replication.
  Generalized doors select their opening/closing sound using their final speed,
  destination and retrigger/obstruction path.
- Voodoo dolls slide using their own collision shape while collision checks
  retain the real player's position.

- Generalized floors (`0x6000`–`0x7fff`): all eight trigger modes, four speeds,
  up/down targets, crushing, and trigger/numeric model texture/type changes.
  Generalized floors use the floor mover slot independently of ceiling motion.
- Generalized doors (`0x3c00`–`0x3fff`): triggers, speeds, waits and
  open/close variants used by the recorded paths below. Wait selections use
  exactly 35, 150, 300 and 1050 tics.
- Manual door lighting: regular and generalized push doors, including locked
  doors, use their nonzero line tag to change tagged sectors' light levels
  when opening or closing finishes. Boom 2.02 endpoint timing, current neighbor
  levels, ordered updates and blocked final snaps are preserved. Switched and
  other tag-operated doors do not gain this effect; vanilla demos retain their
  original behavior. MBF's gradual lighting is not implemented.
- Static ceiling/floor scrollers (250–251), carry-only conveyors (252), and
  combined floor scrolling/carrying (253). Carry impulses participate in the
  ordered thinker loop; CPU/GPU plane sampling includes the texture offsets.
- Displacement scrollers (245–249) and accelerative scrollers (214–218):
  controller floor-plus-ceiling displacement, persistent acceleration, and
  additive floor/ceiling/wall/carry effects. Wall scrollers (254/255) include
  rotation into the target wall's coordinates, sidedef-derived rates and both
  texture axes. Only the target's first side scrolls. Scrolling sky-control
  sidedefs feed the transferred sky's offsets too.
  Mutable controllers run in thinker order and preserve velocity, previous
  control height and phase across saves and authority snapshots. Arithmetic
  uses Boom's signed 16.16 wrapping. Underwater carry also reaches floating
  actors and missiles, with strict water-height and noclip checks.
- Single-player voodoo dolls: earlier matching starts become physical bodies
  with collision, momentum, player-only walk triggers, pickups and shared
  damage. Their position, health and momentum use existing save fields.
  Multiplayer voodoo ownership and full combat/death fidelity remain unverified.
- Single-player Boom 2.01/2.02 demo headers (201/202). Recording remains in the
  existing vanilla format. Parsing a header does not establish compatibility
  with all options or Boom combat RNG.

- Extended triggers for existing movers and lights: 142–145, 147–149, 152,
  156–164, 167–184, 187–188, and 192–196 (42 additional triggers).
  The exact action table is in
  [boom.go](../internal/mapdata/boom.go). Shoot exits 197–198 were already present.
- Extended W1 triggers clear only after successful activation, so a busy
  sector does not permanently consume them. Vanilla W1 behavior is retained.
- Generalized sector damage (bits 5–6), secrets (bit 7), and low-bit lighting
  and timed doors. Timed door setup consumes the sector special, as in Boom.
  Finding a secret preserves other active flags. Low type 4
  installs both its strobe and 20-point hazard in extended sectors.
- Wall scrolling to the right (85), alongside existing left scrolling (48).
- Unsigned sidedef indices through 65534, with 65535 reserved for a missing
  side. Physics, rendering and binary saves retain these larger indices.
- The last matching player start is used, so maps with duplicate starts do
  not put the player in an earlier voodoo-doll control closet.
- Existing game-mode thing flags (not single-player, not deathmatch, not co-op).

The implementation follows the [Boom editing reference](https://boomref.soulsphere.org/)
and the original extended action dispatch behavior preserved in
[MBF p_spec.c](https://github.com/fragglet/mbf/blob/master/p_spec.c) and
[p_switch.c](https://github.com/fragglet/mbf/blob/master/p_switch.c).

## Try the included map

The original asset-free `testdata/boom/simple.wad` is a single room using Doom II
textures. It has a glowing secret sector with 5-point periodic damage, a
right-scrolling west wall, a north-wall switch (161) that raises the whole room
24 units, and an east-wall exit switch that must be shot (197).

```sh
go run . -wad wads/DOOM2.WAD -file testdata/boom/simple.wad -map MAP01
```

Regenerate its deterministic WAD with `python3 scripts/make_boom_smoke_wad.py`.
The runtime smoke test loads the real WAD, initializes lighting and secret
counts, applies damage, activates the floor, restores its saved state, checks
that the floor carries the player, and triggers the gun exit. These checks do
not replace visual or interactive playtesting.

Audit linedef and sector coverage for individual WADs:

```sh
go run ./cmd/mapaudit testdata/boom/simple.wad
go run ./cmd/mapaudit path/to/addon.wad
```

## Okuplok load check

The local `oku2v31.wad` from `oku2v31.zip` has classic BSP nodes and loads after
correcting unsigned sidedef indices and player-start selection. It contains
38,295 sidedefs, exceeding the old signed 16-bit index range. Its map-data audit
now reports zero unknown linedef or sector specials, after adding its 509
generalized floors, one generalized door and one conveyor. This does not
certify its complete encounter sequence or combat performance.

The checked WAD has SHA-256
`c7dfcdbbaf33b5f7b13c8405a0b36c562c3c7d9e7f63ad48743c999dccb85061`.
This is a startup/render check; progression and large-fight performance are
not established by it.

## Demo verification

`scripts/verify_boom_demos.py` downloads hash-pinned WAD/demo archives, runs
GD-DOOM and DSDA-Doom, and compares every exported reference player position
(X/Y/Z) and angle exactly, plus the level-exit tic. Reference ghost exports
start at completed gametic 1; GD labels that completed command gametic 0.
All five cases are single-player **no-monsters** demos.

| Map | Demo | Reference frames matched | Level time | Demo format |
| --- | --- | ---: | ---: | --- |
| Ancient Aliens MAP02 | `aa02o026.lmp` | 932 / 932 | 26.66 s | Boom 2.02 / complevel 9 |
| Ancient Aliens MAP01 | `aa01o023.lmp` | 835 / 835 | 23.89 s | Boom 2.02 / complevel 9 |
| Sunlust MAP01 | `sl01o016.lmp` | 584 / 584 | 16.71 s | Boom 2.02 / complevel 9 |
| Sunlust MAP02 | `sunlust02nomo48.lmp` | 1,712 / 1,712 | 48.94 s | Boom 2.02 / complevel 9 |
| Sunder MAP01 | `su01o048.lmp` | 1,709 / 1,709 | 48.86 s | Doom 1.9 / complevel 2 |

Ancient Aliens MAP02 exercises a generalized lift and a generalized blue-key
door releasing a voodoo-controlled floor. It exits at leveltime 933.
Sunlust MAP02 contains nine extra player starts, five conveyor lines and
15 generalized floor lines. Its full recorded path stays in sync with the
reference. Sunder is a separate map-loading/movement regression; that demo
does not exercise Boom-specific commands. The reference build was DSDA-Doom
0.27.5. Downloads come from [DSDA](https://dsdarchive.com/); the script pins
archive hashes and writes content/binary hashes, logs, traces, ghosts and
`results.json` under the output directory. WADs and demos are not bundled.

```sh
python3 scripts/verify_boom_demos.py --iwad wads/DOOM2.WAD \
  --reference /path/to/dsda-doom --out /tmp/boom-demo-verification
```

The script requires Go, `xvfb-run`, and DSDA-Doom 0.27.x. Use `--engine` to
reuse a built GD-DOOM binary, or `--case sl02o048` to replay one case. Any
coordinate/angle mismatch, missing frame or exit mismatch fails the run.

The focused tests also cover parameter decoding, floor targets/speeds/model
changes, generalized ceiling targets/crushing/model changes, lift waits and
stasis tags, locked-door key gates, conveyor thinker timing, airborne actors,
a voodoo-triggered floor,
shared damage and binary save restoration. The opt-in renderer test compares
scrolling planes on CPU/GPU in Faithful and Source Port modes:

```sh
xvfb-run -a go test -tags integration ./internal/doomruntime \
  -run '^TestBoomConveyorPlaneRendering$' -count=1
```

Ancient Aliens MAP01 also rendered at six demo viewpoints (tics 1, 80, 130,
300, 500 and 800), at 640×400 with Doom lighting. CPU/GPU output matched pixel
for pixel. These captures verify GD renderer agreement, not DSDA image parity.
A temporary copy of MAP01 with its XNOD payload zlib-compressed into ZNOD
also matched all 835 reference frames and exited at tic 836.
The [XNOD/ZNOD layout](https://github.com/doom-cross-port-collab/udmf/blob/main/formats/bsp.md)
and [DSDA loader](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/p_setup.c)
provide the format reference.
Ancient Aliens MAP02 additionally rendered at demo tics 90, 110, 500, 550 and
800 at 640×400 with Doom lighting; each CPU/GPU pair matched pixel for pixel.
The generalized mover behavior follows [DSDA p_genlin.c](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/p_genlin.c).
Decoder tests cover both encodings, fractional
vertices, indices above 65535, truncated payloads, bad counts and cyclic nodes.

## Sky/light screenshot verification

`testdata/boom/transfers.wad` contains original room geometry and a uniform
flat for isolating transferred light from flat UV sampling. Regenerate it with
`python3 scripts/make_boom_transfer_wad.py`. Its ten fixed views cover floor and
ceiling lights, 271/272 flipping, both offsets, multiple skies and sky floors.
Two additional fixed views use the hash-pinned Ancient Aliens WAD above.

The capture harness builds a private copy of DSDA-Doom **0.27.5**, adding only
camera setup, actor/HUD suppression, raw PLAYPAL export and camera logging.
The software BSP/plane/column renderers are unchanged. Camera X/Y/eye Z,
angle, leveltime and output dimensions must match before comparison succeeds.
Only fixed cameras are accepted by this harness; it does not replay demos.

```sh
python3 scripts/verify_boom_transfers.py --iwad wads/DOOM2.WAD \
  --pwad testdata/boom/transfers.wad \
  --scenes testdata/boom/transfer-scenes.json \
  --reference-source /path/to/clean/dsda-doom-0.27.5 \
  --out /tmp/boom-transfer-fixtures

python3 scripts/verify_boom_transfers.py --iwad wads/DOOM2.WAD \
  --pwad /path/to/aaliens_v1_2.wad \
  --scenes testdata/boom/ancient-aliens-transfer-scenes.json \
  --reference /tmp/boom-transfer-fixtures/reference-build/dsda-doom \
  --out /tmp/boom-transfer-aaliens
```

Building the reference requires CMake, a C/C++ compiler, SDL2/SDL2_mixer, zlib,
libzip and OpenGL development files. Optional music/image libraries are disabled.
`--cmake-arg=-DNAME=value` can select a local dependency prefix. Capturing also
requires Go, Pillow and `xvfb-run`. WADs and reference binaries remain local.

At 320×200, all **15 explicitly checked transfer regions** matched DSDA pixel
for pixel across these 12 views. Every full CPU/GPU pair also matched exactly.
The ten fixture views additionally had identical CPU/GPU output in Source Port
mode with sector lighting enabled. Whole-frame DSDA differences remain:
fixture views differed at about 1.01–2.10% of pixels; the two Ancient Aliens
views differed at 22.99–24.26%, with mean absolute channel error 1.72–1.73/255.
The residual images include wall sampling and boundary differences; these checks
establish the selected transfer effects, not full-frame DSDA raster parity.

Each output directory includes `index.html`, three-way comparison images,
amplified difference images, exact camera logs, source/content/binary hashes and
`results.json`. The manifest's explicit regions use zero differing pixels as
the pass criterion. Whole-frame differences are reported separately.
Implementation references are [DSDA sky planes](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/r_plane.c)
and [transfer setup](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/p_spec.c).

## Height-transfer screenshot verification

Regenerate the original two-room fixture with
`python3 scripts/make_boom_water_wad.py`. MAP01 has a physical floor at -64,
a rendered water surface at 0, and a model ceiling at 128. MAP02/03 exercise
the model floor/ceiling sky exceptions. Distinct uniform flats isolate plane
visibility and lighting from texture UV sampling.

```sh
python3 scripts/verify_boom_transfers.py --iwad wads/DOOM2.WAD \
  --pwad testdata/boom/water.wad \
  --scenes testdata/boom/water-scenes.json \
  --reference-source /path/to/clean/dsda-doom-0.27.5 \
  --out /tmp/boom-water-verification
```

Nine fixed 320×200 views cover the shore across the surface, above/below water,
exactly at the waterline, 1/8 unit to either side, above the fake ceiling, and
both sky exceptions. All **18 explicit plane-interior regions** match DSDA
pixel for pixel on CPU and GPU. Every full CPU/GPU pair matches. These regions
avoid wall boundaries: the lowest land-facing wall edge can differ by one row.
Whole-frame DSDA differences range from 0.50–25.75%, including existing wall
sampling differences that become prominent when facing the shore underwater.
The gallery and difference images retain those differences; the checks establish
fake-plane geometry/material/light selection, not complete raster parity.

The reference probe invalidates the fresh map's wall-flag cache by setting
`gametic=1` without advancing simulation. Otherwise its zero-initialized cache
can skip DSDA's first wall classification. Logged leveltime and camera still
match exactly. The existing 12 sky/light views and 15 transfer regions also
pass with this corrected reference setup.

Focused tests cover exact 16.16 waterline transitions, front/back substitutions,
moving control heights, independent live plane lighting, scrolling offsets,
sky visibility, physical movement and binary save restoration. All five
existing no-monsters demo regressions still match DSDA for **5,772 frames**,
including their exit tics. This render-only addition needs no simulation or
save-version change.

The implementation follows [DSDA's fake-flat rendering](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/r_bsp.c)
and [height-sector plane marking](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/r_segs.c).
These original fixed-camera comparisons suppress actors and HUD and cover
world geometry. The additional actor comparisons below enable map sprites.

### Actors and underwater colormaps

`python3 scripts/make_boom_water_actors_wad.py` builds a second original fixture
with uniform indexed sprites: a pillar crossing both model planes and a barrel
entirely below water. It includes three distinct custom light tables and a sky
variant. Uniform sprite colors isolate clipping and light selection from patch
UV rounding. No IWAD artwork is stored in this fixture.

```sh
python3 scripts/verify_boom_transfers.py --iwad wads/DOOM2.WAD \
  --pwad testdata/boom/water-actors.wad \
  --scenes testdata/boom/water-actors-scenes.json --actors --complevel 11 \
  --reference-source /path/to/clean/dsda-doom-0.27.5 \
  --out /tmp/boom-water-actors
```

All **44 checked regions across 19 views** match DSDA pixel for pixel, including
floor/ceiling sprite cuts, the submerged barrel, exact surface/ceiling equality,
all three colormap zones, returning to land, and tinted sky. Every full CPU/GPU pair also matches,
including all 19 views in Source Port mode. Whole-frame DSDA differences remain
0.13–2.89% from wall sampling and raster boundaries; these are reported separately.
The original 21 water/sky/light views and 33 regions still pass, as do the five
demo regressions (5,772 frames and matching exits).

These actor captures explicitly use **MBF compatibility 11**: DSDA deliberately
uses the real sector light for sprites at Boom compatibility 9, retaining an
old Boom lighting bug. Compatibility 11 uses the fake plane-light average,
which GD uses for normal map play. Boom 201/202 demo playback preserves the
older light selection; focused tests cover that distinction. The renderer and
clipping code in the reference remain unchanged. `--actors` keeps map actors
in both engines while continuing to suppress the weapon and HUD.

Focused tests also cover one-fixed-unit transitions, complete sprite rejection,
live plane-light averages, namespace/malformed-table handling, returning to the
default colormap, GPU table invalidation and weapon cache identity. This remains
a presentation change with no new simulation or save format.

## Scroller verification

`python3 scripts/make_boom_scroller_wad.py` regenerates twelve original test
maps, one per remaining scroller special, and their fixed-camera manifest.
An autonomous 30-second door changes a disconnected control sector's ceiling.
Views before, during and after that motion distinguish displacement from
acceleration that continues after the control stops. The wall cases include
a slanted target, horizontal/vertical scrolling and a camera facing the 255
scroller. Patterned flats and an original wall patch expose texture phase.

```sh
python3 scripts/verify_boom_transfers.py --iwad wads/DOOM2.WAD \
  --pwad testdata/boom/scrollers.wad \
  --scenes testdata/boom/scroller-scenes.json --simulate \
  --reference-source /path/to/clean/dsda-doom-0.27.5 \
  --out /tmp/boom-scrollers
```

`--simulate` advances actual stationary-player world tics in both engines,
then selects the fixed rendering camera. Before drawing, it compares exact
16.16 player position/momentum, all sector heights and plane offsets, and all
sidedef offsets. The reference runs its unchanged `P_Ticker`; neither the
controller nor scroller renderer is patched. Without this flag the original
fixed-camera capture behavior remains available.

All **56 fixture views** match this numeric state exactly. All 56 wall check
regions are pixel-exact. Of 112 patterned-plane regions, 106 are pixel-exact
and six differ by at most 1.96% of pixels at UV boundaries, within their
explicit 2% limit. Whole fixture frames differ from DSDA by 1.02–2.25%; their
diagnostic differences remain in the gallery. Every complete CPU/GPU pair
matches, including all 56 views in Source Port mode.

Two additional simulated Ancient Aliens MAP01 views at tics 35/70 use
`testdata/boom/ancient-aliens-scroller-scenes.json` with the hash-pinned WAD.
They also match all recorded numeric state and their sky check regions.
Whole frames retain roughly 22.98% DSDA differences, so these are not claims
of full real-map raster parity. The existing five no-monsters demo regressions
still match all **5,772 frames** and exit tics.

Focused tests cover controller motion/stopping, cancellation between floor
and ceiling changes, additive scrollers, wall rotation/target sides, fixed
wrapping, underwater floating/missile carry, sky-control offsets, and binary
save/keyframe continuation while accelerating. The behavior follows
[DSDA's compatible scroller setup](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/p_spec.c)
and [control-scroller updates](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/dsda/scroll.c).

## Translucency verification

`python3 scripts/make_boom_translucency_wad.py` builds two original PWADs and
their scene manifests. The fixtures cover two overlapping masked walls,
transparent holes, actors in front/between/behind the layers, an opaque front
wall, crossing walls, reverse views, asymmetric named filters and an overriding
global `TRANMAP`. No IWAD artwork or generated IWAD-based filter is stored
in these fixtures.

```sh
python3 scripts/verify_boom_transfers.py --iwad wads/DOOM2.WAD \
  --pwad testdata/boom/translucency.wad \
  --scenes testdata/boom/translucency-scenes.json --actors --check-tranmap \
  --reference-source /path/to/clean/dsda-doom-0.27.5 \
  --out /tmp/boom-translucency

python3 scripts/verify_boom_transfers.py --iwad wads/DOOM2.WAD \
  --pwad testdata/boom/translucency-override.wad \
  --scenes testdata/boom/translucency-override-scenes.json \
  --actors --check-tranmap \
  --reference /tmp/boom-translucency/reference-build/dsda-doom \
  --out /tmp/boom-translucency-override
```

Across **30 fixed views**, 86 of 90 checked regions match DSDA pixel for pixel.
Four regions permit one differing column (5% or 6.67% of their narrow regions)
at native raster boundaries; their limits are explicit in the manifests.
The complete CPU/GPU pairs match exactly, including the Source Port captures.
Whole-frame DSDA differences remain below 1% in these fixtures and are shown
in the diagnostic images. These are scoped comparisons, not full raster parity.

`--check-tranmap` exports the reference filter and compares **all 65,536
entries**. Both the generated default and supplied override match. The shader
tests separately exercise every destination/source pair through two asymmetric
layers using unique palette colors. Other focused tests cover tag/setup order,
shared control sides, malformed-table texture fallback, source lighting before
lookup, successive layers, rebuilt save references and ordinary-map fast paths.
Framebuffer RGB is recovered through the renderer's existing palette lookup;
the tests do not establish every duplicate-palette-index/custom-filter case.

The earlier 19 water/actor views and two Ancient Aliens transfer views still
pass their checks. All five existing no-monsters demo comparisons retain their
**5,772 matching frames and exit tics**. The reference's BSP, sprite, plane and
seg renderers are unchanged; the capture probe only adds resource export.
Setup and blending follow [DSDA 260 setup](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/p_setup.c),
[TRANMAP generation](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/dsda/tranmap.c)
and [column compositing](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/r_drawflush.inl).

### Rendering cost

Maps without 260 keep the original cutout path and do not allocate translucency
references or generate a filter. Visible translucent cutouts enable per-column
depth ordering, grouped into adjacent runs. GPU passes reuse buffers and copy
only the affected background columns; gameplay performs no framebuffer readback.
Seven distinct referenced filters fit the current GPU metadata atlas; maps
exceeding that limit use the CPU renderer.

The before/after benchmark uses commit `5305ee5`, 20 warmup frames and **120
measured frames per case**, at fixed cameras with VSync off. It times the world
draw plus synchronized `ReadPixels`, which is benchmark-only. This host uses
Mesa llvmpipe software rendering under Xvfb, so shader-backend numbers do not
predict hardware-GPU gameplay FPS. A repeat vanilla-only run measured:

| MAP01 view | Before p50/p95/p99 ms | After p50/p95/p99 ms |
| --- | --- | --- |
| CPU, 320×200 | 0.763 / 0.848 / 0.920 | 0.871 / 0.981 / 1.118 |
| Shader, 320×200 | 1.636 / 1.817 / 1.839 | 1.677 / 1.863 / 1.892 |
| CPU, 1280×800 | 6.757 / 7.610 / 8.286 | 6.651 / 7.212 / 7.326 |
| Shader, 1280×800 | 8.747 / 9.198 / 9.241 | 8.884 / 9.385 / 9.580 |

The low-resolution CPU increase is about 0.11 ms; this is not a zero-overhead
claim. The separate full run measured the 1280×800 two-layer fixture at
11.845 / 12.361 / 12.694 ms on CPU and 19.123 / 22.362 / 22.903 ms on the shader
backend (p50/p95/p99). With opaque panels, the corresponding p50 values were
5.533 and 7.382 ms. That comparison includes the extra background visibility,
ordering and lookup work required by transparent panels.

`TestBoomFrameTiming` is opt-in and portable to the previous commit for paired
runs. Set `GD_FRAME_TIMING_OUT`, `GD_FRAME_TIMING_IWAD` and
`GD_FRAME_TIMING_FIXTURE` (the capture harness's merged fixture WAD), then run
`go test -tags integration ./internal/doomruntime -run '^TestBoomFrameTiming$'`.
`GD_FRAME_TIMING_ONLY=vanilla-map01` repeats the vanilla views alone. These local
samples are limited; combat, large translucent scenes and hardware GPUs need
their own measurements.

## Custom animation and switch verification

`make_boom_tables_wad.py` builds an original twelve-map fixture with seven
animation speeds (1, 2, 3, 4, 5, 6, 8), non-first frame aliases, overlapping
ranges, duplicate switch entries, absent assets, episode filtering and replaced
Doom defaults. The 91 captures include initial identity tables, frame boundaries,
wraparound, one-shot activation, repeat presses while a button is active, the
35-tic restoration boundary and activation after restoration. Both engines
receive actual use-button commands before advancing their normal world tick.

For every capture, **46 translation aliases**, sidedef names, line specials,
sector lights, live button timers and existing player/plane/offset state match
DSDA-Doom 0.27.5 exactly. All **273 screenshot regions** match DSDA exactly;
the CPU and GPU whole frames are identical. Whole DSDA/GD image differences
remain diagnostic (up to 1.9344% here), including the existing wall/plane raster
boundaries. This does not claim full-image DSDA equality.

The comparison corrected the previous absolute-index animation phase: DSDA
indexes each range from zero. Cache epochs now follow the common divisor of
active animation speeds, retaining the normal eight-tic cache lifetime for Doom
resources. It also corrected special 138 to set full brightness, as DSDA does.
Button state reuses the existing save/keyframe fields; the custom tables rebuild
from the WAD. The compatibility ID advances because slot selection and repeat
press behavior affect simulation.

Reproduce with an owned Doom II IWAD and the private reference capture build:

```sh
python3 scripts/make_boom_tables_wad.py
GOCACHE=/tmp/gd-doom-go-cache python3 scripts/verify_boom_transfers.py \
  --iwad wads/DOOM2.WAD --pwad testdata/boom/tables.wad \
  --scenes testdata/boom/tables-scenes.json --simulate \
  --reference /path/to/capture-build/dsda-doom --out /tmp/gddoom-boom-tables
```

The capture probe resets animation translations to fresh-process identity
between independent scenes; it does not alter the reference's update or render
logic. `use_tics` is a list of zero-based world tics in the scene manifest.
`table_probes` on the first scene selects the aliases logged for all scenes.
The five existing no-monsters demos still match all **5,772 frames** and level
exit tics. The full Go suite and CPU/GPU integration regression also pass.

## Silent teleport actor verification

`make_boom_teleports_wad.py` builds an original 30-map fixture and 1,456 cases.
The matrix covers all fourteen silent specials, including the existing 207–208,
with **490 player, 532 imp, 420 cacodemon and 14 missile cases**. Source geometry
includes vertical, horizontal and diagonal lines. Exit geometry
includes vertical, horizontal and diagonal lines in both directions, unequal
source/exit lengths, fractional positions, and front-higher, back-higher and
equal exit floors. Ground-level, rising and falling actor states check relative
height and positive/negative/zero vertical momentum.

All **1,456 integer actor-state comparisons match DSDA-Doom 0.27.5, complevel 9**:
position, floor/ceiling, angle, all three momentum components, reaction time,
remaining line special, blocker survival and absence of teleport fog. Negative
cases check monster-only player rejection, back-side crossings, missing exits,
blocked monsters, player telefrags and missile rejection. W1 specials remain
available on failure and clear on success; WR specials remain active.

The reference runs with monsters enabled. Most cases capture immediately after
`P_TryMove` crosses a line or `P_UseSpecialLine` activates a switch, before subsequent friction or gravity changes the
exit state. Forty-two additional cases use actual imp `P_MobjThinker` movement
for 1, 2 and 5 updates; state transitions are delayed to isolate movement from
AI randomness. These verify crossing and continued physical movement, not full
combat/AI RNG parity. The reference's `p_telept.c`, `p_spec.c`, `p_switch.c`,
`p_map.c`, `p_mobj.c` and `p_enemy.c` are byte-identical to the pristine source.

This exposed and fixed a movement difference: Boom momentum probes allow
ledge dropoff while AI walking probes retain their restriction. Normal play
selects that behavior from Boom map features; 201/202 demos select Boom rules,
while vanilla demos retain their original rules. Exit references cache by tag
in map order, so repeated crossings avoid repeated full-map searches. No
additional mutable save fields are needed. Unit tests cover state restoration,
first-exit rejection, zero tags, switch/button lifetime, non-player use restrictions
and the unchanged AI/vanilla ledge gates.

```sh
python3 scripts/make_boom_teleports_wad.py
GOCACHE=/tmp/gd-doom-go-cache python3 scripts/verify_boom_teleports.py \
  --iwad wads/DOOM2.WAD --reference /path/to/capture-build/dsda-doom \
  --out /tmp/gddoom-boom-teleports
```

Use `--reference-source` with pristine DSDA-Doom 0.27.5 to create a private capture
build instead. The driver saves both CSVs, the input cases, hashes and
`results.json`. The five existing no-monsters demos still match all **5,772
frames** and exit tics; the full Go suite also passes.

## Friction and environmental force verification

`make_boom_environment_wad.py` creates 20 original maps and **1,081 cases**:
441 player, 320 imp and 320 cacodemon cases. The DSDA-Doom 0.27.5 reference
runs at compatibility 9 with monsters enabled. **All 8,474 exported state rows
match exactly**, including X/Y/Z, all momentum components and the raw stored
friction/movement factor at every captured tic.

There are 529 isolated force/friction cases and 552 full-simulation cases.
The isolated cases call the reference's actual friction/pusher thinkers;
full simulation uses `P_Ticker`, player commands and real monster thinkers.
Monster state transitions are delayed to keep AI randomness out of the physics
comparison. This verifies force immunity and physical movement for monsters,
not full combat/AI RNG parity. The reference `p_spec.c`, `p_map.c`, `p_mobj.c`,
`p_user.c` and `p_sight.c` remain byte-identical to the pristine source.

Coverage includes ground, air, submersion, wading and exact body waterlines;
positive/negative controller vectors; mud/ice inertia and acceleration;
additive wind/current/friction; overlapping floors; icy wall collisions;
noclip; enable/disable timing; point-force range, center, diagonals and LOS;
cross-sector forces; and last-source selection. Unit tests additionally check
voodoo versus controlled-body noclip behavior, marker collision properties,
binary snapshot compatibility and identical movement after a checkpoint.

The implementation retains Boom's original, unclamped friction formula and
linear point-force approximation. It does not substitute MBF formulas or make
ordinary monsters respond to Boom wind/current/pushers. Controller references
and thinker orders rebuild from the map. A cached voodoo list avoids walking
all monsters for every friction/force controller. No render path changes are
required.

```sh
python3 scripts/make_boom_environment_wad.py
GOCACHE=/tmp/gd-doom-go-cache python3 scripts/verify_boom_environment.py \
  --iwad wads/DOOM2.WAD --reference /path/to/capture-build/dsda-doom \
  --out /tmp/gddoom-boom-environment
```

`--reference-source` can build a private capture copy of pristine DSDA-Doom
0.27.5. Both CSVs, input cases, hashes and `results.json` are saved. The five
existing demos still match **5,772 frames** and their level exit tics. The full
Go suite and targeted save/authority checks pass.

## Distinct card/skull lock verification

`scripts/verify_boom_locks.py` compares **262,144/262,144 matching rows** with
DSDA-Doom 0.27.5 at complevel 9: every one of the 1,024 generalized locked-door
specials, all 64 six-key combinations, the player and an actual map imp, and
two consecutive trigger attempts. It checks the unlock predicate, door creation,
one-shot consumption, repeat/busy rejection, speed, top height, direction,
raise/open kind, wait time and rejection message. Locked doors use Boom's
150-tic `VDOORWAIT`, rather than the 140-tic generalized-door delay selection.

The fixture invokes the real use, walk-crossing and gun dispatchers in each
engine. It does not simulate approach movement, aiming, or monster AI for this
matrix. The reference's lock predicate, trigger dispatchers and movers remain
unchanged; a private messenger accessor exposes the rejection text. GD's
`USE:` debug status is excluded from the message comparison.

Independent card ownership survives pickup order, IDKFA, deathmatch grants,
co-op sharing/respawn, saves, keyframes and authority snapshots. Tests cover
all pickup masks and snapshot variants, co-op merging of separately acquired
cards/skulls, old-format loading and preservation of the previous friction
extension. Older saves reconstruct card ownership from collected pickups and
aggregate color flags, ignoring pickups excluded by session spawn settings.
If an older save acquired both variants through a cheat or another player,
without a recorded card pickup, its original aggregate flags cannot identify
that card; this ambiguity cannot be recovered exactly.

```sh
python3 scripts/make_boom_locks_wad.py
GOCACHE=/tmp/gd-doom-go-cache python3 scripts/verify_boom_locks.py \
  --iwad wads/DOOM2.WAD --reference-source /path/to/pristine/dsda-doom \
  --out /tmp/gddoom-locks
```

The verifier retains both CSVs, input WAD, hashes and `results.json`.
The full Go suite passes. The five existing Ancient Aliens, Sunlust and Sunder
no-monsters demos still match **5,772 frames** and all five exit tics.
The map audit now reports 16 unknown linedefs in Ancient Aliens (previously 21)
and five in Sunlust (previously 12), with no unknown sector specials in either.
This removes all five and seven distinct-lock gaps respectively.

## Legacy mover and elevator verification

`scripts/verify_boom_legacy.py` compares **208,202/208,202 matching rows** with
DSDA-Doom 0.27.5 at complevel 9, across **1,354 cases and 47 added specials**.
The original three-sector fixture includes a correctly oriented donut layout
and a malformed variant. Cases cover positive, negative and fractional heights,
next higher/lower and current-floor elevators, tight clearance, empty sectors,
stationary players and actual imps (including bodies straddling moving/static
sector boundaries), numeric models and missing models, zero and
missing tags, back-side use, monster activation, busy floor/ceiling/elevator
slots, immediate retries and later repeat activation.

Comparisons include every sector's floor/ceiling height, floor texture, type,
secret/friction/push transfer flags, actor height and health, plane-slot
ownership, remaining special, switch texture and button restoration timer.
The harness runs the actual trigger dispatchers and movers in thinker order;
actor AI and horizontal movement are not stepped. Use retries stop after player
death, as they do in the normal input path. Core DSDA dispatch, mover and
collision code remains unchanged.

The implementation follows Boom's ordered elevator plane moves: ceiling first
when descending, floor first when ascending, with a blocked leading plane
suppressing the trailing plane. The original clearance can therefore stall an
elevator around a standing actor even though its final room height is constant.
Completion retains Doom's strict destination overshoot timing. WR combined
special 151 runs both mover functions, whereas switch variants 166/186 use
short-circuit dispatch. Numeric change-only actions accept a tagged sector even
when no model exists. Malformed donuts whose first pillar line is one-sided
are rejected. Turbo stairs retain Boom crushing and rollback behavior; instant
toggles remain in stasis between activations.

```sh
python3 scripts/make_boom_legacy_wad.py
GOCACHE=/tmp/gd-doom-go-cache python3 scripts/verify_boom_legacy.py \
  --iwad wads/DOOM2.WAD --reference-source /path/to/pristine/dsda-doom \
  --out /tmp/gddoom-legacy
```

Both CSVs, input manifest/WAD, hashes and `results.json` are retained. Focused
tests verify elevator slot ownership, save/keyframe continuation, toggle
reversal after loading, turbo-stair crush state and older card-key extensions.
The five existing no-monsters demos still match **5,772 frames** and all exit
tics. The map audit reports one unknown linedef in Ancient Aliens (generalized
stairs, 13136) and one in Sunlust (generalized crusher, 12200), with no unknown
sector specials. An audit without unknown legacy numbers does not establish
full compatibility with every mover interaction.

## Generalized stair and crusher verification

`scripts/verify_boom_generalized.py` compares **495,456/495,456 matching rows**
with DSDA-Doom 0.27.5, complevel 9, across **2,400 cases**. Every one of the
1,024 stair encodings and 128 crusher encodings is exercised. Longer cases
check all speeds, stair sizes, directions and texture rules, chain completion,
early lock rejection and repeat direction changes. Crusher cases include
stationary players and real imps, crushing/death, bodies across sector edges,
slowdown and restored speed, stop/resume, and sector retagging while stopped.
Other cases check monster permissions (including secret lines), zero/missing
tags, front/back activation, busy floor/ceiling/elevator slots, skipped busy
stair neighbors, switch changes and button restoration.

Each row compares all sector plane heights, stair locks and links, mover slot
ownership, floor speeds/destinations/directions, ceiling current/original
speeds/directions, silent flags, actor height/health, remaining special and
switch/button state. The harness runs real trigger dispatchers and ordered
mover functions. Actor AI and horizontal movement are not stepped. Non-selected
observer bodies do not block planes; stop/resume cases call the unchanged
crusher action directly so dead player bodies can remain in the test. Sound
playback is outside this state comparison.

The implementation preserves Boom's generalized walk-crusher dispatcher
omission: both walk triggers reject activation at complevel 9. It does not
substitute MBF21's repaired walk behavior. Slow crushers (speeds 1 and 2) reduce
to 1/8 unit per tic while blocked and restore their original speed at the
bottom. Faster crushers retain their speed. Generalized stairs do not crush;
all steps retain their lock until the chain finishes, including completed steps
whose floor thinker has already been removed. Successful repeat stairs toggle
the direction bit. Saved state preserves these links/locks, original crusher
speed and tag, and flattened player-corpse colliders.

```sh
python3 scripts/make_boom_generalized_cases.py
GOCACHE=/tmp/gd-doom-go-cache python3 scripts/verify_boom_generalized.py \
  --iwad wads/DOOM2.WAD --reference-source /path/to/pristine/dsda-doom \
  --out /tmp/gddoom-generalized
```

The verifier retains both CSVs, input manifest/WAD, hashes and `results.json`.
Focused tests cover saves/keyframes in a partially completed chain and a slowed,
stopped crusher, repeat reversal after loading, preserved original tags and
corpse state. Existing legacy-mover and key-lock matrices still match
**208,202** and **262,144** rows respectively. The five existing no-monsters
demos still match **5,772 frames** and all exit tics; the full Go suite passes.
Ancient Aliens and Sunlust now have **zero unknown linedef and sector specials**
in the map audit. This describes recognized actions and these tested paths,
not full-map or combat compatibility.

## Manual-door/mover interaction verification

`scripts/verify_boom_manual.py` compares **500,346/500,346 matching rows** with
DSDA-Doom 0.27.5 at complevel 9 across **2,042 cases**. The original fixture has
two independent lines sharing a manual-door sector, with 16 classic/generalized
manual door specials and 19 idle or active mover setups. Cases include players
and actual imps, moving/waiting/stopped lifts, ascending/descending elevators,
fast/slow/stopped crushers, active and partially completed locked stair chains,
plain floors/ceilings, and opening/closing/waiting/one-shot/generalized doors.
Other cases check keys, secret lines, back-side use, low targets, tight clearance,
repeated use at different phases, displaced controllers and later replacements.

Each row compares both line specials, all sector plane heights, actual floor
and ceiling slot owner kinds, every live controller in thinker order (including
controllers displaced from their sector slot), door directions/speeds/targets/
waits/countdowns, lift status and waits, crusher/elevator/floor state, stair
locks, and actor height/health. The harness calls real use dispatchers and
ordered movers; it does not step actor AI or approach movement. Non-selected
observer bodies cannot block the fixture. Return values that only report a
handled line are not treated as successful movement: rejection is checked through
unchanged controllers, line consumption and actual slot ownership. Audio is
outside this state comparison.

The implementation preserves Boom's quirks deliberately. A floor-only lift or
stair does not trigger the vanilla manual-door platform cast in Boom mode.
Repeatable legacy doors can reverse an existing door, while generalized manual
doors reject a busy ceiling slot. One-shot legacy doors overwrite ceilingdata
without unlinking the displaced thinker. When that older thinker later finishes,
it clears the pointer even if a newer door owns it; live thinkers can therefore
exist with an empty sector slot. Sparse ownership records and retained thinkers
preserve these cases. Upward Boom lifts cap their effective destination at the
current ceiling, and reused Boom door objects clear stale countdown fields.
Vanilla demo platform-cast behavior remains intact.

```sh
python3 scripts/make_boom_manual_cases.py
GOCACHE=/tmp/gd-doom-go-cache python3 scripts/verify_boom_manual.py \
  --iwad wads/DOOM2.WAD --reference-source /path/to/pristine/dsda-doom \
  --out /tmp/gddoom-manual
```

Both CSVs, input manifest/WAD, hashes and `results.json` are retained. Focused
tests check saves/keyframes while doors and crushers overlap, explicitly empty
slots with old controllers still running, canonical retained-door state,
authority snapshot application, ownership validation, and the older generalized
state extension. The existing generalized, legacy and lock matrices still match
**495,456**, **208,202**, and **262,144** rows respectively. All five no-monsters
demos still match **5,772 frames** and exit tics, and the full Go suite passes.

## Moving-sector sound verification

The sector sound verifier compares **1,340,462/1,340,462 frames** across
**6,062 cases**, containing **72,904 ordered sector sound requests**, against
DSDA-Doom 0.27.5 at complevel 9. It reuses the legacy, generalized and manual-door
matrices and adds 266 sound cases on a fixture translated by `(37, -19)` map
units. Both engines run real activation dispatchers and ordered mover ticks.
Every captured frame includes the ordered sound lump, sector identity, exact
16.16 origin and loop timeout, including frames with no requests. Existing
manual cases aggregate their setup/aging requests into frame zero; the added
sound matrix has no aging stage and captures each subsequent tic separately.

Coverage includes floor/stair/ceiling/elevator movement loops and completion,
platform activation, endpoint waits, blocked reversals and retriggers, crusher
stasis/resume, legacy silent-crusher endpoint sounds, generalized crusher and
toggle-platform silence, and all generalized door speeds. The extra matrix
uses stationary players and real imps as blockers; actor AI is not stepped.
Perpetual lifts now use the normal seeded RNG and map setup without forcing
their starting direction. Both starting directions remain covered.

Only the private reference's `S_StartSectorSound` and `S_LoopSectorSound`
entry points gain observation hooks, after their silent-sector guards. Its
mover/dispatcher sources remain byte-identical to pristine DSDA-Doom. `-nosound`
disables device output while these hooks capture the engine's requests. The
comparison therefore establishes sound selection, request ordering, timing,
spatial origins and silence, rather than equality of mixed audio waveforms.

The PCM backends convert loop refreshes into finite eight-tic windows, retaining
sample phase across consecutive refreshes and letting unrefreshed loops expire.
One-shots replace the movement channel for their sector. Distinct sectors retain
separate channels even when their bounding-box centers coincide. Unit tests check
phase continuity, duplicate refreshes, expiry, cancellation, pitch-adjusted window
lengths and authority sound-event identity/validation. Native Raylib integration
checks run against ALSA null PCM and verify real sound-buffer creation, refresh,
origin-specific replacement and cleanup; this is not a listening check. Existing
PC-speaker effect policy and mixer-wide pitch/channel limits retain their prior
behavior.

```sh
GOCACHE=/tmp/gd-doom-go-cache python3 scripts/verify_boom_sector_sounds.py \
  --iwad wads/DOOM2.WAD --reference-source /path/to/pristine/dsda-doom \
  --out /tmp/gddoom-sector-sounds
```

An already instrumented reference can be supplied with `--reference`. Each suite
retains its movement and sound CSVs, manifests/WAD, logs, hashes and results;
aggregate results are in `results.json`. The legacy/generalized/manual movement
matrices and all **262,144** key-lock rows still match. All five existing demos
still match **5,772 frames** and exit tics. The full Go suite and native audio
integration checks pass. No live listening or mixed-waveform comparison was
performed.

## Boom demo options and RNG verification

The Boom option block now decodes its original flags and big-endian 32-bit seed.
Gameplay random calls carry DSDA's stable class identities. Seed initialization,
32-bit multiply/add wraparound, shared gameplay streams without insurance,
isolated classes with insurance, the logic-tic shuffle, and the separate cosmetic
stream follow DSDA-Doom 0.27.5's `M_ClearRandom`/`P_Random`. Normal Boom maps start
with seed 1993; demo playback uses the recorded seed, including zero. Both
legacy compatibility indices still advance, and vanilla sessions retain their
original table values and call order. The reserved MBF-era class numbers are
preserved without enabling MBF demo formats or option semantics.

Save version **36** and keyframe version **23** retain the RNG mode, initial
seed, insurance setting, all 65 stream seeds, both compatibility indices, logic
tic phase, decoded game options and remembered enemies. Tests save after mixed
class/cosmetic draws and resume another 128 draws/tics after loading. Older
snapshots restore their original table indices instead of inventing missing
seeded history. Failed authority map setup restores the complete RNG state.
Authority snapshots also retain enemy memory; the simulation ID is now
`gd-doom-authority-dev-18`.

`scripts/verify_boom_rng.py` retains manifests, generated WADs/LMPs, raw CSVs,
logs and source/reference hashes. Current DSDA comparisons pass:

| Comparison | Matching rows |
| --- | ---: |
| RNG vectors: five seeds, both insurance settings, four logic phases, fixed and mixed classes | 163,840 / 163,840 |
| 72 perpetual-lift cases: generalized and legacy, prior draws, stop/resume, no forced directions | 36,936 / 36,936 |
| 19 generated option/combat demos, including respawn and continuous-fire exhaustion runs | 10,350 / 10,350 |

The combat fixtures contain actual imps, zombiemen and demons, with full player,
monster, projectile and damage stepping. The 12 basic combat demos vary seed and
insurance mode; six option demos cover recoil, disabled bobbing, fast monsters,
respawn, no-monsters and disabled enemy memory. Eighteen contain monsters;
one specifically verifies the no-monsters option. Each completed tic compares
player position/angle/momentum, health, armor, bullets and physical view height,
all non-player/non-pickup actor positions, health, state/tics and momentum,
and every gameplay RNG seed. Cosmetic misc RNG, player-body animation states and
pickup animation states are outside this comparison. Monster memory and disabled
friction/pusher flags also have focused state/continuation tests.

The reference runs real Boom demo playback through `G_Ticker`; the only added
hook observes completed tics. Its RNG, monster, damage, weapon and mover
implementations remain unchanged. Vector and lift probes exercise those same
reference functions directly. The existing five no-monsters demos still match
**5,772 frames** and exit tics, and all **1,340,462** mover/sound regression frames
still match. The full Go suite passes.

```sh
GOCACHE=/tmp/gd-doom-go-cache python3 scripts/verify_boom_rng.py \
  --iwad wads/DOOM2.WAD --reference-source /path/to/pristine/dsda-doom \
  --out /tmp/gddoom-rng
```

This verifies the listed generated demos, not arbitrary existing combat demos.
The former ammo-exhaustion diagnostic is now included in the default regression
set and matches all **2,200 tics**, including the previous divergence at tic 748.
It remains individually selectable as `--case diagnostic_ammo_exhaustion`.

## Ammo-exhaustion and weapon timing verification

Boom moved automatic fallback selection from `P_CheckAmmo` into input command
construction. An empty Boom weapon therefore waits for an actual change command
when replaying a demo or receiving multiplayer input. Local input selects the
default preferred usable fallback and records that choice. Vanilla demo playback
retains its original immediate fallback behavior. A failed ammo check returns
before starting an attack; switching does not clear held-attack/refire flags
outside their normal psprite actions. Rocket/BFG weapons therefore still require
release/repress when raised while attack is already held.

Boom commands use four weapon bits, with direct fist, shotgun, chainsaw and SSG
identities. Numeric key toggles are resolved while sampling local input. The
network validator accepts the nine valid weapon codes and rejects undefined
codes or weapon bits without a change command. Boom fallback accepts exactly two
SSG shells or 40 BFG cells; old Doom fallback retains its three-shell/41-cell
thresholds. Custom weapon preference ordering is outside this slice.

The new DSDA matrix matches **622,080/622,080 frames across 2,592 cases**. It
covers all nine weapons, twelve ownership/ammo profiles, initially empty and
last-shot states, held attack, release, repress, and explicit mid-attack changes.
It compares old Doom playback, Boom recorded commands, and generated Boom
fallback commands. Unowned switch requests and shared-ammo thresholds are
included. Every frame compares command bytes, ready/pending weapons, attackdown,
nonzero refire, all four ammo counts, canonical weapon and flash state IDs/tics,
weapon Y, fallback selection, gameplay table index and all seeded gameplay
streams. Cosmetic RNG is excluded; the runtime models refire as a boolean.

The private reference calls real `P_PlayerThink`/`P_Ticker` and psprite functions
with monsters disabled for this matrix. Its input controller uses the source
`G_BuildTiccmd` policy with real `P_CheckAmmo`/`P_SwitchWeapon` and explicit default
preferences; this does not test keyboard/device sampling. No reference weapon,
player, RNG, or mover implementation is changed. Separate generated combat demos
run through normal DSDA `G_Ticker` with actual monster AI and damage.

The matrix also caught a BFG flash restarting at projectile launch. The flash now
starts only from its original gun-flash action, preserving its phase through
`A_FireBFG`. All **10,350** combat/option demo frames, **163,840** RNG vectors and
**36,936** lift frames still match, as do the existing five demos (**5,772 frames**
and exit tics). The full Go suite and native Raylib package checks pass.

```sh
GOCACHE=/tmp/gd-doom-go-cache python3 scripts/verify_boom_weapons.py \
  --iwad wads/DOOM2.WAD --reference-source /path/to/pristine/dsda-doom \
  --out /tmp/gddoom-weapons
```

Manifests, original fixture WAD, both CSVs, logs and input/source/reference hashes
are retained with `results.json`. Focused tests cover exact command decoding,
empty Boom weapons, command generation after release, pending-switch suppression,
fallback boundaries and network validation. Save/keyframe layouts remain **36/23**;
this behavior and command interpretation change advances the multiplayer
simulation ID to **`gd-doom-authority-dev-18`**. Matching clients/servers are
required. The auto-switch suppression latch belongs to local input, not replicated
simulation or demo playback.

## Revenant and arch-vile combat comparisons

Revenant tracers now use the saved world/RNG phase for their four-tic homing
cadence, including live games without a demo cursor. Boom resurrection checks
use the restored corpse radius, reject blocked bodies, and restore full bounds
to crushed corpses. Vanilla ghost resurrection remains available in vanilla
compatibility. Idle Boom actors continue their momentum and state countdowns
after the player dies.

The Ancient Aliens combat comparison also exposed related Boom differences:
bullet puffs below two sky ceilings, support refreshes for actors outside a
moving sector's touching set, airborne eye height and ceiling clipping, the
cached onground decision during teleport reactiontime, and silent-teleport bob
phase. These now follow the reference behavior. Combat traces also use corrected
canonical state numbers and terminal corpse countdowns.

Run the reproducible comparison with an owned Doom II IWAD:

```sh
python3 scripts/verify_boom_combat.py \
  --iwad wads/DOOM2.WAD \
  --reference-source /path/to/pristine/dsda-doom-0.27.5/prboom2 \
  --out /tmp/gddoom-combat
```

A prebuilt reference with the repository's current observation probe can instead
be passed with `--reference`. `--aaliens` accepts an existing Ancient Aliens WAD;
otherwise the existing verifier's pinned archive is downloaded. Inputs, observer,
reference core, verifier, Go sources and CSV outputs have SHA-256 provenance in
`results.json`. The local evidence is `/tmp/gddoom-combat-final/results.json`.

| Comparison | Matching tics |
| --- | ---: |
| 16 revenant/vile demos varying seed, RNG insurance and held attack | 5,600 / 5,600 |
| 11 focused live combat scenes | 11,000 / 11,000 |
| Ancient Aliens MAP01 opening combat encounter | 500 / 500 |
| Total | **17,100 / 17,100** |

The focused scenes cover close melee; horizontal/vertical homing, smoke and a
moving target; natural zombie resurrection; restored crushed-shotgunner bounds
and pillar rejection; revenant/baron mutual retaliation; vile retaliation after
missile damage; the lack of retaliation against an arch-vile's blast; and
occluded sight. Coverage assertions reject a comparison that never reaches its
named behavior. The crushed-corpse fixtures prepare a stationary initial corpse
with damage and the reference gib state/bounds; normal live monster thinkers
perform every subsequent search, position check, heal and raise. They do not
establish every crusher/resurrection interaction.

The Ancient Aliens LMP uses the first 500 commands of the pinned `aa01o023`
movement route with monsters enabled and attack held. It includes awake monsters,
a cyberdemon firing rockets, monster damage/death and player death. This is an
opening-encounter combat demo, not a completed-map combat run or a verification
of death/rebirth transitions.

Each tic compares player position, angle, momentum, health, armor, bullets and
eye height; ordered monster, missile and effect states, countdowns and physics;
target/tracer presence and type, chase fields and body/support bounds; the shared
random-table index and every gameplay RNG seed. Static decorations, player and
pickup animation, cosmetic misc RNG and identity among same-type targets are
outside this comparison. The reference gameplay implementations are unchanged;
its added hooks observe completed tics and prepare the explicitly labeled corpse
fixtures.

Existing regressions also pass: 163,840 RNG vectors, 36,936 lift rows, 10,350
option/combat demo frames, 622,080 weapon frames, 1,340,462 mover/sound frames
with 72,904 sound requests, and all 5,772 frames of the five no-monsters routes.
The full Go suite and native raylib runtime tests pass, including binary save,
keyframe and authority-codec round trips. The multiplayer simulation ID is now
`gd-doom-authority-dev-19`; saves are version 37 and keyframes version 24 to
retain the teleport camera's cached onground state. The previous Boom formats
retain their RNG streams when loaded and default the new camera cache.

## Manual door lighting verification

`scripts/verify_boom_manual.py --door-lighting` compares the real door
dispatchers and mover ticks with DSDA-Doom 0.27.5 at Boom complevel 9.
All **381,072/381,072 rows across 272 cases** match using Freedoom 2 as the
base IWAD; **220 cases** change one or more light levels. The matrix covers
regular/keyed/blazing manual doors, both generalized push triggers, all four
generalized speeds/kinds/delays, generalized locks, adjacent tagged sectors,
changing neighboring lights, retriggers, actor obstruction, zero movement
ranges, missing/zero tags, back-side use, tag-operated negative controls,
and overlapping movers. Actor AI and approach movement are outside that matrix.
The original manual-door regression also matches **500,346/500,346 rows across
2,042 cases**.

The wider delay matrix caught an existing generalized-door timing error:
the middle waits were 140 and 315 tics. They now match the reference's 150
and 300 tics. Lighting uses the controlling line retained when the thinker
was created; retriggering it does not replace that line. Open completion takes
the brightest neighboring light (starting at zero); close completion takes
the minimum of the sector's current light and its neighbors. Self-referencing
lines are ignored, and adjacent tagged sectors observe earlier updates in
map order. A zero top-minus-floor range suppresses lighting changes.

```sh
python3 scripts/verify_boom_manual.py --door-lighting \
  --iwad /usr/share/games/doom/freedoom2.wad \
  --reference-source /path/to/pristine/dsda-doom-0.27.5 \
  --out /tmp/gddoom-door-lighting

# Press Use at the starting door. The room brightens when the door finishes
# opening and darkens when it finishes closing.
go run . -wad /usr/share/games/doom/freedoom2.wad \
  -file testdata/boom/door-lighting.wad -map MAP01 -no-monsters \
  -sourceport-mode=true -mouselook=true
```

An owned Doom II IWAD can be substituted. Regenerate the original fixture and
case manifest with `python3 scripts/make_boom_door_lighting_wad.py`.
The comparison retains both CSVs, the generated WAD, manifests, logs, hashes
and `results.json`; this session's evidence is in
`/tmp/gddoom-door-lighting/comparison` and `manual-regression`.
Focused tests cover binary save/keyframe continuation, displaced thinkers,
authority snapshot round trips, invalid lighting references, render-cache
refreshes, vanilla-demo gating and previous-format camera/RNG preservation.

A separate no-monsters preview exercises actual player Use input and renders
initial, opening, open and reclosed views at tics 0, 32, 64 and 280. Its player,
sector, light and resource states match DSDA; CPU/GPU images are identical
in both Faithful and Source Port modes.
Exact DSDA screenshot parity remains outside this change: whole-image
pixel differences range from 0.51% to 18.45% in these views. The local visual
comparison is `/tmp/gddoom-door-lighting/visual`.

## Full-map combat demo verification

`scripts/verify_boom_full_combat.py` replays unchanged Sunlust UV-Max
recordings. The original WAD and recording archives come from
[DSDA](https://dsdarchive.com/wads/sunlust) and are pinned by SHA-256.
Every active map tic matches DSDA-Doom 0.27.5:

| Recording / `--case` | Map | Matching active tics | Exit kills | Secrets | Health |
| --- | --- | ---: | ---: | ---: | ---: |
| `sl01-124` (default) | MAP01 | 2,972/2,972 | 77/77 | 2/2 | 28 |
| `sl01-126` | MAP01 | 3,024/3,024 | 77/77 | 2/2 | 69 |
| `sl02m225` | MAP02 | 5,086/5,086 | 94/89 | 3/3 | 50 |

The MAP01 routes are mhrz's DSDA-Doom 0.24.3 recordings; MAP02 is Kinetic's
DSDA-Doom 0.29.4 recording. All use Boom complevel 9. Independent completion
summaries agree, including leveltime. Final traces contain no surviving counted
monsters; kill totals alone would not establish that after resurrection.
MAP02's extra kills come from arch-vile resurrection, which the verifier
requires the trace to actually exercise. The recordings contain 3,194, 3,135
and 5,101 commands respectively; following intermission commands are outside
the active-map comparison.

Every active tic compares player position, facing, momentum, health, armor,
all four ammo types, ready/pending weapon, weapon and flash state/timing,
attack/refire flags, kills, secrets, every sector's physical floor/ceiling and
light level, ordered combat actors, barrels and effects, and all gameplay RNG
seeds.
The existing combat trace's target/tracer type, chase memory and actor
bounds/support checks are retained. Static decorations, player/pickup animation,
cosmetic misc RNG and identity among same-type targets remain excluded.

The MAP01 routes exposed and now cover these corrections:

- A non-solid Imp corpse stopped against an actor at tic 315. Boom permits
  non-solid movers through solid actors; vanilla demos retain their old rule.
- A Pain Elemental created a skull across a blocking line at tic 1,140.
  Boom rejects the path before spawning, avoiding the extra corpse and RNG
  draws. The implementation also applies Boom's removed population cap and
  pre-move vertical spawn checks.
- Canonical trace labels for several monster states were off by one or two,
  and the Pain Elemental walk cycle was exported as a single state. Labels
  now follow the original state table. These exporter corrections do not
  change the monsters' animation timing or AI.
- Exploded Revenant missiles lost their original tracer type in the trace.
  The existing saved effect-target fields now retain it through normal and
  deferred impacts, binary saves and snapshots.
- The alternate route's Mancubus shot disappeared below a sky ceiling at
  tic 605. Boom removes shots only above the back sector's sky ceiling;
  lower shots explode, including the explosion RNG draw. Vanilla retains
  its unconditional sky-removal behavior.
- A rocket explosion stopped its remaining half-step against a solid actor
  at tic 1,867. Boom permits the non-solid death object through actors while
  retaining line and height checks. Vanilla keeps its original actor collision.

```sh
python3 scripts/verify_boom_full_combat.py \
  --iwad /usr/share/games/doom/freedoom2.wad \
  --reference-source /path/to/pristine/dsda-doom-0.27.5 \
  --out /tmp/gddoom-full-combat
```

An owned Doom II IWAD can be substituted. DSDA runs with `-nodeh` to preserve
the original Doom gameplay definitions when using Freedoom assets (whose patch
changes SSG flash timing). Sunlust's pinned gameplay patch affects Commander
Keen and its states; the verifier checks the patch hash and verifies that the
selected map contains no Keen. This exclusion applies to these maps and does
not establish support for Sunlust's custom actor on other maps.

Select another recording with `--case sl01-126` or `--case sl02m225`.
Both CSVs, completion summaries, input archives, logs and input/source/reference
hashes are retained in `results.json`. Session evidence is in
`/tmp/gddoom-full-combat/results.json`, `/tmp/gddoom-rebirth/map01-alt/results.json`
and `/tmp/gddoom-rebirth/map02/results.json`. These complete surviving
routes do not verify intermission animation, following maps, arbitrary combat
demos or death/rebirth (covered separately below). The previous **17,100**
revenant/arch-vile/Ancient Aliens combat tics remain regression coverage.
Focused tests cover vanilla gating, sky-ceiling equality, explosion RNG draws,
corpse solidity, blocked skull paths without allocation/RNG draws, population
limits, vertical rejection and exploded-missile tracer preservation in saves.

### MAP02 crushers, conveyors and checkpoint continuation

`sl02m225` now completes with **5,086/5,086 matching active tics**, including
barrels' positions, momentum, health, death states/timing, body bounds and
support. Coverage assertions require barrel deaths and explosions, moving
blood and arch-vile resurrection. The previously failing replay is now a
permanent passing regression. The verifier also retains first differences
and row counts if a future change prevents either engine from exiting.

The route exposed these additional corrections:

- Crusher processing now follows Boom's persistent sector touching-actor
  lists, rather than blockmap-cell order. Movement within a sector retains
  the existing node; leaving removes it and re-entering prepends a new one.
  Player bodies, runtime spawns, pickups and removals update membership.
  The height-clip walk restarts after each callback so moving skulls and
  newly linked nodes are handled like `P_CheckSector`.
- Conveyors now carry floor blood and submerged puffs, smoke, teleport/BFG
  effects and missile death effects. Effects retain horizontal momentum.
  Their XY movement clamps and splits steps like Doom, retaining a successful
  half-step even if the remainder is blocked. Boom's non-solid actor collision
  and allowed-dropoff rules apply; wall and height checks remain active.
- Exploding barrels remain solid until removal or crusher gibbing. They do
  not run the monsters' `A_Fall` action. Canonical barrel trace states were
  also corrected to 806–812; that label correction does not alter timing.
- Isolated/self-referencing door sectors now use Boom's lowest-surrounding-
  ceiling rules: exclude self-references and use the 32,000-unit sentinel
  when no lower neighbor exists. The MAP02 machinery's doors therefore open
  instead of snapping down four units at tic 504. Vanilla keeps its previous
  neighbor handling.
- Arachnotron plasma flight states use five tics, including the randomized
  spawn duration. Cacodemon shots retain their four-tic states. This removes
  the final 19 actor-animation differences starting at tic 1,315.

The same full route also matches after **eight binary save/restores** and,
separately, **eight binary keyframe/restores**, at zero-based demo tics 47, 48,
49, 60, 504, 1,315, 2,400 and 4,800. Each checkpoint encodes/decodes the real
binary format, constructs a fresh level, restores state and RNG, and resumes
the original recording. Both traces and the final exit summary still match
DSDA. The reports require every requested checkpoint to have executed.

```sh
python3 scripts/verify_boom_full_combat.py --case sl02m225 \
  --checkpoint-format keyframe \
  --checkpoint-tic 47 --checkpoint-tic 48 --checkpoint-tic 49 \
  --checkpoint-tic 60 --checkpoint-tic 504 --checkpoint-tic 1315 \
  --checkpoint-tic 2400 --checkpoint-tic 4800 \
  --iwad /usr/share/games/doom/freedoom2.wad \
  --reference-source /path/to/pristine/dsda-doom-0.27.5 \
  --out /tmp/gddoom-rebirth/map02-keyframe
```

Use `--checkpoint-format save` for the save-format run. Evidence is in
`/tmp/gddoom-rebirth/map02-save/results.json` and
`/tmp/gddoom-rebirth/map02-keyframe/results.json`, including `checkpoints.json`
with the actual reload tics. Focused tests cover persistent node order versus
block relinking, mutable traversal, leave/re-entry, actor spawn/removal,
independent restored worlds, malformed membership rejection, authority baseline
ownership, conveyor eligibility, blocked split steps, barrel solidity and
neighbor/flight-state rules.

This verifies this MAP02 route and its binary state continuation. It does not
establish arbitrary-map parity, multiplayer voodoo-doll death behavior or
checkpointing in the middle of a simulation tic.

## Death and same-map rebirth verification

`scripts/verify_boom_rebirth.py` compares **35,400/35,400 tics across 33 replays**
against DSDA-Doom 0.27.5: 11 fixtures run continuously, then with binary save
and keyframe restores. The continuous runs include **44 real level reloads**;
the checkpointed variants perform **610 binary restores** at available tics
before, on and after deaths, pending rebirth requests and completed reloads.
It exercises:

- Close shotgun combat plus a damaging floor, seeds 0/1993 and RNG insurance
  off/on, with four separate Use/reload cycles per case.
- Held Use through repeated deaths, and Use on the lethal floor-damage tic.
  The latter waits for the next death thinker before requesting rebirth;
  the actual reload follows one tic later.
- Natural Revenant combat deaths without a damaging floor, with insurance
  off/on, and a low-ceiling variant of the shotgun/floor fixture.

The observer does not inject damage, death, inventory changes or reloads.
Alongside the full-map trace fields, every tic compares player lifecycle state,
local leveltime, reload count, view height and its delta, and completed RNG
phase. Coverage assertions require actual deaths and reloads, restored inventory
and health, continuous RNG streams and reset local time/phase; a quiet matching
fixture cannot pass without exercising rebirth.

The comparison exposed a gameplay bug at the first restart: RNG streams were
preserved correctly, but their shuffle/Revenant homing phase carried into the
new level. Boom's `G_DoLoadLevel` resets `boom_basetic` before spawning things.
Fresh level construction now resets only that phase, preserving seeds and
indices; saved snapshots still restore their exact phase. A focused regression
checks stream preservation and phase reset with insurance off/on.

Boundary checkpoints exposed two more missing pieces of saved state. A save
on the tic when Use requests rebirth lost `playerReborn`, leaving the restored
player dead instead of restarting on the next tic. Saves during Revenant
combat deaths also lost the attacker reference used by the death camera.
The new formats preserve both the queued request and the attacker's thing
index/position; the camera continues tracking a moving attacker. Restore also
clears stale requests when the saved player has none. Authority baselines keep
the viewer's request separate from other players and reject living-player
rebirth requests and invalid attacker references.

Every fixture now compares uninterrupted DSDA playback with uninterrupted
GD-DOOM, save-restored GD-DOOM and keyframe-restored GD-DOOM. Assertions require
all requested checkpoints to execute and to include living, dead and reborn
states. Use `--no-checkpoints` to run only the original continuous comparisons.

```sh
python3 scripts/verify_boom_rebirth.py \
  --iwad /usr/share/games/doom/freedoom2.wad \
  --reference-source /path/to/pristine/dsda-doom-0.27.5 \
  --out /tmp/gddoom-rebirth/matrix
```

Raw demos/WADs, both traces, coverage counts, logs and hashes remain alongside
`results.json`. Current evidence is in
`/tmp/gddoom-next-rebirth-checkpoint/matrix/results.json`. This covers live
single-player same-map rebirth and binary checkpoint continuation. Intermission
animation and multiplayer death/rebirth remain outside this matrix.

## Normal and secret map-transition verification

`scripts/verify_boom_transitions.py` runs real session/intermission/finale
logic for **9 cases and 4,755/4,755 matching active level tics**:

- MAP01 -> MAP02 normal exits, with RNG insurance off/on and seeds 0/1993.
- MAP15 -> MAP31 secret exits with the same RNG combinations.
- A held-attack secret exit that waits for the accelerated finale timeout.

Both engines consume the same recorded commands. Each active-level row also
records its original demo-command index and map, so a different intermission
or finale duration cannot be hidden by aligning only active level rows.
The fixtures verify damaged health and ammo carryover, cleared keys, preserved
weapon ownership by switching away and reselecting the shotgun in the next map,
continued RNG streams and reset per-level RNG phase. Each destination runs for
more than 100 live tics with a monster. No observer forces an exit or map load.

Normal exits reach MAP02 on zero-based command **72**. Secret exits exposed
an existing compatibility bug: GD-DOOM applied vanilla's delayed held-button
skip rule to Boom finales, reaching MAP31 on command 142 instead of 102.
Boom now uses attack/use edges: the first edge accelerates the text, another
edge advances, and accelerated text can time out without another edge. Input
latches carry into the next level. The secret route now reaches MAP31 on
command **102**, while held attack waits until command **1,074**, matching
DSDA. Vanilla's original finale timing remains covered by its existing tests.
Accelerated text presentation follows the faster reveal rate as well.

```sh
python3 scripts/verify_boom_transitions.py \
  --iwad /usr/share/games/doom/freedoom2.wad \
  --reference-source /path/to/pristine/dsda-doom-0.27.5 \
  --out /tmp/gddoom-next-rebirth-checkpoint/transitions
```

The WADs, demos, traces, coverage assertions, logs and hashes are retained in
that directory alongside `results.json`. This verifies these generated normal
and secret routes, including transition timing and destination gameplay. It
does not verify intermission/finale graphics or sounds, saves during those
screens, episode-ending finales, arbitrary campaign recordings or multiplayer
map transitions.

## Co-op voodoo dolls

Native `Authority` co-op joins now create the earlier numbered starts as
physical dolls for the joining slot; the last numbered start is the camera
spawn. Inactive slots and deathmatch do not create dolls. Doll thinkers use
that owner's inventory, keys, view height and player state, including when a
different player is the simulation anchor. Damage honors co-op friendly-fire
policy and retained projectile source generations. Crusher and telefrag damage
also use this routing; protected teammate dolls block a teleport destination.

Each body keeps independent health and momentum. Damage subtracts from the
shared player's health and the struck body's health, without changing another
body's health or momentum. A shared health value of zero does not by itself
kill a surviving body. Killing a doll sets the owner's death state and lowers
the weapon, while the separate camera body can remain physically alive. Dolls
retain health and corpse state across respawns. Explicit slot removal retires
them; a later join creates new actor identities. Native authority scoring still
records one death per incarnation.

`authoritative_voodoo_test.go` covers all four slots, non-anchor damage and
pickups, friendly fire and stale projectile sources, independent health,
respawn, disconnect/rejoin, crusher damage, telefrags, friction array growth,
and four viewer baselines. Save/keyframe world codecs preserve owner types,
body health, momentum and animation countdowns; restore rebuilds the doll
cache. Historical single-player owner types are recovered from original starts.
These world formats do not save the full authority roster, and replication
baselines are not server rollback checkpoints.

A reproducible controlled experiment compares seven damage/rebirth events for
two players against DSDA 0.27.5:

```sh
python3 scripts/verify_boom_voodoo.py \
  --iwad /usr/share/games/doom/freedoom2.wad \
  --reference /path/to/instrumented/dsda-doom \
  --out /tmp/boom-voodoo
```

Use `--reference-source` with pristine DSDA source to build an instrumented
reference, or supply a binary with the current probes installed. The optional
`GD_DSDA_VOODOO_OUT` probe explicitly injects the matched damage and rebirth API
calls after the first real two-player demo tic. It is not an observational
netdemo capture. All **14 player records match**, including shared and body
health, player death state, and doll corpse height/solid/shootable flags. This
does not establish multiplayer movement, RNG, retained camera corpses, or
campaign-transition parity with classic netdemos.

## Recorded co-op demo parity

Boom co-op demos now retain a complete command frame for each world tic,
including sparse slots and a console player other than slot 1. `Script.Tics`
is the console-player projection; `Script.Frames` contains the commands for
all active slots. Formatting writes complete frames and parsing rejects partial
frames, missing/invalid rosters, unsupported slots and multiplayer deathmatch.

Playback creates the recorded players during map setup. Each active start
consumes spawn RNG in THINGS order; earlier starts become dolls and inactive
slots do not spawn. Input runs in slot order and physical thinkers run in map
order against the shared world. Recorded playback uses classic view bobbing,
key/weapon persistence and respawn behavior, separately from native network
join, prediction, team-key and respawn-delay policy.

`verify_boom_coop.py` runs **40 cases / 9,360 recorded frames** against DSDA
0.27.5: movement, mutual pistol combat, monster combat, floor deaths and Use
rebirth, conveyors, crusher-driven doll deaths, persistent pickups, sparse slots
3/4, four-player startup and pause/unpause from another player's slot. Each
scenario uses seeds 0/1993 and RNG insurance off/on. Coverage assertions require
actual movement, damage, mover activation, death/rebirth and retained bodies;
loading a fixture without exercising its feature is not a pass.

```sh
python3 scripts/verify_boom_coop.py \
  --iwad /usr/share/games/doom/freedoom2.wad \
  --reference-source /path/to/pristine/dsda-doom-0.27.5 \
  --out /tmp/boom-coop
```

The co-op observer is read-only: both engines consume the same real demo
commands. Every frame compares all players' position, momentum, health, body
health, armor, ammunition, keys, view height, weapon state/countdown, refiring
boolean and attacker position. It also compares ordered non-camera actors,
retained bodies, pickups, sectors and gameplay RNG streams (cosmetic `pr_misc`
is excluded). Monster target identity/position, chase counters, last-look and
attack flags are included. Reports retain input/source/reference hashes and
first differences. Evidence: `/tmp/gddoom-coop-parity/results.json`.

The recordings exposed Boom's sight bounding-box rejection, the retained
player-search stop slot, non-solid corpse movement, zero-health weapon lowering,
and attacker/target pointers that must keep referring to old bodies after
rebirth. Respawn now retains the former camera actor in its original thinker
position without consuming spawn RNG for the conversion. Sound targets and
homing targets retain the same physical body; a replacement player receives
fresh inventory. A living camera left after a doll death stays an independent,
unowned body after respawn.

These recordings establish generated co-op gameplay parity, rather than universal
co-op or visual/audio parity. Broader community recordings and multiplayer
deathmatch remain follow-up work.

## Recorded co-op campaigns and checkpoints

Recorded co-op now continues through normal and secret exits, intermissions and
Boom commercial finales. Every living player carries health, armor, ammo and
weapons independently; dead players receive fresh rebirth inventory on the next
map. Keys and temporary powers clear for the whole roster. Attack/use edges from
any active player accelerate the screens. Co-op statistics tally each player's
kills, items, secrets and frags, and display the active roster.

`verify_boom_coop_campaign.py` compares **10 routes / 30 runs / 21,774 active
frames** against DSDA 0.27.5: two-player, sparse slots 3/4 and four-player rosters,
normal/secret transitions, natural tally timing, held input through a finale,
and a player dead at the exit. Continuous, save-restored and keyframe-restored
runs match every compared active frame, its original command position, all
players/bodies/items/sectors and gameplay RNG. This includes **520 restores**
before/after exits, during intermissions/finales and at next-map startup.
Evidence: `/tmp/gddoom-coop-campaign-final/results.json`.

```sh
python3 scripts/verify_boom_coop_campaign.py \
  --iwad /path/to/doom2.wad \
  --reference /path/to/instrumented/dsda-doom \
  --out /tmp/gddoom-coop-campaign
```

Build the reference with the observer installation used by
`verify_boom_coop.py --reference-source`. Campaign observation only adds the
original command number and map name; it does not change reference gameplay.
The comparator observes active level frames and checks screen duration through
command positions. It does not compare rendered pixels or screen audio.

Save 41/keyframe 28 include a bounded, versioned recorded-session attachment:
embedded demo commands/cursor/pause state, the complete owned roster, player
activation/latches, physical body ownership, target/sound/attacker references,
frags, pending map/carryover, intermission animations and finale state. Loading
validates the attachment and world before replacing the active session, and
restores the saved RNG after construction. Unit checks cover pending rebirth,
retained bodies/references, all RNG classes, exact binary continuation, screen
state and malformed data rejected without changing the active session. Existing
single-map co-op coverage remains **40 cases / 9,360 matching frames**.

Sparse recordings retain Doom's `playeringame[1]` netgame-rule switch: slots 3/4
alone use single-player item persistence and tally rules while still advancing
their complete recorded roster. Retained former-camera bodies now have one
sector-membership link, preserving crusher traversal through checkpoint loads.

These are recorded co-op session checkpoints. Native authority save/rollback
remains separate from client replication baselines and is not supplied by this
attachment. Community campaign recordings and multiplayer deathmatch still need broader
comparison coverage; generated episodic/endgame verification is described below.

## Episodic and endgame co-op finales

Boom episode endings now show the final map's statistics, queue victory for the
following command, and continue text/art using the whole recorded roster.
Headless playback advances these screens too. Text supports Boom acceleration
and natural timeout; episode art stays on screen instead of returning to the
frontend. Episode three renders the PFUB1/PFUB2 bunny scroll and END0–END6 overlays.

MAP30 renders BOSSBACK, the actor name and front-view sprite. Its cast ticker
uses cast-only Doom state data: walking, alternating attacks, deaths, all 17
actors, and wraparound. It never executes world actions or draws gameplay RNG.
A real host key kills the current cast actor; recorded ticcmd fire/use only
accelerates the story and never substitutes for that key event. Native and
Ebiten hosts use the same finale layout and ticker. The host selects finale,
cast and bunny music, and restores the appropriate track after checkpoint load;
audio sample-position seeking and mixer-wide parity are not established.

`verify_boom_coop_finale.py` compares **12 routes / 36 runs / 66,600 recorded
commands** against DSDA 0.27.5, with **786 checkpoint restores**. It covers E1M8,
E2M8, E3M8, E4M8 and MAP30 with two/four-player rosters, a natural episode-three
text timeout, and an explicit cast-key experiment visiting all 17 actors. Each
row compares screen phase, finale counter/stage, acceleration, cast actor/state/
tics/frame count/attack/death flags, all players' inventory and button latches,
and gameplay RNG (excluding cosmetic pr_misc). The optional key experiment
feeds matched host keydowns through the real reference F_Responder; the observer
otherwise only reads state after G_Ticker. Continuous, save and keyframe runs
match. Evidence: `/tmp/gddoom-coop-finale-final/results.json`.

```sh
python3 scripts/verify_boom_coop_finale.py \
  --iwad /path/to/doom2.wad \
  --episode-iwad /path/to/doom.wad \
  --reference /path/to/instrumented/dsda-doom \
  --out /tmp/gddoom-coop-finale
```

Build the reference from pristine 0.27.5 with the current observer installer.
`generate_finale_cast.py /path/to/dsda/src` reproducibly extracts the 338
cast-reachable state definitions. Rendering verification exports ten cast/bunny
scenes and checks identical pixels between native patch commands and Ebiten;
these are not DSDA pixel or sound comparisons. Run with

```sh
GD_FINALE_RENDER_OUT=/tmp/gddoom-finale-render xvfb-run -a go test -tags integration \
  ./internal/doomruntime -run '^TestFinaleRenderCPUAndNative$' -count=1
```

Save 42/keyframe 29 write co-op attachment version 2, including cast state and
pending episode victory. Version-1 attachments remain readable outside the
previously incomplete cast stage; missing cast actor state is rejected rather
than inventing continuation. Unit checks verify cast-death continuation, no
world RNG consumption, and malformed actor rejection without session mutation.
The existing normal/secret co-op campaign matrix remains 30 passing runs,
21,774 active frames and 520 restores. Native authority rollback, custom
DeHackEd cast changes, broader community campaigns and full presentation parity
remain follow-up work.

## Community co-op recordings

The unchanged Sunlust MAP31 **UV Pacifist co-op 0:14.37** and MAP32 **UV Max
co-op 1:25.89** recordings by 4shockblast and GarrettChan now match DSDA 0.27.5.
Both original console recordings are tested for each route: `sl31b014.lmp`,
`sl31g014.lmp`, `sl32b125.lmp`, and `sl32g125.lmp`. The recordings and WAD archives
are pinned by SHA-256; commands, map geometry and actor placement are unchanged.
Sources: [MAP31 archive](https://dsdarchive.com/files/demos/sunlust/61512/sl31pc014.zip),
[MAP32 archive](https://dsdarchive.com/files/demos/sunlust/92079/sl32mc125.zip).

`verify_boom_community_coop.py` runs both routes and both console views
continuously and through saves/keyframes: **12 runs / 21,054 matching active
frames / 80 restores**. It compares complete player/body coordinates, momentum,
health, weapons/ammo, attack/refire state, keys and attacker positions; combat
actors, projectiles/effects and dolls in thinker order; pickup persistence;
sector planes/lights; and gameplay RNG. Independent exit summaries also match
per-player kills, items, secrets, health/armor/ammo and level totals. MAP32 kills
all 105 counted monsters (88 credited to slot 1 and 17 to slot 2), using pistol,
rockets and BFG. MAP31 verifies its cooperative boost and conveyor-scripted exit.
Evidence: `/tmp/gddoom-community-coop-final/results.json` (combined reports from
`/tmp/gddoom-community-coop-map31-final` and `/tmp/gddoom-community-coop-map32-final`).

```sh
python3 scripts/verify_boom_community_coop.py \
  --iwad /path/to/doom2.wad \
  --reference-source /path/to/pristine/dsda-doom-0.27.5 \
  --out /tmp/gddoom-community-coop
```

The real recordings exposed and fixed:

- Co-op A_Chase reacquisition after losing sight of a target when its retaliation
  threshold expires. A successful search returns before movement and chase RNG.
- Boom lowering lifts continuing through intermediate insufficient-headroom
  checks, while preserving the final-destination obstruction check and vanilla
  reversion behavior.
- Slow doll momentum retaining friction decay when its owner starts moving,
  independent of the selected console player's command.
- Doom's shared `onground` value during co-op teleport freeze. The last roster
  entry's already-saved view flag supplies the next command's value.
- Classic non-player kill attribution to a live retained enemy's owner or the
  first active slot. Kill attribution keeps lastenemy even with AI memory off.
- Weapon pickups initializing an empty restored inventory rather than writing
  to a nil map.

These changes advance the simulation ID to `gd-doom-authority-dev-29`. The
existing save 42/keyframe 29 layouts already contain the required state.
Targeted tests cover the boundary behaviors; the 40-case generated co-op matrix
and the complete single-player Sunlust MAP02 route with keyframes remain matched.

This profile excludes static-decoration animation, cosmetic pr_misc, pixels,
sound and custom story text. It audits the pinned Sunlust DeHackEd patch and
rejects maps containing its altered Commander Keen before using `-nodeh` in the
reference, which also excludes Freedoom's unrelated weapon-frame patch. The
selected maps contain no Keen; active gameplay is unaffected by that exclusion.
The Ancient Aliens candidate requires a separate audit of its custom actors and
was not counted as verified. Community multi-map/endgame recordings, other map
sets and additional co-op combat/death combinations remain follow-up coverage.

## Remaining work

The next milestones are:

1. Expand complete-map combat coverage beyond the two verified Sunlust maps
   and their recorded routes. Add actual campaign recordings and save/keyframe
   boundaries during intermission/finale screens, keeping the movement, combat,
   lifecycle and mover matrices as regressions. Death/rebirth checkpoints and
   generated normal/secret transitions now have DSDA comparisons; MAP02 covers
   a complete crusher/resurrection route with binary checkpoint continuation.
2. Broaden community co-op coverage beyond the two verified Sunlust routes:
   multi-map recordings, other map sets, and more combat/death/exit combinations.
   Generated normal/secret campaigns, episodic/MAP30 finales and full-roster
   checkpoints already have DSDA comparisons. Native authority save/rollback
   remains separate from client replication baselines.
3. Add custom weapon preferences, then broaden complex-map progression and
   large-fight performance coverage. GL nodes and mixer-wide sound parity
   remain separate follow-up work.

Unusual interactions beyond the fixtures remain unverified. Maps may load
while unsupported actions behave incorrectly.

The no-monsters comparisons establish the recorded movement/progression paths,
not full combat RNG parity, every map route, or large-fight performance. The map
audit detects unknown specials; it does not detect every unsupported interaction.

These changes advance the multiplayer simulation ID to
`gd-doom-authority-dev-29`. Deploy matching clients and servers. Door lighting
adds the controlling line reference to owned and displaced door snapshots.
Sector identities extend the sound-event schema. Explicit ceiling ownership
and retained doors extend the mover schema; displaced ceilings remain in the
ordered mover state. Save version 42 reads versions 39–41;
keyframe version 29 reads versions 26–28. Save 42/keyframe 29 add cast and queued
episode-victory state in attachment version 2. Save 41/keyframe 28 add the recorded
co-op session attachment; older formats cannot reconstruct a complete recorded
roster or campaign cursor. Save 40/keyframe 27 added the pending
rebirth request and death-camera attacker. Previous formats default those to
no request/no attacker; they cannot reconstruct them for exact death-boundary
continuation. Save 39/keyframe 26 introduced ordered sector membership and
horizontal momentum on missile death effects. Formats before those reconstruct
membership and default that effect momentum to zero, so historical ordering on
crusher-sensitive maps cannot be recovered. Save/keyframe magic disambiguates
their overlapping version numbers, including historical save 27 versus
keyframe 27.
Formats before save 38/keyframe 25 default door-lighting references to disabled,
since the activation line was not saved. Formats before
save 35/keyframe 22 cannot preserve overlapping manual-door state. Generalized mover,
elevator, card-key and actor friction extensions retain their older layouts.
Older snapshots initialize previously unsupported friction state to defaults.
Scroller data from version 30/keyframe 17 remains readable; earlier snapshots
cannot reconstruct the preceding acceleration history of unsupported scrollers.
Co-op doll ownership is encoded in internal actor types and existing body
arrays. Full-roster recorded-session checkpoints add the save 41/keyframe 28
attachment. The simulation ID also changes for campaign, sparse pickup and
retained-body sector-link behavior.
