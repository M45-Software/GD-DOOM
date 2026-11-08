#!/usr/bin/env python3
"""Replay hash-pinned, unmodified community Boom co-op demos against DSDA 0.27.5.

Each original console recording is tested continuously and through full-roster
save/keyframe restores. Compare every active map tic and independent exit stats.
Static decoration animation and cosmetic pr_misc are outside this profile.
"""
import argparse
import csv
import hashlib
import json
import os
from pathlib import Path
import struct

from verify_boom_demos import WADS, fetch
from verify_boom_full_combat import SUNLUST_DEH_SHA256, compare_complete
from verify_boom_transfers import ROOT, build_reference, digest, merge_wads, run

CASES = {
    'sl31b014': dict(map=31, console=0, frames=699, leveltime=503,
        archive='/files/demos/sunlust/61512/sl31pc014.zip',
        sha='e4bdb6a07af4dfaaeb5be7bf99c4eb191a462833b0ce82181118401945cc1085',
        players=['4shockblast','GarrettChan'], category='UV Pacifist'),
    'sl31g014': dict(map=31, console=1, frames=699, leveltime=503,
        archive='/files/demos/sunlust/61512/sl31pc014.zip',
        sha='e4bdb6a07af4dfaaeb5be7bf99c4eb191a462833b0ce82181118401945cc1085',
        players=['4shockblast','GarrettChan'], category='UV Pacifist'),
    'sl32b125': dict(map=32, console=0, frames=3306, leveltime=3006,
        archive='/files/demos/sunlust/92079/sl32mc125.zip',
        sha='51d3f5a5c2100b418f2d5caaade9dad0059f4861bd65b78015d15606e770a8a8',
        players=['4shockblast','GarrettChan'], category='UV Max'),
    'sl32g125': dict(map=32, console=1, frames=3305, leveltime=3006,
        archive='/files/demos/sunlust/92079/sl32mc125.zip',
        sha='51d3f5a5c2100b418f2d5caaade9dad0059f4861bd65b78015d15606e770a8a8',
        players=['4shockblast','GarrettChan'], category='UV Max'),
}


def audit_inputs(pwad, demo, case):
    raw=demo.read_bytes()
    if raw[0]!=202 or raw[8:13]!=bytes([3,1,case['map'],0,case['console']]) or raw[77:81]!=bytes([1,1,0,0]):
        raise ValueError('unexpected original Boom recording header')
    end=109
    while end<len(raw) and raw[end]!=128:end+=8
    if end>=len(raw) or (end-109)//8!=case['frames']:
        raise ValueError('original recording length changed')
    # This pinned patch changes only Commander Keen and his states, plus text.
    # The selected maps contain no Keen. -nodeh excludes that unused patch and
    # Freedoom's unrelated weapon-frame patch in the reference. We verify active
    # map state/exit, not Sunlust's custom story text or rendered sprites.
    raw=pwad.read_bytes();n,directory=struct.unpack_from('<ii',raw,4)
    current=None;patch=False;checked=False
    for i in range(n):
        off,size,name=struct.unpack_from('<ii8s',raw,directory+i*16);name=name.rstrip(b'\0');data=raw[off:off+size]
        if name.startswith(b'MAP') and len(name)==5:current=name
        if name==b'DEHACKED':
            if hashlib.sha256(data).hexdigest()!=SUNLUST_DEH_SHA256:raise ValueError('gameplay patch changed; audit before replay')
            patch=True
        if name==b'THINGS' and current==f"MAP{case['map']:02d}".encode():
            if any(t[3]==72 for t in struct.iter_unpack('<5h',data)):raise ValueError('map requires the Keen patch')
            checked=True
    if not patch or not checked:raise ValueError('missing audited map or patch')


