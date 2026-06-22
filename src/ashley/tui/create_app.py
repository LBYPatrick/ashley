"""Ashley Interactive Skill Builder TUI.

A wizard-style interface for creating new skill definitions.
Guides the user through: name, description, components, resources,
workflow steps, and checklist items. Outputs a JSONC file to skills/.
"""

import json

from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical
from textual.screen import Screen
from textual.widgets import (
    Button,
    Footer,
    Header,
    Input,
    Label,
    Static,
    TextArea,
)

import ashley
from ashley import SKILLS_DIR

# ── Available components (scanned from disk) ──


def _scan_components() -> list[dict]:
    """Scan components/ for available markdown files."""
    from ashley import PROJECT_ROOT

    components_dir = PROJECT_ROOT / "components"
    if not components_dir.is_dir():
        return []
    results = []
    for f in sorted(components_dir.rglob("*.md")):
        rel = str(f.relative_to(PROJECT_ROOT))
        name = f.stem.replace("-", " ").title()
        results.append({"path": rel, "name": name, "selected": False})
    return results


def _scan_resources() -> list[dict]:
    """Scan res/ for available resource files."""
    from ashley import PROJECT_ROOT

    res_dir = PROJECT_ROOT / "res"
    if not res_dir.is_dir():
        return []
    results = []
    for f in sorted(res_dir.rglob("*")):
        if f.is_file() and not f.name.startswith("."):
            rel = str(f.relative_to(PROJECT_ROOT))
            results.append({"path": rel, "name": f.name, "selected": False})
    return results


def _scan_existing_skills() -> list[str]:
    """Get names of existing skills for 'extends' dropdown."""
    if not SKILLS_DIR.is_dir():
        return []
    return sorted(f.stem for f in SKILLS_DIR.glob("*.jsonc"))


