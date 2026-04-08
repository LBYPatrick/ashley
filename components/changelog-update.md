# CHANGELOG Maintenance

Maintain a `CHANGELOG.md` in the project root following [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format with [Semantic Versioning](https://semver.org/).

## Format

```markdown
# Changelog

## [Unreleased]

### Added
- New features

### Changed
- Changes in existing functionality

### Fixed
- Bug fixes

### Removed
- Removed features

## [X.Y.Z] - YYYY-MM-DD
...
```

## Rules

1. **`[Unreleased]`** section sits at the top — all in-progress changes accumulate here until the next release.
2. **Categories** (use only those that apply): `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security`.
3. Each entry is a single bullet starting with an imperative verb (Add, Fix, Change, Remove, Update).
4. Group related changes under the most specific category. `Added` = wholly new features/files. `Changed` = modifications to existing behavior. `Fixed` = bug corrections.
5. When a version is released, rename `[Unreleased]` to `[X.Y.Z] - YYYY-MM-DD` and create a new empty `[Unreleased]` section above it.
6. **Do NOT** log internal refactors, formatting-only changes, or dependency bumps unless they affect user-visible behavior.

## When to Update

- **Every commit** that changes user-visible behavior, adds features, fixes bugs, or removes functionality.
- On commit: add entries under `[Unreleased]`. On release: move `[Unreleased]` entries to a versioned section.

## Creating CHANGELOG.md from Scratch

If the project has no `CHANGELOG.md`:
1. Create it with the header and an `[Unreleased]` section.
2. Optionally backfill recent history from `git log --oneline` grouped by version tags (if any).
3. Don't try to document every historical commit — focus on significant changes.
