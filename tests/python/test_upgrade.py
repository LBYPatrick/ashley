"""Tests for coding-agent CLI detection and upgrade routing."""

from dataclasses import replace
from pathlib import Path
from unittest.mock import patch

from ashley import PROJECT_ROOT
from ashley import upgrade as upgrade_mod
from ashley.agents import CLAUDE, CODEX
from ashley.upgrade import (
    BREW,
    MISSING,
    NATIVE,
    AgentStatus,
    brew_package_from_path,
    detect,
    upgrade_plan,
)

BREW_PREFIX = Path("/opt/homebrew")


# ── Homebrew package identification (pure) ──


def test_brew_package_from_cellar_path():
    resolved = BREW_PREFIX / "Cellar/codex/0.145.0/bin/codex"
    assert brew_package_from_path(resolved, BREW_PREFIX) == ("codex", False)


def test_brew_package_from_caskroom_path():
    resolved = BREW_PREFIX / "Caskroom/claude-code/2.1.220/claude"
    assert brew_package_from_path(resolved, BREW_PREFIX) == ("claude-code", True)


def test_brew_package_does_not_guess_from_binary_name():
    """The prefix alone does not establish Homebrew ownership."""
    resolved = BREW_PREFIX / "bin/codex"
    assert brew_package_from_path(resolved, BREW_PREFIX) is None


def test_brew_package_rejects_paths_outside_the_prefix():
    resolved = Path("/Users/someone/.local/bin/claude")
    assert brew_package_from_path(resolved, BREW_PREFIX) is None


def test_brew_package_handles_a_bare_store_directory():
    """A truncated Cellar path names no package."""
    assert brew_package_from_path(Path("/opt/homebrew/Cellar"), BREW_PREFIX) is None


# ── Detection ──


def _detected(which: str | None, prefix: Path | None, version: str = "1.2.3"):
    """Run detect() against a synthetic environment."""
    return (
        patch.object(upgrade_mod.shutil, "which", lambda _binary: which),
        patch.object(upgrade_mod, "brew_prefix", lambda: prefix),
        patch.object(upgrade_mod, "agent_version", lambda _path: version),
    )


def test_detect_reports_a_missing_agent():
    with_which, with_prefix, with_version = _detected(None, BREW_PREFIX)
    with with_which, with_prefix, with_version:
        status = detect(CODEX)
    assert status.source == MISSING
    assert not status.installed
    assert status.path is None
    assert status.version is None
    assert status.source_label == "not installed"


def test_detect_reports_a_native_install():
    home_bin = "/Users/someone/.local/bin/codex"
    with_which, with_prefix, with_version = _detected(home_bin, BREW_PREFIX)
    with with_which, with_prefix, with_version:
        status = detect(CODEX)
    assert status.source == NATIVE
    assert status.installed
    assert status.path == Path(home_bin)
    assert status.version == "1.2.3"
    assert status.brew_package is None


def test_detect_reports_a_brew_install(tmp_path):
    """A binary linked out of the Cellar is attributed to its formula."""
    cellar_bin = tmp_path / "Cellar/codex/0.145.0/bin"
    cellar_bin.mkdir(parents=True)
    real = cellar_bin / "codex"
    real.touch()
    link = tmp_path / "bin/codex"
    link.parent.mkdir(parents=True)
    link.symlink_to(real)

    with_which, with_prefix, with_version = _detected(str(link), tmp_path)
    with with_which, with_prefix, with_version:
        status = detect(CODEX)

    assert status.source == BREW
    assert status.brew_package == "codex"
    assert status.brew_cask is False
    assert status.source_label == "Homebrew (formula codex)"


def test_detect_treats_a_missing_brew_as_a_native_install():
    """Without Homebrew there is nothing to attribute the binary to."""
    with_which, with_prefix, with_version = _detected("/usr/local/bin/claude", None)
    with with_which, with_prefix, with_version:
        status = detect(CLAUDE)
    assert status.source == NATIVE


# ── Upgrade routing ──


def _status(source: str, package: str | None = None, cask: bool = False):
    return AgentStatus(
        agent=CLAUDE,
        source=source,
        path=Path("/somewhere/claude"),
        version="1.0.0",
        brew_package=package,
        brew_cask=cask,
    )


def _native(spec):
    return ["bash", str(PROJECT_ROOT / "scripts" / spec.install_script), "--force"]


def test_brew_formula_upgrades_with_brew():
    assert upgrade_plan(_status(BREW, "codex")) == [["brew", "upgrade", "codex"]]


def test_brew_cask_upgrades_with_the_cask_flag():
    assert upgrade_plan(_status(BREW, "claude-code", cask=True)) == [
        ["brew", "upgrade", "--cask", "claude-code"]
    ]


def test_brew_needs_no_fallback():
    """brew already skips the work when the package is current."""
    assert len(upgrade_plan(_status(BREW, "codex"))) == 1


def test_installed_agent_asks_the_cli_to_update_itself_first():
    """The self-update checks for a new version before downloading."""
    plan = upgrade_plan(_status(NATIVE))
    assert plan[0] == ["/somewhere/claude", "update"]


def test_installed_agent_falls_back_to_the_native_installer():
    """Covers an npm install whose own updater refuses."""
    plan = upgrade_plan(_status(NATIVE))
    assert plan[1] == _native(CLAUDE)
    assert len(plan) == 2


def test_missing_agent_installs_through_the_native_installer():
    """There is no binary to self-update, so the installer is the only path."""
    plan = upgrade_plan(
        AgentStatus(agent=CODEX, source=MISSING, path=None, version=None)
    )
    assert plan == [_native(CODEX)]


def test_agent_without_a_self_update_uses_the_installer_only():
    spec = replace(CODEX, self_update_args=())
    status = AgentStatus(
        agent=spec, source=NATIVE, path=Path("/somewhere/codex"), version="1.0.0"
    )
    assert upgrade_plan(status) == [_native(spec)]


def test_no_upgrade_path_ever_uses_a_node_package_manager():
    """Node package managers are deliberately never invoked."""
    plans = [
        upgrade_plan(_status(BREW, "claude-code", cask=True)),
        upgrade_plan(_status(BREW, "codex")),
        upgrade_plan(_status(NATIVE)),
        upgrade_plan(_status(MISSING)),
    ]
    flattened = " ".join(part for plan in plans for command in plan for part in command)
    for manager in ("npm", "pnpm", "yarn"):
        assert manager not in flattened


def test_native_install_script_exists_for_every_agent():
    for spec in (CLAUDE, CODEX):
        assert (PROJECT_ROOT / "scripts" / spec.install_script).is_file()


def test_npm_codex_under_brew_prefix_is_not_a_formula(tmp_path):
    script = tmp_path / "lib/node_modules/@openai/codex/bin/codex.js"
    script.parent.mkdir(parents=True)
    script.touch()
    link = tmp_path / "bin/codex"
    link.parent.mkdir()
    link.symlink_to(script)
    with_which, with_prefix, with_version = _detected(str(link), tmp_path)
    with with_which, with_prefix, with_version:
        status = detect(CODEX)
    assert status.source == NATIVE
    assert status.brew_package is None
    assert all(command[0] != "brew" for command in upgrade_plan(status))


def test_unrelated_cellar_directory_is_not_homebrew():
    assert (
        brew_package_from_path(Path("/tmp/Cellar/codex/1/bin/codex"), BREW_PREFIX)
        is None
    )
