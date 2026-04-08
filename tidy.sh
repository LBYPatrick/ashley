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
if compgen -G "scripts/*.sh" >/dev/null 2>&1; then
    uv run -m beautysh scripts/*.sh
fi
if [ -f "tidy.sh" ]; then
    uv run -m beautysh tidy.sh
fi

echo "Formatting complete."
