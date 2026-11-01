#!/bin/bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Runs the headless AnyMobile benchmark N times and writes one JSON line per
# run plus a CSV. It creates throwaway devices in a temp dir; a user's devices
# are never touched.
#
#   RUNS=3 scripts/mobile-bench.sh build/mobile-bench
#
# Needs a runtime: MOBILE_RUNTIME (defaults to the installed one).
set -euo pipefail
runs=${RUNS:-1}
out=${1:?output directory}
rt=${MOBILE_RUNTIME:-$HOME/Library/Application Support/AnyProfile/mobile/runtime}
[ -x "$rt/qemu/bin/qemu-system-aarch64" ] || {
  echo "no runtime at $rt (set MOBILE_RUNTIME)" >&2
  exit 1
}
mkdir -p "$out"
jsonl="$out/bench.jsonl"
csv="$out/bench.csv"
: >"$jsonl"
echo "run,device,image,host,macos,create_ms,boot_ms,idle_cpu_pct,rss_mb,peak_rss_mb,screenshot_ms,stop_ms,disk_mb,idle_seconds" >"$csv"

for i in $(seq 1 "$runs"); do
  line=$(ANYPROFILE_MOBILE_RUNTIME="$rt" \
    go test -tags vmintegration -run '^TestBench$' -count=1 -v -timeout 30m ./internal/mobile/vm/ 2>&1 |
    grep '^BENCH ' | sed 's/^BENCH //') || {
    echo "run $i failed (see test output above)" >&2
    exit 1
  }
  printf '%s\n' "$line" >>"$jsonl"
  printf '%s\n' "$line" |
    jq -r --arg run "$i" '[$run,.device,.image,.host,.macos,.create_ms,.boot_ms,.idle_cpu_pct,.rss_mb,.peak_rss_mb,.screenshot_ms,.stop_ms,.disk_mb,.idle_seconds] | @csv' >>"$csv"
  echo "run $i: $line"
done
echo "wrote $csv"
