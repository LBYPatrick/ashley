<h1 align="center">Ashley</h1>

<p align="center">
  <strong>Interactive skill set framework for <a href="https://docs.anthropic.com/en/docs/claude-code">Claude Code</a>, <a href="https://developers.openai.com/codex">OpenAI Codex</a>, Grok Build, OpenCode, and Kilo Code</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-native_binary-00ADD8?logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/version-1.3.0-blue" alt="Version" />
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
- **Zellij-backed sessions** — every run is crash-resilient; detach to background with `--detached`
- **Invocation history** — every run logged to SQLite for search and review

---

## Native binary

Ashley ships as one Go executable for macOS and Linux on arm64 and amd64.
Skills, components, and resources are embedded: users need no Go, Python, uv,
or source checkout. Coding-agent CLIs and Zellij are installed automatically during full setup.
The project stays open source; development and release tooling use Go.

Existing YAML settings, JSON preferences, SQLite history, and tmux sessions
remain compatible. See [Migrating from Python](#migrating-from-python) for the
launcher replacement and custom-skill import procedure. Development and release
commands are documented in [binary release development](docs/releases.md).

## Quick Start

### Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| macOS or Linux | arm64 or amd64 | A matching prebuilt release; no language runtime needed |
| Claude Code, Codex, Grok Build, OpenCode, or Kilo Code | latest | For running skills — installed for you |
| [Zellij](https://zellij.dev) | 0.45+ | Detected automatically; a verified release is installed when missing |

### One-Line Install

```bash
curl -fsSL https://raw.githubusercontent.com/LBYPatrick/ashley/main/scripts/remote-install.sh | bash
```

Downloads and verifies the matching release binary into `~/.local/bin/ash`,
then installs Zellij, skills, and the selected agent. No checkout, Python, uv, or Go
toolchain is installed. Native release assets are available starting with v0.4.0.

The optional [skills.sh CLI](https://skills.sh/docs/cli) is **not installed by
default**. To install it and any missing runtime dependencies automatically:

```bash
curl -fsSL https://raw.githubusercontent.com/LBYPatrick/ashley/main/scripts/remote-install.sh | ASHLEY_INSTALL_SKILLS=1 bash
```

This opt-in also applies to `scripts/install.sh`, including binary-only installs.

When Skills is already available or you opted into installing it, agent setup
also asks whether to install all of [Emil Kowalski's skills](https://github.com/emilkowalski/skills)
plus `find-skills` from `vercel-labs/skills`. Accepting installs the bundle globally
for the same selected Ashley agents, without further skill or agent pickers.
Declining leaves the community bundle uninstalled. This question is also offered
by `ash install`; binary-only installation does not select agents or offer bundles.
Without a terminal, the optional bundle is skipped unless configured below.

The installer asks which coding agent to set up. Skip the question with a flag:

```bash
curl -fsSL .../remote-install.sh | bash -s -- --codex   # or --claude, --grok, --opencode, --kilo, --all
```

### Migrating from Python

The normal remote installer automatically migrates Python installations; no separate migration command is needed:

```bash
curl -fsSL https://raw.githubusercontent.com/LBYPatrick/ashley/main/scripts/remote-install.sh \
  | bash -s -- --version 1.1.0
```

It finds the old checkout through launcher symlinks or `~/.ashley/repo`
(`ASHLEY_DIR` is also supported), verifies the native release, and imports skill
definitions, components, resources, and generated packages into `~/.ashley`.
Existing user files win over imported files. Imported files remain local
overrides of the embedded defaults, preserving customizations.

Before replacing the launcher, migration backs it up and moves existing agent
skill links off the checkout using the native executable. This preservation step
runs without Python, uv, dependency installation, or prompts. The normal remote
installer then performs the requested agent setup. Use `--skills-only --codex`
to skip agent CLI installation, or `--binary-only` to skip subsequent setup
while still preserving an existing skill installation.

The installer also backs up and redirects recognized Python or Go wrappers
that take precedence on PATH. An unwritable shadowing launcher produces an
error before replacement and instructions to put the install directory first
on PATH. Unrelated executables (including a system shell named `ash`) are never
redirected. `--install-dir DIR` selects a custom destination.

Backups live under `~/.ashley/migrations/python-to-go-*`. Settings, SQLite
history, session logs, the old checkout/virtualenv, and shared Python/uv
installations are retained. If verification or migration skill setup fails,
the launcher stays unchanged; any imported files and backups remain available
for inspection. Symlinked skill data is rejected rather than copied through.
Subsequent native installs do not repeat migration.

For offline or explicit-source recovery, the standalone compatibility helper
is still available:

```bash
bash scripts/migrate-python.sh --binary /path/to/ash --source ~/code/ashley
```

Start a new shell (or run `hash -r`), then check `ash --version`, `ash history show`,
and `ash list`. After verifying your custom skills and old history, you can
remove the old checkout and its `.venv`; the Go installation no longer needs them.
Do not run the old checkout's `make uninstall`, which would remove the new links.
To restore the previous launcher, retain its old checkout, remove the new
launcher, and copy the saved `ash` back with `cp -Pp BACKUP/ash ~/.local/bin/ash`.

<details>
<summary>Build from source (developers)</summary>

```bash
git clone https://github.com/LBYPatrick/ashley.git
cd ashley
make build           # Standalone executable: build/ash-go
./build/ash-go --version
make install         # Optional: install the CLI and skills
```

Requires the Go version specified in `go.mod`, Make, and Bash. Dependencies download
on the first build; all skill assets are embedded automatically. No Python or uv
is needed. Cross-compile with Go's target variables:

```bash
make build GOOS=linux GOARCH=arm64 BUILD_OUTPUT=build/ash-linux-arm64
make build GOOS=windows GOARCH=amd64  # build/ash-go.exe
```

Without Make, `go build -o ash ./cmd/ash` builds directly from the repository root
(use `-o ash.exe` on Windows). Full agent, hook, and Zellij workflows are supported
on macOS and Linux; use WSL for those workflows on Windows. A successful Windows
build does not imply native Windows support for Unix tools.

</details>

<details>
<summary>Environment variables</summary>

| Variable | Default | Description |
|----------|---------|-------------|
| `ASHLEY_INSTALL_DIR` | `~/.local/bin` | Binary installation directory |
| `ASHLEY_REPO` | `LBYPatrick/ashley` | GitHub release repository |
| `ASHLEY_VERSION` | latest stable | Specific binary release version |
| `ASHLEY_AUTOMATED_CONFIG` | unset | Path to a local automated installation JSON profile |
| `ASHLEY_AUTOMATED` | unset | `1`, `true`, or `yes`: activate the profile and disable installation prompts |
| `ASHLEY_INSTALL_SKILLS` | unset | `1`, `true`, or `yes`: install skills.sh dependencies without prompting during installation or `ash skills` |
| `ASHLEY_NO_COLOR` | unset | Disable colored output (`1` to enable) |
| `ASHLEY_AGENT` | unset | Preselect the agent (`claude`, `codex`, `grok`, `opencode`, `kilo`, `both`, or `all`) |

</details>

### Unattended installation

Copy [ashley-automated.example.json](ashley-automated.example.json) to a local
`ashley-automated.json`, then edit your choices:

```json
{
  "agents": ["claude", "codex"],
  "skills_only": false,
  "install_skills": true,
  "community_skills": true
}
```

```bash
curl -fsSL https://raw.githubusercontent.com/LBYPatrick/ashley/main/scripts/remote-install.sh | \
  ASHLEY_AUTOMATED_CONFIG="$PWD/ashley-automated.json" ASHLEY_AUTOMATED=1 bash
```

Both variables are required to activate the profile. The path refers to an existing
local file (a mounted file works too); the filename itself is unrestricted. Supplying
a path without enabling `ASHLEY_AUTOMATED` leaves normal interactive setup active.
The same variables work with `ash install` and `scripts/install.sh`.

| JSON field | Default | Meaning |
|------------|---------|---------|
| `agents` | required | Nonempty list of `claude`, `codex`, `grok`, `opencode`, or `kilo` |
| `skills_only` | `false` | Skip installing coding-agent CLIs; still install Ashley skills |
| `install_skills` | `false` | Authorize automatic Skills CLI and runtime dependency installation |
| `community_skills` | `false` | Install every Emil skill plus `find-skills` for the selected agents |

The profile overrides command-line agent/skills-only selections and
`ASHLEY_INSTALL_SKILLS`. `community_skills: true` can reuse an existing Skills
installation with `install_skills: false`; missing dependencies then cause an error.
Unknown fields, duplicate/unknown agents, invalid JSON, and missing configuration
fail without prompting. The downloaded binary validates the profile before replacing
the existing launcher. Automated mode rejects `--binary-only` because the profile
specifies agent setup.

Silent mode means **no interactive input or selection prompts**: progress and errors
remain visible, and the installation log includes community setup output. External
commands use noninteractive settings, and bundle installation passes explicit agent
and skill selections plus `--yes`. Failures return a nonzero status; no interactive
fallback is attempted. Agent account authentication remains a separate step.

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

# Give the session a persistent, searchable name
ash run -n "Login API" debug "Fix the 500 error on /api/users"

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

## skills.sh

On Linux and macOS, `ash skills` forwards all arguments, input, output, and exit
status to the native [Skills CLI](https://skills.sh/docs/cli):

```bash
ash skills --help
ash skills find
ash skills add vercel-labs/agent-skills
ash skills list
ash skills remove
ash skills update
```

Ashley detects pnpm/npm, Node.js, and the `skills` executable. If dependencies
are missing, it displays an installation plan and asks for confirmation. Declining
or reaching end-of-input cancels setup. For unattended use:

```bash
ASHLEY_INSTALL_SKILLS=1 ash skills --version
```

Setup uses an existing pnpm or npm installation; if neither exists, it installs
standalone pnpm. Missing or unsupported Node.js is installed as LTS through pnpm
(bootstrapping pnpm if needed). Skills is installed with `pnpm add --global skills`
or `npm install --global --prefix ~/.ashley/tools skills`. No sudo is needed.
Ashley refreshes PATH and verifies dependencies in the same invocation, including
both older pnpm layouts and pnpm 12's `PNPM_HOME/bin`, so no terminal restart is
needed. `PNPM_HOME` is respected; otherwise it uses `~/Library/pnpm` on macOS or
`${XDG_DATA_HOME:-~/.local/share}/pnpm` on Linux. The pnpm installer may also update
your shell configuration.

Every argument after `ash skills`, including `--help` and `--yes`, belongs to the
Skills CLI; use the environment variable above to approve Ashley's dependency
setup. This integration requires Bash and curl for bootstrap; use WSL on Windows.
Skills installed through this command are managed by the upstream CLI. Ashley's
built-in catalog remains available through `ash list` and `ash install`.

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

Run `ash` to open a workspace for starting and continuing agent conversations.
Home prioritizes new work, active sessions, and recent conversations. Select a
recent task to resume it directly. First launch uses the default agent and theme;
appearance setup never blocks starting work.

| Feature | Description | Direct CLI |
|---------|-------------|------------|
| **New run** | Multiline task composer with optional skill, agent, and permission controls | Ctrl+2 |
| **Library** | Browse, create, sync, and manage community skills | Ctrl+4; `ash vibe` opens the skill chooser |
| **Activity** | Sessions and History, with task titles, logs, and conversation resume | Ctrl+3; `ash sessions` or `ash history browse` |
| **Sync** | Generate skills, then install for all detected agents | — |
| **Create** | Guided skill builder with preview and JSON editing | `ash create` |
| **Stats** | Usage analytics (top skills, by agent) | `ash history stats` |
| **Settings** | Default coding agent, theme & colour | Ctrl+5 |

The persistent navigation is **Ctrl+1 Home · Ctrl+2 New run · Ctrl+3 Activity · Ctrl+4 Library ·
Ctrl+5 Settings** (`^1`–`^5` in the tab bar). The TUI enables enhanced keyboard
reporting for terminals that support it. On older terminals that cannot distinguish
Ctrl+number combinations, tap a tab or use `Ctrl+P`. `?` opens shortcut help outside text fields.

In **New run**, write a multiline task. `Ctrl+S` opens the searchable skill
chooser; Enter selects a skill and returns to your draft. `Ctrl+D` removes the
skill for a direct conversation. Tab moves between the task, agent, permissions,
and Start; Enter changes an option, while Enter in the task inserts a newline.
`Ctrl+R` launches using the options shown. Drafts and browser positions survive
workspace navigation for the lifetime of the TUI.

`Ctrl+N` names a draft; `Shift+N` renames a selected session or history entry.
The CLI accepts `-n NAME` or `--name NAME`. Names are saved in the local SQLite
history database and session metadata, appear in lists, and are searchable in
History. An empty name restores the task title; IDs and resume links stay stable.

The interface uses aligned columns, shared section rules, and distinct text,
metadata, and accent tiers.

In **Activity**, `T` or the visible tabs switch between Sessions and History.
Enter attaches a running session, opens a finished session's log, or resumes a
history conversation. `I` opens the technical inspector. Narrow terminals use
a single list; `V` opens details and Esc returns to the list. Stop and delete
actions require confirmation, with Cancel selected by default.

Open **Library** from Home or the `Ctrl+P` command palette. Select an action
with arrows and Enter; Find and Add accept a search term or repository/URL.
Add, List, and Remove operate on globally installed skills. Native Skills prompts
handle skill/agent selection and missing-dependency consent. Command output remains
visible until you press Enter to return to Ashley. Community bundle setup reuses
Ashley’s agent selection and asks before adding Emil’s skills and `find-skills`.
Opening the page does not install anything.

Sync detects supported agent executables on PATH and in native install locations,
then generates skills into `~/.ashley/generated` before installing links for every
detected agent. Every activation runs the complete sync again. The full per-file and per-agent
log stays visible; use PgUp/PgDn or Home/End to scroll, and `R` to rerun.
Press `I` to set up the agent selected in Settings. Standalone `ash generate` and
`ash install` commands remain available for scripts and explicit CLI use.

The whole TUI is fully keyboard-operable (Tab, arrows, Enter, Esc) — no mouse
required, so it works over SSH/mosh.

The creator guides you through basics, component/resource selection, workflow,
and preview. Use Tab to change fields, Ctrl+N to advance, Esc to go back, and
Ctrl+S to save. In the workflow step, Ctrl+A adds a step, Ctrl+D removes it,
and Ctrl+Left/Right switches steps. Ctrl+E opens the advanced JSON editor.
New definitions live in `~/.ashley/skills` (or `--root/skills` for a checkout).

The composer's permission choices map to the existing CLI modes: **Ask first**
(Normal), **Full access** (DSP), **Auto edits** (AUTO), and **Autonomous** (AFK).
The selected agent and permissions are passed explicitly for each launch.

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
They remain usable without the checkout. Every install regenerates prompts and
replaces local edits and conflicting paths for the skills being installed. Edit
source definitions under `~/.ashley/skills` and `~/.ashley/components` to maintain
custom behavior; generated `SKILL.md` files are disposable outputs.

CLI output groups setup and sync into readable sections. Each installation saves
a complete log under `~/.ashley/logs/`; use `ash install --verbose` to also stream
per-file details. TUI Sync continues to display its full log. Human-facing tables
adapt to terminal width, and redirected output has no ANSI styling. `NO_COLOR`
or `ASHLEY_NO_COLOR` disables terminal colors.

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

`ash update` refreshes Ashley skills without offering the optional community bundle
or replaying automated installation profiles. Use `ash install` or the TUI
Skills.sh page to add that bundle explicitly.

The default is saved to `~/.ashley/prefs.json` and can also be changed from the
TUI **Settings** screen. Run modes map to the available backend controls:

| Ashley mode | Claude Code | OpenAI Codex |
|-------------|-------------|--------------|
| Normal | *(defaults)* | *(defaults)* |
| `-dsp` | `--dangerously-skip-permissions` | `--dangerously-bypass-approvals-and-sandbox --sandbox danger-full-access` |
| `--auto` | `--permission-mode auto` | `--sandbox workspace-write --ask-for-approval never` |
| `-afk` | DSP + autonomous instructions | DSP + autonomous instructions |

Grok maps `-dsp` to `--always-approve` and `--auto` to `--permission-mode auto`.
OpenCode and Kilo map both modes to `--auto`; explicit deny rules still apply.
For every backend, `-afk` adds autonomous instructions to the DSP mode.
`--leon` is an alias for AFK. Codex DSP and AFK explicitly select Full Access
with `--sandbox danger-full-access` alongside the YOLO flag. These are per-run
options; Ashley does not change the user's persistent Codex configuration.
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

Ashley's look is configurable from the **Settings** screen in the TUI. Choose:

- **Mode** — Auto (default), Dark, or Light
- **Colour** — a primary colour (Blue, Green, Purple, Orange, Rose, Cyan) or a
  dual-tone preset (Ocean, Sunset, Grape, Forest)

Changes preview instantly and are saved to `~/.ashley/theme.json`, then
auto-loaded on every launch. The default is **Blue + Auto**. Auto follows terminal
brightness and uses Dark when brightness cannot be determined. Saved Clear
preferences load as Auto; existing Dark and Light selections are preserved.

---

## Remote terminals (SSH and Mosh)

Run Ashley on the remote host; sessions and history stay on that host. Reconnect
and use `ash attach ID` or Activity to continue the same agent. SSH disconnects
and terminal resizes do not require restarting the agent. Mosh itself provides
connection recovery; Zellij keeps the work alive after the Mosh client exits.

- Ctrl+1–5 uses enhanced keyboard reporting where supported. Legacy terminals
  and Mosh may not distinguish these combinations: all five tabs remain tappable
  on narrow screens, and Ctrl+P offers the same navigation without function keys.
- Remote copies target the client clipboard via OSC 52 (explicit `c` selector).
  Enable clipboard writes in your terminal; use Mosh 1.4+ on the server. Ashley
  reports that it sent a request because the terminal does not acknowledge writes.
  Remote copies are limited to 64 KiB; save larger prompts with `ash prompt`.
- Auto appearance falls back to Dark when a terminal cannot report brightness.
  Choose Light or Dark explicitly in Settings if the remote terminal proxy reports
  a different background. No animation or special fonts are required.
- Multiline paste, Unicode text, mouse scrolling, resize, detach/reattach, and
  agent logs are covered by PTY integration tests. Loopback SSH and Mosh tests
  exercise their actual transports when those programs are installed.
- `ash sessions --json`, `ash history show --json`, `ash logs`, and
  `ash prompt` remain available for scripts and non-interactive connections.

Protocol references: [Zellij compatibility](https://zellij.dev/documentation/compatibility.html),
[Zellij session controls](https://zellij.dev/documentation/programmatic-control.html),
and [enhanced keyboard reporting](https://sw.kovidgoyal.net/kitty/keyboard-protocol/).

## Sessions

New runs use Zellij with a private Ashley configuration and a single agent pane.
Ashley detects Zellij 0.45 or newer during full installation and before a run.
If missing or outdated, it installs the official 0.45.1 binary to
`~/.local/bin/zellij`, verifies its pinned SHA-256 checksum, and validates its
version. No root privileges or language toolchain are required. Skills-only and
binary-only installs defer session setup until the first run.

Without `--detached`, Ashley attaches immediately; detached runs continue in
the background. Closing an SSH/Mosh connection detaches the client. Agent output
is recorded by Ashley's PTY supervisor before the agent starts, so logs and
completion hooks keep working while detached. Finished-session logs are retained.

Existing tmux sessions remain attachable and manageable until they finish; new
runs use Zellij. Your global tmux and Zellij configuration files are not replaced.

Zellij starts in locked mode so agent shortcuts pass through. Press **Ctrl+B,
then D** to detach; **Ctrl+B, then S** enters scrollback. In scrollback use arrows,
PageUp/PageDown, Home/End, and Q or Esc to return to typing. Ctrl+B then B sends a
literal Ctrl+B. Mouse selection copies through OSC 52, and wheel events reach
mouse-aware agents. Hold the terminal's mouse override modifier (often Shift) for
native selection. Pasting uses the terminal's normal paste action.

```bash
ash run feat "Add OAuth support"            # runs in Zellij, attaches immediately
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
| Enter | Attach to a running session or open a finished session's log |
| t | Switch to History |
| i | Toggle the technical inspector |
| c | Copy session ID to clipboard |
| l | View full log |
| s | Cycle sort (newest / skill) |
| K | Kill session |
| X | Kill all running sessions |
| d | Delete record |
| r | Refresh |
| Ctrl+P → Clean finished sessions | Confirm removal of finished session records and logs |

---

## Invocation History

Every `ash run` is logged to SQLite.

```bash
ash history show                 # Recent invocations
ash history show --skill feat    # Filter by skill
ash history browse               # Interactive browser (TUI)
ash history resume 42            # Reattach or resume this invocation's conversation
ash history stats                # Usage by skill and by agent
ash history stats --agent codex  # Restrict analytics to one agent
ash history prune 30             # Delete entries older than 30 days
ash history info                 # DB location and stats
```

Each invocation records which coding agent ran it. Existing databases are
migrated automatically on the next run — invocations logged before multi-agent
support are counted as Claude Code.

Press **Enter** on a history entry to reattach its running session, or resume
its recorded agent conversation in a new Zellij session. Resuming preserves
the original working directory, agent, and permission mode, without replaying
the original prompt. Each new launch gets its own history entry linked to
the same conversation. The details panel and `ash history show --json` expose
the conversation ID separately from the multiplexer session ID.

| Agent | Conversation tracking | Native resume command |
|--------|-----------------------|-----------------------|
| Claude Code | Explicit UUID at launch | `claude --resume ID` |
| Codex | SessionStart / Stop hooks | `codex resume ID` |
| Grok | Explicit UUID at launch | `grok --resume ID` |
| OpenCode | Temporary `session.created` plugin | `opencode --session ID` |
| Kilo | Temporary `session.created` plugin | `kilo --session ID` |

Codex tracking requires a recent CLI with lifecycle hooks and `--no-daemon`.
Trust Ashley's callbacks in Codex's `/hooks` interface; the Stop callback
can record the first conversation after trust is granted and a response
finishes. Ashley does not bypass hook trust or edit agent configuration files.
OpenCode and Kilo receive a temporary plugin through their inline configuration.
The agent's native conversation storage must still exist to resume it.
Older entries without a conversation ID can reattach a live tmux session,
but cannot automatically resume after it exits. Ashley never guesses a
conversation from the most recent session in a directory.

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
ash skills [commands/options]    Forward native skills.sh commands
ash prompt <skill> [question]    Print prompt to stdout
ash sessions                     Manage detached sessions (TUI)
ash attach <id>                  Attach to a session
ash logs [-f] <id>               View/follow session logs
ash kill <id|all>                Kill sessions
ash history show                 Show invocation history
ash history browse               Interactive history browser
ash history resume <id>          Reattach or resume an agent conversation
ash history stats [--agent X]    Usage analytics by skill and agent
ash history prune <days>         Delete old entries
ash history clear                Delete all history
ash history info                 Database stats
ash agent [name]                 Show or set the default coding agent
ash upgrade [names] [--all]      Detect + upgrade the agent CLIs (--check to report only)
ash install [--claude|--codex|--grok|--opencode|--kilo|--all]   Generate + install skills
ash uninstall                    Remove skills
ash update [--version VERSION]  Install verified binary release + refresh skills
ash --version                    Print version
```

### Run Options

| Flag | Description |
|------|-------------|
| `-dsp` / `--dangerously-skip-permissions` | Skip all permission checks |
| `--auto` | Auto-accept safe tools |
| `--normal` | Use normal permissions, overriding the configured default |
| `-afk` / `--away-from-keyboard` | Fully autonomous, implies `-dsp` |
| `--detached` | Run in background Zellij session |
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
tests/integration/  Go binary, terminal, package, installer, and migration tests
tests/fixtures/     Reviewed regression reference outputs
scripts/release/    Packaging, version validation, and publishing
scripts/dev/        Local development helpers
docs/migration/     Migration acceptance and release parity checklist
```

`assets.go` stays at the module root so Go can embed the shared skill sources
directly, without a generated copy. Build outputs (`build/`, `dist/`, and
`generated/`) and test caches are ignored; `make clean` removes them.

For new projects without an explicit stack, coding guidance selects Rust for
edge workloads requiring extreme performance, Python for ML/data analytics when
lower runtime performance is acceptable, and Go otherwise. Web frontends default
to Vue + TypeScript + Vite. Explicit choices and existing stacks take precedence;
both Vue and React component references remain bundled.

Coding guidance prefers Go for new standalone test scripts and one-shot
automation while retaining each project's native unit-test framework. Backend
servers use Domain Driven Design, organizing files by bounded context with
domain, application, and adapter boundaries, including in Go. Go style follows
[Google's guide](https://google.github.io/styleguide/go/); formatter guidance
covers `gofmt`, optional `goimports`, and `go vet` alongside the Python and
TypeScript tooling.

Skills are JSONC files referencing reusable components and code resources. The generator assembles them into self-contained markdown prompts with all resources inlined. Project detection provides tech stack context to Jinja2 templates for conditional content.

---

## Development

```bash
git clone https://github.com/LBYPatrick/ashley.git
cd ashley
make build          # Build the standalone executable
make generate       # Regenerate skills
make test           # Run tests
make format         # Format Go and check shell syntax
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make help` | Show all targets |
| `make build` | Build `build/ash-go`; supports `GOOS`, `GOARCH`, and `BUILD_OUTPUT` |
| `make install` | Build native CLI and install skills (`AGENT=claude\|codex\|grok\|opencode\|kilo\|both\|all`) |
| `make uninstall` | Remove skills and CLI |
| `make generate` | Regenerate skill markdown files |
| `make list` | List skill definitions |
| `make format` | Format Go and check shell syntax |
| `make test` | Go unit/release tests, race/coverage/vet, and binary/terminal integration |
| `make clean` | Remove build outputs, release archives, generated skills, and test caches |
| `make update` | Update an explicit developer checkout |
| `make upgrade` | Detect + upgrade the agent CLIs (`AGENT=<agent key>`) |

---

## License

[MIT](LICENSE)
