"""Tests for Ashley project detector."""

import json
import tempfile
from pathlib import Path

from ashley.detect import detect_project


def test_detect_empty_directory():
    with tempfile.TemporaryDirectory() as tmpdir:
        ctx = detect_project(tmpdir)
        assert ctx["python"] is False
        assert ctx["typescript"] is False
        assert ctx["name"] == Path(tmpdir).name


def test_detect_python_project():
    with tempfile.TemporaryDirectory() as tmpdir:
        (Path(tmpdir) / "pyproject.toml").write_text("[project]\nname = 'myapp'\n")
        (Path(tmpdir) / "uv.lock").write_text("")
        ctx = detect_project(tmpdir)
        assert ctx["python"] is True
        assert ctx["uv"] is True


def test_detect_typescript_react():
    with tempfile.TemporaryDirectory() as tmpdir:
        pkg = {
            "name": "my-react-app",
            "dependencies": {"react": "^19", "react-dom": "^19"},
            "devDependencies": {"typescript": "^5"},
        }
        (Path(tmpdir) / "package.json").write_text(json.dumps(pkg))
        (Path(tmpdir) / "tsconfig.json").write_text("{}")
        ctx = detect_project(tmpdir)
        assert ctx["javascript"] is True
        assert ctx["typescript"] is True
        assert ctx["react"] is True
        assert ctx["name"] == "my-react-app"


def test_detect_go():
    with tempfile.TemporaryDirectory() as tmpdir:
        (Path(tmpdir) / "go.mod").write_text("module example.com/foo\n")
        ctx = detect_project(tmpdir)
        assert ctx["go"] is True


def test_detect_docker_and_ci():
    with tempfile.TemporaryDirectory() as tmpdir:
        (Path(tmpdir) / "Dockerfile").write_text("FROM alpine\n")
        workflows = Path(tmpdir) / ".github" / "workflows"
        workflows.mkdir(parents=True)
        (workflows / "ci.yml").write_text("name: CI\n")
        (Path(tmpdir) / ".git").mkdir()
        ctx = detect_project(tmpdir)
        assert ctx["docker"] is True
        assert ctx["ci_github"] is True
        assert ctx["git"] is True


def test_detect_tests_and_readme():
    with tempfile.TemporaryDirectory() as tmpdir:
        (Path(tmpdir) / "tests").mkdir()
        (Path(tmpdir) / "README.md").write_text("# Hello\n")
        ctx = detect_project(tmpdir)
        assert ctx["has_tests"] is True
        assert ctx["has_readme"] is True


def test_detect_fastapi():
    with tempfile.TemporaryDirectory() as tmpdir:
        (Path(tmpdir) / "pyproject.toml").write_text(
            '[project]\ndependencies = ["fastapi"]\n'
        )
        ctx = detect_project(tmpdir)
        assert ctx["python"] is True
        assert ctx["fastapi"] is True
