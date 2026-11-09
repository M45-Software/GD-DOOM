#!/usr/bin/env python3
"""Compare canonical patched monster thinkers/deaths and cold checkpoints to DSDA.

Fixtures explicitly select each actor's original spawn root, so the patched
state executor runs even though the selected value equals the original value.
The death cases lower health and fire the player's pistol; no observer mutates
gameplay. This is a synthetic actor check, not community-demo evidence.
"""
import argparse
import json
import os
from pathlib import Path

from verify_boom_rng import fixture, demo, compare
from verify_boom_transfers import ROOT, run, digest, merge_wads
from verify_boom_community_coop_campaign import source_digest as source_hash

# editor number, one-based DeHackEd Thing number, original spawn state
ACTORS = {
    'zombie': (3004, 2, 174), 'shotgun': (9, 3, 207),
    'vile': (64, 4, 241), 'revenant': (66, 6, 321),
    'mancubus': (67, 9, 362), 'chaingun': (65, 11, 406),
    'imp': (3001, 12, 442), 'demon': (3002, 13, 475),
    'spectre': (58, 14, 475), 'caco': (3005, 15, 502),
    'baron': (3003, 16, 527), 'knight': (69, 18, 556),
    'skull': (3006, 19, 585), 'spider': (7, 20, 601),
    'arach': (68, 21, 632), 'cyber': (16, 22, 674),
    'pain': (71, 23, 701), 'wolf': (84, 24, 726),
    'keen': (72, 25, 763), 'barrel': (2035, 31, 806),
}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad', required=True, type=Path)
    parser.add_argument('--reference', required=True, type=Path)
    parser.add_argument('--out', type=Path, default=ROOT / 'tmp/dehacked-actors')
    parser.add_argument('--case', action='append', choices=list(ACTORS))
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    iwad, reference = args.iwad.resolve(), args.reference.resolve()
    env = {k: v for k, v in os.environ.items()
           if not k.startswith(('GD_RNG_', 'GD_DSDA_', 'GD_DEHACKED_', 'GD_COOP_', 'GD_DEBUG_'))}
    env.update(SDL_AUDIODRIVER='dummy', GD_RNG_COMBAT_ONLY='1', GD_RNG_FULL_MAP='1',
               GD_DEHACKED_CAPTURE='1', GD_RNG_DEHACKED_ACTORS='1')
    source, verifier, native_binary = source_hash(), digest(Path(__file__)), digest(reference)
    capture = out / 'actors.test'
    run(['go', 'test', '-c', '-tags', 'integration', '-o', capture,
         './internal/doomruntime'], out / 'build.log', env)
    results = {}
    for name in args.case or ACTORS:
        editor, thing, state = ACTORS[name]
        variants = [(False, False), (True, False)]
        if name in ('demon', 'spectre'):
            variants += [(False, True), (True, True)]
        for death, fast in variants:
            label = name + ('-death' if death else '-live') + ('-fast' if fast else '')
            folder = out / label
            folder.mkdir(exist_ok=True)
            pwad, merged, lmp, deh = (folder / x for x in
                                      ('fixture.wad', 'merged.wad', 'fixture.lmp', 'patch.deh'))
            fixture(pwad, editor)
            patch = f'Thing {thing}\nInitial frame = {state}\n'
            if death:
                patch += 'Hit points = 10\n'
            deh.write_text(patch + '\n')
            merge_wads([iwad, pwad], merged)
            demo(lmp, 1993, 1, attack=death, tics=320, fast=fast)
            case_env = dict(env, GD_DEHACKED_FLAG_TYPES=str(thing - 1),
                            GD_DEHACKED_FILES=str(deh), GD_RNG_WAD=str(merged),
                            GD_RNG_DEMO=str(lmp))
            run(['xvfb-run', '-a', reference, '-iwad', iwad, '-file', pwad,
                 '-deh', deh, '-timedemo', lmp, '-nosound', '-nodraw', '-noautoload',
                 '-data', folder / 'reference-data', '-config', folder / 'dsda.cfg'],
                folder / 'dsda.log', dict(case_env, GD_DSDA_RNG_DEMO_OUT=str(folder / 'dsda.csv')))
            checks = {}
            for mode in ('baseline', 'save', 'keyframe'):
                target = folder / (mode + '.csv')
                capture_env = dict(case_env, GD_RNG_OUT=str(target),
                                   GD_RNG_SUMMARY=str(folder / (mode + '-summary.json')),
                                   GD_RNG_CHECKPOINT_LOG=str(folder / (mode + '-checkpoints.json')))
                if mode != 'baseline':
                    capture_env.update(GD_RNG_CHECKPOINT_TICS='23,67,211',
                                       GD_RNG_CHECKPOINT_FORMAT=mode)
                run(['xvfb-run', '-a', capture, '-test.run', '^TestBoomRNGDemoCapture$',
                     '-test.count=1'], folder / (mode + '.log'), capture_env)
                if json.loads((folder / (mode + '-checkpoints.json')).read_text()) != ([] if mode == 'baseline' else [23, 67, 211]):
                    raise ValueError('checkpoint reconstruction was not observed')
                checks[mode] = compare(target, folder / 'dsda.csv', 320)
                print(label, mode, checks[mode]['matched'], checks[mode]['rows'], flush=True)
            results[label] = dict(checks=checks, wad_sha256=digest(pwad),
                                 patch_sha256=digest(deh), demo_sha256=digest(lmp))
    if source_hash() != source or digest(Path(__file__)) != verifier or digest(reference) != native_binary:
        raise ValueError('sources/verifier/reference changed during verification')
    checks = [check for case in results.values() for check in case['checks'].values()]
    passed = all(check['passed'] for check in checks)
    report = dict(passed=passed, comparisons=len(checks), compared_tics=sum(c['rows'] for c in checks),
                  source_sha256=source, iwad_sha256=digest(iwad), reference_sha256=digest(reference),
                  capture_sha256=digest(capture), script_sha256=digest(Path(__file__)),
                  observer_sha256=digest(ROOT / 'scripts/visual_compare/dsda_rng_demo_probe.inc'),
                  dependencies={name: digest(ROOT / 'scripts' / name) for name in
                                ('verify_boom_rng.py', 'verify_boom_transfers.py', 'make_boom_legacy_wad.py', 'verify_boom_community_coop_campaign.py')},
                  cases=results, scope=__doc__)
    (out / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
    if not passed:
        raise SystemExit('actor comparison failed; see results.json')


if __name__ == '__main__':
    main()
