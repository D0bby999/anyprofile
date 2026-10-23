#!/bin/bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Prints the engines.json entry pinning a LineageOS image from
# jqssun/android-lineage-qemu, after checking it: GitHub's digest for the
# asset, then the build attestation (gh attestation verify). AnyProfile never
# hosts the image; the app downloads it from this URL and checks these hashes.
#
#   scripts/pin-android-vm-image.sh v2026.10.18
#
# Run a device on it end to end before pinning (internal/mobile/vm,
# go test -tags vmintegration), then put the entry in engines.json, or in
# vm.ImageBuiltin for the image new installs get.
set -euo pipefail
tag=${1:?release tag, like v2026.10.18}
repo=jqssun/android-lineage-qemu
asset=$(gh api "repos/$repo/releases/tags/$tag" \
  --jq '.assets[] | select(.name | test("^UTM-VM-lineage-.*-virtio_arm64only\\.zip$")) | [.name, .browser_download_url, .digest] | @tsv')
IFS=$'\t' read -r name url digest <<<"$asset"
[ -n "$name" ] || { echo "no arm64 UTM image in $tag" >&2; exit 1; }
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
curl -fL --progress-bar -o "$work/$name" "$url"
sha=$(shasum -a 256 "$work/$name" | awk '{print $1}')
[ "sha256:$sha" = "$digest" ] || { echo "sha256 $sha is not GitHub's $digest" >&2; exit 1; }
gh attestation verify "$work/$name" --repo "$repo" >&2
lineage=$(sed -E 's/^UTM-VM-lineage-([0-9.]+)-([0-9]{8})-.*/\1/' <<<"$name")
date=$(sed -E 's/^UTM-VM-lineage-([0-9.]+)-([0-9]{8})-.*/\2/' <<<"$name")
data=LineageOS_on_arm64.utm/Data
vda=$(unzip -p "$work/$name" "$data/vda.qcow2" | shasum -a 256 | awk '{print $1}')
vdb=$(unzip -p "$work/$name" "$data/vdb.qcow2" | shasum -a 256 | awk '{print $1}')
jq -n --arg v "$lineage.$date" --arg url "$url" --arg sha "$sha" --arg vda "$vda" --arg vdb "$vdb" --arg d "$data" \
  --arg date "${date:0:4}-${date:4:2}-${date:6:2}" \
  '{engine: "android-vm-image", version: $v, chromium: "16.0", platform: "darwin-arm64", date: $date, url: $url,
    sha256: $sha, files: {($d + "/vda.qcow2"): $vda, ($d + "/vdb.qcow2"): $vdb}, bin: ($d + "/vda.qcow2"),
    catalogue: "android-vm-image"}'
echo "check the Android version (chromium) against the release notes before signing" >&2
