# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Replace the Python runtime with a standalone Go executable, preserving skill generation, custom templates, configuration, history, pipelines, lifecycle hooks, tmux sessions and all interactive screens.
- Add a guided skill creator with workflow/resource selection and advanced JSON editing; retain complete custom skill packages without a source checkout.
- Add a full Makefile gate covering Python regressions, Go parity/race/coverage checks, binary smoke tests, and release installer tests.
- Add binary archives, checksum-verified installation, a publish-release skill, and GitHub Actions for macOS/Linux release builds; require full parity before stable binary releases.
- Add Grok Build, OpenCode, and Kilo Code support across installation, invocation, pipelines, settings, and upgrades.
- Add `ash install --all` for all five agents while retaining `--both` for Claude Code and Codex.

### Changed

- Group regression tests, reference fixtures, release tooling, and migration documentation into dedicated folders; expand `make clean` to remove development artifacts.

### Fixed

- Rework Settings with a centered layout, compact grouped choices, small palette swatches, distinct focus and selection indicators, and responsive keyboard navigation.

- Fix the Go Settings freeze and panel styling spills by composing terminal cells instead of duplicating ANSI sequences.
- Restore the original Stats lightness gradients and rounded bar lengths.
- Fix session log rendering with live tmux screen capture, archived redraw handling, and scrolling based on wrapped viewport lines.
- Restore the original Python terminal layouts, information panels, inline session logs, theme swatches and grayscale selection styling in the Go UI; add cross-language screen snapshots and resize regression checks.

- Capture mouse-wheel scrolling in Ashley tmux sessions instead of forwarding arrow keys to coding agents, including when reattaching existing sessions.
- Fix npm-installed executables such as `codex.js` under the Homebrew prefix being mistaken for Homebrew formulae.
- Respect explicit Normal permissions when automatic permissions are configured, and retain all agent selections during binary bootstrap installation.

## [0.3.0] - 2026-08-03

### Added

- Support [OpenAI Codex](https://developers.openai.com/codex) alongside Claude Code as a coding-agent backend; the same generated skills install unchanged for either agent (`~/.claude/skills` and `~/.codex/skills` both read `SKILL.md`)
- Ask which coding agent to set up on first install, with `--claude` / `--codex` / `--both` (or `AGENT=` / `ASHLEY_AGENT=`) to skip the question
- Save the chosen agent as an Ashley preference in `~/.ashley/prefs.json`, configurable later via `ash agent <name>` or the TUI Settings screen
- Add `-c` / `--claude` and `-o` / `--codex` to `ash run` and `ash pipe` to override the agent for a single invocation
- Map every run mode onto Codex's equivalent flags: DSP → `--dangerously-bypass-approvals-and-sandbox`, AUTO → `--sandbox workspace-write --ask-for-approval never`, AFK → DSP plus the autonomous-operation instructions
- Add `scripts/install_codex.sh` for native Codex CLI installation
- Track the coding agent per invocation in the history database and break usage down by agent in `ash history stats` and the TUI Stats screen, with `--agent` to filter
- Add `ash upgrade` to detect and update the coding-agent CLIs: Homebrew installs are upgraded with `brew upgrade` (or `brew upgrade --cask`), an installed agent is asked to update itself (`claude update` / `codex update`), and a missing agent — or one whose own updater refuses, as an npm install would — falls back to the vendor's native installer; npm and pnpm are never invoked
- Check for a new version before downloading anything, so re-running `ash upgrade` or `ash update` on an up-to-date agent costs a version check rather than a full reinstall
- Detect an agent's install source by resolving its binary out of Homebrew's `Cellar`/`Caskroom`, so no network call or package-name guessing is needed
- Add `--all` and `--check` to `ash upgrade`, plus a `make upgrade` target (`AGENT=claude|codex`)
- Accept `--force` / `--upgrade` in `scripts/install_claude.sh` and `scripts/install_codex.sh` so the native installers reinstall in place instead of exiting early
- Upgrade the agent CLIs as the final step of `ash update`, covering whichever agents have Ashley skills linked; set `SKIP_TOOL=true` (or `1`/`yes`) to skip it

### Changed

- Rename the Settings screen to "Preferences" and add a coding-agent picker above the appearance controls
- Record the coding agent on each detached session and show it in the `ash run --detached` summary
- `ash install` now reinstalls for whichever agents already have skills instead of re-asking, so `ash update` stays non-interactive
- Honour `CLAUDE_CONFIG_DIR` and `CODEX_HOME` when locating an agent's skills directory
- Migrate existing history databases in place on the next run, adding the `agent_type` column and attributing all prior invocations to Claude Code
- Break ties in the analytics group-by deterministically so bar ordering no longer varies between runs
- Show the installed version and install source in `ash agent`
- Report the version delta after an agent upgrade (`now at X (was Y)`), or that it was already up to date

### Fixed

- Keep `ash update` running to completion when the generated skills are unchanged; it previously returned early and skipped the remaining steps

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
