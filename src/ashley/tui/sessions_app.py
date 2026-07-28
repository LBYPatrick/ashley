"""Ashley Sessions TUI — Manage detached coding-agent sessions."""

import subprocess

from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.widgets import Footer, Header, Label, ListItem, ListView, Static

from ashley.tui.theme import BASE_CSS, apply_theme, fade_in

# Sort modes cycled through with the "s" key.
SORT_MODES = ("time", "skill")
SORT_LABELS = {"time": "newest", "skill": "skill"}


class LogViewer(Static):
    """Displays session log content."""

    pass


class SessionsApp(App):
    """TUI for managing detached Ashley sessions."""

    TITLE = "Ashley Sessions"
    SUB_TITLE = "Manage detached coding-agent sessions"

    CSS = (
        BASE_CSS
        + """
    #main {
        height: 1fr;
    }

    #session-list-container {
        width: 42;
        border: round $surface-lighten-2;
        padding: 0 1;
        margin: 1 0 1 1;
    }

    #session-list-label {
        text-style: bold;
        padding: 0 0 1 0;
        color: $accent-lighten-1;
    }

    #session-list {
        height: 1fr;
    }

    #detail-container {
        width: 1fr;
        border: round $surface-lighten-2;
        padding: 1 2;
        margin: 1 1 0 1;
        overflow-y: auto;
    }

    #detail {
        width: 1fr;
    }

    #log-container {
        height: 2fr;
        border: round $surface-lighten-2;
        padding: 0 2 1 2;
        margin: 1 1 1 1;
        overflow-y: auto;
    }

    #log-label {
        text-style: bold;
        color: $accent-lighten-1;
        padding: 0 0 1 0;
    }

    #log-viewer {
        width: 1fr;
        height: 1fr;
    }

    .session-item {
        padding: 0 1;
    }

    #empty-message {
        padding: 3;
        text-align: center;
        color: $text-muted;
    }
    """
    )

    BINDINGS = [
        Binding("q", "quit", "Quit", show=True),
        Binding("enter", "attach", "Attach", show=True),
        Binding("c", "copy_id", "Copy ID", show=True),
        Binding("l", "view_log", "Log", show=True),
        Binding("s", "cycle_sort", "Sort", show=True),
        Binding("K", "kill_session", "Kill", show=True),
        Binding("X", "kill_all", "Kill All", show=True),
        Binding("d", "delete_session", "Delete", show=True),
        Binding("r", "refresh", "Refresh", show=True),
        Binding("k", "cleanup", "Cleanup Dead", show=True),
    ]

    def __init__(self):
        super().__init__()
        self._sessions: list = []
        self._selected_session = None
        self._sort_mode = "time"

    def compose(self) -> ComposeResult:
        yield Header(show_clock=True)
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
        apply_theme(self)
        fade_in(self.query_one("#main"))
        self._refresh_sessions()

    def _refresh_sessions(self) -> None:
        from ashley.sessions import load_all_sessions, sort_sessions

        self._sessions = sort_sessions(load_all_sessions(), self._sort_mode)
        self._update_list_label()
        list_view = self.query_one("#session-list", ListView)
        list_view.clear()

        if not self._sessions:
            detail = self.query_one("#detail", Static)
            detail.update(
                "No sessions found.\n\nStart one with: ash run --detached <skill> <question>"
            )
            log_viewer = self.query_one("#log-viewer", LogViewer)
            log_viewer.update("")
            self._selected_session = None
            return

        from rich.markup import escape

        for session in self._sessions:
            try:
                status = session.status()
                status_icon = (
                    "[green]●[/green]" if status == "running" else "[dim]○[/dim]"
                )
                label_text = (
                    f"{status_icon} {escape(session.id)}  "
                    f"{escape(session.skill)}  ({escape(session.elapsed())})"
                )
            except Exception:
                # A broken/corrupt session record must not crash the list.
                label_text = f"[red]![/red] {escape(session.id)}  [dim](corrupt)[/dim]"
            list_view.append(ListItem(Label(label_text, classes="session-item")))

        self._selected_session = self._sessions[0]
        # Highlight the first row so Enter attaches immediately without
        # requiring the user to move the cursor first.
        list_view.index = 0
        self._update_detail()
        list_view.focus()

    def _update_list_label(self) -> None:
        running = sum(1 for s in self._sessions if s.status() == "running")
        label = (
            f"Sessions ({len(self._sessions)}) · "
            f"[green]{running} running[/green] · "
            f"sort: [b]{SORT_LABELS[self._sort_mode]}[/b]"
        )
        self.query_one("#session-list-label", Label).update(label)

    def _update_detail(self) -> None:
        if not self._selected_session:
            return
        try:
            self._render_detail(self._selected_session)
        except Exception as exc:
            # Never let a corrupt session record tear down the TUI.
            detail = self.query_one("#detail", Static)
            detail.update(
                f"[bold red]Could not render session "
                f"{getattr(self._selected_session, 'id', '?')}[/bold red]\n\n"
                f"[dim]{exc}[/dim]"
            )
            log_viewer = self.query_one("#log-viewer", LogViewer)
            log_viewer.update("")

    def _render_detail(self, s) -> None:
        from rich.markup import escape

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

        # Escape user-controlled values so a '[' in a question/skill/path
        # is not parsed as Textual console markup.
        text = (
            f"[bold]Session {escape(s.id)}[/bold]\n\n"
            f"Status:     {status_display}\n"
            f"Skill:      [bold]{escape(s.skill)}[/bold]\n"
            f"Question:   {escape(question_display)}\n"
            f"Started:    {escape(s.started_at[:19].replace('T', ' '))} UTC\n"
            f"Elapsed:    {escape(s.elapsed())}\n"
            f"Directory:  {escape(s.cwd)}\n"
            f"Permission: {escape(s.permission_mode or 'default')}\n"
            f"tmux:       {escape(s.tmux_session)}\n"
            f"Log:        {escape(s.log_file)}"
        )
        detail = self.query_one("#detail", Static)
        detail.update(text)
        self._update_log()

    def _update_log(self) -> None:
        if not self._selected_session:
            return
        import re

        from rich.text import Text

        from ashley.sessions import read_log

        content = read_log(self._selected_session, tail=50)
        # Strip ANSI escape codes for cleaner display
        content = re.sub(r"\x1b\[[0-9;]*[a-zA-Z]", "", content)
        content = re.sub(r"\x1b\][^\x07]*\x07", "", content)  # OSC sequences
        log_viewer = self.query_one("#log-viewer", LogViewer)
        # Render log content as literal text — it is arbitrary output and
        # must never be interpreted as console markup.
        log_viewer.update(Text(content) if content.strip() else "(empty log)")

    def on_list_view_highlighted(self, event: ListView.Highlighted) -> None:
        idx = event.list_view.index
        if idx is not None and idx < len(self._sessions):
            self._selected_session = self._sessions[idx]
            self._update_detail()

    def on_list_view_selected(self, event: ListView.Selected) -> None:
        """Attach when the user presses Enter on the focused list.

        ListView consumes the Enter key to emit this event, so the
        app-level "enter" binding never fires while the list has focus.
        """
        self.action_attach()

    def action_copy_id(self) -> None:
        """Copy the selected session's ID to the clipboard."""
        if not self._selected_session:
            self.notify("No session selected", severity="error")
            return
        session_id = self._selected_session.id
        self.copy_to_clipboard(session_id)
        self.notify(f"Copied session ID {session_id} to clipboard")

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

    def action_cycle_sort(self) -> None:
        """Cycle the session list ordering between time and skill."""
        idx = SORT_MODES.index(self._sort_mode)
        self._sort_mode = SORT_MODES[(idx + 1) % len(SORT_MODES)]
        self._refresh_sessions()
        self.notify(f"Sorted by {SORT_LABELS[self._sort_mode]}")

    def action_kill_all(self) -> None:
        """Kill every running session at once."""
        from ashley.sessions import kill_all_sessions

        killed = kill_all_sessions()
        if killed:
            self.notify(f"Killed {killed} running session(s)")
        else:
            self.notify("No running sessions to kill", severity="warning")
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
