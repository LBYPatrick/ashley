"""Ashley — Interactive skill set framework for Claude Code."""

from pathlib import Path

__version__ = (Path(__file__).parent.parent.parent / "VERSION").read_text().strip()

PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
SKILLS_DIR = PROJECT_ROOT / "skills"
GENERATED_DIR = PROJECT_ROOT / "generated"
