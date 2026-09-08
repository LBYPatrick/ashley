"""Migrate real Python launchers using a compiled binary and isolated user data."""

import os
import platform
import shutil
import sqlite3
import subprocess
from pathlib import Path

from test_binary import ROOT, VERSION, installer_env


def legacy_install(tmp_path):
    home = tmp_path / "home with spaces"
    source = home / ".ashley/repo"
    (source / "bin").mkdir(parents=True)
    launcher = source / "bin/ash"
    launcher.write_text("#!/bin/bash\nexit 99 # old runtime must never run\n")
    launcher.chmod(0o755)
    (source / "skills").mkdir()
    (source / "skills/custom.jsonc").write_text(
        '{"name":"a-custom","description":"Old custom skill",'
        '"components":["components/custom.md"],"resources":[],"output":"generated/a-custom/SKILL.md"}'
    )
    (source / "components").mkdir()
    (source / "components/custom.md").write_text("Preserve my workflow.")
    package = source / "generated/a-custom"
    package.mkdir(parents=True)
    (package / "SKILL.md").write_text("My edited custom prompt.")
    (package / "helper.sh").write_text("#!/bin/sh\necho helper\n")
    (package / "helper.sh").chmod(0o755)
    install_dir = home / ".local/bin"
    install_dir.mkdir(parents=True)
    (install_dir / "ash").symlink_to(Path("../../.ashley/repo/bin/ash"))
    agent = home / ".codex/skills"
    agent.mkdir(parents=True)
    (agent / "a-custom").symlink_to(package)
    (home / ".ashley/prefs.json").write_text('{"agent":"codex"}')
    (home / ".ashley/config.yaml").write_text("custom: retained\n")
    db = history_path(home)
    db.parent.mkdir(parents=True, exist_ok=True)
    with sqlite3.connect(db) as connection:
        connection.executescript("""
        CREATE TABLE invocations (
            id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp TEXT NOT NULL,
            skill TEXT NOT NULL, question TEXT NOT NULL DEFAULT '',
            cwd TEXT NOT NULL DEFAULT '', permission TEXT NOT NULL DEFAULT 'default',
            detached INTEGER NOT NULL DEFAULT 0, session_id TEXT NOT NULL DEFAULT ''
        );
        INSERT INTO invocations(timestamp, skill, question)
        VALUES ('2024-01-01T00:00:00+00:00', 'feat', 'Old session retained');
        """)
    return home, source, install_dir


def history_path(home):
    if platform.system() == "Darwin":
        return home / "Library/Application Support/ashley/history.db"
    return home / ".local/share/ashley/history.db"


def environment(home):
    env = dict(os.environ)
    for key in (
        "CODEX_HOME",
        "CLAUDE_CONFIG_DIR",
        "GROK_HOME",
        "OPENCODE_CONFIG_DIR",
        "ASHLEY_DIR",
        "ASHLEY_VERSION",
        "ASHLEY_INSTALL_DIR",
    ):
        env.pop(key, None)
    env.update(
        HOME=str(home),
        PATH="/usr/bin:/bin",
        XDG_CONFIG_HOME=str(home / ".config"),
        XDG_DATA_HOME=str(home / ".local/share"),
    )
    return env


