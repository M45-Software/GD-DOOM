#!/usr/bin/env python3
"""Frozen generated co-op NoMo inputs on unchanged Surge, replayed against DSDA.

The source movement route is an attributed solo recording. Explicit authored
inputs add a partner, deaths, respawns, parking and screen advances. No gameplay
patch, adaptive controller or world edits are used during verification.
"""
import argparse
import csv
import hashlib
import json
import os
from pathlib import Path
import struct

from verify_boom_demos import fetch
from verify_boom_full_combat import compare_complete
from verify_boom_transfers import ROOT, digest, merge_wads, run
from verify_boom_community_coop_campaign import capture, lumps, source_digest

RECIPE = ROOT / 'scripts/fixtures/surge_coop_recipe.json'
COMMANDS = 17190
MAP_ORDER = [f'MAP{n:02d}' for n in range(1, 18)]


def fixture(recipe, source, console):
    raw = bytearray.fromhex(recipe['header'])
    if len(raw) != 109 or raw[0] != 202 or raw[77:81] != bytes([1, 1, 0, 0]) or raw[21] != 1:
        raise ValueError('expected two-player Boom NoMo fixture header')
    original = source.read_bytes()
    if original[0] != 202 or original[77:81] != bytes([1, 0, 0, 0]):
        raise ValueError('source is not the pinned solo Boom route')
    for segment in recipe['segments']:
        if 'source_first' in segment:
            start, end = segment['source_first'], segment['source_last']
            if not 1 <= start <= end <= 17376:
                raise ValueError('invalid source command range')
            for command in range(start, end + 1):
                raw.extend(original[109 + (command - 1) * 4:109 + command * 4])
                raw.extend(bytes(4))
        else:
            command = bytes.fromhex(segment['input'])
            if len(command) != 8 or segment['repeat'] < 1:
                raise ValueError('invalid authored input run')
            raw.extend(command * segment['repeat'])
    raw.append(128)
    if (len(raw) != 110 + COMMANDS * 8 or recipe['commands'] != COMMANDS or
            hashlib.sha256(raw).hexdigest() != recipe['console0_sha256']):
        raise ValueError('frozen generated fixture changed')
    raw[12] = console
    return raw


def audit(wad):
    data = lumps(wad)
    if any(name == 'DEHACKED' for level, name in data):
        raise ValueError('generated Boom corpus must have no gameplay patch')
    for level in MAP_ORDER:
        starts = {thing[3] for thing in struct.iter_unpack('<5h', data[level, 'THINGS'])}
        if not {1, 2}.issubset(starts):
            raise ValueError('missing co-op starts: ' + level)


