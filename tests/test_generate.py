"""Tests for the Ashley skill generator."""

from ashley import GENERATED_DIR, SKILLS_DIR
from ashley.generate import generate, load_skill_data, resolve_skill_data


def test_load_skill_data():
    """Verify JSONC skill definitions load correctly."""
    data = load_skill_data("coding")
    assert data["name"] == "a-coding"
    assert "description" in data
    assert "components" in data


def test_resolve_inheritance():
    """Verify skill inheritance resolves correctly."""
    data = load_skill_data("feat")
    resolved = resolve_skill_data(data)
    # feat extends refactor — should have merged components
    assert "extends" not in resolved
    assert "components" in resolved
    assert len(resolved["components"]) > 0


def test_generate_all_skills():
    """Verify all skills generate without errors."""
    generate()
    skill_files = sorted(SKILLS_DIR.glob("*.jsonc"))
    for skill_file in skill_files:
        output_dir = GENERATED_DIR / f"a-{skill_file.stem}"
        assert output_dir.is_dir(), f"Missing output for {skill_file.stem}"
        skill_md = output_dir / "SKILL.md"
        assert skill_md.is_file(), f"Missing SKILL.md for {skill_file.stem}"
        content = skill_md.read_text()
        assert content.startswith("---"), f"Missing frontmatter in {skill_file.stem}"
        assert len(content) > 100, f"Suspiciously short output for {skill_file.stem}"


def test_resources_are_inlined():
    """Verify resources are inlined as code blocks, not file-path refs."""
    generate()
    # brainstorm has resources — check they're inlined
    brainstorm_md = GENERATED_DIR / "a-brainstorm" / "SKILL.md"
    content = brainstorm_md.read_text()
    assert "## Reference Resources" in content
    # Should have code blocks, not absolute path references
    assert "```python" in content or "```typescript" in content
    # Should NOT have bare file-path bullets
    import re

    path_refs = re.findall(r"^- `/[^`]+`$", content, re.MULTILINE)
    assert len(path_refs) == 0, f"Found file-path references: {path_refs}"


def test_no_inline_file_generated():
    """Verify no SKILL.inline.md files are generated."""
    generate()
    inline_files = list(GENERATED_DIR.glob("*/SKILL.inline.md"))
    assert len(inline_files) == 0, f"Found unexpected inline files: {inline_files}"
