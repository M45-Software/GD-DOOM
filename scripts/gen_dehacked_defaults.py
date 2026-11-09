#!/usr/bin/env python3
"""Generate canonical DeHackEd tables by compiling original Linux Doom data.

Usage: python3 scripts/gen_dehacked_defaults.py --doom-source /path/linuxdoom-1.10
No engine runs: action functions are stubs and only constant tables are read.
The generated file records source hashes and does not need C sources to build.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import shlex
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
SOURCE_FILES = ['info.c', 'info.h', 'd_items.c', 'sounds.c', 'sounds.h',
                'p_inter.c', 'g_game.c', 'd_englsh.h', 'p_mobj.h',
                'doomdef.h', 'doomtype.h', 'm_fixed.h', 'd_think.h']


def stripped(text):
    return re.sub(r'/\*.*?\*/|//[^\n]*', '', text, flags=re.S)


def enum_names(text, prefix):
    for body in re.findall(r'typedef\s+enum\s*\{(.*?)\}', stripped(text), re.S):
        names = [entry.split('=')[0].strip() for entry in body.split(',')]
        found = [n for n in names if n.startswith(prefix)]
        if found:
            return found
    raise ValueError('enum not found: ' + prefix)


def declaration(text, name):
    match = re.search(r'\bint\s+' + name + r'\s*\[[^;=]*=\s*(\{.*?\});', stripped(text), re.S)
    if not match:
        raise ValueError('declaration not found: ' + name)
    return 'int ' + name + ('[4][10]' if name == 'pars' else '[]') + ' = ' + match[1] + ';'


def extract(source, cc):
    info = (source/'info.c').read_text()
    names = enum_names((source/'info.h').read_text(), 'MT_')
    state_names = enum_names((source/'info.h').read_text(), 'S_')
    actions = sorted(set(re.findall(r'\{\s*(A_\w+)\s*\}', info)))
    strings = re.findall(r'^#define\s+(\w+)\s+"', (source/'d_englsh.h').read_text(), re.M)
    c = r'''
#include <stdio.h>
#include <stdint.h>
#define rcsid info_rcsid
#include "info.c"
#undef rcsid
#define rcsid items_rcsid
#include "d_items.c"
#undef rcsid
#define rcsid sounds_rcsid
#include "sounds.c"
#undef rcsid
#include "d_englsh.h"
static void q(const char *s) {
    putchar('"');
    for (; *s; ++s) {
        unsigned char c = *s;
        if (c == '"' || c == '\\') { putchar('\\'); putchar(c); }
        else if (c < 32 || c >= 127) printf("\\u%04x", c);
        else putchar(c);
    }
    putchar('"');
}
'''
    c += '\n'.join('void ' + name + '() {}' for name in actions) + '\n'
    c += 'static const char *action(actionf_t a) {\n'
    c += '\n'.join(f'if (a.acv == (actionf_v){name}) return "{name[2:]}";' for name in actions)
    c += '\nif (!a.acv) return "NULL"; return "INVALID"; }\n'
    for name in ('maxammo', 'clipammo'):
        c += declaration((source/'p_inter.c').read_text(), name) + '\n'
    for name in ('pars', 'cpars'):
        c += declaration((source/'g_game.c').read_text(), name) + '\n'
    c += 'int main(void) {\n'
    c += 'printf("{\\"states\\":[");\n'
    c += '''for (int i=0;i<NUMSTATES;i++) {
        state_t *s=&states[i]; if(i)putchar(',');
        printf("[%d,%ld,%ld,",s->sprite,s->frame,s->tics); q(action(s->action));
        printf(",%d,%ld,%ld]",s->nextstate,s->misc1,s->misc2);
    }
    printf("],\\"things\\":[");
    for (int i=0;i<NUMMOBJTYPES;i++) {
        mobjinfo_t *m=&mobjinfo[i]; if(i)putchar(',');
'''
    fields = ['doomednum','spawnstate','spawnhealth','seestate','seesound','reactiontime',
              'attacksound','painstate','painchance','painsound','meleestate','missilestate',
              'deathstate','xdeathstate','deathsound','speed','radius','height','mass','damage',
              'activesound','flags','raisestate']
    c += 'printf("[' + ','.join(['%d']*len(fields)) + ']",' + ','.join('m->'+f for f in fields) + ');\n}\n'
    c += r'''
    printf("],\"weapons\":[");
    for(int i=0;i<NUMWEAPONS;i++) {
        weaponinfo_t *w=&weaponinfo[i]; if(i)putchar(',');
        printf("[%d,%d,%d,%d,%d,%d]",w->ammo,w->upstate,w->downstate,w->readystate,w->atkstate,w->flashstate);
    }
    printf("],\"ammo\":[");
    for(int i=0;i<NUMAMMO;i++) { if(i)putchar(','); printf("[%d,%d]",maxammo[i],clipammo[i]); }
    printf("],\"sprites\":[");
    for(int i=0;i<NUMSPRITES;i++) { if(i)putchar(','); q(sprnames[i]); }
    printf("],\"sounds\":[");
    for(int i=0;i<NUMSFX;i++) {
        sfxinfo_t *s=&S_sfx[i]; if(i)putchar(','); putchar('[');q(s->name);
        printf(",%d,%d,%d,%d,%d]",s->singularity,s->priority,s->link?(int)(s->link-S_sfx):-1,s->pitch,s->volume);
    }
    printf("],\"music\":[");
    for(int i=0;i<NUMMUSIC;i++) { if(i)putchar(','); q(S_music[i].name?S_music[i].name:""); }
    printf("],\"strings\":{");
'''.replace('\\\"', '\\"')
    c += '\n'.join(("putchar(',');" if i else '') + 'q('+json.dumps(s)+');putchar(\':\');q('+s+');' for i,s in enumerate(strings))
    c += '\nprintf("},\\"pars\\":{");\n'
    c += '''for(int e=1;e<=3;e++)for(int m=1;m<=9;m++){if(e!=1||m!=1)putchar(',');printf("\\"E%dM%d\\":%d",e,m,pars[e][m]);}
    for(int m=1;m<=32;m++)printf(",\\"MAP%02d\\":%d",m,cpars[m-1]);
    printf("}}\\n");return 0;}
'''
    with tempfile.TemporaryDirectory() as folder:
        folder = Path(folder)
        (folder/'probe.c').write_text(c)
        subprocess.run([cc,'-std=gnu99','-Werror=format','-MMD','-MF',str(folder/'probe.d'),'-I',str(source),str(folder/'probe.c'),'-o',str(folder/'probe')],check=True)
        data = json.loads(subprocess.check_output([str(folder/'probe')]))
        dependencies=shlex.split((folder/'probe.d').read_text().split(':',1)[1].replace('\\\n',' '))
        data['source_files']=sorted(set(SOURCE_FILES)|{str(Path(name).relative_to(source)) for name in dependencies if Path(name).is_relative_to(source)})
    data['thing_names'] = names
    data['state_names'] = state_names
    return data


def generate(source, cc):
    data = extract(source, cc)
    for key, count in [('things',137),('states',967),('sprites',138),('sounds',109),('weapons',9),('ammo',4)]:
        if len(data[key]) != count:
            raise ValueError(f'expected {count} {key}, found {len(data[key])}')
    q = lambda s: json.dumps(s,ensure_ascii=True)
    lines = ['// Code generated by scripts/gen_dehacked_defaults.py; DO NOT EDIT.',
             '// Canonical Doom data copyright (C) 1993-1996 id Software, Inc.',
             '// Extracted from the original Linux Doom 1.10 tables.', 'package dehacked', '']
    lines += ['var vanillaSourceSHA256 = map[string]string{']
    for name in data['source_files']:
        lines.append(q(name)+': '+q(hashlib.sha256((source/name).read_bytes()).hexdigest())+',')
    lines += ['}', 'var vanillaThings = [...]Thing{']
    for name, row in zip(data['thing_names'], data['things'], strict=True):
        row = list(row);row[21] &= 0xffffffff
        lines.append('{'+q(name)+','+','.join(map(str,row))+'},')
    lines += ['}', 'var vanillaStates = [...]State{']
    for name,row in zip(data['state_names'],data['states'],strict=True):
        values = [q(v) if isinstance(v,str) else str(v) for v in row]
        lines.append('{'+q(name)+','+','.join(values)+'},')
    for key, typ in [('weapons','Weapon'),('ammo','Ammo'),('sounds','Sound')]:
        lines += ['}', f'var vanilla{key.capitalize()} = [...]{typ}{{']
        for row in data[key]:
            lines.append('{'+','.join(q(v) if isinstance(v,str) else str(v) for v in row)+'},')
    for key in ('sprites','music'):
        lines += ['}', f'var vanilla{key.capitalize()} = [...]string{{']
        lines += [q(v)+',' for v in data[key]]
    lines += ['}', 'var vanillaStrings = map[string]string{']
    lines += [q(k)+':'+q(v)+',' for k,v in sorted(data['strings'].items())]
    lines += ['}', 'var vanillaPars = map[string]int32{']
    lines += [q(k)+':'+str(v)+',' for k,v in sorted(data['pars'].items())]
    lines += ['}', '']
    return '\n'.join(lines)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--doom-source',type=Path,required=True)
    parser.add_argument('--out',type=Path,default=ROOT/'internal/dehacked/defaults_generated.go')
    parser.add_argument('--cc',default='cc')
    args=parser.parse_args()
    content=generate(args.doom_source.resolve(),args.cc)
    args.out.parent.mkdir(parents=True,exist_ok=True)
    args.out.write_text(content)
    subprocess.run(['gofmt','-w',str(args.out)],check=True)


if __name__=='__main__':
    main()
