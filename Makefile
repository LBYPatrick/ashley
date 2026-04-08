.PHONY: help ensure-uv generate clean list install uninstall format tidy commit update test

SHELL := /bin/bash
VERSION := $(shell cat VERSION 2>/dev/null | tr -d '\n' || echo "0.1.0")

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

install: ensure-uv generate ## Generate skills, install to ~/.claude/skills/, and symlink CLI
	@bash scripts/install_claude.sh
	@uv run python -m ashley.install
	@mkdir -p "$(HOME)/.local/bin"
	@ln -sf "$(CURDIR)/bin/ash" "$(HOME)/.local/bin/ash"
	@echo "Installed: ash → $(HOME)/.local/bin/ash"

ensure-uv: ## Ensure uv is installed and deps synced
	@bash scripts/install_uv.sh
	@uv sync

uninstall: ## Remove skills from ~/.claude/skills/ and CLI from ~/.local/bin
	@uv run python -m ashley.install uninstall
	@rm -f "$(HOME)/.local/bin/ash"
	@echo "Removed: $(HOME)/.local/bin/ash"

generate: ## Generate skill markdown files from JSONC definitions
	@uv run python -m ashley.generate

list: ## List all available skill definitions
	@uv run python -m ashley.generate list

format: ## Run ruff formatter
	@bash tidy.sh
tidy: format

clean: ## Remove generated files
	@rm -rf generated/
	@echo "Generated files removed."

update: ## Pull latest, re-generate, and re-install skills if changed (BRANCH=main)
	@uv run python -m ashley.update $(or $(BRANCH),main)

test: ## Run tests
	@uv run pytest tests/ -v
