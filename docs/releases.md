# Binary release development

Ashley remains open source. Release users download a compiled executable;
source checkouts, language toolchains and dependency resolution belong only
on developer/CI machines. Feature acceptance is recorded in
[migration acceptance](migration/assessment.md). The default `ash` command
runs Go, including the interactive screens, agent execution, history, sessions,
installation and updates. Python remains a development reference used by the
test suite.

## Verification

| Command | Checks |
| --- | --- |
| `make gate` | Formatting, lint, workflow validation, all tests, build and binary integration |
| `make test` | Python regressions/release tests, reference fixture freshness, Go race/coverage/vet, build, binary integration |
| `make test-python` | Python reference suite and publishing/version validation |
| `make test-go` | Go tests with race detection and aggregate coverage ≥70%, then vet |
| `make go-parity` | Python still produces the checked-in reference fixtures |
| `make build` | Pure-Go native executable at `build/ash-go` |
| `make test-integration` | Built binary, no-runtime execution, archive contents, checksum installation and failure preservation; run `make build` first |
| `make go-dist` | All four macOS/Linux × arm64/amd64 release packages |
| `make package PLATFORM=linux ARCH=amd64` | One selected binary archive plus checksum |

`tests/fixtures/parity/python.json` freezes Python outputs for every bundled skill and
prompt, custom inheritance/globs/resources, representative Jinja templates and
project detection. To intentionally change reference behavior, review the Python
change, run `uv run python tests/reference/capture_parity.py`, and inspect the fixture diff.
Do not regenerate fixtures just to make failing Go tests pass.

The gate runs on macOS and Linux in GitHub Actions. Local verification only runs
native executables; cross-compilation checks the other targets compile but does
not replace testing those architectures on hardware. Workflow tests and command
doubles do not publish real tags or authenticate coding agents.

## Artifacts and installation

Each package is `ashley-X.Y.Z-OS-ARCH.tar.gz`, containing `ash` and `LICENSE`.
A matching `.tar.gz.sha256` verifies the downloaded archive. Assets are embedded
from the source tree at compile time; no Python sources or checkout are needed.

After a release exists:

```bash
curl -fsSL https://raw.githubusercontent.com/LBYPatrick/ashley/main/scripts/install.sh | bash
# Explicit experimental release, without replacing the regular ash installation:
curl -fsSL https://raw.githubusercontent.com/LBYPatrick/ashley/main/scripts/install.sh | \
  bash -s -- --version 0.4.0-alpha.1 --install-dir "$HOME/.local/ashley-experimental/bin"
```

The prerelease version above is an example, not a currently published release.
The installer defaults to the latest stable tag. `ASHLEY_VERSION`,
`ASHLEY_INSTALL_DIR`, and `ASHLEY_REPO` provide equivalent configuration; a fork
can set `ASHLEY_REPO=owner/repository`. A corrupt download or wrong binary version
leaves the previous executable intact. The remote installer downloads release
binaries; `make install` builds and copies a standalone executable locally.

## Python migration rollout

Merge `scripts/migrate-python.sh` with the native runtime before directing users
to it. Publish a native release with all four archives and checksums first;
legacy Python tags have no binary assets. The migration command and offline
`--binary` path are documented in the [README](../README.md#migrating-from-python).
The script uses `ash install --legacy-root CHECKOUT --skills-only` to recognize
links owned by that particular old checkout, import user files, and replace the
launcher after setup succeeds. It never invokes the old Python environment.

## Publishing

The repository skill lives at `.agents/skills/publish-release/SKILL.md`, with a
Claude-compatible link. Publishing is a maintainer operation, not an installation
step. It requires Go, uv/Python, Git and authenticated GitHub CLI on the maintainer
machine; none are included in the release downloads.

1. Merge tested implementation and changelog changes to `main`.
2. Choose a version. Stable `X.Y.Z` is refused until every parity group is complete.
   Experimental versions use `X.Y.Z-alpha.N`, `X.Y.Z-beta.N`, or `X.Y.Z-rc.N`.
3. Commit the dated changelog section and leave an empty Unreleased section.
4. Write release notes to a temporary Markdown file, then run:
   `make publish V=X.Y.Z NOTES=/absolute/path/notes.md YES=1`.
5. Monitor `.github/workflows/release.yaml`. It checks tag/VERSION/Python metadata/
   README agreement, reruns the full gate, builds all four platforms, verifies
   checksums, retains draft notes, and publishes only the complete artifact set.

Omit `YES=1` to prepare versions and run checks without committing or pushing.
The script refuses a non-main branch, stray changes, a preexisting tag, or a branch
behind origin/main. If remote mutation fails partway through, inspect the tag and
draft before retrying. Re-run a failed Actions build for an existing correct tag;
never force-move a public version tag. `make release-check TAG=vX.Y.Z` runs local
identity/parity validation without publishing.

The source branch stays public on GitHub, including GitHub's automatic source
archive links. The supported end-user installation path downloads the binary
artifacts rather than those source archives.
