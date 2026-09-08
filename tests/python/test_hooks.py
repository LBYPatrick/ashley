"""Tests for Ashley hook runner."""

import tempfile
from pathlib import Path

from ashley.config import HookConfig
from ashley.hooks import run_before_hooks, run_hooks


def test_run_hooks_empty():
    assert run_hooks([], cwd="/tmp", skill="test") is True


def test_run_hooks_success():
    assert run_hooks(["true"], cwd="/tmp", skill="test") is True


def test_run_hooks_failure():
    assert run_hooks(["false"], cwd="/tmp", skill="test") is False


def test_run_hooks_env_variables():
    """Verify environment variables are passed to hooks."""
    with tempfile.NamedTemporaryFile(mode="w", suffix=".sh", delete=False) as f:
        f.write("#!/bin/bash\necho $ASHLEY_SKILL > /tmp/ashley_hook_test.txt\n")
        f.flush()
        result = run_hooks(
            [f"bash {f.name}"],
            cwd="/tmp",
            skill="test-skill",
        )
        assert result is True
        output = Path("/tmp/ashley_hook_test.txt").read_text().strip()
        assert output == "test-skill"
        Path("/tmp/ashley_hook_test.txt").unlink(missing_ok=True)


def test_run_hooks_stops_on_failure():
    """Verify hooks stop at first failure."""
    with tempfile.NamedTemporaryFile(mode="w", suffix=".txt", delete=False) as f:
        marker = f.name

    # First command succeeds and creates marker, second fails, third should not run
    result = run_hooks(
        ["true", "false", f"echo third > {marker}"],
        cwd="/tmp",
        skill="test",
    )
    assert result is False
    content = Path(marker).read_text()
    assert "third" not in content
    Path(marker).unlink(missing_ok=True)


def test_before_hooks_abort():
    hooks = HookConfig(before_run=["false"])
    assert run_before_hooks(hooks, "feat", "test", "/tmp") is False


def test_before_hooks_pass():
    hooks = HookConfig(before_run=["true"])
    assert run_before_hooks(hooks, "feat", "test", "/tmp") is True
