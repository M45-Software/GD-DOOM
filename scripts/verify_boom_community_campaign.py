#!/usr/bin/env python3
"""Replay unchanged, hash-pinned community Boom campaigns and screen checkpoints.

Active gameplay and every command's campaign phase are independent DSDA traces.
No replacement commands, injected exits or synthetic tails are used.
"""
import argparse
import csv
import hashlib
import json
import os
from pathlib import Path
import struct

from verify_boom_demos import WADS, fetch
from verify_boom_full_combat import SUNLUST_DEH_SHA256
from verify_boom_transfers import ROOT, build_reference, digest, merge_wads, run
from verify_boom_transitions import compare

CASES = {
    'slep1o546': dict(archive='/files/demos/sunlust/50338/slep1o546.zip',
        sha='cd0eeee3ed88ca2955c8dca5293b9b322aaff6b26a9c4bde2cd8853922a11b6d',
        player='Teedre (TomatoTom)', category='NoMo', commands=12995, nomonsters=True),
    'sle1x344': dict(archive='/files/demos/sunlust/48879/sle1x344.zip',
        sha='4bfde799ea002b2b01041b5b039fde308230a94ddaaaafb1231a03fdfc70b09c',
        player='SuperWeaponDude', category='UV Speed TAS', commands=8702, nomonsters=False),
}


def audit_inputs(pwad, lmp, case):
    raw = lmp.read_bytes()
    if raw[:1] != bytes([202]) or raw[8:13] != bytes([3,1,1,0,0]) or raw[77:81] != bytes([1,0,0,0]) or bool(raw[21]) != case['nomonsters']:
        raise ValueError('unexpected original campaign header')
    end = 109
    while end < len(raw) and raw[end] != 128:
        end += 4
    if end >= len(raw) or (end-109)//4 != case['commands']:
        raise ValueError('original campaign command count changed')
    raw = pwad.read_bytes()
    count, directory = struct.unpack_from('<ii', raw, 4)
    current, checked, patch = None, set(), False
    for i in range(count):
        off,size,name = struct.unpack_from('<ii8s',raw,directory+i*16)
        name,data = name.rstrip(b'\0'),raw[off:off+size]
        if name.startswith(b'MAP') and len(name)==5:
            current=name.decode()
        if name==b'DEHACKED':
            if hashlib.sha256(data).hexdigest()!=SUNLUST_DEH_SHA256:
                raise ValueError('Sunlust patch changed; audit before replay')
            patch=True
        if name==b'THINGS' and current in {f'MAP{n:02d}' for n in range(1,11)}:
            if any(t[3]==72 for t in struct.iter_unpack('<5h',data)):
                raise ValueError('campaign contains a patched Keen')
            checked.add(current)
    if not patch or len(checked)!=10:
        raise ValueError('missing audited maps or patch')


