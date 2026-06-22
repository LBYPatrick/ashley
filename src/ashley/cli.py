"""Ashley CLI — Interactive skill set framework for Claude Code."""

import os
import shutil
import subprocess
import sys

import click

import ashley
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

    class VibeApp(App):
        TITLE = f"Ashley v{ashley.__version__}"
        SUB_TITLE = "Vibe — Skill Browser"

        def on_mount(self):
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


def _skill_is_installed(skill: str) -> bool:
    """Check if a skill is installed in ~/.claude/skills/ as a symlink."""
    from pathlib import Path

    skills_dir = Path.home() / ".claude" / "skills"
    # Check both "a-<skill>" and "<skill>" forms
    for name in (f"a-{skill}", skill):
        candidate = skills_dir / name
        if candidate.is_dir() and (candidate / "SKILL.md").is_file():
            return True
    return False


def _build_claude_invocation(
    skill: str,
    question_str: str,
    dangerously_skip_permissions: bool,
    auto_mode: bool,
    away_from_keyboard: bool,
    detached: bool = False,
) -> tuple[list[str], str]:
    """Build claude CLI arguments.

    When a skill is installed in ~/.claude/skills/, we let Claude Code
    load it natively via the slash command — no need to inline the
    entire prompt as --append-system-prompt. This avoids tmux/shell
    argument length limits in detached mode.

    When the skill is NOT installed, we fall back to
    --append-system-prompt for interactive mode, or write the prompt
    to a temp file and pass it via --system-prompt-file for detached
    mode (avoiding shell argument limits).

    Returns (claude_args_list, permission_mode_label).
    """
    claude_bin = shutil.which("claude")
    if not claude_bin:
        click.echo("Error: Claude Code not found. Install it first.", err=True)
        sys.exit(1)

    is_raw = skill == "raw"
    installed = not is_raw and _skill_is_installed(skill)

    # Only generate/inline the prompt if the skill isn't installed
    system_prompt = ""
    if not is_raw and not installed:
        from ashley import GENERATED_DIR

        if not GENERATED_DIR.is_dir() or not any(GENERATED_DIR.iterdir()):
            click.echo("Generating skills...", err=True)
            do_generate()
        system_prompt = generate_prompt(skill, "")

    claude_args = [claude_bin]
    permission_mode = "default"

    if dangerously_skip_permissions or away_from_keyboard:
        claude_args.append("--dangerously-skip-permissions")
        permission_mode = "dsp"
    elif auto_mode:
        claude_args.extend(["--permission-mode", "auto"])
        permission_mode = "auto"

    afk_addendum = ""
    if away_from_keyboard:
        permission_mode = "afk"
        afk_addendum = """

## AFK Mode — Autonomous Operation

The user is away from the keyboard. You MUST operate fully autonomously:
- NEVER ask the user for clarification, confirmation, or input — make your best judgement call and proceed.
- ALWAYS choose 'yes' / 'proceed' / 'continue' when faced with any prompt or decision point.
- If a tool requires user approval, assume approval is granted.
- If you encounter ambiguity, pick the most reasonable option and move forward.
- Complete the entire task end-to-end without stopping."""

    if installed:
        # Skill is installed — Claude Code will load it via slash command.
        # Only pass AFK addendum if needed (it's short enough for CLI args).
        if afk_addendum:
            claude_args.extend(["--append-system-prompt", afk_addendum])

        # Trigger the slash command; append question if provided
        if question_str:
            claude_args.append(f"/a-{skill} {question_str}")
        else:
            claude_args.append(f"/a-{skill}")
    else:
        # Skill not installed — inline the prompt
        system_prompt += afk_addendum

        if system_prompt and detached:
            # Detached mode: write prompt to temp file to avoid
            # shell argument length limits with tmux.
            # We store the file path as a marker so sessions.py can
            # build the shell command with proper cat expansion.
            import tempfile

            prompt_file = tempfile.NamedTemporaryFile(
                mode="w",
                prefix="ashley-prompt-",
                suffix=".md",
                delete=False,
            )
            prompt_file.write(system_prompt)
            prompt_file.close()
            # Marker for sessions.py to handle
            claude_args.append(f"__ASHLEY_PROMPT_FILE__={prompt_file.name}")
        elif system_prompt:
            claude_args.extend(["--append-system-prompt", system_prompt])

        # User message
        if question_str:
            claude_args.append(question_str)
        elif not is_raw:
            claude_args.append(f"/a-{skill}")

    return claude_args, permission_mode


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
def run(
    skill,
    question,
    dangerously_skip_permissions,
    auto_mode,
    away_from_keyboard,
    detached,
):
    """Launch Claude Code with a skill prompt."""
    question_str = " ".join(question) if question else ""

    claude_args, permission_mode = _build_claude_invocation(
        skill,
        question_str,
        dangerously_skip_permissions,
        auto_mode,
        away_from_keyboard,
        detached=detached,
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

    if detached:
        from ashley.sessions import create_detached_session, ensure_tmux

        if not ensure_tmux():
            click.echo("Error: tmux is required for detached mode.", err=True)
            sys.exit(1)

        session = create_detached_session(
            skill=skill,
            question=question_str,
            claude_args=claude_args,
            cwd=cwd,
            permission_mode=permission_mode,
        )

        # Record in history
        from ashley.history import record

        record(
            skill=skill,
            question=question_str,
            cwd=cwd,
            permission=permission_mode,
            detached=True,
            session_id=session.id,
        )

        click.echo(
            f"\n  \033[0;32m●\033[0m Session started: \033[1m{session.id}\033[0m"
        )
        click.echo(f"    Skill:    {skill}")
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
        # Record in history
        import time

        from ashley.history import record, record_outcome

        inv_id = record(
            skill=skill,
            question=question_str,
            cwd=cwd,
            permission=permission_mode,
            detached=False,
        )

        start_time = time.monotonic()
        result = subprocess.run(claude_args)
        elapsed = time.monotonic() - start_time

        # Record outcome
        record_outcome(inv_id, result.returncode, elapsed)

        # Run after hooks
        run_after_hooks(
            hooks,
            skill,
            question_str,
            cwd,
            exit_code=result.returncode,
            permission=permission_mode,
        )

        sys.exit(result.returncode)


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
def pipe(
    pipeline, question, dangerously_skip_permissions, auto_mode, away_from_keyboard
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
    from ashley.sessions import kill_session, load_all_sessions, resolve_session

    if session_id == "all":
        sessions_list = load_all_sessions()
        killed = 0
        for s in sessions_list:
            if s.is_alive():
                kill_session(s)
                click.echo(f"  Killed {s.id} ({s.skill})")
                killed += 1
            else:
                s.meta_path.unlink(missing_ok=True)
        click.echo(f"\nKilled {killed} session(s).")
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
def install():
    """Generate and install skills to ~/.claude/skills/."""
    do_generate()
    do_install()


@main.command()
def uninstall():
    """Remove ashley skills from ~/.claude/skills/."""
    do_uninstall()


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
def history_stats(skill):
    """Show skill invocation analytics and success rates."""
    from ashley.history import stats as hstats

    data = hstats(skill=skill)

    click.echo()
    click.echo(f"  \033[1mAshley Analytics\033[0m")
    if skill:
        click.echo(f"  Skill: {skill}")
    click.echo(f"  {'─' * 40}")
    click.echo()
    click.echo(f"  Total invocations:  {data['total']}")
    click.echo(f"  Success:            \033[0;32m{data['success']}\033[0m")
    click.echo(f"  Failure:            \033[0;31m{data['failure']}\033[0m")
    click.echo(f"  Unknown:            {data['unknown']}")

    if data["avg_duration"] is not None:
        click.echo()
        click.echo(f"  Avg duration:       {data['avg_duration']:.0f}s")
        click.echo(f"  Min duration:       {data['min_duration']:.0f}s")
        click.echo(f"  Max duration:       {data['max_duration']:.0f}s")

    if data["top_skills"]:
        click.echo()
        click.echo(f"  \033[1mTop Skills\033[0m")
        for name, cnt in data["top_skills"]:
            bar = "█" * min(cnt, 30)
            click.echo(f"    {name:<12} {cnt:>4}  {bar}")

    if data["skill_rates"]:
        click.echo()
        click.echo(f"  \033[1mSuccess Rates\033[0m")
        for name, total, wins, rate in data["skill_rates"]:
            color = (
                "\033[0;32m"
                if rate >= 80
                else ("\033[0;33m" if rate >= 50 else "\033[0;31m")
            )
            click.echo(f"    {name:<12} {color}{rate:5.1f}%\033[0m  ({wins}/{total})")

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
