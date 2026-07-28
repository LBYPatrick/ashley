#!/bin/bash
# Install OpenAI Codex CLI via the official native installer if not already present.
set -euo pipefail

GREEN=$'\033[0;32m'
CYAN=$'\033[0;36m'
RED=$'\033[0;31m'
NC=$'\033[0m'

if command -v codex &>/dev/null; then
    echo -e "${GREEN}Codex CLI already installed: $(codex --version 2>/dev/null | head -1)${NC}"
    exit 0
fi

echo -e "${CYAN}Codex CLI not found. Installing via native installer...${NC}"

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
echo -e "${GREEN}Codex CLI installed.${NC}"
