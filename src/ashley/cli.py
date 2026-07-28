"""Ashley CLI — Interactive skill set framework for coding agents."""

import os
import shutil
import subprocess
import sys

import click

import ashley
from ashley.agents import (
    AGENT_KEYS,
    get_agent,
    permission_args,
    select_agent,
    skill_trigger,
    skills_dir,
)
from ashley.generate import generate as do_generate
from ashley.generate import list_skills
from ashley.install import install as do_install
from ashley.install import uninstall as do_uninstall
from ashley.prompt import generate_prompt
from ashley.update import update as do_update


@click.group(invoke_without_command=True)
@click.option("-i", "--interactive", is_flag=True, help="Launch interactive TUI")
@click.option("--version", is_flag=True, help="Print version")
@click.pass_context
def main(ctx, interactive, version):
    """Ashley — Interactive skill set framework for Claude Code."""
    if version:
        click.echo(f"ashley {ashley.__version__}")
        return

    if ctx.invoked_subcommand is None or interactive:
        from ashley.tui.app import AshleyApp

        app = AshleyApp()
        app.run()


@main.command()
def vibe():
    """Launch the skill browser TUI."""
    from textual.app import App

    from ashley.tui.app import VibeScreen
    from ashley.tui.theme import BASE_CSS, apply_theme

    class VibeApp(App):
        TITLE = f"Ashley v{ashley.__version__}"
        SUB_TITLE = "Vibe — Skill Browser"

        CSS = BASE_CSS

        def on_mount(self):
            apply_theme(self)
            self.push_screen(VibeScreen())

    app = VibeApp()
    app.run()


@main.command()
def generate():
    """Generate skill markdown files from JSONC definitions."""
    do_generate()


@main.command("list")
def list_cmd():
    """List all available skill definitions."""
    list_skills()


@main.command()
@click.argument("skill")
@click.argument("question", nargs=-1)
def prompt(skill, question):
    """Print a copy-pasteable prompt to stdout."""
    question_str = " ".join(question) if question else ""
    result = generate_prompt(skill, question_str)
    click.echo(result)


AFK_ADDENDUM = """\
## AFK Mode — Autonomous Operation

The user is away from the keyboard. You MUST operate fully autonomously:
- NEVER ask the user for clarification, confirmation, or input — make your best judgement call and proceed.
- ALWAYS choose 'yes' / 'proceed' / 'continue' when faced with any prompt or decision point.
- If a tool requires user approval, assume approval is granted.
- If you encounter ambiguity, pick the most reasonable option and move forward.
- Complete the entire task end-to-end without stopping."""

# Marker consumed by ashley.sessions: the arg is replaced in place with a
# shell "$(cat …)" expansion so huge prompts never enter tmux's arg buffer.
PROMPT_FILE_MARKER = "__ASHLEY_PROMPT_FILE__="

# Prompts longer than this are spilled to a temp file in detached mode.
ARG_TEXT_LIMIT = 4000


def _resolve_agent_flags(use_claude: bool, use_codex: bool) -> str | None:
    """Resolve the per-run ``-c``/``-o`` flags, exiting on a conflict."""
    try:
        return select_agent(use_claude, use_codex)
    except ValueError as exc:
        click.echo(f"Error: {exc}", err=True)
        sys.exit(1)


def _skill_is_installed(skill: str, agent=None) -> bool:
    """Check if a skill is installed in the agent's skills directory."""
    directory = skills_dir(agent)
    # Check both "a-<skill>" and "<skill>" forms
    for name in (f"a-{skill}", skill):
        candidate = directory / name
        if candidate.is_dir() and (candidate / "SKILL.md").is_file():
            return True
    return False


def _text_arg(text: str, detached: bool) -> str:
    """Return *text* as a CLI argument, spilling large text to a temp file.

    tmux chokes on very long arguments, so detached runs get a
    :data:`PROMPT_FILE_MARKER` that :mod:`ashley.sessions` expands at
    launch time instead of the literal text.
    """
    if not detached or len(text) <= ARG_TEXT_LIMIT:
        return text

    import tempfile

    prompt_file = tempfile.NamedTemporaryFile(
        mode="w",
        prefix="ashley-prompt-",
        suffix=".md",
        delete=False,
    )
    prompt_file.write(text)
    prompt_file.close()
    return f"{PROMPT_FILE_MARKER}{prompt_file.name}"