class CreateScreen(Screen):
    """Multi-step skill creation wizard."""

    CSS = """
    #create-main {
        height: 1fr;
        padding: 1 2;
    }

    .form-label {
        margin-top: 1;
        text-style: bold;
    }

    .form-help {
        color: $text-muted;
        margin-bottom: 1;
    }

    #step-indicator {
        text-style: bold;
        padding: 1 0;
        color: $accent;
    }

    #preview-area {
        height: 1fr;
        border: solid $surface-lighten-2;
        padding: 1;
        overflow-y: auto;
    }

    #component-list {
        height: 12;
    }

    #resource-list {
        height: 8;
    }

    .workflow-step-input {
        margin-bottom: 1;
    }

    #button-row {
        height: 3;
        padding: 1 0;
    }
    """

    BINDINGS = [
        Binding("escape", "go_back", "Back/Cancel", show=True),
        Binding("ctrl+s", "save_skill", "Save", show=True),
    ]

    def __init__(self):
        super().__init__()
        self._step = 0  # 0=basics, 1=components, 2=workflow, 3=preview
        self._skill_data = {
            "name": "",
            "description": "",
            "extends": "",
            "preamble": "",
            "components": [],
            "resources": [],
            "workflow_name": "",
            "workflow_steps": [],
            "checklist": [],
        }
        self._components = _scan_components()
        self._resources = _scan_resources()
        self._existing_skills = _scan_existing_skills()

    def compose(self) -> ComposeResult:
        yield Header()
        with Vertical(id="create-main"):
            yield Static("Step 1/4 — Basics", id="step-indicator")

            # Step 1: Basics
            yield Label("Skill Name", classes="form-label")
            yield Static("Lowercase, no spaces (e.g., 'lint-fix')", classes="form-help")
            yield Input(placeholder="my-skill", id="skill-name")

            yield Label("Description", classes="form-label")
            yield Input(
                placeholder="What this skill does (shown in skill list)",
                id="skill-description",
            )

            yield Label("Extends (optional)", classes="form-label")
            yield Static(
                f"Available: {', '.join(self._existing_skills) or '(none)'}",
                classes="form-help",
            )
            yield Input(placeholder="(leave empty for standalone)", id="skill-extends")

            yield Label("Preamble", classes="form-label")
            yield Static("System prompt intro for Claude", classes="form-help")
            yield TextArea(id="skill-preamble")

            yield Label("", id="preview-area")

            with Horizontal(id="button-row"):
                yield Button("Next →", id="btn-next", variant="primary")
                yield Button("Save", id="btn-save", variant="success")

        yield Footer()

    def on_mount(self) -> None:
        self._update_step()

    def _update_step(self) -> None:
        indicator = self.query_one("#step-indicator", Static)
        step_names = ["Basics", "Components & Resources", "Workflow", "Preview & Save"]
        indicator.update(f"Step {self._step + 1}/4 — {step_names[self._step]}")

    def on_button_pressed(self, event: Button.Pressed) -> None:
        if event.button.id == "btn-next":
            self._collect_current_step()
            if self._step < 3:
                self._step += 1
                self._update_step()
                if self._step == 3:
                    self._show_preview()
        elif event.button.id == "btn-save":
            self._collect_current_step()
            self._save_skill()

    def _collect_current_step(self) -> None:
        """Collect form data from the current step."""
        if self._step == 0:
            try:
                self._skill_data["name"] = self.query_one(
                    "#skill-name", Input
                ).value.strip()
                self._skill_data["description"] = self.query_one(
                    "#skill-description", Input
                ).value.strip()
                self._skill_data["extends"] = self.query_one(
                    "#skill-extends", Input
                ).value.strip()
                self._skill_data["preamble"] = self.query_one(
                    "#skill-preamble", TextArea
                ).text.strip()
            except Exception:
                pass

    def _show_preview(self) -> None:
        """Show a JSON preview of the skill definition."""
        preview = self._build_jsonc()
        try:
            self.query_one("#preview-area", Label).update(
                f"[bold]JSONC Preview:[/bold]\n\n{preview}"
            )
        except Exception:
            pass

    def _build_jsonc(self) -> str:
        """Build the JSONC content from collected data."""
        d = self._skill_data
        name = d["name"] or "my-skill"

        obj: dict = {
            "name": f"a-{name}",
            "description": d["description"],
        }

        if d["extends"]:
            obj["extends"] = d["extends"]
            if d["components"]:
                obj["components+"] = d["components"]
            if d["resources"]:
                obj["resources+"] = d["resources"]
        else:
            obj["components"] = d["components"] or [
                "components/guidelines-coding.md",
                "components/quality-assurance.md",
            ]
            obj["resources"] = d["resources"] or []

        obj["output"] = f"generated/a-{name}/SKILL.md"

        if d["preamble"]:
            obj["preamble"] = d["preamble"]

        if d["workflow_steps"]:
            obj["workflow"] = {
                "name": d["workflow_name"] or f"Run {name}",
                "steps": d["workflow_steps"],
            }

        if d["checklist"]:
            obj["checklist"] = d["checklist"]
        else:
            obj["checklist"] = [
                "Task completed as specified.",
                "Code follows project conventions.",
                "Tests pass (if configured).",
            ]

        return json.dumps(obj, indent=2)

    def _save_skill(self) -> None:
        """Write the skill JSONC file."""
        d = self._skill_data
        name = d["name"] or "my-skill"

        SKILLS_DIR.mkdir(parents=True, exist_ok=True)
        output_path = SKILLS_DIR / f"{name}.jsonc"

        content = self._build_jsonc()
        output_path.write_text(content + "\n")

        self.app.notify(f"Skill saved: {output_path}")

    def action_go_back(self) -> None:
        if self._step > 0:
            self._step -= 1
            self._update_step()
        else:
            self.app.pop_screen()

    def action_save_skill(self) -> None:
        self._collect_current_step()
        self._save_skill()


class CreateApp(App):
    """Standalone app for the skill builder."""

    TITLE = f"Ashley v{ashley.__version__}"
    SUB_TITLE = "Create New Skill"

    def on_mount(self) -> None:
        self.push_screen(CreateScreen())
