#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Fails when a source file lacks the SPDX license line in its first three lines.
set -eu
cd "$(dirname "$0")/.."
missing=$(git ls-files --cached --others --exclude-standard -- \
  '*.go' '*.ts' '*.tsx' '*.mjs' '*.rs' '*.sql' '*.sh' '*.css' '*.py' \
  ':!frontend/wailsjs/**' ':!*.d.ts' ':!examples/**' |
  while read -r f; do head -n 3 "$f" | grep -q 'SPDX-License-Identifier: AGPL-3.0-or-later' || echo "$f"; done)
if [ -n "$missing" ]; then
  echo "missing SPDX-License-Identifier: AGPL-3.0-or-later in:" >&2
  echo "$missing" >&2
  exit 1
fi
# Examples are 0BSD, so people can copy them into scripts of their own.
missing=$(git ls-files --cached --others --exclude-standard -- 'examples/*.mjs' 'examples/*.js' 'examples/*.py' |
  while read -r f; do head -n 3 "$f" | grep -q 'SPDX-License-Identifier: 0BSD' || echo "$f"; done)
if [ -n "$missing" ]; then
  echo "missing SPDX-License-Identifier: 0BSD in:" >&2
  echo "$missing" >&2
  exit 1
fi
