#!/usr/bin/env python3
"""Compare Boom co-op map transitions and full-roster checkpoints with DSDA."""
import argparse,csv,hashlib,json,os,struct
from pathlib import Path
from verify_boom_transitions import make_fixture,compare
from verify_boom_rng import demo
from verify_boom_transfers import ROOT,merge_wads,run,digest


def fixture(path,secret,slots,dead=False):
    make_fixture(path,secret)
    raw=path.read_bytes();count,directory=struct.unpack_from('<ii',raw,4);lumps=[];second=False
    for i in range(count):
        pos,size,name=struct.unpack_from('<ii8s',raw,directory+i*16);data=raw[pos:pos+size];label=name.rstrip(b'\0')
        if label in [b'MAP02',b'MAP31']:second=True
        if label==b'THINGS':
            things=[]
            for j,slot in enumerate(slots):
                y=-96+j*64
                things += [(-320,y,0,slot,7),(-256,y,0,slot,7)]
                if not second:things += [(-256,y,0,[2001,2002,2003,2004][j],7),(-256,y,0,[5,13,6,2018][j],7),(-256,y,0,2007,7)]
            things += [(-64,128,180,3004,7)] if second else []
            if dead and not second: things += [(-256,-32,0,2022,7)]
            data=b''.join(struct.pack('<5h',*th) for th in things)
        lumps.append((name,data))
    payload,entries=bytearray(12),bytearray()
    for name,data in lumps:entries+=struct.pack('<ii8s',len(payload),len(data),name);payload+=data
    struct.pack_into('<4sii',payload,0,b'PWAD',len(lumps),len(payload));path.write_bytes(payload+entries)


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--iwad',required=True,type=Path);p.add_argument('--reference',required=True,type=Path);p.add_argument('--out',required=True,type=Path);p.add_argument('--case',action='append');args=p.parse_args()
    out=args.out.resolve();out.mkdir(parents=True,exist_ok=True);results={}
    cases={f'{kind}_{roster}':dict(secret=kind=='secret',slots=slots) for kind in ['normal','secret'] for roster,slots in [('two',[1,2]),('sparse',[3,4]),('four',[1,2,3,4])]}
    cases['normal_dead']=dict(secret=False,slots=[1,2],dead=True)
    cases['normal_natural_sparse']=dict(secret=False,slots=[3,4],natural=True)
    cases['normal_natural']=dict(secret=False,slots=[1,2],natural=True)
    cases['secret_held']=dict(secret=True,slots=[1,2],held=True)
    if args.case and set(args.case)-cases.keys():p.error('unknown case')
    for name,c in cases.items():
        if args.case and name not in args.case:continue
        folder=out/name;folder.mkdir(exist_ok=True);pwad=folder/'campaign.wad';merged=folder/'merged.wad';lmp=folder/'campaign.lmp'
        fixture(pwad,c['secret'],c['slots'],c.get('dead',False));merge_wads([args.iwad.resolve(),pwad],merged);demo(lmp,1993,1,attack=False,tics=1)
        h=bytearray(lmp.read_bytes()[:109]);h[10]=15 if c['secret'] else 1;h[12]=c['slots'][-1]-1;h[77:81]=bytes(int(i+1 in c['slots']) for i in range(4))
        tics=1800 if c.get('dead') else 1500 if c.get('held') else 800;commands=bytearray()
        for tic in range(tics):
            for slot in c['slots']:
                buttons=1 if slot==c['slots'][0] and tic%20==1 and (not c.get('natural') or tic>500) else 0
                if c.get('held') and slot==c['slots'][0] and tic>=81:buttons=1
                moving=(tic<8 or 650<=tic<730) and slot==2 if c.get('dead') else tic<80
                commands+=bytes([25 if moving else 0,0,0,buttons])
        lmp.write_bytes(h+commands+b'\x80')
        env=dict(os.environ,GD_COOP_CAMPAIGN='1')
        run(['xvfb-run','-a',args.reference.resolve(),'-iwad',args.iwad.resolve(),'-file',pwad,'-nodeh','-timedemo',lmp,'-nosound','-nodraw','-config',folder/'dsda.cfg'],folder/'dsda.log',dict(env,GD_DSDA_COOP_OUT=str(folder/'dsda.csv')))
        with (folder/'dsda.csv').open() as f:rows=list(csv.DictReader(f))
        maps=sorted(set(r['map'] for r in rows));exits=[int(rows[i-1]['demotic']) for i in range(1,len(rows)) if rows[i]['map']!=rows[i-1]['map']];arrivals=[int(rows[i]['demotic']) for i in range(1,len(rows)) if rows[i]['map']!=rows[i-1]['map']]
        checkpoints=sorted(set([0,1,20,40,100,200,400]+list(range(60,min(tics,720),60))+[n+d for n in exits+arrivals for d in [-2,-1,0,1] if 0<=n+d<tics]))
        for mode in ['baseline','save','keyframe']:
            actual=folder/f'{mode}.csv'
            run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime','-run','^TestBoomCoopDemoCapture$','-count=1'],folder/f'{mode}.log',dict(env,GD_COOP_OUT=str(actual),GD_COOP_WAD=str(merged),GD_COOP_DEMO=str(lmp),GD_COOP_CHECKPOINT_TICS='' if mode=='baseline' else ','.join(map(str,checkpoints)),GD_COOP_CHECKPOINT_FORMAT=mode))
            result=compare(actual,folder/'dsda.csv');result['coverage']=dict(maps=maps,exits=exits,arrivals=arrivals,restores=0 if mode=='baseline' else len(checkpoints));result['passed'] &= len(maps)==2 and len(exits)==1
            if exits and arrivals:
                old=next(r for r in rows if int(r['demotic'])==exits[0]);new=next(r for r in rows if int(r['demotic'])==arrivals[0])
                a=[list(map(int,p.split(':'))) for p in old['players'].strip(';').split(';')];b=[list(map(int,p.split(':'))) for p in new['players'].strip(';').split(';')]
                checks=dict(roster=[x[0] for x in a]==c['slots']==[x[0] for x in b],keys_cleared=all(x[24]==0 for x in b),new_leveltime=new['tic']=='0',carryover=all(y[8]==(100 if x[15]!=0 else x[8]) and (x[15]!=0 or x[11:15]==y[11:15]) for x,y in zip(a,b)))
                if c.get('dead'):checks['dead_reborn']=a[0][15]==1 and b[0][15]==0 and b[0][8]==100
                result['coverage']['checks']=checks;result['passed'] &= all(checks.values())
            result['inputs']=dict(wad=digest(pwad),demo=digest(lmp),settings=c,commands=tics)
            results[name+'_'+mode]=result;print(name,mode,result['matched'],'/',result['rows'],flush=True)
            if not result['passed']:print(json.dumps(result['failures'][:1],indent=2),flush=True)
    source=hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):source.update(str(path.relative_to(ROOT)).encode()+b'\0'+path.read_bytes())
    report=dict(metadata=dict(gd_source=source.hexdigest(),verifier=digest(Path(__file__)),observer=digest(ROOT/'scripts/visual_compare/dsda_coop_demo_probe.inc')),passed=bool(results) and all(r['passed'] for r in results.values()),cases=results,scope='Generated co-op normal/secret campaign routes; active frames plus command positions, full roster and gameplay RNG; excludes cosmetic pr_misc',reference=digest(args.reference),iwad=digest(args.iwad))
    (out/'results.json').write_text(json.dumps(report,indent=2)+'\n');raise SystemExit(0 if report['passed'] else 1)
if __name__=='__main__':main()
