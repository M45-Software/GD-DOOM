#!/usr/bin/env python3
"""Compare an original full Sunlust co-op movie and cold checkpoints with DSDA.

Inputs are hash-pinned and commands are never replaced or extended. Reference
captures are cached with input, executable, observer and trace hashes. Stop a GD
replay at its first difference rather than continuing a desynchronized campaign.
Use --patched to load the exact original Sunlust patch in the Go runtime too.
Use --through for an explicitly bounded diagnostic; it never claims full parity.
"""
import argparse
import csv
import hashlib
import json
import os
from pathlib import Path
import signal
import struct
import subprocess
import time

from verify_boom_demos import WADS, fetch
from verify_boom_full_combat import SUNLUST_DEH_SHA256, compare_complete
from verify_boom_transfers import ROOT, build_reference, digest, merge_wads, run

ARCHIVE = '/files/demos/sunlust/112514/slallc45115.zip'
ARCHIVE_SHA = '8d4c1d297abe2850fc3f2f35c3d057c2c46deff652629ac96cd28e39b9634a20'
FIX_SHA = '3e6954e5898babc5df625e0c070a55539238067cd00fbbdc8caf6b872f7bd443'
COMMANDS = 652981
CASES = {'slallk45115': 0, 'slallj45115': 1}
MAP_ORDER = ([f'MAP{n:02d}' for n in range(1, 16)] + ['MAP31', 'MAP32'] +
             [f'MAP{n:02d}' for n in range(16, 31)])


def lumps(path):
    raw = path.read_bytes()
    count, directory = struct.unpack_from('<ii', raw, 4)
    current = ''
    result = {}
    for i in range(count):
        offset, size, name = struct.unpack_from('<ii8s', raw, directory + i * 16)
        name = name.rstrip(b'\0').decode('ascii')
        if name.startswith('MAP') and len(name) == 5:
            current = name
        result[current, name] = raw[offset:offset + size]
    return result


def audit_inputs(wad, fix, demo, console, deh_path):
    raw = demo.read_bytes()
    if (raw[0] != 202 or raw[8:13] != bytes([3, 1, 1, 0, console]) or
            raw[77:81] != bytes([1, 1, 0, 0])):
        raise ValueError('unexpected original two-player Boom movie header')
    end = 109
    while end < len(raw) and raw[end] != 128:
        end += 8
    if end >= len(raw) or (end - 109) // 8 != COMMANDS:
        raise ValueError('original movie command count changed')
    base, patch = lumps(wad), lumps(fix)
    differences = {key for key, data in patch.items() if base.get(key) != data}
    if differences != {('MAP26', 'THINGS')}:
        raise ValueError('MAP26 fix changes more than the audited starts')
    before = base['MAP26', 'THINGS']
    after = patch['MAP26', 'THINGS']
    expected_starts = {(3216, -552, 135, 2, 23), (3232, -512, 135, 3, 23),
                       (3184, -504, 135, 4, 23)}
    before_things = list(struct.iter_unpack('<5h', before))
    after_things = list(struct.iter_unpack('<5h', after))
    if (len(after) != len(before) + 30 or
            {thing for thing in after_things if thing in expected_starts} != expected_starts or
            [thing for thing in after_things if thing not in expected_starts] != before_things):
        raise ValueError('MAP26 fix does not contain exactly the documented starts')
    deh = next(data for (level, name), data in base.items() if name == 'DEHACKED')
    if hashlib.sha256(deh).hexdigest() != SUNLUST_DEH_SHA256:
        raise ValueError('Sunlust gameplay patch changed')
    keen = {level: sum(t[3] == 72 for t in struct.iter_unpack('<5h', data))
            for (level, name), data in base.items() if name == 'THINGS'}
    if {level: count for level, count in keen.items() if count} != {'MAP30': 56}:
        raise ValueError('patched actor placement changed')
    # Preserve the active Keen/state patch and custom story text in DSDA.
    # -nodeh suppresses Freedoom's unrelated embedded patch; -deh loads this
    # exact original Sunlust patch explicitly, without editing its contents.
    deh_path.write_bytes(deh)


