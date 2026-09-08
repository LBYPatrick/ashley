"""Tests for per-agent history tracking and the schema migration."""

import sqlite3
import tempfile
from pathlib import Path
from unittest.mock import patch

from ashley.history import Invocation, migrate, query, record, stats

# The schema exactly as it shipped before multi-agent support, used to
# build a realistic "old local db file" for the migration tests.
_LEGACY_SCHEMA = """
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
"""


def _use_temp_db():
    """Patch the DB path to a fresh temp file."""
    tmp = tempfile.NamedTemporaryFile(suffix=".db", delete=False)
    tmp.close()
    return patch("ashley.history.DB_PATH", Path(tmp.name))


def _legacy_db(rows: list[tuple[str, str]]) -> Path:
    """Write a pre-multi-agent database containing *rows* of (skill, cwd)."""
    tmp = tempfile.NamedTemporaryFile(suffix=".db", delete=False)
    tmp.close()
    conn = sqlite3.connect(tmp.name)
    conn.executescript(_LEGACY_SCHEMA)
    conn.executemany(
        "INSERT INTO invocations (timestamp, skill, cwd) VALUES (?, ?, ?)",
        [("2026-01-01T00:00:00+00:00", skill, cwd) for skill, cwd in rows],
    )
    conn.commit()
    conn.close()
    return Path(tmp.name)


# ── Recording ──


def test_record_defaults_to_claude():
    with _use_temp_db():
        record(skill="feat", cwd="/tmp")
        assert query()[0].agent_type == "claude"


def test_record_stores_the_agent():
    with _use_temp_db():
        record(skill="feat", cwd="/tmp", agent_type="codex")
        inv = query()[0]
        assert inv.agent_type == "codex"
        assert inv.agent_label == "OpenAI Codex"


def test_query_filters_by_agent():
    with _use_temp_db():
        record(skill="feat", cwd="/tmp", agent_type="codex")
        record(skill="debug", cwd="/tmp", agent_type="claude")

        assert [i.skill for i in query(agent_type="codex")] == ["feat"]
        assert [i.skill for i in query(agent_type="claude")] == ["debug"]
        assert len(query()) == 2


# ── Stats ──


def test_stats_counts_by_agent():
    with _use_temp_db():
        record(skill="feat", cwd="/tmp", agent_type="codex")
        record(skill="feat", cwd="/tmp", agent_type="codex")
        record(skill="debug", cwd="/tmp", agent_type="claude")

        data = stats()
        assert data["total"] == 3
        # Ordered by count, descending.
        assert data["by_agent"] == [("codex", 2), ("claude", 1)]


def test_stats_filters_by_agent():
    with _use_temp_db():
        record(skill="feat", cwd="/tmp", agent_type="codex")
        record(skill="debug", cwd="/tmp", agent_type="claude")

        data = stats(agent_type="codex")
        assert data["total"] == 1
        assert data["top_skills"] == [("feat", 1)]
        assert data["by_agent"] == [("codex", 1)]


def test_stats_empty_has_no_agents():
    with _use_temp_db():
        assert stats()["by_agent"] == []


# ── Migration of an existing database file ──


def test_migration_adds_agent_type_to_an_old_db():
    path = _legacy_db([("feat", "/tmp"), ("debug", "/tmp")])
    with patch("ashley.history.DB_PATH", path):
        # First access migrates the file in place.
        data = stats()
        assert data["total"] == 2
        # Pre-existing rows are attributed to Claude Code.
        assert data["by_agent"] == [("claude", 2)]
        assert all(i.agent_type == "claude" for i in query())


def test_migration_preserves_existing_rows_and_accepts_new_ones():
    path = _legacy_db([("feat", "/old")])
    with patch("ashley.history.DB_PATH", path):
        record(skill="refactor", cwd="/new", agent_type="codex")

        by_skill = {i.skill: i for i in query()}
        assert by_skill["feat"].agent_type == "claude"
        assert by_skill["feat"].cwd == "/old"
        assert by_skill["refactor"].agent_type == "codex"
        assert stats()["by_agent"] == [("claude", 1), ("codex", 1)]


