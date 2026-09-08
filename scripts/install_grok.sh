#!/bin/bash
# Install grok via the official native installer.
# With --force, reinstall even when grok is already present — that is
# how the native installer upgrades an existing install in place.
set -euo pipefail

GREEN=$'\033[0;32m'
CYAN=$'\033[0;36m'
RED=$'\033[0;31m'
NC=$'\033[0m'

FORCE=false
for arg in "$@"; do
    case "$arg" in
        --force | --upgrade) FORCE=true ;;
        *)
            echo -e "${RED}Unknown option: $arg${NC}" >&2
            exit 1
            ;;
    esac
done

if ! $FORCE && command -v grok &>/dev/null; then
    echo -e "${GREEN}grok already installed: $(grok --version 2>/dev/null | head -1)${NC}"
    exit 0
fi

if $FORCE; then
    echo -e "${CYAN}Installing latest grok via native installer...${NC}"
else
    echo -e "${CYAN}grok not found. Installing via native installer...${NC}"
fi

case "$(uname -s)" in
    Darwin | Linux)
        curl -fsSL https://x.ai/cli/install.sh | bash
        export PATH="${GROK_BIN_DIR:-$HOME/.grok/bin}:$HOME/.local/bin:$PATH"
        ;;
    *)
        echo -e "${RED}Unsupported OS. Install grok manually: https://x.ai/cli${NC}" >&2
        exit 1
        ;;
esac

if ! command -v grok &>/dev/null; then
    echo -e "${RED}grok installation failed. Install manually: https://x.ai/cli${NC}" >&2
    exit 1
fi
echo -e "${GREEN}grok installed: $(grok --version 2>/dev/null | head -1)${NC}"
