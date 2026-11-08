#!/usr/bin/env python3
"""Compare recorded co-op finale commands, cast states and checkpoints with DSDA."""
import argparse,csv,hashlib,json,os,struct
from pathlib import Path
from verify_boom_coop_campaign import fixture
from verify_boom_transitions import compare
from verify_boom_rng import demo
from verify_boom_transfers import ROOT,merge_wads,run,digest


def rename_maps(path,mapname):
    raw=bytearray(path.read_bytes());count,directory=struct.unpack_from('<ii',raw,4)
    for i in range(count):
        pos=directory+i*16;name=bytes(raw[pos+8:pos+16]).rstrip(b'\0')
        if name in [b'MAP01',b'MAP02']:
            new=mapname if name==b'MAP01' else ('MAP31' if mapname=='MAP30' else mapname[:3]+'9')
            raw[pos+8:pos+16]=new.encode().ljust(8,b'\0')
    path.write_bytes(raw)


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--iwad',type=Path,required=True);p.add_argument('--episode-iwad',type=Path);p.add_argument('--reference',type=Path,required=True);p.add_argument('--out',type=Path,required=True);p.add_argument('--case',action='append');args=p.parse_args()
    cases={}
    for mapname in ['E1M8','E2M8','E3M8','E4M8','MAP30']:
        for roster,slots in [('two',[1,2]),('four',[1,2,3,4])]:cases[mapname+'_'+roster]=dict(map=mapname,slots=slots)
    cases['E3M8_natural']=dict(map='E3M8',slots=[1,2],natural=True)
    cases['MAP30_keys']=dict(map='MAP30',slots=[1,2,3,4],keys=True)
    if args.case and set(args.case)-cases.keys():p.error('unknown case')
    out=args.out.resolve();out.mkdir(parents=True,exist_ok=True);results={}
    for name,c in cases.items():
        if args.case and name not in args.case:continue
        folder=out/name;folder.mkdir(exist_ok=True);pwad,merged,lmp=[folder/n for n in ['finale.wad','merged.wad','finale.lmp']]
        iwad=args.iwad if c['map']=='MAP30' else args.episode_iwad or args.iwad
        fixture(pwad,False,c['slots']);rename_maps(pwad,c['map']);merge_wads([iwad.resolve(),pwad],merged)
        demo(lmp,1993,1,attack=False,tics=1);h=bytearray(lmp.read_bytes()[:109]);h[9]=1 if c['map']=='MAP30' else int(c['map'][1]);h[10]=30 if c['map']=='MAP30' else 8;h[12]=c['slots'][-1]-1;h[77:81]=bytes(int(i+1 in c['slots']) for i in range(4))
        tics=4200 if c.get('natural') else 3000 if c.get('keys') else 1500
        commands=bytearray()
        for tic in range(tics):
            for slot in c['slots']:
                buttons=1 if slot==c['slots'][0] and (tic==601 if c.get('natural') else tic%20==1) else 0
                commands+=bytes([25 if tic<80 else 0,0,0,buttons])
        lmp.write_bytes(h+commands+b'\x80');keys=list(range(150,2900,120)) if c.get('keys') else []
        env=dict(os.environ,GD_COOP_CAMPAIGN='1',GD_COOP_FINALE='1',GD_COOP_CAST_KEYS=','.join(map(str,keys)))
        run(['xvfb-run','-a',args.reference.resolve(),'-iwad',iwad.resolve(),'-file',pwad,'-timedemo',lmp,'-nodeh','-nosound','-nodraw','-config',folder/'dsda.cfg'],folder/'dsda.log',dict(env,GD_DSDA_COOP_OUT=str(folder/'dsda.csv')))
        rows=list(csv.DictReader((folder/'dsda.csv').open()));changes=[i for i,r in enumerate(rows) if i and (r['phase'],r['stage'],r['castnum'],r['death'])!=(rows[i-1]['phase'],rows[i-1]['stage'],rows[i-1]['castnum'],rows[i-1]['death'])]
        checkpoints=sorted(set([0,1,20,100,200,400,1000,tics-2]+list(range(60,tics,300))+[i+d for i in changes for d in [-1,0,1] if 0<=i+d<tics]))
        for mode in ['baseline','save','keyframe']:
            actual=folder/f'{mode}.csv'
            run(['xvfb-run','-a','go','test','-tags','integration','./internal/doomruntime','-run','^TestBoomCoopDemoCapture$','-count=1'],folder/f'{mode}.log',dict(env,GD_COOP_OUT=str(actual),GD_COOP_WAD=str(merged),GD_COOP_DEMO=str(lmp),GD_COOP_MAP=c['map'],GD_COOP_CHECKPOINT_TICS='' if mode=='baseline' else ','.join(map(str,checkpoints)),GD_COOP_CHECKPOINT_FORMAT=mode))
            result=compare(actual,folder/'dsda.csv');checks=dict(all_commands=len(rows)==tics,finale=any(r['phase']=='2' for r in rows),ending=any(r['phase']=='2' and r['stage']==('2' if c['map']=='MAP30' else '1') for r in rows))
            if c.get('natural'):checks['natural_text']=all(r['accelerated']=='0' for r in rows if r['phase']=='2')
            if c.get('keys'):checks.update(cast_deaths=any(r['death']=='1' for r in rows),all_actors=len(set(r['castnum'] for r in rows if r['stage']=='2'))==17)
            result['coverage']=dict(checks=checks,restores=0 if mode=='baseline' else len(checkpoints),changes=changes);result['passed'] &= all(checks.values());result['inputs']=dict(wad=digest(pwad),demo=digest(lmp),iwad=digest(iwad),settings=c)
            results[name+'_'+mode]=result;print(name,mode,result['matched'],'/',result['rows'],checks,flush=True)
            if not result['passed']:print(json.dumps(result['failures'][:1],indent=2),flush=True)
    source=hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):source.update(str(path.relative_to(ROOT)).encode()+b'\0'+path.read_bytes())
    report=dict(metadata=dict(gd_source=source.hexdigest(),verifier=digest(Path(__file__))),passed=bool(results) and all(c['passed'] for c in results.values()),cases=results,reference=digest(args.reference),iwad=digest(args.iwad),observer=digest(ROOT/'scripts/visual_compare/dsda_finale_probe.inc'),scope='All real command frames, finale/cast state, roster inventory/latches and gameplay RNG. Optional explicit host key events test cast deaths; ticcmd attacks do not kill cast actors.')
    (out/'results.json').write_text(json.dumps(report,indent=2)+'\n');raise SystemExit(0 if report['passed'] else 1)
if __name__=='__main__':main()
