"""Tests for agent selection during skill installation."""

from unittest.mock import patch

from ashley import install as install_mod
from ashley.install import parse_argv, resolve_agents

# ── Argument parsing for `python -m ashley.install` ──


def test_parse_argv_defaults_to_install_and_no_choice():
    assert parse_argv([]) == ("install", None)


def test_parse_argv_uninstall():
    assert parse_argv(["uninstall"]) == ("uninstall", None)


def test_parse_argv_agent_forms():
    assert parse_argv(["--codex"]) == ("install", ["codex"])
    assert parse_argv(["--agent", "codex"]) == ("install", ["codex"])
    assert parse_argv(["--agent=codex"]) == ("install", ["codex"])


def test_parse_argv_both_aliases():
    assert parse_argv(["--both"]) == ("install", ["claude", "codex"])
    assert parse_argv(["--agent", "both"]) == ("install", ["claude", "codex"])


def test_parse_argv_dedupes():
    assert parse_argv(["--claude", "--claude", "--codex"]) == (
        "install",
        ["claude", "codex"],
    )


def test_parse_argv_ignores_unknown_flags():
    assert parse_argv(["--verbose"]) == ("install", None)


# ── Agent resolution ──


def test_resolve_agents_honours_explicit_keys():
    assert resolve_agents(["codex"]) == ["codex"]


def test_resolve_agents_drops_unknown_keys():
    assert resolve_agents(["gemini"]) == []


def test_resolve_agents_keeps_saved_default_first():
    """A reinstall covering both agents must not switch the default."""
    with patch.object(install_mod, "load_agent", lambda: "codex"):
        assert resolve_agents(["claude", "codex"]) == ["codex", "claude"]


def test_resolve_agents_reuses_existing_install_without_prompting():
    with (
        patch.object(install_mod, "has_skills", lambda key: key == "codex"),
        patch.object(install_mod, "prompt_for_agents", _fail_if_called),
        patch.object(install_mod, "load_agent", lambda: "claude"),
    ):
        assert resolve_agents(None) == ["codex"]


def test_resolve_agents_prompts_on_a_fresh_machine():
    with (
        patch.object(install_mod, "has_skills", lambda key: False),
        patch.object(install_mod, "prompt_for_agents", lambda: ["codex"]),
    ):
        assert resolve_agents(None) == ["codex"]


def _fail_if_called():
    raise AssertionError("should not prompt when skills are already installed")


# ── The first-install question ──


def _answer(reply: str):
    """Patch the terminal read with a canned reply."""
    return patch.object(install_mod, "_read_choice", lambda _prompt: reply)


def test_prompt_accepts_numeric_choice():
    with _answer("2"), patch.object(install_mod, "load_agent", lambda: "claude"):
        assert install_mod.prompt_for_agents() == ["codex"]


def test_prompt_accepts_agent_name():
    with _answer("codex"), patch.object(install_mod, "load_agent", lambda: "claude"):
        assert install_mod.prompt_for_agents() == ["codex"]


def test_prompt_both_puts_the_default_first():
    with _answer("both"), patch.object(install_mod, "load_agent", lambda: "codex"):
        assert install_mod.prompt_for_agents() == ["codex", "claude"]


def test_prompt_falls_back_to_the_saved_default():
    """An empty answer (or no terminal at all) keeps the saved preference."""
    with _answer(""), patch.object(install_mod, "load_agent", lambda: "codex"):
        assert install_mod.prompt_for_agents() == ["codex"]


def test_parse_new_agents_and_all():
    from ashley.agents import AGENT_KEYS

    assert parse_argv(["--grok", "--opencode", "--kilo"]) == (
        "install",
        ["grok", "opencode", "kilo"],
    )
    assert parse_argv(["--all"]) == ("install", list(AGENT_KEYS))


def test_all_prompt_preserves_saved_default():
    with _answer("6"), patch.object(install_mod, "load_agent", lambda: "kilo"):
        assert install_mod.prompt_for_agents() == [
            "kilo",
            "claude",
            "codex",
            "grok",
            "opencode",
        ]


def test_cli_install_new_agents():
    from click.testing import CliRunner

    from ashley.cli import main

    with patch("ashley.cli.do_generate"), patch("ashley.cli.do_install") as install:
        result = CliRunner().invoke(main, ["install", "--grok", "--opencode", "--kilo"])
        assert result.exit_code == 0, result.output
        install.assert_called_once_with(["grok", "opencode", "kilo"])


def test_cli_pipeline_selects_new_backend_and_rejects_conflict():
    from click.testing import CliRunner

    from ashley.cli import main

    with patch("ashley.pipeline.run_pipeline", return_value=0) as pipeline:
        result = CliRunner().invoke(main, ["pipe", "--kilo", "feat+commit", "task"])
        assert result.exit_code == 0, result.output
        assert pipeline.call_args.kwargs["agent"] == "kilo"
        result = CliRunner().invoke(main, ["pipe", "--kilo", "--grok", "feat+commit"])
        assert result.exit_code == 1
        assert "Choose only one" in result.output
