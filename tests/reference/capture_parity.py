"""Capture Python behavior for Go parity tests; never needed by the Go binary."""

import argparse
import hashlib
import json
import shutil
import tempfile
from dataclasses import asdict
from pathlib import Path
from unittest.mock import patch

from ashley import PROJECT_ROOT, SKILLS_DIR
from ashley import generate as generator
from ashley.agents import AGENTS
from ashley.detect import detect_project
from ashley.prompt import generate_prompt

FIXTURE = PROJECT_ROOT / "tests" / "fixtures" / "parity" / "python.json"


def digest(text: str) -> str:
    return hashlib.sha256(text.encode()).hexdigest()


def capture() -> dict:
    """Generate expected results with the unchanged Python implementation."""
    bundled = {}
    for definition in sorted(SKILLS_DIR.glob("*.jsonc")):
        with tempfile.TemporaryDirectory() as directory:
            output_root = Path(directory)
            for folder in ("skills", "components", "res"):
                shutil.copytree(PROJECT_ROOT / folder, output_root / folder)
            with (
                patch.object(generator, "PROJECT_ROOT", output_root),
                patch.object(generator, "SKILLS_DIR", output_root / "skills"),
            ):
                relative, _ = generator.assemble_skill(
                    output_root / "skills" / definition.name
                )
            document = (output_root / relative).read_text()
            with patch("ashley.prompt.GENERATED_DIR", output_root / "generated"):
                prompt = generate_prompt(
                    definition.stem, "add login $HOME `literal`\nnext line"
                )
            bundled[definition.stem] = {
                "output": relative,
                "sha256": digest(document),
                "prompt_sha256": digest(prompt),
            }

    context = {"python": True, "react": False, "name": "demo", "languages": ["a", "b"]}
    templates = [
        "{% if project.python %}Python{% else %}Other{% endif %}\n",
        "{{ project.name | upper }}\n",
        "{{ project.missing }}\n",
        "{{ project.python }} {{ project.react }}\n",
        "{% for item in project.languages %}{{ loop.index }}:{{ item }};{% endfor %}",
        "{{ project.missing | default('fallback') }}",
        "{% set greeting = 'hello' %}{{ greeting }} {{ project.name }}",
        "{% macro greet(name) %}Hello {{ name }}{% endmacro %}{{ greet(project.name) }}",
        "{%- if project.python -%}\ntrimmed\n{%- endif -%}",
        "{{ '<tag>&' }}\n\n",
        "{{ project.languages | join(', ') }}",
        "{{ {'a': 1, 'b': 2} | length }}",
        "{% if project.react %}React{% elif project.python %}Python{% endif %}",
        "{{ project.name is string }}",
        "{% if broken %}",
        "plain text\n",
        "{% for k, v in {'b': 2, 'a': 1}.items() %}{{ k }}={{ v }};{% endfor %}",
        "{{ project.name.startswith('de') }} {{ project.name.replace('de', 'DE') }}",
        "{{ project.languages | map('upper') | join(',') }}",
        "{{ [1, 2, 3] | select('odd') | join(',') }}",
        "{{ [3, 1, 2] | sort | join(',') }}",
        "{{ project.missing is defined }} {{ project.missing is undefined }}",
        "{% set ns = namespace(total=0) %}{% for n in [1,2,3] %}{% set ns.total = ns.total + n %}{% endfor %}{{ ns.total }}",
        "{% for n in range(3) %}{{ n }}{% else %}empty{% endfor %}",
        "{% for n in [] %}unused{% else %}empty{% endfor %}",
        "{{ 7 // 2 }} {{ 7 % 2 }} {{ 'x' ~ 3 }}",
        "{{ ' abc ' | trim | capitalize }}",
        "{{ {'b':2,'a':1} | dictsort | length }}",
        "{% for key, value in {'b':2,'a':1}|dictsort %}{{ key }}={{ value }};{% endfor %}",
        "{% for k in {'b':2,'a':1}.keys() %}{{ k }};{% endfor %}",
        "{% for v in {'b':2,'a':1}.values() %}{{ v }};{% endfor %}",
        "{{ 'aaa'.replace('a','x',2) }}",
        "{{ {'a':1}.get('missing', 'fallback') }}",
        "{{ project.name.upper() }} {{ project.name.endswith('mo') }}",
        "{{ 'hello\nworld' | indent(2) }}",
        "first\r\n{{ project.name }}\r\nlast\r",
    ]
    template_cases = [
        {
            "source": source,
            "context": context,
            "expected": generator.render_template(source, context),
        }
        for source in templates
    ]

    files = {
        "skills/base.jsonc": json.dumps(
            {
                "name": "a-base",
                "description": "base",
                "output": "generated/a-base/SKILL.md",
                "components": ["components/one.md"],
                "resources": ["res/tool.py"],
            }
        ),
        "skills/middle.jsonc": json.dumps(
            {
                "name": "a-middle",
                "description": "middle",
                "output": "generated/a-middle/SKILL.md",
                "extends": "base",
                "components+": ["components/nested/**/*.md"],
            }
        ),
        "skills/custom.jsonc": """{
            // JSON5 features used in custom skill definitions
            name: 'a-custom', description: 'custom',
            output: 'generated/a-custom/SKILL.md', extends: 'middle',
            'components+': ['components/missing.md', 'absent/*.md'],
            'resources+': ['res/absent.ts'],
            preamble: 'Custom preamble', epilogue: 'Finish here',
        }""",
        "skills/replace.jsonc": json.dumps(
            {
                "name": "a-replace",
                "description": "replace",
                "output": "generated/a-replace/SKILL.md",
                "extends": "custom",
                "components": [],
                "resources": [],
                "epilogue": "ignored",
                "checklist": ["Done"],
            }
        ),
        "components/one.md": "{% if project.python %}Use Python{% endif %}\n",
        "components/nested/b.md": "Second\n",
        "components/nested/a.md": "First {{ project.name }}\n",
        "components/nested/deeper/c.md": "Third\n",
        "res/tool.py": "print('hello')  \n\n",
    }
    custom = []
    with tempfile.TemporaryDirectory() as directory:
        root = Path(directory)
        for name, content in files.items():
            dest = root / name
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_text(content)
        with (
            patch.object(generator, "PROJECT_ROOT", root),
            patch.object(generator, "SKILLS_DIR", root / "skills"),
        ):
            for name in ("custom", "replace"):
                for project in (None, context):
                    relative, _ = generator.assemble_skill(
                        root / "skills" / f"{name}.jsonc", project
                    )
                    custom.append(
                        {
                            "name": name,
                            "context": project,
                            "output": relative,
                            "expected": (root / relative).read_text(),
                        }
                    )

    projects = []
    for project_files in (
        {},
        {
            "pyproject.toml": '[project]\ndependencies=["FastAPI", "django", "flask"]',
            "uv.lock": "",
            "requirements.txt": "",
            "README.md": "",
            "tests/test.py": "",
        },
        {
            "package.json": '{"name":"web","dependencies":{"next":"*","vue":"*","express":"*"},"devDependencies":{"typescript":"*","svelte":"*"}}',
            "pnpm-lock.yaml": "",
            "yarn.lock": "",
            "package-lock.json": "",
        },
        {"package.json": "{bad", "tsconfig.json": "{}", "package-lock.json": ""},
        {
            "go.mod": "module demo",
            "Cargo.toml": "",
            "pom.xml": "",
            "pubspec.yaml": "flutter:",
            "CMakeLists.txt": "",
            "Makefile": "",
            "Dockerfile": "",
            ".git": "gitdir: elsewhere",
            ".github/workflows/ci.yml": "",
            ".gitlab-ci.yml": "",
            "scripts/test.sh": "",
            "spec/test": "",
        },
    ):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / "project"
            root.mkdir()
            for name, content in project_files.items():
                dest = root / name
                dest.parent.mkdir(parents=True, exist_ok=True)
                dest.write_text(content)
            projects.append({"files": project_files, "expected": detect_project(root)})
    return {
        "agents": [asdict(agent) for agent in AGENTS.values()],
        "bundled": bundled,
        "templates": template_cases,
        "custom_files": files,
        "custom": custom,
        "projects": projects,
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    text = json.dumps(capture(), indent=2, ensure_ascii=False) + "\n"
    if args.check:
        if not FIXTURE.is_file() or FIXTURE.read_text() != text:
            raise SystemExit(
                "Python parity fixtures are stale; review changes and run tests/reference/capture_parity.py"
            )
        print("Python parity fixtures match current behavior.")
    else:
        FIXTURE.parent.mkdir(parents=True, exist_ok=True)
        FIXTURE.write_text(text)
        print(f"Wrote {FIXTURE.relative_to(PROJECT_ROOT)}")


if __name__ == "__main__":
    main()
