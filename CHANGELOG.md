# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-07-09

### Added

- Add a configurable appearance system: light/dark mode plus a colour swatch of single-tone (Blue, Green, Purple, Orange, Rose, Cyan) and dual-tone (Ocean, Sunset, Grape, Forest) presets, saved to `~/.ashley/theme.json` and auto-loaded on every launch
- Add a first-run setup wizard that prompts for appearance on the first TUI launch
- Add a Settings screen and a Stats analytics screen to the TUI hub
- Add "Kill all" (`X`) and sort-by-time/skill (`s`) actions to the Sessions TUI, with hotkeys
- Make the whole TUI fully keyboard-operable (arrows, Tab, Enter, Esc) for SSH/mosh, including unified arrow-key navigation across the Settings screen sections
- Add gradient-shaded bars to the Stats analytics so neighbouring bars stay distinguishable
- Add `c` key in the Sessions TUI to copy the selected session ID to the clipboard

### Changed

- Launch every `ash run` inside a tmux session; without `--detached` it now attaches immediately and cleans up on exit (tmux is now required to run skills)
- Polish the TUI: rounded panels, accent-coloured highlights, consistent theming, and header clocks
- Highlight the first session on load so Enter attaches immediately without moving the cursor
- Rebind Sessions TUI keys: `k` now cleans up dead sessions and `K` kills the selected session

### Removed

- Remove success-rate tracking and run-duration figures from analytics

### Fixed

- Escape user-controlled text (session/history questions, paths, and log output) so content containing `[` is no longer parsed as Textual markup and can no longer crash the TUI
- Fix the colour swatches rendering as transparent blocks caused by the mount fade-in compositing their backgrounds
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
