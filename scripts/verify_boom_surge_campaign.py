#!/usr/bin/env python3
"""Replay Tezur0's unchanged Surge UV-Max campaign and cold checkpoints.

The original recording includes Doom II MAP31/MAP32, inherited from the IWAD.
Full verification requires the original Doom II IWAD. --through is an explicit
MAP01-MAP15 diagnostic and never claims full campaign parity.
"""
import argparse
from collections import Counter
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

ARCHIVE = '/files/demos/surge/53710/srall-5337.zip'
ARCHIVE_SHA = '3f3d3e7f0579519e0c50133e74031d6fffeb4a4cee72d889072668782db2c8d8'
WAD_ARCHIVE = '/files/wads/doom2/2733/surge.zip'
WAD_SHA = '4f49d63682adbc299e9dce56877720704eaa8ec504ba3da370c05875b405ae22'
IWAD_SHA = '10d67824b11025ddd9198e8cfc87ca335ee6e2d3e63af4180fa9b8a471893255'
COMMANDS = 114375
BEFORE_SECRET = 94428
MAP_ORDER = [f'MAP{n:02d}' for n in range(1, 16)] + ['MAP31', 'MAP32', 'MAP16', 'MAP17']
# Exact active tics reconstructed from the original archive's per-map times.
LEVEL_TICS = dict(zip(MAP_ORDER, [1084,4175,3705,4339,5195,5401,4361,9517,
    5794,9109,5712,7764,6087,11866,9344,8773,2198,4343,4209]))
SECRET_TOTALS = dict(zip(MAP_ORDER, [1,1,0,2,0,1,0,0,0,0,0,2,1,1,0,4,6,2,1]))


def audit_inputs(wad, demo, iwad, through):
    raw = demo.read_bytes()
    if (raw[0] != 202 or raw[8:13] != bytes([3,1,1,0,0]) or raw[21] != 0 or
            raw[77:81] != bytes([1,0,0,0])):
        raise ValueError('expected original single-player UV Boom combat header')
    end = 109
    while end < len(raw) and raw[end] != 128:
        end += 4
    if end >= len(raw) or (end - 109) // 4 != COMMANDS:
        raise ValueError('original campaign command count changed')
    data = lumps(wad)
    if any(name == 'DEHACKED' for level, name in data):
        raise ValueError('Surge must have no DeHackEd patch')
    if any((level, 'THINGS') in data for level in ['MAP31', 'MAP32']):
        raise ValueError('secret maps no longer inherited from Doom II')
    if not all((f'MAP{n:02d}', 'THINGS') in data for n in range(1, 18)):
        raise ValueError('released Surge maps missing')
    base = lumps(iwad)
    freedoom = any(name == 'FREEDOOM' for level, name in base)
    if freedoom and not through:
        raise ValueError('Full original UV-Max replay requires Doom II MAP31/MAP32; '
                         'Freedoom is valid only for --through <= 94428 diagnostics')
    if not through and digest(iwad) != IWAD_SHA:
        raise ValueError('Full verification requires the audited original Doom II 1.9 IWAD '
                         '(SHA-256 ' + IWAD_SHA + ')')
    for level in ['MAP31', 'MAP32']:
        if (level, 'THINGS') not in base:
            raise ValueError('IWAD is missing the inherited secret maps')
    return dict(player='Tezur0', category='UV Max', recorded_engine='DSDA-Doom 0.14.0 cl9',
                original_commands=COMMANDS, unmodified_commands=True, gameplay_patch=False,
                inherited_maps=['MAP31', 'MAP32'], freedoom_diagnostic=freedoom,
                secret_map_lumps={level: {name: hashlib.sha256(base[level, name]).hexdigest()
                    for name in ['THINGS', 'LINEDEFS', 'SIDEDEFS', 'VERTEXES', 'SECTORS']}
                    for level in ['MAP31', 'MAP32']})


