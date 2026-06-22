# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Add `c` key in the Sessions TUI to copy the selected session ID to the clipboard

### Changed

- Highlight the first session on load so Enter attaches immediately without moving the cursor
- Rebind Sessions TUI keys: `k` now cleans up dead sessions and `K` kills the selected session

### Fixed

- Prevent the Sessions TUI from crashing on broken session records (e.g. a malformed timestamp after a reboot); corrupt entries now render gracefully

## [0.1.0] - 2026-04-04

### Added

- Initial release with 14 skills: feat, refactor, debug, optimize, brainstorm, scaffold, coding, commit, rebase, pretty, ci, readme, changelog, claudemd
- Interactive Textual TUI for browsing and launching skills
- Click-based CLI (`ash`) with run, generate, list, prompt, install, uninstall, update commands
- Always-inline resource embedding for maximum portability
- Skill inheritance system with composable components
- Google Style Guide compliance component for Python, TypeScript, Go, Java, C++, Shell, Dart
- MIT license
