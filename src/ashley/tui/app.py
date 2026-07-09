"""Ashley TUI — Main hub with feature navigation."""

from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Grid, Horizontal, Vertical
from textual.message import Message
from textual.screen import Screen
from textual.widgets import (
    Button,
    Footer,
    Header,
    Label,
    ListItem,
    ListView,
    Static,
)

import ashley
from ashley.config import THEME_PRESETS, load_theme, save_theme, theme_configured
from ashley.tui.sessions_app import SORT_LABELS, SORT_MODES
from ashley.tui.theme import BASE_CSS, apply_theme, fade_in

# ── Feature definitions ──

FEATURES = [
    {
        "key": "vibe",
        "title": "Vibe — Skill Browser",
        "description": "Browse skills, preview workflows, and launch Claude Code with a skill prompt.",
        "icon": "▸",
    },
    {
        "key": "sessions",
        "title": "Sessions — Detached Runs",
        "description": "Manage background Claude Code sessions. Attach, view logs, or kill running sessions.",
        "icon": "⇢",
    },
    {
        "key": "history",
        "title": "History — Invocation Log",
        "description": "Browse your full invocation history. Search, filter, prune, or clear past runs.",
        "icon": "◷",
    },
    {
        "key": "generate",
        "title": "Generate — Rebuild Skills",
        "description": "Regenerate all skill markdown files from JSONC definitions.",
        "icon": "⚙",
    },
    {
        "key": "install",
        "title": "Install — Deploy Skills",
        "description": "Generate skills and install them to ~/.claude/skills/ as slash commands.",
        "icon": "↓",
    },
    {
        "key": "create",
        "title": "Create — New Skill",
        "description": "Build a new skill interactively with a step-by-step wizard.",
        "icon": "+",
    },
    {
        "key": "stats",
        "title": "Stats — Analytics",
        "description": "View skill usage analytics and run-duration insights.",
        "icon": "◆",
    },
    {
        "key": "settings",
        "title": "Settings — Appearance",
        "description": "Choose light or dark mode and a primary colour or dual-tone preset.",
        "icon": "✎",
    },
]


class HubScreen(Screen):
    """Main hub screen showing Ashley features."""

    CSS = """
    #hub-main {
        height: 1fr;
    }

    #feature-list-container {
        width: 38;
        border: round $surface-lighten-2;
        padding: 0 1;
        margin: 1 0 1 1;
    }

    #feature-list-label {
        text-style: bold;
        padding: 0 0 1 0;
        color: $accent-lighten-1;
    }

    #feature-list {
        height: 1fr;
    }

    #feature-detail {
        width: 1fr;
        border: round $surface-lighten-2;
        padding: 2 3;
        margin: 1 1 1 1;
    }

    .feature-item {
        padding: 0 1;
    }
    """

    BINDINGS = [
        Binding("q", "quit_app", "Quit", show=True),
    ]

    def __init__(self):
        super().__init__()
        self._selected_index = 0

    def compose(self) -> ComposeResult:
        yield Header(show_clock=True)
        with Horizontal(id="hub-main"):
            with Vertical(id="feature-list-container"):
                yield Label("Ashley", id="feature-list-label")
                yield ListView(
                    *[
                        ListItem(
                            Label(
                                f"{f['icon']}  {f['title']}",
                                classes="feature-item",
                            ),
                            id=f"feature-{i}",
                        )
                        for i, f in enumerate(FEATURES)
                    ],
                    id="feature-list",
                )
            with Vertical(id="feature-detail"):
                yield Static(id="detail")
        yield Footer()

    def on_mount(self) -> None:
        fade_in(self.query_one("#hub-main"))
        self._update_detail(0)
        self.query_one("#feature-list", ListView).focus()

    def on_list_view_highlighted(self, event: ListView.Highlighted) -> None:
        idx = event.list_view.index
        if idx is not None:
            self._selected_index = idx
            self._update_detail(idx)

    def _update_detail(self, idx: int) -> None:
        if idx >= len(FEATURES):
            return
        f = FEATURES[idx]
        text = (
            f"[bold $accent-lighten-1]{f['icon']}  {f['title']}[/]\n\n"
            f"{f['description']}\n\n"
            f"[dim]Press Enter to open[/dim]"
        )
        self.query_one("#detail", Static).update(text)

    def on_list_view_selected(self, event: ListView.Selected) -> None:
        """Handle Enter key on feature list."""
        feature = FEATURES[self._selected_index]
        key = feature["key"]
        if key == "vibe":
            self.app.push_screen(VibeScreen())
        elif key == "sessions":
            self.app.push_screen(SessionsScreen())
        elif key == "history":
            self.app.push_screen(HistoryScreen())
        elif key == "generate":
            self._run_generate()
        elif key == "install":
            self._run_install()
        elif key == "create":
            from ashley.tui.create_app import CreateScreen

            self.app.push_screen(CreateScreen())
        elif key == "stats":
            self.app.push_screen(StatsScreen())
        elif key == "settings":
            self.app.push_screen(SettingsScreen())

    def _run_generate(self) -> None:
        from ashley.generate import generate as do_generate

        with self.app.suspend():
            do_generate()
            input("\nPress Enter to return to Ashley...")

    def _run_install(self) -> None:
        from ashley.generate import generate as do_generate
        from ashley.install import install as do_install

        with self.app.suspend():
            do_generate()
            do_install()
            input("\nPress Enter to return to Ashley...")

    def action_quit_app(self) -> None:
        self.app.exit()