def coverage(path):
    maps, phases, checkpoints = [], {0: 0, 1: 0, 2: 0}, {0}
    stories, intermissions, deaths, rebirths = set(), set(), set(), set()
    previous, segment_start, rows = None, 0, 0
    checkpoint_phase = {}
    segments = []
    with path.open() as stream:
        for index, row in enumerate(csv.DictReader(stream)):
            rows += 1
            if int(row['demotic']) != index + 1:
                raise ValueError('missing or reordered campaign command')
            phase, level = int(row['phase']), row['map']
            phases[phase] += 1
            if not maps or maps[-1] != level:
                maps.append(level)
            if phase == 1:
                intermissions.add(level)
            if phase == 2:
                stories.add(level)
            players = [list(map(int, p.split(':'))) for p in row['players'].strip(';').split(';')]
            if [p[0] for p in players] != [1, 2]:
                raise ValueError('full roster absent from a command phase')
            current = (level, phase, {p[0]: p[1] for p in players})
            if previous:
                if current[:2] != previous[:2]:
                    checkpoints.update([index - 1, index])
                    segments.append((segment_start, index - 1, previous[1]))
                    segment_start = index
                for slot, health in current[2].items():
                    old = previous[2][slot]
                    if old > 0 >= health and phase == 0:
                        deaths.add(slot)
                        checkpoints.update([index - 1, index])
                    if old <= 0 < health and phase == 0:
                        rebirths.add(slot)
                        checkpoints.update([index - 1, index])
            previous = current
    if previous:
        segments.append((segment_start, rows - 1, previous[1]))
        checkpoints.add(rows - 1)
    for start, end, phase in segments:
        if phase in (1, 2):
            checkpoints.add((start + end) // 2)
    for start, end, phase in segments:
        for index in checkpoints:
            if start <= index <= end:
                checkpoint_phase[index] = phase
    checks = dict(all_commands=rows == COMMANDS, map_order=maps == MAP_ORDER,
                  all_intermissions=intermissions == set(MAP_ORDER),
                  story_screens=stories == {'MAP06', 'MAP11', 'MAP15', 'MAP31', 'MAP20'},
                  both_players_die=deaths == {1, 2}, both_players_reborn=rebirths == {1, 2},
                  original_ending=bool(previous and previous[:2] == ('MAP30', 1)))
    return dict(passed=all(checks.values()), checks=checks, commands=rows,
                maps=maps, phases=phases, checkpoints=sorted(checkpoints),
                checkpoint_phases={p: sum(v == p for v in checkpoint_phase.values()) for p in phases},
                finale_limit='Original movie ends in MAP30 intermission; no added finale commands.')


def capture(command, log, env, actual, expected, timeout):
    """Follow bounded trace lines and terminate only this replay on mismatch."""
    handles, matched, failure = {}, {kind: 0 for kind in actual}, None
    started = last_update = time.monotonic()
    with log.open('w') as output:
        process = subprocess.Popen(list(map(str, command)), cwd=ROOT, env=env,
                                   stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
        try:
            while True:
                progress = False
                differences = []
                for kind in actual:
                    if kind not in handles:
                        if not actual[kind].exists():
                            continue
                        handles[kind] = (actual[kind].open('rb'), expected[kind].open('rb'))
                    left, right = handles[kind]
                    while True:
                        a_pos, b_pos = left.tell(), right.tell()
                        a, b = left.readline(), right.readline()
                        if not a.endswith(b'\n'):
                            left.seek(a_pos); right.seek(b_pos)
                            break
                        progress = True
                        if a != b:
                            differences.append(dict(kind=kind, row=matched[kind],
                                                    gd=a.decode().strip(), dsda=b.decode().strip()))
                            break
                        matched[kind] += 1
                if differences:
                    def command_number(item):
                        value = item['gd'].split(',')[0]
                        return int(value) if value.isdecimal() else 0
                    failure = min(differences, key=command_number)
                if failure or process.poll() is not None and not progress:
                    break
                if time.monotonic() - started > timeout:
                    raise TimeoutError('co-op campaign replay exceeded its explicit timeout')
                if time.monotonic() - last_update > 30:
                    print('matched trace lines', matched, flush=True)
                    last_update = time.monotonic()
                if not progress:
                    time.sleep(.1)
        finally:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGTERM)
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait()
            for pair in handles.values():
                for stream in pair:
                    stream.close()
    return dict(returncode=process.returncode, first_difference=failure,
                matched_lines=matched, elapsed_seconds=time.monotonic() - started)


def reference_identity(exe, iwad, wad, fix, demo, deh):
    return dict(line_state=True, executable=digest(exe), iwad=digest(iwad), wad=digest(wad),
                fix=digest(fix), demo=digest(demo), deh=digest(deh),
                observers={name: digest(ROOT / 'scripts/visual_compare' / name) for name in
                           ['dsda_coop_demo_probe.inc', 'dsda_rng_demo_probe.inc', 'dsda_intermission_probe.inc', 'dsda_finale_probe.inc']},
                noautoload=True,
                installer=digest(ROOT / 'scripts/verify_boom_transfers.py'))


def source_digest():
    source = hashlib.sha256()
    for path in sorted((ROOT / 'internal').rglob('*.go')):
        source.update(str(path.relative_to(ROOT)).encode() + b'\0' + path.read_bytes())
    return source.hexdigest()


def trace_prefix(source, destination, commands):
    rows, maps = 0, set()
    with source.open('rb') as src, destination.open('wb') as dst:
        dst.write(src.readline())
        for row in src:
            fields = row.split(b',', 2)
            if int(fields[0]) > commands:
                break
            dst.write(row)
            rows += 1
            maps.add(fields[1].decode())
    return rows, sorted(maps)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--iwad', type=Path, required=True)
    ref = parser.add_mutually_exclusive_group(required=True)
    ref.add_argument('--reference', type=Path)
    ref.add_argument('--reference-source', type=Path)
    parser.add_argument('--cmake-arg', action='append', default=[])
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--case', choices=CASES, action='append')
    parser.add_argument('--mode', choices=['baseline', 'save', 'keyframe'], action='append')
    parser.add_argument('--timeout', type=int, default=43200)
    parser.add_argument('--patched', action='store_true', help='load the original Sunlust gameplay/text patch in both engines')
    parser.add_argument('--reference-only', action='store_true', help='capture/audit native references without claiming Go parity')
    parser.add_argument('--through', type=int,
                        help='diagnostic original-command prefix; never full-campaign evidence')
    args = parser.parse_args()
    if args.through is not None and not 1 <= args.through < COMMANDS:
        parser.error('--through must select a nonempty, strictly partial original recording')
    out = args.out.resolve(); out.mkdir(parents=True, exist_ok=True)
    exe = args.reference.resolve() if args.reference else build_reference(args.reference_source, out, args.cmake_arg)
    env = {k: v for k, v in os.environ.items() if not k.startswith(('GD_COOP_', 'GD_DSDA_', 'GD_DEBUG_', 'GD_DEHACKED_', 'GD_RNG_'))}
    env.update(SDL_AUDIODRIVER='dummy', GD_COOP_CAMPAIGN='1', GD_COOP_COMBAT_ONLY='1',
               GD_COOP_COMPACT='1', GD_COOP_LINES='1', GD_COOP_COLD_RESTORE='1')
    source = source_digest()
    verifier = digest(Path(__file__))
    selected_cases = list(dict.fromkeys(args.case or list(CASES)))
    selected_modes = list(dict.fromkeys(args.mode or ['baseline', 'save', 'keyframe']))
    requested = {name + '_' + mode for name in selected_cases for mode in selected_modes}
    results = {}
    binary = out / 'campaign.test'
    if not args.reference_only:
        run(['go', 'test', '-c', '-tags', 'integration', '-o', binary,
             './internal/doomruntime'], out / 'build.log', env)
    reference_binary = digest(exe)
    for name in selected_cases:
        folder = out / name; folder.mkdir(exist_ok=True)
        wad = fetch(folder, 'sunlust.zip', *WADS['sunlust'])
        fix = fetch(folder, 'sunlust.zip', WADS['sunlust'][0], WADS['sunlust'][1], 'sunlust-m26.wad')
        if digest(fix) != FIX_SHA:
            raise ValueError('MAP26 start-fix hash changed')
        demo = fetch(folder, Path(ARCHIVE).name, ARCHIVE, ARCHIVE_SHA, name + '.lmp')
        deh = folder / 'sunlust.deh'
        audit_inputs(wad, fix, demo, CASES[name], deh)
        case_env = dict(env)
        if args.patched:
            # Match -nodeh -deh exactly: load this unchanged original patch,
            # independently of any IWAD's unrelated embedded patch.
            case_env.update(GD_DEHACKED_CAPTURE='1', GD_DEHACKED_NO_EMBEDDED='1',
                            GD_DEHACKED_FILES=str(deh))
        merged = folder / 'merged.wad'; merge_wads([args.iwad.resolve(), wad, fix], merged)
        expected = {kind: folder / ('dsda-' + kind + '.csv') for kind in ['world', 'phase']}
        identity = reference_identity(exe, args.iwad, wad, fix, demo, deh)
        manifest = folder / 'reference.json'
        cached = json.loads(manifest.read_text()) if manifest.exists() else {}
        valid = (cached.get('identity') == identity and
                 all(path.exists() and cached.get('traces', {}).get(kind) == digest(path)
                     for kind, path in expected.items()))
        if not valid:
            manifest.unlink(missing_ok=True)
            # Shared immutable reference files may be hard-linked into mode
            # directories. Replace stale captures instead of truncating them.
            for path in expected.values():
                path.unlink(missing_ok=True)
            run(['xvfb-run', '-a', exe, '-iwad', args.iwad.resolve(), '-file', wad, fix,
                 '-timedemo', demo, '-nodeh', '-deh', deh, '-nosound', '-nodraw', '-noautoload',
                 '-config', folder / 'dsda.cfg'], folder / 'dsda.log',
                dict(env, GD_DSDA_COOP_OUT=str(expected['world']),
                     GD_DSDA_PHASE_OUT=str(expected['phase'])), timeout=args.timeout)
            cached = dict(identity=identity, traces={kind: digest(path) for kind, path in expected.items()})
            manifest.write_text(json.dumps(cached, indent=2) + '\n')
        cov = coverage(expected['phase'])
        if not cov['passed']:
            raise ValueError(f'original reference coverage failed: {cov["checks"]}')
        if args.reference_only:
            if digest(exe) != reference_binary or digest(Path(__file__)) != verifier:
                raise ValueError('reference/verifier changed during native capture')
            (folder / 'reference-coverage.json').write_text(json.dumps(
                dict(reference_only=True, identity=identity, coverage=cov), indent=2) + '\n')
            print(name, 'native reference coverage', cov, flush=True)
            continue
        commands = args.through or COMMANDS
        suffix = f'-through-{commands}' if args.through else ''
        frames = cov['phases'][0]
        replay_maps = cov['maps']
        if args.through:
            prefix = {}
            for kind, path in expected.items():
                prefix[kind] = folder / ('dsda-' + kind + suffix + '.csv')
                count, maps = trace_prefix(path, prefix[kind], commands)
                if kind == 'world':
                    frames = count
                else:
                    if count != commands:
                        raise ValueError('reference prefix is missing campaign commands')
                    replay_maps = maps
            if not args.patched and 'MAP30' in replay_maps:
                raise ValueError('Boom-only diagnostic includes DeHackEd-dependent MAP30')
            expected = prefix
        checkpoints = [tic for tic in cov['checkpoints'] if tic < commands]
        for mode in selected_modes:
            actual = {kind: folder / (mode + '-' + kind + suffix + '.csv') for kind in expected}
            for path in actual.values():
                path.unlink(missing_ok=True)
            replay_env = dict(case_env, GD_COOP_WAD=str(merged), GD_COOP_DEMO=str(demo),
                              GD_COOP_OUT=str(actual['world']), GD_COOP_PHASE_OUT=str(actual['phase']),
                              GD_COOP_CHECKPOINT_FORMAT=mode, GD_COOP_CHECKPOINT_TICS=
                              '' if mode == 'baseline' else ','.join(map(str, checkpoints)))
            if args.through:
                replay_env['GD_COOP_CAPTURE_COMMANDS'] = str(commands)
            # Exports are for diagnosis only. Every reported comparison still
            # starts at command one; GD_COOP_LOAD_SAVE is scrubbed above.
            states = folder / 'baseline-states' / source
            if mode == 'baseline':
                replay_env['GD_COOP_EXPORT_SAVES'] = str(states)
            execution = capture(['xvfb-run', '-a', binary, '-test.run', '^TestBoomCoopDemoCapture$',
                                 '-test.count=1', '-test.timeout=' + str(args.timeout) + 's'],
                                folder / (mode + suffix + '.log'), replay_env, actual, expected, args.timeout)
            if source_digest() != source or digest(Path(__file__)) != verifier or digest(exe) != reference_binary:
                raise ValueError('source/verifier changed during campaign verification; replay this attempt')
            result = dict(passed=False, execution=execution, reference_coverage=cov, inputs=identity,
                          replay=dict(commands=commands, active_frames=frames, maps=replay_maps),
                          diagnostic_only=bool(args.through),
                          restores=0 if mode == 'baseline' else len(checkpoints))
            if mode == 'baseline':
                result['diagnostic_states'] = str(states)
            if execution['returncode'] == 0:
                result['world'] = compare_complete(actual['world'], expected['world'], frames)
                result['phase'] = compare_complete(actual['phase'], expected['phase'], commands)
                result['passed'] = result['world']['passed'] and result['phase']['passed']
            results[name + '_' + mode] = result
            complete = set(results) == requested
            report = dict(passed=complete and all(r['passed'] for r in results.values()),
                          complete=complete, cases=results,
                          source=source, verifier=verifier,
                          patched=args.patched, capture_sha256=digest(binary),
                          requested_cases=selected_cases, requested_modes=selected_modes,
                          diagnostic_only=bool(args.through),
                          full_suite=args.patched and not args.through and len(results) == len(CASES) * 3 and all(r['passed'] for r in results.values()),
                          scope=('Diagnostic original-command prefix; no full-campaign parity claim. '
                                 if args.through else 'Original full two-player Sunlust D2All commands. ') +
                                ('Original unchanged Sunlust gameplay/text patch enabled in both engines. ' if args.patched else 'Go replay excludes DeHackEd. ') +
                                'Complete active-world '
                                'CSV digests and every campaign phase. Cold save/keyframe restores '
                                'at map/phase boundaries, screen interiors and player deaths/rebirths. '
                                'Excludes cosmetic RNG, decoration animation, custom text content, pixels and audio. '
                                'Original recording stops in MAP30 intermission, before its finale.')
            (out / ('results' + suffix + '.json')).write_text(json.dumps(report, indent=2) + '\n')
            print(name, mode, 'passed', result['passed'], execution, flush=True)
            if not result['passed']:
                raise SystemExit(1)


if __name__ == '__main__':
    main()
