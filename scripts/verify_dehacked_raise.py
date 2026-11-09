#!/usr/bin/env python3
"""Require native-observed death and resurrection of each patched raisable actor.

The player fires for fifty commands, then stops. A non-solid Keen waits 150
tics before running VileChase. Observers only read state; no injected damage,
forced corpse state, or forced resurrection is used.
"""
import argparse
import csv
import json
import os
from pathlib import Path
import struct

from verify_boom_rng import fixture, demo, compare
from verify_boom_transfers import ROOT, run, digest, merge_wads
from verify_boom_community_coop_campaign import source_digest
from verify_dehacked_actors import ACTORS

RAISABLE = ('zombie', 'shotgun', 'revenant', 'mancubus', 'chaingun', 'imp',
            'demon', 'spectre', 'caco', 'baron', 'knight', 'arach', 'pain', 'wolf')


def add_healer(path):
    raw = path.read_bytes()
    count, directory = struct.unpack_from('<ii', raw, 4)
    payload, entries = bytearray(12), bytearray()
    for i in range(count):
        start, size, name = struct.unpack_from('<ii8s', raw, directory + 16 * i)
        data = raw[start:start + size]
        if name.rstrip(b'\0') == b'THINGS':
            data += struct.pack('<5h', 28, -48, 0, 72, 7)
        entries += struct.pack('<ii8s', len(payload), len(data), name)
        payload += data
    struct.pack_into('<4sii', payload, 0, b'PWAD', count, len(payload))
    path.write_bytes(payload + entries)


def observed_raise(path, typ):
    dead = False
    with path.open() as stream:
        for row in csv.DictReader(stream):
            for actor in row['mobjs'].strip(';').split(';'):
                fields = actor.split(':')
                if len(fields) < 9 or int(fields[0]) != typ:
                    continue
                health = int(fields[8])
                if dead and health > 0:
                    return int(row['tic'])
                dead |= health <= 0
    raise ValueError('fixture did not actually resurrect its victim')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad', required=True, type=Path)
    parser.add_argument('--reference', required=True, type=Path)
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--case', action='append', choices=RAISABLE)
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    iwad, reference = args.iwad.resolve(), args.reference.resolve()
    env = {k: v for k, v in os.environ.items() if not k.startswith(
        ('GD_RNG_', 'GD_DSDA_', 'GD_COOP_', 'GD_DEHACKED_', 'GD_DEBUG_'))}
    env.update(SDL_AUDIODRIVER='dummy', GD_RNG_COMBAT_ONLY='1', GD_RNG_FULL_MAP='1',
               GD_DEHACKED_CAPTURE='1', GD_RNG_DEHACKED_ACTORS='1')
    source, verifier, native_binary = source_digest(), digest(Path(__file__)), digest(reference)
    capture = out / 'raise.test'
    run(['go', 'test', '-c', '-tags', 'integration', '-o', capture, './internal/doomruntime'], out / 'build.log', env)
    results = {}
    for name in args.case or RAISABLE:
        editor, thing, state = ACTORS[name]
        folder = out / name
        folder.mkdir(exist_ok=True)
        pwad, merged, deh, lmp = (folder / x for x in ('fixture.wad', 'merged.wad', 'patch.deh', 'fixture.lmp'))
        fixture(pwad, editor)
        add_healer(pwad)
        patch = f'Thing {thing}\nInitial frame = {state}\nHit points = 10\nSpeed = 0\n\n'
        # The original pain elemental disappears at the end of its death
        # chain. Explicitly retain its last pose to exercise its raise root.
        if name == 'pain':
            patch += 'Frame 719\nDuration = -1\n\n'
        deh.write_text(patch +
                       'Thing 25\nBits = 0\nSpeed = 0\n\n'
                       'Frame 763\nDuration = 1\nNext frame = 764\n\n'
                       'Frame 764\nDuration = 150\nNext frame = 765\n\n'
                       'Frame 765\nDuration = 1\nNext frame = 765\n\n'
                       '[CODEPTR]\nFRAME 763 = NULL\nFRAME 764 = NULL\nFRAME 765 = VileChase\n')
        merge_wads([iwad, pwad], merged)
        demo(lmp, 1993, 1, attack=True, attack_tics=50, tics=480)
        case_env = dict(env, GD_RNG_WAD=str(merged), GD_RNG_DEMO=str(lmp),
                        GD_DEHACKED_FILES=str(deh), GD_DEHACKED_FLAG_TYPES=f'{thing-1},24')
        run(['xvfb-run', '-a', reference, '-iwad', iwad, '-file', pwad, '-deh', deh,
             '-timedemo', lmp, '-nosound', '-nodraw', '-noautoload',
             '-config', folder / 'dsda.cfg', '-data', folder / 'reference-data'],
            folder / 'dsda.log', dict(case_env, GD_DSDA_RNG_DEMO_OUT=str(folder / 'dsda.csv')))
        native_raise = observed_raise(folder / 'dsda.csv', thing - 1)
        checkpoints = sorted({23, 150, native_raise + 1, 211})
        checks = {}
        for mode in ('baseline', 'save', 'keyframe'):
            target = folder / (mode + '.csv')
            capture_env = dict(case_env, GD_RNG_OUT=str(target),
                               GD_RNG_SUMMARY=str(folder / (mode + '-summary.json')),
                               GD_RNG_CHECKPOINT_LOG=str(folder / (mode + '-checkpoints.json')))
            if mode != 'baseline':
                capture_env.update(GD_RNG_CHECKPOINT_TICS=','.join(map(str, checkpoints)),
                                   GD_RNG_CHECKPOINT_FORMAT=mode)
            run(['xvfb-run', '-a', capture, '-test.run', '^TestBoomRNGDemoCapture$',
                 '-test.count=1'], folder / (mode + '.log'), capture_env)
            if json.loads((folder / (mode + '-checkpoints.json')).read_text()) != ([] if mode == 'baseline' else checkpoints):
                raise ValueError('checkpoint reconstruction was not observed')
            checks[mode] = compare(target, folder / 'dsda.csv', 480)
            print(name, mode, checks[mode]['matched'], checks[mode]['rows'], flush=True)
        results[name] = dict(checks=checks, native_resurrection_tic=native_raise,
                             wad_sha256=digest(pwad), patch_sha256=digest(deh), demo_sha256=digest(lmp))
    if source_digest() != source or digest(Path(__file__)) != verifier or digest(reference) != native_binary:
        raise ValueError('sources/verifier/reference changed during verification')
    checks = [c for case in results.values() for c in case['checks'].values()]
    passed = all(c['passed'] for c in checks)
    report = dict(passed=passed, comparisons=len(checks), compared_tics=sum(c['rows'] for c in checks),
                  cases=results, source_sha256=source, reference_sha256=digest(reference),
                  iwad_sha256=digest(iwad), capture_sha256=digest(capture), script_sha256=digest(Path(__file__)),
                  observer_sha256=digest(ROOT / 'scripts/visual_compare/dsda_rng_demo_probe.inc'),
                  dependencies={name: digest(ROOT / 'scripts' / name) for name in
                                ('verify_dehacked_actors.py', 'verify_boom_rng.py', 'verify_boom_transfers.py',
                                 'verify_boom_community_coop_campaign.py', 'make_boom_legacy_wad.py')}, scope=__doc__)
    (out / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
    if not passed:
        raise SystemExit('resurrection comparison failed; see results.json')


if __name__ == '__main__':
    main()
