#!/bin/bash
# Install OpenAI Codex CLI via the official native installer.
# With --force, reinstall even when Codex is already present — that is how
# the native installer upgrades an existing install in place.
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

if ! $FORCE && command -v codex &>/dev/null; then
    echo -e "${GREEN}Codex CLI already installed: $(codex --version 2>/dev/null | head -1)${NC}"
    exit 0
fi

if $FORCE; then
    echo -e "${CYAN}Installing latest Codex CLI via native installer...${NC}"
else
    echo -e "${CYAN}Codex CLI not found. Installing via native installer...${NC}"
fi

case "$(uname -s)" in
    Darwin | Linux)
        curl -fsSL https://chatgpt.com/codex/install.sh | sh
        export PATH="$HOME/.local/bin:$PATH"
        ;;
    *)
        echo -e "${RED}Unsupported OS. Install Codex manually: https://developers.openai.com/codex${NC}" >&2
        exit 1
        ;;
esac

if ! command -v codex &>/dev/null; then
    echo -e "${RED}Codex CLI installation failed. Install manually: https://developers.openai.com/codex${NC}" >&2
    exit 1
fi
echo -e "${GREEN}Codex CLI installed: $(codex --version 2>/dev/null | head -1)${NC}"
