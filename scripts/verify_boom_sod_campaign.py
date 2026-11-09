#!/usr/bin/env python3
"""Verify Kinetic's original Speed of Doom UV-Max campaign and cold checkpoints.

The recording visits all 32 standard maps and ends in MAP30 intermission.
No commands or synthetic finale tail are added. The original patch contains
only titles/story strings, and is explicitly applied in the native reference.
"""
import argparse
from collections import Counter
from concurrent.futures import ThreadPoolExecutor, as_completed
import csv
import hashlib
import json
import os
from pathlib import Path
import re

from verify_boom_demos import fetch
from verify_boom_full_combat import compare_complete
from verify_boom_transfers import ROOT, digest, merge_wads, run
from verify_boom_community_coop_campaign import capture, lumps, source_digest
from verify_boom_community_coop import SOD_WAD, SOD_DEH_SHA256

ARCHIVE = '/files/demos/sodfinal/91587/sodallm24126.zip'
ARCHIVE_SHA = '1915f3fe37a0c6076cc1d77eaa32815b2ad76b6948b1df7183016cb2e1bea5a7'
DEMO_SHA = 'b9f604f03f09a1ae611b1c3e0af0c8d49ac38ee71442d7938d11e3a1a389bd13'
IWAD_SHA = '10d67824b11025ddd9198e8cfc87ca335ee6e2d3e63af4180fa9b8a471893255'
COMMANDS = 341051
MAP_ORDER = ([f'MAP{n:02d}' for n in range(1,16)] + ['MAP31','MAP32'] +
             [f'MAP{n:02d}' for n in range(16,31)])
STORIES = {'MAP06','MAP11','MAP15','MAP20','MAP31'}
RECORDED_SECONDS = 2*3600 + 41*60 + 26
# Zero-based post-command checkpoints surrounding the diagnosed interactions.
INTERACTION_CHECKPOINTS = {
    51810,51811,                    # Lost soul / dropped pickup.
    73204,73205,                    # Use-through switches.
    169874,169875,                  # Retained-sector arch-vile blast.
    170086,170087,                  # Crusher blood teleport.
    198018,198019,198020,198021,      # Doll death, self attacker and slide.
    217490,217491,                  # Unspawned teleport marker.
    278271,278272,                  # Immediately exploding mancubus volley.
    286669,286670,                  # Fallen Keen conveyor eligibility.
    333544,333545,                  # MAP30 boss landing markers.
    189850,189854,189855,           # Cold blocklink-order / rocket blast.
}


def audit_inputs(wad, demo, iwad, patch):
    raw = demo.read_bytes()
    if (len(raw) < 109 or raw[0] != 202 or raw[8:13] != bytes([3,1,1,0,0]) or
            raw[19:22] != bytes(3) or raw[77:81] != bytes([1,0,0,0])):
        raise ValueError('expected original solo UV Boom combat header')
    if digest(demo) != DEMO_SHA:
        raise ValueError('original commands or footer changed')
    end = 109
    while end < len(raw) and raw[end] != 128:
        end += 4
    if end >= len(raw) or (end - 109)//4 != COMMANDS:
        raise ValueError('original command count changed')
    if digest(iwad) != IWAD_SHA:
        raise ValueError('use the audited original Doom II 1.9 IWAD')
    data = lumps(wad)
    if not all((level,'THINGS') in data for level in MAP_ORDER):
        raise ValueError('Speed of Doom maps missing')
    patches = [body for (level,name),body in data.items() if name == 'DEHACKED']
    if len(patches) != 1 or hashlib.sha256(patches[0]).hexdigest() != SOD_DEH_SHA256:
        raise ValueError('original strings-only patch changed')
    header,strings = patches[0].decode('ascii').split('[STRINGS]',1)
    allowed = ('Patch File for DeHackEd','Doom version =','Patch format =','Engine config =','IWAD =')
    if any(not line.strip().startswith(allowed) for line in header.splitlines()
           if line.strip() and not line.lstrip().startswith('#')) or '[' in strings:
        raise ValueError('patch contains gameplay changes')
    keys = set(re.findall(r'^([A-Z0-9_]+)\s*=',strings,re.M))
    expected_keys = {f'HUSTR_{n}' for n in range(1,33)} | {'PHUSTR_1'} | {f'C{n}TEXT' for n in range(1,7)}
    if keys != expected_keys:
        raise ValueError('patch is not the audited title/story substitution set')
    patch.write_bytes(patches[0])
    return dict(player='Kinetic',category='UV Max',recorded_engine='DSDA-Doom 0.27.5 cl9',
                original_commands=COMMANDS,original_footer_preserved=True,
                gameplay_patch=False,native_original_strings=True,text_keys=sorted(keys),
                ending='MAP30 intermission; no recorded cast finale')


