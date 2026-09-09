#!/bin/bash
# Maintainer workflow: validate, gate, commit version files, push/tag and draft.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
version="${V:-$(tr -d '[:space:]' < VERSION)}"
tag="v$version"
branch="$(git branch --show-current)"
[[ "$branch" == main ]] || { echo "Publish from main after the migration work is merged." >&2; exit 1; }
stray="$(git status --porcelain | grep -Ev '^.. (VERSION|README\.md)$' || true)"
[[ -z "$stray" ]] || { echo "Commit outstanding work and changelog notes before publishing:" >&2; echo "$stray" >&2; exit 1; }
if [[ -n "${NOTES:-}" && ! -s "$NOTES" ]]; then
    echo "NOTES must be a nonempty Markdown file: $NOTES" >&2
    exit 1
fi
command -v gh >/dev/null || { echo "Publishing requires the GitHub CLI (gh)." >&2; exit 1; }
gh auth status
git fetch --tags origin
[[ "$(git rev-list --count HEAD..origin/main)" == 0 ]] || { echo "main is behind origin/main; update it first." >&2; exit 1; }
if git show-ref --verify --quiet "refs/tags/$tag"; then
    echo "$tag already exists; choose a new version." >&2
    exit 1
fi
go run ./scripts/release prepare "$version"
go run ./scripts/release check "$tag"
make gate
[[ "${YES:-}" == 1 ]] || { echo "Checks passed. Run make publish V=$version YES=1 to publish."; exit 0; }
git add VERSION README.md
if ! git diff --cached --quiet; then
    git commit -m "chore(release): prepare $tag"
fi
# Prepare release arguments before any pushes. Actions preserve authored notes.
release_args=(--draft --target "$(git rev-parse HEAD)" --title "Ashley $version")
if [[ "$version" == *-* ]]; then release_args+=(--prerelease); fi
if [[ -n "${NOTES:-}" ]]; then
    [[ -f "$NOTES" ]] || { echo "Missing NOTES markdown file: $NOTES" >&2; exit 1; }
    release_args+=(--notes-file "$NOTES")
else
    release_args+=(--generate-notes)
fi
git push origin main
git tag -a "$tag" -m "Ashley $version"
git push origin "$tag"
gh release create "$tag" --verify-tag "${release_args[@]}"
echo "Release builds: $(gh repo view --json url --jq .url)/actions/workflows/release.yaml"
