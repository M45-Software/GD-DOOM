#!/usr/bin/env python3
"""Compare live Boom death, Use and level reloads against DSDA.

Fixtures use ordinary player/monster thinkers and damaging floors. No observer
injects death, modifies inventory or performs a reload. Every recorded command
is compared, including the dead-player interval and the first post-reload tic.
"""
import argparse
import csv
import hashlib
import json
import os
from pathlib import Path
import struct

from verify_boom_combat import arrange
from verify_boom_rng import compare, demo, fixture
from verify_boom_transfers import ROOT, build_reference, digest, merge_wads, run


def make_fixture(path, monster, hazard, ceiling):
    fixture(path, monster or 9)
    things = [(-256, -48, 0, 1, 7)]
    if monster:
        things.append((-208, -48, 180, monster, 7))
    # The shotgun and clip are consumed again after each real map rebuild.
    things += [(-256, -48, 0, 2001, 7), (-256, -48, 0, 2007, 7)]
    arrange(path, things)
    raw = path.read_bytes()
    count, directory = struct.unpack_from('<ii', raw, 4)
    lumps = []
    for i in range(count):
        start, size, name = struct.unpack_from('<ii8s', raw, directory + i * 16)
        data = raw[start:start + size]
        if name.rstrip(b'\0') == b'SECTORS':
            data = bytearray(data)
            for j in range(0, len(data), 26):
                struct.pack_into('<h', data, j + 2, ceiling)
            struct.pack_into('<h', data, 22, 16 if hazard else 0)
        lumps.append((name, data))
    payload, entries = bytearray(12), bytearray()
    for name, data in lumps:
        entries += struct.pack('<ii8s', len(payload), len(data), name)
        payload += data
    struct.pack_into('<4sii', payload, 0, b'PWAD', len(lumps), len(payload))
    path.write_bytes(payload + entries)


def coverage(path, held, lethal_use):
    with path.open() as f:
        rows = list(csv.DictReader(f))
    deaths = [i for i, r in enumerate(rows) if r['playerstate'] == '1' and
              (i == 0 or rows[i - 1]['playerstate'] == '0')]
    reloads = [i for i, r in enumerate(rows) if i and
               int(r['reloads']) > int(rows[i - 1]['reloads'])]
    checks = dict(
        player_died=bool(deaths),
        repeated_reloads=len(reloads) >= (1 if lethal_use else 2),
        use_sets_reborn=bool(reloads) and all(rows[i - 1]['playerstate'] == '2' for i in reloads),
        reload_resets_level=bool(reloads) and all(rows[i]['leveltime'] == '1' for i in reloads),
        reload_resets_rng_phase=bool(reloads) and all(rows[i]['logictic'] == '1' for i in reloads),
        rng_streams_continue=bool(reloads) and all(rows[i]['seeds'] != rows[0]['seeds'] for i in reloads),
        inventory_rebuilt=bool(reloads) and all(rows[i]['ammo'] == rows[0]['ammo'] and
                                               rows[i]['shells'] == rows[0]['shells'] for i in reloads),
        health_restored=bool(reloads) and all(int(rows[i]['health']) > 0 for i in reloads),
        death_lowers_view=any(int(r['viewheight']) < 41 * 65536 for r in rows if r['playerstate'] == '1'),
    )
    if held:
        checks['held_use_restarts_after_later_deaths'] = len(reloads) >= 2 and deaths[-1] > 150
    if lethal_use:
        # Floor damage kills during the tic; Use is first processed by the
        # death thinker on the next tic, and the reload follows one tic later.
        checks['lethal_tic_use_waits_for_death_thinker'] = deaths[0] == 128 and reloads[0] == 130
    return dict(passed=all(checks.values()), checks=checks, deaths=deaths, reload_tics=reloads)


