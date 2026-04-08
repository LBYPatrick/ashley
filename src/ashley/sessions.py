"""Ashley Session Manager.

Manages detached Claude Code sessions using tmux with a JSON-based
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
        started = datetime.fromisoformat(self.started_at)
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
    for meta_file in sorted(SESSIONS_DIR.glob("*.json"), reverse=True):
        try:
            data = json.loads(meta_file.read_text())
            sessions.append(Session(**data))
        except (json.JSONDecodeError, TypeError):
            continue
    return sessions


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


def create_detached_session(
    skill: str,
    question: str,
    claude_args: list[str],
    cwd: str,
    permission_mode: str = "",
) -> Session:
    """Launch Claude Code in a detached tmux session with log capture.

    Handles the __ASHLEY_PROMPT_FILE__ marker from _build_claude_invocation
    to read the prompt from a temp file instead of passing it as a shell arg
    (which would hit tmux's input buffer limit for large prompts).

    Returns the created Session object.
    """
    session_id = uuid.uuid4().hex[:8]
    tmux_name = f"ashley-{session_id}"
    log_file = str(SESSIONS_DIR / f"{session_id}.log")

    SESSIONS_DIR.mkdir(parents=True, exist_ok=True)

    # Extract prompt file marker if present
    prompt_file = None
    filtered_args = []
    for arg in claude_args:
        if arg.startswith("__ASHLEY_PROMPT_FILE__="):
            prompt_file = arg.split("=", 1)[1]
        else:
            filtered_args.append(arg)
    claude_args = filtered_args

    # Build the claude command string for tmux.
    # When a prompt file is present, we inject --append-system-prompt
    # with a $(cat ...) expansion so the shell reads the file at
    # runtime instead of embedding the content in the arg string.
    cmd_parts = []
    for arg in claude_args:
        escaped = arg.replace("'", "'\\''")
        cmd_parts.append(f"'{escaped}'")

    if prompt_file:
        # Insert the system prompt via file read — shell expands $(...)
        escaped_path = prompt_file.replace("'", "'\\''")
        cmd_parts.insert(1, "'--append-system-prompt'")
        cmd_parts.insert(2, f"\"$(cat '{escaped_path}')\"")

    claude_cmd = " ".join(cmd_parts)

    # Build cleanup for temp prompt file
    cleanup = ""
    if prompt_file:
        escaped_path = prompt_file.replace("'", "'\\''")
        cleanup = f"rm -f '{escaped_path}'; "

    # Create tmux session running claude, with automatic exit message
    shell_cmd = (
        f"cd '{cwd}' && {claude_cmd}; "
        f"{cleanup}"
        f"echo ''; echo '[Session ended - press Enter to close]'; read"
    )

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
