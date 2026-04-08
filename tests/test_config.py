"""Tests for Ashley configuration system."""

import tempfile
from pathlib import Path
from unittest.mock import patch

from ashley.config import (
    AshleyConfig,
    HookConfig,
    _parse_hook_config,
    _parse_hook_entry,
    get_hooks_for_skill,
    load_config,
)


def test_parse_hook_entry_string():
    assert _parse_hook_entry("make test") == ["make test"]


def test_parse_hook_entry_list():
    assert _parse_hook_entry(["a", "b"]) == ["a", "b"]


def test_parse_hook_entry_none():
    assert _parse_hook_entry(None) == []


def test_parse_hook_config_empty():
    result = _parse_hook_config(None)
    assert result.before_run == []
    assert result.after_run == []
    assert result.on_error == []


def test_parse_hook_config_full():
    data = {
        "before_run": "echo before",
        "after_run": ["echo after", "echo done"],
        "on_error": "echo fail",
    }
    result = _parse_hook_config(data)
    assert result.before_run == ["echo before"]
    assert result.after_run == ["echo after", "echo done"]
    assert result.on_error == ["echo fail"]


def test_get_hooks_for_skill_global_only():
    config = AshleyConfig(
        global_hooks=HookConfig(before_run=["echo global"]),
    )
    hooks = get_hooks_for_skill(config, "feat")
    assert hooks.before_run == ["echo global"]


def test_get_hooks_for_skill_override():
    config = AshleyConfig(
        global_hooks=HookConfig(
            before_run=["echo global"],
            after_run=["echo global-after"],
        ),
        skill_hooks={
            "feat": HookConfig(before_run=["echo feat-before"]),
        },
    )
    hooks = get_hooks_for_skill(config, "feat")
    # before_run overridden by skill
    assert hooks.before_run == ["echo feat-before"]
    # after_run falls back to global
    assert hooks.after_run == ["echo global-after"]


def test_load_config_missing_file():
    with patch("ashley.config.CONFIG_PATH", Path("/nonexistent/config.yaml")):
        config = load_config()
        assert config.permission_mode == "default"
        assert config.pipelines == {}


def test_load_config_valid_yaml():
    yaml_content = """\
defaults:
  permission_mode: auto
hooks:
  global:
    before_run: "echo hi"
  skills:
    feat:
      after_run: "make format"
pipelines:
  ship:
    - feat
    - commit
"""
    with tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False) as f:
        f.write(yaml_content)
        f.flush()
        with patch("ashley.config.CONFIG_PATH", Path(f.name)):
            config = load_config()
            assert config.permission_mode == "auto"
            assert config.global_hooks.before_run == ["echo hi"]
            assert config.skill_hooks["feat"].after_run == ["make format"]
            assert config.pipelines["ship"] == ["feat", "commit"]
