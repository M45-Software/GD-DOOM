#!/usr/bin/env python3
"""Record Speed of Doom MAP22 live, then replay the new demos in GD-DOOM and DSDA."""
import argparse
import json
import os
from pathlib import Path
import tempfile

from verify_boom_community_coop import SOD_WAD
from verify_boom_community_coop_campaign import source_digest
from verify_boom_demos import fetch
from verify_boom_full_combat import compare_complete
from verify_boom_sod_campaign import IWAD_SHA
from verify_boom_transfers import digest, merge_wads, run


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad', required=True, type=Path)
    parser.add_argument('--reference', required=True, type=Path,
                        help='DSDA-Doom with the campaign world and phase observers')
    parser.add_argument('--out', required=True, type=Path)
    args = parser.parse_args()
    iwad, reference, out = args.iwad.resolve(), args.reference.resolve(), args.out.resolve()
    if digest(iwad) != IWAD_SHA:
        parser.error('use the original Doom II 1.9 IWAD')
    out.mkdir(parents=True, exist_ok=True)
    # Fresh paths ensure an observer that emits nothing cannot reuse an old trace.
    recording = Path(tempfile.mkdtemp(prefix='recording-', dir=out))
    wad = fetch(recording, 'sodfinal.zip', *SOD_WAD)
    merged = recording / 'merged.wad'
    merge_wads([iwad, wad], merged)
    binary = recording / 'campaign.test'
    env = dict(os.environ, GD_BOOM_RECORD_SOD_WAD=str(merged), GD_BOOM_RECORD_OUT=str(recording),
               GD_COOP_CAMPAIGN='1', GD_COOP_COMBAT_ONLY='1', GD_COOP_LINES='1', SDL_AUDIODRIVER='dummy')
    run(['go', 'test', '-c', '-tags', 'integration', '-o', binary, './internal/doomruntime'], recording/'build.log')
    run(['xvfb-run', '-a', binary, '-test.v', '-test.run=^TestSpeedOfDoomRecordedEncounter$'],
        recording/'record.log', env)
    results = {}
    for name in ['automatic', 'custom']:
        demo = recording / (name + '.lmp')
        folder = recording / name
        folder.mkdir()
        run(['xvfb-run', '-a', reference, '-iwad', iwad, '-file', wad, '-timedemo', demo,
             '-nodeh', '-noautoload', '-nosound', '-nodraw', '-config', folder/'dsda.cfg'],
            folder/'dsda.log', dict(env, GD_DSDA_COOP_OUT=str(folder/'dsda-world.csv'),
                                    GD_DSDA_PHASE_OUT=str(folder/'dsda-phase.csv')), cwd=folder)
        run(['xvfb-run', '-a', binary, '-test.run=^TestBoomCoopDemoCapture$'], folder/'gd.log',
            dict(env, GD_COOP_DEMO=str(demo), GD_COOP_WAD=str(merged), GD_COOP_OUT=str(folder/'gd-world.csv'),
                 GD_COOP_PHASE_OUT=str(folder/'gd-phase.csv')))
        comparisons = {kind: compare_complete(folder/f'gd-{kind}.csv', folder/f'dsda-{kind}.csv', 180)
                       for kind in ['world', 'phase']}
        results[name] = dict(lmp_sha256=digest(demo), **comparisons)
    passed = all(result[kind]['passed'] for result in results.values() for kind in ['world', 'phase'])
    receipt = dict(passed=passed, runtime_source=source_digest(), reference_sha256=digest(reference),
                   iwad_sha256=digest(iwad), wad_sha256=digest(wad), results=results)
    path = recording / 'results.json'
    path.write_text(json.dumps(receipt, indent=2) + '\n')
    print(path)
    return 0 if passed else 1


if __name__ == '__main__':
    raise SystemExit(main())
