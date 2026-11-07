#!/usr/bin/env python3
"""Compare real Boom normal/secret exits and next-map continuation with DSDA.

Both engines run recorded commands through their real session/intermission
logic. Compare active level tics plus their positions in the original command
stream, so a different intermission duration cannot silently align the traces.
"""
import argparse
import csv
import hashlib
import itertools
import json
import os
from pathlib import Path
import struct

from verify_boom_rng import demo, fixture
from verify_boom_transfers import ROOT, build_reference, digest, merge_wads, run


def make_fixture(path, secret):
    fixture(path, 9)
    raw = path.read_bytes()
    count, directory = struct.unpack_from('<ii', raw, 4)
    lumps, second = [], False
    for i in range(count):
        start, size, name = struct.unpack_from('<ii8s', raw, directory + i * 16)
        data = raw[start:start + size]
        label = name.rstrip(b'\0')
        if label == b'MAP01' and secret:
            name = b'MAP15'
        if label == b'MAP02':
            second = True
            if secret:
                name = b'MAP31'
        if label == b'THINGS':
            things = [(-256, -48, 0, 1, 7), (-320, 48, 0, 9, 7)]
            things += [(-256, -48, 0, 2022, 7)] if second else [
                (-256, -48, 0, 2001, 7), (-256, -48, 0, 2007, 7), (-256, -48, 0, 5, 7)]
            data = b''.join(struct.pack('<5h', *thing) for thing in things)
        if label == b'LINEDEFS':
            rows = [list(line) for line in struct.iter_unpack('<7H', data)]
            for row in rows:
                row[3:5] = [0, 0]
            if not second:
                rows[2][3] = 124 if secret else 52
            data = b''.join(struct.pack('<7H', *row) for row in rows)
        if label == b'SECTORS':
            data = bytearray(data)
            struct.pack_into('<h', data, 22, 0 if second else 7)
        lumps.append((name, data))
    payload, entries = bytearray(12), bytearray()
    for name, data in lumps:
        entries += struct.pack('<ii8s', len(payload), len(data), name)
        payload += data
    struct.pack_into('<4sii', payload, 0, b'PWAD', len(lumps), len(payload))
    path.write_bytes(payload + entries)


def compare(actual, expected):
    with actual.open() as f:
        a = list(csv.DictReader(f))
    with expected.open() as f:
        b = list(csv.DictReader(f))
    matched, failures = 0, []
    for row, (x, y) in enumerate(itertools.zip_longest(a, b)):
        if x is None or y is None:
            diff = {'missing_row': dict(gd=x is None, dsda=y is None)}
        else:
            diff = {k: dict(gd=x[k], dsda=y[k]) for k in y if x[k] != y[k]}
        matched += not diff
        if diff and len(failures) < 6:
            failures.append(dict(row=row, diff=diff))
    return dict(passed=not failures and bool(b), rows=max(len(a), len(b)), matched=matched,
                row_counts=dict(gd=len(a), dsda=len(b)), failures=failures,
                gd_hash=digest(actual), dsda_hash=digest(expected))


