"""Exercise real release binaries and the installer without public downloads."""

import hashlib
import os
import platform
import shutil
import subprocess
import tarfile
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
VERSION = (ROOT / "VERSION").read_text().strip()


def test_binary_runs_outside_checkout_without_language_runtimes(binary, tmp_path):
    standalone = tmp_path / "ash"
    shutil.copy2(binary, standalone)
    env = {"HOME": str(tmp_path), "PATH": "", "GOPROXY": "off"}
    for args in (
        ("--version",),
        ("list",),
        ("prompt", "feat", "a task"),
        ("generate", "--output", str(tmp_path / "output")),
        ("install", "--all", "--skills-only"),
    ):
        result = subprocess.run(
            [str(standalone), *args],
            cwd=tmp_path,
            env=env,
            capture_output=True,
            text=True,
        )
        assert result.returncode == 0, result.stderr
    assert (tmp_path / "output/generated/a-feat/SKILL.md").is_file()
    assert not (tmp_path / ".venv").exists()
    for directory in (".claude", ".codex", ".grok", ".config/opencode", ".kilo"):
        document = tmp_path / directory / "skills/a-feat/SKILL.md"
        assert document.is_file()
        assert document.resolve().is_relative_to(tmp_path / ".ashley/generated")


def installer_env(tmp_path, release_archive):
    tools = tmp_path / "tools"
    tools.mkdir()
    curl = tools / "curl"
    # Route only installer downloads to the local archive. Unexpected network
    # requests fail rather than reaching GitHub.
    curl.write_text("""#!/bin/bash
set -euo pipefail
url=""; output=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        -o) output="$2"; shift 2 ;;
        --retry) shift 2 ;;
        -fsSL) shift ;;
        https://github.com/*/releases/download/*) url="$1"; shift ;;
        https://raw.githubusercontent.com/*/main/scripts/install.sh) url="$1"; shift ;;
        *) exit 90 ;;
    esac
done
if [[ "$url" == https://raw.githubusercontent.com/* ]]; then
    cp "$INSTALLER_FIXTURE" "$output"
else
    cp "$FIXTURE_DIR/${url##*/}" "$output"
fi
""")
    curl.chmod(0o755)
    return {
        **os.environ,
        "PATH": f"{tools}:{os.environ['PATH']}",
        "FIXTURE_DIR": str(release_archive.parent),
        "INSTALLER_FIXTURE": str(ROOT / "scripts/install.sh"),
    }


