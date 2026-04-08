# Format and Re-stage

Run the project's formatter (if configured) and ensure formatted files are staged before committing.

## Detecting a Formatter

Look for these signals:

- **General:** `tidy.sh` in the project root, `make format` or `make tidy` target in the Makefile
- **Python:** `ruff.toml`, `pyproject.toml` with `[tool.ruff]`, `.flake8`
- **TypeScript/JavaScript:** `.prettierrc`, `eslint.config.js`, `.eslintrc.*`, `biome.json`
- **Rust:** `rustfmt.toml`
- **Go:** `gofmt` / `goimports` (always available)

If no formatter is detected, skip this step entirely.

## Running the Formatter

Use the first available option:

1. `tidy.sh` (preferred — wraps all formatters)
2. `make format` or `make tidy`
3. The language-specific formatter directly (last resort)

## Re-staging

After the formatter runs, some staged files may have been modified on disk but are now out of sync with the index. Run `git add` on every file that the formatter touched so the formatted versions are what gets committed.