def build_agent_invocation(
    skill: str,
    question_str: str,
    dangerously_skip_permissions: bool,
    auto_mode: bool,
    away_from_keyboard: bool,
    detached: bool = False,
    agent: str | None = None,
) -> tuple[list[str], str]:
    """Build the coding-agent CLI arguments for a skill run.

    When the skill is installed in the agent's skills directory we let the
    agent load it natively (``/a-feat`` for Claude Code, ``$a-feat`` for
    Codex) instead of inlining the whole prompt. When it is not installed,
    the generated prompt is passed as extra system instructions — via
    ``--append-system-prompt`` for agents that support it, or prepended to
    the user prompt for those that don't.

    Args:
        skill: Skill stem, or ``"raw"`` to launch the agent with no skill.
        question_str: The user's question, possibly empty.
        dangerously_skip_permissions: Skip all permission checks.
        auto_mode: Auto-accept edits.
        away_from_keyboard: Fully autonomous run (implies DSP).
        detached: Whether the run will be handed to tmux, which limits how
            much text may travel in a single argument.
        agent: Backend key; falls back to the saved preference.

    Returns:
        An ``(argv, permission_mode)`` pair.
    """
    from ashley.config import load_agent

    spec = get_agent(agent if agent is not None else load_agent())
    agent_bin = shutil.which(spec.binary)
    if not agent_bin:
        click.echo(f"Error: {spec.label} not found. Install it first.", err=True)
        click.echo(f"See: {spec.docs_url}", err=True)
        sys.exit(1)

    is_raw = skill == "raw"
    installed = not is_raw and _skill_is_installed(skill, spec)

    args = [agent_bin]
    perm_flags, permission_mode = permission_args(
        spec,
        dsp=dangerously_skip_permissions,
        auto=auto_mode,
        afk=away_from_keyboard,
    )
    args.extend(perm_flags)

    # Split the payload into instructions the agent should treat as system
    # context and the message the user "types".
    instructions = [AFK_ADDENDUM] if away_from_keyboard else []
    if installed:
        user_text = skill_trigger(spec, skill, question_str)
    else:
        if not is_raw:
            from ashley import GENERATED_DIR

            if not GENERATED_DIR.is_dir() or not any(GENERATED_DIR.iterdir()):
                click.echo("Generating skills...", err=True)
                do_generate()
            instructions.insert(0, generate_prompt(skill, ""))
        # With no question, still name the skill so the agent knows which
        # workflow to start — same trigger text as the installed path.
        user_text = question_str or (
            "" if is_raw else skill_trigger(spec, skill, question_str)
        )

    system_text = "\n\n".join(instructions)

    if spec.system_prompt_flag and system_text:
        args.extend([spec.system_prompt_flag, _text_arg(system_text, detached)])
    elif system_text:
        # No system-prompt flag — fold the instructions into the prompt.
        user_text = "\n\n".join(t for t in (system_text, user_text) if t)

    if user_text:
        args.append(_text_arg(user_text, detached))

    return args, permission_mode


