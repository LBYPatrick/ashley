"""Ashley Invocation History.

Tracks every skill invocation with timestamp, working directory, skill
name, question, permission mode, coding agent, and whether it was
detached. Stored in a SQLite database at the platform-appropriate user
data directory:

  - macOS:  ~/Library/Application Support/ashley/history.db
  - Linux:  ~/.local/share/ashley/history.db

The database is created automatically on first use, and older database
files are migrated in place on the next connection.
"""

import platform
import sqlite3
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

from ashley.agents import DEFAULT_AGENT

# XDG_DATA_HOME on Linux, ~/Library/Application Support on macOS
_system = platform.system()
if _system == "Darwin":
    _data_dir = Path.home() / "Library" / "Application Support" / "ashley"
elif _system == "Linux":
    _xdg = Path(
        __import__("os").environ.get(
            "XDG_DATA_HOME", str(Path.home() / ".local" / "share")
        )
    )
    _data_dir = _xdg / "ashley"
else:
    # Fallback for other platforms
    _data_dir = Path.home() / ".ashley"

DB_PATH = _data_dir / "history.db"

_SCHEMA = f"""
CREATE TABLE IF NOT EXISTS invocations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   TEXT    NOT NULL,
    skill       TEXT    NOT NULL,
    question    TEXT    NOT NULL DEFAULT '',
    cwd         TEXT    NOT NULL DEFAULT '',
    permission  TEXT    NOT NULL DEFAULT 'default',
    detached    INTEGER NOT NULL DEFAULT 0,
    session_id  TEXT    NOT NULL DEFAULT '',
    exit_code   INTEGER DEFAULT NULL,
    duration_s  REAL    DEFAULT NULL,
    outcome     TEXT    NOT NULL DEFAULT 'unknown',
    agent_type  TEXT    NOT NULL DEFAULT '{DEFAULT_AGENT}'
);
CREATE INDEX IF NOT EXISTS idx_invocations_timestamp ON invocations(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_invocations_skill ON invocations(skill);
"""

# Columns added after the original schema, mapped to their ALTER TABLE
# definition. Existing rows take the column default, so history recorded
# before multi-agent support is attributed to Claude Code.
_ADDED_COLUMNS: dict[str, str] = {
    "exit_code": "INTEGER DEFAULT NULL",
    "duration_s": "REAL DEFAULT NULL",
    "outcome": "TEXT NOT NULL DEFAULT 'unknown'",
    "agent_type": f"TEXT NOT NULL DEFAULT '{DEFAULT_AGENT}'",
}


@dataclass
class Invocation:
    id: int
    timestamp: str
    skill: str
    question: str
    cwd: str
    permission: str
    detached: bool
    session_id: str
    exit_code: int | None = None
    duration_s: float | None = None
    outcome: str = "unknown"  # unknown | success | failure | cancelled
    agent_type: str = DEFAULT_AGENT  # claude | codex

    @property
    def agent_label(self) -> str:
        """Human-readable name of the coding agent that ran this skill."""
        from ashley.agents import get_agent

        return get_agent(self.agent_type).label

    @property
    def time_display(self) -> str:
        """Human-readable timestamp (local time)."""
        try:
            dt = datetime.fromisoformat(self.timestamp)
            return dt.strftime("%Y-%m-%d %H:%M:%S")
        except ValueError:
            return self.timestamp[:19]

    @property
    def question_short(self) -> str:
        """Truncated question for display."""
        if not self.question:
            return "(no question)"
        return self.question[:80] + "..." if len(self.question) > 80 else self.question

    @property
    def duration_display(self) -> str:
        """Human-readable duration."""
        if self.duration_s is None:
            return "—"
        secs = int(self.duration_s)
        if secs < 60:
            return f"{secs}s"
        if secs < 3600:
            return f"{secs // 60}m {secs % 60}s"
        hours = secs // 3600
        mins = (secs % 3600) // 60
        return f"{hours}h {mins}m"

    @property
    def outcome_icon(self) -> str:
        """Icon for outcome status."""
        return {
            "success": "✓",
            "failure": "✗",
            "cancelled": "○",
            "unknown": "?",
        }.get(self.outcome, "?")


