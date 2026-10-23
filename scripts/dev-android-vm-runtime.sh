#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Lays out the Android VM runtime for development, from Homebrew (qemu,
# android-platform-tools, scrcpy) and a LineageOS image, in the same tree the
# "android-vm" and "android-vm-image" downloads make (internal/mobile/vm/runtime.go).
#
#   scripts/dev-android-vm-runtime.sh <runtime-dir> <UTM-VM-lineage-*.zip or its Data folder>
#
# The runtime dir of a dev build is "<data dir>/mobile/runtime"; see docs/android-vm-runtime.md.
set -eu
rt=${1:?runtime dir}
src=${2:?image zip or Data folder}
brew=$(brew --prefix)
for f in "$brew/bin/qemu-system-aarch64" "$brew/bin/qemu-img" "$brew/share/qemu/edk2-aarch64-code.fd" \
  "$brew/share/qemu/edk2-arm-vars.fd" "$brew/bin/adb" "$brew/bin/scrcpy" "$brew/share/scrcpy/scrcpy-server"; do
  [ -e "$f" ] || { echo "missing $f: brew install qemu android-platform-tools scrcpy" >&2; exit 1; }
done
mkdir -p "$rt/qemu/bin" "$rt/qemu/share" "$rt/image"
ln -sf "$brew/bin/qemu-system-aarch64" "$brew/bin/qemu-img" "$brew/bin/adb" "$brew/bin/scrcpy" "$rt/qemu/bin/"
ln -sf "$brew/share/qemu/edk2-aarch64-code.fd" "$brew/share/qemu/edk2-arm-vars.fd" "$brew/share/scrcpy/scrcpy-server" "$rt/qemu/share/"

if [ -d "$src" ]; then
  data=$src
  ver=$(basename "$(dirname "$src")")
else
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  unzip -q "$src" -d "$tmp"
  data=$(find "$tmp" -type d -name Data | head -n 1)
  ver=$(basename "$src" .zip)
fi
case $ver in UTM-VM-lineage-*) ver=${ver#UTM-VM-lineage-}; ver=${ver%%-jqssun*} ;; esac
[ "$ver" = "LineageOS_on_arm64.utm" ] && ver=dev
mkdir -p "$rt/image/$ver"
for f in vda.qcow2 vdb.qcow2; do
  [ -e "$rt/image/$ver/$f" ] || cp -c "$data/$f" "$rt/image/$ver/$f" 2>/dev/null || cp "$data/$f" "$rt/image/$ver/$f"
done
printf '%s\n' "$ver" >"$rt/image/CURRENT"
echo "runtime ready in $rt (image $ver)"
