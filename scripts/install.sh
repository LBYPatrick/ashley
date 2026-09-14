#!/bin/bash
# Install a prebuilt Ashley release. Requires no Go, Python, uv, git, or npm.
set -euo pipefail
version="${ASHLEY_VERSION:-}"
install_dir="${ASHLEY_INSTALL_DIR:-$HOME/.local/bin}"
repo="${ASHLEY_REPO:-LBYPatrick/ashley}"
while [[ $# -gt 0 ]]; do
    case "$1" in
        --version | --install-dir)
            [[ $# -ge 2 ]] || { echo "$1 requires a value" >&2; exit 1; }
            case "$1" in
                --version) version="$2" ;;
                --install-dir) install_dir="$2" ;;
            esac
            shift 2
            ;;
        --claude | --codex | --grok | --opencode | --kilo | --both | --all | --agent=* | --binary-only)
            # Agent setup is handled by remote-install.sh after binary installation.
            shift
            ;;
        --help)
            echo "Usage: install.sh [--version X.Y.Z] [--install-dir DIR]"
            exit 0
            ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done

# Inspect launchers as files: the old Python runtime may already be gone.
resolve_launcher() {
    local resolved="$1" target depth
    for ((depth=0; depth<40; depth++)); do
        if [[ ! -L "$resolved" ]]; then printf '%s\n' "$resolved"; return; fi
        target="$(readlink "$resolved")"
        if [[ "$target" == /* ]]; then resolved="$target"; else resolved="$(dirname "$resolved")/$target"; fi
    done
    echo "Symlink loop: $1" >&2
    return 1
}
native_launcher() {
    [[ -f "$1" ]] || return 1
    case "$(od -An -tx1 -N4 "$1" | tr -d ' \n')" in
        7f454c46|cffaedfe|feedfacf|cefaedfe|feedface|cafebabe|bebafeca) return 0 ;;
        *) return 1 ;;
    esac
}
legacy_checkout() {
    local resolved parent
    resolved="$(resolve_launcher "$1")" || return 1
    [[ "$(basename "$resolved")" == ash && "$(basename "$(dirname "$resolved")")" == bin ]] || return 0
    parent="$(dirname "$resolved")/.."
    if [[ -d "$parent/skills" ]]; then (cd "$parent" && pwd -P); fi
}
known_wrapper() {
    [[ -f "$1" ]] || return 1
    [[ "$(head -c 2 "$1")" == '#!' ]] || return 1
    # Match repository launcher commands without invoking an old runtime.
    head -c 8192 "$1" | grep -Fqx -e 'exec uv run --project "$ASHLEY_ROOT" ash "$@"' -e 'exec "$ASHLEY_ROOT/build/ash-go" "$@"'
}
prepare_migration() {
    source_dir=""
    shadow=""
    backup=""
    path_launcher="$(type -P ash || true)"
    if [[ -n "$path_launcher" ]]; then
        path_launcher="$(cd "$(dirname "$path_launcher")" && pwd -P)/$(basename "$path_launcher")"
    fi
    local old detected migrate=false
    for old in "$launcher" "$path_launcher"; do
        [[ -n "$old" ]] || continue
        detected=""
        if [[ -L "$old" ]]; then detected="$(legacy_checkout "$old")"; fi
        if [[ -n "$detected" ]]; then
            if [[ -n "$source_dir" && "$source_dir" != "$detected" ]]; then
                echo 'Conflicting legacy checkouts; adjust PATH to select one before installing.' >&2
                exit 1
            fi
            source_dir="$detected"
        fi
        if [[ -n "$detected" ]] || known_wrapper "$old"; then
            migrate=true
            if [[ "$old" != "$launcher" ]]; then shadow="$old"; fi
        fi
    done
    if [[ -e "$launcher" || -L "$launcher" ]] && ! native_launcher "$launcher"; then migrate=true; fi
    if [[ -z "$source_dir" && ( "$migrate" == true || ! -e "$launcher" ) ]]; then
        local parent="${ASHLEY_DIR:-$HOME/.ashley/repo}"
        if [[ -d "$parent/skills" ]]; then source_dir="$(cd "$parent" && pwd -P)"; migrate=true; fi
    fi
    [[ "$migrate" == true ]] || return 0
    if [[ -n "$shadow" && ! -w "$(dirname "$shadow")" ]]; then
        printf 'Legacy launcher shadows the installation and requires administrator permission: %s\n' "$shadow" >&2
        printf 'Put the install directory first on PATH before rerunning:\n  export PATH=%q:"$PATH"\n' "$install_dir" >&2
        exit 1
    fi
    # Downloads and version validation have succeeded before any backups/writes.
    (umask 077; mkdir -p "$HOME/.ashley/migrations")
    backup="$(mktemp -d "$HOME/.ashley/migrations/python-to-go-XXXXXX")"
    if [[ -e "$launcher" || -L "$launcher" ]]; then cp -Pp "$launcher" "$backup/ash"; fi
    if [[ -n "$shadow" ]]; then cp -Pp "$shadow" "$backup/path-ash"; fi
    printf 'Legacy source: %s\nLauncher: %s\nPATH launcher: %s\n' "$source_dir" "$launcher" "$shadow" > "$backup/origin.txt"
    printf '  Migration backup  %s\n' "$backup"
}
repair_migration_path() {
    if [[ -n "$shadow" ]]; then
        local stage
        stage="$(mktemp -d "$(dirname "$shadow")/.ash-link-XXXXXX")"
        ln -s "$launcher" "$stage/ash"
        mv -f "$stage/ash" "$shadow"
        rmdir "$stage"
    fi
    if [[ -n "$path_launcher" && "$path_launcher" != "$launcher" && -z "$shadow" ]]; then
        printf '  Another executable is first on PATH: %s\n  Prefer this installation: export PATH=%q:"$PATH"\n' "$path_launcher" "$install_dir"
    fi
    if [[ -n "$backup" ]]; then
        echo '  Migration complete. User data, the old checkout, and shared runtimes were retained.'
        echo '  Start a new shell or run hash -r.'
    fi
}

case "$(uname -s)" in
    Darwin) platform=darwin ;;
    Linux) platform=linux ;;
    *) echo "Ashley binary releases support macOS and Linux." >&2; exit 1 ;;
esac
case "$(uname -m)" in
    arm64 | aarch64) arch=arm64 ;;
    x86_64 | amd64) arch=amd64 ;;
    *) echo "Unsupported CPU architecture." >&2; exit 1 ;;
esac
if [[ -z "$version" ]]; then
    latest="$(curl --retry 3 -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")"
    version="${latest##*/}"
fi
version="${version#v}"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-z]+\.[0-9]+)?$ ]] || { echo "Invalid release version: $version" >&2; exit 1; }
printf '\n  Ashley / Download\n\n  Version      %s\n  Platform     %s / %s\n  Downloading and verifying binary…\n' "$version" "$platform" "$arch"
archive="ashley-$version-$platform-$arch.tar.gz"
url="https://github.com/$repo/releases/download/v$version"
tmp="$(mktemp -d)"
candidate=""
trap 'rm -rf "$tmp"; if [[ -n "$candidate" ]]; then rm -f "$candidate"; fi' EXIT
# The bootstrap is commonly piped into bash, so detect the terminal on stderr.
# Only the archive transfer has meaningful progress; metadata stays quiet.
download_flags=(-fsSL)
if [[ -t 2 ]]; then download_flags=(-fSL --progress-bar); fi
curl --retry 3 "${download_flags[@]}" "$url/$archive" -o "$tmp/$archive"
curl --retry 3 -fsSL "$url/$archive.sha256" -o "$tmp/$archive.sha256"
(
    cd "$tmp"
    read -r expected checksum_file < "$archive.sha256"
    [[ "$checksum_file" == "$archive" && "$expected" =~ ^[0-9a-fA-F]{64}$ ]] || { echo "Invalid checksum manifest" >&2; exit 1; }
    if command -v sha256sum >/dev/null; then
        actual="$(sha256sum "$archive")"
    else
        actual="$(shasum -a 256 "$archive")"
    fi
    [[ "${actual%% *}" == "$expected" ]] || { echo "Checksum mismatch" >&2; exit 1; }
)
tar -xzf "$tmp/$archive" -C "$tmp" ash
[[ -f "$tmp/ash" && ! -L "$tmp/ash" ]] || { echo "Archive does not contain an ash executable" >&2; exit 1; }
chmod +x "$tmp/ash"
actual="$("$tmp/ash" --version)"
[[ "$actual" == "ashley $version" ]] || { echo "Downloaded binary version mismatch: $actual" >&2; exit 1; }
case "${ASHLEY_AUTOMATED:-}" in
    1|true|yes) "$tmp/ash" __validate-automation ;;
