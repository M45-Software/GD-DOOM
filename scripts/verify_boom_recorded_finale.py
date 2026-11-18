#!/usr/bin/env python3
"""Replay the original sl30-5219 fight, intermission and cast against DSDA."""
import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
import csv
import json
import os
from pathlib import Path

from verify_boom_community_coop_campaign import capture, lumps, source_digest
from verify_boom_demos import WADS, fetch
from verify_boom_full_combat import SUNLUST_DEH_SHA256, compare_complete
from verify_boom_sod_campaign import IWAD_SHA
from verify_boom_transfers import ROOT, digest, merge_wads, run

ARCHIVE = ('/files/demos/sunlust/105304/sl30-5219.zip',
           '9f9d349fe055685a44e9b6c904ea5dea230613e97033457c6a304c688f3c556b',
           'sl30-5219.lmp')
DEMO_SHA = 'bb875c536cc6268f59bb2ba9ab17d124b189dc6c88ecae5264afc4ec923eda99'
# First zero-based command of each observed phase, followed by the end marker.
SEGMENTS = ((0, 0, 0), (109871, 1, 0), (111363, 2, 0), (112403, 2, 2))
COMMANDS = 113960


def coverage(phase_file):
    segments = []
    count = 0
    previous = None
    with phase_file.open() as stream:
        for count, row in enumerate(csv.DictReader(stream), 1):
            if int(row['demotic']) != count or row['map'] != 'MAP30':
                raise ValueError('missing command or unexpected map')
            phase = int(row['phase']), int(row['stage'])
            if phase != previous:
                segments.append((count - 1, *phase))
                previous = phase
    if count != COMMANDS or tuple(segments) != SEGMENTS:
        raise ValueError(f'original finale coverage changed: {count} commands, {segments}')
    checkpoints = set()
    starts = [s[0] for s in segments] + [count]
    for start, stop in zip(starts, starts[1:]):
        checkpoints.update((start, (start + stop) // 2, stop - 1))
    return sorted(checkpoints)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad', type=Path, required=True)
    parser.add_argument('--reference', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--timeout', type=int, default=14400)
    parser.add_argument('--jobs', type=int, choices=(1, 2, 3), default=1,
                        help='maximum concurrent replays, each with separate trace files')
    args = parser.parse_args()
    iwad, exe, out = args.iwad.resolve(), args.reference.resolve(), args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    result_file = out / 'results.json'
    result_file.unlink(missing_ok=True)
    if digest(iwad) != IWAD_SHA:
        parser.error('use the original Doom II 1.9 IWAD')
    wad = fetch(out, 'sunlust.zip', *WADS['sunlust'])
    demo = fetch(out, 'sl30-5219.zip', *ARCHIVE)
    if digest(demo) != DEMO_SHA:
        raise ValueError('original LMP or footer changed')
    deh = out / 'sunlust.deh'
    deh.write_bytes(next(data for (_, name), data in lumps(wad).items() if name == 'DEHACKED'))
    if digest(deh) != SUNLUST_DEH_SHA256:
        raise ValueError('unexpected Sunlust patch')
    merged = out / 'merged.wad'
    merge_wads([iwad, wad], merged)
    source = source_digest()
    helper_names = ('verify_boom_recorded_finale', 'verify_boom_community_coop_campaign',
                    'verify_boom_full_combat', 'verify_boom_demos',
                    'verify_boom_sod_campaign', 'verify_boom_transfers')
    def verifier_hashes():
        return {name: digest(ROOT / 'scripts' / (name + '.py')) for name in helper_names}
    verifiers = verifier_hashes()
    env = {k: v for k, v in os.environ.items()
           if not k.startswith(('GD_COOP_', 'GD_DSDA_', 'GD_DEHACKED_', 'GD_RNG_', 'GD_DEBUG_'))}
    env.update(SDL_AUDIODRIVER='dummy', GD_COOP_CAMPAIGN='1', GD_COOP_COMPACT='1',
               GD_COOP_COMBAT_ONLY='1', GD_COOP_LINES='1', GD_COOP_COLD_RESTORE='1')
    reference = {kind: out / f'dsda-{kind}.csv' for kind in ('world', 'phase')}
    inputs = {p.name: digest(p) for p in (exe, iwad, wad, demo, deh)}
    observers = {p.name: digest(p) for p in (ROOT / 'scripts/visual_compare').glob('dsda_*probe.inc')}
    manifest = out / 'reference.json'
    cached = json.loads(manifest.read_text()) if manifest.exists() else {}
    identity = dict(inputs=inputs, observers=observers)
    if (cached.get('identity') != identity or
            not all(p.exists() and cached.get('traces', {}).get(k) == digest(p)
                    for k, p in reference.items())):
        run(['xvfb-run', '-a', exe, '-iwad', iwad, '-file', wad, '-nodeh', '-deh', deh,
             '-timedemo', demo, '-nosound', '-nodraw', '-noautoload', '-config', out / 'dsda.cfg'],
            out / 'dsda.log', dict(env, GD_DSDA_COOP_OUT=str(reference['world']),
                                   GD_DSDA_PHASE_OUT=str(reference['phase'])), timeout=args.timeout)
        coverage(reference['phase'])
        manifest.write_text(json.dumps(dict(identity=identity, traces={k: digest(p) for k, p in reference.items()}), indent=2) + '\n')
    checkpoints = coverage(reference['phase'])
    binary = out / 'campaign.test'
    run(['go', 'test', '-c', '-tags', 'integration', '-o', binary, './internal/doomruntime'],
        out / 'build.log', env)
    report = dict(passed=False, source=source, verifiers=verifiers, binary=digest(binary),
                  inputs=inputs, observers=observers, commands=COMMANDS, segments=SEGMENTS,
                  cold_checkpoints=checkpoints, cases={})
    def replay_mode(mode):
        actual = {kind: out / f'{mode}-{kind}.csv' for kind in reference}
        for path in actual.values():
            path.unlink(missing_ok=True)
        replay = dict(env, GD_COOP_DEMO=str(demo), GD_COOP_WAD=str(merged),
                      GD_DEHACKED_CAPTURE='1', GD_DEHACKED_NO_EMBEDDED='1', GD_DEHACKED_FILES=str(deh),
                      GD_COOP_OUT=str(actual['world']), GD_COOP_PHASE_OUT=str(actual['phase']),
                      GD_COOP_CHECKPOINT_FORMAT=mode,
                      GD_COOP_CHECKPOINT_TICS='' if mode == 'baseline' else ','.join(map(str, checkpoints)))
        execution = capture(['xvfb-run', '-a', binary, '-test.run', '^TestBoomCoopDemoCapture$',
                             '-test.count=1', f'-test.timeout={args.timeout}s'],
                            out / f'{mode}.log', replay, actual, reference, args.timeout)
        case = dict(passed=False, execution=execution)
        if execution['returncode'] == 0:
            for kind, rows in [('phase', COMMANDS), ('world', SEGMENTS[1][0])]:
                case[kind] = compare_complete(actual[kind], reference[kind], rows)
            case['passed'] = case['world']['passed'] and case['phase']['passed']
        if source_digest() != source or verifier_hashes() != verifiers:
            raise ValueError('sources changed during replay')
        return case

    modes = ('baseline', 'save', 'keyframe')
    with ThreadPoolExecutor(max_workers=args.jobs) as pool:
        futures = {pool.submit(replay_mode, mode): mode for mode in modes}
        for future in as_completed(futures):
            mode = futures[future]
            case = report['cases'][mode] = future.result()
            report['passed'] = len(report['cases']) == len(modes) and all(
                c['passed'] for c in report['cases'].values())
            result_file.write_text(json.dumps(report, indent=2) + '\n')
            print(mode, case['execution'], flush=True)
    if not report['passed']:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