def coverage(path, secret, held=False):
    with path.open() as f:
        rows = list(csv.DictReader(f))
    first = [r for r in rows if r['map'] == ('MAP15' if secret else 'MAP01')]
    second = [r for r in rows if r['map'] == ('MAP31' if secret else 'MAP02')]
    checks = dict(first_map=bool(first), next_map=bool(second))
    if first and second:
        a, b = first[-1], second[0]
        checks.update(command_gap=int(b['demotic']) > int(a['demotic']) + 1,
                      new_leveltime=b['leveltime'] == '1', reset_rng_phase=b['logictic'] == '1',
                      streams_continue=b['seeds'] != first[0]['seeds'],
                      health_carried=b['health'] == a['health'] and 0 < int(b['health']) < 100,
                      ammo_carried=all(b[k] == a[k] for k in ['ammo', 'shells', 'cells', 'rockets']),
                      keys_cleared=int(a['cards']) != 0 and int(b['cards']) == 0,
                      next_map_stepped=int(second[-1]['leveltime']) > 100,
                      next_map_has_monster=any(r['mobjs'] for r in second),
                      shotgun_reselected=any(int(r['demotic']) > (1270 if held else 310) and r['ready'] == '2' and r['attackdown'] == '1' for r in second))
    return dict(passed=all(checks.values()), checks=checks,
                last_exit_map_command=int(first[-1]['demotic']) if first else None,
                first_next_map_command=int(second[0]['demotic']) if second else None)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad', required=True, type=Path)
    ref = parser.add_mutually_exclusive_group(required=True)
    ref.add_argument('--reference', type=Path)
    ref.add_argument('--reference-source', type=Path)
    parser.add_argument('--cmake-arg', action='append', default=[])
    parser.add_argument('--out', type=Path, default=ROOT / 'tmp/boom-transitions')
    parser.add_argument('--case', action='append')
    args = parser.parse_args()
    cases = {f'{kind}_i{insurance}_s{seed}': dict(secret=kind == 'secret', insurance=insurance, seed=seed)
             for kind in ['normal', 'secret'] for insurance in [0, 1] for seed in [0, 1993]}
    cases['secret_held_i1_s1993'] = dict(secret=True, insurance=1, seed=1993, held=True)
    if args.case and set(args.case) - cases.keys():
        parser.error('unknown case')
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    exe = args.reference.resolve() if args.reference else build_reference(args.reference_source, out, args.cmake_arg)
    env = dict(os.environ, SDL_AUDIODRIVER='dummy', GD_RNG_COMBAT_ONLY='1', GD_RNG_REBIRTH='1', GD_RNG_CAMPAIGN='1')
    for key in list(env):
        if key.startswith(('GD_DSDA_', 'GD_RNG_CHECKPOINT_')) or key in ['GD_RNG_FULL_MAP', 'GD_RNG_CRUSHED_CORPSE']:
            env.pop(key)
    results = {}
    for name, settings in cases.items():
        if args.case and name not in args.case:
            continue
        folder = out / name
        folder.mkdir(exist_ok=True)
        pwad, lmp, merged = [folder / n for n in ['campaign.wad', 'campaign.lmp', 'merged.wad']]
        make_fixture(pwad, settings['secret'])
        merge_wads([args.iwad.resolve(), pwad], merged)
        held = settings.get('held', False)
        tics = 1500 if held else 600
        demo(lmp, settings['seed'], settings['insurance'], attack=False, tics=tics)
        data = bytearray(lmp.read_bytes())
        if settings['secret']:
            data[10] = 15
        for tic in range(tics):
            data[109 + tic * 4] = 25 if tic < 80 else 0
            data[109 + tic * 4 + 3] = 1 if tic % 20 == 1 or (held and tic >= 81) else 0
        pistol, shotgun = (1140, 1220) if held else (180, 260)
        data[109 + pistol * 4 + 3] = (1 if held else 0) | 4 | (1 << 3)
        data[109 + shotgun * 4 + 3] = (1 if held else 0) | 4 | (2 << 3)
        lmp.write_bytes(data)
        for engine in ['gd', 'dsda']:
            (folder / f'{engine}.csv').unlink(missing_ok=True)
        run(['xvfb-run', '-a', 'go', 'test', '-tags', 'integration', './internal/doomruntime',
             '-run', '^TestBoomRNGDemoCapture$', '-count=1'], folder / 'gd.log',
            dict(env, GD_RNG_DEMO=str(lmp), GD_RNG_WAD=str(merged), GD_RNG_OUT=str(folder / 'gd.csv')))
        run(['xvfb-run', '-a', exe, '-iwad', args.iwad.resolve(), '-file', pwad, '-timedemo', lmp,
             '-nodeh', '-nosound', '-nodraw', '-config', out / 'dsda.cfg'], folder / 'dsda.log',
            dict(env, GD_DSDA_RNG_DEMO_OUT=str(folder / 'dsda.csv')))
        result = compare(folder / 'gd.csv', folder / 'dsda.csv')
        result['coverage'] = coverage(folder / 'dsda.csv', settings['secret'], held)
        result['passed'] &= result['coverage']['passed']
        result['inputs'] = {p.name: digest(p) for p in [pwad, lmp]}
        result['settings'] = settings
        results[name] = result
        print(name, result['matched'], '/', result['rows'], 'coverage', result['coverage'], flush=True)
        if result['failures']:
            print('first mismatch:', result['failures'][0]['row'], list(result['failures'][0]['diff']), flush=True)
    source = hashlib.sha256()
    for p in sorted((ROOT / 'internal').rglob('*.go')):
        source.update(str(p.relative_to(ROOT)).encode() + b'\0' + p.read_bytes())
    report = dict(passed=all(r['passed'] for r in results.values()), cases=results,
                  metadata=dict(iwad=digest(args.iwad), reference=digest(exe), gd_source=source.hexdigest(),
                                verifier=digest(Path(__file__)),
                                observer=digest(ROOT / 'scripts/visual_compare/dsda_rng_demo_probe.inc')),
                  scope='Live Boom normal MAP01->MAP02 and secret MAP15->MAP31 transitions. Compare all '
                        'active level tics, command-stream positions across intermission, inventory/key '
                        'carry and RNG state. Intermission graphics/sounds, mid-intermission saves, '
                        'finale graphics/sounds and multiplayer map transitions are not covered. Held-button '
                        'secret exit also checks accelerated finale timeout without a second press.')
    (out / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
    if not report['passed']:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
