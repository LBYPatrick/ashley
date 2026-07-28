"""Ashley Session Manager.

Manages detached coding-agent sessions using tmux with a JSON-based
session registry for reliable tracking, log persistence, and metadata.

Sessions are stored in ~/.ashley/sessions/ as JSON files with
corresponding .log files for output capture.
"""

import json
import shutil
import subprocess
import uuid
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from pathlib import Path

SESSIONS_DIR = Path.home() / ".ashley" / "sessions"

GREEN = "\033[0;32m"
CYAN = "\033[0;36m"
RED = "\033[0;31m"
YELLOW = "\033[0;33m"
BOLD = "\033[1m"
DIM = "\033[2m"
NC = "\033[0m"


@dataclass
class Session:
    id: str
    skill: str
    question: str
    tmux_session: str
    log_file: str
    started_at: str
    cwd: str
    permission_mode: str = ""
    # Empty on records written before multi-agent support; treated as Claude.
    agent: str = ""
    extra_flags: list[str] = field(default_factory=list)

    @property
    def meta_path(self) -> Path:
        return SESSIONS_DIR / f"{self.id}.json"

    def save(self) -> None:
        SESSIONS_DIR.mkdir(parents=True, exist_ok=True)
        self.meta_path.write_text(json.dumps(asdict(self), indent=2) + "\n")

    def delete(self) -> None:
        self.meta_path.unlink(missing_ok=True)
        Path(self.log_file).unlink(missing_ok=True)

    def is_alive(self) -> bool:
        """Check if the tmux session is still running."""
        result = subprocess.run(
            ["tmux", "has-session", "-t", self.tmux_session],
            capture_output=True,
        )
        return result.returncode == 0

    def status(self) -> str:
        return "running" if self.is_alive() else "exited"

    def elapsed(self) -> str:
        try:
            started = datetime.fromisoformat(self.started_at)
        except (ValueError, TypeError):
            return "?"
        if started.tzinfo is None:
            started = started.replace(tzinfo=timezone.utc)
        delta = datetime.now(timezone.utc) - started
        secs = int(delta.total_seconds())
        if secs < 60:
            return f"{secs}s"
        if secs < 3600:
            return f"{secs // 60}m {secs % 60}s"
        hours = secs // 3600
        mins = (secs % 3600) // 60
        return f"{hours}h {mins}m"


def load_session(session_id: str) -> Session | None:
    """Load a session by its ID."""
    meta_path = SESSIONS_DIR / f"{session_id}.json"
    if not meta_path.is_file():
        return None
    data = json.loads(meta_path.read_text())
    return Session(**data)


def load_all_sessions() -> list[Session]:
    """Load all sessions, sorted by start time (newest first)."""
    if not SESSIONS_DIR.is_dir():
        return []
    sessions = []
    for meta_file in SESSIONS_DIR.glob("*.json"):
        try:
            data = json.loads(meta_file.read_text())
            sessions.append(Session(**data))
        except (json.JSONDecodeError, TypeError):
            continue
    sessions.sort(key=lambda s: s.started_at or "", reverse=True)
    return sessions


def sort_sessions(sessions: list[Session], mode: str = "time") -> list[Session]:
    """Return a new list of sessions ordered by the given mode.

    Args:
        sessions: Sessions to sort.
        mode: ``"time"`` for newest first, or ``"skill"`` for alphabetical
            by skill (newest first within each skill). Unknown modes fall
            back to ``"time"``.

    Returns:
        A new, sorted list. The input list is left unmodified.
    """
    by_time = sorted(sessions, key=lambda s: s.started_at or "", reverse=True)
    if mode == "skill":
        # Stable sort keeps the newest-first order within each skill group.
        return sorted(by_time, key=lambda s: s.skill.lower())
    return by_time


def resolve_session(short_id: str) -> Session | None:
    """Resolve a short ID prefix to a session."""
    for s in load_all_sessions():
        if s.id.startswith(short_id):
            return s
    return None


def ensure_tmux() -> bool:
    """Ensure tmux is installed, attempt auto-install if missing."""
    if shutil.which("tmux"):
        return True

    print(f"{CYAN}tmux not found. Installing...{NC}")
    import platform

    system = platform.system()
    try:
        if system == "Darwin":
            if shutil.which("brew"):
                subprocess.run(["brew", "install", "tmux"], check=True)
            else:
                print(f"{RED}Homebrew not found. Install tmux: brew install tmux{NC}")
                return False
        elif system == "Linux":
            if shutil.which("apt-get"):
                subprocess.run(["sudo", "apt-get", "update", "-qq"], check=True)
                subprocess.run(
                    ["sudo", "apt-get", "install", "-y", "-qq", "tmux"],
                    check=True,
                )
            elif shutil.which("dnf"):
                subprocess.run(["sudo", "dnf", "install", "-y", "tmux"], check=True)
            elif shutil.which("pacman"):
                subprocess.run(
                    ["sudo", "pacman", "-S", "--noconfirm", "tmux"], check=True
                )
            else:
                print(f"{RED}Could not auto-install tmux. Install it manually.{NC}")
                return False
        else:
            print(f"{RED}Unsupported OS. Install tmux manually.{NC}")
            return False
    except subprocess.CalledProcessError:
        print(f"{RED}tmux installation failed.{NC}")
        return False

    return shutil.which("tmux") is not None


