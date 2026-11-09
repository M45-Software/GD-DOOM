#!/usr/bin/env python3
"""Compare supported patched monster states/actions against an observed DSDA build.

Requires the existing dsda_rng_demo_probe.inc observer. Generated Boom demos
exercise actual thinkers, full combat/RNG, and save/keyframe reconstruction.
This is deliberately not a claim of complete DeHackEd or community-map parity.
"""
import argparse
import hashlib
import json
import os
import re
from pathlib import Path
import struct

from verify_boom_rng import fixture, demo, compare
from verify_boom_transfers import ROOT, run, digest, merge_wads

CASES = {
    'baseline': '',
    'durations': 'Frame 174\nDuration = 2\nFrame 175\nDuration = 3\nFrame 176\nDuration = 1\nFrame 177\nDuration = 2\n',
    'shotgun-pointer': 'Thing 2\nHeight = 2097152\n[CODEPTR]\nFRAME 185 = SPosAttack\n',
    'chaingun-pointer': '[CODEPTR]\nFRAME 185 = CPosAttack\n',
    'zero-chain': 'Frame 176\nDuration = 0\nNext frame = 177\n[CODEPTR]\nFRAME 176 = NULL\n',
    'null-removal': 'Frame 174\nDuration = 1\nNext frame = 0\n[CODEPTR]\nFRAME 174 = NULL\n',
    'infinite-spawn': 'Frame 174\nDuration = -1\n[CODEPTR]\nFRAME 174 = SPosAttack\n',
    'missile-disabled': 'Thing 2\nFar attack frame = 0\n',
    'pain-root-redirect': 'Thing 2\nInjury frame = 220\n',
    'pain-chain': 'Frame 187\nDuration = 0\nFrame 188\nDuration = 3\n',
    'classic-pointer': 'Pointer 0 (Frame 185)\nCodep Frame = 218\n',
    'head-pointer': 'Thing 2\nHeight = 2097152\n[CODEPTR]\nFRAME 185 = HeadAttack\n',
    'plasma-pointer': '[CODEPTR]\nFRAME 185 = BspiAttack\n',
    'shotgun-duration': 'Frame 207\nDuration = 2\n',
    'chaingun-duration': 'Frame 406\nDuration = 2\n',
    'caco-duration': 'Frame 502\nDuration = 2\n',
    'skull-duration': 'Frame 585\nDuration = 2\n',
    'arachnotron-duration': 'Frame 632\nDuration = 2\n',
    'spawn-redirect': 'Thing 2\nInitial frame = 207\nFrame 207\nDuration = 2\n',
}

CHAIN = list(range(174, 189)) + [207,208,209]
CASES['long-zero-chain'] = 'Thing 2\nInitial frame = 218\n' + ''.join(
    f'Frame {state}\nDuration = 0\nNext frame = {following}\n'
    for state, following in zip(CHAIN, CHAIN[1:]+[210])) + (
    'Frame 218\nDuration = 1\nNext frame = 174\nFrame 210\nDuration = 2\nNext frame = 210\n[CODEPTR]\n' +
    ''.join(f'FRAME {state} = NULL\n' for state in CHAIN+[218,210]))
TYPES = {'shotgun-duration':9, 'chaingun-duration':65, 'caco-duration':3005, 'skull-duration':3006, 'arachnotron-duration':68}


def append_patches(path, patches):
    raw = path.read_bytes()
    count, offset = struct.unpack_from('<ii', raw, 4)
    payload = bytearray(raw[:offset])
    directory = bytearray(raw[offset:offset + count * 16])
    for patch in patches:
        data = patch.encode('ascii')
        directory += struct.pack('<ii8s', len(payload), len(data), b'DEHACKED')
        payload += data
    struct.pack_into('<4sii', payload, 0, b'PWAD', count + len(patches), len(payload))
    path.write_bytes(payload + directory)


