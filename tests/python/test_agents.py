"""Tests for multi-agent support (Claude Code and OpenAI Codex)."""

import tempfile
from pathlib import Path
from unittest.mock import patch

import pytest

from ashley.agents import (
    CLAUDE,
    CODEX,
    get_agent,
    is_agent,
    permission_args,
    select_agent,
    skill_trigger,
    skills_dir,
)

# ── Agent registry ──


def test_get_agent_resolves_keys_and_falls_back():
    assert get_agent("codex") is CODEX
    assert get_agent("CLAUDE") is CLAUDE
    assert get_agent(CODEX) is CODEX
    # Unknown or missing keys fall back to the default backend.
    assert get_agent(None) is CLAUDE
    assert get_agent("gemini") is CLAUDE


def test_is_agent():
    assert is_agent("codex") is True
    assert is_agent("claude") is True
    assert is_agent("gpt") is False
    assert is_agent(None) is False


def test_skills_dir_defaults_and_env_override():
    with patch.dict("os.environ", {}, clear=True):
        assert skills_dir("claude") == Path.home() / ".claude" / "skills"
        assert skills_dir("codex") == Path.home() / ".codex" / "skills"

    with patch.dict("os.environ", {"CODEX_HOME": "/tmp/cx"}, clear=True):
        assert skills_dir("codex") == Path("/tmp/cx/skills")


# ── Run modes → permission flags ──


@pytest.mark.parametrize(
    ("agent", "kwargs", "expected"),
    [
        ("claude", {}, ([], "default")),
        ("claude", {"dsp": True}, (["--dangerously-skip-permissions"], "dsp")),
        ("claude", {"auto": True}, (["--permission-mode", "auto"], "auto")),
        ("claude", {"afk": True}, (["--dangerously-skip-permissions"], "afk")),
        ("codex", {}, ([], "default")),
        (
            "codex",
            {"dsp": True},
            (["--dangerously-bypass-approvals-and-sandbox"], "dsp"),
        ),
        (
            "codex",
            {"auto": True},
            (["--sandbox", "workspace-write", "--ask-for-approval", "never"], "auto"),
        ),
        (
            "codex",
            {"afk": True},
            (["--dangerously-bypass-approvals-and-sandbox"], "afk"),
        ),
    ],
)
def test_permission_args(agent, kwargs, expected):
    assert permission_args(agent, **kwargs) == expected


def test_afk_implies_dsp_over_auto():
    """AFK must skip permissions even when --auto was also given."""
    for agent in ("claude", "codex"):
        flags, mode = permission_args(agent, auto=True, afk=True)
        assert flags == list(get_agent(agent).dsp_flags)
        assert mode == "afk"


# ── Skill triggers ──


def test_skill_trigger_per_agent():
    assert skill_trigger("claude", "feat") == "/a-feat"
    assert skill_trigger("claude", "feat", "add login") == "/a-feat add login"
    assert skill_trigger("codex", "feat") == "$a-feat"
    assert skill_trigger("codex", "feat", "add login") == "$a-feat add login"


# ── -c / -o flag resolution ──


def test_select_agent():
    assert select_agent() is None
    assert select_agent(use_claude=True) == "claude"
    assert select_agent(use_codex=True) == "codex"


def test_select_agent_rejects_both():
    with pytest.raises(ValueError):
        select_agent(use_claude=True, use_codex=True)


# ── Preference persistence ──


def test_agent_preference_roundtrip():
    with tempfile.TemporaryDirectory() as d:
        path = Path(d) / "prefs.json"
        with (
            patch("ashley.config.PREFS_PATH", path),
            patch("ashley.config.CONFIG_DIR", Path(d)),
        ):
            from ashley.config import load_agent, save_agent

            # Defaults to Claude Code before anything is saved.
            assert load_agent() == "claude"
            save_agent("codex")
            assert load_agent() == "codex"


def test_agent_preference_rejects_unknown():
    with tempfile.TemporaryDirectory() as d:
        with (
            patch("ashley.config.PREFS_PATH", Path(d) / "prefs.json"),
            patch("ashley.config.CONFIG_DIR", Path(d)),
        ):
            from ashley.config import save_agent

            with pytest.raises(ValueError):
                save_agent("gemini")


def test_agent_preference_survives_corrupt_file():
    with tempfile.TemporaryDirectory() as d:
        path = Path(d) / "prefs.json"
        path.write_text("{not json")
        with patch("ashley.config.PREFS_PATH", path):
            from ashley.config import load_agent

            assert load_agent() == "claude"


def test_agent_preference_preserves_other_keys():
    """Saving the agent must not drop unrelated preferences."""
    with tempfile.TemporaryDirectory() as d:
        path = Path(d) / "prefs.json"
        path.write_text('{"future_setting": 42}')
        with (
            patch("ashley.config.PREFS_PATH", path),
            patch("ashley.config.CONFIG_DIR", Path(d)),
        ):
            import json

            from ashley.config import save_agent

            save_agent("codex")
            assert json.loads(path.read_text()) == {
                "future_setting": 42,
                "agent": "codex",
            }


@pytest.mark.parametrize(
    "key, directory",
    [("grok", ".grok"), ("opencode", ".config/opencode"), ("kilo", ".kilo")],
)
def test_new_agent_directories_and_selection(key, directory):
    with patch.dict("os.environ", {}, clear=True):
        assert skills_dir(key) == Path.home() / directory / "skills"
    assert select_agent(**{f"use_{key}": True}) == key
    with pytest.raises(ValueError):
        select_agent(use_codex=True, **{f"use_{key}": True})


def test_opencode_xdg_directory():
    with patch.dict("os.environ", {"XDG_CONFIG_HOME": "/tmp/config"}, clear=True):
        assert skills_dir("opencode") == Path("/tmp/config/opencode/skills")
    with patch.dict("os.environ", {"OPENCODE_CONFIG_DIR": "/tmp/custom"}, clear=True):
        assert skills_dir("opencode") == Path("/tmp/custom/skills")