@main.command()
@click.argument("skill")
@click.argument("question", nargs=-1)
@click.option(
    "-dsp",
    "--dangerously-skip-permissions",
    is_flag=True,
    help="Skip all permission checks",
)
@click.option("--auto", "auto_mode", is_flag=True, help="Auto permission mode")
@click.option(
    "-afk",
    "--away-from-keyboard",
    "--leon",
    is_flag=True,
    help="Fully autonomous (implies -dsp)",
)
@click.option(
    "--detached",
    is_flag=True,
    help="Run in background tmux session (use 'ash sessions' to manage)",
)
@click.option(
    "-c",
    "--claude",
    "use_claude",
    is_flag=True,
    help="Use Claude Code for this run",
)
@click.option(
    "-o",
    "--codex",
    "use_codex",
    is_flag=True,
    help="Use OpenAI Codex for this run",
)
def run(
    skill,
    question,
    dangerously_skip_permissions,
    auto_mode,
    away_from_keyboard,
    detached,
    use_claude,
    use_codex,
):
    """Launch the coding agent with a skill prompt."""
    from ashley.config import load_agent

    question_str = " ".join(question) if question else ""
    agent = get_agent(_resolve_agent_flags(use_claude, use_codex) or load_agent())

    # Every run is launched inside a tmux session for crash resilience.
    # Without --detached we simply attach to it immediately; with it we
    # leave it running in the background. Building with detached=True keeps
    # large prompts out of the shell arg buffer in both cases.
    claude_args, permission_mode = build_agent_invocation(
        skill,
        question_str,
        dangerously_skip_permissions,
        auto_mode,
        away_from_keyboard,
        detached=True,
        agent=agent.key,
    )

    # Load config and hooks
    from ashley.config import get_hooks_for_skill, load_config
    from ashley.hooks import run_after_hooks, run_before_hooks

    config = load_config()
    hooks = get_hooks_for_skill(config, skill)
    cwd = os.getcwd()

    # Run before hooks (abort if they fail)
    if not run_before_hooks(hooks, skill, question_str, cwd, permission_mode):
        click.echo("\033[0;31mAborted:\033[0m before_run hook failed.", err=True)
        sys.exit(1)

    from ashley.sessions import create_detached_session, ensure_tmux

    if not ensure_tmux():
        click.echo("Error: tmux is required to run sessions.", err=True)
        sys.exit(1)

    session = create_detached_session(
        skill=skill,
        question=question_str,
        claude_args=claude_args,
        cwd=cwd,
        permission_mode=permission_mode,
        agent=agent.key,
    )

    from ashley.history import record

    inv_id = record(
        skill=skill,
        question=question_str,
        cwd=cwd,
        permission=permission_mode,
        detached=detached,
        session_id=session.id,
        agent_type=agent.key,
    )

    if detached:
        click.echo(
            f"\n  \033[0;32m●\033[0m Session started: \033[1m{session.id}\033[0m"
        )
        click.echo(f"    Skill:    {skill}")
        click.echo(f"    Agent:    {agent.label}")
        if question_str:
            display_q = (
                question_str[:60] + "..." if len(question_str) > 60 else question_str
            )
            click.echo(f"    Question: {display_q}")
        click.echo(f"    tmux:     {session.tmux_session}")
        click.echo(f"    Log:      {session.log_file}")
        click.echo()
        click.echo(f"  \033[0;36mManage:\033[0m  ash sessions")
        click.echo(f"  \033[0;36mAttach:\033[0m  ash attach {session.id}")
        click.echo(f"  \033[0;36mLogs:\033[0m    ash logs {session.id}")
        click.echo(f"  \033[0;36mKill:\033[0m    ash kill {session.id}")
        click.echo()
    else:
        # Foreground run: attach immediately and block until the user exits
        # or detaches (Ctrl-b d).
        import time

        from ashley.history import record_outcome
        from ashley.sessions import attach_session

        start_time = time.monotonic()
        attach_session(session)
        elapsed = time.monotonic() - start_time

        if session.is_alive():
            # User detached — leave the session running in the background.
            click.echo(
                f"\n  \033[0;33m●\033[0m Detached; session \033[1m{session.id}\033[0m "
                f"still running."
            )
            click.echo(f"  \033[0;36mRe-attach:\033[0m  ash attach {session.id}")
        else:
            # Session finished; tidy up its record and log the outcome.
            record_outcome(inv_id, 0, elapsed)
            session.delete()

        run_after_hooks(
            hooks,
            skill,
            question_str,
            cwd,
            exit_code=0,
            permission=permission_mode,
        )


@main.command()
@click.argument("pipeline")
@click.argument("question", nargs=-1)
@click.option(
    "-dsp",
    "--dangerously-skip-permissions",
    is_flag=True,
    help="Skip all permission checks",
)
@click.option("--auto", "auto_mode", is_flag=True, help="Auto permission mode")
@click.option(
    "-afk",
    "--away-from-keyboard",
    "--leon",
    is_flag=True,
    help="Fully autonomous (implies -dsp)",
)
@click.option(
    "-c",
    "--claude",
    "use_claude",
    is_flag=True,
    help="Use Claude Code for this run",
)
@click.option(
    "-o",
    "--codex",
    "use_codex",
    is_flag=True,
    help="Use OpenAI Codex for this run",
)
def pipe(
    pipeline,
    question,
    dangerously_skip_permissions,
    auto_mode,
    away_from_keyboard,
    use_claude,
    use_codex,
):
    """Run a skill pipeline (e.g., ash pipe feat+commit+changelog "add login").

    Chain skills with '+' or use a named pipeline from config.yaml.
    """
    from ashley.pipeline import run_pipeline

    question_str = " ".join(question) if question else ""
    exit_code = run_pipeline(
        pipeline,
        question=question_str,
        dangerously_skip_permissions=dangerously_skip_permissions,
        auto_mode=auto_mode,
        away_from_keyboard=away_from_keyboard,
        agent=_resolve_agent_flags(use_claude, use_codex),
    )
    sys.exit(exit_code)