def test_install_verified_binary(release_archive, tmp_path):
    destination = tmp_path / "installed"
    result = subprocess.run(
        [
            "bash",
            str(ROOT / "scripts/install.sh"),
            "--version",
            VERSION,
            "--install-dir",
            str(destination),
        ],
        env=installer_env(tmp_path, release_archive),
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stderr
    assert (
        subprocess.check_output(
            [str(destination / "ash"), "--version"], text=True
        ).strip()
        == f"ashley {VERSION}"
    )
    assert sorted(p.name for p in destination.iterdir()) == ["ash"]


def test_corrupt_download_keeps_existing_install(release_archive, tmp_path):
    release_archive.write_bytes(b"corrupt download")
    destination = tmp_path / "installed"
    destination.mkdir()
    old = destination / "ash"
    old.write_text("existing executable")
    result = subprocess.run(
        [
            "bash",
            str(ROOT / "scripts/install.sh"),
            "--version",
            VERSION,
            "--install-dir",
            str(destination),
        ],
        env=installer_env(tmp_path, release_archive),
        capture_output=True,
        text=True,
    )
    assert result.returncode != 0
    assert old.read_text() == "existing executable"


def test_package_contains_only_binary_and_license(binary):
    subprocess.run(
        ["bash", str(ROOT / "scripts/release/package.sh")],
        cwd=ROOT,
        check=True,
        capture_output=True,
    )
    target_os = "darwin" if platform.system() == "Darwin" else "linux"
    arch = "arm64" if platform.machine() in ("arm64", "aarch64") else "amd64"
    archive = ROOT / f"dist/ashley-{VERSION}-{target_os}-{arch}.tar.gz"
    with tarfile.open(archive) as package:
        assert sorted(package.getnames()) == ["LICENSE", "ash"]
        assert package.extractfile("ash").read(4) != b"#!/b"
    assert (
        archive.with_name(archive.name + ".sha256").read_text().split()[0]
        == hashlib.sha256(archive.read_bytes()).hexdigest()
    )


def test_wrong_checksum_filename_is_rejected(release_archive, tmp_path):
    release_archive.with_name(release_archive.name + ".sha256").write_text(
        "0" * 64 + "  unrelated.tar.gz\n"
    )
    destination = tmp_path / "installed"
    result = subprocess.run(
        [
            "bash",
            str(ROOT / "scripts/install.sh"),
            "--version",
            VERSION,
            "--install-dir",
            str(destination),
        ],
        env=installer_env(tmp_path, release_archive),
        capture_output=True,
        text=True,
    )
    assert result.returncode != 0
    assert not (destination / "ash").exists()


def test_existing_python_settings_and_history_survive_go(binary, tmp_path):
    import json
    import sqlite3

    from ashley import history

    env = {"HOME": str(tmp_path), "PATH": "", "GOPROXY": "off"}
    config = tmp_path / ".ashley"
    config.mkdir()
    yaml_text = "# keep comments\nhooks:\n  global:\n    before_run: make test\n"
    (config / "config.yaml").write_text(yaml_text)
    (config / "prefs.json").write_text('{"agent":"codex","extension":42}')
    (config / "theme.json").write_text('{"mode":"light","preset":"ocean"}')
    data = (
        tmp_path / "Library/Application Support/ashley"
        if platform.system() == "Darwin"
        else tmp_path / ".local/share/ashley"
    )
    data.mkdir(parents=True)
    db = data / "history.db"
    # Produce an original, pre-outcome and pre-agent database using Python's
    # sqlite3, then let the real Go binary migrate and read it.
    with sqlite3.connect(db) as connection:
        connection.executescript("""
            CREATE TABLE invocations (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                timestamp TEXT NOT NULL, skill TEXT NOT NULL,
                question TEXT NOT NULL DEFAULT '', cwd TEXT NOT NULL DEFAULT '',
                permission TEXT NOT NULL DEFAULT 'default',
                detached INTEGER NOT NULL DEFAULT 0,
                session_id TEXT NOT NULL DEFAULT ''
            );
            INSERT INTO invocations(timestamp,skill,question)
            VALUES ('2024-01-01T00:00:00+00:00','feat','legacy question');
        """)

    def run(*args):
        result = subprocess.run(
            [str(binary), *args],
            cwd=tmp_path,
            env=env,
            capture_output=True,
            text=True,
        )
        assert result.returncode == 0, result.stderr
        return result.stdout

    records = json.loads(run("history", "show", "--json"))
    assert len(records) == 1
    assert records[0]["question"] == "legacy question"
    assert records[0]["agent_type"] == "claude"
    assert records[0]["outcome"] == "unknown"
    assert "codex" in run("agent")
    run("agent", "grok")
    run("config")
    assert json.loads((config / "prefs.json").read_text()) == {
        "agent": "grok",
        "extension": 42,
    }
    assert (config / "config.yaml").read_text() == yaml_text
    assert json.loads((config / "theme.json").read_text()) == {
        "mode": "light",
        "preset": "ocean",
    }
    # Python still recognizes the database after Go's migration.
    with sqlite3.connect(db) as connection:
        assert history.migrate(connection) == []
        assert connection.execute(
            "SELECT question, agent_type FROM invocations"
        ).fetchone() == ("legacy question", "claude")
    run("history", "clear", "--yes")
    with sqlite3.connect(db) as connection:
        assert connection.execute("SELECT count(*) FROM invocations").fetchone()[0] == 0


def test_detached_run_records_real_exit_and_completion_hooks(binary, tmp_path):
    import json
    import shlex
    import time
    import uuid

    tmux = shutil.which("tmux")
    if not tmux:
        pytest.skip("tmux is not installed")
    socket = f"ashley-binary-{uuid.uuid4().hex}"
    tools = tmp_path / "tools"
    tools.mkdir()
    (tools / "tmux").write_text(
        f'#!/bin/sh\nexec {shlex.quote(tmux)} -L {socket} -f /dev/null "$@"\n'
    )
    (tools / "tmux").chmod(0o700)
    (tools / "codex").write_text(
        '#!/bin/sh\nprintf "agent-start\\n"\nprintf "%s" "$*" > "$HOME/agent-args"\nexit 7\n'
    )
    (tools / "codex").chmod(0o700)
    config = tmp_path / ".ashley"
    config.mkdir()
    (config / "config.yaml").write_text(
        "hooks:\n  global:\n"
        "    before_run: 'echo before > before-hook'\n"
        "    after_run: 'echo $ASHLEY_EXIT_CODE:$ASHLEY_SESSION_ID > after-hook'\n"
        "    on_error: 'echo error > error-hook'\n"
    )
    env = {
        **os.environ,
        "HOME": str(tmp_path),
        "XDG_DATA_HOME": str(tmp_path / "data"),
        "PATH": str(tools) + os.pathsep + os.environ["PATH"],
        "TMUX": "",
    }

    def run(*args):
        result = subprocess.run(
            [str(binary), *args],
            cwd=tmp_path,
            env=env,
            capture_output=True,
            text=True,
            timeout=10,
        )
        assert result.returncode == 0, result.stderr
        return result.stdout

    try:
        question = "literal $HOME `text` " + "x" * 8000
        run("run", "--codex", "--detached", "raw", question)
        sessions = json.loads(run("sessions", "--json"))
        assert len(sessions) == 1
        session = sessions[0]
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            if (tmp_path / "error-hook").exists():
                break
            time.sleep(0.02)
        assert (tmp_path / "before-hook").read_text().strip() == "before"
        assert (tmp_path / "after-hook").read_text().strip() == f"7:{session['id']}"
        assert (tmp_path / "error-hook").is_file()
        assert (tmp_path / "agent-args").read_text() == question
        records = json.loads(run("history", "show", "--json"))
        assert records[0]["exit_code"] == 7
        assert records[0]["outcome"] == "failure"
        assert records[0]["session_id"] == session["id"]
        assert records[0]["detached"] is True
        assert "agent-start" in run("logs", session["id"])
        run("kill", session["id"])
        assert json.loads(run("sessions", "--json")) == []
        assert Path(session["log_file"]).is_file()
        assert list(config.glob("sessions/.ashley-job-*.json")) == []
    finally:
        subprocess.run([tmux, "-L", socket, "kill-server"], capture_output=True)


@pytest.mark.parametrize(
    "command", [[], ["vibe"], ["sessions"], ["history", "browse"], ["create"]]
)
@pytest.mark.parametrize("monochrome", [True, False])
def test_interactive_screens_start_and_restore_terminal(
    binary, tmp_path, command, monochrome
):
    import fcntl
    import pty
    import select
    import struct
    import termios
    import time

    config = tmp_path / ".ashley"
    config.mkdir()
    (config / "theme.json").write_text('{"mode":"dark","preset":"blue"}')
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 32, 120, 0, 0))
    env = {
        **os.environ,
        "HOME": str(tmp_path),
        "TERM": "xterm-256color",
        "XDG_DATA_HOME": str(tmp_path / "data"),
        "NO_COLOR": "1",
    }
    if not monochrome:
        env.pop("NO_COLOR", None)
    env["COLORTERM"] = "truecolor"
    process = subprocess.Popen(
        [str(binary), *command],
        cwd=tmp_path,
        env=env,
        stdin=slave,
        stdout=slave,
        stderr=slave,
    )
    os.close(slave)
    output = b""
    try:
        deadline = time.monotonic() + 8
        while time.monotonic() < deadline and b"Ashley" not in output:
            if select.select([master], [], [], 0.1)[0]:
                output += os.read(master, 65536)
            assert process.poll() is None, output.decode(errors="replace")
        assert b"Ashley" in output, output.decode(errors="replace")
        if not command:
            # Reproduce the reported hub -> Settings Enter freeze in a real PTY.
            os.write(master, b"\x1b[B" * 6 + b"\r")
            settings_output = b""
            deadline = time.monotonic() + 3
            while time.monotonic() < deadline:
                if select.select([master], [], [], 0.05)[0]:
                    settings_output += os.read(master, 65536)
                if b"Accent color" in settings_output:
                    break
            assert b"Accent color" in settings_output, settings_output
            assert len(settings_output) < 100_000
            os.write(master, b"\x1b")
            restored = b""
            deadline = time.monotonic() + 3
            while time.monotonic() < deadline:
                if select.select([master], [], [], 0.05)[0]:
                    restored += os.read(master, 65536)
                if b"Skill Browser" in restored:
                    break
            assert b"Skill Browser" in restored, restored
        if command == ["create"]:
            # Complete the real guided form using terminal key events.
            os.write(
                master,
                b"pty-skill\tTerminal skill\t\tCreated through the terminal\x0e\x0e\x0e\x13",
            )
            saved = config / "skills/pty-skill.jsonc"
            deadline = time.monotonic() + 5
            while not saved.exists() and time.monotonic() < deadline:
                if select.select([master], [], [], 0.1)[0]:
                    output += os.read(master, 65536)
                assert process.poll() is None, output.decode(errors="replace")
            assert saved.exists(), output.decode(errors="replace")
            rendered = subprocess.run(
                [str(binary), "prompt", "pty-skill"],
                env=env,
                cwd=tmp_path,
                capture_output=True,
                text=True,
            )
            assert rendered.returncode == 0, rendered.stderr
            assert "Created through the terminal" in rendered.stdout
        os.write(master, b"\x03")
        deadline = time.monotonic() + 5
        while process.poll() is None and time.monotonic() < deadline:
            if select.select([master], [], [], 0.1)[0]:
                try:
                    output += os.read(master, 65536)
                except OSError:
                    break
        process.wait(timeout=1)
        assert process.returncode == 0, output.decode(errors="replace")
    finally:
        if process.poll() is None:
            process.kill()
            process.wait(timeout=5)
        os.close(master)


