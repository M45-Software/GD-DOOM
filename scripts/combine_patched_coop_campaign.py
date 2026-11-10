#!/usr/bin/env python3
"""Combine the six complete patched Sunlust campaign replay receipts.

Each input directory is a separate --patched --case NAME --mode MODE run of
verify_boom_community_coop_campaign.py. Partial or mixed-build reports cannot
be promoted to full campaign evidence.
"""
import argparse
import json
from pathlib import Path

from verify_boom_community_coop_campaign import CASES, COMMANDS, MAP_ORDER, source_digest
from verify_boom_transfers import ROOT, digest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('runs', nargs='+', type=Path)
    parser.add_argument('--out', required=True, type=Path)
    args = parser.parse_args()
    expected = {name + '_' + mode for name in CASES for mode in ('baseline', 'save', 'keyframe')}
    combined, components, references = None, {}, {}
    for folder in args.runs:
        path = folder / 'results.json'
        report = json.loads(path.read_text())
        if not (report['passed'] and report['complete'] and report['patched'] and
                not report['diagnostic_only'] and len(report['cases']) == 1):
            raise ValueError('expected one complete passing patched case per input')
        if combined is None:
            combined = dict(report, cases={})
        for key in ('source', 'verifier', 'capture_sha256'):
            if report[key] != combined[key]:
                raise ValueError('proof identities disagree: ' + key)
        name, mode = report['requested_cases'][0], report['requested_modes'][0]
        label = name + '_' + mode
        if label not in expected or label in combined['cases'] or set(report['cases']) != {label}:
            raise ValueError('duplicate or unexpected case')
        result = report['cases'][label]
        cov = result['reference_coverage']
        restores = 0 if mode == 'baseline' else len(cov['checkpoints'])
        if not (result['passed'] and not result['diagnostic_only'] and cov['passed'] and
                all(cov['checks'].values()) and cov['commands'] == COMMANDS and
                cov['maps'] == MAP_ORDER and result['replay']['commands'] == COMMANDS and
                result['world']['rows'] == cov['phases']['0'] and result['phase']['rows'] == COMMANDS and
                result['restores'] == restores):
            raise ValueError('incomplete campaign or checkpoint coverage')
        identity = dict(inputs=result['inputs'], coverage=cov,
                        world=result['world']['dsda_hash'], phase=result['phase']['dsda_hash'])
        if name in references and references[name] != identity:
            raise ValueError('reference identities differ between replay modes')
        references[name] = identity
        combined['cases'][label] = result
        components[label] = digest(path)
    if set(combined['cases']) != expected or combined['source'] != source_digest():
        raise ValueError('missing cases or reports do not describe the current Go source')
    if combined['verifier'] != digest(ROOT / 'scripts/verify_boom_community_coop_campaign.py'):
        raise ValueError('reports do not describe the current verifier')
    combined.update(passed=True, complete=True, full_suite=True,
                    requested_cases=list(CASES), requested_modes=['baseline', 'save', 'keyframe'],
                    total_commands=COMMANDS * len(expected),
                    cold_restores=sum(r['restores'] for r in combined['cases'].values()),
                    component_reports_sha256=components,
                    combination_script_sha256=digest(Path(__file__)))
    args.out.write_text(json.dumps(combined, indent=2) + '\n')
    print('passed:', len(expected), 'complete campaign cases;', combined['cold_restores'], 'cold restores')


if __name__ == '__main__':
    main()
