"""Ashley Invocation History.

Tracks every skill invocation with timestamp, working directory, skill
name, question, permission mode, and whether it was detached. Stored in
a SQLite database at the platform-appropriate user data directory:

  - macOS:  ~/Library/Application Support/ashley/history.db
  - Linux:  ~/.local/share/ashley/history.db

The database is created automatically on first use.
"""

import platform
import sqlite3
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

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

_SCHEMA = """
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
    outcome     TEXT    NOT NULL DEFAULT 'unknown'
);
CREATE INDEX IF NOT EXISTS idx_invocations_timestamp ON invocations(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_invocations_skill ON invocations(skill);
"""

# Migration: add new columns to existing databases
_MIGRATIONS = [
    "ALTER TABLE invocations ADD COLUMN exit_code INTEGER DEFAULT NULL",
    "ALTER TABLE invocations ADD COLUMN duration_s REAL DEFAULT NULL",
    "ALTER TABLE invocations ADD COLUMN outcome TEXT NOT NULL DEFAULT 'unknown'",
]


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


def _get_conn() -> sqlite3.Connection:
    """Get a database connection, creating the DB and tables if needed."""
    DB_PATH.parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(str(DB_PATH))
    conn.executescript(_SCHEMA)
    # Run migrations (ignore errors for columns that already exist)
    for migration in _MIGRATIONS:
        try:
            conn.execute(migration)
            conn.commit()
        except sqlite3.OperationalError:
            pass  # Column already exists
    return conn


def record(
    skill: str,
    question: str = "",
    cwd: str = "",
    permission: str = "default",
    detached: bool = False,
    session_id: str = "",
) -> int:
    """Record an invocation. Returns the row ID."""
    conn = _get_conn()
    try:
        cur = conn.execute(
            """
            INSERT INTO invocations (timestamp, skill, question, cwd, permission, detached, session_id)
            VALUES (?, ?, ?, ?, ?, ?, ?)
            """,
            (
                datetime.now(timezone.utc).isoformat(),
                skill,
                question,
                cwd,
                permission,
                1 if detached else 0,
                session_id,
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
) -> list[Invocation]:
    """Query invocations, newest first.

    Optional filters: skill name, full-text search in question/cwd.
    """
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

        rows = conn.execute(
            f"""
            SELECT id, timestamp, skill, question, cwd, permission, detached,
                   session_id, exit_code, duration_s, outcome
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


def stats(skill: str | None = None) -> dict:
    """Get aggregated statistics for skill invocations.

    Returns a dict with total, success, failure, avg_duration, etc.
    """
    conn = _get_conn()
    try:
        where = ""
        params: list = []
        if skill:
            where = "WHERE skill = ?"
            params.append(skill)

        # Overall counts
        row = conn.execute(
            f"SELECT COUNT(*) FROM invocations {where}", params
        ).fetchone()
        total = row[0]

        # Outcome breakdown
        outcome_rows = conn.execute(
            f"""
            SELECT outcome, COUNT(*) FROM invocations
            {where}
            GROUP BY outcome
            """,
            params,
        ).fetchall()
        outcomes = {r[0]: r[1] for r in outcome_rows}

        # Average duration for completed runs
        duration_where = f"{where} {'AND' if where else 'WHERE'} duration_s IS NOT NULL"
        row = conn.execute(
            f"SELECT AVG(duration_s), MIN(duration_s), MAX(duration_s) FROM invocations {duration_where}",
            params,
        ).fetchone()
        avg_duration = row[0]
        min_duration = row[1]
        max_duration = row[2]

        # Top skills by usage
        skill_rows = conn.execute(
            f"""
            SELECT skill, COUNT(*) as cnt FROM invocations
            {where}
            GROUP BY skill
            ORDER BY cnt DESC
            LIMIT 10
            """,
            params,
        ).fetchall()
        top_skills = [(r[0], r[1]) for r in skill_rows]

        # Success rate per skill
        rate_rows = conn.execute(
            f"""
            SELECT skill,
                   COUNT(*) as total,
                   SUM(CASE WHEN outcome = 'success' THEN 1 ELSE 0 END) as wins
            FROM invocations
            {where}
            GROUP BY skill
            ORDER BY total DESC
            """,
            params,
        ).fetchall()
        skill_rates = [
            (r[0], r[1], r[2], (r[2] / r[1] * 100) if r[1] > 0 else 0)
            for r in rate_rows
        ]

        return {
            "total": total,
            "success": outcomes.get("success", 0),
            "failure": outcomes.get("failure", 0),
            "unknown": outcomes.get("unknown", 0),
            "avg_duration": avg_duration,
            "min_duration": min_duration,
            "max_duration": max_duration,
            "top_skills": top_skills,
            "skill_rates": skill_rates,
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
