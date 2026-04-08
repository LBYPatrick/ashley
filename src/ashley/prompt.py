"""Prompt generator for Ashley.

Reads a generated SKILL.md, strips YAML frontmatter, and combines
it with a user question to produce a ready-to-use prompt.

Supports on-the-fly Jinja2 template rendering using the current
project's detected stack context.
"""

import re
import sys

from ashley import GENERATED_DIR


def strip_frontmatter(text: str) -> str:
    """Remove YAML frontmatter (--- delimited) from the beginning of text."""
    return re.sub(r"^---\n.*?\n---\n*", "", text, count=1, flags=re.DOTALL)


def generate_prompt(
    skill_name: str, question: str, project_context: dict | None = None
) -> str:
    """Build a combined prompt from a skill and user question.

    If project_context is provided, any remaining Jinja2 templates
    in the generated skill are rendered with the project data.
    """
    # Support both "feat" and "a-feat" forms
    candidates = [
        GENERATED_DIR / f"a-{skill_name}" / "SKILL.md",
        GENERATED_DIR / skill_name / "SKILL.md",
    ]

    skill_path = None
    for candidate in candidates:
        if candidate.is_file():
            skill_path = candidate
            break

    if skill_path is None:
        print(f"Error: Skill '{skill_name}' not found.", file=sys.stderr)
        print(f"Searched: {', '.join(str(c) for c in candidates)}", file=sys.stderr)
        print(
            "Run 'ash generate' first, then 'ash list' to see available skills.",
            file=sys.stderr,
        )
        sys.exit(1)

    body = strip_frontmatter(skill_path.read_text())

    # Render Jinja2 templates if context provided
    if project_context and ("{{" in body or "{%" in body):
        from ashley.generate import render_template

        body = render_template(body, project_context)

    parts = [body.strip()]
    if question:
        parts.append(f"\n<command-args>\n{question}\n</command-args>")

    return "\n".join(parts)


def main() -> None:
    if len(sys.argv) < 2:
        print("Usage: python -m ashley.prompt <skill> [question]", file=sys.stderr)
        sys.exit(1)

    skill_name = sys.argv[1]
    question = " ".join(sys.argv[2:]) if len(sys.argv) > 2 else ""
    print(generate_prompt(skill_name, question))


if __name__ == "__main__":
    main()
