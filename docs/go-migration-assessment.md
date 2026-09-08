# Go migration assessment

Status: investigated; no rewrite started. Full feature parity is required.

## Finding

A Go implementation is feasible and should simplify end-user installation:
ship an executable per OS/architecture, with no Python, uv, virtual environment,
or Go toolchain required on the user's machine. Go compiles dependencies into
an executable ([Go build tutorial](https://go.dev/doc/tutorial/compile-install)).
Use pure-Go dependencies and validate builds with `CGO_ENABLED=0`.

The current launcher invokes `uv run --project` on every launch. Installation
runs `uv sync`; updates require a Git checkout, pull source, sync dependencies,
and regenerate skills. The local development `.venv` occupies approximately
50 MiB, excluding the Python interpreter and external caches; this is a local
development measurement, not a clean-user install benchmark. No Go binary size
or startup benchmark exists yet, so exact savings are unmeasured.

The reviewed source is about 6,450 Python lines, including roughly 2,600 lines
of TUI code. This is a real application port, particularly the interface and
custom template support, rather than just replacing the launcher.

## Feature parity requirements

| Surface | Required behavior in Go |
| --- | --- |
| CLI | Preserve commands, aliases, options, exit behavior, help, prompt output, raw runs, and per-run agent selection. |
| Agents | Claude, Codex, Grok, OpenCode, Kilo; installation, upgrades, environment overrides, skills, permission modes, AFK, saved defaults. |
| Skills | JSONC/JSON5 definitions, inheritance and append semantics, component assembly, resource inlining, workflows, checklists, frontmatter, project detection, runtime template rendering. |
| Custom authoring | Create/edit custom skills, components and resources; retain existing files and format compatibility. Shipping only pre-generated bundled skills would lose functionality. |
| TUIs | Hub, skill browser, prompt previews, clipboard actions, custom skill creator, first-run wizard, themes, agent picker, history and session browsers. Preserve keyboard/mouse navigation and terminal resize behavior. |
| Execution | Pipelines and named pipelines, hooks, working directories, permission metadata, subprocess exits, tmux creation/attachment, detached execution, large prompt files, shell quoting, logs, kill and session sorting. |
| Persistence | Read existing config.yaml, prefs.json, theme.json, history and session records, including old records lacking agent metadata. Preserve filtering, search, pagination, analytics, pruning and clearing. |
| Distribution | Installer, uninstall, version, agent upgrades, self-update, and skill refresh. Preserve branch-oriented development/update workflows alongside released binaries. |

## Proposed implementation and distribution

1. Capture current behavior in cross-language fixtures: generated markdown,
   custom templates, invocation arguments, config/history/session migration,
   shell quoting and failure cases. Reuse the current 167-test suite as a
   behavior inventory; supplement with end-to-end checks.
2. Port configuration, skill generation, project detection, history, agents,
   execution, hooks and sessions. Go's `text/template` is not a Jinja replacement:
   evaluate compatibility against existing tests and user-authored Jinja features
   before selecting an engine. Do not silently limit templates to built-in examples.
3. Rebuild all Textual screens. [Bubble Tea](https://github.com/charmbracelet/bubbletea)
   is a candidate Go terminal framework, but layouts, styling and interaction need
   implementation and parity testing; Textual code/CSS cannot be reused directly.
4. Embed bundled definitions, components, resources and installer assets with
   [`go:embed`](https://pkg.go.dev/embed). Extract writable defaults into an Ashley
   data directory; preserve user overrides. Agent skill directories must point to
   real files on disk, not an embedded virtual filesystem or temporary directory.
5. Publish macOS and Linux arm64/amd64 archives, checksums and a Homebrew package.
   Make installation download the matching release and atomically replace the
   executable. Update bundled skill assets without overwriting customizations.
   Keep source/development mode for branch-based workflows, with Go needed only
   by developers, and test migration from existing symlinks/checkouts.
6. Switch distribution only after all feature groups pass parity tests. Measure
   clean-machine download count/size, installed footprint, startup latency and
   update behavior on each platform before claiming specific savings.

Agent CLIs and tmux remain external dependencies. Kilo's current documented
installer requires npm. A Go Ashley removes Ashley's Python dependency; it does
not remove the runtimes or network access needed by external agents. Release
installs still need a download, but avoid Python distribution and dependency
resolution requests. AI requests remain network-dependent.

Recommendation: proceed with a full-parity Go port as a separate implementation
project, keeping the Python release available until the complete Go application
passes acceptance. A CLI-only or pre-generated-skills-only release would not
meet the user's requirements.
