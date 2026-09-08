#!/bin/bash
# Migrate a Python Ashley installation without invoking Python, uv, Go, or git.
set -euo pipefail
install_dir="${ASHLEY_INSTALL_DIR:-$HOME/.local/bin}"
source_dir=""
binary=""
version="${ASHLEY_VERSION:-}"
repo="${ASHLEY_REPO:-LBYPatrick/ashley}"
agent_args=()
while [[ $# -gt 0 ]]; do
    case "$1" in
        --binary|--source|--version|--install-dir)
            [[ $# -ge 2 && -n "$2" ]] || { echo "$1 requires a value" >&2; exit 1; }
            case "$1" in
                --binary) binary="$2" ;;
                --source) source_dir="$2" ;;
                --version) version="$2" ;;
                --install-dir) install_dir="$2" ;;
            esac
            shift 2 ;;
        --claude|--codex|--grok|--opencode|--kilo|--both|--all)
            agent_args+=("$1"); shift ;;
        --help)
            echo 'Usage: migrate-python.sh [--version VERSION | --binary PATH] [--source CHECKOUT] [--install-dir DIR] [--AGENT ...]'
            echo 'Defaults: latest binary release, detected legacy checkout, ~/.local/bin; skills for configured/detected agents.'
            exit 0 ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done
[[ -z "$binary" || -z "$version" ]] || { echo 'Choose --binary or --version, not both.' >&2; exit 1; }
launcher="$install_dir/ash"
# Resolve the legacy launcher without ever executing it (its runtime may be gone).
if [[ -z "$source_dir" && -L "$launcher" ]]; then
    resolved="$launcher"
    for ((depth=0; depth<40; depth++)); do
        [[ -L "$resolved" ]] || break
        target="$(readlink "$resolved")"
        if [[ "$target" == /* ]]; then resolved="$target"; else resolved="$(dirname "$resolved")/$target"; fi
    done
    if [[ -f "$resolved" && "$(basename "$(dirname "$resolved")")" == bin ]]; then
        legacy_root="$(cd "$(dirname "$resolved")/.." && pwd)"
        if [[ -d "$legacy_root/skills" ]]; then source_dir="$legacy_root"; fi
    fi
fi
if [[ -z "$source_dir" && -d "${ASHLEY_DIR:-$HOME/.ashley/repo}/skills" ]]; then
    source_dir="${ASHLEY_DIR:-$HOME/.ashley/repo}"
fi
if [[ -n "$source_dir" ]]; then
    [[ -d "$source_dir/skills" ]] || { echo "Not an Ashley source directory: $source_dir" >&2; exit 1; }
    source_dir="$(cd "$source_dir" && pwd)"
fi
tmp="$(mktemp -d)"
candidate=""
trap 'rm -rf "$tmp"; if [[ -n "$candidate" ]]; then rm -f "$candidate"; fi' EXIT
if [[ -n "$binary" ]]; then
    [[ -f "$binary" ]] || { echo "Binary not found: $binary" >&2; exit 1; }
    cp "$binary" "$tmp/ash"
else
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    if [[ -f "$script_dir/install.sh" ]]; then
        cp "$script_dir/install.sh" "$tmp/install.sh"
    else
        curl --retry 3 -fsSL "https://raw.githubusercontent.com/$repo/main/scripts/install.sh" -o "$tmp/install.sh"
    fi
    version_args=()
    if [[ -n "$version" ]]; then version_args=(--version "$version"); fi
    bash "$tmp/install.sh" --install-dir "$tmp" ${version_args[@]+"${version_args[@]}"}
fi
# Reject Python/shell launchers before executing a user-supplied --binary.
magic="$(od -An -tx1 -N4 "$tmp/ash" | tr -d ' \n')"
case "$magic" in
    7f454c46|cffaedfe|feedfacf|cefaedfe|feedface|cafebabe|bebafeca) ;;
    *) echo 'Migration requires a native Ashley release binary.' >&2; exit 1 ;;
esac
chmod 755 "$tmp/ash"
"$tmp/ash" --version
mkdir -p "$install_dir" "$HOME/.ashley/migrations"
backup="$(mktemp -d "$HOME/.ashley/migrations/python-to-go-XXXXXX")"
if [[ -e "$launcher" || -L "$launcher" ]]; then cp -Pp "$launcher" "$backup/ash"; fi
printf 'Legacy source: %s\nLauncher: %s\n' "$source_dir" "$launcher" > "$backup/origin.txt"
echo "Migration backup: $backup"
# Snapshot only user skill data, never the checkout's .venv or live SQLite files.
for directory in skills components res generated; do
    destination="$HOME/.ashley/$directory"
    [[ ! -L "$destination" ]] || { echo "Refusing symlinked user data directory: $destination" >&2; exit 1; }
    if [[ -d "$destination" ]]; then
        [[ -z "$(find "$destination" -type l -print -quit)" ]] || { echo "User skill data contains symlinks: $destination" >&2; exit 1; }
        cp -Rp "$destination" "$backup/$directory"
    fi
    if [[ -n "$source_dir" && -d "$source_dir/$directory" && "$source_dir" != "$HOME/.ashley" ]]; then
        [[ -z "$(find "$source_dir/$directory" -type l -print -quit)" ]] || { echo "Legacy skill data contains symlinks: $source_dir/$directory" >&2; exit 1; }
        # Copy missing files explicitly: cp -n has different exit semantics
        # across macOS and Linux when a destination already exists.
        mkdir -p "$destination"
        while IFS= read -r -d '' file; do
            relative="${file#"$source_dir/$directory/"}"
            if [[ -e "$destination/$relative" ]]; then continue; fi
            mkdir -p "$(dirname "$destination/$relative")"
            cp -p "$file" "$destination/$relative"
        done < <(find "$source_dir/$directory" -type f -print0)
    fi
done
if [[ ${#agent_args[@]} -eq 0 ]]; then
    keys=(claude codex grok opencode kilo)
    directories=("${CLAUDE_CONFIG_DIR:-$HOME/.claude}" "${CODEX_HOME:-$HOME/.codex}" "${GROK_HOME:-$HOME/.grok}" "${OPENCODE_CONFIG_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}/opencode}" "$HOME/.kilo")
    for i in "${!keys[@]}"; do
        key="${keys[$i]}"
        if [[ -d "${directories[$i]}" ]] || command -v "$key" >/dev/null 2>&1 || [[ -x "$HOME/.local/bin/$key" ]] || [[ -x "$HOME/.$key/bin/$key" ]]; then
            agent_args+=("--$key")
        fi
    done
    [[ ${#agent_args[@]} -gt 0 ]] || { echo 'No agents detected; rerun with --claude, --codex, or --all.' >&2; exit 1; }
fi
# Imported definitions/resources are a persistent user overlay; no checkout is needed.
legacy_args=()
if [[ -n "$source_dir" ]]; then legacy_args=(--legacy-root "$source_dir"); fi
"$tmp/ash" install --skills-only "${agent_args[@]}" ${legacy_args[@]+"${legacy_args[@]}"}
candidate="$(mktemp "$install_dir/.ash-migrate-XXXXXX")"
cp "$tmp/ash" "$candidate"
chmod 755 "$candidate"
mv -f "$candidate" "$launcher"
candidate=""
echo "Migrated to native Ashley: $launcher"
echo "Settings, history, sessions, the old checkout, and shared Python/uv installations are retained."
echo "Start a new shell (or run hash -r), then run: ash --version"
echo "Launcher backup: $backup/ash (if a previous launcher existed)."
