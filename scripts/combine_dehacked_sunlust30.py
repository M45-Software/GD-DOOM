#!/usr/bin/env python3
"""Combine independent MAP30 mode runs only when their proof identities agree.

Use verify_dehacked_sunlust30.py --mode MODE with a separate output directory
for each parallel run. This also compares the completed-level summaries and
retains hashes of the unmodified component reports.
"""
import argparse
import json
from pathlib import Path

from verify_boom_community_coop_campaign import source_digest
from verify_boom_transfers import digest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('baseline', 'save', 'keyframe', 'out'):
        parser.add_argument('--' + name, required=True, type=Path)
    parser.add_argument('--blockmap', type=Path)
    args = parser.parse_args()
    combined, components, summaries = None, {}, {}
    for mode in ('baseline', 'save', 'keyframe'):
        folder = getattr(args, mode).resolve()
        path = folder / 'results.json'
        report = json.loads(path.read_text())
        if not (report['passed'] and report['complete'] and
                report['requested_modes'] == [mode] and set(report['cases']) == {mode}):
            raise ValueError('expected a complete passing independent ' + mode + ' run')
        if combined is None:
            combined = dict(report, cases={})
        for key in ('source', 'verifier', 'inputs', 'capture_sha256', 'reference_coverage', 'dependencies'):
            if report[key] != combined[key]:
                raise ValueError('mode proof identities disagree: ' + key)
        result = report['cases'][mode]
        coverage = report['reference_coverage']
        if (result['world']['rows'] != 15997 or result['phase']['rows'] != 16496 or
                result['cold_restores'] != (0 if mode == 'baseline' else 9) or
                coverage['commands'] != 16496 or coverage['phases'] != {'0': 15997, '1': 499, '2': 0}):
            raise ValueError('incomplete original recording or checkpoint coverage')
        actual = json.loads((folder / (mode + '-summary.json')).read_text())
        expected = json.loads((folder / 'dsda-summary.json').read_text())
        if not (actual == expected and actual['completed'] and actual['leveltime'] == 15997):
            raise ValueError('completed-level summary differs from native reference')
        combined['cases'][mode] = result
        summaries[mode] = dict(passed=True, value=actual,
                               gd_sha256=digest(folder / (mode + '-summary.json')),
                               dsda_sha256=digest(folder / 'dsda-summary.json'))
        components[mode] = digest(path)
    if source_digest() != combined['source']:
        raise ValueError('reports do not describe the current Go source')
    combined.update(requested_modes=['baseline', 'save', 'keyframe'],
                    component_reports_sha256=components, summaries=summaries,
                    combination_script_sha256=digest(Path(__file__)))
    if args.blockmap:
        blockmap = json.loads(args.blockmap.read_text())
        if not (blockmap['passed'] and blockmap['cells'] == 20880 and
                blockmap['source_sha256'] == combined['source'] and
                blockmap['capture_sha256'] == combined['capture_sha256'] and
                blockmap['reference_sha256'] == combined['inputs']['reference']):
            raise ValueError('blockmap proof does not describe this build')
        combined['blockmap'] = blockmap
    args.out.write_text(json.dumps(combined, indent=2) + '\n')
    print('passed: three complete modes, eighteen cold restores, matching exit summaries')


if __name__ == '__main__':
    main()
