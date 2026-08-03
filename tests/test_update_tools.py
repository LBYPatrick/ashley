"""Tests for the agent-CLI upgrade step of the Ashley self-updater."""

from unittest.mock import patch

from ashley import update as update_mod
from ashley.update import SKIP_TOOL_ENV, skip_tool_upgrade, tracked_agents

# ── SKIP_TOOL parsing (pure) ──


def test_skip_tool_accepts_the_documented_truthy_values():
    for value in ("true", "1", "yes"):
        assert skip_tool_upgrade({SKIP_TOOL_ENV: value}) is True


def test_skip_tool_is_case_and_whitespace_insensitive():
    assert skip_tool_upgrade({SKIP_TOOL_ENV: " TRUE "}) is True
    assert skip_tool_upgrade({SKIP_TOOL_ENV: "Yes"}) is True


def test_skip_tool_ignores_other_values():
    for value in ("", "false", "0", "no", "maybe"):
        assert skip_tool_upgrade({SKIP_TOOL_ENV: value}) is False


def test_skip_tool_defaults_to_upgrading():
    assert skip_tool_upgrade({}) is False


def test_skip_tool_reads_the_process_environment_by_default():
    with patch.dict("os.environ", {SKIP_TOOL_ENV: "yes"}):
        assert skip_tool_upgrade() is True
    with patch.dict("os.environ", {}, clear=True):
        assert skip_tool_upgrade() is False


# ── Which agents the updater touches ──


def test_tracked_agents_prefers_agents_with_skills_linked():
    """A Codex-only machine must not pull in Claude Code."""
    with (
        patch.object(update_mod, "has_skills", lambda key: key == "codex"),
        patch.object(update_mod, "load_agent", lambda: "claude"),
    ):
        assert tracked_agents() == ["codex"]


def test_tracked_agents_covers_every_linked_agent():
    with patch.object(update_mod, "has_skills", lambda key: True):
        assert tracked_agents() == ["claude", "codex"]


def test_tracked_agents_falls_back_to_the_saved_default():
    with (
        patch.object(update_mod, "has_skills", lambda key: False),
        patch.object(update_mod, "load_agent", lambda: "codex"),
    ):
        assert tracked_agents() == ["codex"]


# ── The step is wired into update() ──


def _updater(upgraded: list[list[str]], changed: bool = False):
    """Patch update()'s side effects, recording agent upgrades."""
    hashes = iter(("before", "after" if changed else "before"))

    def fake_upgrade(agents):
        upgraded.append(list(agents))
        return 0

    return (
        patch.object(update_mod.subprocess, "run", lambda *a, **k: _ok()),
        patch.object(update_mod, "get_generated_hash", lambda: next(hashes)),
        patch.object(update_mod, "upgrade_agents", fake_upgrade),
        patch.object(update_mod, "tracked_agents", lambda: ["codex"]),
    )


class _ok:
    """Stand-in for a successful CompletedProcess."""

    returncode = 0


def test_update_upgrades_the_agent_clis_by_default():
    upgraded: list[list[str]] = []
    with_run, with_hash, with_upgrade, with_tracked = _updater(upgraded)
    with with_run, with_hash, with_upgrade, with_tracked:
        update_mod.update("main")
    assert upgraded == [["codex"]]


def test_update_skips_the_upgrade_when_asked(capsys):
    upgraded: list[list[str]] = []
    with_run, with_hash, with_upgrade, with_tracked = _updater(upgraded)
    with with_run, with_hash, with_upgrade, with_tracked:
        update_mod.update("main", upgrade_tools=False)
    assert upgraded == []
    assert SKIP_TOOL_ENV in capsys.readouterr().out


def test_update_upgrades_even_when_no_skills_changed(capsys):
    """The unchanged-skills path used to return early — it must not."""
    upgraded: list[list[str]] = []
    with_run, with_hash, with_upgrade, with_tracked = _updater(upgraded, changed=False)
    with with_run, with_hash, with_upgrade, with_tracked:
        update_mod.update("main")
    out = capsys.readouterr().out
    assert "Skipping reinstall" in out
    assert upgraded == [["codex"]]


def test_update_upgrades_after_reinstalling_changed_skills(capsys):
    upgraded: list[list[str]] = []
    with_run, with_hash, with_upgrade, with_tracked = _updater(upgraded, changed=True)
    with with_run, with_hash, with_upgrade, with_tracked:
        update_mod.update("main")
    out = capsys.readouterr().out
    assert "re-installing skills" in out
    assert upgraded == [["codex"]]


def test_update_survives_a_failed_agent_upgrade(capsys):
    """Ashley is already current, so a CLI upgrade failure is a warning."""
    hashes = iter(("same", "same"))
    with (
        patch.object(update_mod.subprocess, "run", lambda *a, **k: _ok()),
        patch.object(update_mod, "get_generated_hash", lambda: next(hashes)),
        patch.object(update_mod, "upgrade_agents", lambda agents: 1),
        patch.object(update_mod, "tracked_agents", lambda: ["codex"]),
    ):
        update_mod.update("main")
    out = capsys.readouterr().out
    assert "could not be upgraded" in out
    assert "Update complete" in out
