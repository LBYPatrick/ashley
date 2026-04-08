"""Ashley Configuration System.

Loads user configuration from ~/.ashley/config.yaml with support for:
- Hook definitions (pre/post hooks per skill or globally)
- Pipeline definitions (named skill chains)
- Default settings (permission mode, detached preferences)

Config is created with sensible defaults on first access.
"""

from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

CONFIG_DIR = Path.home() / ".ashley"
CONFIG_PATH = CONFIG_DIR / "config.yaml"

_DEFAULT_CONFIG = """\
# Ashley Configuration
# See: https://github.com/lbypatrick/ashley

# Default settings for `ash run`
defaults:
  permission_mode: default  # default | auto | dsp | afk

# Hooks — shell commands that run before/after skill execution.
# Global hooks apply to all skills; per-skill hooks override globals.
#
# Available hook points:
#   before_run  — runs before Claude Code starts
#   after_run   — runs after Claude Code exits (exit code in $ASHLEY_EXIT_CODE)
#   on_error    — runs only when Claude Code exits non-zero
#
# Each hook value can be a string (single command) or list (multiple commands).
# Commands run in the skill's working directory.
# If a before_run hook exits non-zero, the skill run is aborted.
hooks:
  global: {}
    # before_run: "echo 'Starting skill: $ASHLEY_SKILL'"
    # after_run: "echo 'Finished: $ASHLEY_SKILL (exit $ASHLEY_EXIT_CODE)'"
  skills: {}
    # feat:
    #   after_run: "make format"
    # commit:
    #   before_run: "make test"

# Pipelines — named skill chains for common workflows.
# Each pipeline is a list of skills executed sequentially.
# The question is passed to the first skill; subsequent skills
# receive "/a-<skill>" as the trigger.
#
# Use '+' syntax on the CLI: ash pipe feat+commit "add login"
# Or define named pipelines here:
pipelines: {}
  # ship:
  #   - feat
  #   - commit
  #   - changelog
  # review:
  #   - refactor
  #   - commit
"""


@dataclass
class HookConfig:
    """Hook definitions for a scope (global or per-skill)."""

    before_run: list[str] = field(default_factory=list)
    after_run: list[str] = field(default_factory=list)
    on_error: list[str] = field(default_factory=list)


@dataclass
class AshleyConfig:
    """Parsed Ashley configuration."""

    # Defaults
    permission_mode: str = "default"

    # Hooks
    global_hooks: HookConfig = field(default_factory=HookConfig)
    skill_hooks: dict[str, HookConfig] = field(default_factory=dict)

    # Pipelines
    pipelines: dict[str, list[str]] = field(default_factory=dict)

    # Raw data for extension
    raw: dict = field(default_factory=dict)


def _parse_hook_entry(data: Any) -> list[str]:
    """Normalize a hook value to a list of command strings."""
    if data is None:
        return []
    if isinstance(data, str):
        return [data]
    if isinstance(data, list):
        return [str(cmd) for cmd in data]
    return []


def _parse_hook_config(data: dict | None) -> HookConfig:
    """Parse a hook definition dict into a HookConfig."""
    if not data or not isinstance(data, dict):
        return HookConfig()
    return HookConfig(
        before_run=_parse_hook_entry(data.get("before_run")),
        after_run=_parse_hook_entry(data.get("after_run")),
        on_error=_parse_hook_entry(data.get("on_error")),
    )


def load_config() -> AshleyConfig:
    """Load configuration from ~/.ashley/config.yaml.

    Returns defaults if file doesn't exist or is invalid.
    """
    if not CONFIG_PATH.is_file():
        return AshleyConfig()

    try:
        import yaml
    except ImportError:
        # PyYAML not installed — return defaults silently
        return AshleyConfig()

    try:
        raw = yaml.safe_load(CONFIG_PATH.read_text()) or {}
    except Exception:
        return AshleyConfig()

    if not isinstance(raw, dict):
        return AshleyConfig()

    # Parse defaults
    defaults = raw.get("defaults", {}) or {}
    permission_mode = defaults.get("permission_mode", "default")

    # Parse hooks
    hooks_data = raw.get("hooks", {}) or {}
    global_hooks = _parse_hook_config(hooks_data.get("global"))

    skill_hooks: dict[str, HookConfig] = {}
    skills_hooks_data = hooks_data.get("skills", {}) or {}
    for skill_name, hook_data in skills_hooks_data.items():
        if isinstance(hook_data, dict):
            skill_hooks[skill_name] = _parse_hook_config(hook_data)

    # Parse pipelines
    pipelines_data = raw.get("pipelines", {}) or {}
    pipelines: dict[str, list[str]] = {}
    for name, steps in pipelines_data.items():
        if isinstance(steps, list):
            pipelines[name] = [str(s) for s in steps]

    return AshleyConfig(
        permission_mode=permission_mode,
        global_hooks=global_hooks,
        skill_hooks=skill_hooks,
        pipelines=pipelines,
        raw=raw,
    )


def init_config() -> Path:
    """Create the default config file if it doesn't exist.

    Returns the config file path.
    """
    CONFIG_DIR.mkdir(parents=True, exist_ok=True)
    if not CONFIG_PATH.is_file():
        CONFIG_PATH.write_text(_DEFAULT_CONFIG)
    return CONFIG_PATH


def get_hooks_for_skill(config: AshleyConfig, skill: str) -> HookConfig:
    """Get the effective hooks for a skill.

    Per-skill hooks override global hooks per hook-point.
    If a skill defines before_run, it replaces the global before_run
    (not merged). Hook points not defined in the skill fall back to global.
    """
    global_h = config.global_hooks
    skill_h = config.skill_hooks.get(skill)

    if not skill_h:
        return global_h

    return HookConfig(
        before_run=skill_h.before_run or global_h.before_run,
        after_run=skill_h.after_run or global_h.after_run,
        on_error=skill_h.on_error or global_h.on_error,
    )
