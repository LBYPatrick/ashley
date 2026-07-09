"""Tests for Ashley history with outcome tracking."""

import tempfile
from pathlib import Path
from unittest.mock import patch

from ashley.history import clear, record, record_outcome, stats


def _use_temp_db():
    """Patch the DB path to a temp file for testing."""
    tmp = tempfile.NamedTemporaryFile(suffix=".db", delete=False)
    tmp.close()
    return patch("ashley.history.DB_PATH", Path(tmp.name))


def test_record_and_outcome():
    with _use_temp_db():
        inv_id = record(skill="feat", question="add login", cwd="/tmp")
        assert inv_id > 0
        record_outcome(inv_id, exit_code=0, duration_s=45.2)

        data = stats()
        assert data["total"] == 1
        assert data["top_skills"] == [("feat", 1)]
        clear()


def test_stats_empty():
    with _use_temp_db():
        data = stats()
        assert data["total"] == 0
        assert data["top_skills"] == []
        # Run-duration and success-rate tracking have been removed.
        assert "avg_duration" not in data
        assert "skill_rates" not in data


def test_stats_multiple_skills():
    with _use_temp_db():
        id1 = record(skill="feat", cwd="/tmp")
        record_outcome(id1, exit_code=0, duration_s=30)

        id2 = record(skill="feat", cwd="/tmp")
        record_outcome(id2, exit_code=1, duration_s=10)

        id3 = record(skill="debug", cwd="/tmp")
        record_outcome(id3, exit_code=0, duration_s=60)

        data = stats()
        assert data["total"] == 3

        # Top skills by usage
        assert data["top_skills"][0][0] == "feat"
        assert data["top_skills"][0][1] == 2

        # Success-rate tracking has been removed.
        assert "skill_rates" not in data
        assert "success" not in data

        clear()
