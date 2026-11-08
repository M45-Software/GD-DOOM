#!/usr/bin/env bash
set -euo pipefail
playtest_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
attempt_name="${1:-attempt-07-detailed}"
recording_dir="$playtest_dir"
if [[ $# -gt 0 ]]; then shift; fi
case "$attempt_name" in
  attempt-07-detailed) recording_dir="$playtest_dir/detailed-showcase" ;;
  attempt-03-classic|attempt-04-classic-clear) ;;
  *) echo "Choose attempt-07-detailed, attempt-03-classic or attempt-04-classic-clear; older experimental recordings use a different playback profile." >&2; exit 2 ;;
esac
cd "$playtest_dir/../../.."
exec go run . -config= -wad DOOM1.WAD \
  -file "$recording_dir/graveyard-shift-playtest.wad" \
  -demo "$recording_dir/$attempt_name/session.lmp" -render=true -sourceport-mode=true -width=1920 -height=1080 -detail-level=0 "$@"

