# Go migration acceptance

The Go migration is complete on `feat/exp-go`. Ashley's runtime is a standalone
executable; Python remains in the open-source repository as a development test
reference. No Python, uv, Go toolchain, virtual environment, or checkout is
required to run a release installation.

## Requirements and evidence

| Requirement | Implementation and verification |
| --- | --- |
| Preserve skill generation and prompts | `internal/skills` embeds all 14 built-ins, supports JSON5 inheritance and append semantics, component/resource assembly, workflows, checklists, frontmatter and Jinja rendering. Tests compare full document/prompt hashes and custom outputs to Python fixtures; 36 template cases cover the supported existing template behavior. |
| Preserve custom authoring | The guided creator supports basics, inheritance, component/resource selection, workflow steps, checklists, assembled preview and advanced JSON editing. Real-terminal integration creates a skill and renders it through `ash prompt`. Definitions live under `~/.ashley/skills` or an explicit `--root`. Complete generated custom packages retain supporting files and executable permissions after removal of the checkout. |
| Preserve project detection | `internal/project` and frozen Python fixtures cover detected languages, frameworks, package managers and project context. |
| Support all five agents | `internal/agents`, `internal/invocation`, `internal/install` and `internal/upgrade` implement Claude, Codex, Grok Build, OpenCode and Kilo. Tests cover registry parity, native binary locations, argument construction, permission modes, environment overrides, installation, upgrade fallback and the Homebrew `codex.js` regression. Real-terminal first-install tests select Codex or all agents and verify links/preferences. |
| Preserve execution | Native run/pipeline execution, raw mode, agent selection, default/auto/DSP/AFK permissions, hooks and detached supervision. Tests exercise real subprocesses and an isolated tmux server, long literal prompts, failure propagation, completion hooks, cancellation and persisted outcomes. Explicit Normal mode overrides configured automatic permissions. |
| Preserve configuration and data | Existing YAML settings, JSON preferences/themes and SQLite history paths/schemas remain supported. Tests cover malformed/legacy settings, concurrent migration, queries, pagination, outcomes, analytics, prune/clear and preservation of unknown preference fields. Binary integration migrates a Python-created database and verifies Python can still read it. |
| Preserve interactive screens | Bubble Tea implements hub, vibe, sessions/log viewer, history, stats, settings/first-run setup, and skill creation. The original Textual panel geometry and information sections are retained. Snapshot tests compare all seven main screens against Python renders; populated-panel tests cover skill metadata, session details/logs and history fields. Bounds checks cover 50×20, 80×24, 100×30 and 140×50. Model tests exercise keyboard/mouse navigation, resize, filtering, modes, themes, actions and clipboard commands. Real-terminal tests launch and exit each standalone screen and complete skill creation. |
| Fix tmux wheel scrolling | New and reattached Ashley sessions use their own key table. Real-tmux tests verify scrollback in alternate-screen applications and preservation of unrelated bindings. |
| Ship executables | `scripts/release/package.sh` produces macOS/Linux arm64/amd64 archives containing only `ash` and `LICENSE`, with SHA-256 manifests. All four targets build with `CGO_ENABLED=0`. Integration runs a copied executable outside the checkout with an empty PATH. |
| Install and update without source | Release bootstrap downloads and validates a binary, preserves all agent selections, and atomically replaces the executable. A local release-server test downloads an actual compiled Ashley candidate, rejects a bad checksum, preserves the old executable on failure, and verifies skill refresh and agent-upgrade orchestration through the installed executable. Source updates remain an explicit developer-only `--root` operation with clean-checkout/fast-forward checks. |
| Full tests and Makefile | `make gate` runs formatting/lint/workflow validation, 188 Python regressions, fresh Python parity fixtures, Go race/coverage tests and vet, native build, and 22 binary integrations. The gate enforces aggregate Go coverage of at least 70%. `make go-dist` builds all four release targets. |
| Release CI and skill like Shelf | The repository publishing skill invokes `make publish`; the tag-triggered workflow checks version identity, runs the macOS/Linux gate, builds four archives, verifies checksums and publishes the complete artifact set while retaining draft notes. Publishing helper tests use command doubles; workflow syntax is checked with actionlint. |

## Footprint and operational limits

Release installation transfers one compressed executable archive and its checksum.
Ashley itself no longer downloads Python or resolves language dependencies.
Agent CLIs and tmux remain external dependencies; Kilo's installer requires npm.
Network access is still required to download releases and use remote agents.

The test suite verifies compatibility with the existing Ashley feature set and
custom-template cases, not every possible third-party Python/Jinja extension.
Cross-compilation proves all four targets build; local executable tests run on
the host architecture. GitHub Actions supplies the macOS/Linux gate when pushed.

The publishing infrastructure is implemented and locally validated. No remote
workflow execution or published Go release is claimed by this acceptance record.
See [release procedure](../releases.md) for the release procedure.
