# Software Installation Guidelines

When a task requires installing software or dependencies:

## Package Manager First
- If the project has a package manager configured (`uv`, `pnpm`, `cargo`, `go mod`), **use it**. Do not introduce a second one.

## Custom Install Scripts
- If no package manager is available, create `scripts/install-xxx.sh` (where `xxx` is the tool name).
- Requirements: (1) Check existing installation first (lazy/idempotent), (2) Support `aarch64`/`arm64`/`amd64`, (3) Support Linux (Debian/Ubuntu, RHEL/Fedora, Alpine) + macOS, (4) `set -euo pipefail`, (5) Color-coded success/failure output.

## Script Template
```bash
#!/bin/bash
set -euo pipefail
if command -v <tool> &>/dev/null; then
    echo "<tool> already installed ($(which <tool>))"; exit 0
fi
ARCH="$(uname -m)"; OS="$(uname -s)"
case "$OS" in
    Linux) case "$ARCH" in
        x86_64|amd64) PLATFORM="linux-amd64" ;; aarch64|arm64) PLATFORM="linux-arm64" ;;
        *) echo "Unsupported arch: $ARCH"; exit 1 ;; esac ;;
    Darwin) case "$ARCH" in
        x86_64|amd64) PLATFORM="darwin-amd64" ;; aarch64|arm64) PLATFORM="darwin-arm64" ;;
        *) echo "Unsupported arch: $ARCH"; exit 1 ;; esac ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac
# Install logic here...
echo "<tool> installed successfully"
```