def checkpoint_boundaries(path):
    """Save before, on and after each death and each queued/completed reload."""
    with path.open() as f:
        rows = list(csv.DictReader(f))
    boundaries = set()
    for i, row in enumerate(rows):
        previous = rows[i - 1] if i else None
        death = row['playerstate'] == '1' and (previous is None or previous['playerstate'] == '0')
        queued = row['playerstate'] == '2'
        reloaded = previous is not None and int(row['reloads']) > int(previous['reloads'])
        if death or queued or reloaded:
            boundaries.update(t for t in [i - 1, i, i + 1] if 0 <= t < len(rows))
    return sorted(boundaries)


def capture_gd(folder, env, lmp, merged, label='', checkpoints=(), fmt='save'):
    csv_path = folder / f'gd{label}.csv'
    log_path = folder / f'checkpoints{label}.json'
    csv_path.unlink(missing_ok=True)
    log_path.unlink(missing_ok=True)
    run(['xvfb-run', '-a', 'go', 'test', '-tags', 'integration', './internal/doomruntime',
         '-run', '^TestBoomRNGDemoCapture$', '-count=1'], folder / f'gd{label}.log',
        dict(env, GD_RNG_DEMO=str(lmp), GD_RNG_WAD=str(merged), GD_RNG_OUT=str(csv_path),
             GD_RNG_CHECKPOINT_TICS=','.join(map(str, checkpoints)), GD_RNG_CHECKPOINT_FORMAT=fmt,
             GD_RNG_CHECKPOINT_LOG=str(log_path)))
    return json.loads(log_path.read_text())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad', required=True, type=Path)
    ref = parser.add_mutually_exclusive_group(required=True)
    ref.add_argument('--reference', type=Path)
    ref.add_argument('--reference-source', type=Path)
    parser.add_argument('--cmake-arg', action='append', default=[])
    parser.add_argument('--out', type=Path, default=ROOT / 'tmp/boom-rebirth')
    parser.add_argument('--case', action='append')
    parser.add_argument('--no-checkpoints', action='store_true', help='only compare continuous playback')
    args = parser.parse_args()
    cases = {}
    for insurance in [0, 1]:
        for seed in [0, 1993]:
            cases[f'shotgun_i{insurance}_s{seed}'] = dict(insurance=insurance, seed=seed)
        cases[f'held_use_i{insurance}'] = dict(insurance=insurance, seed=1993, held=True)
        cases[f'lethal_use_i{insurance}'] = dict(insurance=insurance, seed=1993, lethal=True, monster=0)
        cases[f'revenant_i{insurance}'] = dict(insurance=insurance, seed=1993, monster=66, hazard=False, tics=2000)
    cases['low_ceiling'] = dict(insurance=1, seed=0, ceiling=64)
    if args.case and set(args.case) - cases.keys():
        parser.error('unknown case')
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    iwad = args.iwad.resolve()
    exe = args.reference.resolve() if args.reference else build_reference(args.reference_source, out, args.cmake_arg)
    env = dict(os.environ, SDL_AUDIODRIVER='dummy', GD_RNG_COMBAT_ONLY='1', GD_RNG_REBIRTH='1')
    for key in list(env):
        if key.startswith(('GD_DSDA_', 'GD_RNG_CHECKPOINT_')) or key in ['GD_RNG_FULL_MAP', 'GD_RNG_CRUSHED_CORPSE', 'GD_RNG_CAMPAIGN']:
            env.pop(key)
    results = {}
    for name, settings in cases.items():
        if args.case and name not in args.case:
            continue
        folder = out / name
        folder.mkdir(exist_ok=True)
        pwad, lmp, merged = (folder / file for file in ['combat.wad', 'combat.lmp', 'merged.wad'])
        make_fixture(pwad, settings.get('monster', 9), settings.get('hazard', True), settings.get('ceiling', 128))
        merge_wads([iwad, pwad], merged)
        tics = settings.get('tics', 400 if settings.get('lethal') else 1000)
        demo(lmp, settings['seed'], settings['insurance'], attack=False, tics=tics)
        data = bytearray(lmp.read_bytes())
        if settings.get('held'):
            use = range(150, tics)
        elif settings.get('lethal'):
            use = [128, 129]
        elif not settings.get('hazard', True):
            use = range(500, tics, 500)
        else:
            use = range(200, tics, 200)
        for tic in use:
            data[109 + tic * 4 + 3] = 2
        lmp.write_bytes(data)
        for file in ['gd.csv', 'dsda.csv']:
            (folder / file).unlink(missing_ok=True)
        capture_gd(folder, env, lmp, merged)
        run(['xvfb-run', '-a', exe, '-iwad', iwad, '-file', pwad, '-timedemo', lmp,
             '-nodeh', '-nosound', '-nodraw', '-config', out / 'dsda.cfg'], folder / 'dsda.log',
            dict(env, GD_DSDA_RNG_DEMO_OUT=str(folder / 'dsda.csv')))
        result = compare(folder / 'gd.csv', folder / 'dsda.csv', tics)
        result['coverage'] = coverage(folder / 'dsda.csv', settings.get('held'), settings.get('lethal'))
        result['passed'] &= result['coverage']['passed']
        result['checkpoint_runs'] = {}
        if not args.no_checkpoints:
            boundaries = checkpoint_boundaries(folder / 'dsda.csv')
            with (folder / 'dsda.csv').open() as f:
                reference_rows = list(csv.DictReader(f))
            states = {reference_rows[t]['playerstate'] for t in boundaries}
            for fmt in ['save', 'keyframe']:
                applied = capture_gd(folder, env, lmp, merged, '-' + fmt, boundaries, fmt)
                check = compare(folder / f'gd-{fmt}.csv', folder / 'dsda.csv', tics)
                check.update(checkpoint_tics=boundaries, applied=applied,
                             coverage=dict(all_requested_applied=applied == boundaries,
                                           living_dead_reborn={'0', '1', '2'} <= states))
                check['passed'] &= all(check['coverage'].values())
                result['checkpoint_runs'][fmt] = check
                result['passed'] &= check['passed']
                print(name, fmt, check['matched'], '/', check['rows'], 'checkpoints', len(applied), flush=True)
                if check['failures']:
                    print('first mismatch:', check['failures'][0]['row'], list(check['failures'][0]['diff']), flush=True)
        result['settings'] = settings
        result['inputs'] = {file.name: digest(file) for file in [pwad, lmp]}
        results[name] = result
        print(name, result['matched'], '/', result['rows'], 'coverage', result['coverage']['passed'], flush=True)
        if result['failures']:
            print('first mismatch:', result['failures'][0]['row'], list(result['failures'][0]['diff']), flush=True)
        if not result['coverage']['passed']:
            print(result['coverage'], flush=True)
    source = hashlib.sha256()
    for path in sorted((ROOT / 'internal').rglob('*.go')):
        source.update(str(path.relative_to(ROOT)).encode() + b'\0' + path.read_bytes())
    report = dict(passed=all(r['passed'] for r in results.values()), cases=results,
                  metadata=dict(iwad=digest(iwad), reference=digest(exe), gd_source=source.hexdigest(),
                                verifier=digest(Path(__file__)),
                                observer=digest(ROOT / 'scripts/visual_compare/dsda_rng_demo_probe.inc')),
                  compared_tics=sum(r['rows'] + sum(c['rows'] for c in r['checkpoint_runs'].values()) for r in results.values()),
                  binary_restores=sum(len(c['applied']) for r in results.values() for c in r['checkpoint_runs'].values()),
                  scope='Live single-player Boom death and same-map rebirth, including player physics, '
                        'inventory, weapons, actors, sectors, RNG streams and per-level phase. '
                        'Binary save/keyframe continuation at death and pending/completed rebirth boundaries is '
                        'included unless --no-checkpoints is selected. Intermission, map transition and '
                        'multiplayer rebirth are outside this matrix.')
    core = exe.parent.parent / 'reference-source/src'
    for name in ['g_game.c', 'p_enemy.c', 'p_mobj.c', 'p_inter.c', 'p_map.c', 'p_user.c', 'p_pspr.c', 'm_random.c']:
        if (core / name).exists():
            report['metadata'][f'reference_core/{name}'] = digest(core / name)
    (out / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
    if not report['passed']:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
