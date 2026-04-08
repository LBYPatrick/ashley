"""Ashley Self-Updater.

Pulls the latest changes from the remote, re-generates skills,
and re-installs them if changes are detected.
"""

import subprocess
import sys

from ashley import GENERATED_DIR, PROJECT_ROOT

GREEN = "\033[0;32m"
CYAN = "\033[0;36m"
RED = "\033[0;31m"
YELLOW = "\033[0;33m"
BOLD = "\033[1m"
NC = "\033[0m"


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


def update(branch: str = "main") -> None:
    """Pull latest, regenerate skills, and reinstall if changed."""
    print()
    print(f"{BOLD}==========================================={NC}")
    print(f"{BOLD}          Ashley Self-Updater{NC}")
    print(f"{BOLD}==========================================={NC}")
    print()
    print(f"  {CYAN}Ashley root:{NC}  {PROJECT_ROOT}")
    print(f"  {CYAN}Branch:{NC}       {branch}")
    print()

    # Step 1: Git pull
    print(f"{BOLD}[1/3] Pulling latest from {branch}...{NC}")
    result = subprocess.run(
        ["git", "pull", "origin", branch],
        cwd=PROJECT_ROOT,
    )
    if result.returncode != 0:
        print(f"{RED}Git pull failed. Aborting update.{NC}")
        sys.exit(1)
    print()

    # Step 2: Snapshot current generated content
    old_hash = get_generated_hash()

    # Step 3: Sync dependencies
    print(f"{BOLD}[2/3] Syncing dependencies...{NC}")
    result = subprocess.run(["uv", "sync"], cwd=PROJECT_ROOT)
    if result.returncode != 0:
        print(f"{YELLOW}Dependency sync failed — continuing anyway.{NC}")
    print()

    # Step 4: Re-generate skills
    print(f"{BOLD}[3/3] Re-generating skills...{NC}")
    result = subprocess.run(
        [sys.executable, "-m", "ashley.generate"],
        cwd=PROJECT_ROOT,
    )
    if result.returncode != 0:
        print(f"{RED}Generation failed. Aborting update.{NC}")
        sys.exit(1)

    # Check if anything changed and re-install if so
    new_hash = get_generated_hash()
    if old_hash == new_hash:
        print(f"{CYAN}No changes detected in generated skills. Skipping reinstall.{NC}")
        print()
        return

    print(f"{BOLD}Changes detected — re-installing skills...{NC}")
    result = subprocess.run(
        [sys.executable, "-m", "ashley.install"],
        cwd=PROJECT_ROOT,
    )
    if result.returncode != 0:
        print(f"{RED}Installation failed.{NC}")
        sys.exit(1)

    print(f"{GREEN}Update complete.{NC}")
    print()


if __name__ == "__main__":
    branch = sys.argv[1] if len(sys.argv) > 1 else "main"
    update(branch)
