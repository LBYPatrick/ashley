#!/bin/bash
# Install a prebuilt Ashley release. Requires no Go, Python, uv, git, or npm.
set -euo pipefail
version="${ASHLEY_VERSION:-}"
install_dir="${ASHLEY_INSTALL_DIR:-$HOME/.local/bin}"
repo="${ASHLEY_REPO:-LBYPatrick/ashley}"
while [[ $# -gt 0 ]]; do
    case "$1" in
        --version | --install-dir)
            [[ $# -ge 2 ]] || { echo "$1 requires a value" >&2; exit 1; }
            case "$1" in
                --version) version="$2" ;;
                --install-dir) install_dir="$2" ;;
            esac
            shift 2
            ;;
        --claude | --codex | --grok | --opencode | --kilo | --both | --all | --agent=* | --binary-only)
            # Agent setup is handled by remote-install.sh after binary installation.
            shift
            ;;
        --help)
            echo "Usage: install.sh [--version X.Y.Z] [--install-dir DIR]"
            exit 0
            ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done
case "$(uname -s)" in
    Darwin) platform=darwin ;;
    Linux) platform=linux ;;
    *) echo "Ashley binary releases support macOS and Linux." >&2; exit 1 ;;
esac
case "$(uname -m)" in
    arm64 | aarch64) arch=arm64 ;;
    x86_64 | amd64) arch=amd64 ;;
    *) echo "Unsupported CPU architecture." >&2; exit 1 ;;
esac
if [[ -z "$version" ]]; then
    latest="$(curl --retry 3 -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")"
    version="${latest##*/}"
fi
version="${version#v}"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-z]+\.[0-9]+)?$ ]] || { echo "Invalid release version: $version" >&2; exit 1; }
archive="ashley-$version-$platform-$arch.tar.gz"
url="https://github.com/$repo/releases/download/v$version"
tmp="$(mktemp -d)"
candidate=""
trap 'rm -rf "$tmp"; if [[ -n "$candidate" ]]; then rm -f "$candidate"; fi' EXIT
curl --retry 3 -fsSL "$url/$archive" -o "$tmp/$archive"
curl --retry 3 -fsSL "$url/$archive.sha256" -o "$tmp/$archive.sha256"
(
    cd "$tmp"
    read -r expected checksum_file < "$archive.sha256"
    [[ "$checksum_file" == "$archive" && "$expected" =~ ^[0-9a-fA-F]{64}$ ]] || { echo "Invalid checksum manifest" >&2; exit 1; }
    if command -v sha256sum >/dev/null; then
        actual="$(sha256sum "$archive")"
    else
        actual="$(shasum -a 256 "$archive")"
    fi
    [[ "${actual%% *}" == "$expected" ]] || { echo "Checksum mismatch" >&2; exit 1; }
)
tar -xzf "$tmp/$archive" -C "$tmp" ash
[[ -f "$tmp/ash" && ! -L "$tmp/ash" ]] || { echo "Archive does not contain an ash executable" >&2; exit 1; }
chmod +x "$tmp/ash"
actual="$("$tmp/ash" --version)"
[[ "$actual" == "ashley $version" ]] || { echo "Downloaded binary version mismatch: $actual" >&2; exit 1; }
mkdir -p "$install_dir"
candidate="$(mktemp "$install_dir/.ash-XXXXXX")"
cp "$tmp/ash" "$candidate"
chmod 755 "$candidate"
mv -f "$candidate" "$install_dir/ash"
candidate=""
echo "Installed Ashley $version to $install_dir/ash"
case ":$PATH:" in
    *":$install_dir:"*) ;;
    *) echo "Add $install_dir to PATH to run ash." ;;
esac
