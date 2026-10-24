#!/bin/bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Builds the "android-vm" runtime package for macOS (Apple Silicon): QEMU and
# its libraries from Homebrew, the EDK2 firmware, scrcpy and adb from
# Genymobile's release. The tree matches internal/mobile/vm/runtime.go
# (installed as <runtime>/qemu):
#
#   bin/qemu-system-aarch64 bin/qemu-img bin/scrcpy bin/adb
#   lib/*.dylib   share/edk2-aarch64-code.fd share/edk2-arm-vars.fd share/scrcpy-server
#   SOURCES.txt   where the source of every GPL/LGPL part is
#
#   scripts/build-android-vm-runtime.sh <version> <out-dir> [codesign identity, "-" = ad hoc]
#
# Prints the engines.json entry (without its URL) to stdout when done.
set -euo pipefail
version=${1:?version, like 1.0.0}
out=${2:?output dir}
ident=${3:--}
here=$(cd "$(dirname "$0")/.." && pwd)
ents="$here/build/darwin/qemu.entitlements"

# Pinned scrcpy; how to update it: docs/mobile-maintenance.md.
scrcpy_ver=5.0.1
scrcpy_sha=33611e51977a8289e2e124b220f727895181ef2eb9060e0987a4dc2db9cf0d6b

# stdout is the engines.json entry: brew's progress goes to stderr.
brew list qemu >/dev/null 2>&1 || brew install qemu >&2
brew=$(brew --prefix)
qemu_ver=$(brew list --versions qemu | awk '{print $2}')

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
tree="$work/tree"
mkdir -p "$tree/bin" "$tree/lib" "$tree/share"

# --- QEMU and the libraries it links, rewritten to load from lib/.
cp "$brew/bin/qemu-system-aarch64" "$brew/bin/qemu-img" "$tree/bin/"
cp "$brew/share/qemu/edk2-aarch64-code.fd" "$brew/share/qemu/edk2-arm-vars.fd" "$tree/share/"
chmod u+w "$tree"/bin/* "$tree"/share/*

# deps prints the Homebrew libraries a Mach-O file links.
deps() { otool -L "$1" | awk 'NR>1{print $1}' | grep -E "^($brew|/usr/local)/" || true; }

queue=("$tree/bin/qemu-system-aarch64" "$tree/bin/qemu-img")
while [ ${#queue[@]} -gt 0 ]; do
  f=${queue[0]}
  queue=("${queue[@]:1}")
  for dep in $(deps "$f"); do
    name=$(basename "$dep")
    if [ ! -e "$tree/lib/$name" ]; then
      cp "$(realpath "$dep")" "$tree/lib/$name"
      chmod u+w "$tree/lib/$name"
      install_name_tool -id "@rpath/$name" "$tree/lib/$name" 2>/dev/null
      queue+=("$tree/lib/$name")
    fi
    case $f in
      "$tree"/bin/*) install_name_tool -change "$dep" "@executable_path/../lib/$name" "$f" 2>/dev/null ;;
      *) install_name_tool -change "$dep" "@loader_path/$name" "$f" 2>/dev/null ;;
    esac
  done
done
left=$(for f in "$tree"/bin/qemu-* "$tree"/lib/*; do deps "$f"; done | sort -u)
[ -z "$left" ] || { echo "still linked outside the package: $left" >&2; exit 1; }

# --- scrcpy and adb, from Genymobile's release (adb is Apache-2.0).
curl -fsSL -o "$work/scrcpy.tgz" \
  "https://github.com/Genymobile/scrcpy/releases/download/v$scrcpy_ver/scrcpy-macos-aarch64-v$scrcpy_ver.tar.gz"
echo "$scrcpy_sha  $work/scrcpy.tgz" | shasum -a 256 -c - >/dev/null
tar -xzf "$work/scrcpy.tgz" -C "$work"
s="$work/scrcpy-macos-aarch64-v$scrcpy_ver"
cp "$s/scrcpy" "$s/adb" "$tree/bin/"
cp "$s/scrcpy-server" "$tree/share/"

# --- Sign. QEMU needs the Hypervisor entitlement to use HVF. With a
# Developer ID every file shares one team, so the hardened runtime can check
# the libraries; ad hoc signatures have no team, and library validation would
# refuse them, so an ad hoc build goes without the hardened runtime.
opts=()
[ "$ident" = "-" ] || opts=(--options runtime --timestamp)
for f in "$tree"/lib/* "$tree"/bin/scrcpy "$tree"/bin/adb; do codesign --force --sign "$ident" ${opts[@]+"${opts[@]}"} "$f"; done
for f in "$tree"/bin/qemu-system-aarch64 "$tree"/bin/qemu-img; do
  codesign --force --sign "$ident" ${opts[@]+"${opts[@]}"} --entitlements "$ents" "$f"
done
codesign -d --entitlements - "$tree/bin/qemu-system-aarch64" 2>/dev/null | grep -q com.apple.security.hypervisor

# The package must run without Homebrew.
DYLD_FALLBACK_LIBRARY_PATH=/nonexistent "$tree/bin/qemu-system-aarch64" -version >/dev/null
"$tree/bin/qemu-img" --version >/dev/null

cat >"$tree/SOURCES.txt" <<EOF
AnyProfile android-vm runtime $version

QEMU $qemu_ver (GPL-2.0): https://download.qemu.org/qemu-$qemu_ver.tar.xz
  built by Homebrew: https://github.com/Homebrew/homebrew-core/blob/HEAD/Formula/q/qemu.rb
EDK2 firmware (BSD-2-Clause-Patent), shipped with QEMU $qemu_ver.
Libraries in lib/ (LGPL or permissive), as Homebrew built them:
$(for f in "$tree"/lib/*; do echo "  $(basename "$f")"; done)
  Sources: https://github.com/Homebrew/homebrew-core (one formula per library).
scrcpy $scrcpy_ver and adb (Apache-2.0): https://github.com/Genymobile/scrcpy/releases/tag/v$scrcpy_ver
Build script: scripts/build-android-vm-runtime.sh in https://github.com/D0bby999/anyprofile
EOF

mkdir -p "$out"
pkg="android-vm-$version-darwin-arm64.tar.xz"
tar -C "$tree" -cJf "$out/$pkg" .
sum() { shasum -a 256 "$1" | awk '{print $1}'; }
files="{}"
for rel in bin/qemu-system-aarch64 bin/qemu-img bin/scrcpy bin/adb share/scrcpy-server share/edk2-aarch64-code.fd share/edk2-arm-vars.fd; do
  files=$(jq --arg k "$rel" --arg v "$(sum "$tree/$rel")" '. + {($k): $v}' <<<"$files")
done
jq -n --arg v "$version" --arg q "$qemu_ver" --arg sha "$(sum "$out/$pkg")" --argjson files "$files" --arg pkg "$pkg" \
  '{engine: "android-vm", version: $v, chromium: $q, platform: "darwin-arm64", url: ("REPLACE/" + $pkg),
    sha256: $sha, files: $files, bin: "bin/qemu-system-aarch64", catalogue: "android-vm"}'
echo "built $out/$pkg ($(du -h "$out/$pkg" | cut -f1))" >&2
