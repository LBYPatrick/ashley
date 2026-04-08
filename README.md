<h1 align="center">Ashley</h1>

<p align="center">
  <strong>Interactive skill set framework for <a href="https://docs.anthropic.com/en/docs/claude-code">Claude Code</a></strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/python-≥3.13-3776AB?logo=python&logoColor=white" alt="Python" />
  <img src="https://img.shields.io/badge/version-0.1.0-blue" alt="Version" />
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="License" /></a>
</p>

---

Ashley provides 14 composable, production-ready skills that encode software engineering best practices as structured prompts for Claude Code. Skills are assembled from reusable components and inlined resources, then installed as slash commands (`/a-feat`, `/a-refactor`, etc.).

- **14 specialized skills** — feat, refactor, debug, optimize, commit, scaffold, and more
- **Skill pipelines** — chain skills with `+` syntax (`feat+commit+changelog`) or named pipelines
- **Lifecycle hooks** — run shell commands before/after any skill execution
- **Project detection** — auto-detects tech stack for context-aware prompts
- **Interactive TUI** — hub with skill browser, session manager, and history viewer
- **Detached sessions** — background runs with tmux, log capture, and live status
- **Invocation history** — every run logged to SQLite for search and review

---

## Quick Start

### Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| Python | ≥ 3.13 | |
| [uv](https://docs.astral.sh/uv/) | latest | Python package manager |
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | latest | For running skills |
| [tmux](https://github.com/tmux/tmux) | latest | Optional — detached sessions only |

### One-Line Install

```bash
curl -fsSL https://raw.githubusercontent.com/LBYPatrick/ashley/main/scripts/remote-install.sh | bash
```

Clones to `~/.ashley/repo`, installs dependencies, generates skills, symlinks to `~/.claude/skills/`, and adds `ash` to `~/.local/bin`.

<details>
<summary>Manual install</summary>

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
| `ASHLEY_DIR` | `~/.ashley/repo` | Install location |
| `ASHLEY_REPO_URL` | `https://github.com/LBYPatrick/ashley.git` | Override repo URL |
| `ASHLEY_USE_CN` | unset | Use China-accessible mirrors (`1` to enable) |
| `ASHLEY_NO_COLOR` | unset | Disable colored output (`1` to enable) |

</details>

### Uninstall

```bash
cd ~/.ashley/repo && make uninstall
rm -rf ~/.ashley
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
| **Vibe** | Skill browser — preview and launch | `ash vibe` |
| **Sessions** | Manage detached runs | `ash sessions` |
| **History** | Browse invocation log | `ash history browse` |
| **Generate** | Rebuild skill files | `ash generate` |
| **Install** | Deploy skills to ~/.claude/skills/ | `ash install` |

---

## Detached Sessions

Run skills in the background and manage them later:

```bash
ash run --detached feat "Add OAuth support"
ash sessions               # TUI session manager
ash attach <session-id>    # Attach to interact
ash logs -f <session-id>   # Follow log in real time
ash kill <session-id>      # Kill a session
ash kill all               # Kill all sessions
```

| Key | Action |
|-----|--------|
| Enter | Attach to session |
| l | View full log |
| k | Kill session |
| d | Delete record |
| r | Refresh |
| c | Cleanup dead sessions |

---

## Invocation History

Every `ash run` is logged to SQLite.

```bash
ash history show                 # Recent invocations
ash history show --skill feat    # Filter by skill
ash history browse               # Interactive browser (TUI)
ash history prune 30             # Delete entries older than 30 days
ash history info                 # DB location and stats
```

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
ash run raw [question]           Run Claude Code without a skill
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
ash history prune <days>         Delete old entries
ash history clear                Delete all history
ash history info                 Database stats
ash install                      Generate + install skills
ash uninstall                    Remove skills
ash update [--branch NAME]       Pull latest + reinstall
ash --version                    Print version
```

### Run Options

| Flag | Description |
|------|-------------|
| `-dsp` / `--dangerously-skip-permissions` | Skip all permission checks |
| `--auto` | Auto-accept safe tools |
| `-afk` / `--away-from-keyboard` | Fully autonomous, implies `-dsp` |
| `--detached` | Run in background tmux session |

---

## Architecture

```
skills/          JSONC skill definitions (name, components, resources, workflow)
components/      Reusable markdown instruction blocks
res/             Code templates and reference docs
src/ashley/      Python package (generator, CLI, TUI, hooks, pipelines)
generated/       Output: assembled SKILL.md files (always inlined)
```

Skills are JSONC files referencing reusable components and code resources. The generator assembles them into self-contained markdown prompts with all resources inlined. Project detection provides tech stack context to Jinja2 templates for conditional content.

---

## Development

```bash
git clone https://github.com/LBYPatrick/ashley.git
cd ashley
uv sync --group dev
make generate       # Regenerate skills
make test           # Run tests
make format         # Run ruff formatter
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make help` | Show all targets |
| `make install` | Generate, install skills, symlink CLI |
| `make uninstall` | Remove skills and CLI |
| `make generate` | Regenerate skill markdown files |
| `make list` | List skill definitions |
| `make format` | Run ruff formatter |
| `make test` | Run pytest |
| `make clean` | Remove generated files |
| `make update` | Pull latest + reinstall |

---

## License

[MIT](LICENSE)
