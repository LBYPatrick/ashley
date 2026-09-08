#!/bin/bash
# Install Kilo Code via the official npm installer.
# With --force, reinstall even when Kilo Code is already present — that is
# how the npm installer upgrades an existing install in place.
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

if ! $FORCE && command -v kilo &>/dev/null; then
    echo -e "${GREEN}Kilo Code already installed: $(kilo --version 2>/dev/null | head -1)${NC}"
    exit 0
fi

if $FORCE; then
    echo -e "${CYAN}Installing latest Kilo Code via npm installer...${NC}"
else
    echo -e "${CYAN}Kilo Code not found. Installing via npm installer...${NC}"
fi

case "$(uname -s)" in
    Darwin | Linux)
        command -v npm >/dev/null || { echo "Install Node.js and npm first: https://nodejs.org" >&2; exit 1; }
        npm install -g @kilocode/cli
        export PATH="$HOME/.local/bin:$PATH"
        ;;
    *)
        echo -e "${RED}Unsupported OS. Install Kilo Code manually: https://kilo.ai/docs/code-with-ai/platforms/cli${NC}" >&2
        exit 1
        ;;
esac

if ! command -v kilo &>/dev/null; then
    echo -e "${RED}Kilo Code installation failed. Install manually: https://kilo.ai/docs/code-with-ai/platforms/cli${NC}" >&2
    exit 1
fi
echo -e "${GREEN}Kilo Code installed: $(kilo --version 2>/dev/null | head -1)${NC}"
