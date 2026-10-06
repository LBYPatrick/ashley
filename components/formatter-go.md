# Go Formatter & Static Analysis

Use this reference for Go projects alongside the language-specific references for other languages in a mixed repository.

- Follow the [Google Go Style Guide](https://google.github.io/styleguide/go/). `gofmt` is the formatting authority and ships with Go; it needs no separate installation or configuration file.
- Detect `go.mod`, `go.work`, the project's supported Go version, existing Makefile targets, and existing lint configuration before adding tooling. Preserve configured tools and pinned versions.
- Make `tidy.sh` / `make format` run `gofmt -w` on project-owned Go files, excluding vendored and generated code. If import management is desired, use the configured `goimports`; pin a Go-compatible `golang.org/x/tools/cmd/goimports` version when installing it.
- For CI, use `gofmt -l` over the same files and explicitly fail if its output is nonempty; `gofmt -l` alone does not fail for formatting differences. Do not modify files in a format-check target.
- Run `go vet ./...` for static analysis. Reuse an existing golangci-lint configuration when present rather than adding competing linters. Keep lint and tests separate from the mutation-only formatter target.
- Verify with `go test ./...` and `go build ./...`, from each module root when the repository has multiple modules. Use `go test -race ./...` for concurrent code where the toolchain supports it.
- `go mod tidy` changes dependency metadata; it is not a source formatter. Run it only when dependencies change and review `go.mod` / `go.sum`.

References: [gofmt](https://pkg.go.dev/cmd/gofmt), [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports), [go vet](https://pkg.go.dev/cmd/vet).
