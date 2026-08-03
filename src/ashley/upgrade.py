"""Detection and upgrading of the coding-agent CLIs Ashley drives.

Ashley does not own Claude Code or Codex — the user may have installed
either one with Homebrew, with the vendor's native installer, or with a
Node package manager. This module works out where an agent's binary came
from and upgrades it the same way it was installed, with one deliberate
exception: Node package managers are never invoked, so anything that is
not a Homebrew install is (re)installed with the vendor's native
installer. A missing agent takes the same native path.

Detection is pure path arithmetic (:func:`brew_package_from_path`) over
the resolved binary location; the subprocess calls that discover that
location and act on it live at the edges of the module.
"""

import shutil
import subprocess
from dataclasses import dataclass
from functools import cache
from pathlib import Path

from ashley import PROJECT_ROOT
from ashley.agents import Agent, get_agent

GREEN = "\033[0;32m"
CYAN = "\033[0;36m"
RED = "\033[0;31m"
YELLOW = "\033[0;33m"
BOLD = "\033[1m"
NC = "\033[0m"

# How an agent binary got onto the machine.
BREW = "brew"
NATIVE = "native"
MISSING = "missing"

# Homebrew keeps every package under one of these directories and links
# the binaries it exposes into ``<prefix>/bin``.
BREW_STORES: tuple[tuple[str, bool], ...] = (("Cellar", False), ("Caskroom", True))


@dataclass(frozen=True)
class AgentStatus:
    """Where an agent's CLI came from, and which version is installed.

    Attributes:
        agent: The backend this status describes.
        source: One of :data:`BREW`, :data:`NATIVE` or :data:`MISSING`.
        path: Location of the binary on ``PATH``, or ``None`` when absent.
        version: First line of ``<binary> --version``, when it answered.
        brew_package: Homebrew package name for :data:`BREW` installs.
        brew_cask: Whether that package is a cask rather than a formula.
    """

    agent: Agent
    source: str
    path: Path | None
    version: str | None
    brew_package: str | None = None
    brew_cask: bool = False

    @property
    def installed(self) -> bool:
        """Return True when the agent's binary is on ``PATH``."""
        return self.source != MISSING

    @property
    def source_label(self) -> str:
        """Human-readable description of the install source."""
        if self.source == BREW:
            kind = "cask" if self.brew_cask else "formula"
            return f"Homebrew ({kind} {self.brew_package})"
        if self.source == NATIVE:
            return "native installer"
        return "not installed"


def brew_package_from_path(resolved: Path, prefix: Path) -> tuple[str, bool] | None:
    """Identify the Homebrew package a resolved binary path belongs to.

    Homebrew links the binaries it exposes into ``<prefix>/bin`` from
    ``<prefix>/Cellar/<formula>/<version>/…`` for formulae and
    ``<prefix>/Caskroom/<cask>/<version>/…`` for casks, so the package
    name is recoverable from the fully resolved path.

    Args:
        resolved: Fully resolved (symlinks followed) path of the binary.
        prefix: Homebrew installation prefix, e.g. ``/opt/homebrew``.

    Returns:
        A ``(package, is_cask)`` pair, or ``None`` when the path does not
        come from Homebrew.
    """
    parts = resolved.parts
    for store, is_cask in BREW_STORES:
        if store in parts:
            index = parts.index(store)
            if index + 1 < len(parts):
                return parts[index + 1], is_cask

    # A few packages drop real files straight into <prefix>/bin, where the
    # binary name is also the package name.
    if resolved.is_relative_to(prefix):
        return resolved.name, False
    return None


@cache
def brew_prefix() -> Path | None:
    """Return the Homebrew prefix, or ``None`` when brew is unavailable."""
    if not shutil.which("brew"):
        return None
    result = subprocess.run(
        ["brew", "--prefix"], capture_output=True, text=True, check=False
    )
    prefix = result.stdout.strip()
    if result.returncode != 0 or not prefix:
        return None
    return Path(prefix)


