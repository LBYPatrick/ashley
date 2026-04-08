#!/bin/bash
set -euo pipefail

if command -v uv &>/dev/null; then
    echo "uv already installed ($(which uv))"
    exit 0
fi

echo "Installing uv..."
curl -LsSf https://astral.sh/uv/install.sh | sh

if command -v uv &>/dev/null; then
    echo "uv installed successfully."
else
    echo "uv installation failed. Please install manually: https://docs.astral.sh/uv/"
    exit 1
fi
