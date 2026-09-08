"""Coding-agent backends supported by Ashley.

Ashley drives Claude Code, Codex, Grok Build, OpenCode, and Kilo Code.
This module describes their binaries, skill locations, and invocation flags.
All backends consume the same generated SKILL.md packages.

Everything here is pure — installing binaries and persisting the user's
preference live in :mod:`ashley.install` and :mod:`ashley.config`.
"""

import os
from dataclasses import dataclass
from pathlib import Path

DEFAULT_AGENT = "claude"


@dataclass(frozen=True)
class Agent:
    """Static description of a coding-agent CLI backend.

    Attributes:
        key: Short identifier used in configuration and CLI flags.
        label: Human-readable name.
        binary: Executable looked up on ``PATH``.
        home_env: Environment variable that relocates the agent's config
            directory, if it defines one.
        home_dir: Default config directory name under ``$HOME``.
        install_script: Bootstrap script under ``scripts/``.
        self_update_args: Subcommand that makes the CLI update itself,
            checking whether a new version exists before downloading
            anything. Empty when the agent has no such subcommand.
        docs_url: Where to send the user when auto-install is unavailable.
        skill_trigger: Prefix that names an installed skill in a prompt.
        system_prompt_flag: Flag that appends extra system instructions, or
            ``None`` when the agent has no such flag (the text is folded
            into the user prompt instead).
        dsp_flags: Flags for "skip every permission check".
        auto_flags: Flags for "auto-accept edits without prompting".
        prompt_flag: Option for the initial prompt, or None for positional text.
    """

    key: str
    label: str
    binary: str
    home_env: str
    home_dir: str
    install_script: str
    self_update_args: tuple[str, ...]
    docs_url: str
    skill_trigger: str
    system_prompt_flag: str | None
    dsp_flags: tuple[str, ...]
    auto_flags: tuple[str, ...]
    prompt_flag: str | None = None


CLAUDE = Agent(
    key="claude",
    label="Claude Code",
    binary="claude",
    home_env="CLAUDE_CONFIG_DIR",
    home_dir=".claude",
    install_script="install_claude.sh",
    self_update_args=("update",),
    docs_url="https://claude.ai/download",
    skill_trigger="/",
    system_prompt_flag="--append-system-prompt",
    dsp_flags=("--dangerously-skip-permissions",),
    auto_flags=("--permission-mode", "auto"),
)

CODEX = Agent(
    key="codex",
    label="OpenAI Codex",
    binary="codex",
    home_env="CODEX_HOME",
    home_dir=".codex",
    install_script="install_codex.sh",
    self_update_args=("update",),
    docs_url="https://developers.openai.com/codex",
    skill_trigger="$",
    # Codex has no separate system-prompt flag; instructions are prepended
    # to the prompt itself.
    system_prompt_flag=None,
    dsp_flags=("--dangerously-bypass-approvals-and-sandbox",),
    # Closest analogue to Claude's auto mode: never stop to ask, but stay
    # sandboxed to the workspace.
    auto_flags=("--sandbox", "workspace-write", "--ask-for-approval", "never"),
)

GROK = Agent(
    key="grok",
    label="Grok Build",
    binary="grok",
    home_env="GROK_HOME",
    home_dir=".grok",
    install_script="install_grok.sh",
    self_update_args=("update",),
    docs_url="https://docs.x.ai/build/overview",
    skill_trigger="/",
    system_prompt_flag=None,
    dsp_flags=("--always-approve",),
    auto_flags=("--permission-mode", "auto"),
)

OPENCODE = Agent(
    key="opencode",
    label="OpenCode",
    binary="opencode",
    home_env="OPENCODE_CONFIG_DIR",
    home_dir=".config/opencode",
    install_script="install_opencode.sh",
    self_update_args=("upgrade",),
    docs_url="https://opencode.ai/docs/",
    skill_trigger="Use the skill ",
    system_prompt_flag=None,
    dsp_flags=("--auto",),
    auto_flags=("--auto",),
    prompt_flag="--prompt",
)