def test_migrate_reports_added_columns_and_is_idempotent():
    path = _legacy_db([("feat", "/tmp")])
    conn = sqlite3.connect(str(path))
    try:
        assert migrate(conn) == ["agent_type"]
        # Running again is a no-op — nothing left to add.
        assert migrate(conn) == []
    finally:
        conn.close()


def test_migrate_upgrades_a_pre_outcome_database():
    """A very old file missing several columns gains them all at once."""
    tmp = tempfile.NamedTemporaryFile(suffix=".db", delete=False)
    tmp.close()
    conn = sqlite3.connect(tmp.name)
    conn.executescript(
        """
        CREATE TABLE invocations (
            id         INTEGER PRIMARY KEY AUTOINCREMENT,
            timestamp  TEXT NOT NULL,
            skill      TEXT NOT NULL,
            question   TEXT NOT NULL DEFAULT '',
            cwd        TEXT NOT NULL DEFAULT '',
            permission TEXT NOT NULL DEFAULT 'default',
            detached   INTEGER NOT NULL DEFAULT 0,
            session_id TEXT NOT NULL DEFAULT ''
        );
        """
    )
    conn.execute(
        "INSERT INTO invocations (timestamp, skill) VALUES ('2026-01-01T00:00:00', 'feat')"
    )
    conn.commit()
    try:
        assert migrate(conn) == ["exit_code", "duration_s", "outcome", "agent_type"]
    finally:
        conn.close()

    with patch("ashley.history.DB_PATH", Path(tmp.name)):
        inv = query()[0]
        assert inv.outcome == "unknown"
        assert inv.exit_code is None
        assert inv.agent_type == "claude"


# ── Dataclass defaults ──


def test_invocation_defaults_to_claude():
    inv = Invocation(
        id=1,
        timestamp="2026-01-01T00:00:00",
        skill="feat",
        question="",
        cwd="/tmp",
        permission="default",
        detached=False,
        session_id="",
    )
    assert inv.agent_type == "claude"
    assert inv.agent_label == "Claude Code"


# ── TUI rendering ──


def test_stats_screen_renders_the_agent_breakdown():
    """The TUI Stats screen shows a 'By agent' section with agent labels."""
    import asyncio

    import ashley.config as cfg
    import ashley.tui.app as appmod
    from ashley.tui.app import AshleyApp

    path = Path(tempfile.NamedTemporaryFile(suffix=".db", delete=False).name)
    captured: dict[str, str] = {}

    async def scenario():
        with tempfile.TemporaryDirectory() as d, patch("ashley.history.DB_PATH", path):
            for _ in range(3):
                record(skill="feat", cwd="/tmp", agent_type="codex")
            record(skill="debug", cwd="/tmp", agent_type="claude")

            cfg.THEME_PATH = Path(d) / "theme.json"
            cfg.PREFS_PATH = Path(d) / "prefs.json"
            cfg.CONFIG_DIR = Path(d)
            appmod.theme_configured = cfg.theme_configured
            cfg.save_theme("dark", "blue")

            app = AshleyApp()
            async with app.run_test(size=(100, 44)) as pilot:
                await pilot.pause()
                lv = app.screen.query_one("#feature-list")
                lv.index = [f["key"] for f in appmod.FEATURES].index("stats")
                await pilot.pause()
                await pilot.press("enter")
                await pilot.pause()

                body = app.screen.query_one("#stats-body")
                original = body.update
                body.update = lambda text: captured.update(text=text) or original(text)
                app.screen._refresh_stats()
                await pilot.pause()
                captured["exc"] = app._exception

    asyncio.run(scenario())
    assert captured["exc"] is None
    text = captured["text"]
    assert "By agent" in text
    assert "OpenAI Codex" in text
    assert "Claude Code" in text
    # Codex ran more often, so it is listed first.
    assert text.index("OpenAI Codex") < text.index("Claude Code")