def coverage(path):
    maps, phases, checkpoints = [], {0: 0, 1: 0, 2: 0}, {0}
    stories, intermissions, deaths, rebirths = set(), set(), set(), set()
    previous, segment_start, rows = None, 0, 0
    checkpoint_phase = {}
    segments = []
    with path.open() as stream:
        for index, row in enumerate(csv.DictReader(stream)):
            rows += 1
            if int(row['demotic']) != index + 1:
                raise ValueError('missing or reordered campaign command')
            phase, level = int(row['phase']), row['map']
            phases[phase] += 1
            if not maps or maps[-1] != level:
                maps.append(level)
            if phase == 1:
                intermissions.add(level)
            if phase == 2:
                stories.add(level)
            players = [list(map(int, p.split(':'))) for p in row['players'].strip(';').split(';')]
            if [p[0] for p in players] != [1, 2]:
                raise ValueError('full roster absent from a command phase')
            current = (level, phase, {p[0]: p[1] for p in players})
            if previous:
                if current[:2] != previous[:2]:
                    checkpoints.update([index - 1, index])
                    segments.append((segment_start, index - 1, previous[1]))
                    segment_start = index
                for slot, health in current[2].items():
                    old = previous[2][slot]
                    if old > 0 >= health and phase == 0:
                        deaths.add(slot)
                        checkpoints.update([index - 1, index])
                    if old <= 0 < health and phase == 0:
                        rebirths.add(slot)
                        checkpoints.update([index - 1, index])
            previous = current
    if previous:
        segments.append((segment_start, rows - 1, previous[1]))
        checkpoints.add(rows - 1)
    for start, end, phase in segments:
        if phase in (1, 2):
            checkpoints.add((start + end) // 2)
    for start, end, phase in segments:
        for index in checkpoints:
            if start <= index <= end:
                checkpoint_phase[index] = phase
    checks = dict(all_commands=rows == COMMANDS, map_order=maps == MAP_ORDER,
                  all_intermissions=intermissions == set(MAP_ORDER),
                  story_screens=stories == {'MAP06', 'MAP11'},
                  both_players_die=deaths == {1, 2}, both_players_reborn=rebirths == {1, 2},
                  original_ending=bool(previous and previous[:2] == ('MAP17', 1)))
    return dict(passed=all(checks.values()), checks=checks, commands=rows,
                maps=maps, phases=phases, checkpoints=sorted(checkpoints),
                checkpoint_phases={p: sum(v == p for v in checkpoint_phase.values()) for p in phases},
                finale_limit='Generated fixture ends in MAP17 intermission; no cast finale.')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad', type=Path, required=True)
    parser.add_argument('--reference', type=Path, required=True,
                        help='DSDA with the repository co-op observers installed')
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--console', type=int, choices=[0, 1], action='append')
    parser.add_argument('--mode', choices=['baseline', 'save', 'keyframe'], action='append')
    parser.add_argument('--timeout', type=int, default=1800)
    args = parser.parse_args()
    out = args.out.resolve(); out.mkdir(parents=True, exist_ok=True)
    (out / 'results.json').unlink(missing_ok=True)
    recipe = json.loads(RECIPE.read_text())
    wad = fetch(out, 'surge.zip', recipe['wad_archive'],
                recipe['wad_archive_sha256'], recipe['wad_entry'])
    source_demo = fetch(out, 'srallo721.zip', recipe['route_archive'],
                        recipe['route_archive_sha256'], recipe['route_entry'])
    audit(wad)
    merged = out / 'merged.wad'
    merge_wads([args.iwad.resolve(), wad], merged)
    consoles = list(dict.fromkeys(args.console or [0, 1]))
    modes = list(dict.fromkeys(args.mode or ['baseline', 'save', 'keyframe']))
    requested = {f'console{console}_{mode}' for console in consoles for mode in modes}
    source_hash = source_digest()
    guards = {str(path): digest(path) for path in [Path(__file__), RECIPE, ROOT / 'scripts/verify_boom_community_coop_campaign.py', ROOT / 'scripts/verify_boom_full_combat.py']}
    env = {k: v for k, v in os.environ.items()
           if not k.startswith(('GD_COOP_', 'GD_DSDA_', 'GD_DEBUG_', 'GD_TEMP_'))}
    env.update(SDL_AUDIODRIVER='dummy', GD_COOP_CAMPAIGN='1', GD_COOP_COMBAT_ONLY='1',
               GD_COOP_COMPACT='1', GD_COOP_LINES='1', GD_COOP_COLD_RESTORE='1')
    results = {}
    for console in consoles:
        folder = out / f'console{console}'; folder.mkdir(exist_ok=True)
        demo = folder / 'generated.lmp'
        demo.write_bytes(fixture(recipe, source_demo, console))
        identity = {key: digest(path) for key, path in dict(
            executable=args.reference, iwad=args.iwad, wad=wad, demo=demo,
            observer=ROOT / 'scripts/visual_compare/dsda_coop_demo_probe.inc',
            phase_observer=ROOT / 'scripts/visual_compare/dsda_intermission_probe.inc',
            finale_observer=ROOT / 'scripts/visual_compare/dsda_finale_probe.inc').items()}
        expected = {kind: folder / f'dsda-{kind}.csv' for kind in ['world', 'phase']}
        manifest = folder / 'reference.json'
        cached = json.loads(manifest.read_text()) if manifest.exists() else {}
        if not (cached.get('identity') == identity and all(
                path.exists() and cached.get('traces', {}).get(kind) == digest(path)
                for kind, path in expected.items())):
            manifest.unlink(missing_ok=True)
            run(['xvfb-run', '-a', args.reference.resolve(), '-iwad', args.iwad.resolve(),
                 '-file', wad, '-timedemo', demo, '-nodeh', '-nosound', '-nodraw',
                 '-config', folder / 'dsda.cfg'], folder / 'dsda.log',
                dict(env, GD_DSDA_COOP_OUT=str(expected['world']),
                     GD_DSDA_PHASE_OUT=str(expected['phase'])), timeout=args.timeout)
            manifest.write_text(json.dumps(dict(identity=identity, traces={
                kind: digest(path) for kind, path in expected.items()}), indent=2) + '\n')
        cov = coverage(expected['phase'])
        if not cov['passed']:
            raise ValueError(f'generated native coverage failed: {cov["checks"]}')
        for mode in modes:
            actual = {kind: folder / f'{mode}-{kind}.csv' for kind in expected}
            for path in actual.values():
                path.unlink(missing_ok=True)
            replay = dict(env, GD_COOP_WAD=str(merged), GD_COOP_DEMO=str(demo),
                          GD_COOP_OUT=str(actual['world']), GD_COOP_PHASE_OUT=str(actual['phase']),
                          GD_COOP_CHECKPOINT_FORMAT=mode, GD_COOP_CHECKPOINT_TICS=
                          '' if mode == 'baseline' else ','.join(map(str, cov['checkpoints'])))
            execution = capture(['xvfb-run', '-a', 'go', 'test', '-tags', 'integration',
                                 './internal/doomruntime', '-run', '^TestBoomCoopDemoCapture$',
                                 '-count=1', f'-timeout={args.timeout}s'],
                                folder / f'{mode}.log', replay, actual, expected, args.timeout)
            if source_digest() != source_hash or any(digest(Path(p)) != sha for p, sha in guards.items()):
                raise ValueError('source or fixture changed during verification; rerun')
            result = dict(passed=False, execution=execution, coverage=cov, inputs=identity,
                          restores=0 if mode == 'baseline' else len(cov['checkpoints']))
            if execution['returncode'] == 0:
                result['world'] = compare_complete(actual['world'], expected['world'], cov['phases'][0])
                result['phase'] = compare_complete(actual['phase'], expected['phase'], COMMANDS)
                result['passed'] = result['world']['passed'] and result['phase']['passed']
            results[f'console{console}_{mode}'] = result
            complete = set(results) == requested
            passed = complete and all(r['passed'] for r in results.values())
            report = dict(passed=passed, complete=complete, full_suite=passed and len(results) == 6,
                          source=source_hash, guards=guards, cases=results,
                          scope=recipe['scope'] + ' Both console headers are generated views. '
                          'Released MAP01-MAP17 only; MAP18 is incomplete. Full canonical active-world '
                          'digests and every command phase, with cold restores around map/phase and '
                          'death/rebirth boundaries and inside screens. Excludes cosmetic RNG, '
                          'decoration animation, custom text, pixels and audio. No cast-finale coverage.')
            (out / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
            print(console, mode, 'passed', result['passed'], execution, flush=True)
            if not result['passed']:
                raise SystemExit(1)


if __name__ == '__main__':
    main()


