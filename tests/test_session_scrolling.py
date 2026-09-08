"""Exercise session-scoped scrolling on an isolated real tmux server."""

import shutil
import subprocess
import uuid
from unittest.mock import patch

import pytest

from ashley import sessions


@pytest.fixture
def tmux_server():
    if not shutil.which("tmux"):
        pytest.skip("tmux is not installed")
    run = subprocess.run
    socket = f"ashley-test-{uuid.uuid4().hex}"

    def tmux(args, **kwargs):
        return run(["tmux", "-L", socket, "-f", "/dev/null", *args], **kwargs)

    def routed(args, **kwargs):
        assert args[0] == "tmux"
        return tmux(args[1:], **kwargs)

    tmux(["new-session", "-d", "-s", "ashley-test", "sleep", "300"], check=True)
    try:
        with patch.object(sessions.subprocess, "run", side_effect=routed):
            yield tmux
    finally:
        tmux(["kill-server"], capture_output=True)


def test_scrolling_preserves_other_sessions_and_shortcuts(tmux_server):
    tmux = tmux_server
    tmux(["new-session", "-d", "-s", "unrelated", "sleep", "300"], check=True)
    tmux(
        ["bind-key", "-T", "root", "F12", "display-message", "custom shortcut"],
        check=True,
    )
    before = tmux(["list-keys", "-T", "root"], capture_output=True, text=True).stdout
    sessions.configure_scrolling("ashley-test")
    sessions.configure_scrolling("ashley-test")  # Reattaching is idempotent.
    for name, expected in (("ashley-test", "on"), ("unrelated", "off")):
        result = tmux(
            ["show-options", "-Av", "-t", name, "mouse"], capture_output=True, text=True
        )
        assert result.stdout.strip() == expected
    assert (
        tmux(["list-keys", "-T", "root"], capture_output=True, text=True).stdout
        == before
    )
    bindings = tmux(
        ["list-keys", "-T", "ashley-scroll-root"], capture_output=True, text=True
    ).stdout
    assert "custom shortcut" in bindings
    wheel_up = next(line for line in bindings.splitlines() if "WheelUpPane" in line)
    assert "copy-mode -e" in wheel_up
    assert "send-keys -M" not in wheel_up
    wheel_down = next(line for line in bindings.splitlines() if "WheelDownPane" in line)
    assert "scroll-down" in wheel_down


def test_create_and_reattach_configure_scrolling(tmp_path):
    with (
        patch.object(sessions, "SESSIONS_DIR", tmp_path),
        patch.object(sessions.subprocess, "run") as run,
        patch.object(sessions, "configure_scrolling") as configure,
    ):
        session = sessions.create_detached_session(
            "feat", "hello", ["claude"], str(tmp_path)
        )
        configure.assert_called_once_with(session.tmux_session)
        configure.reset_mock()
        run.return_value.returncode = 0
        assert sessions.attach_session(session) == 0
        configure.assert_called_once_with(session.tmux_session)
        configure.reset_mock()
        run.return_value.returncode = 1
        assert sessions.attach_session(session) == 1
        configure.assert_not_called()


def test_wheel_enters_scrollback_in_alternate_screen(tmux_server):
    import os
    import pty
    import time

    tmux = tmux_server
    sessions.configure_scrolling("ashley-test")
    tmux(
        [
            "respawn-pane",
            "-k",
            "-t",
            "ashley-test",
            "sh",
            "-c",
            "printf '\033[?1049h'; sleep 300",
        ],
        check=True,
    )
    master, slave = pty.openpty()
    # Use the same isolated server as the fixture, with a real terminal client.
    server_path = tmux(
        ["display-message", "-p", "#{socket_path}"], capture_output=True, text=True
    ).stdout.strip()
    client = subprocess.Popen(
        ["tmux", "-S", server_path, "attach-session", "-t", "ashley-test"],
        stdin=slave,
        stdout=slave,
        stderr=slave,
        env={**os.environ, "TERM": "xterm-256color", "TMUX": ""},
    )
    os.close(slave)
    try:
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            status = tmux(
                [
                    "display-message",
                    "-p",
                    "-t",
                    "ashley-test",
                    "#{session_attached}:#{alternate_on}",
                ],
                capture_output=True,
                text=True,
            ).stdout.strip()
            if status == "1:1":
                break
            time.sleep(0.02)
        assert status == "1:1"
        os.write(master, b"\x1b[<64;10;5M")
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            mode = tmux(
                ["display-message", "-p", "-t", "ashley-test", "#{pane_in_mode}"],
                capture_output=True,
                text=True,
            ).stdout.strip()
            if mode == "1":
                break
            time.sleep(0.02)
        assert mode == "1", "wheel event was forwarded instead of opening scrollback"
    finally:
        client.terminate()
        client.wait(timeout=5)
        os.close(master)
