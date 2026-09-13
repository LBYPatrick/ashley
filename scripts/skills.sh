#!/bin/bash
# Shared skills.sh bootstrap, embedded in ash. All arguments belong to skills.
set -euo pipefail

case "$(uname -s)" in
    Darwin) pnpm_default="$HOME/Library/pnpm" ;;
    Linux) pnpm_default="${XDG_DATA_HOME:-$HOME/.local/share}/pnpm" ;;
    *) echo 'ash skills requires macOS or Linux (use WSL on Windows).' >&2; exit 1 ;;
esac
export PNPM_HOME="${PNPM_HOME:-$pnpm_default}"
tools_dir="$HOME/.ashley/tools"
export PATH="$PNPM_HOME/bin:$PNPM_HOME:$tools_dir/bin:$PATH:$HOME/.local/bin"

# Discover global bins even when the invoking shell has not been restarted.
refresh_path() {
    local bin_dir
    if command -v pnpm >/dev/null 2>&1; then
        bin_dir="$(pnpm bin -g 2>/dev/null)" || bin_dir=""
        if [[ -n "$bin_dir" ]]; then export PATH="$PATH:$bin_dir"; fi
    fi
    if command -v npm >/dev/null 2>&1; then
        bin_dir="$(npm prefix -g 2>/dev/null)" || bin_dir=""
        if [[ -n "$bin_dir" ]]; then export PATH="$PATH:$bin_dir/bin"; fi
    fi
    hash -r
}

# Current skills releases require Node >=22.20.0.
node_ready() {
    command -v node >/dev/null 2>&1 && node -e 'const [a,b] = process.versions.node.split(".").map(Number); process.exit(a > 22 || (a === 22 && b >= 20) ? 0 : 1)' >/dev/null 2>&1
}

refresh_path
if [[ "${ASHLEY_SKILLS_CHECK:-}" == 1 ]]; then command -v skills >/dev/null 2>&1; exit; fi
manager=""
if command -v pnpm >/dev/null 2>&1; then
    manager=pnpm
elif command -v npm >/dev/null 2>&1; then
    manager=npm
fi
need_node=false
node_ready || need_node=true
need_skills=false
command -v skills >/dev/null 2>&1 || need_skills=true
if [[ -z "$manager" || "$need_node" == true || "$need_skills" == true ]]; then
    echo 'Ashley / skills.sh installation plan:' >&2
    if [[ -z "$manager" || ( "$need_node" == true && "$manager" != pnpm ) ]]; then
        echo "  Install standalone pnpm from get.pnpm.io into $PNPM_HOME (may update shell configuration)." >&2
    fi
    if [[ "$need_node" == true ]]; then
        echo "  Install and activate Node.js LTS using pnpm in $PNPM_HOME." >&2
    fi
    if [[ "$need_skills" == true ]]; then
        echo "  Install the skills npm package using pnpm or npm (user-owned installation)." >&2
    fi
    echo '  Refresh PATH for this invocation, verify dependencies, then run skills.' >&2
    case "${ASHLEY_INSTALL_SKILLS:-}" in
        1|true|yes) ;;
        *)
            case "${ASHLEY_AUTOMATED:-}" in
                1|true|yes) echo 'Missing skills.sh dependencies; enable install_skills in the automated config.' >&2; exit 1 ;;
            esac
            printf 'Proceed with installation? [y/N] ' >&2
            answer=""
            read -r answer || true
            case "$answer" in
                y|Y|yes|YES|Yes) ;;
                *) echo 'Installation cancelled. For unattended setup, set ASHLEY_INSTALL_SKILLS=1.' >&2; exit 1 ;;
            esac
            ;;
    esac
    if [[ -z "$manager" || ( "$need_node" == true && "$manager" != pnpm ) ]]; then
        installer="$(mktemp)"
        trap 'rm -f "$installer"' EXIT
        curl --retry 3 -fsSL https://get.pnpm.io/install.sh -o "$installer"
        SHELL="${SHELL:-/bin/bash}" bash "$installer"
        rm -f "$installer"
        trap - EXIT
        export PATH="$PNPM_HOME/bin:$PNPM_HOME:$PATH"
        hash -r
        command -v pnpm >/dev/null 2>&1 || { echo 'pnpm installation did not produce an executable.' >&2; exit 1; }
        manager=pnpm
    fi
    if [[ "$need_node" == true ]]; then
        pnpm env use --global lts
        export PATH="$PNPM_HOME/bin:$PNPM_HOME:$PATH"
        hash -r
        node_ready || { echo 'Node.js installation did not provide Node >=22.20.0.' >&2; exit 1; }
    fi
    if [[ "$need_skills" == true ]]; then
        if [[ "$manager" == pnpm ]]; then
            pnpm add --global skills
        else
            npm install --global --prefix "$tools_dir" skills
        fi
        refresh_path
    fi
fi
command -v skills >/dev/null 2>&1 || { echo 'skills installation did not produce an executable.' >&2; exit 1; }
if [[ "${ASHLEY_SKILLS_ENSURE:-}" == 1 ]]; then exit 0; fi
exec skills "$@"
