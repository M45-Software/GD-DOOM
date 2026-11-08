#!/usr/bin/env bash
set -euo pipefail
mod_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$mod_dir/../.."
exec go run . -wad DOOM1.WAD -file "$mod_dir/graveyard-shift.wad" -map E1M1 "$@"
