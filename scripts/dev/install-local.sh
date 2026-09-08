#!/bin/bash
# Atomically install a locally built executable; never follow an old launcher link.
set -euo pipefail
binary="${1:?binary path required}"
install_dir="${2:?install directory required}"
mkdir -p "$install_dir"
candidate="$(mktemp "$install_dir/.ash-install-XXXXXX")"
trap 'rm -f "$candidate"' EXIT
cp "$binary" "$candidate"
chmod 755 "$candidate"
"$candidate" --version
mv -f "$candidate" "$install_dir/ash"
echo "Installed: $install_dir/ash"