esac
mkdir -p "$install_dir"
install_dir="$(cd "$install_dir" && pwd -P)"
launcher="$install_dir/ash"
[[ ! -d "$launcher" ]] || { echo "Launcher is a directory: $launcher" >&2; exit 1; }
source_dir=""; shadow=""; backup=""; path_launcher=""
if [[ "${ASHLEY_MIGRATION_STAGING:-}" != 1 ]]; then prepare_migration; fi
if [[ -n "$backup" ]]; then
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

    # Relink only existing agents during migration; normal bootstrap setup follows.
    # A private automation profile prevents dependency installation or prompts here.
    migration_agents=()
    migration_agent_args=()
    keys=(claude codex grok opencode kilo)
    directories=("${CLAUDE_CONFIG_DIR:-$HOME/.claude}" "${CODEX_HOME:-$HOME/.codex}" "${GROK_HOME:-$HOME/.grok}" "${OPENCODE_CONFIG_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}/opencode}" "$HOME/.kilo")
    for i in "${!keys[@]}"; do
        if [[ -d "${directories[$i]}/skills" ]]; then
            migration_agents+=("${keys[$i]}")
            migration_agent_args+=("--${keys[$i]}")
        fi
    done
    if [[ ${#migration_agents[@]} -gt 0 ]]; then
        automation="$tmp/migration.json"
        (umask 077
            printf '{"agents":[' > "$automation"
            separator=""
            for agent in "${migration_agents[@]}"; do
                printf '%s"%s"' "$separator" "$agent" >> "$automation"
                separator=,
            done
            printf '],"skills_only":true,"install_skills":false,"community_skills":false}\n' >> "$automation"
        )
        legacy_args=()
        if [[ -n "$source_dir" ]]; then legacy_args=(--legacy-root "$source_dir"); fi
        # Skill installation saves its first agent as the default. Migration
        # must retain the existing preference, including on a failed import.
        prefs="$HOME/.ashley/prefs.json"
        had_prefs=false
        if [[ -e "$prefs" || -L "$prefs" ]]; then
            cp -Pp "$prefs" "$backup/prefs.json"
            had_prefs=true
        fi
        migration_status=0
        ASHLEY_AUTOMATED=1 ASHLEY_AUTOMATED_CONFIG="$automation" "$tmp/ash" install --skills-only "${migration_agent_args[@]}" ${legacy_args[@]+"${legacy_args[@]}"} </dev/null || migration_status=$?
        if [[ "$had_prefs" == true ]]; then
            candidate="$(mktemp "$HOME/.ashley/.prefs-migrate-XXXXXX")"
            cp -Pp "$backup/prefs.json" "$candidate"
            mv -f "$candidate" "$prefs"
            candidate=""
        else
            rm -f "$prefs"
        fi
        [[ "$migration_status" == 0 ]] || exit "$migration_status"
    fi
fi
candidate="$(mktemp "$install_dir/.ash-XXXXXX")"
cp "$tmp/ash" "$candidate"
chmod 755 "$candidate"
mv -f "$candidate" "$install_dir/ash"
candidate=""
repair_migration_path
printf '\n  ✓ Installed Ashley %s\n  Executable   %s/ash\n\n' "$version" "$install_dir"
if [[ "${ASHLEY_BOOTSTRAP:-}" != 1 ]]; then
case ":$PATH:" in
    *":$install_dir:"*) ;;
    *) printf '  Add to PATH  export PATH=%q:"$PATH"\n\n' "$install_dir" ;;
esac
fi

# Explicit opt-in only; the embedded bootstrap also serves ash skills.
if [[ "${ASHLEY_BOOTSTRAP:-}" != 1 ]]; then
    case "${ASHLEY_AUTOMATED:-}" in
        1|true|yes) "$install_dir/ash" install </dev/null; exit ;;
    esac
    case "${ASHLEY_INSTALL_SKILLS:-}" in
        1|true|yes) "$install_dir/ash" skills --version ;;
    esac
fi
