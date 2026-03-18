SHELL := /bin/bash

# Directories
BUILD_DIR := ./build

# MQTT broker URL for helper targets
MQTT_BROKER ?= tcp://localhost:1884

.PHONY: help deps build-primitives build-dev test fmt clean \
        setup-dev infra infra-down migrate psql mqtt-sub \
        run-ingest-standing ingest-all-standing list-standing \
        run-entry-ingest run-reembed list-entries list-associations \
        run-concept-extract extract

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
	go build -o $(BUILD_DIR)/ingest-standing ./cmd/ingest-standing/
	go build -o $(BUILD_DIR)/entry-ingest ./cmd/entry-ingest/
	go build -o $(BUILD_DIR)/reembed ./cmd/reembed/
	go build -o $(BUILD_DIR)/concept-extract ./cmd/concept-extract/
	@echo "Done. Binaries in $(BUILD_DIR)/"

# Build with debug symbols
build-dev: ## Build all primitives with debug symbols
	@echo "Building primitives for development..."
	@mkdir -p $(BUILD_DIR)
	go build -gcflags="all=-N -l" -o $(BUILD_DIR)/ingest-standing ./cmd/ingest-standing/
	go build -gcflags="all=-N -l" -o $(BUILD_DIR)/entry-ingest ./cmd/entry-ingest/
	go build -gcflags="all=-N -l" -o $(BUILD_DIR)/reembed ./cmd/reembed/
	go build -gcflags="all=-N -l" -o $(BUILD_DIR)/concept-extract ./cmd/concept-extract/
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

# ── Standing document targets ─────────────────────────────────────────────────

run-ingest-standing: build-primitives ## Ingest a standing document (FILE=path/to/doc.md)
	$(BUILD_DIR)/ingest-standing --file $(FILE) --config .env.dev

ingest-all-standing: build-primitives ## Ingest all standing documents from standing-documents/
	@for f in standing-documents/*.md; do \
		echo "Ingesting $$f..."; \
		$(BUILD_DIR)/ingest-standing --file "$$f" --config .env.dev; \
	done

list-standing: ## List current standing documents in the database
	@docker exec journal_postgres psql -U journal -d journal -c \
		"SELECT slug, title, version, created_at, \
		 CASE WHEN embedding IS NOT NULL THEN 'yes' ELSE 'no' END as has_embedding \
		 FROM standing_documents sd1 \
		 WHERE version = (SELECT MAX(version) FROM standing_documents sd2 WHERE sd2.slug = sd1.slug) \
		 ORDER BY slug;"

# ── Entry targets ────────────────────────────────────────────────────────────

run-entry-ingest: build-primitives ## Run entry ingestion service
	$(BUILD_DIR)/entry-ingest -config .env.dev

run-reembed: build-primitives ## Re-embed entries with null embeddings
	$(BUILD_DIR)/reembed -config .env.dev

list-entries: ## List recent journal entries
	@docker exec journal_postgres psql -U journal -d journal -c \
		"SELECT id, repository, LEFT(summary, 60) as summary, \
		 CASE WHEN embedding IS NOT NULL THEN 'yes' ELSE 'no' END as embedded, \
		 created_at \
		 FROM journal_entries ORDER BY created_at DESC LIMIT 20;"

list-associations: ## List entry-standing associations
	@docker exec journal_postgres psql -U journal -d journal -c \
		"SELECT je.id, je.repository, LEFT(je.summary, 40) as summary, \
		 esa.standing_slug, round(esa.similarity::numeric, 3) as similarity \
		 FROM entry_standing_associations esa \
		 JOIN journal_entries je ON je.id = esa.entry_id \
		 ORDER BY je.created_at DESC, esa.similarity DESC LIMIT 30;"

# ── Concept extractor ────────────────────────────────────────────────────────

run-concept-extract: build-primitives ## Run concept extraction (REPO=path DAYS=1)
	$(BUILD_DIR)/concept-extract --repo $(REPO) --days $(or $(DAYS),1) --config .env.dev

extract: build-primitives ## Extract concepts from a repo (REPO=path DAYS=7)
	$(BUILD_DIR)/concept-extract --repo $(REPO) --days $(or $(DAYS),1) --deep --config .env.dev

# CI simulation
ci: deps fmt test build-primitives ## Full CI pipeline
