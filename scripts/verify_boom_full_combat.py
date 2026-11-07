#!/usr/bin/env python3
"""Replay an unchanged, hash-pinned Sunlust UV-Max demo through map completion.

Compare every active map tic, including weapons, all ammo, sector planes/lights,
combat actors and gameplay RNG, then independently check both exit summaries.
Intermission animation and following maps are outside this single-map check.
"""
import argparse
import csv
import hashlib
import json
import itertools
import os
from pathlib import Path
import struct

from verify_boom_demos import fetch, WADS
from verify_boom_transfers import ROOT, build_reference, digest, merge_wads, run


CASES = {
    'sl01-124': dict(map=1, tics=2972, demo=(
        '/files/demos/sunlust/68243/sl01-124.zip',
        '6c1250e5bb649abfa953fc352303f3ca9e25f2e7f42082a17ec9cb3e1e9ebbcd',
        'sl01-124.lmp')),
    'sl01-126': dict(map=1, tics=3024, demo=(
        '/files/demos/sunlust/68243/sl01-124.zip',
        '6c1250e5bb649abfa953fc352303f3ca9e25f2e7f42082a17ec9cb3e1e9ebbcd',
        'sl01-126.lmp')),
    'sl02m225': dict(map=2, tics=5086, demo=(
        '/files/demos/sunlust/112684/sl02m225.zip',
        '00e30905b64abecf1839a48848634554005c316217d75dbea3c306d53b3e571a',
        'sl02m225.lmp')),
}
SUNLUST_DEH_SHA256 = '91f46075aea96dc191573d846e291f2e6cc5bf01abea64ceddca3fb60f94e0a1'


