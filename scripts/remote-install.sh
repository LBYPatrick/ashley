#!/bin/bash
# Bootstrap a binary release; no checkout or language runtime is downloaded.
set -euo pipefail
repo="${ASHLEY_REPO:-LBYPatrick/ashley}"
install_dir="${ASHLEY_INSTALL_DIR:-$HOME/.local/bin}"
binary_args=()
agent_args=()
skills_only=()
binary_only=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        --version|--install-dir)
            [[ $# -ge 2 && -n "$2" ]] || { echo "$1 requires a value" >&2; exit 1; }
            binary_args+=("$1" "$2")
            if [[ "$1" == --install-dir ]]; then install_dir="$2"; fi
            shift 2
            ;;
        --claude|--codex|--grok|--opencode|--kilo|--both|--all)
            agent_args+=("$1")
            shift
            ;;
        --agent|--agent=*)
            if [[ "$1" == --agent ]]; then
                [[ $# -ge 2 ]] || { echo '--agent requires a value' >&2; exit 1; }
                agent="$2"
                shift 2
            else
                agent="${1#--agent=}"
                shift
            fi
            case "$agent" in
                claude|codex|grok|opencode|kilo|both|all) agent_args+=("--agent" "$agent") ;;
                *) echo "Unknown agent: $agent" >&2; exit 1 ;;
            esac
            ;;
        --skills-only) skills_only=(--skills-only); shift ;;
        --binary-only) binary_only=true; shift ;;
        --help)
            echo 'Usage: remote-install.sh [--version VERSION] [--install-dir DIR] [--AGENT ...] [--skills-only|--binary-only]'
            exit 0
            ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done
if [[ ${#agent_args[@]} -eq 0 && -n "${ASHLEY_AGENT:-}" ]]; then
    case "$ASHLEY_AGENT" in
        claude|codex|grok|opencode|kilo|both|all) agent_args=(--agent "$ASHLEY_AGENT") ;;
        *) echo "Unknown agent: $ASHLEY_AGENT" >&2; exit 1 ;;
    esac
fi
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
curl --retry 3 -fsSL "https://raw.githubusercontent.com/$repo/main/scripts/install.sh" -o "$tmp/install.sh"
bash "$tmp/install.sh" ${binary_args[@]+"${binary_args[@]}"}
if [[ "$binary_only" == false ]]; then
    "$install_dir/ash" install ${agent_args[@]+"${agent_args[@]}"} ${skills_only[@]+"${skills_only[@]}"}
fi
