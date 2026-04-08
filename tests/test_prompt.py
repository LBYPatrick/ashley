"""Tests for the Ashley prompt generator."""

from ashley.generate import generate
from ashley.prompt import generate_prompt, strip_frontmatter


def test_strip_frontmatter():
    text = "---\nname: test\ndescription: foo\n---\n\nBody content here."
    result = strip_frontmatter(text)
    assert result.strip() == "Body content here."
    assert "---" not in result


def test_generate_prompt_with_question():
    generate()
    result = generate_prompt("feat", "Add a login page")
    assert "<command-args>" in result
    assert "Add a login page" in result
    assert "---" not in result.split("\n")[0]  # frontmatter stripped


def test_generate_prompt_without_question():
    generate()
    result = generate_prompt("coding", "")
    assert "<command-args>" not in result
    assert len(result) > 100


def test_generate_prompt_accepts_prefixed_name():
    generate()
    result = generate_prompt("a-feat", "test")
    assert "<command-args>" in result
