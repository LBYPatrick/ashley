"""Ashley History TUI — Browse and manage invocation history."""

from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.widgets import Footer, Header, Input, Label, ListItem, ListView, Static

from ashley.history import Invocation, count, db_path, db_size, query


class HistoryApp(App):
    """TUI for browsing ashley invocation history."""

    TITLE = "Ashley History"
    SUB_TITLE = "Invocation log"

    CSS = """
    #main {
        height: 1fr;
    }

    #list-container {
        width: 50;
        border-right: solid $surface-lighten-2;
        padding: 0 1;
    }

    #list-label {
        text-style: bold;
        padding: 1 0 0 0;
        color: $text;
    }

    #history-list {
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

    #filter-container {
        height: 3;
        padding: 0 1;
        border-top: solid $surface-lighten-2;
    }

    #filter-input {
        width: 1fr;
    }

    .history-item {
        padding: 0 1;
    }
    """

    BINDINGS = [
        Binding("q", "quit", "Quit", show=True),
        Binding("slash", "focus_filter", "Search", show=True),
        Binding("r", "refresh", "Refresh", show=True),
        Binding("d", "delete_entry", "Delete", show=True),
        Binding("n", "next_page", "Next", show=True),
        Binding("p", "prev_page", "Prev", show=True),
        Binding("escape", "clear_filter", "Clear", show=False),
    ]

    PAGE_SIZE = 50

    def __init__(self):
        super().__init__()
        self._invocations: list[Invocation] = []
        self._selected: Invocation | None = None
        self._page = 0
        self._search = ""
        self._total = 0

    def compose(self) -> ComposeResult:
        yield Header()
        with Horizontal(id="main"):
            with Vertical(id="list-container"):
                yield Label("History", id="list-label")
                yield ListView(id="history-list")
            with Vertical(id="detail-container"):
                yield Static(id="detail")
        with Horizontal(id="filter-container"):
            yield Input(
                placeholder="Search history (skill, question, directory)...",
                id="filter-input",
            )
        yield Footer()

    def on_mount(self) -> None:
        self._refresh()

    def _refresh(self) -> None:
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
            detail = self.query_one("#detail", Static)
            if self._search:
                detail.update(f'No results for "{self._search}"')
            else:
                detail.update(
                    "No invocation history yet.\n\nRun a skill with: ash run <skill> [question]"
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
        page_display = f"Page {self._page + 1}/{total_pages}"
        label_text = f"History ({self._total} total) — {page_display}"
        if self._search:
            label_text += f' — filter: "{self._search}"'
        self.query_one("#list-label", Label).update(label_text)

    def _update_detail(self) -> None:
        if not self._selected:
            return
        inv = self._selected
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
        detail = self.query_one("#detail", Static)
        detail.update(text)

    def on_list_view_highlighted(self, event: ListView.Highlighted) -> None:
        idx = event.list_view.index
        if idx is not None and idx < len(self._invocations):
            self._selected = self._invocations[idx]
            self._update_detail()

    def on_input_submitted(self, event: Input.Submitted) -> None:
        if event.input.id == "filter-input":
            self._search = event.value.strip()
            self._page = 0
            self._refresh()

    def action_focus_filter(self) -> None:
        self.query_one("#filter-input", Input).focus()

    def action_clear_filter(self) -> None:
        self.query_one("#filter-input", Input).value = ""
        self._search = ""
        self._page = 0
        self._refresh()
        self.query_one("#history-list", ListView).focus()

    def action_refresh(self) -> None:
        self._refresh()
        self.notify("Refreshed")

    def action_delete_entry(self) -> None:
        if not self._selected:
            self.notify("Nothing selected", severity="warning")
            return
        from ashley.history import delete_one

        delete_one(self._selected.id)
        self.notify(f"Deleted #{self._selected.id}")
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
