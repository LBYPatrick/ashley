#!/bin/bash
# Install Claude Code via the official native installer if not already present.
set -euo pipefail

GREEN=$'\033[0;32m'
CYAN=$'\033[0;36m'
RED=$'\033[0;31m'
NC=$'\033[0m'

if command -v claude &>/dev/null; then
    echo -e "${GREEN}Claude Code already installed: $(claude --version 2>/dev/null | head -1)${NC}"
    exit 0
fi

echo -e "${CYAN}Claude Code not found. Installing via native installer...${NC}"

case "$(uname -s)" in
    Darwin | Linux)
        curl -fsSL https://claude.ai/install.sh | bash
        export PATH="$HOME/.local/bin:$PATH"
        ;;
    *)
        echo -e "${RED}Unsupported OS. Install Claude Code manually: https://claude.ai/download${NC}" >&2
        exit 1
        ;;
esac

if ! command -v claude &>/dev/null; then
    echo -e "${RED}Claude Code installation failed. Install manually: https://claude.ai/download${NC}" >&2
    exit 1
fi
echo -e "${GREEN}Claude Code installed.${NC}"
