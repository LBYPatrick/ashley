"""Tests for session sorting and bulk-kill helpers."""

from ashley.sessions import Session, kill_all_sessions, sort_sessions


def _session(session_id: str, skill: str, started_at: str) -> Session:
    return Session(
        id=session_id,
        skill=skill,
        question="",
        tmux_session=f"ashley-{session_id}",
        log_file=f"/tmp/{session_id}.log",
        started_at=started_at,
        cwd="/tmp",
    )


def _fixture_sessions() -> list[Session]:
    return [
        _session("a", "a-feat", "2026-06-21T10:00:00+00:00"),
        _session("b", "a-debug", "2026-06-21T12:00:00+00:00"),
        _session("c", "a-feat", "2026-06-21T08:00:00+00:00"),
    ]


def test_sort_by_time_newest_first():
    ordered = sort_sessions(_fixture_sessions(), "time")
    assert [s.id for s in ordered] == ["b", "a", "c"]


def test_sort_by_skill_alphabetical_then_newest():
    ordered = sort_sessions(_fixture_sessions(), "skill")
    # a-debug first, then a-feat group ordered newest-first (a before c).
    assert [s.id for s in ordered] == ["b", "a", "c"]
    assert [s.skill for s in ordered] == ["a-debug", "a-feat", "a-feat"]


def test_sort_unknown_mode_falls_back_to_time():
    ordered = sort_sessions(_fixture_sessions(), "bogus")
    assert [s.id for s in ordered] == ["b", "a", "c"]


def test_sort_does_not_mutate_input():
    sessions = _fixture_sessions()
    original = list(sessions)
    sort_sessions(sessions, "skill")
    assert sessions == original


def test_kill_all_kills_running_and_prunes_dead(monkeypatch, tmp_path):
    from ashley import sessions as sessions_module

    # Point the session store at a temp dir so real files are created/pruned.
    monkeypatch.setattr(sessions_module, "SESSIONS_DIR", tmp_path)

    running = _session("live0001", "a-feat", "2026-06-21T10:00:00+00:00")
    dead = _session("dead0001", "a-debug", "2026-06-21T09:00:00+00:00")
    dead.save()  # dead session leaves a metadata file behind
    assert dead.meta_path.exists()

    monkeypatch.setattr(running, "is_alive", lambda: True)
    monkeypatch.setattr(dead, "is_alive", lambda: False)
    monkeypatch.setattr(sessions_module, "load_all_sessions", lambda: [running, dead])

    killed_ids: list[str] = []
    monkeypatch.setattr(
        sessions_module, "kill_session", lambda s: killed_ids.append(s.id)
    )

    killed = kill_all_sessions()
    assert killed == 1
    assert killed_ids == ["live0001"]
    assert not dead.meta_path.exists()  # dead record pruned
