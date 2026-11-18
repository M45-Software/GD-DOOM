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
    'sd01mc138': dict(wad='sodfinal', map=1, console=0, frames=4267, leveltime=3463,
        archive='/files/demos/sodfinal/78316/sd01mc138.zip',
        sha='6818426c0233b9145fa6f5b6f6e7a79f790d605d88c57204a3d38de15193c872',
        players=['Brainfreezzzzz','Mahik'], category='UV Max', requires_dolls=False,
        weapons_min=2),
    'sd02mc343': dict(wad='sodfinal', map=2, console=0, frames=8680, leveltime=7813,
        archive='/files/demos/sodfinal/78317/sd02mc343.zip',
        sha='b98c11bf923a4262efe263cf77d1810dbaa2129312a3ad7460f0169c9c1a9a69',
        players=['Brainfreezzzzz','Mahik'], category='UV Max', requires_dolls=False,
        weapons_min=2),
}
# The two original consoles stop on adjacent tics after entering MAP02.
for console, (name, extra) in enumerate([('sw01b028', 98), ('sw01g028', 97)]):
    CASES[name] = dict(wad='swtw', map=1, console=console, frames=1073 + extra,
        leveltime=1014, next_map=2, next_map_frames=extra,
        archive='/files/demos/swtw/61831/sw01mc028.zip',
        sha='9c4c00f10d968140a0b494a1e8dfab061777491acb208b359f7c63c0bb88de26',
        players=['4shockblast', 'GarrettChan'], category='UV Max', requires_dolls=False,
        weapons_min=2)
SOD_WAD = ('/files/wads/doom2/2615/sodfinal.zip',
           'e956bf271e1cb458a5402374729c1401a8f27e1fa7d05ee66835ff3531118497', 'SODfinal.wad')
SOD_DEH_SHA256 = '609bb4802bd956f2d31240851d76ad189968c1b4f0231ea1e6d190e1a86ae656'

COMMUNITY_WADS = {**WADS, 'sodfinal': SOD_WAD,
                  'swtw': ('/files/wads/doom2/2755/swtw.zip',
                           'c29687941b88d1888d3da49832408ba433f94b558f0379a1323781b7fce2067b', 'swtw.wad')}
STRINGS_PATCHES = {'sodfinal': SOD_DEH_SHA256,
                   'swtw': '6d619cb8bfe0a15bcfc2a4810eb61f1842941b44b0c41c097bd32f9117172c78'}