def coverage(path,summary,case):
    active=0;dolls=False;damage=False;weapons=set();dead=False
    with path.open() as f:
        for row in csv.DictReader(f):
            active+=1
            if row['map']!=f"MAP{case['map']:02d}":raise ValueError('recording entered an unaudited map')
            for player in row['players'].strip(';').split(';'):
                p=list(map(int,player.split(':')))
                damage |= p[8]<100
                dead |= p[15]!=0
                if p[22]:weapons.add(p[18])
            dolls |= any(body.startswith('0:') for body in row['bodies'].strip(';').split(';'))
    checks=dict(completed=summary['completed'],expected_exit=summary['leveltime']==case['leveltime'],
                active_frames=active==case['leveltime'],roster=[p['slot'] for p in summary['players']]==[1,2],voodoo_dolls=dolls)
    if case['category']=='UV Max':
        checks.update(max_kills=sum(p['kills'] for p in summary['players'])>=summary['total_kills']>0,
                      max_secrets=sum(p['secrets'] for p in summary['players'])>=summary['total_secrets'],
                      damaged=damage,uses_multiple_weapons=len(weapons)>=3)
    return dict(passed=all(checks.values()),checks=checks,weapons=sorted(weapons),death_or_rebirth=dead)


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--iwad',type=Path,required=True)
    ref=p.add_mutually_exclusive_group(required=True);ref.add_argument('--reference',type=Path);ref.add_argument('--reference-source',type=Path)
    p.add_argument('--cmake-arg',action='append',default=[]);p.add_argument('--out',type=Path,required=True);p.add_argument('--case',action='append',choices=CASES);p.add_argument('--mode',action='append',choices=['baseline','save','keyframe'])
    args=p.parse_args();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True)
    exe=args.reference.resolve() if args.reference else build_reference(args.reference_source,out,args.cmake_arg)
    results={}
    for name,case in CASES.items():
        if args.case and name not in args.case:continue
        folder=out/name;folder.mkdir(exist_ok=True)
        pwad=fetch(folder,'sunlust.zip',*WADS['sunlust'])
        lmp=fetch(folder,Path(case['archive']).name,case['archive'],case['sha'],name+'.lmp')
        audit_inputs(pwad,lmp,case);merged=folder/'merged.wad';merge_wads([args.iwad.resolve(),pwad],merged)
        env=dict(os.environ,SDL_AUDIODRIVER='dummy')
        for key in list(env):
            if key.startswith(('GD_COOP_','GD_DSDA_','GD_DEBUG_')):env.pop(key)
        env.update(GD_COOP_CAMPAIGN='1',GD_COOP_COMBAT_ONLY='1')
        for file in ['dsda.csv','dsda-summary.json']:(folder/file).unlink(missing_ok=True)
        run(['xvfb-run','-a',exe,'-iwad',args.iwad.resolve(),'-file',pwad,'-timedemo',lmp,'-nodeh','-nosound','-nodraw','-config',folder/'dsda.cfg'],folder/'dsda.log',dict(env,GD_DSDA_COOP_OUT=str(folder/'dsda.csv'),GD_COOP_SUMMARY=str(folder/'dsda-summary.json')))
        summary=json.loads((folder/'dsda-summary.json').read_text());cov=coverage(folder/'dsda.csv',summary,case)
        checkpoints=([0,15,16,123,124,399,400,case['leveltime']-1,case['leveltime'],case['frames']-2]
                     if case['map']==31 else [0,34,35,636,637,1712,1713,case['leveltime']-1,case['leveltime'],case['frames']-2])
        for mode in args.mode or ['baseline','save','keyframe']:
            actual=folder/f'{mode}.csv';sp=folder/f'{mode}-summary.json';actual.unlink(missing_ok=True);sp.unlink(missing_ok=True)
            run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime','-run','^TestBoomCoopDemoCapture$','-count=1'],folder/f'{mode}.log',dict(env,GD_COOP_OUT=str(actual),GD_COOP_WAD=str(merged),GD_COOP_DEMO=str(lmp),GD_COOP_SUMMARY=str(sp),GD_COOP_CHECKPOINT_TICS='' if mode=='baseline' else ','.join(map(str,checkpoints)),GD_COOP_CHECKPOINT_FORMAT=mode))
            result=compare_complete(actual,folder/'dsda.csv',case['leveltime']);got=json.loads(sp.read_text()) if sp.exists() else None
            result.update(coverage=cov,summary_match=got==summary,gd_summary=got,dsda_summary=summary,restores=0 if mode=='baseline' else len(checkpoints),inputs=dict(case=case,wad=digest(pwad),demo=digest(lmp)))
            result['passed'] &= cov['passed'] and result['summary_match'];results[name+'_'+mode]=result
            print(name,mode,result['matched'],'/',result['rows'],'summary',result['summary_match'],'coverage',cov['checks'],flush=True)
            if not result['passed']:print(json.dumps(result['failures'][:1],indent=2)[:5000],flush=True)
    source=hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):source.update(str(path.relative_to(ROOT)).encode()+b'\0'+path.read_bytes())
    report=dict(passed=bool(results) and all(r['passed'] for r in results.values()),cases=results,metadata=dict(reference=digest(exe),iwad=digest(args.iwad),gd_source=source.hexdigest(),verifier=digest(Path(__file__)),observer=digest(ROOT/'scripts/visual_compare/dsda_coop_demo_probe.inc')),scope='Unchanged Sunlust community co-op recordings, both original console views. Active frames, player/combat-body state, pickups, sectors, gameplay RNG and independent exit totals; excludes static decoration animation, cosmetic pr_misc, custom story text and audio/visual parity.')
    (out/'results.json').write_text(json.dumps(report,indent=2)+'\n');raise SystemExit(0 if report['passed'] else 1)

if __name__=='__main__':main()