def agent_version(binary: Path) -> str | None:
    """Return the first line of ``<binary> --version``, or ``None``.

    Version output is only ever displayed, so it is kept verbatim rather
    than parsed — the two agents format it differently.
    """
    try:
        result = subprocess.run(
            [str(binary), "--version"],
            capture_output=True,
            text=True,
            timeout=30,
            check=False,
        )
    except (OSError, subprocess.SubprocessError):
        return None
    if result.returncode != 0:
        return None
    lines = result.stdout.strip().splitlines()
    return lines[0].strip() if lines else None


def detect(agent: Agent | str | None) -> AgentStatus:
    """Work out whether *agent* is installed, from where, and at which version."""
    spec = get_agent(agent)
    found = shutil.which(spec.binary)
    if not found:
        return AgentStatus(agent=spec, source=MISSING, path=None, version=None)

    path = Path(found)
    version = agent_version(path)
    prefix = brew_prefix()
    package = brew_package_from_path(path.resolve(), prefix) if prefix else None
    if package:
        return AgentStatus(
            agent=spec,
            source=BREW,
            path=path,
            version=version,
            brew_package=package[0],
            brew_cask=package[1],
        )
    return AgentStatus(agent=spec, source=NATIVE, path=path, version=version)


def upgrade_command(status: AgentStatus) -> list[str]:
    """Build the command that installs or upgrades the agent in place.

    Homebrew installs are upgraded with ``brew``. Everything else — a
    native install, an install from a Node package manager, or nothing at
    all — goes through the vendor's native installer, which upgrades in
    place and never involves npm or pnpm.
    """
    if status.source == BREW and status.brew_package:
        cask = ["--cask"] if status.brew_cask else []
        return ["brew", "upgrade", *cask, status.brew_package]
    script = PROJECT_ROOT / "scripts" / status.agent.install_script
    return ["bash", str(script), "--force"]


def describe(status: AgentStatus) -> str:
    """Format a one-line summary of *status* for the terminal."""
    label = f"  {CYAN}{status.agent.label:<14}{NC}"
    if not status.installed:
        return f"{label} {YELLOW}not installed{NC}"
    version = status.version or "unknown version"
    return f"{label} {BOLD}{version}{NC}  {status.source_label}  {status.path}"


def upgrade(agent: Agent | str | None) -> bool:
    """Install or upgrade one agent CLI, reporting what changed.

    Args:
        agent: Backend to upgrade.

    Returns:
        True when the upgrade command succeeded.
    """
    spec = get_agent(agent)
    before = detect(spec)

    print()
    print(describe(before))
    command = upgrade_command(before)
    action = "Installing" if not before.installed else "Upgrading"
    print(f"  {CYAN}▶{NC} {action} via: {' '.join(command)}")
    print()

    result = subprocess.run(command, check=False)
    if result.returncode != 0:
        print(f"  {RED}✗{NC} {spec.label} upgrade failed (exit {result.returncode}).")
        if before.source != BREW:
            print(f"    Install it manually: {spec.docs_url}")
        return False

    after = detect(spec)
    if not after.installed:
        print(f"  {RED}✗{NC} {spec.label} is still not on PATH.")
        print(f"    Install it manually: {spec.docs_url}")
        return False

    if before.version and after.version == before.version:
        print(f"  {GREEN}✓{NC} {spec.label} already up to date ({after.version}).")
    else:
        was = f" (was {before.version})" if before.version else ""
        print(f"  {GREEN}✓{NC} {spec.label} now at {after.version}{was}.")
    return True


def upgrade_agents(agents: list[str]) -> int:
    """Upgrade several agents in turn.

    Args:
        agents: Agent keys, in the order they should be processed.

    Returns:
        The number of agents that failed to upgrade.
    """
    failures = 0
    for key in agents:
        if not upgrade(key):
            failures += 1
    print()
    return failures