def migrate(conn: sqlite3.Connection) -> list[str]:
    """Bring an older database file up to the current schema.

    Adds any column missing from ``invocations``. SQLite backfills each
    existing row with the column's default, so invocations recorded before
    a column existed get a sensible value rather than NULL — in particular
    pre-multi-agent history is attributed to Claude Code.

    Args:
        conn: An open connection to the history database.

    Returns:
        The names of the columns that were added, in schema order.
    """
    existing = {row[1] for row in conn.execute("PRAGMA table_info(invocations)")}
    added = [name for name in _ADDED_COLUMNS if name not in existing]
    for name in added:
        conn.execute(
            f"ALTER TABLE invocations ADD COLUMN {name} {_ADDED_COLUMNS[name]}"
        )
    if added:
        conn.commit()
    return added


def _get_conn() -> sqlite3.Connection:
    """Get a database connection, creating or migrating the DB as needed."""
    DB_PATH.parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(str(DB_PATH))
    conn.executescript(_SCHEMA)
    migrate(conn)
    return conn


def record(
    skill: str,
    question: str = "",
    cwd: str = "",
    permission: str = "default",
    detached: bool = False,
    session_id: str = "",
    agent_type: str = DEFAULT_AGENT,
) -> int:
    """Record an invocation. Returns the row ID."""
    conn = _get_conn()
    try:
        cur = conn.execute(
            """
            INSERT INTO invocations
                (timestamp, skill, question, cwd, permission, detached,
                 session_id, agent_type)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                datetime.now(timezone.utc).isoformat(),
                skill,
                question,
                cwd,
                permission,
                1 if detached else 0,
                session_id,
                agent_type,
            ),
        )
        conn.commit()
        return cur.lastrowid
    finally:
        conn.close()


def query(
    skill: str | None = None,
    limit: int = 50,
    offset: int = 0,
    search: str | None = None,
    agent_type: str | None = None,
) -> list[Invocation]:
    """Query invocations, newest first.

    Optional filters: skill name, coding agent, full-text search in
    question/cwd.
    """
    conn = _get_conn()
    try:
        conditions = []
        params: list = []

        if skill:
            conditions.append("skill = ?")
            params.append(skill)
        if agent_type:
            conditions.append("agent_type = ?")
            params.append(agent_type)
        if search:
            conditions.append("(question LIKE ? OR cwd LIKE ? OR skill LIKE ?)")
            pattern = f"%{search}%"
            params.extend([pattern, pattern, pattern])

        where = f"WHERE {' AND '.join(conditions)}" if conditions else ""

        rows = conn.execute(
            f"""
            SELECT id, timestamp, skill, question, cwd, permission, detached,
                   session_id, exit_code, duration_s, outcome, agent_type
            FROM invocations
            {where}
            ORDER BY timestamp DESC
            LIMIT ? OFFSET ?
            """,
            [*params, limit, offset],
        ).fetchall()

        return [
            Invocation(
                id=r[0],
                timestamp=r[1],
                skill=r[2],
                question=r[3],
                cwd=r[4],
                permission=r[5],
                detached=bool(r[6]),
                session_id=r[7],
                exit_code=r[8],
                duration_s=r[9],
                outcome=r[10] or "unknown",
                agent_type=r[11] or DEFAULT_AGENT,
            )
            for r in rows
        ]
    finally:
        conn.close()


def count(skill: str | None = None, search: str | None = None) -> int:
    """Count total invocations matching filters."""
    conn = _get_conn()
    try:
        conditions = []
        params: list = []
        if skill:
            conditions.append("skill = ?")
            params.append(skill)
        if search:
            conditions.append("(question LIKE ? OR cwd LIKE ? OR skill LIKE ?)")
            pattern = f"%{search}%"
            params.extend([pattern, pattern, pattern])

        where = f"WHERE {' AND '.join(conditions)}" if conditions else ""
        row = conn.execute(
            f"SELECT COUNT(*) FROM invocations {where}", params
        ).fetchone()
        return row[0]
    finally:
        conn.close()


def prune(days: int) -> int:
    """Delete invocations older than N days. Returns count deleted."""
    conn = _get_conn()
    try:
        cutoff = datetime.now(timezone.utc)
        from datetime import timedelta

        cutoff = cutoff - timedelta(days=days)
        cur = conn.execute(
            "DELETE FROM invocations WHERE timestamp < ?",
            (cutoff.isoformat(),),
        )
        conn.commit()
        return cur.rowcount
    finally:
        conn.close()


def clear() -> int:
    """Delete all invocations. Returns count deleted."""
    conn = _get_conn()
    try:
        cur = conn.execute("DELETE FROM invocations")
        conn.commit()
        return cur.rowcount
    finally:
        conn.close()


def record_outcome(
    invocation_id: int,
    exit_code: int,
    duration_s: float,
) -> None:
    """Update an invocation with its outcome after completion."""
    outcome = "success" if exit_code == 0 else "failure"
    conn = _get_conn()
    try:
        conn.execute(
            """
            UPDATE invocations
            SET exit_code = ?, duration_s = ?, outcome = ?
            WHERE id = ?
            """,
            (exit_code, duration_s, outcome, invocation_id),
        )
        conn.commit()
    finally:
        conn.close()


def stats(skill: str | None = None, agent_type: str | None = None) -> dict:
    """Get aggregated usage statistics for skill invocations.

    Args:
        skill: Restrict the figures to a single skill.
        agent_type: Restrict the figures to a single coding agent.

    Returns:
        A dict with the total invocation count, the most-used skills, and
        the per-agent breakdown. Invocations recorded before multi-agent
        support are counted as Claude Code.
    """
    conn = _get_conn()
    try:
        conditions = []
        params: list = []
        if skill:
            conditions.append("skill = ?")
            params.append(skill)
        if agent_type:
            conditions.append("agent_type = ?")
            params.append(agent_type)
        where = f"WHERE {' AND '.join(conditions)}" if conditions else ""

        # Overall counts
        row = conn.execute(
            f"SELECT COUNT(*) FROM invocations {where}", params
        ).fetchone()
        total = row[0]

        # Top skills by usage
        skill_rows = conn.execute(
            f"""
            SELECT skill, COUNT(*) as cnt FROM invocations
            {where}
            GROUP BY skill
            ORDER BY cnt DESC, skill ASC
            LIMIT 10
            """,
            params,
        ).fetchall()
        top_skills = [(r[0], r[1]) for r in skill_rows]

        # Usage per coding agent
        agent_rows = conn.execute(
            f"""
            SELECT agent_type, COUNT(*) as cnt FROM invocations
            {where}
            GROUP BY agent_type
            ORDER BY cnt DESC, agent_type ASC
            """,
            params,
        ).fetchall()
        by_agent = [(r[0] or DEFAULT_AGENT, r[1]) for r in agent_rows]

        return {
            "total": total,
            "top_skills": top_skills,
            "by_agent": by_agent,
        }
    finally:
        conn.close()


def delete_one(invocation_id: int) -> bool:
    """Delete a single invocation by ID. Returns True if found."""
    conn = _get_conn()
    try:
        cur = conn.execute("DELETE FROM invocations WHERE id = ?", (invocation_id,))
        conn.commit()
        return cur.rowcount > 0
    finally:
        conn.close()


def db_path() -> Path:
    """Return the database file path."""
    return DB_PATH


def db_size() -> str:
    """Return the database file size as a human-readable string."""
    if not DB_PATH.is_file():
        return "0 B"
    size = DB_PATH.stat().st_size
    for unit in ("B", "KB", "MB", "GB"):
        if size < 1024:
            return f"{size:.1f} {unit}" if unit != "B" else f"{size} {unit}"
        size /= 1024
    return f"{size:.1f} TB"
