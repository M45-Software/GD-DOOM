# COMPET-N demo compatibility

GD-DOOM uses COMPET-N speedrun recordings to check its gameplay against the
original Doom engine. The completed sweep covers **4,646 unique single-player
recordings** from Ultimate Doom and Doom II. Results were verified on
**2026-10-02** against the gameplay source shipped in **v0.1.1**.

## Results

| Outcome | Recordings | Share of corpus |
| --- | ---: | ---: |
| Strict state and gameplay RNG match | 4,621 | 99.46% |
| Semantic state and gameplay RNG match only | 7 | 0.15% |
| Documented exclusions | 18 | 0.39% |
| Pending or unexplained failures | 0 | 0.00% |
| Total | 4,646 | 100.00% |

**4,628 recordings (99.61%) pass** with the limited semantic policy described
below. Exclusions count in the denominator and never count as passes. All
eligible inputs are accounted for; passing recordings were checked on the
current gameplay source rather than credited from older runtime results.

The [machine-readable results](COMPET-N-results.json) retain each recording's
SHA-256, archive provenance, compared tic count, strict outcome, independent
gameplay RNG audit, and raw-result hash. Semantic results are recorded
separately. The [exclusion guide](COMPET-N-exclusions.md) explains the eighteen
remaining cases and the compatibility fixes that resolved earlier exclusions.

These results apply to this snapshot and these replay windows. They do not
establish compatibility with every Doom demo, modified WAD, or original DOS
memory-corruption behavior.

## Corpus

