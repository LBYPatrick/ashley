#!/bin/bash
# Build one self-contained release archive. No interpreter or checkout is shipped.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
platform="${1:-$(go env GOOS)}"
arch="${2:-$(go env GOARCH)}"
case "$platform/$arch" in
    darwin/arm64 | darwin/amd64 | linux/arm64 | linux/amd64) ;;
    *) echo "Unsupported release platform: $platform/$arch" >&2; exit 1 ;;
esac
version="$(tr -d '[:space:]' < VERSION)"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-z]+\.[0-9]+)?$ ]] || { echo "Invalid VERSION" >&2; exit 1; }
archive="ashley-$version-$platform-$arch.tar.gz"
mkdir -p dist
staging="$(mktemp -d)"
trap 'rm -rf "$staging"' EXIT
CGO_ENABLED=0 GOOS="$platform" GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o "$staging/ash" ./cmd/ash
cp LICENSE "$staging/LICENSE"
COPYFILE_DISABLE=1 tar -czf "dist/$archive" -C "$staging" ash LICENSE
(
    cd dist
    if command -v sha256sum >/dev/null; then
        sha256sum "$archive" > "$archive.sha256"
    else
        shasum -a 256 "$archive" > "$archive.sha256"
    fi
)
echo "dist/$archive"
