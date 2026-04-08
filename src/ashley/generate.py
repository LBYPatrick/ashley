"""Ashley Skill Generator.

Reads JSONC skill definitions from skills/ and assembles them into
complete markdown prompts in generated/. Resources are always inlined
as code blocks for maximum portability.

Supports skill inheritance via the "extends" field — a skill can
reference another skill by filename (without .jsonc) and inherit
its components and resources. Use "components+" / "resources+" to
append to the inherited lists instead of replacing them.

Components support Jinja2 templating with project context — use
{{ project.python }}, {% if project.react %}, etc. to conditionally
include content based on the detected project stack.
"""

import sys
from pathlib import Path

import jinja2
import json5

from ashley import GENERATED_DIR, PROJECT_ROOT, SKILLS_DIR

# Jinja2 environment — undefined variables render as empty string
_JINJA_ENV = jinja2.Environment(
    undefined=jinja2.Undefined,
    keep_trailing_newline=True,
)

# ANSI colors
GREEN = "\033[0;32m"
CYAN = "\033[0;36m"
RED = "\033[0;31m"
BOLD = "\033[1m"
NC = "\033[0m"

# File extension -> code fence language
EXT_LANG_MAP: dict[str, str] = {
    ".py": "python",
    ".ts": "typescript",
    ".tsx": "typescript",
    ".js": "javascript",
    ".jsx": "javascript",
    ".json": "json",
    ".sh": "bash",
    ".md": "",
}


def render_workflow(workflow: dict) -> str:
    """Render a structured workflow definition into markdown."""
    lines: list[str] = []
    wf_name = workflow["name"]
    steps = workflow["steps"]

    lines.append(f"## Workflow: {wf_name}\n")
    lines.append("**IMPORTANT — Sequential Workflow Orchestration Rules:**")
    lines.append("- Execute steps in numbered order. Never jump ahead.")
    lines.append(
        "- After completing each step, validate it succeeded before proceeding."
    )
    lines.append(
        "- If validation fails, fix the issue in the current step "
        "before moving on. Retry up to 2 times."
    )
    lines.append(
        "- If a step fails after retries, report to the user: "
        "what failed, what you tried, and suggested next steps."
    )
    lines.append(
        "- On unrecoverable failure, rollback partial changes so "
        "the codebase is left clean."
    )
    lines.append(
        '- Announce each step before starting (e.g., "Step 1: '
        'Analyzing the codebase...").\n'
    )

    for i, step in enumerate(steps, 1):
        step_name = step["name"]
        instructions = step["instructions"]
        validation = step["validation"]

        lines.append(f"### Step {i}: {step_name}")
        for j, instruction in enumerate(instructions, 1):
            lines.append(f"{j}. {instruction}")
        lines.append(f"- **Validation:** {validation}\n")

    return "\n".join(lines)


def render_checklist(checklist: list[str]) -> str:
    """Render a completion checklist from a list of items."""
    lines = ["## Completion Checklist\n"]
    for item in checklist:
        lines.append(f"- [ ] {item}")
    return "\n".join(lines)


def load_skill_data(skill_name: str) -> dict:
    """Load and return raw JSONC data for a skill by its filename stem."""
    skill_path = SKILLS_DIR / f"{skill_name}.jsonc"
    if not skill_path.is_file():
        raise FileNotFoundError(f"Base skill not found: {skill_path}")
    return json5.load(skill_path.open())


def resolve_skill_data(data: dict) -> dict:
    """Resolve inheritance via the 'extends' field.

    Supports chained inheritance (A extends B extends C).
    """
    extends = data.get("extends")
    if not extends:
        return data

    base_raw = load_skill_data(extends)
    base = resolve_skill_data(base_raw)

    base_components: list[str] = base.get("components", [])
    base_resources: list[str] = base.get("resources", [])

    if "components" in data:
        components = data["components"]
    elif "components+" in data:
        components = base_components + data["components+"]
    else:
        components = base_components

    if "resources" in data:
        resources = data["resources"]
    elif "resources+" in data:
        resources = base_resources + data["resources+"]
    else:
        resources = base_resources

    resolved = dict(data)
    resolved["components"] = components
    resolved["resources"] = resources
    resolved.pop("extends", None)
    resolved.pop("components+", None)
    resolved.pop("resources+", None)

    return resolved


def render_template(text: str, context: dict | None = None) -> str:
    """Render Jinja2 template in text with project context.

    If the text contains no Jinja2 markers, returns it unchanged.
    If context is None, returns text unchanged (generation-time only).
    """
    if context is None:
        return text
    # Quick check — skip Jinja2 if no template syntax present
    if "{{" not in text and "{%" not in text:
        return text
    try:
        template = _JINJA_ENV.from_string(text)
        return template.render(project=context)
    except jinja2.TemplateSyntaxError:
        # If template syntax is invalid, return original text
        return text


def inline_resource(resource_path: Path) -> str:
    """Read a resource file and return it as a fenced code block."""
    lang = EXT_LANG_MAP.get(resource_path.suffix, "")
    content = resource_path.read_text().rstrip()
    return f"### `{resource_path.name}`\n\n```{lang}\n{content}\n```\n"


