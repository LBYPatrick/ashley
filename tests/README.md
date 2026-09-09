# Test layout

- `integration/`: Go tests against the built executable, real terminals, release archives, installers, and Python-to-Go migration.
- `fixtures/parity/python.json`: frozen compatibility outputs from the former Python runtime, read directly by Go tests.

Go unit tests live beside their packages in `internal/`. Release validation and
publisher tests live in `scripts/release/`. Run `make gate` from the repository
root for the complete suite; `make test-integration` builds the binary and runs
the integration suite with the `integration` build tag and race detection.

Fixtures are reviewed test inputs. The Python implementation and capture tools
have been removed; intended behavior changes need an explicit fixture review and
focused Go regression tests. Screen tests cover responsive geometry, content,
and interaction. Real-terminal integration copies only the binary into an
isolated directory with an empty PATH and verifies Sync for all five agents.
The Go migration tests preserve legacy launchers, custom skills, and history,
including verified downloads and failures that must leave the launcher intact.

Put optional visual exports under `test-results/`, for example:

```sh
ASHLEY_UI_EXPORT="$PWD/test-results/ui" go test ./internal/tui
```

Binaries, archives, coverage output, generated skills, caches, and visual exports
must remain untracked. `make clean` removes disposable build/test outputs.