def source_hash():
    h = hashlib.sha256()
    for package in ['doomruntime', 'doomrand', 'demo', 'dehacked', 'runtimecfg', 'launchcatalog', 'netgame']:
        for path in sorted((ROOT / 'internal' / package).glob('*.go')):
            h.update(str(path.relative_to(ROOT)).encode() + b'\0' + path.read_bytes() + b'\0')
    return h.hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad', required=True, type=Path)
    parser.add_argument('--reference', required=True, type=Path)
    parser.add_argument('--out', type=Path, default=ROOT / 'tmp/dehacked-gameplay')
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    iwad, reference = args.iwad.resolve(), args.reference.resolve()
    env = dict(os.environ, SDL_AUDIODRIVER='dummy', GD_RNG_COMBAT_ONLY='1', GD_RNG_FULL_MAP='1', GD_DEHACKED_CAPTURE='1')
    for key in ['GD_RNG_CHECKPOINT_TICS', 'GD_RNG_CHECKPOINT_FORMAT', 'GD_RNG_CAMPAIGN', 'GD_RNG_REBIRTH', 'GD_RNG_CRUSHED_CORPSE', 'GD_DEHACKED_FILES']:
        env.pop(key, None)
    source_before = source_hash()
    capture = out / 'gameplay.test'
    run(['go', 'test', '-c', '-tags', 'integration', '-o', capture, './internal/doomruntime'], out / 'build.log', env)
    results = {}
    for name, patch in CASES.items():
        folder = out / name
        folder.mkdir(exist_ok=True)
        pwad, merged, lmp, deh = (folder / x for x in ['fixture.wad', 'merged.wad', 'fixture.lmp', 'patch.deh'])
        fixture(pwad, TYPES.get(name, 3004))
        # Every non-baseline case also verifies that external patches override
        # both duplicate embedded lumps, rather than only loading the last lump.
        if patch:
            append_patches(pwad, ['Thing 2\nHit points = 40\n', 'Thing 2\nReaction time = 4\n'])
            patch = 'Thing 2\nHit points = 123\nReaction time = 8\n' + patch
        patch = re.sub(r"(?m)^(Thing \d+|Frame \d+|Pointer [^\n]+|\[CODEPTR\])", r"\n\1", patch).lstrip() + "\n"
        deh.write_text(patch)
        merge_wads([iwad, pwad], merged)
        demo(lmp, 1993, 1, attack=name in ('pain-chain','pain-root-redirect'), tics=160)
        case_env = dict(env, GD_RNG_DEMO=str(lmp), GD_RNG_WAD=str(merged), GD_DEHACKED_FILES=str(deh))
        run(['xvfb-run', '-a', reference, '-iwad', iwad, '-file', pwad, '-deh', deh, '-timedemo', lmp, '-nosound', '-nodraw', '-noautoload', '-data', folder / 'reference-data', '-config', folder / 'dsda.cfg'], folder / 'dsda.log', dict(env, GD_DSDA_RNG_DEMO_OUT=str(folder / 'dsda.csv')))
        cases = {}
        for mode in ['baseline', 'save', 'keyframe']:
            target = folder / (mode + '.csv')
            capture_env = dict(case_env, GD_RNG_OUT=str(target), GD_RNG_SUMMARY=str(folder / (mode + '-summary.json')), GD_RNG_CHECKPOINT_LOG=str(folder / (mode + '-checkpoints.json')))
            if mode != 'baseline':
                capture_env.update(GD_RNG_CHECKPOINT_TICS='23,67', GD_RNG_CHECKPOINT_FORMAT=mode)
            run(['xvfb-run', '-a', capture, '-test.run', '^TestBoomRNGDemoCapture$', '-test.count=1'], folder / (mode + '.log'), capture_env)
            expected_checkpoints = [] if mode == 'baseline' else [23, 67]
            if json.loads((folder / (mode + '-checkpoints.json')).read_text()) != expected_checkpoints:
                raise ValueError('checkpoint reconstruction was not observed')
            cases[mode] = compare(target, folder / 'dsda.csv', 160)
            print(name, mode, cases[mode]['matched'], cases[mode]['rows'], flush=True)
        results[name] = dict(checks=cases, wad_sha256=digest(pwad), patch_sha256=digest(deh), demo_sha256=digest(lmp))
    passed = all(check['passed'] for case in results.values() for check in case['checks'].values())
    if source_hash() != source_before:
        raise ValueError('gameplay sources changed during verification')
    checks = [check for case in results.values() for check in case['checks'].values()]
    report = dict(passed=passed, comparisons=len(checks), compared_tics=sum(check['rows'] for check in checks), dependencies={name: digest(ROOT / 'scripts' / name) for name in ['verify_boom_rng.py', 'verify_boom_transfers.py', 'make_boom_legacy_wad.py']}, source_sha256=source_before, iwad_sha256=digest(iwad), reference_sha256=digest(reference), capture_sha256=digest(capture), script_sha256=digest(Path(__file__)), cases=results,
                  scope='Generated Boom 2.02 single-map demos; supported live exact monster states/actions and actor properties, with full observed combat/weapon/sector/RNG columns and checkpoints at commands 23 and 67. Does not establish full DeHackEd, arbitrary state/action, renderer, campaign, or community demo parity.')
    (out / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
    if not passed:
        raise SystemExit('DeHackEd gameplay comparison failed; see results.json')


if __name__ == '__main__':
    main()
