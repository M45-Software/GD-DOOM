#!/usr/bin/env python3
"""Compare controlled co-op doll damage/rebirth API calls with DSDA.

This is a lifecycle experiment, not a multiplayer netdemo/RNG comparison.
DSDA spawns players from a two-player Boom demo, then the explicit probe runs
seven matched events. GD uses the native Authority facade and the same events.
"""
import argparse
import csv
import hashlib
import json
import os
from pathlib import Path
import struct

from verify_boom_rng import compare, demo, fixture
from verify_boom_transfers import ROOT, build_reference, digest, merge_wads, run


def make_fixture(path):
    fixture(path)
    raw = path.read_bytes()
    count, directory = struct.unpack_from('<ii', raw, 4)
    lumps = []
    for i in range(count):
        start, size, name = struct.unpack_from('<ii8s', raw, directory+i*16)
        data = raw[start:start+size]
        label = name.rstrip(b'\0')
        if label == b'THINGS':
            things = [(-256, -48, 0, 1, 7), (-256, 48, 0, 1, 7),
                      (-128, -48, 0, 2, 7), (-64, -48, 0, 2, 7), (-128, 48, 0, 2, 7)]
            data = b''.join(struct.pack('<5h', *th) for th in things)
        elif label == b'LINEDEFS':
            data = bytearray(data)
            for j in range(0, len(data), 14):
                struct.pack_into('<HH', data, j+6, 0, 0)
        elif label == b'SECTORS':
            data = bytearray(data)
            for j in range(0, len(data), 26):
                struct.pack_into('<h', data, j+22, 0)
        lumps.append((name, data))
    payload, entries = bytearray(12), bytearray()
    for name, data in lumps:
        entries += struct.pack('<ii8s', len(payload), len(data), name)
        payload += data
    struct.pack_into('<4sii', payload, 0, b'PWAD', len(lumps), len(payload))
    path.write_bytes(payload+entries)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad', required=True, type=Path)
    ref = parser.add_mutually_exclusive_group(required=True)
    ref.add_argument('--reference', type=Path)
    ref.add_argument('--reference-source', type=Path)
    parser.add_argument('--cmake-arg', action='append', default=[])
    parser.add_argument('--out', type=Path, default=ROOT/'tmp/boom-voodoo')
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    exe = args.reference.resolve() if args.reference else build_reference(args.reference_source, out/'reference', args.cmake_arg)
    pwad, merged, lmp = [out/name for name in ['voodoo.wad', 'merged.wad', 'voodoo.lmp']]
    make_fixture(pwad)
    merge_wads([args.iwad.resolve(), pwad], merged)
    demo(lmp, 1993, 1, attack=False, tics=2)
    header = bytearray(lmp.read_bytes()[:109])
    header[78] = 1  # slot 2 is active; two commands per tic
    lmp.write_bytes(header+bytes(2*2*4)+b'\x80')
    env = dict(os.environ)
    for name in ['gd.csv', 'dsda.csv']:
        (out/name).unlink(missing_ok=True)
    run(['xvfb-run', '-a', 'go', 'test', '-tags', 'integration', './internal/doomruntime',
         '-run', '^TestBoomVoodooLifecycleCapture$', '-count=1'], out/'gd.log',
        dict(env, GD_VOODOO_WAD=str(merged), GD_VOODOO_OUT=str(out/'gd.csv')))
    run(['xvfb-run', '-a', exe, '-iwad', args.iwad.resolve(), '-file', pwad,
         '-nodeh', '-nosound', '-nodraw', '-timedemo', lmp, '-config', out/'dsda.cfg'], out/'dsda.log',
        dict(env, GD_DSDA_VOODOO_OUT=str(out/'dsda.csv')))
    result = compare(out/'gd.csv', out/'dsda.csv', 14)
    with (out/'gd.csv').open() as f:
        rows = {(int(r['event']), int(r['slot'])): r for r in csv.DictReader(f)}
    result['coverage'] = dict(owner_only=rows[2, 1]['health']=='100' and rows[2, 2]['health']=='50',
                              surviving_zero_health=rows[3, 2]['health']=='0' and rows[3, 2]['dead']=='0',
                              doll_kills_owner=rows[4, 2]['dead']=='1',
                              respawn=rows[5, 2]['health']=='100' and rows[5, 2]['dead']=='0',
                              old_doll_kills_new_life=rows[6, 2]['health']=='70' and rows[6, 2]['dead']=='1')
    result['passed'] &= all(result['coverage'].values())
    result['scope'] = 'controlled lifecycle API experiment; not netdemo/RNG parity'
    result['metadata'] = dict(reference=digest(exe), iwad=digest(args.iwad), verifier=digest(Path(__file__)),
                              probe=digest(ROOT/'scripts/visual_compare/dsda_voodoo_lifecycle_probe.inc'))
    source = hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):
        source.update(str(path.relative_to(ROOT)).encode()+b'\0'+path.read_bytes())
    result['metadata']['gd_source'] = source.hexdigest()
    (out/'results.json').write_text(json.dumps(result, indent=2)+'\n')
    print(json.dumps(result, indent=2))
    raise SystemExit(0 if result['passed'] else 1)


if __name__ == '__main__':
    main()