def campaign_coverage(path, case):
    with path.open() as f:
        rows=list(csv.DictReader(f))
    maps=list(dict.fromkeys(r['map'] for r in rows))
    phases={p:sum(r['phase']==str(p) for r in rows) for p in range(3)}
    changes=[i for i,r in enumerate(rows) if i and (r['map'],r['phase'])!=(rows[i-1]['map'],rows[i-1]['phase'])]
    checkpoints={0,len(rows)-1}
    # Each boundary is restored before and at it. Interior points also
    # exercise animated counters and story text, rather than only phase edges.
    for i in changes:
        checkpoints.update(j for j in [i-1,i] if 0<=j<len(rows))
    for phase in ['1','2']:
        start=None
        for i in range(len(rows)+1):
            active=i<len(rows) and rows[i]['phase']==phase
            if active and start is None:start=i
            if not active and start is not None:
                checkpoints.add((start+i-1)//2)
                start=None
    checks=dict(all_commands=len(rows)==case['commands'],
        maps=maps==[f'MAP{n:02d}' for n in range(1,11)],
        intermissions=all(any(r['map']==m and r['phase']=='1' for r in rows) for m in maps),
        story_finale=any(r['map']=='MAP06' and r['phase']=='2' for r in rows),
        no_unaudited_maps=set(maps)=={f'MAP{n:02d}' for n in range(1,11)})
    return dict(passed=all(checks.values()),checks=checks,maps=maps,phases=phases,
                checkpoints=sorted(checkpoints),
                checkpoint_phases={str(p):sum(rows[i]['phase']==str(p) for i in checkpoints) for p in range(3)})


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--iwad',required=True,type=Path)
    ref=p.add_mutually_exclusive_group(required=True)
    ref.add_argument('--reference',type=Path);ref.add_argument('--reference-source',type=Path)
    p.add_argument('--cmake-arg',action='append',default=[])
    p.add_argument('--out',required=True,type=Path)
    p.add_argument('--case',action='append',choices=CASES)
    p.add_argument('--mode',action='append',choices=['baseline','save','keyframe'])
    args=p.parse_args();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True)
    exe=args.reference.resolve() if args.reference else build_reference(args.reference_source,out,args.cmake_arg)
    results={}
    for name,case in CASES.items():
        if args.case and name not in args.case:continue
        folder=out/name;folder.mkdir(exist_ok=True)
        pwad=fetch(folder,'sunlust.zip',*WADS['sunlust'])
        lmp=fetch(folder,Path(case['archive']).name,case['archive'],case['sha'],name+'.lmp')
        audit_inputs(pwad,lmp,case)
        merged=folder/'merged.wad';merge_wads([args.iwad.resolve(),pwad],merged)
        env=dict(os.environ,SDL_AUDIODRIVER='dummy')
        for k in list(env):
            if k.startswith(('GD_COOP_','GD_DSDA_','GD_DEBUG_')):env.pop(k)
        env.update(GD_COOP_CAMPAIGN='1',GD_COOP_COMBAT_ONLY='1',GD_COOP_COLD_RESTORE='1')
        run(['xvfb-run','-a',exe,'-iwad',args.iwad.resolve(),'-file',pwad,'-timedemo',lmp,'-nodeh','-nosound','-nodraw','-config',folder/'dsda.cfg'],folder/'dsda.log',dict(env,GD_DSDA_COOP_OUT=str(folder/'dsda-world.csv'),GD_DSDA_PHASE_OUT=str(folder/'dsda-phase.csv')))
        coverage=campaign_coverage(folder/'dsda-phase.csv',case)
        for mode in args.mode or ['baseline','save','keyframe']:
            run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime','-run','^TestBoomCoopDemoCapture$','-count=1'],folder/f'{mode}.log',dict(env,GD_COOP_OUT=str(folder/f'{mode}-world.csv'),GD_COOP_PHASE_OUT=str(folder/f'{mode}-phase.csv'),GD_COOP_WAD=str(merged),GD_COOP_DEMO=str(lmp),GD_COOP_CHECKPOINT_TICS='' if mode=='baseline' else ','.join(map(str,coverage['checkpoints'])),GD_COOP_CHECKPOINT_FORMAT=mode),timeout=1200)
            world=compare(folder/f'{mode}-world.csv',folder/'dsda-world.csv')
            phase=compare(folder/f'{mode}-phase.csv',folder/'dsda-phase.csv')
            result=dict(passed=world['passed'] and phase['passed'] and coverage['passed'],world=world,phase=phase,coverage=coverage,restores=0 if mode=='baseline' else len(coverage['checkpoints']),inputs=dict(settings=case,wad=digest(pwad),demo=digest(lmp)))
            results[name+'_'+mode]=result
            print(name,mode,'world',world['matched'],'/',world['rows'],'phase',phase['matched'],'/',phase['rows'],flush=True)
            if not result['passed']:print(json.dumps(dict(world=world['failures'][:1],phase=phase['failures'][:1],coverage=coverage['checks']),indent=2)[:5000],flush=True)
    source=hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):source.update(str(path.relative_to(ROOT)).encode()+b'\0'+path.read_bytes())
    report=dict(passed=bool(results) and all(r['passed'] for r in results.values()),cases=results,metadata=dict(reference=digest(exe),iwad=digest(args.iwad),gd_source=source.hexdigest(),verifier=digest(Path(__file__)),observer=digest(ROOT/'scripts/visual_compare/dsda_coop_demo_probe.inc'),intermission_observer=digest(ROOT/'scripts/visual_compare/dsda_intermission_probe.inc'),finale_observer=digest(ROOT/'scripts/visual_compare/dsda_finale_probe.inc')),scope='Unmodified Sunlust MAP01-10 community single-player campaign recordings. Full active combat-body, pickup, sector and gameplay RNG state plus every command phase, roster inventory/latches, intermission counters and finale/cast state. Save/keyframe restores include phase interiors and boundaries. Excludes cosmetic pr_misc, decoration animation, embedded DeHackEd text and audio/visual parity; selected maps are audited to contain no patched Keen.')
    (out/'results.json').write_text(json.dumps(report,indent=2)+'\n')
    raise SystemExit(0 if report['passed'] else 1)

if __name__=='__main__':main()
