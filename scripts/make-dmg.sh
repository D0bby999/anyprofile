#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Packs an app into the disk image people download: open it, drag AnyProfile
# onto Applications. Needs dmgbuild (MIT), installed into a throwaway venv.
#
#   scripts/make-dmg.sh build/bin/AnyProfile.app AnyProfile-macos-arm64.dmg
#
# The in-app updater keeps using the .zip: AnyProfile 0.1.0 and 0.1.1 only
# know how to unpack that.
set -eu
app=$1
out=$2
root=$(cd "$(dirname "$0")/.." && pwd)
venv=${DMGBUILD_VENV:-${TMPDIR:-/tmp}/anyprofile-dmgbuild}
if [ ! -x "$venv/bin/dmgbuild" ]; then
  python3 -m venv "$venv"
  "$venv/bin/pip" install --quiet 'dmgbuild==1.6.7'
fi
rm -f "$out"
"$venv/bin/dmgbuild" -s "$root/build/darwin/dmg/settings.py" -D app="$app" -D dir="$root/build/darwin/dmg" AnyProfile "$out"
# A signing identity other than ad hoc signs the image too, as notarization
# asks; MACOS_SIGN_ID is set only when the release has a Developer ID.
if [ -n "${MACOS_SIGN_ID:-}" ]; then
  codesign --force --timestamp --sign "$MACOS_SIGN_ID" "$out"
fi