KILO = Agent(
    key="kilo",
    label="Kilo Code",
    binary="kilo",
    home_env="",
    home_dir=".kilo",
    install_script="install_kilo.sh",
    self_update_args=("upgrade",),
    docs_url="https://kilo.ai/docs/code-with-ai/platforms/cli",
    skill_trigger="Use the skill ",
    system_prompt_flag=None,
    dsp_flags=("--auto",),
    auto_flags=("--auto",),
    prompt_flag="--prompt",
)

AGENTS: dict[str, Agent] = {a.key: a for a in (CLAUDE, CODEX, GROK, OPENCODE, KILO)}
AGENT_KEYS: tuple[str, ...] = tuple(AGENTS)


def get_agent(agent: "Agent | str | None") -> Agent:
    """Resolve an agent key to its :class:`Agent`.

    Args:
        agent: An :class:`Agent`, an agent key, or ``None``.

    Returns:
        The matching agent, or the default backend for unknown keys.
    """
    if isinstance(agent, Agent):
        return agent
    return AGENTS.get((agent or "").strip().lower(), AGENTS[DEFAULT_AGENT])


def is_agent(key: str | None) -> bool:
    """Return True when *key* names a supported backend."""
    return (key or "").strip().lower() in AGENTS


def skills_dir(agent: "Agent | str | None") -> Path:
    """Return the directory the agent scans for ``SKILL.md`` packages.

    Honours the agent's own config-directory override (``CLAUDE_CONFIG_DIR``
    or ``CODEX_HOME``) so skills land where the agent will actually look.
    """
    spec = get_agent(agent)
    root = os.environ.get(spec.home_env)
    base = Path(root).expanduser() if root else Path.home() / spec.home_dir
    if spec.key == "opencode" and not root:
        base = (
            Path(os.environ.get("XDG_CONFIG_HOME", str(Path.home() / ".config")))
            / "opencode"
        )
    return base / "skills"


def permission_args(
    agent: "Agent | str | None",
    *,
    dsp: bool = False,
    auto: bool = False,
    afk: bool = False,
) -> tuple[list[str], str]:
    """Map Ashley's run modes onto the agent's permission flags.

    ``afk`` implies ``dsp``; ``dsp`` wins over ``auto``.

    Args:
        agent: Backend to build flags for.
        dsp: Skip every permission check.
        auto: Auto-accept edits without prompting.
        afk: Fully autonomous operation.

    Returns:
        A ``(flags, permission_mode)`` pair, where ``permission_mode`` is
        one of ``"default"``, ``"auto"``, ``"dsp"`` or ``"afk"``.
    """
    spec = get_agent(agent)
    if dsp or afk:
        flags, mode = list(spec.dsp_flags), "dsp"
    elif auto:
        flags, mode = list(spec.auto_flags), "auto"
    else:
        flags, mode = [], "default"
    if afk:
        mode = "afk"
    return flags, mode


def skill_trigger(agent: "Agent | str | None", skill: str, question: str = "") -> str:
    """Build the prompt text that invokes an installed skill.

    Claude Code triggers skills as slash commands (``/a-feat``); Codex
    mentions them with a ``$`` prefix (``$a-feat``).
    """
    spec = get_agent(agent)
    trigger = f"{spec.skill_trigger}a-{skill}"
    return f"{trigger} {question}" if question else trigger


def select_agent(
    use_claude: bool = False,
    use_codex: bool = False,
    use_grok: bool = False,
    use_opencode: bool = False,
    use_kilo: bool = False,
) -> str | None:
    """Resolve mutually exclusive per-run backend flags."""
    selected = [
        key
        for key, enabled in zip(
            AGENT_KEYS, (use_claude, use_codex, use_grok, use_opencode, use_kilo)
        )
        if enabled
    ]
    if len(selected) > 1:
        raise ValueError("Choose only one coding agent per run.")
    return selected[0] if selected else None