def coverage(path, commands):
    diagnostic = commands != COMMANDS
    expected_maps = MAP_ORDER[:15] if diagnostic else MAP_ORDER
    maps, phases, active = [], Counter(), Counter()
    checkpoints, segments = {0, commands - 1}, []
    previous, start, count = None, 0, 0
    intermissions, stories, deaths, rebirths = set(), set(), [], []
    maxima = {}
    with path.open() as stream:
        for index, row in enumerate(csv.DictReader(stream)):
            if int(row['demotic']) != index + 1:
                raise ValueError('missing/reordered native command')
            count += 1
            level, phase = row['map'], int(row['phase'])
            if not maps or maps[-1] != level:
                maps.append(level)
            phases[phase] += 1
            if phase == 0:
                active[level] += 1
            elif phase == 1:
                intermissions.add(level)
                if row['wi_counts']:
                    values = list(map(int, row['wi_counts'].split(';')[0].split(':')))
                    maxima[level] = [max(a,b) for a,b in zip(maxima.get(level, [-1]*4), values)]
            else:
                stories.add(level)
            player = row['players'].strip(';').split(';')
            if len(player) != 1 or not player[0].startswith('1:'):
                raise ValueError('original solo roster changed')
            health = int(player[0].split(':')[1])
            current = (level, phase, health)
            if previous:
                if current[:2] != previous[:2]:
                    checkpoints.update([index - 1, index])
                    segments.append((start, index - 1, previous[1]))
                    start = index
                if phase == 0 and previous[2] > 0 >= health:
                    deaths.append(index + 1); checkpoints.update([index - 1, index])
                if phase == 0 and previous[2] <= 0 < health:
                    rebirths.append(index + 1); checkpoints.update([index - 1, index])
            # Interior combat checkpoints also cover sustained fights between exits.
            if phase == 0 and active[level] % 1750 == 0:
                checkpoints.add(index)
            previous = current
    if previous:
        segments.append((start, count - 1, previous[1]))
    for first, last, phase in segments:
        if phase in (1,2):
            checkpoints.add((first + last) // 2)
    # Partial diagnostics can end before MAP15; do not pretend those maps ran.
    wanted = expected_maps if not diagnostic or commands == BEFORE_SECRET else maps
    checks = dict(all_commands=count == commands, map_order=maps == wanted,
                  active_times=all(active[m] == LEVEL_TICS[m] for m in maps
                                   if m in intermissions),
                  all_intermissions=intermissions == set(wanted),
                  story_screens=stories == ({'MAP06','MAP11','MAP15','MAP31'} if not diagnostic
                      else {m for m in ['MAP06','MAP11','MAP15'] if m in intermissions}),
                  max_kills=all(maxima.get(m,[-1])[0] >= 100 for m in intermissions),
                  max_secrets=all(not SECRET_TOTALS[m] or maxima.get(m,[-1]*3)[2] >= 100
                                  for m in intermissions),
                  ending=bool(previous and previous[:2] == (wanted[-1],1 if not diagnostic else 2)))
    # The fixed prefix ends inside MAP15's story; the original full movie ends
    # during MAP17 intermission. No input or ending is appended to either run.
    return dict(passed=all(checks.values()), checks=checks, commands=count, maps=maps,
                phases=dict(phases), active_tics=dict(active), intermission_maxima=maxima,
                deaths=deaths, rebirths=rebirths, checkpoints=sorted(checkpoints),
                checkpoint_phases={str(p):sum(first <= tic <= last
                    for first,last,phase in segments if phase == p for tic in checkpoints)
                    for p in range(3)})


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad', type=Path, required=True)
    parser.add_argument('--reference', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--mode', choices=['baseline', 'save', 'keyframe'], action='append')
    parser.add_argument('--through', type=int, choices=[BEFORE_SECRET],
                        help='diagnose exactly MAP01-MAP15, before inherited Doom II maps')
    parser.add_argument('--timeout', type=int, default=3600)
    args = parser.parse_args()
    out = args.out.resolve(); out.mkdir(parents=True, exist_ok=True)
    commands = args.through or COMMANDS
    label = f'through-{commands}' if args.through else 'full'
    result_file = out / f'results-{label}.json'
    result_file.unlink(missing_ok=True)
    wad = fetch(out, 'surge.zip', WAD_ARCHIVE, WAD_SHA, 'surge.wad')
    demo = fetch(out, 'srall-5337.zip', ARCHIVE, ARCHIVE_SHA, 'srall-5337.lmp')
    audit = audit_inputs(wad, demo, args.iwad, args.through)
    folder = out / label; folder.mkdir(exist_ok=True)
    merged = folder / 'merged.wad'
    merge_wads([args.iwad.resolve(), wad], merged)
    modes = list(dict.fromkeys(args.mode or ['baseline', 'save', 'keyframe']))
    source = source_digest()
    guards = {str(p): digest(p) for p in [Path(__file__),
        ROOT / 'scripts/verify_boom_community_coop_campaign.py',
        ROOT / 'scripts/verify_boom_full_combat.py', ROOT / 'scripts/verify_boom_demos.py',
        ROOT / 'scripts/verify_boom_transfers.py']}
    identity = dict(commands=commands, executable=digest(args.reference),
        iwad=digest(args.iwad), wad=digest(wad), demo=digest(demo),
        observers={name: digest(ROOT / 'scripts/visual_compare' / name) for name in
            ['dsda_coop_demo_probe.inc','dsda_intermission_probe.inc','dsda_finale_probe.inc']})
    env = {k:v for k,v in os.environ.items()
           if not k.startswith(('GD_COOP_','GD_DSDA_','GD_DEBUG_','GD_SURGE_'))}
    env.update(SDL_AUDIODRIVER='dummy', GD_COOP_CAMPAIGN='1', GD_COOP_COMBAT_ONLY='1',
               GD_COOP_COMPACT='1', GD_COOP_LINES='1', GD_COOP_COLD_RESTORE='1')
    if args.through:
        env['GD_COOP_CAPTURE_COMMANDS'] = str(commands)
    expected = {kind: folder / f'dsda-{kind}.csv' for kind in ['world','phase']}
    manifest = folder / 'reference.json'
    cached = json.loads(manifest.read_text()) if manifest.exists() else {}
    if not (cached.get('identity') == identity and all(path.exists() and
            cached.get('traces',{}).get(kind) == digest(path) for kind,path in expected.items())):
        manifest.unlink(missing_ok=True)
        run(['xvfb-run','-a',args.reference.resolve(),'-iwad',args.iwad.resolve(),'-file',wad,
             '-timedemo',demo,'-nodeh','-nosound','-nodraw','-config',folder/'dsda.cfg'],
            folder/'dsda.log',dict(env,GD_DSDA_COOP_OUT=str(expected['world']),
            GD_DSDA_PHASE_OUT=str(expected['phase'])),timeout=args.timeout)
        manifest.write_text(json.dumps(dict(identity=identity,traces={
            kind:digest(path) for kind,path in expected.items()}),indent=2)+'\n')
    cov = coverage(expected['phase'], commands)
    if not cov['passed']:
        raise ValueError(f'original native coverage failed: {cov["checks"]}')
    results = {}
    for mode in modes:
        actual = {kind: folder / f'{mode}-{kind}.csv' for kind in expected}
        for path in actual.values():
            path.unlink(missing_ok=True)
        replay = dict(env, GD_COOP_WAD=str(merged),GD_COOP_DEMO=str(demo),
            GD_COOP_OUT=str(actual['world']),GD_COOP_PHASE_OUT=str(actual['phase']),
            GD_COOP_CHECKPOINT_FORMAT=mode,GD_COOP_CHECKPOINT_TICS=
            '' if mode == 'baseline' else ','.join(map(str,cov['checkpoints'])))
        if mode == 'baseline':
            replay['GD_COOP_EXPORT_SAVES'] = str(folder/'baseline-states'/source)
        execution = capture(['xvfb-run','-a','go','test','-tags','integration',
            './internal/doomruntime','-run','^TestBoomCoopDemoCapture$','-count=1',
            f'-timeout={args.timeout}s'],folder/f'{mode}.log',replay,actual,expected,args.timeout)
        if source_digest() != source or any(digest(Path(p)) != sha for p,sha in guards.items()):
            raise ValueError('source/verifier changed during replay; rerun this attempt')
        result = dict(passed=False,execution=execution,coverage=cov,
                      restores=0 if mode == 'baseline' else len(cov['checkpoints']))
        if execution['returncode'] == 0:
            result['world'] = compare_complete(actual['world'],expected['world'],cov['phases'][0])
            result['phase'] = compare_complete(actual['phase'],expected['phase'],commands)
            result['passed'] = result['world']['passed'] and result['phase']['passed']
        results[mode] = result
        complete = set(results) == set(modes)
        passed = complete and all(r['passed'] for r in results.values())
        report = dict(passed=passed,complete=complete,full_suite=passed and not args.through
            and set(results)=={'baseline','save','keyframe'},diagnostic_only=bool(args.through),
            audit=audit,inputs=identity,source=source,guards=guards,requested_modes=modes,cases=results,
            scope=('Diagnostic original-command prefix through MAP15; no full-campaign parity claim. '
                   if args.through else 'Original full 19-map Surge UV-Max campaign, including inherited '
                   'Doom II MAP31/MAP32. ') + 'Every canonical active-world row and every campaign '
                   'phase; cold saves/keyframes at boundaries, screen interiors, deaths/rebirths and '
                   'sustained fights. Original commands and ending preserved. No gameplay patch. '
                   'Excludes cosmetic RNG, decoration animation, custom text, pixels and audio.')
        result_file.write_text(json.dumps(report,indent=2)+'\n')
        print(mode,'passed',result['passed'],execution,flush=True)
        if not result['passed']:
            raise SystemExit(1)


if __name__ == '__main__':
    main()


