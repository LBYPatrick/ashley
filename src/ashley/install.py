"""Ashley Skills Installer.

Symlinks generated skills into the coding agent's skills directory
(``~/.claude/skills`` for Claude Code, ``~/.codex/skills`` for Codex) so
they are available across all projects. Both agents read the same
``SKILL.md`` format, so a single generated skill serves either backend.
"""

import shutil
import subprocess
import sys
from collections.abc import Sequence
from pathlib import Path

from ashley import GENERATED_DIR, PROJECT_ROOT
from ashley.agents import AGENT_KEYS, AGENTS, Agent, get_agent, is_agent, skills_dir
from ashley.config import load_agent, save_agent

GREEN = "\033[0;32m"
CYAN = "\033[0;36m"
RED = "\033[0;31m"
YELLOW = "\033[0;33m"
BOLD = "\033[1m"
NC = "\033[0m"


def clean_existing(directory: Path) -> int:
    """Remove any existing ashley symlinks from *directory*.

    Detects ashley symlinks by checking if the target path contains
    'ashley/generated/', so it works across repo relocations.
    """
    if not directory.is_dir():
        return 0

    removed = 0
    for link in sorted(directory.iterdir()):
        if not link.is_symlink():
            continue
        try:
            target = str(link.resolve())
            if "ashley/generated/" in target or target.endswith("ashley/generated"):
                link.unlink()
                print(f"  {YELLOW}✗{NC} Removed stale {link.name}/")
                removed += 1
        except (OSError, ValueError):
            continue
    return removed


def has_skills(agent: Agent | str) -> bool:
    """Return True when the agent's skills directory holds ashley symlinks."""
    directory = skills_dir(agent)
    if not directory.is_dir():
        return False
    for link in directory.iterdir():
        if not link.is_symlink():
            continue
        try:
            if "ashley/generated/" in str(link.resolve()):
                return True
        except (OSError, ValueError):
            continue
    return False


def ensure_agent(agent: Agent | str) -> None:
    """Install the agent's CLI via its native installer if not present."""
    spec = get_agent(agent)
    install_script = PROJECT_ROOT / "scripts" / spec.install_script
    if install_script.is_file():
        subprocess.run(["bash", str(install_script)], check=False)
    elif not shutil.which(spec.binary):
        print(f"{YELLOW}{spec.label} not found. Install it from {spec.docs_url}{NC}")


def _read_choice(prompt: str) -> str:
    """Read one line from the terminal, even when stdin is a pipe.

    ``curl … | bash`` leaves stdin bound to the script itself, so fall back
    to the controlling terminal. Returns an empty string when there is no
    terminal to ask.
    """
    if sys.stdin.isatty():
        try:
            return input(prompt).strip()
        except EOFError:
            return ""
    try:
        with open("/dev/tty") as tty:
            print(prompt, end="", flush=True)
            return tty.readline().strip()
    except OSError:
        return ""


def prompt_for_agents() -> list[str]:
    """Ask which coding agent to set up.

    Falls back to the saved preference when no terminal is available.

    Returns:
        The agent keys to install for, most-preferred first.
    """
    saved = load_agent()
    options = [(spec.key, spec.label) for spec in AGENTS.values()]

    print()
    print(f"{BOLD}Which coding agent should Ashley use?{NC}")
    for i, (key, label) in enumerate(options, 1):
        default_mark = f" {CYAN}(default){NC}" if key == saved else ""
        print(f"  {BOLD}{i}{NC}) {label}{default_mark}")
    print(f"  {BOLD}{len(options) + 1}{NC}) Both — install skills for each")
    print()

    choice = _read_choice(f"  Choice [1-{len(options) + 1}]: ")
    print()

    if choice == str(len(options) + 1) or choice.lower() in ("both", "all"):
        return [saved] + [k for k, _ in options if k != saved]
    for i, (key, _) in enumerate(options, 1):
        if choice == str(i) or choice.lower() == key:
            return [key]
    return [saved]


def _saved_first(keys: list[str]) -> list[str]:
    """Order *keys* so the saved preference stays the default.

    The first entry becomes the new default agent, so a reinstall covering
    several agents must not silently switch the user's choice.
    """
    if len(keys) < 2:
        return keys
    saved = load_agent()
    return sorted(keys, key=lambda key: key != saved)


def resolve_agents(agents: Sequence[str] | None) -> list[str]:
    """Decide which agents to install for.

    Explicit keys win. Otherwise Ashley reinstalls for whichever agents
    already have skills linked, and only asks on a fresh machine.
    """
    if agents:
        return _saved_first([a for a in agents if is_agent(a)])

    existing = [key for key in AGENT_KEYS if has_skills(key)]
    if existing:
        return _saved_first(existing)
    return prompt_for_agents()