@main.command()
def sessions():
    """Manage detached sessions (interactive TUI)."""
    from ashley.tui.sessions_app import SessionsApp

    app = SessionsApp()
    app.run()


@main.command()
@click.argument("session_id")
def attach(session_id):
    """Attach to a detached session."""
    from ashley.sessions import attach_session, resolve_session

    session = resolve_session(session_id)
    if not session:
        click.echo(f"No session found matching: {session_id}", err=True)
        click.echo("Run 'ash sessions' to see all sessions.", err=True)
        sys.exit(1)

    if not session.is_alive():
        click.echo(f"Session {session.id} is not running.", err=True)
        click.echo(f"View its log: ash logs {session.id}", err=True)
        sys.exit(1)

    sys.exit(attach_session(session))


@main.command()
@click.argument("session_id")
@click.option("-f", "--follow", is_flag=True, help="Follow log output")
@click.option("-n", "--tail", default=0, help="Show last N lines (0 = all)")
def logs(session_id, follow, tail):
    """View logs of a detached session."""
    from ashley.sessions import read_log, resolve_session

    session = resolve_session(session_id)
    if not session:
        click.echo(f"No session found matching: {session_id}", err=True)
        sys.exit(1)

    if follow:
        # Use tail -f on the log file
        from pathlib import Path

        log_path = Path(session.log_file)
        if not log_path.is_file():
            click.echo("(no log file yet)")
            sys.exit(0)
        subprocess.run(["tail", "-f", str(log_path)])
    else:
        content = read_log(session, tail=tail)
        click.echo(content if content.strip() else "(empty log)")


@main.command()
@click.argument("session_id")
def kill(session_id):
    """Kill a detached session."""
    from ashley.sessions import kill_all_sessions, kill_session, resolve_session

    if session_id == "all":
        killed = kill_all_sessions()
        click.echo(f"Killed {killed} session(s).")
        return

    session = resolve_session(session_id)
    if not session:
        click.echo(f"No session found matching: {session_id}", err=True)
        sys.exit(1)

    kill_session(session)
    click.echo(f"Killed session {session.id} ({session.skill})")


@main.command()
def create():
    """Create a new skill interactively (TUI wizard)."""
    from ashley.tui.create_app import CreateApp

    app = CreateApp()
    app.run()


@main.command()
def config():
    """Open or initialize the Ashley config file."""
    from ashley.config import CONFIG_PATH, init_config

    config_path = init_config()
    click.echo(f"Config: {config_path}")
    if shutil.which("$EDITOR") or os.environ.get("EDITOR"):
        editor = os.environ.get("EDITOR", "vi")
        subprocess.run([editor, str(config_path)])
    else:
        click.echo(f"Edit it at: {CONFIG_PATH}")


@main.command()
@click.option(
    "-c",
    "--claude",
    "use_claude",
    is_flag=True,
    help="Install for Claude Code (skip the prompt)",
)
@click.option(
    "-o",
    "--codex",
    "use_codex",
    is_flag=True,
    help="Install for OpenAI Codex (skip the prompt)",
)
@click.option("--both", is_flag=True, help="Install for both agents (skip the prompt)")
def install(use_claude, use_codex, both):
    """Generate and install skills for a coding agent.

    Without a flag, Ashley reinstalls for whichever agents already have
    skills, and asks which agent to set up on a fresh machine.
    """
    if both:
        agents = list(AGENT_KEYS)
    elif use_claude or use_codex:
        agents = [k for k, on in (("claude", use_claude), ("codex", use_codex)) if on]
    else:
        agents = None

    do_generate()
    do_install(agents)


@main.command()
def uninstall():
    """Remove ashley skills from every agent's skills directory."""
    do_uninstall()


@main.command()
@click.argument("name", required=False)
def agent(name):
    """Show or set the default coding agent (claude | codex).

    With no argument, prints the current preference.
    """
    from ashley.config import load_agent, save_agent

    if name is None:
        current = get_agent(load_agent())
        click.echo(f"  Default agent:  {current.label} ({current.key})")
        click.echo(f"  Skills dir:     {skills_dir(current)}")
        click.echo(f"  Available:      {', '.join(AGENT_KEYS)}")
        click.echo("\n  Change it with: ash agent <name>")
        return

    key = name.strip().lower()
    try:
        save_agent(key)
    except ValueError:
        click.echo(
            f"Error: unknown agent '{name}'. Choose one of: {', '.join(AGENT_KEYS)}",
            err=True,
        )
        sys.exit(1)

    spec = get_agent(key)
    click.echo(f"Default agent set to {spec.label}.")
    if not shutil.which(spec.binary):
        click.echo(
            f"Note: '{spec.binary}' is not on PATH — run 'ash install --{spec.key}'."
        )


