#!/bin/bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Runs the example flows and writes pass/fail + wall time per flow.
#   ANYPROFILE_TOKEN=… scripts/automation-eval.sh [out-dir]
set -euo pipefail
: "${ANYPROFILE_TOKEN:?set ANYPROFILE_TOKEN (Settings -> Local API)}"
out=${1:-build/automation-eval}
mkdir -p "$out"
echo "flow,result,seconds" > "$out/results.csv"
for f in examples/automation/*.json; do
  name=$(basename "$f" .json)
  start=$(python3 -c 'import time; print(time.time())')
  if sdk/typescript/bin/anyprofile.mjs run "$f" > "$out/$name.json" 2> "$out/$name.err"; then
    result=pass
  else
    result=fail
  fi
  end=$(python3 -c 'import time; print(time.time())')
  python3 - "$name" "$result" "$start" "$end" >> "$out/results.csv" <<'PY'
import sys
name, result, start, end = sys.argv[1:5]
print(f"{name},{result},{float(end)-float(start):.2f}")
PY
  echo "$name: $result"
done