def _install_for(spec: Agent) -> tuple[int, int]:
    """Symlink every generated skill into *spec*'s skills directory.

    Returns:
        A ``(linked, skipped)`` count pair.
    """
    directory = skills_dir(spec)
    directory.mkdir(parents=True, exist_ok=True)

    print()
    print(f"  {CYAN}Agent:{NC}          {spec.label}")
    print(f"  {CYAN}Generated dir:{NC}  {GENERATED_DIR}")
    print(f"  {CYAN}Install dir:{NC}    {directory}")
    print()

    if clean_existing(directory) > 0:
        print()

    linked = 0
    skipped = 0
    for skill_dir in sorted(GENERATED_DIR.iterdir()):
        if not skill_dir.is_dir():
            continue
        if not (skill_dir / "SKILL.md").exists():
            continue

        skill_name = skill_dir.name
        target = directory / skill_name

        if target.is_symlink():
            existing = target.resolve()
            if existing == skill_dir.resolve():
                print(f"  {CYAN}○{NC} {skill_name}/ (already linked)")
                skipped += 1
                continue
            target.unlink()

        if target.exists():
            print(
                f"  {YELLOW}!{NC} {skill_name}/ — directory exists and is not a symlink, skipping"
            )
            skipped += 1
            continue

        target.symlink_to(skill_dir.resolve())
        print(f"  {GREEN}✓{NC} {skill_name}/ → {skill_dir.resolve()}")
        linked += 1

    return linked, skipped


def install(agents: Sequence[str] | None = None) -> None:
    """Install generated skills for the selected coding agents.

    Args:
        agents: Agent keys to install for. When omitted, Ashley reuses the
            agents that already have skills, or asks on a fresh machine.
    """
    if not GENERATED_DIR.is_dir():
        print(f"{RED}No generated/ directory found. Run 'ash generate' first.{NC}")
        sys.exit(1)

    selected = resolve_agents(agents)
    if not selected:
        print(f"{RED}No known coding agent selected. Nothing to do.{NC}")
        sys.exit(1)

    print()
    print(f"{BOLD}==========================================={NC}")
    print(f"{BOLD}       Ashley Skills Install{NC}")
    print(f"{BOLD}==========================================={NC}")

    total_linked = 0
    total_skipped = 0
    for key in selected:
        spec = get_agent(key)
        ensure_agent(spec)
        linked, skipped = _install_for(spec)
        total_linked += linked
        total_skipped += skipped

    # The first selected agent becomes the default for `ash run`.
    save_agent(selected[0])

    print()
    print(f"{BOLD}-------------------------------------------{NC}")
    print(f"  {GREEN}Linked:{NC}  {total_linked}")
    if total_skipped > 0:
        print(f"  {CYAN}Skipped:{NC} {total_skipped}")
    print(f"  {CYAN}Default:{NC} {get_agent(selected[0]).label}")
    print(f"{BOLD}-------------------------------------------{NC}")
    print()


def uninstall(agents: Sequence[str] | None = None) -> None:
    """Remove ashley symlinks from every agent's skills directory."""
    selected = [a for a in (agents or AGENT_KEYS) if is_agent(a)]

    print()
    removed = 0
    for key in selected:
        directory = skills_dir(key)
        if not directory.is_dir():
            continue
        print(f"{BOLD}Removing ashley skills from {directory}{NC}")
        removed += clean_existing(directory)
        print()

    if removed == 0:
        print(f"  {CYAN}No ashley symlinks found.{NC}")
    else:
        print(f"  {GREEN}Removed:{NC} {removed}")
    print()


def parse_argv(argv: Sequence[str]) -> tuple[str, list[str] | None]:
    """Parse ``python -m ashley.install`` arguments.

    Recognises ``uninstall``, ``--claude``, ``--codex``, ``--both`` and
    ``--agent <key>`` (also ``--agent=<key>``).

    Returns:
        A ``(command, agent_keys)`` pair, where ``agent_keys`` is ``None``
        when the caller made no explicit choice.
    """
    command = "install"
    keys: list[str] = []

    def add(value: str) -> None:
        """Record an agent key, expanding the 'both'/'all' aliases."""
        if value.strip().lower() in ("both", "all"):
            keys.extend(AGENT_KEYS)
        else:
            keys.append(value)

    args = list(argv)
    while args:
        arg = args.pop(0)
        if arg == "uninstall":
            command = "uninstall"
        elif arg.startswith("--agent="):
            add(arg.split("=", 1)[1])
        elif arg == "--agent" and args:
            add(args.pop(0))
        elif arg.startswith("--") and arg[2:] in (*AGENTS, "both", "all"):
            add(arg[2:])

    # Preserve order while dropping duplicates.
    unique = list(dict.fromkeys(k.strip().lower() for k in keys))
    return command, unique or None


def main(argv: Sequence[str] | None = None) -> None:
    """Entry point for ``python -m ashley.install``."""
    command, keys = parse_argv(sys.argv[1:] if argv is None else argv)
    if command == "uninstall":
        uninstall(keys)
    else:
        install(keys)


if __name__ == "__main__":
    main()
