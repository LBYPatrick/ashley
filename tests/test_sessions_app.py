"""Tests for the Ashley Sessions TUI."""

import asyncio

import pytest

from ashley import sessions as sessions_module
from ashley.sessions import Session
from ashley.tui.sessions_app import SessionsApp


def _make_session(session_id: str = "abc12345") -> Session:
    """Build a Session whose tmux session does not exist (so it is 'exited')."""
    return Session(
        id=session_id,
        skill="a-coding",
        question="do a thing",
        tmux_session=f"ashley-{session_id}",
        log_file=f"/tmp/{session_id}.log",
        started_at="2026-06-21T00:00:00+00:00",
        cwd="/tmp",
    )


@pytest.fixture
def one_session(monkeypatch):
    session = _make_session()
    monkeypatch.setattr(sessions_module, "load_all_sessions", lambda: [session])
    monkeypatch.setattr(sessions_module, "read_log", lambda *a, **k: "")
    return session


def test_copy_id_copies_selected_session(one_session):
    """Pressing 'C' copies the selected session's ID to the clipboard."""
    copied: list[str] = []

    async def scenario():
        app = SessionsApp()
        async with app.run_test() as pilot:
            app.copy_to_clipboard = lambda text: copied.append(text)
            await pilot.press("C")
            await pilot.pause()

    asyncio.run(scenario())
    assert copied == [one_session.id]


def test_enter_on_list_attaches(one_session):
    """Selecting a list item (Enter) triggers the attach action."""
    attached: list[bool] = []

    async def scenario():
        app = SessionsApp()
        async with app.run_test() as pilot:
            await pilot.pause()
            app.action_attach = lambda: attached.append(True)
            await pilot.press("enter")
            await pilot.pause()

    asyncio.run(scenario())
    assert attached == [True]


def test_broken_session_does_not_crash(monkeypatch):
    """A session with malformed metadata must not crash the TUI."""
    broken = _make_session("broken01")
    broken.started_at = "not-a-timestamp"
    monkeypatch.setattr(sessions_module, "load_all_sessions", lambda: [broken])
    monkeypatch.setattr(sessions_module, "read_log", lambda *a, **k: "")

    errors: list[BaseException] = []

    async def scenario():
        app = SessionsApp()
        async with app.run_test() as pilot:
            await pilot.pause()
            if app._exception is not None:
                errors.append(app._exception)

    asyncio.run(scenario())
    assert errors == []
