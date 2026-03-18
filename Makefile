SHELL := /bin/bash

# Directories
BUILD_DIR := ./build

# MQTT broker URL for helper targets
MQTT_BROKER ?= tcp://localhost:1884

.PHONY: help deps build-primitives build-dev test fmt clean \
        setup-dev infra infra-down migrate psql mqtt-sub

# Default target
help: ## Show this help message
	@echo "Journal — Persistent thinking in motion"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Dependencies
deps: ## Install Go dependencies
	go mod download
	go mod verify

# Build all primitives
build-primitives: ## Build all primitive binaries
	@echo "Building primitives..."
	@mkdir -p $(BUILD_DIR)
	@echo "Done. Binaries in $(BUILD_DIR)/"

# Build with debug symbols
build-dev: ## Build all primitives with debug symbols
	@echo "Building primitives for development..."
	@mkdir -p $(BUILD_DIR)
	@echo "Done."

# Tests
test: ## Run tests
	go test -v ./...

test-coverage: ## Run tests with coverage report
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Format
fmt: ## Format Go code
	go fmt ./...

# Clean
clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Development setup
setup-dev: deps ## Setup development environment
	@echo "Setting up development environment..."
	cp .env.example .env.dev
	@echo "Edit .env.dev with your configuration"

# Infrastructure
infra: ## Start development infrastructure (Postgres + Mosquitto)
	docker compose up -d postgres mosquitto
	@echo "Postgres on localhost:5433, Mosquitto on localhost:1884"

infra-down: ## Stop development infrastructure
	docker compose down

# Database
migrate: ## Run database migrations
	@echo "Running migrations..."
	@docker exec journal_postgres psql -U journal -d journal -c "SELECT 1;" > /dev/null 2>&1 || \
		(echo "Error: Cannot connect to Postgres. Run 'make infra' first." && exit 1)
	@echo "Migrations are run automatically by services on startup."
	@echo "To verify, connect with: make psql"

psql: ## Open psql shell to journal database
	docker exec -it journal_postgres psql -U journal -d journal

# MQTT debugging
mqtt-sub: ## Watch all journal MQTT topics
	mosquitto_sub -h localhost -p 1884 -v -t "journal/#"

# Quick development workflow
quick-test: fmt test ## Format and test

# CI simulation
ci: deps fmt test build-primitives ## Full CI pipeline
