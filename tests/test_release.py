"""Release identity and migration gates, without remote mutations."""

import importlib.util
import json
from pathlib import Path

import pytest
import yaml

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "release_tools", ROOT / "scripts/release.py"
)
release = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(release)


@pytest.fixture
def release_root(tmp_path):
    for name in (
        "VERSION",
        "README.md",
        "pyproject.toml",
        "docs/go-migration-status.json",
    ):
        path = tmp_path / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text((ROOT / name).read_text())

    # Exercise an incomplete migration independently of the repository's status.
    status_path = tmp_path / "docs/go-migration-status.json"
    status = json.loads(status_path.read_text())
    status_path.write_text(json.dumps(dict.fromkeys(status, False)))
    return tmp_path


def test_stable_requires_all_features(release_root):
    release.prepare(release_root, "1.0.0")
    with pytest.raises(ValueError, match="full feature parity"):
        release.check(release_root, "v1.0.0")
    status_path = release_root / "docs/go-migration-status.json"
    status = json.loads(status_path.read_text())
    status_path.write_text(json.dumps(dict.fromkeys(status, True)))
    assert release.check(release_root, "v1.0.0") == "1.0.0"


def test_prerelease_synchronizes_all_version_surfaces(release_root):
    release.prepare(release_root, "0.4.0-rc.1")
    assert release.check(release_root, "v0.4.0-rc.1") == "0.4.0-rc.1"
    assert 'version = "0.4.0rc1"' in (release_root / "pyproject.toml").read_text()
    assert "version-0.4.0--rc.1-blue" in (release_root / "README.md").read_text()
    release.prepare(release_root, "0.4.0-rc.1")  # idempotent
    assert release.check(release_root, "v0.4.0-rc.1") == "0.4.0-rc.1"


@pytest.mark.parametrize(
    "version",
    ["v1.0.0", "01.0.0", "1.2", "../bad", "1.0.0;echo nope", "1.0.0-unknown.1"],
)
def test_invalid_versions_do_not_modify_files(release_root, version):
    before = (release_root / "VERSION").read_text()
    with pytest.raises(ValueError):
        release.prepare(release_root, version)
    assert (release_root / "VERSION").read_text() == before


def test_mismatched_tag_is_rejected(release_root):
    release.prepare(release_root, "0.4.0-beta.1")
    with pytest.raises(ValueError, match="disagree"):
        release.check(release_root, "v0.4.0-beta.2")


def test_unknown_badge_does_not_partially_update(release_root):
    (release_root / "README.md").write_text("No badge")
    before = (release_root / "VERSION").read_text()
    with pytest.raises(ValueError, match="badge"):
        release.prepare(release_root, "1.0.0")
    assert (release_root / "VERSION").read_text() == before


def test_workflows_require_all_platforms_and_gate_before_publish():
    workflow = yaml.safe_load((ROOT / ".github/workflows/release.yaml").read_text())
    jobs = workflow["jobs"]
    assert jobs["package"]["needs"] == ["version", "gate"]
    assert jobs["publish"]["needs"] == ["version", "package"]
    assert jobs["package"]["strategy"]["matrix"] == {
        "platform": ["darwin", "linux"],
        "arch": ["arm64", "amd64"],
    }
    assert jobs["publish"]["permissions"] == {"contents": "write"}