@main.command()
@click.option("--branch", default="main", help="Branch to pull from")
def update(branch):
    """Pull latest, re-generate, and re-install skills."""
    do_update(branch)


@main.command()
def version():
    """Print version."""
    click.echo(f"ashley {ashley.__version__}")


# ── History commands ──


@main.group()
def history():
    """View and manage invocation history."""
    pass


@history.command("show")
@click.option("--skill", default=None, help="Filter by skill name")
@click.option("--search", "-s", default=None, help="Search in skill/question/directory")
@click.option("--limit", "-n", default=20, help="Number of entries to show")
@click.option("--offset", default=0, help="Skip first N entries")
@click.option("--tui", is_flag=True, help="Open interactive history browser")
def history_show(skill, search, limit, offset, tui):
    """Show invocation history (default: last 20)."""
    if tui:
        from ashley.tui.history_app import HistoryApp

        app = HistoryApp()
        app.run()
        return

    from ashley.history import query as hquery

    entries = hquery(skill=skill, limit=limit, offset=offset, search=search)
    if not entries:
        click.echo("No history entries found.")
        return

    # Table header
    click.echo(f"  {'ID':>5}  {'Time':<17}  {'Skill':<12}  {'Dir':<30}  {'Question'}")
    click.echo(f"  {'─' * 5}  {'─' * 17}  {'─' * 12}  {'─' * 30}  {'─' * 30}")
    for inv in entries:
        detached_mark = " ⇢" if inv.detached else ""
        cwd_short = inv.cwd
        if len(cwd_short) > 30:
            cwd_short = "…" + cwd_short[-(29):]
        click.echo(
            f"  {inv.id:>5}  {inv.time_display:<17}  "
            f"{inv.skill + detached_mark:<12}  "
            f"{cwd_short:<30}  {inv.question_short}"
        )


@history.command("browse")
def history_browse():
    """Open interactive history browser (TUI)."""
    from ashley.tui.history_app import HistoryApp

    app = HistoryApp()
    app.run()


@history.command("prune")
@click.argument("days", type=int)
def history_prune(days):
    """Delete entries older than N days."""
    from ashley.history import prune

    deleted = prune(days)
    click.echo(f"Pruned {deleted} entries older than {days} days.")


@history.command("clear")
@click.confirmation_option(prompt="Delete ALL history entries?")
def history_clear():
    """Delete all history entries."""
    from ashley.history import clear

    deleted = clear()
    click.echo(f"Cleared {deleted} history entries.")


@history.command("stats")
@click.option("--skill", default=None, help="Filter stats by skill name")
@click.option(
    "--agent",
    "agent_type",
    default=None,
    type=click.Choice(AGENT_KEYS),
    help="Filter stats by coding agent",
)
def history_stats(skill, agent_type):
    """Show skill usage analytics."""
    from ashley.history import stats as hstats

    data = hstats(skill=skill, agent_type=agent_type)

    click.echo()
    click.echo("  \033[1mAshley Analytics\033[0m")
    if skill:
        click.echo(f"  Skill: {skill}")
    if agent_type:
        click.echo(f"  Agent: {get_agent(agent_type).label}")
    click.echo(f"  {'─' * 40}")
    click.echo()
    click.echo(f"  Total invocations:  {data['total']}")

    if data["top_skills"]:
        click.echo()
        click.echo("  \033[1mTop Skills\033[0m")
        for name, cnt in data["top_skills"]:
            bar = "█" * min(cnt, 30)
            click.echo(f"    {name:<12} {cnt:>4}  {bar}")

    if data["by_agent"]:
        click.echo()
        click.echo("  \033[1mBy Agent\033[0m")
        for key, cnt in data["by_agent"]:
            bar = "█" * min(cnt, 30)
            click.echo(f"    {get_agent(key).label:<14} {cnt:>4}  {bar}")

    click.echo()


@history.command("info")
def history_info():
    """Show history database location and stats."""
    from ashley.history import count as hcount
    from ashley.history import db_path, db_size

    click.echo(f"  Database:  {db_path()}")
    click.echo(f"  Size:      {db_size()}")
    click.echo(f"  Entries:   {hcount()}")


if __name__ == "__main__":
    main()