def compare_complete(actual_path, expected_path, expected_rows):
    """Keep useful mismatch evidence even when one engine fails to exit."""
    rows = matched = actual_rows = reference_rows = 0
    failures = []
    with actual_path.open() as a, expected_path.open() as b:
        for actual, expected in itertools.zip_longest(csv.DictReader(a), csv.DictReader(b)):
            rows += 1
            actual_rows += actual is not None
            reference_rows += expected is not None
            if actual is None or expected is None:
                diff = {'missing_row': dict(gd=actual is None, dsda=expected is None)}
            else:
                diff = {k: dict(gd=actual[k], dsda=expected[k]) for k in expected if actual[k] != expected[k]}
            matched += not diff
            if diff and len(failures) < 6:
                failures.append(dict(row=rows, diff=diff))
    counts = dict(gd=actual_rows, dsda=reference_rows, expected=expected_rows)
    return dict(passed=matched == rows and actual_rows == reference_rows == expected_rows,
                rows=rows, matched=matched, row_counts=counts, failures=failures,
                gd_hash=digest(actual_path), dsda_hash=digest(expected_path))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad', required=True, type=Path)
    parser.add_argument('--case', choices=CASES, default='sl01-124')
    parser.add_argument('--checkpoint-tic', type=int, action='append', default=[])
    parser.add_argument('--checkpoint-format', choices=['save', 'keyframe'], default='save')
    ref = parser.add_mutually_exclusive_group(required=True)
    ref.add_argument('--reference', type=Path)
    ref.add_argument('--reference-source', type=Path)
    parser.add_argument('--cmake-arg', action='append', default=[])
    parser.add_argument('--out', type=Path, default=ROOT / 'tmp/boom-full-combat')
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    case = CASES[args.case]
    map_name = f"MAP{case['map']:02d}"
    checkpoints = sorted(set(args.checkpoint_tic))
    if any(t < 0 or t >= case['tics'] for t in checkpoints):
        parser.error('checkpoint tic outside the active map')
    demo = case['demo']
    iwad = args.iwad.resolve()
    pwad = fetch(out, 'sunlust.zip', *WADS['sunlust'])
    lmp = fetch(out, args.case + '.zip', *demo)
    raw = lmp.read_bytes()
    if raw[0] != 202 or raw[8:11] != bytes([3, 1, case['map']]) or raw[21] != 0:
        raise ValueError(f'expected the original UV Boom 2.02 {map_name} combat recording')
    end = 109
    while end < len(raw) and raw[end] != 128:
        end += 4
    if end >= len(raw):
        raise ValueError('missing demo end marker')
    # Sunlust's pinned gameplay patch changes only Commander Keen and its
    # frames. The selected map must contain no Keen. Verify both facts before using -nodeh
    # to also exclude Freedoom's unrelated super-shotgun flash timing patch.
    wad = pwad.read_bytes()
    count, directory = struct.unpack_from('<ii', wad, 4)
    in_map, checked_map, checked_patch = False, False, False
    for i in range(count):
        start, size, name = struct.unpack_from('<ii8s', wad, directory + i * 16)
        name, data = name.rstrip(b'\0'), wad[start:start + size]
        if name == map_name.encode():
            in_map = True
        elif in_map and name == b'THINGS':
            if any(thing[3] == 72 for thing in struct.iter_unpack('<5h', data)):
                raise ValueError(f'{map_name} unexpectedly requires the Commander Keen patch')
            in_map, checked_map = False, True
        if name == b'DEHACKED':
            if hashlib.sha256(data).hexdigest() != SUNLUST_DEH_SHA256:
                raise ValueError('Sunlust gameplay patch changed; audit before replaying')
            checked_patch = True
    if not checked_map or not checked_patch:
        raise ValueError('missing expected map or patch')
    exe = args.reference.resolve() if args.reference else build_reference(args.reference_source, out, args.cmake_arg)
    merged = out / 'merged.wad'
    merge_wads([iwad, pwad], merged)
    for name in ['gd.csv', 'dsda.csv', 'gd-summary.json', 'dsda-summary.json', 'results.json', 'checkpoints.json']:
        (out / name).unlink(missing_ok=True)
    env = dict(os.environ, SDL_AUDIODRIVER='dummy', GD_RNG_COMBAT_ONLY='1', GD_RNG_FULL_MAP='1')
    env.update(GD_RNG_CHECKPOINT_TICS=','.join(map(str, checkpoints)),
               GD_RNG_CHECKPOINT_FORMAT=args.checkpoint_format,
               GD_RNG_CHECKPOINT_LOG=str(out / 'checkpoints.json'))
    # A controlled corpse fixture or another probe must not affect this replay.
    for key in list(env):
        if key.startswith('GD_DSDA_') or key in ['GD_RNG_CRUSHED_CORPSE', 'GD_RNG_REBIRTH']:
            env.pop(key)
    run(['xvfb-run', '-a', 'go', 'test', '-tags', 'integration', './internal/doomruntime',
         '-run', '^TestBoomRNGDemoCapture$', '-count=1'], out / 'gd.log',
        dict(env, GD_RNG_DEMO=str(lmp), GD_RNG_WAD=str(merged), GD_RNG_OUT=str(out / 'gd.csv'),
             GD_RNG_SUMMARY=str(out / 'gd-summary.json')))
    run(['xvfb-run', '-a', exe, '-iwad', iwad, '-file', pwad, '-timedemo', lmp,
         '-nodeh', '-nosound', '-nodraw', '-config', out / 'dsda.cfg'], out / 'dsda.log',
        dict(env, GD_DSDA_RNG_DEMO_OUT=str(out / 'dsda.csv'), GD_RNG_SUMMARY=str(out / 'dsda-summary.json')))
    result = compare_complete(out / 'gd.csv', out / 'dsda.csv', case['tics'])
    summaries = {name: json.loads((out / f'{name}-summary.json').read_text()) for name in ['gd', 'dsda']}
    expected = summaries['dsda']
    coverage = dict(
        completed_map=all(s['completed'] for s in summaries.values()),
        requested_checkpoints_reloaded=json.loads((out / 'checkpoints.json').read_text()) == checkpoints,
        expected_exit_tic=expected['leveltime'] == case['tics'],
        all_monsters_killed=expected['kills'] >= expected['total_kills'] > 0,
        all_secrets_found=expected['secrets'] == expected['total_secrets'] > 0,
        player_survived=expected['health'] > 0,
    )
    weapons, damaged = set(), False
    barrel_death = barrel_explosion = carried_blood = resurrection = False
    with (out / 'dsda.csv').open() as f:
        for row in csv.DictReader(f):
            if row['attackdown'] == '1':
                weapons.add(int(row['ready']))
            damaged |= int(row['health']) < 100
            for actor in row['mobjs'].strip(';').split(';'):
                if not actor:
                    continue
                m = list(map(int, actor.split(':')))
                barrel_death |= m[0] == 30 and m[8] <= 0
                barrel_explosion |= m[0] == 30 and m[10] == 811
                carried_blood |= m[0] == 38 and (m[5] != 0 or m[6] != 0)
                resurrection |= m[0] == 3 and 266 <= m[10] <= 268
    coverage.update(multiple_weapons_fired=len(weapons) > 1, player_damaged=damaged)
    # Kill totals alone can hide living monsters after arch-vile resurrection.
    # Canonical Doom counted-monster types; Lost Souls (18) are not counted.
    counted = {1, 2, 3, 5, 8, 10, 11, 12, 13, 14, 15, 17, 19, 20, 21, 22, 23, 24}
    remaining = [m for m in row['mobjs'].strip(';').split(';')
                 if m and int(m.split(':')[0]) in counted and int(m.split(':')[8]) > 0]
    coverage['no_surviving_counted_monsters'] = not remaining
    if args.case == 'sl02m225':
        coverage.update(barrels_killed=barrel_death, barrel_explosion=barrel_explosion,
                        moving_blood=carried_blood, arch_vile_resurrection=resurrection)
    result.update(case=args.case, map=map_name, checkpoints=dict(tics=checkpoints, format=args.checkpoint_format), summaries=summaries, summaries_equal=summaries['gd'] == expected,
                  coverage=coverage, weapons_fired=sorted(weapons), recorded_commands=(end - 109) // 4)
    result['passed'] &= result['summaries_equal'] and all(coverage.values())
    source = hashlib.sha256()
    for path in sorted((ROOT / 'internal').rglob('*.go')):
        source.update(str(path.relative_to(ROOT)).encode() + b'\0' + path.read_bytes())
    result['metadata'] = dict(iwad=digest(iwad), pwad=digest(pwad), demo=digest(lmp),
                              reference=digest(exe), gd_source=source.hexdigest(),
                              verifier=digest(Path(__file__)),
                              observer=digest(ROOT / 'scripts/visual_compare/dsda_rng_demo_probe.inc'),
                              demo_archive=demo[1], wad_archive=WADS['sunlust'][1],
                              reference_nodeh=True, sunlust_dehacked=SUNLUST_DEH_SHA256,
                              selected_map_has_no_commander_keen=True)
    core = exe.parent.parent / 'reference-source/src'
    for name in ['p_enemy.c', 'p_mobj.c', 'p_inter.c', 'p_map.c', 'p_user.c', 'p_pspr.c', 'm_random.c']:
        if (core / name).exists():
            result['metadata'][f'reference_core/{name}'] = digest(core / name)
    result['scope'] = (f'Unchanged Sunlust {map_name} {args.case} UV-Max recording through completed-map exit. '
                       'Compare every active map tic and both completion summaries; intermission animation, '
                       'death/rebirth and following maps are not covered. Static decorations, player/pickup '
                       'animation, cosmetic misc RNG and identity among same-type targets are excluded. '
                       'DSDA -nodeh preserves original Doom gameplay definitions with Freedoom assets.')
    (out / 'results.json').write_text(json.dumps(result, indent=2) + '\n')
    print(f"{result['matched']}/{result['rows']} rows match; summaries equal: {result['summaries_equal']}; coverage: {coverage}")
    if result['failures']:
        print('first mismatch:', result['failures'][0]['row'], list(result['failures'][0]['diff']))
    if not result['passed']:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
