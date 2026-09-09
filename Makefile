.PHONY: help generate clean list install uninstall format tidy update upgrade test

SHELL := /bin/bash
VERSION := $(shell cat VERSION 2>/dev/null | tr -d '\n' || echo "0.1.0")

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

INSTALL_DIR ?= $(HOME)/.local/bin

install: build ## Build and install the native CLI and skills (AGENT=all|both|key, SKILLS_ONLY=1 skips agent installation)
	@./build/ash-go install $(if $(AGENT),--agent $(AGENT),) $(if $(SKILLS_ONLY),--skills-only,)
	@bash scripts/dev/install-local.sh build/ash-go "$(INSTALL_DIR)"

uninstall: build ## Remove Ashley skill links and the installed binary; retain user data
	@./build/ash-go uninstall
	@rm -f "$(INSTALL_DIR)/ash"

generate: build ## Generate skills with the native CLI
	@./build/ash-go --root "$(CURDIR)" generate --output "$(CURDIR)"

list: build ## List available skill definitions
	@./build/ash-go --root "$(CURDIR)" list

format: ## Format Go source and validate shell syntax
	@bash tidy.sh
tidy: format

clean: ## Remove build outputs, release archives, generated skills and test caches
	@rm -rf build/ dist/ generated/ test-results/
	@rm -f coverage.out build-coverage.out
	@echo "Build outputs and caches removed."

update: build ## Update an explicit developer checkout (BRANCH=main, SKIP_TOOL=1 skips agent updates)
	@./build/ash-go --root "$(CURDIR)" update --branch $(or $(BRANCH),main)

upgrade: build ## Upgrade coding-agent CLIs (AGENT=key, default: all)
	@./build/ash-go upgrade $(if $(AGENT),$(AGENT),--all)

.PHONY: test test-go test-integration build gate format-check lint package publish release-check go-build go-test go-dist workflow-check

test: ## Run all Go regression, race, coverage, and binary integration tests
	@$(MAKE) --no-print-directory test-go
	@$(MAKE) --no-print-directory test-integration

test-go: ## Run Go race tests, enforce 70% coverage, and vet
	@mkdir -p build
	go test -race -coverpkg=./... -coverprofile=build/coverage.out ./...
	go tool cover -func=build/coverage.out
	@go tool cover -func=build/coverage.out | awk '/^total:/ { seen=1; if ($$3+0 < 70) exit 1 } END { if (!seen) exit 1 }'
	go vet ./...

go-test: test-go

# Defaults follow the host; override for cross-compilation without a C compiler.
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BUILD_OUTPUT ?= build/ash-go$(if $(filter windows,$(GOOS)),.exe,)

build: ## Build a standalone CLI (GOOS=..., GOARCH=..., BUILD_OUTPUT=... optional)
	@mkdir -p "$(dir $(BUILD_OUTPUT))"
	CGO_ENABLED=0 GOOS="$(GOOS)" GOARCH="$(GOARCH)" go build -trimpath -ldflags='-s -w' -o "$(BUILD_OUTPUT)" ./cmd/ash

go-build: build

test-integration: build ## Test the executable, terminal UI, package, installer, and migration
	go test -race -tags=integration -timeout=5m ./tests/integration

format-check: ## Check Go formatting and shell syntax without changing files
	@test -z "$$(gofmt -l assets.go cmd internal scripts/release tests/integration)"
	@bash -n tidy.sh
	@while IFS= read -r -d '' script; do bash -n "$$script" || exit; done < <(find scripts -type f -name '*.sh' -print0)

workflow-check: ## Validate GitHub Actions syntax and expressions
	go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7 -shellcheck= -pyflakes=

lint: format-check workflow-check

gate: ## Full local/CI gate: formatting, tests, coverage, build and installation
	@$(MAKE) --no-print-directory lint
	@$(MAKE) --no-print-directory test

package: ## Build a binary release archive (PLATFORM=darwin|linux ARCH=arm64|amd64)
	@bash scripts/release/package.sh $(PLATFORM) $(ARCH)

go-dist: ## Build all four binary release archives and checksums
	@for platform in darwin linux; do \
		for arch in arm64 amd64; do \
			bash scripts/release/package.sh "$$platform" "$$arch" || exit; \
		done; \
	done

release-check: ## Verify release identity and full parity for stable releases (TAG=vX.Y.Z)
	go run ./scripts/release check "$(TAG)"

publish: ## Gate, update versions, push/tag and draft a binary release (V=... NOTES=notes.md YES=1)
	@V="$(V)" NOTES="$(NOTES)" YES="$(YES)" bash scripts/release/publish.sh
