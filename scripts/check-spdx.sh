#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Fails when a source file lacks the SPDX license line in its first three lines.
set -eu
cd "$(dirname "$0")/.."
missing=$(git ls-files --cached --others --exclude-standard -- \
  '*.go' '*.ts' '*.tsx' '*.mjs' '*.rs' '*.sql' '*.sh' '*.css' \
  ':!frontend/wailsjs/**' ':!*.d.ts' |
  while read -r f; do head -n 3 "$f" | grep -q 'SPDX-License-Identifier: AGPL-3.0-or-later' || echo "$f"; done)
if [ -n "$missing" ]; then
  echo "missing SPDX-License-Identifier: AGPL-3.0-or-later in:" >&2
  echo "$missing" >&2
  exit 1
fi
