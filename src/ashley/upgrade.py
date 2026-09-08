"""Detection and upgrading of the coding-agent CLIs Ashley drives.

Ashley identifies Homebrew installations from their Cellar/Caskroom paths.
Other installations use the CLI updater with a bootstrap-script fallback.
Kilo's bootstrap uses npm; the other backends use native installers.

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
    try:
        parts = resolved.relative_to(prefix).parts
    except ValueError:
        return None
    for store, is_cask in BREW_STORES:
        if len(parts) >= 4 and parts[0] == store:
            return parts[1], is_cask
    # npm globals may live under Homebrew's prefix too. Their script
    # basenames (e.g. codex.js) are not Homebrew formula names.
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


def native_install_command(spec: Agent) -> list[str]:
    """Build the vendor's native install command for *spec*.

    This installs the latest version unconditionally, so it is only ever
    the last resort: it is how a missing agent is bootstrapped and how an
    agent whose own updater refuses (an npm install, say) is migrated
    onto the native distribution.
    """
    script = PROJECT_ROOT / "scripts" / spec.install_script
    return ["bash", str(script), "--force"]


def upgrade_plan(status: AgentStatus) -> list[list[str]]:
    """Build the commands to try, in order, until one succeeds.

    Homebrew installs are upgraded with ``brew``, which already skips the
    work when the package is current. An installed agent is otherwise
    asked to update itself using its configured update subcommand, which
    checks for a new version before downloading anything, so re-running
    ``ash upgrade`` on an up-to-date agent costs one version check rather
    than a full reinstall. The native installer is the fallback, and the
    only option for an agent that is not installed at all.

    Package installation details live in each backend bootstrap script.
    """
    if status.source == BREW and status.brew_package:
        cask = ["--cask"] if status.brew_cask else []
        return [["brew", "upgrade", *cask, status.brew_package]]

    native = native_install_command(status.agent)
    if status.installed and status.agent.self_update_args and status.path:
        return [[str(status.path), *status.agent.self_update_args], native]
    return [native]


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
    action = "Upgrading" if before.installed else "Installing"

    for attempt, command in enumerate(upgrade_plan(before)):
        if attempt:
            print()
            print(f"  {YELLOW}!{NC} Falling back to the native installer.")
        print(f"  {CYAN}▶{NC} {action} via: {' '.join(command)}")
        print()
        if subprocess.run(command, check=False).returncode == 0:
            break
    else:
        print(f"  {RED}✗{NC} {spec.label} upgrade failed.")
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
