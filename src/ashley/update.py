"""Ashley Self-Updater.

Pulls the latest changes from the remote, re-generates skills,
re-installs them if changes are detected, and brings the coding-agent
CLIs up to date. Set ``SKIP_TOOL=true`` (or ``1``/``yes``) to leave the
agent CLIs alone.
"""

import os
import subprocess
import sys
from collections.abc import Mapping

from ashley import GENERATED_DIR, PROJECT_ROOT
from ashley.agents import AGENT_KEYS, get_agent
from ashley.config import load_agent
from ashley.install import has_skills
from ashley.upgrade import upgrade_agents

GREEN = "\033[0;32m"
CYAN = "\033[0;36m"
RED = "\033[0;31m"
YELLOW = "\033[0;33m"
BOLD = "\033[1m"
NC = "\033[0m"

# Opt-out for the agent-CLI upgrade step.
SKIP_TOOL_ENV = "SKIP_TOOL"
TRUTHY = ("true", "1", "yes")


def skip_tool_upgrade(env: Mapping[str, str] | None = None) -> bool:
    """Return True when ``SKIP_TOOL`` asks to leave the agent CLIs alone.

    Args:
        env: Environment mapping to read; defaults to the process
            environment.

    Returns:
        True when the variable is set to ``true``, ``1`` or ``yes``
        (case-insensitive).
    """
    value = (os.environ if env is None else env).get(SKIP_TOOL_ENV, "")
    return value.strip().lower() in TRUTHY


def tracked_agents() -> list[str]:
    """Return the agents worth upgrading alongside Ashley itself.

    Prefers whichever agents already have Ashley skills linked, so a
    Codex-only machine never installs Claude Code as a side effect of
    ``ash update``. Falls back to the saved default.
    """
    linked = [key for key in AGENT_KEYS if has_skills(key)]
    return linked or [get_agent(load_agent()).key]


def get_generated_hash() -> str:
    """Get a content hash of the generated/ directory for change detection."""
    if not GENERATED_DIR.is_dir():
        return ""
    result = subprocess.run(
        ["find", str(GENERATED_DIR), "-type", "f", "-exec", "md5", "-q", "{}", ";"],
        capture_output=True,
        text=True,
        cwd=PROJECT_ROOT,
    )
    if result.returncode != 0:
        result = subprocess.run(
            [
                "find",
                str(GENERATED_DIR),
                "-type",
                "f",
                "-exec",
                "md5sum",
                "{}",
                ";",
            ],
            capture_output=True,
            text=True,
            cwd=PROJECT_ROOT,
        )
    return result.stdout.strip()


def update(branch: str = "main", *, upgrade_tools: bool = True) -> None:
    """Pull latest, regenerate skills, reinstall if changed, upgrade the CLIs.

    Args:
        branch: Branch to pull from.
        upgrade_tools: Whether to also upgrade the coding-agent CLIs.
    """
    print()
    print(f"{BOLD}==========================================={NC}")
    print(f"{BOLD}          Ashley Self-Updater{NC}")
    print(f"{BOLD}==========================================={NC}")
    print()
    print(f"  {CYAN}Ashley root:{NC}  {PROJECT_ROOT}")
    print(f"  {CYAN}Branch:{NC}       {branch}")
    print()

    # Pull the latest source.
    print(f"{BOLD}[1/4] Pulling latest from {branch}...{NC}")
    result = subprocess.run(
        ["git", "pull", "origin", branch],
        cwd=PROJECT_ROOT,
    )
    if result.returncode != 0:
        print(f"{RED}Git pull failed. Aborting update.{NC}")
        sys.exit(1)
    print()

    # Snapshot the generated skills so we only reinstall on a real change.
    old_hash = get_generated_hash()

    # Sync dependencies.
    print(f"{BOLD}[2/4] Syncing dependencies...{NC}")
    result = subprocess.run(["uv", "sync"], cwd=PROJECT_ROOT)
    if result.returncode != 0:
        print(f"{YELLOW}Dependency sync failed — continuing anyway.{NC}")
    print()

    # Re-generate skills.
    print(f"{BOLD}[3/4] Re-generating skills...{NC}")
    result = subprocess.run(
        [sys.executable, "-m", "ashley.generate"],
        cwd=PROJECT_ROOT,
    )
    if result.returncode != 0:
        print(f"{RED}Generation failed. Aborting update.{NC}")
        sys.exit(1)

    # Check if anything changed and re-install if so
    if old_hash == get_generated_hash():
        print(f"{CYAN}No changes detected in generated skills. Skipping reinstall.{NC}")
    else:
        print(f"{BOLD}Changes detected — re-installing skills...{NC}")
        result = subprocess.run(
            [sys.executable, "-m", "ashley.install"],
            cwd=PROJECT_ROOT,
        )
        if result.returncode != 0:
            print(f"{RED}Installation failed.{NC}")
            sys.exit(1)
    print()

    # Bring the coding-agent CLIs up to date. A failure here does
    # not fail the update — Ashley itself is already current.
    print(f"{BOLD}[4/4] Upgrading coding-agent CLIs...{NC}")
    if not upgrade_tools:
        print(f"{CYAN}Skipped ({SKIP_TOOL_ENV} is set).{NC}")
    elif upgrade_agents(tracked_agents()):
        print(f"{YELLOW}Some agent CLIs could not be upgraded.{NC}")

    print(f"{GREEN}Update complete.{NC}")
    print()


if __name__ == "__main__":
    branch = sys.argv[1] if len(sys.argv) > 1 else "main"
    update(branch, upgrade_tools=not skip_tool_upgrade())