def assemble_skill(
    skill_path: Path, project_context: dict | None = None
) -> tuple[str, str | None]:
    """Parse a JSONC skill definition and assemble its markdown.

    Resources are always inlined as code blocks.
    If project_context is provided, Jinja2 templates in components
    are rendered with project detection data.
    Returns (output_path_relative, error_message_or_none).
    """
    raw_data = json5.load(skill_path.open())
    data = resolve_skill_data(raw_data)

    skill_name: str = data["name"]
    description: str = data["description"]
    output_rel: str = data["output"]
    preamble: str = data.get("preamble", "")
    epilogue: str = data.get("epilogue", "")
    components: list[str] = data.get("components", [])
    resources: list[str] = data.get("resources", [])
    workflow: dict | None = data.get("workflow")
    checklist: list[str] | None = data.get("checklist")

    parts: list[str] = []

    # YAML frontmatter
    parts.append("---")
    parts.append(f"name: {skill_name}")
    parts.append(f"description: {description}")
    parts.append("---\n")

    if preamble:
        parts.append(f"{preamble}\n")

    if workflow:
        parts.append(render_workflow(workflow))

    # Components — inlined markdown blocks with optional Jinja2 rendering
    parts.append("---\n")
    for component in components:
        component_path = PROJECT_ROOT / component
        if any(c in component for c in ("*", "?", "[")):
            matches = sorted(PROJECT_ROOT.glob(component))
            if not matches:
                parts.append(
                    f"<!-- WARNING: No files matched pattern: {component} -->\n"
                )
            for match in matches:
                parts.append(render_template(match.read_text(), project_context))
                parts.append("\n")
        elif component_path.is_file():
            parts.append(
                render_template(component_path.read_text(), project_context)
            )
            parts.append("\n")
        else:
            parts.append(f"<!-- WARNING: Component not found: {component} -->\n")

    # Resources — always inlined as code blocks
    if resources:
        resolved_resources: list[Path] = []
        for resource in resources:
            resource_path = PROJECT_ROOT / resource
            if resource_path.is_file():
                resolved_resources.append(resource_path)
            else:
                parts.append(f"<!-- WARNING: Resource not found: {resource} -->\n")

        if resolved_resources:
            parts.append("---\n")
            parts.append("## Reference Resources\n")
            parts.append("Use these code templates as reference when implementing:\n")
            for p in resolved_resources:
                parts.append(inline_resource(p))

    # Checklist
    if checklist:
        parts.append("---\n")
        parts.append(render_checklist(checklist))
        parts.append("")
    elif epilogue:
        parts.append("---\n")
        parts.append(f"{epilogue}\n")

    content = "\n".join(parts)
    output_path = PROJECT_ROOT / output_rel
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(content)

    return output_rel, None


def list_skills() -> None:
    """Print all available skills with their descriptions."""
    skill_files = sorted(SKILLS_DIR.glob("*.jsonc"))
    if not skill_files:
        print(f"{RED}No skill definitions found in {SKILLS_DIR}/{NC}")
        sys.exit(1)

    print("Available skills:")
    for skill_file in skill_files:
        data = json5.load(skill_file.open())
        name = skill_file.stem
        desc = data.get("description", "")
        print(f"  {CYAN}{name:<15}{NC} {desc}")


def generate(project_context: dict | None = None) -> None:
    """Generate all skills.

    If project_context is provided, Jinja2 templates in components
    are rendered with the detected project stack data.
    """
    GENERATED_DIR.mkdir(parents=True, exist_ok=True)

    skill_files = sorted(SKILLS_DIR.glob("*.jsonc"))
    if not skill_files:
        print(f"{RED}No skill definitions found in {SKILLS_DIR}/{NC}")
        sys.exit(1)

    print()
    print(f"{BOLD}==========================================={NC}")
    print(f"{BOLD}        Ashley Skill Generator{NC}")
    print(f"{BOLD}==========================================={NC}")
    print()
    print(f"  {CYAN}Skills directory:{NC}  {SKILLS_DIR}")
    print(f"  {CYAN}Output directory:{NC}  {GENERATED_DIR}")
    print(f"  {CYAN}Skills found:{NC}      {len(skill_files)}")
    if project_context:
        detected = [k for k, v in project_context.items() if v is True]
        if detected:
            print(f"  {CYAN}Detected stack:{NC}   {', '.join(detected)}")
    print()

    generated = 0
    failed = 0

    for skill_file in skill_files:
        skill_name = skill_file.stem
        try:
            output_rel, error = assemble_skill(skill_file, project_context)
            if error:
                print(f"  {RED}✗{NC} {skill_name} — {error}")
                failed += 1
            else:
                print(f"  {GREEN}✓{NC} {skill_name} → {output_rel}")
                generated += 1
        except Exception as e:
            print(f"  {RED}✗{NC} {skill_name} — {e}")
            failed += 1

    print()
    print(f"{BOLD}-------------------------------------------{NC}")
    print(f"  {GREEN}Generated:{NC} {generated}")
    if failed > 0:
        print(f"  {RED}Failed:{NC}    {failed}")
    print(f"{BOLD}-------------------------------------------{NC}")
    print()

    if failed > 0:
        sys.exit(1)


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "list":
        list_skills()
    else:
        generate()
