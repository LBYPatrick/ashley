"""Tests for building agent invocations and their tmux shell commands."""

import tempfile
from pathlib import Path
from unittest.mock import patch

import pytest

from ashley.cli import PROMPT_FILE_MARKER, build_agent_invocation
from ashley.sessions import build_shell_command


def _install_skill(root: Path, name: str = "a-feat") -> None:
    """Create a fake installed skill inside *root*."""
    skill = root / name
    skill.mkdir(parents=True)
    (skill / "SKILL.md").write_text("---\nname: a-feat\n---\nbody\n")


def _build(agent: str, skills_root: Path, **kwargs):
    """Build an invocation with the agent binary and skills dir stubbed."""
    defaults = {
        "skill": "feat",
        "question_str": "add login",
        "dangerously_skip_permissions": False,
        "auto_mode": False,
        "away_from_keyboard": False,
    }
    defaults.update(kwargs)
    with (
        patch("ashley.cli.shutil.which", lambda binary: f"/usr/bin/{binary}"),
        patch("ashley.cli.skills_dir", lambda _agent: skills_root),
    ):
        return build_agent_invocation(agent=agent, **defaults)


@pytest.fixture
def skills_root():
    with tempfile.TemporaryDirectory() as d:
        yield Path(d)


# ── Installed skills use the agent's native trigger ──


def test_claude_installed_skill_uses_slash_command(skills_root):
    _install_skill(skills_root)
    args, mode = _build("claude", skills_root)
    assert args == ["/usr/bin/claude", "/a-feat add login"]
    assert mode == "default"


def test_codex_installed_skill_uses_dollar_mention(skills_root):
    _install_skill(skills_root)
    args, mode = _build("codex", skills_root)
    assert args == ["/usr/bin/codex", "$a-feat add login"]
    assert mode == "default"


# ── Permission modes ──


def test_codex_dsp_and_auto_flags(skills_root):
    _install_skill(skills_root)
    args, mode = _build("codex", skills_root, dangerously_skip_permissions=True)
    assert args[1] == "--dangerously-bypass-approvals-and-sandbox"
    assert mode == "dsp"

    args, mode = _build("codex", skills_root, auto_mode=True)
    assert args[1:5] == ["--sandbox", "workspace-write", "--ask-for-approval", "never"]
    assert mode == "auto"


def test_claude_afk_appends_addendum_as_system_prompt(skills_root):
    _install_skill(skills_root)
    args, mode = _build("claude", skills_root, away_from_keyboard=True)
    assert mode == "afk"
    assert args[1] == "--dangerously-skip-permissions"
    assert args[2] == "--append-system-prompt"
    assert "AFK Mode" in args[3]
    assert args[4] == "/a-feat add login"


def test_codex_afk_folds_addendum_into_the_prompt(skills_root):
    """Codex has no system-prompt flag, so instructions ride in the prompt."""
    _install_skill(skills_root)
    args, mode = _build("codex", skills_root, away_from_keyboard=True)
    assert mode == "afk"
    assert args[1] == "--dangerously-bypass-approvals-and-sandbox"
    assert len(args) == 3
    assert "AFK Mode" in args[2]
    assert args[2].endswith("$a-feat add login")


# ── Skills that are not installed get the prompt inlined ──


def test_uninstalled_skill_inlines_prompt(skills_root):
    claude_args, _ = _build("claude", skills_root)
    assert claude_args[1] == "--append-system-prompt"
    assert "Workflow" in claude_args[2]
    assert claude_args[3] == "add login"

    codex_args, _ = _build("codex", skills_root)
    # One combined prompt argument — no system-prompt flag for Codex.
    assert len(codex_args) == 2
    assert "Workflow" in codex_args[1]
    assert codex_args[1].endswith("add login")


def test_raw_skill_launches_agent_bare(skills_root):
    args, _ = _build("codex", skills_root, skill="raw", question_str="")
    assert args == ["/usr/bin/codex"]


# ── Large prompts spill to a temp file in detached mode ──


def test_detached_large_prompt_becomes_marker(skills_root):
    args, _ = _build("codex", skills_root, detached=True)
    marker = args[1]
    assert marker.startswith(PROMPT_FILE_MARKER)

    path = Path(marker[len(PROMPT_FILE_MARKER) :])
    try:
        assert "Workflow" in path.read_text()
    finally:
        path.unlink(missing_ok=True)


def test_short_prompt_stays_inline_when_detached(skills_root):
    _install_skill(skills_root)
    args, _ = _build("claude", skills_root, detached=True)
    assert args == ["/usr/bin/claude", "/a-feat add login"]


# ── tmux shell rendering ──


def test_build_shell_command_expands_prompt_marker():
    cmd, files = build_shell_command(
        ["/usr/bin/codex", f"{PROMPT_FILE_MARKER}/tmp/p.md"],
        "/work/dir",
    )
    assert files == ["/tmp/p.md"]
    assert "cd '/work/dir' && '/usr/bin/codex' \"$(cat '/tmp/p.md')\"" in cmd
    assert "rm -f '/tmp/p.md';" in cmd


def test_build_shell_command_quotes_arguments():
    cmd, files = build_shell_command(
        ["/usr/bin/claude", "it's fine"],
        "/work/dir",
    )
    assert files == []
    assert """'it'\\''s fine'""" in cmd


@pytest.mark.parametrize("agent", ["grok", "opencode", "kilo"])
def test_new_agent_invocation_and_permissions(agent, skills_root):
    _install_skill(skills_root)
    args, mode = _build(agent, skills_root, away_from_keyboard=True)
    assert args[0] == f"/usr/bin/{agent}"
    assert mode == "afk"
    assert "AFK Mode" in args[-1]
    assert "a-feat" in args[-1]
    if agent == "grok":
        assert args[1] == "--always-approve"
    else:
        assert args[1:3] == ["--auto", "--prompt"]
    assert _build(agent, skills_root, skill="raw", question_str="")[0] == [
        f"/usr/bin/{agent}"
    ]
    fallback, _ = _build(agent, skills_root / "missing")
    assert "Workflow" in fallback[-1]
    assert fallback[-1].endswith("add login")
