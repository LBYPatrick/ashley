"""Shared compiled binary and release archive fixtures."""

import hashlib
import platform
import tarfile
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
VERSION = (ROOT / "VERSION").read_text().strip()


@pytest.fixture(scope="module")
def binary():
    path = ROOT / "build/ash-go"
    assert path.is_file(), "Run make build before make test-integration"
    return path


@pytest.fixture
def release_archive(binary, tmp_path):
    target_os = "darwin" if platform.system() == "Darwin" else "linux"
    arch = "arm64" if platform.machine() in ("arm64", "aarch64") else "amd64"
    archive = tmp_path / f"ashley-{VERSION}-{target_os}-{arch}.tar.gz"
    with tarfile.open(archive, "w:gz") as package:
        package.add(binary, arcname="ash")
        package.add(ROOT / "LICENSE", arcname="LICENSE")
    checksum = archive.with_name(archive.name + ".sha256")
    checksum.write_text(
        f"{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n"
    )
    return archive
