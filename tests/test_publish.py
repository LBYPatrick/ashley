"""Exercise maintainer publishing against local command doubles, never GitHub."""

import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]


@pytest.fixture
def publisher(tmp_path):
    for name in (
        "scripts/publish.sh",
        "scripts/release.py",
        "VERSION",
        "README.md",
        "pyproject.toml",
        "docs/go-migration-status.json",
    ):
        dest = tmp_path / name
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / name, dest)
    # Publishing refusal cases must not depend on the live migration checklist.
    status_path = tmp_path / "docs/go-migration-status.json"
    status = json.loads(status_path.read_text())
    status_path.write_text(json.dumps(dict.fromkeys(status, False)))
    tools = tmp_path / "tools"
    tools.mkdir()
    runner = tools / "runner"
    runner.write_text(
        "#!"
        + sys.executable
        + "\n"
        + """
import json, os, subprocess, sys
from pathlib import Path
name = Path(sys.argv[0]).name
args = sys.argv[1:]
with open(os.environ["COMMAND_LOG"], "a") as log:
    log.write(json.dumps([name, *args]) + "\\n")
if name == "git":
    if args[:2] == ["branch", "--show-current"]: print(os.environ.get("TEST_BRANCH", "main"))
    elif args[:2] == ["status", "--porcelain"]: print(os.environ.get("STRAY", ""))
    elif args[:2] == ["rev-list", "--count"]: print("0")
    elif args[0] == "show-ref": sys.exit(1)
    elif args[0] == "rev-parse": print("abc123")
    elif args[0] == "diff": sys.exit(1)
elif name == "gh":
    if args[:2] == ["repo", "view"]: print("https://github.com/example/ashley")
elif name == "uv":
    sys.exit(subprocess.call([sys.executable, *args[2:]]))
elif name == "make":
    sys.exit(int(os.environ.get("FAIL_GATE", "0")))
"""
    )
    runner.chmod(0o755)
    for name in ("git", "gh", "uv", "make"):
        (tools / name).symlink_to(runner)
    log = tmp_path / "commands.jsonl"
    env = {
        **os.environ,
        "PATH": f"{tools}:{os.environ['PATH']}",
        "V": "0.4.0-alpha.1",
        "YES": "1",
        "COMMAND_LOG": str(log),
    }
    return tmp_path, env, log


def run_publish(publisher, **overrides):
    root, env, log = publisher
    result = subprocess.run(
        ["bash", str(root / "scripts/publish.sh")],
        env={**env, **overrides},
        capture_output=True,
        text=True,
    )
    import json

    commands = (
        [json.loads(line) for line in log.read_text().splitlines()]
        if log.exists()
        else []
    )
    return result, commands


def test_publish_runs_gate_before_push_and_preserves_notes(publisher):
    root, _, _ = publisher
    notes = root / "notes.md"
    notes.write_text("# Notes\n\nLiteral `code` and $HOME.\n")
    result, commands = run_publish(publisher, NOTES=str(notes))
    assert result.returncode == 0, result.stderr
    gate = commands.index(["make", "gate"])
    push = commands.index(["git", "push", "origin", "main"])
    tag = commands.index(["git", "push", "origin", "v0.4.0-alpha.1"])
    assert gate < push < tag
    create = next(
        command for command in commands if command[:3] == ["gh", "release", "create"]
    )
    assert "--draft" in create and "--prerelease" in create and "--verify-tag" in create
    assert create[create.index("--notes-file") + 1] == str(notes)


@pytest.mark.parametrize(
    "overrides",
    [
        {"FAIL_GATE": "1"},
        {"TEST_BRANCH": "feat/exp-go"},
        {"STRAY": " M unrelated.py"},
        {"V": "1.0.0"},
        {"NOTES": "/nonexistent-notes"},
    ],
)
def test_publish_failure_never_pushes(publisher, overrides):
    result, commands = run_publish(publisher, **overrides)
    assert result.returncode != 0
    assert not any(command[:2] == ["git", "push"] for command in commands)
    assert not any(command[:3] == ["gh", "release", "create"] for command in commands)


def test_publish_preview_stops_before_commit_and_push(publisher):
    result, commands = run_publish(publisher, YES="")
    assert result.returncode == 0, result.stderr
    assert ["make", "gate"] in commands
    assert not any(
        command[:2] in (["git", "push"], ["git", "commit"]) for command in commands
    )
