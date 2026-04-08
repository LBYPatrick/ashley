#!/bin/bash
set -euo pipefail

# Resolve the real script location (follows symlinks)
SOURCE="${BASH_SOURCE[0]}"
while [ -L "$SOURCE" ]; do
    DIR="$(cd "$(dirname "$SOURCE")" && pwd)"
    SOURCE="$(readlink "$SOURCE")"
    [[ "$SOURCE" != /* ]] && SOURCE="$DIR/$SOURCE"
done
ASHLEY_ROOT="$(cd "$(dirname "$SOURCE")/.." && pwd)"

# Ensure uv is available
if ! command -v uv &>/dev/null; then
    echo "Error: uv not found. Install it: https://docs.astral.sh/uv/" >&2
    exit 1
fi

# Run the ashley CLI via uv from the project root
exec uv run --project "$ASHLEY_ROOT" ash "$@"
