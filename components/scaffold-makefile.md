# Project Scaffolding — Makefile

Generate a Makefile as the single entry point for all project operations.

## Required Targets

- **`make help`** (default): Auto-discover targets via grep pattern:
  ```makefile
  help: ## Show this help message
  	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
  ```
- **`make install`**: Check/install system deps → init git submodules → install language packages → build assets. Support `VERBOSE=1`. Delegate to `scripts/install.sh`.
- **`make uninstall`**: Remove packages, venvs, node_modules, lock files, build artifacts. Delegate to `scripts/uninstall.sh`.
- **`make clean`**: Lightweight cleanup — `.venv`, `node_modules`, `dist/`, `__pycache__/`, `.ruff_cache/`, build artifacts. Safe to run anytime.
- **`make build`**: Production build. Support `CHANNEL`, `VERBOSE`, `CLEAN` vars.
- **`make test`**: Run project test suite.
- **`make format`** / **`make tidy`**: Run formatters/linters. Delegate to `tidy.sh`.
- **`make commit`**: Format, re-stage, commit. Delegate to `scripts/commit.sh`.

## Conventions
```makefile
.PHONY: help install uninstall clean build test format tidy commit
SHELL := /bin/bash
VERSION := $(shell cat VERSION 2>/dev/null | tr -d '\n' || echo "0.1.0")
```
- Always `.PHONY` non-file targets. Set `SHELL := /bin/bash`. Read version from `VERSION` file.
- Use `?=` for user-overridable defaults. Provide aliases (`tidy: format`, `setup: install`).

## Supporting Scripts (under `scripts/`)
- **`install.sh`**: Orchestrate installation with color-coded progress output and timestamped logs.
- **`uninstall.sh`**: Remove venvs, node_modules, locks, outputs, markers with status output.
- **`clean.sh`**: Lighter version — caches and build artifacts only.
- **`ensure_deps.sh`**: Check required system tools, auto-install if missing, cross-platform (Linux + macOS).
