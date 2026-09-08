---
name: publish-release
description: Prepare and publish an Ashley binary release using make publish and the tag-triggered GitHub Actions workflow. Use when asked to cut, publish, or ship an Ashley release.
---

# Publish an Ashley release

The source remains public; release downloads contain the `ash` executable and
LICENSE. End users must not need a checkout, Go, Python, or uv to run Ashley.

1. Read `CHANGELOG.md`, the commits since the latest tag, and
   `docs/migration/status.json`. Stable binaries require every parity item
   to be complete and tested. While migration is incomplete, only an explicitly
   described experimental prerelease is eligible; do not mark features complete
   merely to bypass the release check.
2. Use the user's version, or choose SemVer from the visible changes. Supported
   prereleases are `X.Y.Z-alpha.N`, `X.Y.Z-beta.N`, and `X.Y.Z-rc.N`. Report the
   choice and proceed within the requested release scope.
3. Move Unreleased entries to `## [X.Y.Z] - YYYY-MM-DD` and leave a fresh
   Unreleased section. Commit the changelog and implementation before publishing.
4. Write reader-facing release notes to a temporary Markdown file. For a
   prerelease, state which Go features are available and which still require the
   Python application. Do not advertise incomplete binaries as a replacement.
5. Run `make publish V=X.Y.Z NOTES=/absolute/path/notes.md YES=1` from `main`.
   The script synchronizes VERSION, Python metadata and the README badge, checks
   the release identity and parity status, runs `make gate`, commits version
   files, pushes main and an annotated tag, then creates draft notes.
   It refuses outstanding non-version changes, an existing tag, or a branch
   behind origin/main. Resolve the reported condition; never force-push a tag.
6. Monitor `.github/workflows/release.yaml`. All four platform archives and
   their checksums must build before the draft goes live. The workflow retains
   authored notes and uploads only binaries/checksums. Report the release URL
   and build result. If a publish step fails, inspect remote tag/release state
   before retrying; do not delete a published release or overwrite its tag.

`make gate` runs formatting checks, Python regression tests, Go race/coverage
tests, Python-to-Go parity checks, builds and standalone smoke tests. Python and
uv are development/CI tools during migration, never release dependencies.

Adding or editing this skill is not authorization to publish a release.
