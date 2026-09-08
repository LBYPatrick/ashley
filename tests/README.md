# Test layout

- `python/`: regression tests for the Python reference and release tooling.
- `integration/`: tests against the built Go executable, release archive, and installers.
- `fixtures/`: reviewed expected outputs for cross-language and visual regression tests.
- `reference/`: development-only scripts that capture those fixtures from Python.

Go unit tests live beside their packages in `internal/`, following Go conventions.
Run `make gate` from the repository root for the complete suite.

Fixtures are test inputs, not disposable build outputs. Regenerate them only
when an intended behavior change has been reviewed, using `make ui-reference`
or `uv run python tests/reference/capture_parity.py`. Settings has a redesigned
Go layout with its own responsive and interaction tests; it no longer uses a
Python screen snapshot.

Put optional visual exports under `test-results/`, for example:

```sh
ASHLEY_UI_EXPORT="$PWD/test-results/ui" go test ./internal/tui
```

Binaries, archives, coverage output, generated skills, caches, and visual exports
must remain untracked. `make clean` removes disposable build/test outputs.
