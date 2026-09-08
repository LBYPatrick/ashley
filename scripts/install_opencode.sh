#!/bin/bash
# Install opencode via the official native installer.
# With --force, reinstall even when opencode is already present — that is
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

if ! $FORCE && command -v opencode &>/dev/null; then
    echo -e "${GREEN}opencode already installed: $(opencode --version 2>/dev/null | head -1)${NC}"
    exit 0
fi

if $FORCE; then
    echo -e "${CYAN}Installing latest opencode via native installer...${NC}"
else
    echo -e "${CYAN}opencode not found. Installing via native installer...${NC}"
fi

case "$(uname -s)" in
    Darwin | Linux)
        curl -fsSL https://opencode.ai/install | bash
        export PATH="$HOME/.opencode/bin:$PATH"
        ;;
    *)
        echo -e "${RED}Unsupported OS. Install opencode manually: https://opencode.ai/docs/${NC}" >&2
        exit 1
        ;;
esac

if ! command -v opencode &>/dev/null; then
    echo -e "${RED}opencode installation failed. Install manually: https://opencode.ai/docs/${NC}" >&2
    exit 1
fi
echo -e "${GREEN}opencode installed: $(opencode --version 2>/dev/null | head -1)${NC}"
