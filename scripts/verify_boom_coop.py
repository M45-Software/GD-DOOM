#!/usr/bin/env python3
"""Compare real co-op Boom command frames, player/body state and RNG with DSDA."""
import argparse
import csv
import hashlib
import json
import os
import struct
from pathlib import Path

from verify_boom_rng import demo, compare
from verify_boom_voodoo import make_fixture
from verify_boom_transfers import ROOT, build_reference, digest, merge_wads, run


def alter_fixture(path, hazard=False, mode=None):
    raw=path.read_bytes();count,directory=struct.unpack_from('<ii',raw,4);lumps=[]
    for i in range(count):
        offset,size,name=struct.unpack_from('<ii8s',raw,directory+i*16)
        data=bytearray(raw[offset:offset+size])
        label=name.rstrip(b'\0')
        if label==b'SECTORS':
            if hazard:struct.pack_into('<h',data,22,16)
            if mode in ['conveyor','crusher']:
                for j in range(0,len(data),26):struct.pack_into('<h',data,j+24,7 if j==0 else 0)
        if label==b'LINEDEFS':
            if mode=='conveyor':struct.pack_into('<HH',data,14+6,253,7)
            if mode=='crusher':struct.pack_into('<HH',data,2*14+6,25,7)
        if label==b'THINGS' and mode=='monsters':
            data+=struct.pack('<5h',0,-48,180,3001,7)+struct.pack('<5h',256,48,180,3004,7)
        if label==b'THINGS' and mode=='pickups':
            for x in [-256,-128]:
                for typ in [5,2001,2007]:data+=struct.pack('<5h',x+4,48,0,typ,7)
        if label==b'THINGS' and mode=='sparse':
            for j in range(0,len(data),10):
                typ=struct.unpack_from('<h',data,j+6)[0]
                struct.pack_into('<h',data,j+6,typ+2)
        if label==b'THINGS' and mode=='four':
            things=[]
            for slot,x in enumerate([-320,-224,-32,64],1):
                things.extend([(x,-80,0,slot,7),(x,80,0,slot,7)])
            data=bytearray(b''.join(struct.pack('<5h',*th) for th in things))
        lumps.append((name,data))
    payload,entries=bytearray(12),bytearray()
    for name,data in lumps:entries+=struct.pack('<ii8s',len(payload),len(data),name);payload+=data
    struct.pack_into('<4sii',payload,0,b'PWAD',len(lumps),len(payload));path.write_bytes(payload+entries)


