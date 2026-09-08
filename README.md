<h1 align="center">Ashley</h1>

<p align="center">
  <strong>Interactive skill set framework for <a href="https://docs.anthropic.com/en/docs/claude-code">Claude Code</a>, <a href="https://developers.openai.com/codex">OpenAI Codex</a>, Grok Build, OpenCode, and Kilo Code</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-native_binary-00ADD8?logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/version-0.3.0-blue" alt="Version" />
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="License" /></a>
</p>

---

Ashley provides 14 composable, production-ready skills that encode software engineering best practices as structured prompts for your coding agent. Skills are assembled from reusable components and inlined resources, then installed for Claude Code (`/a-feat`), OpenAI Codex (`$a-feat`), Grok Build, OpenCode, or Kilo Code — the same `SKILL.md` serves all five.

- **14 specialized skills** — feat, refactor, debug, optimize, commit, scaffold, and more
- **Five agent backends** — Claude Code, Codex, Grok Build, OpenCode, and Kilo Code, switchable globally or per run
- **Skill pipelines** — chain skills with `+` syntax (`feat+commit+changelog`) or named pipelines
- **Lifecycle hooks** — run shell commands before/after any skill execution
- **Project detection** — auto-detects tech stack for context-aware prompts
- **Interactive TUI** — hub with skill browser, session manager, history, analytics, and themeable appearance
- **tmux-backed sessions** — every run is crash-resilient; detach to background with `--detached`
- **Invocation history** — every run logged to SQLite for search and review

---

## Go migration (`feat/exp-go`)

Ashley now runs as a standalone Go executable. All CLI entry points and the
interactive screens have Go implementations verified by the migration acceptance
suite. Existing YAML settings, JSON preferences, and SQLite history are
preserved. The Python implementation remains a development-only test reference.
Evidence is recorded in [the migration acceptance](docs/migration/assessment.md).

```bash
make build
./build/ash-go -i
./build/ash-go list
./build/ash-go agent
./build/ash-go install --all --skills-only  # install prompts without installing CLIs
./build/ash-go history stats
./build/ash-go run --codex --detached feat "Add login"
./build/ash-go prompt feat "Add login"
./build/ash-go generate --output /tmp/ashley-skills
./build/ash-go --root /path/to/custom-ashley prompt --project /path/to/project feat
./build/ash-go update --check # inspect the latest stable release
make gate                  # full local/CI test and build gate
make go-dist               # four binary archives + SHA-256 checksums
```

The binary runs without Go, Python, uv, or a source checkout. Release archives
contain only `ash` and `LICENSE`; external coding-agent CLIs and tmux remain
separate dependencies. The repository stays
open source. Once binary releases are available, `scripts/install.sh` downloads
the matching release, verifies its checksum/version, and replaces the executable
atomically. It does not clone or compile source or resolve language dependencies.


`ash-go update` downloads a matching binary release, validates its archive and
SHA-256 checksum, checks the executable version, and atomically replaces the
running executable. It refreshes installed prompts using the new binary and
upgrades tracked agents unless `--skip-tools` or `SKIP_TOOL=1` is set. Use
`--version X.Y.Z` for a specific release, or `--install-dir ~/.local/bin` to
replace an old launcher symlink while leaving its checkout intact. For explicit developer-checkout updates, use
`ash-go --root /path/to/ashley update --branch feat/example`; this requires a
clean Git checkout and Go, and leaves user installations on the binary path.

Development and CI still use Go and uv/Python to test the port against the
reference application. `make test` runs the complete current suite, including
Python regressions, Go race tests with a 70% coverage floor, parity fixtures,
and binary/package/installer integration tests. `make gate` also checks formatting,
static analysis, and GitHub Actions workflows.

Releases follow the Shelf workflow: use the repository's
[publish-release skill](.agents/skills/publish-release/SKILL.md) and
`make publish V=X.Y.Z NOTES=/path/to/notes.md YES=1`. A `v*` tag triggers tests on
macOS/Linux and builds all four macOS/Linux arm64/amd64 archives. The draft goes
live only after all packages and checksums are present. Incomplete migrations
can only produce explicitly labeled prereleases. See [release development](docs/releases.md).

## Quick Start

### Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| macOS or Linux | arm64 or amd64 | A matching prebuilt release; no language runtime needed |
| Claude Code, Codex, Grok Build, OpenCode, or Kilo Code | latest | For running skills — installed for you |
| [tmux](https://github.com/tmux/tmux) | latest | Required — every run launches in a tmux session |

### One-Line Install

```bash
curl -fsSL https://raw.githubusercontent.com/LBYPatrick/ashley/main/scripts/remote-install.sh | bash
```

Downloads and verifies the matching release binary into `~/.local/bin/ash`,
then installs skills and the selected agent. No checkout, Python, uv, or Go
toolchain is installed. Binary release assets must be published before using
this installation path; use the developer build below to try this branch.

The installer asks which coding agent to set up. Skip the question with a flag:

```bash
curl -fsSL .../remote-install.sh | bash -s -- --codex   # or --claude, --grok, --opencode, --kilo, --all
```

<details>
<summary>Build from source (developers)</summary>

```bash
git clone https://github.com/LBYPatrick/ashley.git
cd ashley
make install
```

</details>

<details>
<summary>Environment variables</summary>

| Variable | Default | Description |
|----------|---------|-------------|
| `ASHLEY_INSTALL_DIR` | `~/.local/bin` | Binary installation directory |
| `ASHLEY_REPO` | `LBYPatrick/ashley` | GitHub release repository |
| `ASHLEY_VERSION` | latest stable | Specific binary release version |
| `ASHLEY_NO_COLOR` | unset | Disable colored output (`1` to enable) |
| `ASHLEY_AGENT` | unset | Preselect the agent (`claude`, `codex`, `grok`, `opencode`, `kilo`, `both`, or `all`) |

</details>

### Uninstall

```bash
ash uninstall
rm ~/.local/bin/ash
# Your settings, custom skills, logs, and history are retained.
```

---

## Usage

```bash
# Launch interactive TUI (default)
ash

# Run a skill directly
ash run feat "Add a login page"
ash run debug "Fix the 500 error on /api/users"

# Autonomous mode (no prompts)
ash run -afk feat "Add dark mode toggle"

# Pick the agent just for this run
ash run -o feat "Add dark mode toggle"     # OpenAI Codex
ash run -c feat "Add dark mode toggle"     # Claude Code

# Run a pipeline (chain skills)
ash pipe feat+commit+changelog "Add OAuth support"

# Detached session (background)
ash run --detached feat "Add OAuth support"

# Generate a copy-pasteable prompt
ash prompt feat "Add OAuth support"

# List available skills
ash list
```

---

## Available Skills

| Skill | Description |
|-------|-------------|
| `a-feat` | Implement a new feature from a spec |
| `a-refactor` | Refactor code for quality and performance |
| `a-debug` | Find and fix bugs |
| `a-optimize` | Speed up slow code |
| `a-brainstorm` | Design and build a new project from scratch |
| `a-scaffold` | Set up project scaffolding (Makefile, scripts) |
| `a-coding` | General coding quality guard |
| `a-commit` | Stage, format, and commit with conventional messages |
| `a-rebase` | Clean up branch commits |
| `a-pretty` | Set up formatter and linter tooling |
| `a-ci` | Set up or fix CI pipeline |
| `a-readme` | Write a polished README |
| `a-changelog` | Create or update CHANGELOG.md |
| `a-claudemd` | Generate project CLAUDE.md guidelines |

---

## Pipelines

Chain multiple skills into sequential execution. The first skill receives your question; subsequent skills run with their default trigger. Execution stops on first failure.

```bash
# Inline pipeline with '+' syntax
ash pipe feat+commit "Add login page"
ash pipe refactor+commit+changelog "Clean up auth"

# Named pipelines in ~/.ashley/config.yaml
pipelines:
  ship:
    - feat
    - commit
    - changelog
```

---

## Hooks

Run shell commands before or after skill execution. Hooks receive context via environment variables (`$ASHLEY_SKILL`, `$ASHLEY_QUESTION`, `$ASHLEY_EXIT_CODE`).

```yaml
# ~/.ashley/config.yaml
hooks:
  global:
    before_run: "echo 'Starting: $ASHLEY_SKILL'"
    after_run: "echo 'Done: $ASHLEY_SKILL (exit $ASHLEY_EXIT_CODE)'"
  skills:
    feat:
      after_run: "make format"
    commit:
      before_run: "make test"
```

Hook points: `before_run`, `after_run`, `on_error`. A non-zero `before_run` aborts the skill run.

---

## Interactive TUI

Run `ash` to launch the hub:

| Feature | Description | Direct CLI |
|---------|-------------|------------|
| **Vibe** | Skill browser — preview, pick a run mode, and launch | `ash vibe` |
| **Sessions** | Manage detached runs | `ash sessions` |
| **History** | Browse invocation log | `ash history browse` |
| **Generate** | Rebuild skill files | `ash generate` |
| **Install** | Deploy skills to your agent's skills dir | `ash install` |
| **Create** | Guided skill builder with preview and JSON editing | `ash create` |
| **Stats** | Usage analytics (top skills, by agent) | `ash history stats` |
| **Settings** | Coding agent, theme & colour | — |

Generate runs in the background and keeps its destination and result visible;
press `R` to regenerate. Install detects supported agent executables on PATH and in native install
locations; Enter installs embedded skills for every detected agent. Press `I`
to set up the agent selected in Settings.

On first launch the TUI runs a quick setup wizard to pick your appearance.
The whole TUI is fully keyboard-operable (Tab, arrows, Enter, Esc) — no mouse
required, so it works over SSH/mosh.

The creator guides you through basics, component/resource selection, workflow,
and preview. Use Tab to change fields, Ctrl+N to advance, Esc to go back, and
Ctrl+S to save. In the workflow step, Ctrl+A adds a step, Ctrl+D removes it,
and Ctrl+Left/Right switches steps. Ctrl+E opens the advanced JSON editor.
New definitions live in `~/.ashley/skills` (or `--root/skills` for a checkout).

Inside **Vibe** you can pick a run mode before launching — **Normal** (standard
permission prompts), **DSP** (skip all permission checks), **AUTO** (auto-accept
edits), or **AFK** (fully autonomous, implies DSP). Press `m` to cycle modes or
click a chip; these map to the same flags as `ash run`.

---

## Coding Agent

Ashley supports Claude Code, Codex, [Grok Build](https://github.com/xai-org/grok-build),
[OpenCode](https://opencode.ai/docs/), and [Kilo Code](https://kilo.ai/docs/code-with-ai/platforms/cli).
All read the same generated `SKILL.md` packages. Claude and Grok use slash
commands, Codex uses `$` mentions, and Ashley asks OpenCode and Kilo to load
the named skill. Kilo installation requires Node.js/npm; its bootstrap uses
`npm install -g @kilocode/cli`.

The shipped executable embeds all built-in skill definitions, components, and
resources. `ash install --all --skills-only` assembles them into
`~/.ashley/generated` and links them into the agents’ user directories, without
a source checkout, network access, or Go/Python tooling. Agent CLI setup may
require its vendor’s network installer.

Installing from `--root` imports complete custom packages from `generated/`,
including supporting files and executable scripts, into `~/.ashley/generated`.
They remain usable without the checkout; later installs preserve local edits.

```bash
ash install --codex        # install skills for Codex
ash install --both         # Claude Code + Codex (backward-compatible)
ash install --grok --opencode --kilo
ash install --all          # all five agents

ash agent                  # show the current default, its version and install source
ash agent codex            # change the default

ash run -o feat "..."      # override for one run (Codex)
ash run -c feat "..."      # override for one run (Claude Code)
ash run --grok feat "..."
ash run --opencode feat "..."
ash pipe --kilo feat+commit "..."  # same flags work for pipelines
```

### Keeping the agent CLIs up to date

Ashley detects how each agent CLI was installed and upgrades it the same way:

```bash
ash upgrade --check --all  # report version + install source, change nothing
ash upgrade                # upgrade the default agent
ash upgrade codex          # upgrade a specific agent
ash upgrade --all          # upgrade all five
```

| Detected install | Upgrade path |
|------------------|--------------|
| Homebrew formula | `brew upgrade <formula>` |
| Homebrew cask | `brew upgrade --cask <cask>` |
| Anything else, already installed | the CLI's configured updater, falling back to its bootstrap script |
| Not installed | the vendor's installer (npm for Kilo) |

Homebrew ownership is detected from Cellar/Caskroom paths. Files elsewhere
under the brew prefix, including npm's `codex.js`, are not treated as formulae.
Native installers are used for Claude, Codex, Grok, and OpenCode; Kilo uses npm.

`ash update` runs this upgrade as its last step, covering whichever agents have
Ashley skills linked. Skip it with `SKIP_TOOL`:

```bash
ash update                 # update Ashley, then upgrade the agent CLIs
SKIP_TOOL=1 ash update     # update Ashley only (also: true / yes)
SKIP_TOOL=1 make update
```

The default is saved to `~/.ashley/prefs.json` and can also be changed from the
TUI **Settings** screen. Run modes map to the available backend controls:

| Ashley mode | Claude Code | OpenAI Codex |
|-------------|-------------|--------------|
| Normal | *(defaults)* | *(defaults)* |
| `-dsp` | `--dangerously-skip-permissions` | `--dangerously-bypass-approvals-and-sandbox` |
| `--auto` | `--permission-mode auto` | `--sandbox workspace-write --ask-for-approval never` |
| `-afk` | DSP + autonomous instructions | DSP + autonomous instructions |

Grok maps `-dsp` to `--always-approve` and `--auto` to `--permission-mode auto`.
OpenCode and Kilo map both modes to `--auto`; explicit deny rules still apply.
For every backend, `-afk` adds autonomous instructions to the DSP mode.
See the [Grok permission guide](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/22-permissions-and-safety.md),
[OpenCode CLI reference](https://opencode.ai/docs/cli/), and
[Kilo CLI reference](https://kilo.ai/docs/code-with-ai/platforms/cli-reference).


| Agent | Skills directory |
|-------|------------------|
| Claude Code | `~/.claude/skills/` (or `$CLAUDE_CONFIG_DIR/skills`) |
| OpenAI Codex | `~/.codex/skills/` (or `$CODEX_HOME/skills`) |
| Grok Build | `~/.grok/skills/` (or `$GROK_HOME/skills`) |
| OpenCode | `~/.config/opencode/skills/` (honours `XDG_CONFIG_HOME` and `OPENCODE_CONFIG_DIR`) |
| Kilo Code | `~/.kilo/skills/` |

---

## Appearance

Ashley's look is configurable from the **Settings** screen in the TUI (or the
first-run wizard). Choose:

- **Mode** — light or dark
- **Colour** — a primary colour (Blue, Green, Purple, Orange, Rose, Cyan) or a
  dual-tone preset (Ocean, Sunset, Grape, Forest)

Changes preview instantly and are saved to `~/.ashley/theme.json`, then
auto-loaded on every launch. The default is **Blue + dark**.

---

## Sessions

Every run launches the coding agent inside a tmux session for crash
resilience. Without
`--detached`, Ashley attaches to it immediately (exiting cleans it up); with
`--detached`, it runs in the background for you to manage later.

Ashley enables mouse scrolling for its sessions: wheel up opens tmux
scrollback instead of sending arrow keys to the agent. Press `q` (or `Esc`
in vi copy mode) to return to typing. Existing sessions receive this fix
when reattached with `ash attach`. Other tmux sessions keep their settings.

```bash
ash run feat "Add OAuth support"            # runs in tmux, attaches immediately
ash run --detached feat "Add OAuth support" # background session
ash sessions               # TUI session manager
ash attach <session-id>    # Attach to interact
ash logs -f <session-id>   # Follow log in real time
ash kill <session-id>      # Kill a session
ash kill all               # Kill all sessions
```

Detaching from a foreground run (`Ctrl-b d`) leaves it running in the
background, just like `--detached`.

**Session manager keys:**

| Key | Action |
|-----|--------|
| Enter | Attach to session |
| c | Copy session ID to clipboard |
| l | View full log |
| s | Cycle sort (newest / skill) |
| K | Kill session |
| X | Kill all running sessions |
| d | Delete record |
| r | Refresh |
| k | Cleanup dead sessions |

---

## Invocation History

Every `ash run` is logged to SQLite.

```bash
ash history show                 # Recent invocations
ash history show --skill feat    # Filter by skill
ash history browse               # Interactive browser (TUI)
ash history stats                # Usage by skill and by agent
ash history stats --agent codex  # Restrict analytics to one agent
ash history prune 30             # Delete entries older than 30 days
ash history info                 # DB location and stats
```

Each invocation records which coding agent ran it. Existing databases are
migrated automatically on the next run — invocations logged before multi-agent
support are counted as Claude Code.

| Platform | Database location |
|----------|-------------------|
| macOS | `~/Library/Application Support/ashley/history.db` |
| Linux | `~/.local/share/ashley/history.db` (respects `XDG_DATA_HOME`) |

---

## CLI Reference

```
ash                              Launch hub TUI
ash vibe                         Skill browser TUI
ash run <skill> [question]       Run a skill
ash run --detached <skill> [q]   Run in background
ash run raw [question]           Run the coding agent without a skill
ash pipe <a+b+c> [question]      Run a skill pipeline
ash generate                     Assemble skill files from JSONC
ash list                         List available skills
ash prompt <skill> [question]    Print prompt to stdout
ash sessions                     Manage detached sessions (TUI)
ash attach <id>                  Attach to a session
ash logs [-f] <id>               View/follow session logs
ash kill <id|all>                Kill sessions
ash history show                 Show invocation history
ash history browse               Interactive history browser
ash history stats [--agent X]    Usage analytics by skill and agent
ash history prune <days>         Delete old entries
ash history clear                Delete all history
ash history info                 Database stats
ash agent [name]                 Show or set the default coding agent
ash upgrade [names] [--all]      Detect + upgrade the agent CLIs (--check to report only)
ash install [--claude|--codex|--grok|--opencode|--kilo|--all]   Generate + install skills
ash uninstall                    Remove skills
ash update [--branch NAME]       Pull latest + reinstall + upgrade agent CLIs
ash --version                    Print version
```

### Run Options

| Flag | Description |
|------|-------------|
| `-dsp` / `--dangerously-skip-permissions` | Skip all permission checks |
| `--auto` | Auto-accept safe tools |
| `--normal` | Use normal permissions, overriding the configured default |
| `-afk` / `--away-from-keyboard` | Fully autonomous, implies `-dsp` |
| `--detached` | Run in background tmux session |
| `-c` / `--claude` | Use Claude Code for this run |
| `-o` / `--codex` | Use OpenAI Codex for this run |
| `--grok` / `--opencode` / `--kilo` | Use the named agent for this run |

---

## Architecture

```
skills/             JSONC skill definitions
components/         Reusable markdown instruction blocks
res/                Code templates and reference docs
cmd/ash/            Native executable entry point
internal/           Go CLI, TUI, generation, sessions, history, and updates
src/ashley/         Python reference implementation (development only)
tests/python/       Python reference and release-tool regression tests
tests/integration/  Compiled binary, package, and installer tests
tests/fixtures/     Reviewed regression reference outputs
tests/reference/    Developer tools for capturing reference fixtures
scripts/release/    Packaging, version validation, and publishing
scripts/dev/        Local development helpers
docs/migration/     Migration acceptance and release parity checklist
```

`assets.go` stays at the module root so Go can embed the shared skill sources
directly, without a generated copy. Build outputs (`build/`, `dist/`, and
`generated/`) and test caches are ignored; `make clean` removes them.

Skills are JSONC files referencing reusable components and code resources. The generator assembles them into self-contained markdown prompts with all resources inlined. Project detection provides tech stack context to Jinja2 templates for conditional content.

---

## Development

```bash
git clone https://github.com/LBYPatrick/ashley.git
cd ashley
uv sync --group dev
make generate       # Regenerate skills
make test           # Run tests
make format         # Format Go, Python reference tests, and shell scripts
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make help` | Show all targets |
| `make install` | Build native CLI and install skills (`AGENT=claude\|codex\|grok\|opencode\|kilo\|both\|all`) |
| `make uninstall` | Remove skills and CLI |
| `make generate` | Regenerate skill markdown files |
| `make list` | List skill definitions |
| `make format` | Run Go, Python, and shell formatters |
| `make test` | Python reference tests, Go race/coverage tests, parity, and binary integration |
| `make clean` | Remove build outputs, release archives, generated skills, and test caches |
| `make update` | Pull latest + reinstall + upgrade agent CLIs (`SKIP_TOOL=1` to skip) |
| `make upgrade` | Detect + upgrade the agent CLIs (`AGENT=<agent key>`) |

---

## License

[MIT](LICENSE)
