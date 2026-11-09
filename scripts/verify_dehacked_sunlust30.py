#!/usr/bin/env python3
"""Replay Bredd's unchanged Sunlust MAP30 UV-Speed demo with its original patch.

Checks the complete original command stream and cold save/keyframe restores.
No -nodeh override, edited gameplay patch, altered WAD, or appended demo tail.
The original recording ends during intermission; finale execution is not proved.
"""
import argparse
import csv
import hashlib
import json
import os
from pathlib import Path

from verify_boom_community_coop_campaign import capture, lumps, source_digest
from verify_boom_full_combat import compare_complete, SUNLUST_DEH_SHA256
from verify_boom_transfers import ROOT, digest, merge_wads, run

COMMANDS = 16496
WAD_SHA = '6a90becf56040896fd2876a29f60a7a528d9802b081e0a1cc7d7cc65d546a964'
DEMO_SHA = '36d39a6a9b0926929085eccdfdf36ba27d282dc76363abbf4d0f26ec35ececb3'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('iwad', 'wad', 'demo', 'reference', 'out'):
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument('--mode', choices=('baseline', 'save', 'keyframe'), action='append')
    parser.add_argument('--timeout', type=int, default=3600)
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    iwad, wad, demo, reference = (p.resolve() for p in (args.iwad, args.wad, args.demo, args.reference))
    if digest(wad) != WAD_SHA or digest(demo) != DEMO_SHA:
        raise ValueError('expected original Sunlust WAD and sl30-737.lmp')
    raw = demo.read_bytes()
    end = 109
    while end < len(raw) and raw[end] != 128:
        end += 4
    if (raw[0] != 202 or raw[8:13] != bytes((3, 1, 30, 0, 0)) or
            raw[77:81] != bytes((1, 0, 0, 0)) or (end - 109) // 4 != COMMANDS):
        raise ValueError('original one-player MAP30 Boom commands changed')
    deh = next(data for (_, name), data in lumps(wad).items() if name == 'DEHACKED')
    if hashlib.sha256(deh).hexdigest() != SUNLUST_DEH_SHA256:
        raise ValueError('original embedded Sunlust patch changed')
    source, verifier = source_digest(), digest(Path(__file__))
    env = {k: v for k, v in os.environ.items()
           if not k.startswith(('GD_COOP_', 'GD_DSDA_', 'GD_RNG_', 'GD_DEHACKED_', 'GD_DEBUG_'))}
    env.update(SDL_AUDIODRIVER='dummy', GD_COOP_CAMPAIGN='1', GD_COOP_COMBAT_ONLY='1',
               GD_COOP_COMPACT='1', GD_COOP_LINES='1', GD_COOP_COLD_RESTORE='1',
               GD_DEHACKED_CAPTURE='1')
    observers = {name: digest(ROOT / 'scripts/visual_compare' / name) for name in
                 ('dsda_coop_demo_probe.inc', 'dsda_rng_demo_probe.inc',
                  'dsda_intermission_probe.inc', 'dsda_finale_probe.inc')}
    identity = dict(iwad=digest(iwad), wad=digest(wad), demo=digest(demo),
                    deh=SUNLUST_DEH_SHA256, reference=digest(reference), observers=observers)
    expected = {kind: out / ('dsda-' + kind + '.csv') for kind in ('world', 'phase')}
    manifest = out / 'reference.json'
    cached = json.loads(manifest.read_text()) if manifest.exists() else {}
    if not (cached.get('identity') == identity and
            all(p.exists() and cached.get('traces', {}).get(k) == digest(p) for k, p in expected.items())):
        manifest.unlink(missing_ok=True)
        run(['xvfb-run', '-a', reference, '-iwad', iwad, '-file', wad, '-timedemo', demo,
             '-nosound', '-nodraw', '-noautoload', '-config', out / 'dsda.cfg'],
            out / 'dsda.log', dict(env, GD_DSDA_COOP_OUT=str(expected['world']),
                                  GD_DSDA_PHASE_OUT=str(expected['phase']),
                                  GD_COOP_SUMMARY=str(out / 'dsda-summary.json')), timeout=args.timeout)
        manifest.write_text(json.dumps(dict(identity=identity, traces={k: digest(p) for k, p in expected.items()}), indent=2) + '\n')
    rows = list(csv.DictReader(expected['phase'].open()))
    if len(rows) != COMMANDS or any(int(r['demotic']) != i + 1 for i, r in enumerate(rows)):
        raise ValueError('native capture omitted original commands')
    frames = sum(1 for _ in csv.DictReader(expected['world'].open()))
    checkpoints = {22, 66, 6042, 6043, 9999, COMMANDS - 1}
    transitions = [i for i in range(1, len(rows)) if rows[i]['phase'] != rows[i - 1]['phase']]
    for i in transitions:
        checkpoints.update((i - 1, i, (i + COMMANDS - 1) // 2))
    checkpoints = sorted(checkpoints)
    coverage = dict(commands=COMMANDS, active_frames=frames,
                    phases={str(p): sum(int(r['phase']) == p for r in rows) for p in (0, 1, 2)},
                    cold_checkpoints_after_commands=[n + 1 for n in checkpoints],
                    checkpoint_phases=[int(rows[n]['phase']) for n in checkpoints])
    merged, binary = out / 'merged.wad', out / 'capture.test'
    merge_wads([iwad, wad], merged)
    run(['go', 'test', '-c', '-tags', 'integration', '-o', binary, './internal/doomruntime'], out / 'build.log', env)
    modes = list(dict.fromkeys(args.mode or ('baseline', 'save', 'keyframe')))
    report = dict(passed=False, complete=False, source=source, verifier=verifier,
                  inputs=identity, capture_sha256=digest(binary), reference_coverage=coverage,
                  requested_modes=modes, cases={}, scope=__doc__,
                  exclusions='Cosmetic RNG, decoration animation, text pixels, rendering and audio. No appended finale commands.',
                  dependencies={name: digest(ROOT / 'scripts' / name) for name in
                                ('verify_boom_community_coop_campaign.py', 'verify_boom_full_combat.py', 'verify_boom_transfers.py')})
    for mode in modes:
        actual = {kind: out / (mode + '-' + kind + '.csv') for kind in expected}
        for path in actual.values():
            path.unlink(missing_ok=True)
        replay_env = dict(env, GD_COOP_WAD=str(merged), GD_COOP_DEMO=str(demo),
                          GD_COOP_OUT=str(actual['world']), GD_COOP_PHASE_OUT=str(actual['phase']),
                          GD_COOP_SUMMARY=str(out / (mode + '-summary.json')),
                          GD_COOP_CHECKPOINT_FORMAT=mode,
                          GD_COOP_CHECKPOINT_TICS='' if mode == 'baseline' else ','.join(map(str, checkpoints)))
        if mode == 'baseline':
            replay_env['GD_COOP_EXPORT_SAVES'] = str(out / 'diagnostic-states')
            replay_env['GD_COOP_DUMP_COMMAND'] = '6043'
        execution = capture(['xvfb-run', '-a', binary, '-test.run', '^TestBoomCoopDemoCapture$',
                             '-test.count=1', '-test.timeout=' + str(args.timeout) + 's'],
                            out / (mode + '.log'), replay_env, actual, expected, args.timeout)
        if source_digest() != source or digest(Path(__file__)) != verifier or digest(reference) != identity['reference']:
            raise ValueError('sources/verifier/reference changed during verification; replay this attempt')
        result = dict(passed=False, execution=execution,
                      cold_restores=0 if mode == 'baseline' else len(checkpoints))
        if execution['returncode'] == 0:
            result['world'] = compare_complete(actual['world'], expected['world'], frames)
            result['phase'] = compare_complete(actual['phase'], expected['phase'], COMMANDS)
            result['passed'] = result['world']['passed'] and result['phase']['passed']
        report['cases'][mode] = result
        report['complete'] = set(report['cases']) == set(modes)
        report['passed'] = report['complete'] and all(r['passed'] for r in report['cases'].values())
        (out / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
        print(mode, result, flush=True)
        if not result['passed']:
            raise SystemExit(1)


if __name__ == '__main__':
    main()