def reference_identity(exe, iwad, wad, demo, patch):
    return dict(commands=COMMANDS,executable=digest(exe),iwad=digest(iwad),wad=digest(wad),
                demo=digest(demo),patch=digest(patch),native_original_strings=True,
                observers={name:digest(ROOT/'scripts/visual_compare'/name) for name in
                ['dsda_coop_demo_probe.inc','dsda_intermission_probe.inc','dsda_finale_probe.inc']})


def coverage(path, commands=COMMANDS):
    maps,phases,active = [],Counter(),Counter()
    checkpoints,segments = {0,commands-1}|INTERACTION_CHECKPOINTS,[]
    intermissions,stories,deaths,rebirths = set(),set(),[],[]
    previous,start,count = None,0,0
    maxima = {}
    with path.open() as stream:
        for index,row in enumerate(csv.DictReader(stream)):
            if int(row['demotic']) != index+1:
                raise ValueError('missing/reordered native command')
            count += 1
            level,phase = row['map'],int(row['phase'])
            if not maps or maps[-1] != level:
                maps.append(level)
            phases[phase] += 1
            if phase == 0:
                active[level] += 1
            elif phase == 1:
                intermissions.add(level)
                values = list(map(int,row['wi_counts'].split(';')[0].split(':')))
                maxima[level] = [max(a,b) for a,b in zip(maxima.get(level,[-1]*4),values)]
            else:
                stories.add(level)
            roster = row['players'].strip(';').split(';')
            if len(roster) != 1 or not roster[0].startswith('1:'):
                raise ValueError('original solo roster changed')
            health = int(roster[0].split(':')[1])
            current = (level,phase,health)
            if previous:
                if current[:2] != previous[:2]:
                    checkpoints.update([index-1,index])
                    segments.append((start,index-1,previous[1]));start=index
                if phase == 0 and previous[2] > 0 >= health:
                    deaths.append(index+1);checkpoints.update([index-1,index])
                if phase == 0 and previous[2] <= 0 < health:
                    rebirths.append(index+1);checkpoints.update([index-1,index])
            if phase == 0 and active[level] % 1750 == 0:
                checkpoints.add(index)
            previous=current
    if previous:
        segments.append((start,count-1,previous[1]))
    for first,last,phase in segments:
        if phase in (1,2):
            checkpoints.add((first+last)//2)
    checks = dict(all_commands=count==COMMANDS,map_order=maps==MAP_ORDER,
        all_intermissions=intermissions==set(MAP_ORDER),story_screens=stories==STORIES,
        recorded_time=sum(t//35 for t in active.values())==RECORDED_SECONDS,
        max_kills=all(maxima.get(m,[-1])[0]>=100 for m in MAP_ORDER),
        max_secrets=all(maxima.get(m,[-1]*3)[2]>=100 for m in MAP_ORDER),
        original_ending=bool(previous and previous[:2]==('MAP30',1)))
    return dict(passed=all(checks.values()),checks=checks,commands=count,maps=maps,
        phases=dict(phases),active_tics=dict(active),recorded_seconds=RECORDED_SECONDS,
        intermission_maxima=maxima,deaths=deaths,rebirths=rebirths,
        checkpoints=sorted(checkpoints),
        checkpoint_phases={str(p):sum(first<=tic<=last for first,last,phase in segments
            if phase==p for tic in checkpoints) for p in range(3)},cast_finale_recorded=False)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad', type=Path, required=True)
    parser.add_argument('--reference', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--mode', choices=['baseline', 'save', 'keyframe'], action='append')
    parser.add_argument('--timeout', type=int, default=3600)
    parser.add_argument('--jobs', type=int, choices=[1,2,3], default=1,
                        help='independent replay processes; each has its own traces')
    args = parser.parse_args()
    out = args.out.resolve(); out.mkdir(parents=True, exist_ok=True)
    commands = COMMANDS
    result_file = out / 'results.json'
    result_file.unlink(missing_ok=True)
    wad = fetch(out, 'sodfinal.zip', *SOD_WAD)
    demo = fetch(out, 'sodallm24126.zip', ARCHIVE, ARCHIVE_SHA, 'sodallm24126.lmp')
    patch = out / 'sodfinal.deh'
    audit = audit_inputs(wad, demo, args.iwad, patch)
    folder = out
    merged = folder / 'merged.wad'
    merge_wads([args.iwad.resolve(), wad], merged)
    modes = list(dict.fromkeys(args.mode or ['baseline', 'save', 'keyframe']))
    source = source_digest()
    guards = {str(p): digest(p) for p in [Path(__file__),
        ROOT / 'scripts/verify_boom_community_coop_campaign.py',
        ROOT / 'scripts/verify_boom_full_combat.py', ROOT / 'scripts/verify_boom_demos.py',
        ROOT / 'scripts/verify_boom_transfers.py', ROOT / 'scripts/verify_boom_community_coop.py']}
    identity = reference_identity(args.reference, args.iwad, wad, demo, patch)
    env = {k:v for k,v in os.environ.items()
           if not k.startswith(('GD_COOP_','GD_DSDA_','GD_DEBUG_','GD_SURGE_','GD_SOD_'))}
    env.update(SDL_AUDIODRIVER='dummy', GD_COOP_CAMPAIGN='1', GD_COOP_COMBAT_ONLY='1',
               GD_COOP_COMPACT='1', GD_COOP_LINES='1', GD_COOP_COLD_RESTORE='1')
    expected = {kind: folder / f'strings-dsda-{kind}.csv' for kind in ['world','phase']}
    manifest = folder / 'reference.json'
    cached = json.loads(manifest.read_text()) if manifest.exists() else {}
    if not (cached.get('identity') == identity and all(path.exists() and
            cached.get('traces',{}).get(kind) == digest(path) for kind,path in expected.items())):
        manifest.unlink(missing_ok=True)
        run(['xvfb-run','-a',args.reference.resolve(),'-iwad',args.iwad.resolve(),'-file',wad,
             '-timedemo',demo,'-nodeh','-deh',patch,'-nosound','-nodraw','-config',folder/'dsda.cfg'],
            folder/'dsda.log',dict(env,GD_DSDA_COOP_OUT=str(expected['world']),
            GD_DSDA_PHASE_OUT=str(expected['phase'])),timeout=args.timeout)
        manifest.write_text(json.dumps(dict(identity=identity,traces={
            kind:digest(path) for kind,path in expected.items()}),indent=2)+'\n')
    cov = coverage(expected['phase'], commands)
    if not cov['passed']:
        raise ValueError(f'original native coverage failed: {cov["checks"]}')
    results = {}
    def replay_mode(mode):
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
        return result

    def completed_replays():
        if args.jobs == 1:
            for mode in modes:
                yield mode,replay_mode(mode)
            return
        with ThreadPoolExecutor(max_workers=args.jobs) as pool:
            pending = {pool.submit(replay_mode,mode):mode for mode in modes}
            for future in as_completed(pending):
                yield pending[future],future.result()

    for mode,result in completed_replays():
        results[mode] = result
        complete = set(results) == set(modes)
        passed = complete and all(r['passed'] for r in results.values())
        report = dict(passed=passed,complete=complete,full_suite=passed and set(results)=={'baseline','save','keyframe'},diagnostic_only=False,
            audit=audit,inputs=identity,source=source,guards=guards,requested_modes=modes,cases=results,
            scope='Original full 32-map Speed of Doom UV-Max recording; every original command '
            'and its MAP30 intermission ending are preserved. Original strings-only patch applied '
            'in the native reference; no actor or weapon patches. Every canonical active-world '
            'row and command phase, with cold restores at boundaries, screen interiors, sustained '
            'fights and exposed compatibility interactions. No cast-finale commands were recorded '
            'or appended. Excludes cosmetic RNG, decoration animation, literal custom text, pixels and audio.')
        result_file.write_text(json.dumps(report,indent=2)+'\n')
        print(mode,'passed',result['passed'],result['execution'],flush=True)
        if not result['passed'] and args.jobs == 1:
            raise SystemExit(1)
    if not all(r['passed'] for r in results.values()):
        raise SystemExit(1)


if __name__ == '__main__':
    main()
