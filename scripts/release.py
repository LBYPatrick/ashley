"""Developer-only release validation and version synchronization."""

import argparse
import json
import re
import sys
import tomllib
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
VERSION_RE = re.compile(
    r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-(alpha|beta|rc)\.(0|[1-9]\d*))?"
)
BADGE_RE = re.compile(r"(img\.shields\.io/badge/version-)[^\" ]+(-blue)")


def validate_version(version: str) -> bool:
    """Validate the supported SemVer shape and return whether it is prerelease."""
    match = VERSION_RE.fullmatch(version)
    if not match:
        raise ValueError("Use X.Y.Z or X.Y.Z-alpha.N / beta.N / rc.N")
    return match.group(4) is not None


def python_version(version: str) -> str:
    """Map release SemVer to equivalent PEP 440 metadata while Python remains."""
    validate_version(version)
    return re.sub(
        r"-(alpha|beta|rc)\.(\d+)$",
        lambda m: {"alpha": "a", "beta": "b", "rc": "rc"}[m[1]] + m[2],
        version,
    )


def check(root: Path, tag: str) -> str:
    """Check the release identity and require full feature parity for stable tags."""
    if not tag.startswith("v"):
        raise ValueError("Release tags must start with v")
    version = tag[1:]
    prerelease = validate_version(version)
    if (root / "VERSION").read_text().strip() != version:
        raise ValueError("Tag and VERSION disagree")
    project = tomllib.loads((root / "pyproject.toml").read_text())
    if project["project"]["version"] != python_version(version):
        raise ValueError("Tag and pyproject.toml disagree")
    badge = BADGE_RE.search((root / "README.md").read_text())
    if (
        badge is None
        or badge.group(0)
        != f"img.shields.io/badge/version-{version.replace('-', '--')}-blue"
    ):
        raise ValueError("Tag and README version badge disagree")
    status = json.loads((root / "docs/go-migration-status.json").read_text())
    if (
        not isinstance(status, dict)
        or not status
        or any(type(value) is not bool for value in status.values())
    ):
        raise ValueError("Migration status must be a nonempty map of feature booleans")
    missing = [key for key, complete in status.items() if not complete]
    if missing and not prerelease:
        raise ValueError(
            "Stable binary release requires full feature parity. Pending: "
            + ", ".join(missing)
        )
    return version


def prepare(root: Path, version: str) -> None:
    """Update the three version surfaces after validating every edit."""
    validate_version(version)
    readme = (root / "README.md").read_text()
    readme, count = BADGE_RE.subn(
        lambda m: m[1] + version.replace("-", "--") + m[2], readme
    )
    if count != 1:
        raise ValueError("Expected exactly one README version badge")
    manifest = (root / "pyproject.toml").read_text()
    manifest, count = re.subn(
        r'(?m)^version = "[^"]+"$', f'version = "{python_version(version)}"', manifest
    )
    if count != 1:
        raise ValueError("Expected exactly one project version field")
    (root / "VERSION").write_text(version + "\n")
    (root / "README.md").write_text(readme)
    (root / "pyproject.toml").write_text(manifest)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["check", "prepare"])
    parser.add_argument("version")
    args = parser.parse_args()
    try:
        if args.command == "prepare":
            prepare(ROOT, args.version)
        else:
            print(check(ROOT, args.version))
    except (ValueError, OSError) as exc:
        print(f"Release validation failed: {exc}", file=sys.stderr)
        raise SystemExit(1) from exc


if __name__ == "__main__":
    main()
