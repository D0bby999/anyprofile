#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Serves the pinned AnyBrowser zip over loopback HTTP for development, while
# the real release URL is still a placeholder. The app accepts this only when
# ANYPROFILE_ANYBROWSER_DEV=1 is set and the URL is 127.0.0.1; the SHA256 is
# still checked by the installer.
#
#   scripts/dev-anybrowser-server.sh [zip] [port]
#
# Then start the app with the two variables it prints. In another shell:
#   ANYPROFILE_ANYBROWSER_DEV=1 \
#   ANYPROFILE_ANYBROWSER_URL=http://127.0.0.1:8765/anybrowser-156.0.8078.74-macos-arm64.zip \
#     build/bin/AnyProfile.app/Contents/MacOS/AnyProfile
set -eu
zip=${1:-$HOME/chromium-work/anybrowser-trial-20261029-101420/work/dist-74/anybrowser-156.0.8078.74-macos-arm64.zip}
port=${2:-8765}
want=d338a7c0169e053310c3749039dd126ceb179fd9a4ae2a71dc04d13a61b72d5f
[ -f "$zip" ] || { echo "missing $zip" >&2; exit 1; }
got=$(shasum -a 256 "$zip" | awk '{print $1}')
[ "$got" = "$want" ] || { echo "$zip: sha256 $got, want $want" >&2; exit 1; }
name=$(basename "$zip")
echo "serving $name on http://127.0.0.1:$port/$name" >&2
echo "ANYPROFILE_ANYBROWSER_DEV=1" >&2
echo "ANYPROFILE_ANYBROWSER_URL=http://127.0.0.1:$port/$name" >&2
cd "$(dirname "$zip")"
exec python3 -m http.server "$port" --bind 127.0.0.1
