#!/usr/bin/env python3
"""Run the complete 4,646-demo inventory, retain evidence, and notify the desktop."""

import argparse
from collections import Counter
import fcntl
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import traceback

from demo_trace_compare_all import ROOT, atomic_json, comparison_passes, sha256


def validate_inventory(manifest, historical):
    rows = manifest['demos']
    old = historical['current_runtime_verification']
    expected = old['records'] + old['excluded_records']
    keys = [(r['game'], r['sha256']) for r in rows]
    if (len(rows) != 4646 or len(set(keys)) != 4646 or
            set(keys) != {(r['game'], r['sha256']) for r in expected} or
            Counter(r['game'] for r in rows) != {'doom': 2411, 'doom2': 2235} or
            len({(r['game'], r['map']) for r in rows}) != 68 or
            manifest.get('skipped') != [] or
            manifest['archive_sha256'] != historical['corpus']['archive_sha256']):
        raise ValueError('Expected the complete pinned 4,646-input inventory with zero skipped archives')
    return rows


def source_receipt():
    names = subprocess.check_output(
        ['git', 'ls-files', '-co', '--exclude-standard', '-z'], cwd=ROOT).decode().split('\0')
    files = {name: sha256(ROOT/name) for name in sorted(set(names))
             if name.endswith('.go') or name in ('go.mod', 'go.sum')}
    internal = hashlib.sha256()
    for path in sorted((ROOT/'internal').rglob('*.go')):
        internal.update(str(path.relative_to(ROOT)).encode() + b'\0' + path.read_bytes())
    return {'files': files, 'internal_go_source_sha256': internal.hexdigest()}


def pin_file(source, target, expected=None):
    if expected is not None and sha256(source) != expected:
        raise ValueError(f'Input hash differs: {source}')
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, target)
    if sha256(target) != sha256(source):
        raise ValueError(f'Copy verification failed: {target}')


def prepare(args):
    out = args.out
    setup = out/'setup.json'
    if setup.exists():
        if not args.resume:
            raise ValueError('Output already initialized; use --resume or a fresh --out')
        receipt = json.loads(setup.read_text())
        for name, digest in receipt['pinned_files_sha256'].items():
            if sha256(out/name) != digest:
                raise ValueError(f'Pinned audit input changed: {name}')
        return receipt
    if (out/'replays/run.json').exists():
        raise ValueError('Replay output exists without its setup receipt; use a fresh --out')
    manifest = json.loads(args.manifest.read_text())
    historical = json.loads((ROOT/'demos/COMPET-N-results.json').read_text())
    rows = validate_inventory(manifest, historical)
    source = source_receipt()
    atomic_json(out/'source.json', source)
    with (out/'build.log').open('w') as log:
        subprocess.run(['go', 'build', '-o', str(out/'gddoom'), '.'],
                       cwd=ROOT, stdout=log, stderr=subprocess.STDOUT, check=True)
    if source_receipt() != source:
        raise ValueError('Source changed during build; use a fresh output directory')
    pin_file(args.reference, out/'reference-doom', historical['evidence']['original_reference_sha256'])
    selected = []
    # Long recordings start first, reducing the final single-worker tail.
    for item in sorted(rows, key=lambda r: (-r['tics'], r['path'])):
        item = dict(item)
        original = (args.manifest.parent/item['path']).resolve()
        demo = out/'inputs/demos'/original.name
        pin_file(original, demo, item['sha256'])
        item['path'] = str(demo)
        item['wad'] = str(out/'inputs'/Path(item['wad']).name)
        selected.append(item)
    for name in ('DOOM2.WAD', 'DOOMU.WAD'):
        target = out/'inputs'/name
        original = args.wad_root/name
        pin_file(original, target, historical['evidence']['iwad_sha256'][name])
    atomic_json(out/'manifest.json', dict(manifest, demos=selected))
    atomic_json(out/'exclusions.json', historical['current_runtime_verification']['excluded_records'])
    files = ['gddoom', 'reference-doom', 'manifest.json', 'source.json', 'exclusions.json',
             'inputs/DOOM2.WAD', 'inputs/DOOMU.WAD']
    receipt = {'recordings': 4646, 'source': source['internal_go_source_sha256'],
               'pinned_files_sha256': {name: sha256(out/name) for name in files}}
    atomic_json(setup, receipt)
    return receipt


def outcome(result, exclusions, harness_log=''):
    # Never turn a known input into an exclusion when it fails differently.
    if (result['status'] == 'match' and result.get('gameplay_rng_audited') is True
            and result.get('gameplay_rng_mismatch') is None):
        return 'strict_match'
    if result['status'] == 'mismatch' and comparison_passes(result):
        return 'semantic_only_match'
    old = exclusions.get(result['demo_sha256'])
    if old is not None:
        raw = old['raw_result']
        if (raw['status'] == 'error' and raw['exit_code'] == 139 and
                result['status'] == 'error' and result['exit_code'] == 139 and
                'Tracing reference runtime:' in harness_log and 'Tracing GD-DOOM:' not in harness_log):
            return 'documented_exclusion'
        if (raw['status'] == 'mismatch' and result['status'] == 'mismatch' and
                result.get('gameplay_rng_audited') is True and
                result.get('gameplay_rng_mismatch') is None and
                result['report'].startswith('mismatch line=1083 path=root.specials[0].topheight\n') and
                'left_gametic=1082\nright_gametic=1082\n' in result['report']):
            return 'documented_exclusion'
    return 'unexplained_failure'


