"""Tests for Ashley skill pipelines."""

from ashley.config import AshleyConfig
from ashley.pipeline import resolve_pipeline


def test_resolve_pipeline_plus_syntax():
    config = AshleyConfig()
    result = resolve_pipeline("feat+commit+changelog", config)
    assert result == ["feat", "commit", "changelog"]


def test_resolve_pipeline_named():
    config = AshleyConfig(
        pipelines={"ship": ["feat", "commit", "changelog"]}
    )
    result = resolve_pipeline("ship", config)
    assert result == ["feat", "commit", "changelog"]


def test_resolve_pipeline_named_takes_precedence():
    """Named pipeline wins over treating it as a single skill."""
    config = AshleyConfig(
        pipelines={"feat": ["feat", "commit"]}
    )
    result = resolve_pipeline("feat", config)
    assert result == ["feat", "commit"]


def test_resolve_pipeline_single_skill():
    config = AshleyConfig()
    result = resolve_pipeline("feat", config)
    assert result == ["feat"]


def test_resolve_pipeline_empty():
    config = AshleyConfig()
    result = resolve_pipeline("", config)
    assert result == []
