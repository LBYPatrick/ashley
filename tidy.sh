#!/bin/bash
set -euo pipefail

SKIP_CHECK="${1:-}"

# Lazy check for formatter installation
if [ "$SKIP_CHECK" != "--skip-check" ]; then
    if ! uv run ruff --version &>/dev/null; then
        echo "Formatters not found. Installing dev dependencies..."
        uv sync --group dev
    fi
fi

# Python: lint and fix
uv run ruff check --select I,F401 --fix .

# Python: format
uv run ruff format .

# Shell scripts: format
while IFS= read -r -d '' script; do
    uv run -m beautysh "$script"
done < <(find scripts -type f -name '*.sh' -print0)
if [ -f "tidy.sh" ]; then
    uv run -m beautysh tidy.sh
fi

# Format the native runtime alongside the Python development reference.
if [ -f "go.mod" ] && command -v gofmt >/dev/null 2>&1; then
    gofmt -w assets.go cmd internal
fi

echo "Formatting complete."