# ── Vibe Screen (Skill Browser) ──


class VibeScreen(Screen):
    """Skill browser — browse, preview, and launch skills."""

    CSS = """
    #vibe-main {
        height: 1fr;
    }

    #skill-list-container {
        width: 30;
        border: round $surface-lighten-2;
        padding: 0 1;
        margin: 1 0 0 1;
    }

    #skill-list-label {
        text-style: bold;
        padding: 0 0 1 0;
        color: $accent-lighten-1;
    }

    #skill-list {
        height: 1fr;
    }

    #skill-detail-container {
        width: 1fr;
        border: round $surface-lighten-2;
        padding: 1 3;
        margin: 1 1 0 1;
        overflow-y: auto;
    }

    #skill-detail {
        width: 1fr;
    }

    #question-container {
        height: 3;
        padding: 0 1;
        margin: 0 1 1 1;
    }

    #question-input {
        width: 1fr;
    }

    .skill-item {
        padding: 0 1;
    }
    """

    BINDINGS = [
        Binding("escape", "go_back", "Back", show=True),
        Binding("slash", "focus_filter", "Filter", show=True),
        Binding("p", "copy_prompt", "Copy Prompt", show=True),
    ]

    def __init__(self):
        super().__init__()
        self._skills = _load_skill_summaries()
        self._selected_skill: dict | None = None

    def compose(self) -> ComposeResult:
        yield Header(show_clock=True)
        with Horizontal(id="vibe-main"):
            with Vertical(id="skill-list-container"):
                yield Label("Skills", id="skill-list-label")
                yield ListView(
                    *[
                        ListItem(
                            Label(s["name"], classes="skill-item"),
                            id=f"skill-{i}",
                        )
                        for i, s in enumerate(self._skills)
                    ],
                    id="skill-list",
                )
            with Vertical(id="skill-detail-container"):
                yield Static(id="skill-detail")
        with Horizontal(id="question-container"):
            from textual.widgets import Input

            yield Input(
                placeholder="Enter your question, then press Enter to run...",
                id="question-input",
            )
        yield Footer()

    def on_mount(self) -> None:
        fade_in(self.query_one("#vibe-main"))
        if self._skills:
            self._selected_skill = self._skills[0]
            self._update_skill_detail()
            self.query_one("#skill-list", ListView).focus()

    def on_list_view_highlighted(self, event: ListView.Highlighted) -> None:
        idx = event.list_view.index
        if idx is not None and idx < len(self._skills):
            self._selected_skill = self._skills[idx]
            self._update_skill_detail()

    def _update_skill_detail(self) -> None:
        if not self._selected_skill:
            return
        skill = self._selected_skill

        lines: list[str] = []
        # Title + description.
        lines.append(f"[b $accent]▸ {skill['name']}[/]")
        desc = skill["description"] or "No description provided."
        lines.append(f"[$text-muted]{desc}[/]")
        lines.append("")

        # Overview block — aligned label/value rows.
        lines.append("[b $accent-lighten-1]Overview[/]")

        def _row(label: str, value: object) -> str:
            return f"  [$text-muted]{label:<11}[/][b]{value}[/]"

        if skill["extends"]:
            lines.append(_row("Extends", skill["extends"]))
        lines.append(_row("Components", skill["components"]))
        lines.append(_row("Resources", skill["resources"]))
        lines.append(_row("Steps", len(skill["steps"])))

        # Workflow steps — accent-numbered list.
        if skill["steps"]:
            lines.append("")
            lines.append("[b $accent-lighten-1]Workflow[/]")
            for i, name in enumerate(skill["steps"], 1):
                lines.append(f"  [b $accent]{i:>2}[/]  {name}")

        lines.append("")
        lines.append(
            "[$text-muted]Type a question below and press "
            "[b]Enter[/] to run · [b]p[/] to copy the prompt[/]"
        )

        self.query_one("#skill-detail", Static).update("\n".join(lines))

    def on_input_submitted(self, event) -> None:
        from textual.widgets import Input

        if isinstance(event.input, Input) and event.input.id == "question-input":
            self._run_skill(event.value)

    def _run_skill(self, question: str) -> None:
        if not self._selected_skill:
            self.app.notify("No skill selected", severity="error")
            return

        import shutil

        claude_bin = shutil.which("claude")
        if not claude_bin:
            self.app.notify(
                "Claude Code not found. Install it first.", severity="error"
            )
            return

        skill_stem = self._selected_skill["stem"]

        # Record in history
        import os

        from ashley.history import record

        record(
            skill=skill_stem,
            question=question,
            cwd=os.getcwd(),
        )

        import subprocess

        from ashley.cli import _skill_is_installed

        if _skill_is_installed(skill_stem):
            # Skill is installed — let Claude Code load it natively
            args = [claude_bin]
            if question:
                args.append(f"/a-{skill_stem} {question}")
            else:
                args.append(f"/a-{skill_stem}")
        else:
            # Skill not installed — inline the prompt
            from ashley import GENERATED_DIR
            from ashley.generate import generate as do_generate
            from ashley.prompt import generate_prompt

            if not GENERATED_DIR.is_dir() or not any(GENERATED_DIR.iterdir()):
                do_generate()

            system_prompt = generate_prompt(skill_stem, "")
            args = [claude_bin, "--append-system-prompt", system_prompt]
            if question:
                args.append(question)
            else:
                args.append(f"/a-{skill_stem}")

        with self.app.suspend():
            subprocess.run(args)
            input("\nPress Enter to return to Ashley...")

    def action_go_back(self) -> None:
        self.app.pop_screen()

    def action_focus_filter(self) -> None:
        from textual.widgets import Input

        self.query_one("#question-input", Input).focus()

    def action_copy_prompt(self) -> None:
        if not self._selected_skill:
            self.app.notify("No skill selected", severity="error")
            return

        from ashley import GENERATED_DIR
        from ashley.generate import generate as do_generate
        from ashley.prompt import generate_prompt

        if not GENERATED_DIR.is_dir() or not any(GENERATED_DIR.iterdir()):
            do_generate()

        from textual.widgets import Input

        skill_stem = self._selected_skill["stem"]
        question = self.query_one("#question-input", Input).value

        prompt_text = generate_prompt(skill_stem, question)
        try:
            import pyperclip

            pyperclip.copy(prompt_text)
            self.app.notify(f"Prompt for {self._selected_skill['name']} copied!")
        except Exception:
            self.app.notify(
                "Clipboard not available — use 'ash prompt' instead",
                severity="warning",
            )