def audit_inputs(pwad, demo, case):
    raw=demo.read_bytes()
    if raw[0]!=202 or raw[8:13]!=bytes([3,1,case['map'],0,case['console']]) or raw[77:81]!=bytes([1,1,0,0]):
        raise ValueError('unexpected original Boom recording header')
    end=109
    while end<len(raw) and raw[end]!=128:end+=8
    if end>=len(raw) or (end-109)//8!=case['frames']:
        raise ValueError('original recording length changed')
    # Sunlust's pinned patch changes Keen and text; these maps contain no Keen.
    # Speed of Doom and Swim With the Whales use strings only. -nodeh excludes
    # Freedoom's unrelated weapon-frame patch. Custom text/pixels are not compared.
    raw=pwad.read_bytes();n,directory=struct.unpack_from('<ii',raw,4)
    current=None;patch=False;checked=False
    for i in range(n):
        off,size,name=struct.unpack_from('<ii8s',raw,directory+i*16);name=name.rstrip(b'\0');data=raw[off:off+size]
        if name.startswith(b'MAP') and len(name)==5:current=name
        if name==b'DEHACKED':
            expected = STRINGS_PATCHES.get(case.get('wad'), SUNLUST_DEH_SHA256)
            if hashlib.sha256(data).hexdigest()!=expected:raise ValueError('patch changed; audit before replay')
            if case.get('wad') in STRINGS_PATCHES:
                # This exact patch contains metadata and [STRINGS] only.
                # Excluding custom titles/story text changes no gameplay data.
                header, strings = data.decode('ascii').split('[STRINGS]', 1)
                lines = [line.strip() for line in header.splitlines() if line.strip() and not line.lstrip().startswith('#')]
                allowed = ('Patch File for DeHackEd', 'Doom version =', 'Patch format =', 'Engine config =', 'IWAD =')
                if any(not line.startswith(allowed) for line in lines) or '[' in strings:
                    raise ValueError('expected strings-only patch')
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
            expected_map = case['map'] if active <= case['leveltime'] else case.get('next_map')
            if expected_map is None or row['map'] != f'MAP{expected_map:02d}':
                raise ValueError('unexpected map or transition tic')
            for player in row['players'].strip(';').split(';'):
                p=list(map(int,player.split(':')))
                damage |= p[8]<100
                dead |= p[15]!=0
                if p[22]:weapons.add(p[18])
            dolls |= any(body.startswith('0:') for body in row['bodies'].strip(';').split(';'))
    checks=dict(completed=summary['completed'],expected_exit=summary['leveltime']==case['leveltime'],
                active_frames=active==case['leveltime']+case.get('next_map_frames', 0),roster=[p['slot'] for p in summary['players']]==[1,2])
    if case.get('requires_dolls', True):checks['voodoo_dolls']=dolls
    if case['category']=='UV Max':
        checks.update(max_kills=sum(p['kills'] for p in summary['players'])>=summary['total_kills']>0,
                      max_secrets=sum(p['secrets'] for p in summary['players'])>=summary['total_secrets'],
                      damaged=damage,uses_multiple_weapons=len(weapons)>=case.get('weapons_min',3))
    return dict(passed=all(checks.values()),checks=checks,weapons=sorted(weapons),death_or_rebirth=dead)


