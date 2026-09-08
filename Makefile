.PHONY: help ensure-uv generate clean list install uninstall format tidy commit update upgrade test

SHELL := /bin/bash
VERSION := $(shell cat VERSION 2>/dev/null | tr -d '\n' || echo "0.1.0")

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

INSTALL_DIR ?= $(HOME)/.local/bin

install: build ## Build and install the native CLI and skills (AGENT=all|both|key, SKILLS_ONLY=1 skips agent installation)
	@./build/ash-go install $(if $(AGENT),--agent $(AGENT),) $(if $(SKILLS_ONLY),--skills-only,)
	@bash scripts/dev/install-local.sh build/ash-go "$(INSTALL_DIR)"

ensure-uv: ## Install development-only Python tooling for the reference tests
	@bash scripts/install_uv.sh
	@uv sync --group dev

uninstall: build ## Remove Ashley skill links and the installed binary; retain user data
	@./build/ash-go uninstall
	@rm -f "$(INSTALL_DIR)/ash"

generate: build ## Generate skills with the native CLI
	@./build/ash-go --root "$(CURDIR)" generate --output "$(CURDIR)"

list: build ## List available skill definitions
	@./build/ash-go --root "$(CURDIR)" list

format: ## Run ruff formatter
	@bash tidy.sh
tidy: format

clean: ## Remove build outputs, release archives, generated skills and test caches
	@rm -rf build/ dist/ generated/ test-results/ htmlcov/ .pytest_cache/ .ruff_cache/
	@rm -f coverage.out build-coverage.out .coverage .coverage.*
	@find src tests scripts -type d -name __pycache__ -prune -exec rm -rf {} +
	@echo "Build outputs and caches removed."

update: build ## Update an explicit developer checkout (BRANCH=main, SKIP_TOOL=1 skips agent updates)
	@./build/ash-go --root "$(CURDIR)" update --branch $(or $(BRANCH),main)

upgrade: build ## Upgrade coding-agent CLIs (AGENT=key, default: all)
	@./build/ash-go upgrade $(if $(AGENT),$(AGENT),--all)

# Keep the full Python regression suite until Go has complete feature parity.
.PHONY: test test-python test-go test-integration build gate format-check lint package publish release-check go-build go-test go-parity go-dist workflow-check

test: ## Run all regression, parity, race, coverage, and binary integration tests
	@$(MAKE) --no-print-directory test-python
	@$(MAKE) --no-print-directory go-parity
	@$(MAKE) --no-print-directory test-go
	@$(MAKE) --no-print-directory build
	@$(MAKE) --no-print-directory test-integration

test-python: ## Run the Python reference and release-tool regression tests
	uv run pytest tests/python/ -q

test-go: ## Run Go race tests, enforce 70% coverage, and vet
	@mkdir -p build
	go test -race -coverpkg=./... -coverprofile=build/coverage.out ./...
	go tool cover -func=build/coverage.out
	@go tool cover -func=build/coverage.out | awk '/^total:/ { seen=1; if ($$3+0 < 70) exit 1 } END { if (!seen) exit 1 }'
	go vet ./...

go-test: test-go

go-parity: ## Verify frozen Python reference fixtures (development/CI only)
	uv run python tests/reference/capture_parity.py --check

build: ## Build the standalone Go CLI (build/ash-go)
	@mkdir -p build
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o build/ash-go ./cmd/ash

go-build: build

test-integration: ## Test the built executable, package, and binary installer
	uv run pytest tests/integration/ -q

format-check: ## Check Go/Python formatting, lint and shell syntax without changing files
	uv run ruff check .
	uv run ruff format --check .
	@test -z "$$(gofmt -l assets.go cmd internal)"
	@bash -n tidy.sh
	@while IFS= read -r -d '' script; do bash -n "$$script" || exit; done < <(find scripts -type f -name '*.sh' -print0)

workflow-check: ## Validate GitHub Actions syntax and expressions
	go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7 -shellcheck= -pyflakes=

lint: format-check workflow-check

gate: ## Full local/CI gate: formatting, tests, coverage, parity, build and installation
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
	uv run python scripts/release/version.py check "$(TAG)"

publish: ## Gate, update versions, push/tag and draft a binary release (V=... NOTES=notes.md YES=1)
	@V="$(V)" NOTES="$(NOTES)" YES="$(YES)" bash scripts/release/publish.sh

.PHONY: ui-reference
ui-reference: ## Regenerate terminal-layout fixtures from the original Python UI
	uv run python tests/reference/capture_ui.py