# ── Sessions Screen (wraps SessionsApp logic inline) ──


class SessionsScreen(Screen):
    """Detached session manager screen."""

    BINDINGS = [
        Binding("escape", "go_back", "Back", show=True),
        Binding("enter", "attach", "Attach", show=True),
        Binding("c", "copy_id", "Copy ID", show=True),
        Binding("l", "view_log", "Log", show=True),
        Binding("s", "cycle_sort", "Sort", show=True),
        Binding("K", "kill_session", "Kill", show=True),
        Binding("X", "kill_all", "Kill All", show=True),
        Binding("d", "delete_session", "Delete", show=True),
        Binding("r", "refresh", "Refresh", show=True),
        Binding("k", "cleanup", "Cleanup", show=True),
    ]

    CSS = """
    #sessions-main {
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

    #session-detail-container {
        width: 1fr;
        border: round $surface-lighten-2;
        padding: 1 2;
        margin: 1 1 0 1;
        overflow-y: auto;
    }

    #session-detail {
        width: 1fr;
    }

    #session-log-container {
        height: 2fr;
        border: round $surface-lighten-2;
        padding: 0 2 1 2;
        margin: 1 1 1 1;
        overflow-y: auto;
    }

    #session-log-label {
        text-style: bold;
        color: $accent-lighten-1;
        padding: 0 0 1 0;
    }

    #session-log-viewer {
        width: 1fr;
        height: 1fr;
    }

    .session-item {
        padding: 0 1;
    }
    """

    def __init__(self):
        super().__init__()
        self._sessions: list = []
        self._selected_session = None
        self._sort_mode = "time"

    def compose(self) -> ComposeResult:
        yield Header(show_clock=True)
        with Horizontal(id="sessions-main"):
            with Vertical(id="session-list-container"):
                yield Label("Sessions", id="session-list-label")
                yield ListView(id="session-list")
            with Vertical(id="session-detail-container"):
                yield Static(id="session-detail")
                with Vertical(id="session-log-container"):
                    yield Label("Log (last 50 lines)", id="session-log-label")
                    yield Static(id="session-log-viewer")
        yield Footer()

    def on_mount(self) -> None:
        fade_in(self.query_one("#sessions-main"))
        self._refresh_sessions()

    def _refresh_sessions(self) -> None:
        from ashley.sessions import load_all_sessions, sort_sessions

        self._sessions = sort_sessions(load_all_sessions(), self._sort_mode)
        self._update_list_label()
        list_view = self.query_one("#session-list", ListView)
        list_view.clear()

        if not self._sessions:
            self.query_one("#session-detail", Static).update(
                "No sessions found.\n\nStart one with: ash run --detached <skill> <question>"
            )
            self.query_one("#session-log-viewer", Static).update("")
            self._selected_session = None
            return

        from rich.markup import escape

        for session in self._sessions:
            status = session.status()
            icon = "[green]●[/green]" if status == "running" else "[dim]○[/dim]"
            label_text = (
                f"{icon} {escape(session.id)}  "
                f"{escape(session.skill)}  ({escape(session.elapsed())})"
            )
            list_view.append(ListItem(Label(label_text, classes="session-item")))

        self._selected_session = self._sessions[0]
        self._update_session_detail()
        list_view.focus()

    def _update_list_label(self) -> None:
        running = sum(1 for s in self._sessions if s.status() == "running")
        label = (
            f"Sessions ({len(self._sessions)}) · "
            f"[green]{running} running[/green] · "
            f"sort: [b]{SORT_LABELS[self._sort_mode]}[/b]"
        )
        self.query_one("#session-list-label", Label).update(label)

    def _update_session_detail(self) -> None:
        if not self._selected_session:
            return
        s = self._selected_session
        status = s.status()
        status_display = (
            "[bold green]RUNNING[/bold green]"
            if status == "running"
            else "[dim]EXITED[/dim]"
        )
        from rich.markup import escape

        q = s.question[:80] + "..." if len(s.question) > 80 else s.question
        if not q:
            q = "(no question)"

        text = (
            f"[bold]Session {escape(s.id)}[/bold]\n\n"
            f"Status:     {status_display}\n"
            f"Skill:      [bold]{escape(s.skill)}[/bold]\n"
            f"Question:   {escape(q)}\n"
            f"Started:    {escape(s.started_at[:19].replace('T', ' '))} UTC\n"
            f"Elapsed:    {escape(s.elapsed())}\n"
            f"Directory:  {escape(s.cwd)}\n"
            f"tmux:       {escape(s.tmux_session)}"
        )
        self.query_one("#session-detail", Static).update(text)

        # Update log — render literally; it is arbitrary output, never markup.
        import re

        from rich.text import Text

        from ashley.sessions import read_log

        content = read_log(self._selected_session, tail=50)
        content = re.sub(r"\x1b\[[0-9;]*[a-zA-Z]", "", content)
        content = re.sub(r"\x1b\][^\x07]*\x07", "", content)
        self.query_one("#session-log-viewer", Static).update(
            Text(content) if content.strip() else "(empty log)"
        )

    def on_list_view_highlighted(self, event: ListView.Highlighted) -> None:
        idx = event.list_view.index
        if idx is not None and idx < len(self._sessions):
            self._selected_session = self._sessions[idx]
            self._update_session_detail()

    def on_list_view_selected(self, event: ListView.Selected) -> None:
        """Attach when the user presses Enter on the focused session list."""
        self.action_attach()

    def action_go_back(self) -> None:
        self.app.pop_screen()

    def action_copy_id(self) -> None:
        if not self._selected_session:
            self.app.notify("No session selected", severity="error")
            return
        session_id = self._selected_session.id
        self.app.copy_to_clipboard(session_id)
        self.app.notify(f"Copied session ID {session_id} to clipboard")

    def action_cycle_sort(self) -> None:
        idx = SORT_MODES.index(self._sort_mode)
        self._sort_mode = SORT_MODES[(idx + 1) % len(SORT_MODES)]
        self._refresh_sessions()
        self.app.notify(f"Sorted by {SORT_LABELS[self._sort_mode]}")

    def action_kill_all(self) -> None:
        from ashley.sessions import kill_all_sessions

        killed = kill_all_sessions()
        if killed:
            self.app.notify(f"Killed {killed} running session(s)")
        else:
            self.app.notify("No running sessions to kill", severity="warning")
        self._refresh_sessions()

    def action_attach(self) -> None:
        if not self._selected_session or not self._selected_session.is_alive():
            self.app.notify("No running session selected", severity="warning")
            return
        import subprocess

        s = self._selected_session
        with self.app.suspend():
            subprocess.run(["tmux", "attach-session", "-t", s.tmux_session])
            input("\nPress Enter to return to Ashley...")
        self._refresh_sessions()

    def action_view_log(self) -> None:
        if not self._selected_session:
            return
        from ashley.sessions import read_log

        content = read_log(self._selected_session)
        if not content.strip():
            self.app.notify("Log is empty", severity="warning")
            return
        import subprocess

        with self.app.suspend():
            proc = subprocess.Popen(["less", "-R"], stdin=subprocess.PIPE)
            proc.communicate(input=content.encode())
            input("\nPress Enter to return to Ashley...")

    def action_kill_session(self) -> None:
        if not self._selected_session or not self._selected_session.is_alive():
            self.app.notify("No running session to kill", severity="warning")
            return
        from ashley.sessions import kill_session

        kill_session(self._selected_session)
        self.app.notify(f"Killed session {self._selected_session.id}")
        self._refresh_sessions()

    def action_delete_session(self) -> None:
        if not self._selected_session:
            return
        if self._selected_session.is_alive():
            self.app.notify("Kill the session first", severity="warning")
            return
        self._selected_session.delete()
        self.app.notify(f"Deleted session {self._selected_session.id}")
        self._refresh_sessions()

    def action_refresh(self) -> None:
        self._refresh_sessions()

    def action_cleanup(self) -> None:
        from ashley.sessions import cleanup_dead_sessions

        cleaned = cleanup_dead_sessions()
        self.app.notify(f"Cleaned up {cleaned} dead session(s)")
        self._refresh_sessions()