def cold_checkpoints(case):
    exit_tic = case['leveltime']
    last = case['frames'] - 1
    if 'next_map_frames' in case:
        entry = case['frames'] - case['next_map_frames']
        # Save on both sides of the tracer puff first seen at command 581,
        # the MAP01 exit, and MAP02 entry. Indices here start at zero.
        return [0, exit_tic // 2, 579, 580, exit_tic - 1, exit_tic,
                (exit_tic + entry) // 2, entry - 1, entry, last]
    combat = [15, 16, 123, 124, 399, 400] if case['map'] == 31 else [34, 35, 636, 637, 1712, 1713]
    return sorted({0, *combat, exit_tic - 1, exit_tic, (exit_tic + case['frames']) // 2,
                   last - 1, last})


def source_digest():
    source=hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):
        source.update(str(path.relative_to(ROOT)).encode()+b'\0'+path.read_bytes())
    return source.hexdigest()


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--iwad',type=Path,required=True)
    ref=p.add_mutually_exclusive_group(required=True);ref.add_argument('--reference',type=Path);ref.add_argument('--reference-source',type=Path)
    p.add_argument('--cmake-arg',action='append',default=[]);p.add_argument('--out',type=Path,required=True);p.add_argument('--case',action='append',choices=CASES);p.add_argument('--mode',action='append',choices=['baseline','save','keyframe'])
    args=p.parse_args();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True)
    exe=args.reference.resolve() if args.reference else build_reference(args.reference_source,out,args.cmake_arg)
    source,verifier=source_digest(),digest(Path(__file__))
    results={}
    for name,case in CASES.items():
        if args.case and name not in args.case:continue
        folder=out/name;folder.mkdir(exist_ok=True)
        corpus=case.get('wad','sunlust')
        pwad=fetch(folder,corpus+'.zip',*COMMUNITY_WADS[corpus])
        lmp=fetch(folder,Path(case['archive']).name,case['archive'],case['sha'],name+'.lmp')
        audit_inputs(pwad,lmp,case);merged=folder/'merged.wad';merge_wads([args.iwad.resolve(),pwad],merged)
        env=dict(os.environ,SDL_AUDIODRIVER='dummy')
        for key in list(env):
            if key.startswith(('GD_COOP_','GD_DSDA_','GD_DEBUG_')):env.pop(key)
        env.update(GD_COOP_CAMPAIGN='1',GD_COOP_COMBAT_ONLY='1',GD_COOP_LINES='1',GD_COOP_COLD_RESTORE='1')
        for file in ['dsda.csv','dsda-summary.json']:(folder/file).unlink(missing_ok=True)
        run(['xvfb-run','-a',exe,'-iwad',args.iwad.resolve(),'-file',pwad,'-timedemo',lmp,'-nodeh','-nosound','-nodraw','-config',folder/'dsda.cfg'],folder/'dsda.log',dict(env,GD_DSDA_COOP_OUT=str(folder/'dsda.csv'),GD_DSDA_PHASE_OUT=str(folder/'dsda-phase.csv'),GD_COOP_SUMMARY=str(folder/'dsda-summary.json')))
        summary=json.loads((folder/'dsda-summary.json').read_text());cov=coverage(folder/'dsda.csv',summary,case)
        checkpoints = cold_checkpoints(case)
        for mode in args.mode or ['baseline','save','keyframe']:
            actual=folder/f'{mode}.csv';sp=folder/f'{mode}-summary.json';actual.unlink(missing_ok=True);sp.unlink(missing_ok=True)
            phase=folder/f'{mode}-phase.csv'
            run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime','-run','^TestBoomCoopDemoCapture$','-count=1'],folder/f'{mode}.log',dict(env,GD_COOP_OUT=str(actual),GD_COOP_PHASE_OUT=str(phase),GD_COOP_WAD=str(merged),GD_COOP_DEMO=str(lmp),GD_COOP_SUMMARY=str(sp),GD_COOP_CHECKPOINT_TICS='' if mode=='baseline' else ','.join(map(str,checkpoints)),GD_COOP_CHECKPOINT_FORMAT=mode))
            result=compare_complete(actual,folder/'dsda.csv',case['leveltime']+case.get('next_map_frames', 0));got=json.loads(sp.read_text()) if sp.exists() else None
            result['phase']=compare_complete(phase,folder/'dsda-phase.csv',case['frames'])
            result.update(coverage=cov,summary_match=got==summary,gd_summary=got,dsda_summary=summary,restores=0 if mode=='baseline' else len(checkpoints),inputs=dict(case=case,wad=digest(pwad),demo=digest(lmp)))
            result['passed'] &= cov['passed'] and result['summary_match'] and result['phase']['passed'];results[name+'_'+mode]=result
            print(name,mode,result['matched'],'/',result['rows'],'summary',result['summary_match'],'coverage',cov['checks'],flush=True)
            if not result['passed']:print(json.dumps(result['failures'][:1],indent=2)[:5000],flush=True)
    if source_digest()!=source or digest(Path(__file__))!=verifier:
        raise ValueError('source/verifier changed during community verification; replay this attempt')
    report=dict(passed=bool(results) and all(r['passed'] for r in results.values()),cases=results,metadata=dict(reference=digest(exe),iwad=digest(args.iwad),gd_source=source,verifier=verifier,observer=digest(ROOT/'scripts/visual_compare/dsda_coop_demo_probe.inc')),scope='Original Sunlust, Speed of Doom and Swim With the Whales community co-op recordings, retaining each original console header. Active frames, all recorded campaign phases, player/combat-body state, pickups, sectors, line specials, gameplay RNG and independent exit totals; cold save/keyframe restores include intermission interiors. Excludes static decoration animation, cosmetic pr_misc, custom story text and audio/visual parity. Individual maps ending in intermission are not full campaign coverage.')
    (out/'results.json').write_text(json.dumps(report,indent=2)+'\n');raise SystemExit(0 if report['passed'] else 1)

if __name__=='__main__':main()