The inputs come from the [COMPET-N public archive](https://compet-n.gamers.org/public/compet-n/),
using its [2019-01-21 snapshot](https://compet-n.gamers.org/public/compet-n/compet-n_2019-01-21.zip).
The importer verifies the snapshot's pinned SHA-256 before reading its demos.

| Game | Unique recordings | Starting maps | Required IWAD |
| --- | ---: | --- | --- |
| Ultimate Doom | 2,411 | E1M1–E4M9, all 36 maps | `wads/DOOMU.WAD` |
| Doom II | 2,235 | MAP01–MAP32, all 32 maps | `wads/DOOM2.WAD` |

The nine categories are UV Speed, UV Max, Nightmare Speed, Nightmare 100%
Secrets, Tyson, Pacifist, UV Fast, UV Respawn, and No Monsters. The command
streams contain **238.99 recorded hours**; this describes input duration,
not execution time or the duration successfully compared.

Byte-identical recordings within each game are deduplicated, while category
and archive-member provenance are retained. All LMP members are inspected,
including extra attempts inside an archive. Twelve recordings require legacy
ZIP Implode or ARJ decoding; the complete import includes them, verifies their
size and CRC, and has **zero skipped archive entries**.

The selected corpus contains single-player version-109 demos for the two
listed IWADs. Co-op, multiplayer, other IWADs/PWADs, built/miscellaneous
categories, and multi-level movies are outside the selection. Newer archive
submissions are outside this snapshot.

## How comparison works

The harness replays identical inputs in a trace-enabled original Linux Doom
1.10 reference and GD-DOOM, then compares their normalized state at every tic.
It separately checks the gameplay random-number index (`prndindex`) at every
compared tic. Input, runtime, tool, configuration, and result hashes identify
the evidence used for the sweep.

Each comparison ends when the original replay terminates or at the first
player death, **including the death tic**. A pass makes no claim about input
after that boundary. The reference uses an isolated configuration with a
16 MiB memory zone, so existing local reference settings do not affect a run.
See the [harness guide](../desync-harness.md) for setup and trace details.

Strict comparison remains the default, using the comparator's documented
normalization of trace-only differences. The optional semantic policy ignores
only `topheight` when **both** compared records are ceilings of type 2
(`lowerAndCrush`) moving downward (`direction: -1`). Original Doom leaves this
field uninitialized, and that downward action never reads it. Every other
field, the trace length, and gameplay RNG still have to match.

Thirteen recordings exercise this ceiling case. In this sweep six matched
strictly and seven required semantic comparison. The strict split can vary
with the reference allocator's unused contents. Raw strict mismatches are
preserved; semantic matches never become strict matches. A separate check of
thirteen retained nonzero-residue captures also passed this semantic policy
and contributed no additional corpus passes.

## Reproduce a comparison

Use the Go and native dependencies listed in the [README](../README.md), a
trace-enabled original reference executable, and legally obtained copies of
the required IWADs. Install `7z` or `7zz` for the twelve legacy archives.
Generated inputs, manifests, logs, and traces stay in ignored local output
directories; commercial IWADs and the external demo pack are not bundled here.

From the repository root:

```bash
python3 -B scripts/fetch_compet_n.py

# Start with one recording per map, using the default strict comparison.
xvfb-run -a python3 -B scripts/demo_trace_compare_all.py \
  --manifest tmp/compet-n/smoke.json --jobs 2 \
  --discard-matching-traces --compress-retained-traces \
  --out-root tmp/compet-n-smoke

# Run the full inventory with the optional unused-ceiling-field policy.
xvfb-run -a python3 -B scripts/demo_trace_compare_all.py \
  --manifest tmp/compet-n/manifest.json --jobs 2 \
  --semantic-unused-ceiling-fields \
  --discard-matching-traces --compress-retained-traces \
  --out-root tmp/compet-n-full
```

The importer also accepts `--archive PATH` to reuse a downloaded snapshot.
It produces `smoke.json` (one recording per starting map), `combat.json`
(one UV Max recording per map), and `manifest.json` (the full inventory).
Confirm the importer reports 4,646 recordings and zero skipped entries before
claiming complete coverage. Omitting `--manifest` runs the 25 repository demos.
Use `--ref-bin PATH` to override the default reference executable at
`../doom-source/linuxdoom-1.10/linux/linuxxdoom`.

The runner writes per-recording logs and results, plus `summary.json` and
`summary.tsv` when the selection finishes. Matching traces may be discarded
after the RNG audit; mismatch traces remain available for diagnosis.
`--compress-retained-traces` compresses complete retained traces without
dropping tics. Large traces need substantial disk space, so begin with a small
selection and a modest worker count.

An excluded original-reference crash still appears as an error in a new run;
the runner does not turn exclusions into successful comparisons. Classify
results against the exact inputs in the exclusion guide.

## Additional validation

All **101 ordinary validation fixtures**, including the 25 repository demos,
matched state and gameplay RNG on the same gameplay source. All thirteen
door/platform follow-ups now match strictly. The full Go suite, fourteen
Python corpus checks, and release CI/vulnerability checks also passed.
Validation fixtures can overlap the external corpus and do not increase its
4,646-recording total.

The prior completed runtime baseline had 4,602 strict matches and 44 exclusions.
The current sweep supersedes that coverage, resolves all thirteen platform
cases, and separates the thirteen unused-ceiling cases from the remaining
eighteen exclusions. Historical raw captures and operational audit receipts
are retained locally; public results contain portable technical identifiers
without personal paths, host details, or conversation excerpts.

## Post-Boom vanilla regression audit (2026-11-09)

A targeted audit of the Boom branch attempted **187 recordings** against the
original Linux Doom 1.10 reference: **186 strict state and gameplay RNG matches**,
one previously documented reference-crash exclusion, and zero unexplained
failures. The successful comparisons cover **782,366 tics** through original
termination or first player death, including the death tic. This is a new
regression selection; the full 4,646-recording sweep above remains evidence for
v0.1.1, dated 2026-10-02.

The selection includes all 101 ordinary validation fixtures (including the
25 repository demos and shareware fixtures), 68 additional per-map UV-Max
recordings, 17 further samples covering all nine COMPET-N categories in both
commercial games, and an E4M8 Max follow-up. All 68 Ultimate Doom/Doom II
starting maps have successful UV-Max comparisons. The bonus
`DOOMU-E4M8-U48GLICH-bdc56641f6c7.lmp` crashes the original reference before
GD-DOOM playback; its exact SHA matches the existing exclusion. It retains
status `error` and has no state/RNG pass. The additional original
`DOOMU-E4M8-U4M8-219-be97b0d74754.lmp` matches strictly.

The audit found and fixed vanilla behavior affected by the Boom work:

- Revenant homing missiles in vanilla demos use the recorded gametic, including
  pause commands. Boom retains its completed-world-tic/basetic phase. The
  recorded-pause MAP29 regression now matches all 14,407 compared tics.
- Episode finales write one trace frame per recorded command.
- Vanilla episode finales ignore recorded buttons and hold the ending art
  until the recording ends, preserving original Doom's finale behavior.

Regression tests cover the differing vanilla/Boom tracer phases, duplicate
finale frames, held finale buttons, and the art hold beyond the interactive
frontend's timeout. Gameplay, app, networking, RNG and campaign Go tests and
all fourteen Python corpus checks pass. On the same source, the full original
32-map Speed of Doom baseline still matches DSDA: 341,051 commands and 339,681
active-world frames. That additional Boom check is a baseline replay; this
vanilla audit does not add a new cold-checkpoint proof.

The [portable per-recording report](vanilla-boom-regression-results.json)
records source, engine, IWAD, harness and result hashes. It also serves as a
replay manifest after importing the complete corpus into `tmp/compet-n` and
placing the IWADs at the standard paths:

```bash
xvfb-run -a python3 -B scripts/demo_trace_compare_all.py \
  --manifest demos/vanilla-boom-regression-results.json --jobs 4 \
  --semantic-unused-ceiling-fields \
  --discard-matching-traces --compress-retained-traces \
  --out-root tmp/vanilla-boom-regression
```

No recording in this audit required the optional unused-ceiling policy. The
known original-reference crash still produces an error and a nonzero suite
exit; classify it separately by its exact input SHA, as in the existing
[exclusion guide](COMPET-N-exclusions.md).

## Full-corpus runner and desktop notification

`scripts/verify_compet_n_full.py` validates the exact 4,646-recording inventory
against the published input identities, including the twelve recovered legacy
recordings. It builds and pins the current engine, copies and verifies the
reference, IWADs and demos, then replays every input. The historical pass totals
are not reused. Start with the complete importer manifest and original IWADs:

```bash
python3 -B scripts/verify_compet_n_full.py \
  --manifest tmp/compet-n/manifest.json --wad-root wads \
  --reference ../doom-source/linuxdoom-1.10/linux/linuxxdoom \
  --out tmp/compet-n-full --jobs 12 --notify
```

On Linux, `--notify` sends a persistent `notify-send` desktop notification when
all attempts finish, including counts of strict matches, semantic-only matches,
documented exclusions and unexplained failures. An incomplete/stopped run gets
a separate notification. The run can also be launched as a user service with
`systemd-run --user` to continue independently of the terminal or chat.

The output directory contains `replays/progress.json` for live counts,
`replay.log`, immutable input/source receipts, per-demo results and retained
compressed mismatch traces. Final classified counts are in `results.json`;
notification delivery status is in `notification.json`. Existing exclusions
are accepted only for the same input SHA and documented failure signature.
They never count as passes, and unexpected failures remain visible.

To resume, rerun the same command with `--resume`. Completed attempts are reused
only when the pinned engine, inputs, tools, selection and comparison policy
match. Interrupted attempts run again. Error results remain errors; remove a
specific attempt's `replays/DEMO-STEM/result.json` before resuming to explicitly
retry it. Source changes requiring a new engine need a fresh output directory.

## MAP27 countdown follow-up (2026-11-09)

The full post-Boom audit completed all 4,646 attempts on source
`95e3cd1c4a996390957b0b4acb0a0432c3385eeabfa86b0a8470bfb182bb495a`:
4,621 strict matches, seven semantic-only matches, seventeen documented
reference crashes, and one flagged MAP27 door mismatch. That last recording
is the known uninitialized-door case, but this capture first diverged in
`direction` at tic 1084 rather than the historical `topheight` signature.
The [complete per-recording audit](vanilla-boom-full-results.json) preserves
every original outcome and links the later targeted fix separately.

The zero-valued reference fields exposed an additional countdown bug in
GD-DOOM: waiting doors changed state after the counter became negative.
Original Doom and DSDA change state only when the decrement reaches exactly
zero. Both ordinary waiting and five-minute initial waiting now use that
condition. Regression tests cover positive, zero and negative counts in
vanilla and Boom modes, and the runtime/app/network Go suites pass.

On the corrected source, a fresh strict replay of the complete MAP27 recording
matches **all 2,244 tics and gameplay RNG**. The
[follow-up receipt](map27-door-regression-results.json) pins the new source,
engine, reference, IWAD and raw-result hashes. This targeted follow-up does not
relabel the earlier full-corpus run or establish parity with arbitrary original
C allocator contents; see the [timed-door exclusion](COMPET-N-exclusions.md).