# ── History Screen ──


class HistoryScreen(Screen):
    """Invocation history browser screen."""

    BINDINGS = [
        Binding("escape", "go_back", "Back", show=True),
        Binding("slash", "focus_filter", "Search", show=True),
        Binding("d", "delete_entry", "Delete", show=True),
        Binding("r", "refresh", "Refresh", show=True),
        Binding("n", "next_page", "Next", show=True),
        Binding("p_key", "prev_page", "Prev", show=False),
    ]

    CSS = """
    #history-main {
        height: 1fr;
    }

    #history-list-container {
        width: 50;
        border-right: solid $surface-lighten-2;
        padding: 0 1;
    }

    #history-list-label {
        text-style: bold;
        padding: 1 0 0 0;
        color: $text;
    }

    #history-list {
        height: 1fr;
    }

    #history-detail-container {
        width: 1fr;
        padding: 1 2;
        overflow-y: auto;
    }

    #history-detail {
        width: 1fr;
    }

    #history-filter-container {
        height: 3;
        padding: 0 1;
        border-top: solid $surface-lighten-2;
    }

    #history-filter-input {
        width: 1fr;
    }

    .history-item {
        padding: 0 1;
    }
    """

    PAGE_SIZE = 50

    def __init__(self):
        super().__init__()
        self._invocations: list = []
        self._selected = None
        self._page = 0
        self._search = ""
        self._total = 0

    def compose(self) -> ComposeResult:
        from textual.widgets import Input

        yield Header()
        with Horizontal(id="history-main"):
            with Vertical(id="history-list-container"):
                yield Label("History", id="history-list-label")
                yield ListView(id="history-list")
            with Vertical(id="history-detail-container"):
                yield Static(id="history-detail")
        with Horizontal(id="history-filter-container"):
            yield Input(
                placeholder="Search history (skill, question, directory)...",
                id="history-filter-input",
            )
        yield Footer()

    def on_mount(self) -> None:
        self._refresh()

    def _refresh(self) -> None:
        from ashley.history import count, query

        search = self._search or None
        self._total = count(search=search)
        self._invocations = query(
            limit=self.PAGE_SIZE,
            offset=self._page * self.PAGE_SIZE,
            search=search,
        )

        list_view = self.query_one("#history-list", ListView)
        list_view.clear()

        from rich.markup import escape

        if not self._invocations:
            detail = self.query_one("#history-detail", Static)
            if self._search:
                detail.update(f'No results for "{escape(self._search)}"')
            else:
                detail.update(
                    "No history yet.\n\nRun a skill with: ash run <skill> <question>"
                )
            self._selected = None
            self._update_label()
            return

        for inv in self._invocations:
            detached_icon = " [dim]⇢[/dim]" if inv.detached else ""
            label_text = (
                f"[dim]{escape(inv.time_display[5:16])}[/dim]  "
                f"[bold]{escape(inv.skill)}[/bold]{detached_icon}  "
                f"[dim]{escape(inv.question_short[:30])}[/dim]"
            )
            list_view.append(ListItem(Label(label_text, classes="history-item")))

        self._selected = self._invocations[0]
        self._update_detail()
        self._update_label()
        list_view.focus()

    def _update_label(self) -> None:
        from rich.markup import escape

        total_pages = max(1, (self._total + self.PAGE_SIZE - 1) // self.PAGE_SIZE)
        label_text = f"History ({self._total}) — Page {self._page + 1}/{total_pages}"
        if self._search:
            label_text += f' — "{escape(self._search)}"'
        self.query_one("#history-list-label", Label).update(label_text)

    def _update_detail(self) -> None:
        if not self._selected:
            return
        from rich.markup import escape

        from ashley.history import db_path, db_size

        inv = self._selected
        detached_str = (
            "Yes" + (f" (session: {escape(inv.session_id)})" if inv.session_id else "")
            if inv.detached
            else "No"
        )
        text = (
            f"[bold]Invocation #{inv.id}[/bold]\n\n"
            f"Time:       {escape(inv.time_display)} UTC\n"
            f"Skill:      [bold]{escape(inv.skill)}[/bold]\n"
            f"Question:   {escape(inv.question or '(none)')}\n"
            f"Directory:  {escape(inv.cwd)}\n"
            f"Permission: {escape(inv.permission)}\n"
            f"Detached:   {detached_str}\n\n"
            f"[dim]Database: {escape(str(db_path()))} ({escape(db_size())})[/dim]"
        )
        self.query_one("#history-detail", Static).update(text)

    def on_list_view_highlighted(self, event: ListView.Highlighted) -> None:
        idx = event.list_view.index
        if idx is not None and idx < len(self._invocations):
            self._selected = self._invocations[idx]
            self._update_detail()

    def on_input_submitted(self, event) -> None:
        from textual.widgets import Input

        if isinstance(event.input, Input) and event.input.id == "history-filter-input":
            self._search = event.value.strip()
            self._page = 0
            self._refresh()

    def action_go_back(self) -> None:
        self.app.pop_screen()

    def action_focus_filter(self) -> None:
        from textual.widgets import Input

        self.query_one("#history-filter-input", Input).focus()

    def action_refresh(self) -> None:
        self._refresh()

    def action_delete_entry(self) -> None:
        if not self._selected:
            return
        from ashley.history import delete_one

        delete_one(self._selected.id)
        self.app.notify(f"Deleted #{self._selected.id}")
        self._refresh()

    def action_next_page(self) -> None:
        max_page = max(0, (self._total - 1) // self.PAGE_SIZE)
        if self._page < max_page:
            self._page += 1
            self._refresh()

    def action_prev_page(self) -> None:
        if self._page > 0:
            self._page -= 1
            self._refresh()


# ── Stats Screen ──


def _accent_gradient(app, n: int) -> list[str]:
    """Return ``n`` hex shades gradating across the active accent colour.

    Used to tint stacked bar-chart rows so adjacent bars stay legible.
    """
    from textual.color import Color

    raw = app.get_css_variables().get("accent", "#4A9EFF")
    try:
        base = Color.parse(raw)
    except Exception:
        base = Color.parse("#4A9EFF")

    if n <= 1:
        return [base.hex]

    shades = []
    for i in range(n):
        # Fan out from a darker tint to a lighter one across the list.
        amount = (i / (n - 1) - 0.5) * 0.7  # -0.35 .. +0.35
        shade = base.lighten(amount) if amount >= 0 else base.darken(-amount)
        shades.append(shade.hex)
    return shades


class StatsScreen(Screen):
    """Skill usage analytics — invocation counts by skill."""

    BINDINGS = [
        Binding("escape", "go_back", "Back", show=True),
        Binding("r", "refresh", "Refresh", show=True),
    ]

    CSS = """
    #stats-main {
        height: 1fr;
    }

    #stats-body {
        border: round $surface-lighten-2;
        padding: 1 3;
        margin: 1 1;
        height: 1fr;
        overflow-y: auto;
    }
    """

    def compose(self) -> ComposeResult:
        yield Header(show_clock=True)
        with Vertical(id="stats-main"):
            yield Static(id="stats-body")
        yield Footer()

    def on_mount(self) -> None:
        fade_in(self.query_one("#stats-main"))
        self._refresh_stats()

    def _refresh_stats(self) -> None:
        from ashley.history import stats as hstats

        data = hstats()
        lines: list[str] = ["[b $accent]◆ Analytics[/]", ""]
        lines.append(f"  [$text-muted]Total invocations[/]   [b]{data['total']}[/]")

        if data["top_skills"]:
            lines.append("")
            lines.append("[b $accent-lighten-1]Top skills[/]")
            top = data["top_skills"]
            peak = max(cnt for _, cnt in top)
            # Shade each bar a different tint of the primary colour so
            # neighbouring bars stay distinguishable.
            shades = _accent_gradient(self.app, len(top))
            for (name, cnt), shade in zip(top, shades):
                width = max(1, round(cnt / peak * 24)) if peak else 1
                bar = "█" * width
                lines.append(
                    f"  [b]{name:<12}[/] [{shade}]{bar}[/] [$text-muted]{cnt}[/]"
                )
        else:
            lines.append("")
            lines.append("[$text-muted]No invocations recorded yet.[/]")

        self.query_one("#stats-body", Static).update("\n".join(lines))

    def action_go_back(self) -> None:
        self.app.pop_screen()

    def action_refresh(self) -> None:
        self._refresh_stats()
        self.app.notify("Refreshed")


# ── Settings Screen (also the first-run setup wizard) ──


# Number of columns in the swatch grid (kept in sync with the CSS grid-size).
SWATCH_COLUMNS = 5


def _swatch_css() -> str:
    """Build per-preset CSS: a solid primary fill, plus an accent stripe
    on the left for dual-tone presets."""
    rules = []
    for key, preset in THEME_PRESETS.items():
        stripe = ""
        if preset["primary"] != preset["accent"]:
            stripe = f" border-left: thick {preset['accent']};"
        rules.append(f"#sw-{key} {{ background: {preset['primary']};{stripe} }}")
    return "\n".join(rules)


class Swatch(Static):
    """A keyboard- and mouse-operable solid-colour block for picking a preset.

    Enter/Space selects; arrow navigation across the whole settings screen is
    handled by :class:`SettingsScreen` so movement flows between sections.
    """

    can_focus = True

    class Picked(Message):
        """Posted when a swatch is chosen (by click or keyboard)."""

        def __init__(self, key: str) -> None:
            self.key = key
            super().__init__()

    def __init__(self, key: str, label: str) -> None:
        super().__init__(label, id=f"sw-{key}", classes="swatch")
        self._key = key

    def on_click(self) -> None:
        self.post_message(self.Picked(self._key))

    def on_key(self, event) -> None:
        if event.key in ("enter", "space"):
            event.stop()
            self.post_message(self.Picked(self._key))


class ModeChip(Static):
    """A focusable Dark/Light toggle chip.

    Enter/Space (or click) selects the mode; arrow movement is handled by
    the parent screen so it participates in the unified navigation grid.
    """

    can_focus = True

    class Picked(Message):
        """Posted when a mode chip is chosen."""

        def __init__(self, mode: str) -> None:
            self.mode = mode
            super().__init__()

    def __init__(self, mode: str) -> None:
        super().__init__(id=f"mode-{mode}", classes="mode-chip")
        self._mode = mode

    def on_click(self) -> None:
        self.post_message(self.Picked(self._mode))

    def on_key(self, event) -> None:
        if event.key in ("enter", "space"):
            event.stop()
            self.post_message(self.Picked(self._mode))


class SettingsScreen(Screen):
    """Appearance settings — light/dark mode and colour preset.

    Doubles as the first-run setup wizard when ``first_run`` is set.
    Changes preview live and are saved to ``~/.ashley/theme.json``.
    """

    BINDINGS = [
        Binding("escape", "go_back", "Back", show=True),
        Binding("up", "nav('up')", "Up", show=False),
        Binding("down", "nav('down')", "Down", show=False),
        Binding("left", "nav('left')", "Left", show=False),
        Binding("right", "nav('right')", "Right", show=False),
    ]

    CSS = """
    #settings-main {
        height: 1fr;
        overflow-y: auto;
    }

    #settings-card {
        border: round $surface-lighten-2;
        padding: 1 3;
        margin: 1 1;
        height: auto;
    }

    #settings-intro {
        color: $text-muted;
        padding: 0 0 1 0;
    }

    .settings-h {
        text-style: bold;
        color: $accent-lighten-1;
        padding: 1 0 0 0;
    }

    #mode-row {
        height: 3;
        padding: 1 0 0 0;
    }

    .mode-chip {
        width: auto;
        height: 3;
        min-width: 12;
        margin: 0 2 0 0;
        padding: 1 3;
        content-align: center middle;
        text-style: bold;
        background: $surface-lighten-1;
        color: $text;
    }

    .mode-chip.-selected {
        background: $accent;
        color: $surface;
    }

    .mode-chip:focus {
        outline: solid $foreground;
    }

    #swatch-grid {
        grid-size: 5;
        grid-gutter: 1;
        grid-rows: 3;
        height: 7;
        padding: 1 0 0 0;
    }

    .swatch {
        width: 1fr;
        height: 3;
        min-width: 0;
        content-align: center middle;
        color: auto 90%;
        text-style: bold;
    }

    .swatch:focus {
        outline: solid $foreground;
    }

    .swatch.-selected {
        outline: thick $foreground;
    }

    #settings-actions {
        height: auto;
        padding: 1 0 0 0;
    }

    #done-btn {
        background: $accent;
        color: $surface;
        border: none;
        text-style: bold;
        width: auto;
        height: 3;
        padding: 1 4;
    }

    #done-btn:focus {
        outline: solid $foreground;
    }

    #done-btn:hover {
        background: $accent-lighten-1;
    }
    """ + _swatch_css()

    def __init__(self, first_run: bool = False):
        super().__init__()
        self._first_run = first_run
        saved = load_theme()
        self._mode = saved["mode"]
        self._preset = saved["preset"]

    def compose(self) -> ComposeResult:
        yield Header(show_clock=True)
        with Vertical(id="settings-main"):
            with Vertical(id="settings-card"):
                lead = (
                    "Welcome to Ashley — pick a look to get started."
                    if self._first_run
                    else "Adjust Ashley's appearance. Changes preview instantly."
                )
                intro = (
                    f"{lead}\n[dim]Arrows move · Enter selects · Esc saves & exits[/]"
                )
                yield Static(intro, id="settings-intro")

                yield Label("Mode", classes="settings-h")
                with Horizontal(id="mode-row"):
                    yield ModeChip("dark")
                    yield ModeChip("light")

                yield Label("Primary colour / preset", classes="settings-h")
                with Grid(id="swatch-grid"):
                    for key, preset in THEME_PRESETS.items():
                        yield Swatch(key, self._swatch_label(key, preset))

                with Horizontal(id="settings-actions"):
                    yield Button(
                        "Done" if self._first_run else "Save & Close",
                        variant="primary",
                        id="done-btn",
                    )
        yield Footer()

    def _swatch_label(self, key: str, preset: dict) -> str:
        mark = "✓ " if key == self._preset else ""
        return f"{mark}{preset['label']}"

    def on_mount(self) -> None:
        # No fade-in here: animating the container's opacity composites the
        # solid-colour swatch blocks as transparent until they repaint.
        self._refresh_swatches()
        self._refresh_mode_chips()
        # Start focus on the current colour so arrow-key navigation is
        # immediately usable (important for SSH/mosh sessions).
        self.query_one(f"#sw-{self._preset}", Swatch).focus()

    # ── Unified spatial navigation ──

    def _nav_rows(self) -> list[list]:
        """Return the focusable widgets laid out as navigation rows."""
        swatches = [self.query_one(f"#sw-{k}", Swatch) for k in THEME_PRESETS]
        swatch_rows = [
            swatches[i : i + SWATCH_COLUMNS]
            for i in range(0, len(swatches), SWATCH_COLUMNS)
        ]
        return [
            [
                self.query_one("#mode-dark", ModeChip),
                self.query_one("#mode-light", ModeChip),
            ],
            *swatch_rows,
            [self.query_one("#done-btn", Button)],
        ]

    def action_nav(self, direction: str) -> None:
        rows = self._nav_rows()
        focused = self.focused
        pos = next(
            (
                (r, c)
                for r, row in enumerate(rows)
                for c, widget in enumerate(row)
                if widget is focused
            ),
            None,
        )
        if pos is None:
            rows[0][0].focus()
            return

        r, c = pos
        if direction == "left":
            c = max(0, c - 1)
        elif direction == "right":
            c = min(len(rows[r]) - 1, c + 1)
        elif direction == "up":
            r = max(0, r - 1)
            c = min(c, len(rows[r]) - 1)
        elif direction == "down":
            r = min(len(rows) - 1, r + 1)
            c = min(c, len(rows[r]) - 1)
        rows[r][c].focus()

    def on_mode_chip_picked(self, event: ModeChip.Picked) -> None:
        self._mode = event.mode
        self._refresh_mode_chips()
        self._apply()

    def on_button_pressed(self, event: Button.Pressed) -> None:
        if event.button.id == "done-btn":
            self._finish()

    def on_swatch_picked(self, event: Swatch.Picked) -> None:
        self._preset = event.key
        self._refresh_swatches()
        self._apply()

    def _refresh_mode_chips(self) -> None:
        for mode, label in (("dark", "Dark"), ("light", "Light")):
            chip = self.query_one(f"#mode-{mode}", ModeChip)
            mark = "● " if mode == self._mode else "○ "
            chip.update(f"{mark}{label}")
            chip.set_class(mode == self._mode, "-selected")

    def _refresh_swatches(self) -> None:
        for key, preset in THEME_PRESETS.items():
            swatch = self.query_one(f"#sw-{key}", Swatch)
            swatch.update(self._swatch_label(key, preset))
            swatch.set_class(key == self._preset, "-selected")

    def _apply(self) -> None:
        save_theme(self._mode, self._preset)
        apply_theme(self.app)

    def _finish(self) -> None:
        self._apply()
        if self._first_run:
            self.app.switch_screen(HubScreen())
        else:
            self.app.pop_screen()

    def action_go_back(self) -> None:
        self._finish()


# ── Helpers ──


def _load_skill_summaries() -> list[dict]:
    """Load skill metadata from JSONC definitions."""
    import json5

    from ashley import SKILLS_DIR

    skills = []
    for skill_file in sorted(SKILLS_DIR.glob("*.jsonc")):
        data = json5.load(skill_file.open())
        workflow = data.get("workflow", {})
        steps = workflow.get("steps", [])
        skills.append(
            {
                "stem": skill_file.stem,
                "name": data.get("name", skill_file.stem),
                "description": data.get("description", ""),
                "extends": data.get("extends", ""),
                "components": len(data.get("components", data.get("components+", []))),
                "resources": len(data.get("resources", data.get("resources+", []))),
                "steps": [s.get("name", "") for s in steps],
            }
        )
    return skills


# ── Main App ──


class AshleyApp(App):
    """Ashley — Interactive skill set framework for Claude Code."""

    TITLE = f"Ashley v{ashley.__version__}"
    SUB_TITLE = "Interactive Skill Set for Claude Code"

    CSS = BASE_CSS

    SCREENS = {
        "hub": HubScreen,
    }

    def on_mount(self) -> None:
        apply_theme(self)
        if theme_configured():
            self.push_screen(HubScreen())
        else:
            # First launch — run the quick appearance setup wizard.
            self.push_screen(SettingsScreen(first_run=True))
