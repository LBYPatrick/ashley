"""Ashley Skills Installer.

Symlinks generated skills into ~/.claude/skills/ so they are available
across all projects.
"""

import shutil
import subprocess
import sys
from pathlib import Path

from ashley import GENERATED_DIR, PROJECT_ROOT

GREEN = "\033[0;32m"
CYAN = "\033[0;36m"
RED = "\033[0;31m"
YELLOW = "\033[0;33m"
BOLD = "\033[1m"
NC = "\033[0m"

PERSONAL_SKILLS_DIR = Path.home() / ".claude" / "skills"


def clean_existing() -> int:
    """Remove any existing ashley symlinks from ~/.claude/skills/.

    Detects ashley symlinks by checking if the target path contains
    'ashley/generated/', so it works across repo relocations.
    """
    if not PERSONAL_SKILLS_DIR.is_dir():
        return 0

    removed = 0
    for link in sorted(PERSONAL_SKILLS_DIR.iterdir()):
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


def ensure_claude() -> None:
    """Install Claude Code via the native installer if not present."""
    install_script = PROJECT_ROOT / "scripts" / "install_claude.sh"
    if install_script.is_file():
        subprocess.run(["bash", str(install_script)], check=False)
    elif not shutil.which("claude"):
        print(
            f"{YELLOW}Claude Code not found. "
            f"Install it from https://claude.ai/download{NC}"
        )


def install() -> None:
    """Symlink generated skills into ~/.claude/skills/."""
    ensure_claude()

    if not GENERATED_DIR.is_dir():
        print(f"{RED}No generated/ directory found. Run 'ash generate' first.{NC}")
        sys.exit(1)

    PERSONAL_SKILLS_DIR.mkdir(parents=True, exist_ok=True)

    print()
    print(f"{BOLD}==========================================={NC}")
    print(f"{BOLD}       Ashley Skills Install{NC}")
    print(f"{BOLD}==========================================={NC}")
    print()
    print(f"  {CYAN}Generated dir:{NC}  {GENERATED_DIR}")
    print(f"  {CYAN}Install dir:{NC}    {PERSONAL_SKILLS_DIR}")
    print()

    cleaned = clean_existing()
    if cleaned > 0:
        print()

    linked = 0
    skipped = 0
    for skill_dir in sorted(GENERATED_DIR.iterdir()):
        if not skill_dir.is_dir():
            continue
        if not (skill_dir / "SKILL.md").exists():
            continue

        skill_name = skill_dir.name
        target = PERSONAL_SKILLS_DIR / skill_name

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

    print()
    print(f"{BOLD}-------------------------------------------{NC}")
    print(f"  {GREEN}Linked:{NC}  {linked}")
    if skipped > 0:
        print(f"  {CYAN}Skipped:{NC} {skipped}")
    print(f"{BOLD}-------------------------------------------{NC}")
    print()


def uninstall() -> None:
    """Remove ashley symlinks from ~/.claude/skills/."""
    if not PERSONAL_SKILLS_DIR.is_dir():
        print(f"{YELLOW}No ~/.claude/skills/ directory found. Nothing to remove.{NC}")
        return

    print()
    print(f"{BOLD}Removing ashley skills from {PERSONAL_SKILLS_DIR}{NC}")
    print()

    removed = clean_existing()

    if removed == 0:
        print(f"  {CYAN}No ashley symlinks found.{NC}")
    else:
        print()
        print(f"  {GREEN}Removed:{NC} {removed}")
    print()


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "uninstall":
        uninstall()
    else:
        install()
