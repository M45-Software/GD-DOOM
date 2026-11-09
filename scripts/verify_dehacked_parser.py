#!/usr/bin/env python3
"""Compare resolved patch definitions with an independently instrumented DSDA.

The reference must include visual_compare/dsda_dehacked_probe.inc at the end of
d_deh.c and call GD_DehackedProbe() after PostProcessDeh() in d_main.c. The
observer exits before starting gameplay. No IWAD or community patch is bundled.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import struct
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]


def digest(path):
    with path.open('rb') as f:
        return hashlib.file_digest(f, 'sha256').hexdigest()


def source_digest():
    h = hashlib.sha256()
    for path in sorted((ROOT/'internal/dehacked').glob('*.go')):
        h.update(path.name.encode()+b'\0'+path.read_bytes())
    return h.hexdigest()


def fixtures():
    result = {
        'baseline': {},
        'thing': {'main.deh': '''Thing 2 (all properties)
ID # = 12345
Initial frame = 0
Hit points = 777
First moving frame = 176
Alert sound = 1
Reaction time = -1
Attack sound = 2
Injury frame = 187
Pain chance = 17
Pain sound = 3
Close attack frame = 4
Far attack frame = 5
Death frame = 6
Exploding frame = 7
Death sound = 8
Speed = 0x12345
Width = 1310720
Height = 3670016
Mass = 888
Missile damage = 9
Action sound = 10
Bits = SOLID + SHOOTABLE + COUNTKILL + TRANSLUCENT
Respawn frame = 11

'''},
        'frame-graph': {'main.deh': '''Frame 174
Sprite number = 52
Sprite subnumber = 32770
Duration = 0
Next frame = 174
Unknown 1 = -1
Unknown 2 = 0x1234

Frame 175
Duration = -1
Next frame = 0

'''},
        'pointer-order': {
            'main.deh': '[CODEPTR]\nFRAME 174 = VileChase\nFRAME 763 = Chase\n\n',
            'later.deh': 'Pointer 9999 (Frame 763)\nCodep Frame = 174\n\nPointer 0 (Frame 764)\nCodep Frame = 0\n\n'},
        'codeptr-null-and-cross-type': {'main.deh': '[CODEPTR]\nFRAME 763 = VileChase\nFRAME 174 = NULL\nFRAME 10 = Look\n\n'},
        'signed-flags': {'main.deh': 'Thing 2\nBits = -2147483642\n\n'},
        'unsigned-flags': {'main.deh': 'Thing 2\nBits = 2147483654\n\n'},
        'integer-word-encodings': {'main.deh': 'Thing 2\nHit points = 0xffffffff\nReaction time = 0x80000000\n\nFrame 174\nUnknown 1 = 4294967295\nUnknown 2 = 0x80000000\n\nFrame 175\nDuration = 0xffffffff\n\n'},
        'cheat-padding': {'main.deh': b'Cheat 0\nGod mode = newcode\xffpadding\n\n'},
        'legacy-string-bytes': {'main.deh': b'[STRINGS]\nHUSTR_1 = high\xff\n\nText 12 5\npress a key.raw\xff!\n\n'},
        'legacy-resource-bytes': {'main.deh': b'Text 4 4\nTROOAB\xffD\n\n'},
        'explicit-default-overrides': {'main.deh': 'Thing 5\nBits = 528\n\nMisc 0\nMax Health = 100\nMax Soulsphere = 200\nMegasphere Health = 200\nBFG Cells/Shot = 40\n\n'},
        'boom-translation-aliases': {'main.deh': 'Thing 2\nBits = TRANSLATION + UNUSED1 + UNUSED4\n\n'},
        'weapons-ammo-misc': {'main.deh': '''Weapon 6
Ammo type = 5
Deselect frame = 1
Select frame = 2
Bobbing frame = 3
Shooting frame = 4
Firing frame = 5

Ammo 2
Max ammo = 999
Per ammo = 33

Misc 0
Initial Health = 123
Initial Bullets = 12
Max Health = 300
Max Armor = 400
Green Armor Class = 3
Blue Armor Class = 1
Max Soulsphere = 250
Soulsphere Health = 90
Megasphere Health = 350
God Mode Health = 77
IDFA Armor = 333
IDFA Armor Class = 3
IDKFA Armor = 444
IDKFA Armor Class = 4
BFG Cells/Shot = 25
Monsters Infight = 221

'''},
        'sound-cheat-ignored-offsets': {'main.deh': '''Sound 1
Offset = 1234
Value = 77
Zero 2 = 150
Zero 4 = 12

Sprite 0
Offset = 5678

Cheat 0
God mode = newcode

'''},
        'text-renames': {
            'main.deh': 'Text 4 4\nTROOABCD\n\nText 6 6\npistollaser1\n\nText 4 3\ne1m1abc\n\n',
            'later.deh': 'Text 4 4\nTROOEFGH\n\nText 6 6\npistollaser2\n\n'},
        'strings-pars-crlf': {'main.deh': r'''[STRINGS]
HUSTR_1 = one \n\
    two\Nthree

[PARS]
par 1 23
par 2 3 77

'''.replace('\n', '\r\n')},
        'include-notext': {
            'main.deh': 'Thing 25\nHit points = 1\n\nINCLUDE NOTEXT child.deh\n\nThing 25\nMass = 500\n\n',
            'child.deh': 'Thing 25\nHit points = 2\n\nText 4 4\nTROOABCD\n\n[STRINGS]\nHUSTR_1 = Child\n\n'},
        'include-notext-boundary': {
            'main.deh': 'INCLUDE NOTEXT child.deh\n\n[STRINGS]\nHUSTR_1 = Parent\n\n',
            'child.deh': 'Text 4 4\nTROOABCD\n\nFrame 174\nDuration = 3\n\n # boundary\nThing 25\nHit points = 999\n\n[STRINGS]\nHUSTR_2 = Child\n\n'},
    }
    old = 'press a key.'
    new = r'go\nnow'
    result['text-lengthening-and-named-string'] = {
        'main.deh': f'Text 4 6\ne1m1newmus\n\nText {len(old)} {len(new)}\n{old}{new}\n\n',
        'later.deh': f'Text {len(old)} 4\n{old}done\n\n'}
    return result


def embedded_patch(path):
    data = path.read_bytes()
    count, directory = struct.unpack_from('<ii', data, 4)
    patches = []
    for i in range(count):
        pos, size, name = struct.unpack_from('<ii8s', data, directory+16*i)
        if name.rstrip(b'\0') == b'DEHACKED':
            patches.append(data[pos:pos+size])
    if len(patches) != 1:
        raise ValueError('expected exactly one original embedded patch')
    return patches[0]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--reference', type=Path, required=True)
    parser.add_argument('--iwad', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--sunlust-wad', type=Path)
    args = parser.parse_args()
    out = args.out.resolve(); out.mkdir(parents=True, exist_ok=True)
    reference, iwad = args.reference.resolve(), args.iwad.resolve()
    source = source_digest()
    guards = {str(p.relative_to(ROOT)): digest(p) for p in
              [Path(__file__), ROOT/'scripts/visual_compare/dsda_dehacked_probe.inc']}
    inputs = {'reference_sha256': digest(reference), 'iwad_sha256': digest(iwad)}
    cases = fixtures()
    if args.sunlust_wad:
        patch = embedded_patch(args.sunlust_wad)
        if hashlib.sha256(patch).hexdigest() != '91f46075aea96dc191573d846e291f2e6cc5bf01abea64ceddca3fb60f94e0a1':
            raise ValueError('expected the audited original Sunlust patch')
        cases['original-sunlust'] = {'main.deh': patch}
        inputs['sunlust_patch_sha256'] = hashlib.sha256(patch).hexdigest()
    test = out/'dehacked.test'
    with (out/'build.log').open('w') as log:
        subprocess.run(['go', 'test', '-c', '-tags', 'integration', '-o', str(test), './internal/dehacked'],
                       cwd=ROOT, stdout=log, stderr=subprocess.STDOUT, check=True)
    results = []
    env = dict(os.environ, SDL_AUDIODRIVER='dummy')
    for name, files in cases.items():
        folder = out/name; folder.mkdir(exist_ok=True)
        for filename, text in files.items():
            (folder/filename).write_bytes(text if isinstance(text, bytes) else text.encode('ascii'))
        paths = [folder/f for f in ('main.deh', 'later.deh') if f in files]
        for level in (2, 9):
            native = folder/f'native-cl{level}.jsonl'; native.unlink(missing_ok=True)
            # Compare the canonical Doom base tables. Disable DSDA's automatic
            # cosmetic translucency on unpatched actors; explicit patched Bits
            # remain authoritative and are still compared in full.
            config=folder/f'dsda-cl{level}.cfg'
            config.write_text('boom_translucent_sprites 0\n')
            command = ['xvfb-run', '-a', str(reference), '-iwad', str(iwad), '-complevel', str(level),
                       '-nodeh', '-nosound', '-nodraw', '-config', str(config)]
            if paths: command += ['-deh', *map(str, paths)]
            with (folder/f'native-cl{level}.log').open('w') as log:
                native_run = subprocess.run(command, cwd=folder, env=dict(env, GD_DEHACKED_TABLES=str(native)),
                                            stdout=log, stderr=subprocess.STDOUT, timeout=60)
            logpath = folder/f'go-cl{level}.log'
            code, rows = -1, 0
            if native_run.returncode == 0 and native.exists():
                with logpath.open('w') as log:
                    result = subprocess.run([str(test), '-test.run=^TestDSDAResolvedDefinitions$', '-test.v', '-test.count=1'],
                                            cwd=ROOT, env=dict(env, GD_DEH_NATIVE=str(native), GD_DEH_PATCHES=json.dumps(list(map(str,paths)))),
                                            stdout=log, stderr=subprocess.STDOUT, timeout=60)
                code = result.returncode
                match = re.search(r'independent native definition rows matched: (\d+)', logpath.read_text())
                rows = int(match[1]) if match else 0
            passed = code == 0 and rows >= 1647
            results.append(dict(case=name, complevel=level, passed=passed, compared_rows=rows,
                                patch_sha256={p.name:digest(p) for p in paths},
                                native_sha256=digest(native) if native.exists() else None,
                                go_log_sha256=digest(logpath) if logpath.exists() else None))
            print(name, 'cl'+str(level), 'PASS' if passed else 'FAIL', rows, flush=True)
    if source_digest()!=source or digest(reference)!=inputs['reference_sha256'] or any(digest(ROOT/p)!=sha for p,sha in guards.items()):
        raise ValueError('source/reference/verifier changed during the attempt; rerun before claiming parity')
    report = dict(passed=all(r['passed'] for r in results), source=source, inputs=inputs, guards=guards,
                  cases=results, reference_config={'boom_translucent_sprites':0},
                  normalization={'resource_name_case':'ASCII-insensitive','action_mnemonic_case':'ASCII-insensitive',
                                 'legacy_string_transport':'each raw byte encoded as a JSON code point without UTF-8 replacement',
                                 'native_no_ammo_sentinel':'Raven-shared am_noammo mapped to Doom value 5',
                                 'native_actor_flags':'mapped by named standard flag masks to canonical DEH 32-bit layout'},
                  scope='Resolved definition tables after native parsing, before gameplay. Canonical Doom bases: automatic cosmetic Boom translucency is disabled for unpatched actors; explicit patched Bits are compared in full. All canonical actor/state/weapon/ammo rows, resources and par times; supported sound fields, patched English strings and patched cheats, and explicit actor Bits/health/BFG override markers. Does not establish gameplay, rendering, audio, save or multiplayer integration.')
    (out/'results.json').write_text(json.dumps(report, indent=2)+'\n')
    return 0 if report['passed'] else 1


if __name__ == '__main__':
    sys.exit(main())
