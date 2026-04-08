# Python Formatter & Linter Setup

## Tools
- **ruff** — all-in-one linter + formatter. Rules: import sorting (`I`), unused imports (`F401`).
- **uv-sort** — sorts `pyproject.toml` dependency lists.
- **beautysh** — bash/shell script formatter.
- **mbake** — Makefile formatter (optional).

## Installation

Dev dependencies in `pyproject.toml`:
```toml
[dependency-groups]
dev = ["beautysh>=6.4.2", "mbake>=1.4.4", "ruff>=0.15.0", "uv-sort>=0.7.0"]
```

Create `scripts/install-formatter.sh`: check for `uv` (auto-install via `curl -LsSf https://astral.sh/uv/install.sh | sh` if missing), then `uv sync --group dev`.

## tidy.sh

Create `tidy.sh` in project root:
1. Lazy-check: if `uv run ruff --version` fails, run `scripts/install-formatter.sh`. Skip with `--skip-check`.
2. `uv run ruff check --select I,F401 --fix .`
3. `uv run ruff format .`
4. `uv run -m beautysh scripts/*.sh` (if shell scripts exist)
5. `uv run uv-sort`
6. `uv run -m mbake format Makefile` (if Makefile exists)

## Ruff Config in pyproject.toml
```toml
[tool.ruff]
target-version = "py313"
[tool.ruff.lint]
extend-select = ["I"]
[tool.ruff.lint.isort]
known-first-party = ["<project_name>"]
```

## Makefile Integration
```makefile
format: ## Run formatter and linter
	bash tidy.sh
tidy: format
```
