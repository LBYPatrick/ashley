"""Ashley TUI — Main hub with feature navigation."""

from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.screen import Screen
from textual.widgets import Footer, Header, Label, ListItem, ListView, Static

import ashley

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
        "description": "View skill usage analytics, success rates, and performance insights.",
        "icon": "◆",
    },
]


class HubScreen(Screen):
    """Main hub screen showing Ashley features."""

    CSS = """
    #hub-main {
        height: 1fr;
    }

    #feature-list-container {
        width: 36;
        border-right: solid $surface-lighten-2;
        padding: 0 1;
    }

    #feature-list-label {
        text-style: bold;
        padding: 1 0 0 0;
        color: $text;
    }

    #feature-list {
        height: 1fr;
    }

    #feature-detail {
        width: 1fr;
        padding: 2 3;
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
        yield Header()
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
            f"[bold]{f['icon']}  {f['title']}[/bold]\n\n"
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
            self._run_stats()

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

    def _run_stats(self) -> None:
        import subprocess

        with self.app.suspend():
            subprocess.run(["ash", "history", "stats"])
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
        width: 28;
        border-right: solid $surface-lighten-2;
        padding: 0 1;
    }

    #skill-list-label {
        text-style: bold;
        padding: 1 0 0 0;
        color: $text;
    }

    #skill-list {
        height: 1fr;
    }

    #skill-detail-container {
        width: 1fr;
        padding: 1 2;
        overflow-y: auto;
    }

    #skill-detail {
        width: 1fr;
    }

    #question-container {
        height: 3;
        padding: 0 1;
        border-top: solid $surface-lighten-2;
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
        yield Header()
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
        extends_line = f"Extends: **{skill['extends']}**\n" if skill["extends"] else ""
        steps_text = ""
        if skill["steps"]:
            steps_lines = "\n".join(
                f"  {i}. {name}" for i, name in enumerate(skill["steps"], 1)
            )
            steps_text = f"\n**Workflow Steps:**\n{steps_lines}"

        text = (
            f"## {skill['name']}\n\n"
            f"{skill['description']}\n\n"
            f"{extends_line}"
            f"Components: **{skill['components']}** | "
            f"Resources: **{skill['resources']}**"
            f"{steps_text}"
        )
        self.query_one("#skill-detail", Static).update(text)

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
        Binding("l", "view_log", "Log", show=True),
        Binding("k", "kill_session", "Kill", show=True),
        Binding("d", "delete_session", "Delete", show=True),
        Binding("r", "refresh", "Refresh", show=True),
        Binding("c", "cleanup", "Cleanup", show=True),
    ]

    CSS = """
    #sessions-main {
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

    #session-detail-container {
        width: 1fr;
        padding: 1 2;
        overflow-y: auto;
    }

    #session-detail {
        width: 1fr;
    }

    #session-log-container {
        height: 2fr;
        border-top: solid $surface-lighten-2;
        padding: 1 2;
        overflow-y: auto;
    }

    #session-log-label {
        text-style: bold;
        color: $text;
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

    def compose(self) -> ComposeResult:
        yield Header()
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
        self._refresh_sessions()

    def _refresh_sessions(self) -> None:
        from ashley.sessions import load_all_sessions

        self._sessions = load_all_sessions()
        list_view = self.query_one("#session-list", ListView)
        list_view.clear()

        if not self._sessions:
            self.query_one("#session-detail", Static).update(
                "No sessions found.\n\nStart one with: ash run --detached <skill> [question]"
            )
            self.query_one("#session-log-viewer", Static).update("")
            self._selected_session = None
            return

        for session in self._sessions:
            status = session.status()
            icon = "[green]●[/green]" if status == "running" else "[dim]○[/dim]"
            label_text = f"{icon} {session.id}  {session.skill}  ({session.elapsed()})"
            list_view.append(ListItem(Label(label_text, classes="session-item")))

        self._selected_session = self._sessions[0]
        self._update_session_detail()
        list_view.focus()

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
        q = s.question[:80] + "..." if len(s.question) > 80 else s.question
        if not q:
            q = "(no question)"

        text = (
            f"[bold]Session {s.id}[/bold]\n\n"
            f"Status:     {status_display}\n"
            f"Skill:      [bold]{s.skill}[/bold]\n"
            f"Question:   {q}\n"
            f"Started:    {s.started_at[:19].replace('T', ' ')} UTC\n"
            f"Elapsed:    {s.elapsed()}\n"
            f"Directory:  {s.cwd}\n"
            f"tmux:       {s.tmux_session}"
        )
        self.query_one("#session-detail", Static).update(text)

        # Update log
        import re

        from ashley.sessions import read_log

        content = read_log(self._selected_session, tail=50)
        content = re.sub(r"\x1b\[[0-9;]*[a-zA-Z]", "", content)
        content = re.sub(r"\x1b\][^\x07]*\x07", "", content)
        self.query_one("#session-log-viewer", Static).update(
            content if content.strip() else "(empty log)"
        )

    def on_list_view_highlighted(self, event: ListView.Highlighted) -> None:
        idx = event.list_view.index
        if idx is not None and idx < len(self._sessions):
            self._selected_session = self._sessions[idx]
            self._update_session_detail()

    def action_go_back(self) -> None:
        self.app.pop_screen()

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

        if not self._invocations:
            detail = self.query_one("#history-detail", Static)
            if self._search:
                detail.update(f'No results for "{self._search}"')
            else:
                detail.update(
                    "No history yet.\n\nRun a skill with: ash run <skill> [question]"
                )
            self._selected = None
            self._update_label()
            return

        for inv in self._invocations:
            detached_icon = " [dim]⇢[/dim]" if inv.detached else ""
            label_text = (
                f"[dim]{inv.time_display[5:16]}[/dim]  "
                f"[bold]{inv.skill}[/bold]{detached_icon}  "
                f"[dim]{inv.question_short[:30]}[/dim]"
            )
            list_view.append(ListItem(Label(label_text, classes="history-item")))

        self._selected = self._invocations[0]
        self._update_detail()
        self._update_label()
        list_view.focus()

    def _update_label(self) -> None:
        total_pages = max(1, (self._total + self.PAGE_SIZE - 1) // self.PAGE_SIZE)
        label_text = f"History ({self._total}) — Page {self._page + 1}/{total_pages}"
        if self._search:
            label_text += f' — "{self._search}"'
        self.query_one("#history-list-label", Label).update(label_text)

    def _update_detail(self) -> None:
        if not self._selected:
            return
        inv = self._selected
        from ashley.history import db_path, db_size

        detached_str = (
            "Yes" + (f" (session: {inv.session_id})" if inv.session_id else "")
            if inv.detached
            else "No"
        )
        text = (
            f"[bold]Invocation #{inv.id}[/bold]\n\n"
            f"Time:       {inv.time_display} UTC\n"
            f"Skill:      [bold]{inv.skill}[/bold]\n"
            f"Question:   {inv.question or '(none)'}\n"
            f"Directory:  {inv.cwd}\n"
            f"Permission: {inv.permission}\n"
            f"Detached:   {detached_str}\n\n"
            f"[dim]Database: {db_path()} ({db_size()})[/dim]"
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

    SCREENS = {
        "hub": HubScreen,
    }

    def on_mount(self) -> None:
        self.push_screen(HubScreen())
