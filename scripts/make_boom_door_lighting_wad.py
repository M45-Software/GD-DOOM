#!/usr/bin/env python3
"""Original manual-door fixture and Boom 2.02 endpoint lighting cases."""
import json
from pathlib import Path
import struct

from make_boom_manual_cases import build as build_rooms
from make_boom_smoke_wad import name


def build(path):
    build_rooms(path)
    raw = path.read_bytes()
    count, directory = struct.unpack_from('<ii', raw, 4)
    lumps = []
    for i in range(count):
        start, size, label = struct.unpack_from('<ii8s', raw, directory + i * 16)
        key, data = label.rstrip(b'\0').decode(), raw[start:start + size]
        if key == 'MAP02':
            break
        if key == 'LINEDEFS':
            rows = [list(v) for v in struct.iter_unpack('<7H', data)]
            # The opaque, blocking west wall also borders an unseen lighting
            # control sector. No player/actor can enter that control sector.
            rows[0][2] = 5
            rows[0][6] = 12
            rows[2][3:5] = [1, 7]
            rows[10][3:5] = [1, 7]
            data = b''.join(struct.pack('<7H', *v) for v in rows)
        elif key == 'SIDEDEFS':
            rows = [list(v) for v in struct.iter_unpack('<hh8s8s8sH', data)]
            # Upper textures on each side of the door's two boundaries.
            for side in [2, 4, 6, 8]:
                rows[side][2] = name('STARTAN3')
            rows.append([0, 0, name('-'), name('-'), name('STARTAN3'), 3])
            data = b''.join(struct.pack('<hh8s8s8sH', *v) for v in rows)
        elif key == 'SECTORS':
            data = b''.join(struct.pack('<hh8s8shhh', 0, ceil, name('FLOOR0_1'),
                name('CEIL1_1'), light, 0, tag)
                for ceil, light, tag in [(128, 96, 7), (0, 224, 0), (128, 160, 0), (128, 32, 0)])
        elif key == 'THINGS':
            data = struct.pack('<5h', -176, 0, 0, 1, 7) + data[10:]
        lumps.append((label, data))
    payload, directory = bytearray(12), bytearray()
    for label, data in lumps:
        directory.extend(struct.pack('<ii8s', len(payload), len(data), label))
        payload.extend(data)
    struct.pack_into('<4sii', payload, 0, b'PWAD', len(lumps), len(payload))
    path.write_bytes(payload + directory)


def cases():
    result = []

    def add(label, special, **updates):
        c = dict(name=label, special=special, second=special, kind=0, age=0,
                 actor=0, activator=0, side=0, secret=0, keys=1, tics=1400,
                 at2=-1, at3=-1, late=-1, floors=[0, 0, 0],
                 ceilings=[128 * 65536, 0, 128 * 65536],
                 lighttag=7, lightmask=1, lights=[96, 224, 160],
                 neighborat=-1, neighborlight=16)
        c.update(updates)
        result.append(c)

    for special in [1, 26, 27, 28, 31, 32, 33, 34, 117, 118]:
        add(f'legacy_{special}', special)
    for speed in range(4):
        for kind in range(4):
            for delay in range(4):
                for trigger in [6, 7]:
                    special = 0x3c00 | speed << 3 | kind << 5 | delay << 8 | trigger
                    height = 124 * 65536 if kind >= 2 else 0
                    add(f'gen_{special}', special, ceilings=[128 * 65536, height, 128 * 65536])
        for kind in range(2):
            for trigger in [6, 7]:
                special = 0x3800 | speed << 3 | kind << 5 | trigger
                add(f'locked_{special}', special)
    for special in [1, 31, 117, 118, 0x3c07, 0x3c27, 0x3c47, 0x3c67, 0x3807]:
        for mask in [2, 3, 5, 7]:
            add(f'adjacent_{special}_{mask}', special, lightmask=mask)
        for label, updates in [
            ('tagzero', dict(lighttag=0)), ('missingtag', dict(lighttag=99)),
            ('back', dict(side=1)), ('nokey', dict(keys=0)),
            ('retrigger', dict(at2=20, at3=100)),
            ('changed_neighbor', dict(neighborat=100)),
            ('blocked', dict(actor=2)),
            ('zero_range', dict(floors=[0, 124 * 65536, 0],
                                ceilings=[128 * 65536, 124 * 65536, 128 * 65536])),
        ]:
            add(f'{label}_{special}', special, **updates)
    # Switched/tagged doors use their tags to select movers, never lights.
    for special in [29, 63, 111, 114, 0x3c02, 0x3c03, 0x3802, 0x3803]:
        add(f'tagged_{special}', special, lightmask=2)
    # D1 replacement keeps both the original line and its displaced door live.
    add('displaced_door', 31, kind=14, age=20, lightmask=2)
    add('overlapping_floor', 1, kind=12, lightmask=3,
        floors=[-32 * 65536, 0, -64 * 65536])
    return result


if __name__ == '__main__':
    root = Path(__file__).resolve().parents[1] / 'testdata/boom'
    build(root / 'door-lighting.wad')
    (root / 'door-lighting-cases.json').write_text(json.dumps(cases(), indent=2) + '\n')
