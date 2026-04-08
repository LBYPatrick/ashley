"""Ashley Sessions TUI — Manage detached Claude Code sessions."""

import subprocess

from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.widgets import Footer, Header, Label, ListItem, ListView, Static


class LogViewer(Static):
    """Displays session log content."""

    pass


class SessionsApp(App):
    """TUI for managing detached Ashley sessions."""

    TITLE = "Ashley Sessions"
    SUB_TITLE = "Manage detached Claude Code sessions"

    CSS = """
    #main {
        height: 1fr;
    }

    #session-list-container {
        width: 40;
        border-right: solid $surface-lighten-2;
        padding: 0 1;
    }

    #session-list-label {
        text-style: bold;
        padding: 1 0 0 0;
        color: $text;
    }

    #session-list {
        height: 1fr;
    }

    #detail-container {
        width: 1fr;
        padding: 1 2;
        overflow-y: auto;
    }

    #detail {
        width: 1fr;
    }

    #log-container {
        height: 2fr;
        border-top: solid $surface-lighten-2;
        padding: 1 2;
        overflow-y: auto;
    }

    #log-label {
        text-style: bold;
        color: $text;
    }

    #log-viewer {
        width: 1fr;
        height: 1fr;
    }

    .session-item {
        padding: 0 1;
    }

    .status-running {
        color: $success;
    }

    .status-exited {
        color: $text-muted;
    }

    #empty-message {
        padding: 3;
        text-align: center;
        color: $text-muted;
    }
    """

    BINDINGS = [
        Binding("q", "quit", "Quit", show=True),
        Binding("enter", "attach", "Attach", show=True),
        Binding("l", "view_log", "Log", show=True),
        Binding("k", "kill_session", "Kill", show=True),
        Binding("d", "delete_session", "Delete", show=True),
        Binding("r", "refresh", "Refresh", show=True),
        Binding("c", "cleanup", "Cleanup Dead", show=True),
    ]

    def __init__(self):
        super().__init__()
        self._sessions: list = []
        self._selected_session = None

    def compose(self) -> ComposeResult:
        yield Header()
        with Horizontal(id="main"):
            with Vertical(id="session-list-container"):
                yield Label("Sessions", id="session-list-label")
                yield ListView(id="session-list")
            with Vertical(id="detail-container"):
                yield Static(id="detail")
                with Vertical(id="log-container"):
                    yield Label("Log (last 50 lines)", id="log-label")
                    yield LogViewer(id="log-viewer")
        yield Footer()

    def on_mount(self) -> None:
        self._refresh_sessions()

    def _refresh_sessions(self) -> None:
        from ashley.sessions import load_all_sessions

        self._sessions = load_all_sessions()
        list_view = self.query_one("#session-list", ListView)
        list_view.clear()

        if not self._sessions:
            detail = self.query_one("#detail", Static)
            detail.update(
                "No sessions found.\n\nStart one with: ash run --detached <skill> [question]"
            )
            log_viewer = self.query_one("#log-viewer", LogViewer)
            log_viewer.update("")
            self._selected_session = None
            return

        for session in self._sessions:
            status = session.status()
            status_icon = "[green]●[/green]" if status == "running" else "[dim]○[/dim]"
            label_text = (
                f"{status_icon} {session.id}  {session.skill}  ({session.elapsed()})"
            )
            list_view.append(ListItem(Label(label_text, classes="session-item")))

        self._selected_session = self._sessions[0]
        self._update_detail()
        list_view.focus()

    def _update_detail(self) -> None:
        if not self._selected_session:
            return
        s = self._selected_session
        status = s.status()
        status_display = (
            "[bold green]RUNNING[/bold green]"
            if status == "running"
            else "[dim]EXITED[/dim]"
        )

        question_display = (
            s.question[:80] + "..." if len(s.question) > 80 else s.question
        )
        if not question_display:
            question_display = "(no question)"

        text = (
            f"[bold]Session {s.id}[/bold]\n\n"
            f"Status:     {status_display}\n"
            f"Skill:      [bold]{s.skill}[/bold]\n"
            f"Question:   {question_display}\n"
            f"Started:    {s.started_at[:19].replace('T', ' ')} UTC\n"
            f"Elapsed:    {s.elapsed()}\n"
            f"Directory:  {s.cwd}\n"
            f"Permission: {s.permission_mode or 'default'}\n"
            f"tmux:       {s.tmux_session}\n"
            f"Log:        {s.log_file}"
        )
        detail = self.query_one("#detail", Static)
        detail.update(text)
        self._update_log()

    def _update_log(self) -> None:
        if not self._selected_session:
            return
        from ashley.sessions import read_log

        content = read_log(self._selected_session, tail=50)
        # Strip ANSI escape codes for cleaner display
        import re

        content = re.sub(r"\x1b\[[0-9;]*[a-zA-Z]", "", content)
        content = re.sub(r"\x1b\][^\x07]*\x07", "", content)  # OSC sequences
        log_viewer = self.query_one("#log-viewer", LogViewer)
        log_viewer.update(content if content.strip() else "(empty log)")

    def on_list_view_highlighted(self, event: ListView.Highlighted) -> None:
        idx = event.list_view.index
        if idx is not None and idx < len(self._sessions):
            self._selected_session = self._sessions[idx]
            self._update_detail()

    def action_attach(self) -> None:
        """Attach to the selected session."""
        if not self._selected_session:
            self.notify("No session selected", severity="error")
            return
        if not self._selected_session.is_alive():
            self.notify("Session is not running", severity="warning")
            return

        session = self._selected_session
        with self.suspend():
            subprocess.run(["tmux", "attach-session", "-t", session.tmux_session])
            input("\nPress Enter to return to session manager...")
        self._refresh_sessions()

    def action_view_log(self) -> None:
        """View the full log in a pager."""
        if not self._selected_session:
            self.notify("No session selected", severity="error")
            return

        from ashley.sessions import read_log

        log_content = read_log(self._selected_session)
        if not log_content.strip():
            self.notify("Log is empty", severity="warning")
            return

        with self.suspend():
            # Use less as pager
            proc = subprocess.Popen(
                ["less", "-R"],
                stdin=subprocess.PIPE,
            )
            proc.communicate(input=log_content.encode())
            input("\nPress Enter to return to session manager...")

    def action_kill_session(self) -> None:
        """Kill the selected session."""
        if not self._selected_session:
            self.notify("No session selected", severity="error")
            return
        if not self._selected_session.is_alive():
            self.notify("Session already exited", severity="warning")
            return

        from ashley.sessions import kill_session

        kill_session(self._selected_session)
        self.notify(f"Killed session {self._selected_session.id}")
        self._refresh_sessions()

    def action_delete_session(self) -> None:
        """Delete session record and log file."""
        if not self._selected_session:
            self.notify("No session selected", severity="error")
            return
        if self._selected_session.is_alive():
            self.notify("Kill the session first", severity="warning")
            return

        self._selected_session.delete()
        self.notify(f"Deleted session {self._selected_session.id}")
        self._refresh_sessions()

    def action_refresh(self) -> None:
        """Refresh the session list."""
        self._refresh_sessions()
        self.notify("Refreshed")

    def action_cleanup(self) -> None:
        """Remove all dead session records."""
        from ashley.sessions import cleanup_dead_sessions

        cleaned = cleanup_dead_sessions()
        self.notify(f"Cleaned up {cleaned} dead session(s)")
        self._refresh_sessions()