def test_migrate_python_launcher_and_custom_skills(binary, tmp_path):
    home, source, install_dir = legacy_install(tmp_path)
    env = environment(home)
    command = ["bash", str(ROOT / "scripts/migrate-python.sh"), "--binary", str(binary)]
    original_history = history_path(home).read_bytes()
    first = subprocess.run(command, env=env, capture_output=True, text=True)
    assert first.returncode == 0, first.stdout + first.stderr
    assert not (install_dir / "ash").is_symlink()
    assert (install_dir / "ash").read_bytes() == binary.read_bytes()
    backups = list((home / ".ashley/migrations").glob("python-to-go-*"))
    assert len(backups) == 1
    assert (backups[0] / "ash").is_symlink()
    assert os.readlink(backups[0] / "ash") == "../../.ashley/repo/bin/ash"
    assert "exit 99" in (source / "bin/ash").read_text()
    assert history_path(home).read_bytes() == original_history
    history = subprocess.run(
        [str(install_dir / "ash"), "history", "show"],
        env=env,
        capture_output=True,
        text=True,
    )
    assert history.returncode == 0, history.stderr
    assert "Old session retained" in history.stdout
    assert (home / ".ashley/config.yaml").read_text() == "custom: retained\n"
    helper = home / ".codex/skills/a-custom/helper.sh"
    assert helper.is_file() and os.access(helper, os.X_OK)
    assert helper.resolve().is_relative_to(home / ".ashley/generated")
    # After removing the checkout, both definitions and supporting files work.
    shutil.rmtree(source)
    result = subprocess.run(
        [str(install_dir / "ash"), "prompt", "custom", "task"],
        env=env,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stderr
    assert "Preserve my workflow" in result.stdout
    assert (helper.parent / "SKILL.md").read_text() == "My edited custom prompt."
    (home / ".ashley/components/custom.md").write_text("Newer local edit")
    second = subprocess.run(command, env=env, capture_output=True, text=True)
    assert second.returncode == 0, second.stdout + second.stderr
    assert (home / ".ashley/components/custom.md").read_text() == "Newer local edit"


def test_migration_download_failure_keeps_python_launcher(release_archive, tmp_path):
    home, _, install_dir = legacy_install(tmp_path)
    release_archive.write_bytes(b"bad archive")
    env = {**installer_env(tmp_path, release_archive), **environment(home)}
    env["PATH"] = f"{tmp_path / 'tools'}:/usr/bin:/bin"
    result = subprocess.run(
        ["bash", str(ROOT / "scripts/migrate-python.sh"), "--version", VERSION],
        env=env,
        capture_output=True,
        text=True,
    )
    assert result.returncode != 0
    assert (install_dir / "ash").is_symlink()
    assert not (home / ".ashley/migrations").exists()


def test_migration_verified_release_download(release_archive, tmp_path):
    home, _, install_dir = legacy_install(tmp_path)
    env = {**installer_env(tmp_path, release_archive), **environment(home)}
    env["PATH"] = f"{tmp_path / 'tools'}:/usr/bin:/bin"
    result = subprocess.run(
        ["bash", str(ROOT / "scripts/migrate-python.sh"), "--version", VERSION],
        env=env,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    assert not (install_dir / "ash").is_symlink()


def test_migration_rejects_script_as_binary(tmp_path):
    home, source, install_dir = legacy_install(tmp_path)
    result = subprocess.run(
        [
            "bash",
            str(ROOT / "scripts/migrate-python.sh"),
            "--binary",
            str(source / "bin/ash"),
        ],
        env=environment(home),
        capture_output=True,
        text=True,
    )
    assert result.returncode != 0
    assert "native Ashley release binary" in result.stderr
    assert (install_dir / "ash").is_symlink()


def test_migration_setup_failure_keeps_old_launcher(binary, tmp_path):
    home, source, install_dir = legacy_install(tmp_path)
    (source / "skills/custom.jsonc").write_text("invalid JSON")
    result = subprocess.run(
        ["bash", str(ROOT / "scripts/migrate-python.sh"), "--binary", str(binary)],
        env=environment(home),
        capture_output=True,
        text=True,
    )
    assert result.returncode != 0
    assert (install_dir / "ash").is_symlink()
    assert list((home / ".ashley/migrations").glob("python-to-go-*/ash"))


def test_migration_regular_launcher_explicit_source_and_existing_overrides(
    binary, tmp_path
):
    home, source, install_dir = legacy_install(tmp_path)
    custom_source = tmp_path / "my skills checkout"
    source.rename(custom_source)
    (install_dir / "ash").unlink()
    (install_dir / "ash").write_text("old Python entrypoint")
    agent_link = home / ".codex/skills/a-custom"
    agent_link.unlink()
    agent_link.symlink_to(custom_source / "generated/a-custom")
    components = home / ".ashley/components"
    components.mkdir()
    (components / "custom.md").write_text("Existing user override")
    result = subprocess.run(
        [
            "bash",
            str(ROOT / "scripts/migrate-python.sh"),
            "--binary",
            str(binary),
            "--source",
            str(custom_source),
        ],
        env=environment(home),
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    assert (install_dir / "ash").read_bytes() == binary.read_bytes()
    assert (components / "custom.md").read_text() == "Existing user override"
    assert agent_link.resolve().is_relative_to(home / ".ashley/generated")
    assert any(
        path.read_text() == "old Python entrypoint"
        for path in (home / ".ashley/migrations").glob("python-to-go-*/ash")
    )