def coverage(path, settings):
    with path.open() as f:
        rows=list(csv.DictReader(f))
    players=[[list(map(int,p.split(':'))) for p in row['players'].strip(';').split(';')] for row in rows]
    bodies=[[list(map(int,b.split(':'))) for b in row['bodies'].strip(';').split(';') if b] for row in rows]
    mode=settings.get('mode')
    slots=[3,4] if mode=='sparse' else [1,2,3,4] if mode=='four' else [1,2]
    checks=dict(active_slots=bool(rows) and all([p[0] for p in frame]==slots for frame in players))
    flat=[p for frame in players for p in frame]
    if settings.get('hazard') or mode in ['crusher','monsters']:
        checks.update(death=any(p[15]==1 for p in flat),pending_rebirth=any(p[15]==2 for p in flat),
                      retained_body=any(b[0]==0 and b[15]==0 for frame in bodies for b in frame))
    if settings.get('combat'):
        checks.update(both_players_hurt=all(p[8]<100 for p in players[-1]),both_players_fired=all(p[11]<50 for p in players[-1]))
    if mode=='pickups':
        checks.update(keys_owned=all(p[24]&1 for p in players[-1]),
                      weapons_retained=all(len(row['items'].strip(';').split(';'))==4 for row in rows),
                      both_shotguns_fired=all(p[12]<8 for p in players[-1]))
    if mode=='crusher':
        checks.update(ceiling_moves=rows[0]['sectors']!=rows[-1]['sectors'],
                      doll_kills_live_camera=any(p[15]==1 and p[9]>0 for p in flat),
                      live_body_retained=any(b[0]==0 and b[15]==0 and b[8]>0 for frame in bodies for b in frame))
    if mode=='pause':checks['paused_world_tics']=len(set(row['tic'] for row in rows))<len(rows)
    if mode=='conveyor':
        checks.update(neutral_players_carried=any(a[1:3]!=b[1:3] for a,b in zip(players[0],players[-1])),
                      doll_carried=bodies[0]!=bodies[-1])
    if mode in ['sparse','four'] or not mode and not settings.get('hazard') and not settings.get('combat'):
        checks['commanded_players_move']=all(a[1:3]!=b[1:3] for a,b in zip(players[0][:2],players[-1][:2]))
    return dict(passed=all(checks.values()),checks=checks)


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad',required=True,type=Path)
    group=parser.add_mutually_exclusive_group(required=True)
    group.add_argument('--reference',type=Path)
    group.add_argument('--reference-source',type=Path)
    parser.add_argument('--cmake-arg',action='append',default=[])
    parser.add_argument('--out',type=Path,default=ROOT/'tmp/boom-coop')
    parser.add_argument('--case',action='append')
    args=parser.parse_args();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True)
    exe=args.reference.resolve() if args.reference else build_reference(args.reference_source,out/'reference',args.cmake_arg)
    cases={f'movement_i{i}_s{s}':dict(insurance=i,seed=s) for i in [0,1] for s in [0,1993]}
    cases.update({f'combat_i{i}_s{s}':dict(insurance=i,seed=s,combat=True) for i in [0,1] for s in [0,1993]})
    cases.update({f'rebirth_i{i}_s{s}':dict(insurance=i,seed=s,hazard=True) for i in [0,1] for s in [0,1993]})
    cases.update({f'{mode}_i{i}_s{s}':dict(insurance=i,seed=s,mode=mode) for mode in ['conveyor','crusher','sparse','four','monsters','pickups','pause'] for i in [0,1] for s in [0,1993]})
    if args.case and set(args.case)-set(cases):parser.error('unknown cases: '+','.join(sorted(set(args.case)-set(cases))))
    results={}
    for name,settings in cases.items():
        if args.case and name not in args.case:continue
        folder=out/name;folder.mkdir(exist_ok=True)
        pwad,merged,lmp=[folder/n for n in ['coop.wad','merged.wad','coop.lmp']]
        make_fixture(pwad);alter_fixture(pwad,hazard=settings.get('hazard',False),mode=settings.get('mode'));merge_wads([args.iwad.resolve(),pwad],merged)
        demo(lmp,settings['seed'],settings['insurance'],attack=False,tics=1)
        header=bytearray(lmp.read_bytes()[:109]);header[78]=1
        mode=settings.get('mode')
        if mode=='sparse':header[77:81]=bytes([0,0,1,1]);header[12]=3
        if mode=='four':header[77:81]=bytes([1,1,1,1])
        commands=bytearray();tics=360 if mode=='monsters' else 520 if mode=='crusher' else 420 if settings.get('hazard') else 80 if settings.get('combat') else 160
        for tic in range(tics):
            if mode=='crusher':
                commands+=bytes([25 if tic<40 else 0,0,0,2 if tic in [200,400] else 0])
                commands+=bytes([25 if tic<40 else 0,0,64 if tic==0 else 0,2 if tic in [200,400] else 0])
            elif mode=='conveyor':
                commands+=bytes(8)
            elif mode=='monsters':
                commands+=bytes([0,0,0,1 | (2 if tic in [200,300] else 0)])*2
            elif mode=='pickups':
                commands+=bytes([1 if tic<60 else 0,0,0,1 if tic>70 else 0])*2
            elif settings.get('hazard'):
                commands+=bytes([0,0,0,2 if tic in [200,400] else 0])*2
            elif settings.get('combat'):
                commands+=bytes([0,0,0,1])
                commands+=bytes([0,0,128 if tic==0 else 0,1])
            else:
                commands+=bytes([25 if tic<16 else 0,0,0,0])
                commands+=bytes([25 if 20<=tic<48 else 0,0,64 if tic==0 else 0,0])
                if mode=='four':commands+=bytes([0,0,0,0,0,0,0,0])
        if mode=='pause':
            for tic in [20,30]:commands[tic*8+7]=129
        lmp.write_bytes(header+commands+b'\x80')
        env=dict(os.environ)
        for n in ['gd.csv','dsda.csv']:(folder/n).unlink(missing_ok=True)
        run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime','-run','^TestBoomCoopDemoCapture$','-count=1'],folder/'gd.log',dict(env,GD_COOP_OUT=str(folder/'gd.csv'),GD_COOP_WAD=str(merged),GD_COOP_DEMO=str(lmp)))
        run(['xvfb-run','-a',exe,'-iwad',args.iwad.resolve(),'-file',pwad,'-nodeh','-timedemo',lmp,'-nosound','-nodraw','-config',folder/'dsda.cfg'],folder/'dsda.log',dict(env,GD_DSDA_COOP_OUT=str(folder/'dsda.csv')))
        results[name]=compare(folder/'gd.csv',folder/'dsda.csv',tics)
        results[name]['coverage']=coverage(folder/'dsda.csv',settings)
        results[name]['passed'] &= results[name]['coverage']['passed']
        results[name]['inputs']=dict(settings=settings,wad=digest(pwad),demo=digest(lmp))
        print(name,results[name]['matched'],'/',tics,flush=True)
        if not results[name]['passed']:print(json.dumps(results[name]['failures'][:1],indent=2),flush=True)
    source=hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):source.update(str(path.relative_to(ROOT)).encode()+b'\0'+path.read_bytes())
    report=dict(passed=bool(results) and all(r['passed'] for r in results.values()),cases=results,
                metadata=dict(reference=digest(exe),iwad=digest(args.iwad),gd_source=source.hexdigest(),
                              verifier=digest(Path(__file__)),observer=digest(ROOT/'scripts/visual_compare/dsda_coop_demo_probe.inc')),
                scope='real generated single-map co-op demos; gameplay RNG excludes cosmetic pr_misc; refire is compared as a boolean')
    (out/'results.json').write_text(json.dumps(report,indent=2)+'\n')
    raise SystemExit(0 if report['passed'] else 1)


if __name__=='__main__':main()
