# CI Setup — Makefile & GitHub Actions

Set up continuous integration that verifies code quality on every push and pull request.

## Makefile CI Targets

Ensure the project Makefile has these CI-relevant targets (create them if missing):

- **`make format`** / **`make tidy`** — run formatter+linter. Must exit non-zero on failure.
- **`make test`** — run the project's test suite. Must exit non-zero on failure.
- **`make build`** — build the project for production (if applicable).

If these targets don't exist, compose them based on the detected stack:
- **Python:** `ruff check . && ruff format --check .` for format-check, `pytest` for test.
- **TypeScript:** `pnpm exec prettier --check "src/**/*.{ts,tsx}" && pnpm exec eslint .` for format-check, `pnpm test` for test.
- **Go:** `gofmt -l . | grep . && exit 1` for format-check, `go test ./...` for test.
- **Rust:** `cargo fmt -- --check` for format-check, `cargo test` for test.

## GitHub Actions Workflow

Check for `.github/workflows/` directory. If no CI workflow exists, create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # Language-specific setup (pick what applies):

      # Python
      - uses: astral-sh/setup-uv@v5
        if: hashFiles('pyproject.toml') != ''
      - run: uv sync
        if: hashFiles('pyproject.toml') != ''

      # Node.js / TypeScript
      - uses: actions/setup-node@v4
        if: hashFiles('package.json') != ''
        with:
          node-version: '22'
      - uses: pnpm/action-setup@v4
        if: hashFiles('pnpm-lock.yaml') != ''
        with:
          version: latest
      - run: pnpm install
        if: hashFiles('pnpm-lock.yaml') != ''

      # Go
      - uses: actions/setup-go@v5
        if: hashFiles('go.mod') != ''
        with:
          go-version: 'stable'

      # Rust
      - uses: dtolnay/rust-toolchain@stable
        if: hashFiles('Cargo.toml') != ''

      # CI checks
      - name: Format check
        run: make format-check 2>/dev/null || make format
      - name: Test
        run: make test
      - name: Build
        run: make build
        if: hashFiles('Makefile') != '' && contains(hashFiles('Makefile'), 'build')
```

**IMPORTANT:** The template above is a starting point. Adapt it to the actual project:
- Only include setup steps for languages the project actually uses. Remove all others.
- If the project has no Makefile, call tools directly (e.g., `uv run pytest`, `pnpm test`).
- If the project uses a monorepo or workspace structure, adjust paths accordingly.

## Test Framework Detection & Setup

If the project has **no test framework configured**, set one up:

### Python
- Add `pytest` (and `pytest-cov` for coverage) to dev dependencies in `pyproject.toml`.
- Create `tests/` directory with a `conftest.py` and at least one test file.
- Add `make test` target: `uv run pytest --cov=<package> --cov-report=term-missing`.

### TypeScript
- Install `vitest` (preferred) or `jest`: `pnpm add -D vitest`.
- Create a test file alongside source (e.g., `src/utils/__tests__/helper.test.ts`).
- Add `"test": "vitest run"` to `package.json` scripts and `make test` target: `pnpm test`.

### Go
- Tests are built-in. Create `*_test.go` files. `make test` target: `go test -race -coverprofile=coverage.out ./...`.

### Rust
- Tests are built-in. Create `#[cfg(test)]` modules or `tests/` directory. `make test` target: `cargo test`.

## Coverage

Add coverage reporting to the test step:
- **Python:** `pytest --cov=<package> --cov-report=term-missing --cov-fail-under=70`
- **TypeScript (vitest):** add `--coverage` flag, configure in `vitest.config.ts`: `coverage: { provider: 'v8', thresholds: { lines: 70 } }`
- **Go:** `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out`
- **Rust:** use `cargo-tarpaulin`: `cargo tarpaulin --out Stdout --fail-under 70`

Set initial threshold at 70% and increase as coverage improves.

## Validation Procedure

After setting up CI:
1. Run `make format` (or format-check equivalent) locally — must pass.
2. Run `make test` locally — must pass with coverage report.
3. Run `make build` locally (if applicable) — must pass.
4. If GitHub Actions workflow was created, verify YAML is valid and push to trigger it.
