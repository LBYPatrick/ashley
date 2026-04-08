"""Ashley Project Detector.

Auto-detects the project's tech stack by scanning for configuration
files and patterns. Produces a context dict that Jinja2 templates
can use for conditional content inclusion.

Example template usage in components:

    {% if project.python %}
    Use `ruff format` for formatting.
    {% endif %}

    {% if project.react %}
    Follow React component conventions...
    {% endif %}
"""

import json
from pathlib import Path


def detect_project(cwd: str | Path | None = None) -> dict:
    """Scan the working directory and return a project context dict.

    Returns a dict with boolean flags for detected technologies
    and string values for detected metadata. All keys are safe
    for use in Jinja2 templates.
    """
    if cwd is None:
        cwd = Path.cwd()
    else:
        cwd = Path(cwd)

    ctx: dict = {
        # Language flags
        "python": False,
        "typescript": False,
        "javascript": False,
        "go": False,
        "rust": False,
        "java": False,
        "dart": False,
        "c": False,
        "cpp": False,
        "shell": False,
        # Framework flags
        "react": False,
        "nextjs": False,
        "vue": False,
        "svelte": False,
        "fastapi": False,
        "django": False,
        "flask": False,
        "express": False,
        "flutter": False,
        # Tooling flags
        "docker": False,
        "makefile": False,
        "git": False,
        "ci_github": False,
        "ci_gitlab": False,
        # Package manager
        "npm": False,
        "pnpm": False,
        "yarn": False,
        "uv": False,
        "pip": False,
        "cargo": False,
        # Metadata
        "name": cwd.name,
        "has_tests": False,
        "has_readme": False,
    }

    # Python detection
    if (cwd / "pyproject.toml").is_file():
        ctx["python"] = True
        content = (cwd / "pyproject.toml").read_text()
        if "fastapi" in content.lower():
            ctx["fastapi"] = True
        if "django" in content.lower():
            ctx["django"] = True
        if "flask" in content.lower():
            ctx["flask"] = True
    if (cwd / "setup.py").is_file() or (cwd / "setup.cfg").is_file():
        ctx["python"] = True
    if (cwd / "requirements.txt").is_file():
        ctx["python"] = True
        ctx["pip"] = True

    # uv detection
    if (cwd / "uv.lock").is_file():
        ctx["uv"] = True
        ctx["python"] = True

    # Node.js / TypeScript detection
    pkg_json = cwd / "package.json"
    if pkg_json.is_file():
        ctx["javascript"] = True
        try:
            pkg = json.loads(pkg_json.read_text())
            deps = {
                **pkg.get("dependencies", {}),
                **pkg.get("devDependencies", {}),
            }
            if "typescript" in deps:
                ctx["typescript"] = True
            if "react" in deps or "react-dom" in deps:
                ctx["react"] = True
            if "next" in deps:
                ctx["nextjs"] = True
                ctx["react"] = True
            if "vue" in deps:
                ctx["vue"] = True
            if "svelte" in deps:
                ctx["svelte"] = True
            if "express" in deps:
                ctx["express"] = True
            ctx["name"] = pkg.get("name", ctx["name"])
        except (json.JSONDecodeError, OSError):
            pass

    if (cwd / "tsconfig.json").is_file():
        ctx["typescript"] = True

    # Package manager detection
    if (cwd / "pnpm-lock.yaml").is_file():
        ctx["pnpm"] = True
    elif (cwd / "yarn.lock").is_file():
        ctx["yarn"] = True
    elif (cwd / "package-lock.json").is_file():
        ctx["npm"] = True

    # Go
    if (cwd / "go.mod").is_file():
        ctx["go"] = True

    # Rust
    if (cwd / "Cargo.toml").is_file():
        ctx["rust"] = True
        ctx["cargo"] = True

    # Java
    if (cwd / "pom.xml").is_file() or (cwd / "build.gradle").is_file():
        ctx["java"] = True

    # Dart / Flutter
    if (cwd / "pubspec.yaml").is_file():
        ctx["dart"] = True
        try:
            content = (cwd / "pubspec.yaml").read_text()
            if "flutter" in content:
                ctx["flutter"] = True
        except OSError:
            pass

    # C / C++
    if (cwd / "CMakeLists.txt").is_file():
        ctx["cpp"] = True
    if (cwd / "Makefile").is_file():
        # Could be C/C++ but Makefile is generic
        ctx["makefile"] = True

    # Docker
    if (cwd / "Dockerfile").is_file() or (cwd / "docker-compose.yml").is_file():
        ctx["docker"] = True

    # Git
    if (cwd / ".git").exists():
        ctx["git"] = True

    # CI
    if (cwd / ".github" / "workflows").is_dir():
        ctx["ci_github"] = True
    if (cwd / ".gitlab-ci.yml").is_file():
        ctx["ci_gitlab"] = True

    # Shell scripts
    sh_files = list(cwd.glob("*.sh")) + list(cwd.glob("scripts/*.sh"))
    if sh_files:
        ctx["shell"] = True

    # Tests
    for test_dir in ("tests", "test", "__tests__", "spec"):
        if (cwd / test_dir).is_dir():
            ctx["has_tests"] = True
            break

    # README
    for readme in ("README.md", "README.rst", "README.txt", "README"):
        if (cwd / readme).is_file():
            ctx["has_readme"] = True
            break

    return ctx