def test_remote_bootstrap_is_binary_only(release_archive, tmp_path):
    destination = tmp_path / "remote-bin"
    env = installer_env(tmp_path, release_archive)
    result = subprocess.run(
        [
            "bash",
            str(ROOT / "scripts/remote-install.sh"),
            "--version",
            VERSION,
            "--install-dir",
            str(destination),
            "--binary-only",
        ],
        env=env,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stderr
    assert (destination / "ash").is_file()
    assert not (tmp_path / ".ashley/repo").exists()


def test_local_install_replaces_launcher_link_without_modifying_checkout(
    binary, tmp_path
):
    source = tmp_path / "source-launcher"
    source.write_text("original source launcher")
    destination = tmp_path / "bin"
    destination.mkdir()
    (destination / "ash").symlink_to(source)
    result = subprocess.run(
        [
            "bash",
            str(ROOT / "scripts/dev/install-local.sh"),
            str(binary),
            str(destination),
        ],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stderr
    assert not (destination / "ash").is_symlink()
    assert source.read_text() == "original source launcher"
    assert (
        subprocess.check_output(
            [str(destination / "ash"), "--version"], text=True
        ).strip()
        == f"ashley {VERSION}"
    )


@pytest.mark.parametrize(
    "flags,expected",
    [
        (["--grok", "--opencode", "--kilo"], [".grok", ".config/opencode", ".kilo"]),
        (["--agent", "codex", "--agent=claude"], [".codex", ".claude"]),
    ],
)
def test_remote_bootstrap_preserves_all_agent_selections(
    release_archive, tmp_path, flags, expected
):
    destination = tmp_path / "bin"
    env = installer_env(tmp_path, release_archive)
    env["HOME"] = str(tmp_path)
    for variable in (
        "CLAUDE_CONFIG_DIR",
        "CODEX_HOME",
        "GROK_HOME",
        "OPENCODE_CONFIG_DIR",
        "XDG_CONFIG_HOME",
    ):
        env.pop(variable, None)
    result = subprocess.run(
        [
            "/bin/bash",
            str(ROOT / "scripts/remote-install.sh"),
            "--version",
            VERSION,
            "--install-dir",
            str(destination),
            "--skills-only",
            *flags,
        ],
        env=env,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stderr
    for directory in expected:
        assert (tmp_path / directory / "skills/a-feat/SKILL.md").is_file()


@pytest.mark.parametrize(
    "flags", [["--agent=invalid"], ["--agent"], ["--install-dir"], ["--wat"]]
)
def test_remote_bootstrap_rejects_invalid_options_before_downloading(tmp_path, flags):
    result = subprocess.run(
        ["/bin/bash", str(ROOT / "scripts/remote-install.sh"), *flags],
        env={"HOME": str(tmp_path), "PATH": ""},
        capture_output=True,
        text=True,
    )
    assert result.returncode != 0
    assert "command not found" not in result.stderr


@pytest.mark.parametrize(
    "answer,expected",
    [
        (b"6\n", [".claude", ".codex", ".grok", ".config/opencode", ".kilo"]),
        (b"2\n", [".codex"]),
    ],
)
def test_first_install_agent_selection_in_terminal(binary, tmp_path, answer, expected):
    import pty
    import select
    import time

    master, slave = pty.openpty()
    env = {"HOME": str(tmp_path), "PATH": "/usr/bin:/bin", "TERM": "xterm-256color"}
    process = subprocess.Popen(
        [str(binary), "install", "--skills-only"],
        stdin=slave,
        stdout=slave,
        stderr=slave,
        cwd=tmp_path,
        env=env,
    )
    os.close(slave)
    output = b""
    try:
        deadline = time.monotonic() + 5
        while b"Choice [" not in output and time.monotonic() < deadline:
            if select.select([master], [], [], 0.1)[0]:
                output += os.read(master, 65536)
            assert process.poll() is None, output.decode(errors="replace")
        assert b"Choice [" in output, output.decode(errors="replace")
        os.write(master, answer)
        deadline = time.monotonic() + 10
        while process.poll() is None and time.monotonic() < deadline:
            if select.select([master], [], [], 0.1)[0]:
                try:
                    output += os.read(master, 65536)
                except OSError:
                    break
        process.wait(timeout=1)
        assert process.returncode == 0, output.decode(errors="replace")
        for directory in expected:
            assert (tmp_path / directory / "skills/a-feat/SKILL.md").is_file()
        if answer == b"2\n":
            import json

            assert (
                json.loads((tmp_path / ".ashley/prefs.json").read_text())["agent"]
                == "codex"
            )
            assert not (tmp_path / ".claude/skills").exists()
    finally:
        if process.poll() is None:
            process.kill()
            process.wait(timeout=5)
        os.close(master)


def test_tui_sync_uses_only_the_shipped_binary(binary, tmp_path):
    import fcntl
    import pty
    import select
    import struct
    import termios
    import time

    standalone = tmp_path / "ash"
    shutil.copy2(binary, standalone)
    config = tmp_path / ".ashley"
    config.mkdir()
    (config / "theme.json").write_text('{"mode":"dark","preset":"blue"}')
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
    # Detection-only executable fixtures must never be invoked. No runtime tools.
    agent_bin = tmp_path / ".local/bin"
    agent_bin.mkdir(parents=True)
    for agent in ("claude", "codex", "grok", "opencode", "kilo"):
        fixture = agent_bin / agent
        fixture.write_text("not an executable format; detection only")
        fixture.chmod(0o755)
    env = {
        "HOME": str(tmp_path),
        "PATH": "",
        "TERM": "xterm-256color",
        "COLORTERM": "truecolor",
        "XDG_DATA_HOME": str(tmp_path / "data"),
    }
    process = subprocess.Popen(
        [str(standalone)],
        cwd=tmp_path,
        env=env,
        stdin=slave,
        stdout=slave,
        stderr=slave,
    )
    os.close(slave)
    output = b""

    def wait_for(text):
        nonlocal output
        deadline = time.monotonic() + 8
        while text not in output and time.monotonic() < deadline:
            if select.select([master], [], [], 0.05)[0]:
                output += os.read(master, 65536)
            assert process.poll() is None, output.decode(errors="replace")
        assert text in output, output.decode(errors="replace")

    try:
        wait_for(b"Ashley")
        os.write(master, b"\x1b[B" * 3 + b"\r")
        wait_for(b"Skills synced for Claude Code")
        for directory in (
            ".claude",
            ".codex",
            ".grok",
            ".config/opencode",
            ".kilo",
        ):
            path = tmp_path / directory / "skills/a-feat/SKILL.md"
            assert path.is_file()
            assert len(path.read_text()) > 1000
            assert path.resolve().is_relative_to(config / "generated")
        assert not (tmp_path / "generated").exists()
        # Quick operations keep the same alternate screen instead of flashing
        # command output on the shell and losing the result.
        assert output.count(b"\x1b[?1049h") == 1
        assert b"\x1b[?1049l" not in output
        assert not (tmp_path / ".venv").exists()
        os.write(master, b"\x03")
        deadline = time.monotonic() + 3
        while process.poll() is None and time.monotonic() < deadline:
            if select.select([master], [], [], 0.05)[0]:
                try:
                    os.read(master, 65536)
                except OSError:
                    break
        process.wait(timeout=1)
        assert process.returncode == 0
    finally:
        if process.poll() is None:
            process.kill()
            process.wait(timeout=5)
        os.close(master)