def _shell_quote(text: str) -> str:
    """Wrap *text* in single quotes, escaping any it contains."""
    escaped = text.replace("'", "'\\''")
    return f"'{escaped}'"


def build_shell_command(claude_args: list[str], cwd: str) -> tuple[str, list[str]]:
    """Render an agent invocation as a shell command line for tmux.

    Arguments carrying the ``__ASHLEY_PROMPT_FILE__`` marker (see
    :func:`ashley.cli._text_arg`) are replaced in place with a ``$(cat …)``
    expansion, so large prompts are read from disk at launch instead of
    travelling through tmux's input buffer.

    Args:
        claude_args: The agent argv, possibly containing prompt-file markers.
        cwd: Directory the agent should start in.

    Returns:
        A ``(shell_command, temp_files)`` pair; *temp_files* are the spilled
        prompt files the caller should have removed once the agent exits.
    """
    from ashley.cli import PROMPT_FILE_MARKER

    cmd_parts: list[str] = []
    prompt_files: list[str] = []
    for arg in claude_args:
        if arg.startswith(PROMPT_FILE_MARKER):
            path = arg[len(PROMPT_FILE_MARKER) :]
            prompt_files.append(path)
            cmd_parts.append(f'"$(cat {_shell_quote(path)})"')
        else:
            cmd_parts.append(_shell_quote(arg))

    cleanup = "".join(f"rm -f {_shell_quote(p)}; " for p in prompt_files)
    shell_cmd = (
        f"cd {_shell_quote(cwd)} && {' '.join(cmd_parts)}; "
        f"{cleanup}"
        f"echo ''; echo '[Session ended - press Enter to close]'; read"
    )
    return shell_cmd, prompt_files


def create_detached_session(
    skill: str,
    question: str,
    claude_args: list[str],
    cwd: str,
    permission_mode: str = "",
    agent: str = "",
) -> Session:
    """Launch a coding agent in a detached tmux session with log capture.

    Returns the created Session object.
    """
    session_id = uuid.uuid4().hex[:8]
    tmux_name = f"ashley-{session_id}"
    log_file = str(SESSIONS_DIR / f"{session_id}.log")

    SESSIONS_DIR.mkdir(parents=True, exist_ok=True)

    shell_cmd, _ = build_shell_command(claude_args, cwd)

    subprocess.run(
        [
            "tmux",
            "new-session",
            "-d",
            "-s",
            tmux_name,
            "-x",
            "200",
            "-y",
            "50",
            "bash",
            "-c",
            shell_cmd,
        ],
        check=True,
    )

    # Enable automatic logging via pipe-pane
    subprocess.run(
        [
            "tmux",
            "pipe-pane",
            "-t",
            tmux_name,
            "-o",
            f"cat >> '{log_file}'",
        ],
        check=True,
    )

    session = Session(
        id=session_id,
        skill=skill,
        question=question,
        tmux_session=tmux_name,
        log_file=log_file,
        started_at=datetime.now(timezone.utc).isoformat(),
        cwd=cwd,
        permission_mode=permission_mode,
        agent=agent,
    )
    session.save()

    return session


def attach_session(session: Session) -> int:
    """Attach to a tmux session interactively. Returns exit code."""
    if not session.is_alive():
        print(f"{RED}Session {session.id} is not running.{NC}")
        return 1
    result = subprocess.run(["tmux", "attach-session", "-t", session.tmux_session])
    return result.returncode


def kill_session(session: Session) -> None:
    """Kill a tmux session and clean up."""
    if session.is_alive():
        subprocess.run(
            ["tmux", "kill-session", "-t", session.tmux_session],
            capture_output=True,
        )
    # Keep the log file but remove the metadata to mark as cleaned up
    session.meta_path.unlink(missing_ok=True)


def kill_all_sessions() -> int:
    """Kill every running session and remove exited records.

    Returns the number of live sessions that were killed.
    """
    killed = 0
    for session in load_all_sessions():
        if session.is_alive():
            kill_session(session)
            killed += 1
        else:
            session.meta_path.unlink(missing_ok=True)
    return killed


def read_log(session: Session, tail: int = 0) -> str:
    """Read the session log file. If tail > 0, return only the last N lines."""
    log_path = Path(session.log_file)
    if not log_path.is_file():
        return "(no log file)"
    content = log_path.read_text(errors="replace")
    if tail > 0:
        lines = content.splitlines()
        content = "\n".join(lines[-tail:])
    return content


def cleanup_dead_sessions() -> int:
    """Remove metadata for sessions whose tmux session no longer exists.

    Returns the number of cleaned-up sessions.
    """
    cleaned = 0
    for session in load_all_sessions():
        if not session.is_alive():
            session.meta_path.unlink(missing_ok=True)
            cleaned += 1
    return cleaned