def finish(out, receipt):
    raw = json.loads((out/'replays/summary.json').read_text())
    manifest = json.loads((out/'manifest.json').read_text())
    expected = {r['sha256'] for r in manifest['demos']}
    results = raw['results']
    if len(results) != 4646 or {r['demo_sha256'] for r in results} != expected:
        raise ValueError('Incomplete final inventory; no full-corpus result can be claimed')
    exclusions = {r['sha256']: r for r in json.loads((out/'exclusions.json').read_text())}
    records = []
    for result in results:
        folder = out/'replays'/Path(result['demo']).stem
        classification = outcome(result, exclusions, (folder/'harness.log').read_text())
        records.append({'demo': Path(result['demo']).name, 'sha256': result['demo_sha256'],
                        'outcome': classification, 'raw_result_sha256': sha256(folder/'result.json')})
    counts = dict(Counter(r['outcome'] for r in records))
    report = {'complete': True, 'eligible': 4646, 'completed': len(records),
              'no_unexplained_failures': counts.get('unexplained_failure', 0) == 0,
              'counts': counts, 'source': receipt['source'],
              'raw_summary_sha256': sha256(out/'replays/summary.json'),
              'exclusions_are_passes': False, 'records': records}
    atomic_json(out/'results.json', report)
    return report


def notify(out, title, body):
    env = dict(os.environ)
    env.setdefault('DBUS_SESSION_BUS_ADDRESS', f'unix:path=/run/user/{os.getuid()}/bus')
    try:
        destination = out/'results.json' if (out/'results.json').exists() else out/'stopped.json'
        proc = subprocess.run(['notify-send', '--app-name=GD-DOOM', '--expire-time=0', title,
                               body + '\n' + str(destination)],
                              env=env, capture_output=True, text=True, timeout=15)
        receipt = {'sent': proc.returncode == 0, 'returncode': proc.returncode, 'error': proc.stderr}
    except (OSError, subprocess.TimeoutExpired) as error:
        receipt = {'sent': False, 'error': str(error)}
    atomic_json(out/'notification.json', receipt)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest', type=Path, required=True)
    parser.add_argument('--wad-root', type=Path, required=True, help='Directory containing DOOM2.WAD and DOOMU.WAD')
    parser.add_argument('--reference', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--jobs', type=int, default=12)
    parser.add_argument('--resume', action='store_true')
    parser.add_argument('--notify', action='store_true', help='Send a persistent desktop notification when finished or stopped')
    args = parser.parse_args()
    if args.jobs < 1:
        parser.error('--jobs must be positive')
    args.out = args.out.resolve()
    args.manifest = args.manifest.resolve()
    args.wad_root = args.wad_root.resolve()
    args.reference = args.reference.resolve()
    args.out.mkdir(parents=True, exist_ok=True)
    with (args.out/'audit.lock').open('w') as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            parser.error('This output directory already has an active audit')
        try:
            receipt = prepare(args)
            # A retry must not mistake a previous complete summary for this
            # attempt if the child stops before writing its new final report.
            for name in ('results.json', 'stopped.json', 'notification.json', 'replays/summary.json'):
                (args.out/name).unlink(missing_ok=True)
            command = ['xvfb-run', '-a', sys.executable, '-B', str(ROOT/'scripts/demo_trace_compare_all.py'),
                       '--manifest', str(args.out/'manifest.json'), '--gd-bin', str(args.out/'gddoom'),
                       '--ref-bin', str(args.out/'reference-doom'), '--jobs', str(args.jobs),
                       '--semantic-unused-ceiling-fields', '--discard-matching-traces',
                       '--compress-retained-traces', '--out-root', str(args.out/'replays')]
            if args.resume:
                command.append('--resume')
            with (args.out/'replay.log').open('a' if args.resume else 'w') as log:
                proc = subprocess.run(command, cwd=ROOT, env=dict(os.environ, SDL_AUDIODRIVER='dummy'),
                                      stdout=log, stderr=subprocess.STDOUT)
            if proc.returncode not in (0, 1):
                raise RuntimeError(f'Replay runner exited {proc.returncode}; see replay.log')
            report = finish(args.out, receipt)
            counts = report['counts']
            body = (f"4,646 demos checked: {counts.get('strict_match', 0)} strict, "
                    f"{counts.get('semantic_only_match', 0)} semantic, "
                    f"{counts.get('documented_exclusion', 0)} known exclusions, "
                    f"{counts.get('unexplained_failure', 0)} unexplained failures.")
            print(body, flush=True)
            if args.notify:
                notify(args.out, 'GD-DOOM demo audit finished', body)
            return 0 if report['no_unexplained_failures'] else 1
        except Exception as error:
            atomic_json(args.out/'stopped.json', {'complete': False, 'error': str(error)})
            traceback.print_exc()
            if args.notify:
                notify(args.out, 'GD-DOOM demo audit stopped', f'Incomplete run: {error}. Progress can be resumed.')
            return 2


if __name__ == '__main__':
    sys.exit(main())
