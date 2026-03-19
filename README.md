# Journal

Persistent journal system for capturing thinking in motion. Automatically extracts engineering concepts from commits, links them to standing documents (your personal epistemology), and builds a queryable record of how thinking has moved over time.

MQTT-native, Postgres-backed with pgvector, built in Go.

## Prerequisites

- Go 1.21+
- Docker + Docker Compose
- [Ollama](https://ollama.com) running on the host with `nomic-embed-text` pulled
- `mosquitto_sub` (optional, for MQTT debugging)

```bash
ollama pull nomic-embed-text
```

## Setup

```bash
# 1. Install Go dependencies
make deps

# 2. Copy and review config
cp .env.example .env.dev

# 3. Start Postgres + Mosquitto
make infra

# 4. Build all binaries
make build-primitives
```

Migrations run automatically when any service starts — no manual migration step needed.

## Configuration

Key variables in `.env.dev`:

| Variable | Default | Description |
|---|---|---|
| `DB_HOST` / `DB_PORT` | `localhost:5433` | Postgres (offset from 5432 to avoid conflicts) |
| `MQTT_BROKER_URL` | `tcp://localhost:1884` | Mosquitto (offset from 1883) |
| `OLLAMA_BASE_URL` | `http://localhost:11434` | Ollama on host (not in Docker) |
| `OLLAMA_EMBED_MODEL` | `nomic-embed-text` | Embedding model — must produce 768-dim vectors |
| `ASSOCIATION_THRESHOLD` | `0.3` | Minimum similarity for entry-standing associations |

## Binaries

| Binary | Purpose |
|---|---|
| `entry-ingest` | Long-running service — listens on MQTT, ingests journal entries, computes embeddings, associates with standing documents |
| `ingest-standing` | CLI — ingest a standing document from a markdown file |
| `reembed` | CLI — re-embed entries that have null embeddings (run when Ollama was previously unavailable) |
| `concept-extract` | CLI — extract engineering concepts from recent commits in a repository |

## Running in Dev

Start the entry ingestion service (keep running in background):

```bash
make run-entry-ingest
```

Ingest your standing documents:

```bash
# Single document
make run-ingest-standing FILE=standing-documents/my-doc.md

# All documents in standing-documents/
make ingest-all-standing
```

Extract concepts from a repository:

```bash
# Last 1 day (default)
make run-concept-extract REPO=/path/to/repo

# Last 7 days, deep extraction
make extract REPO=/path/to/repo DAYS=7
```

## Inspecting State

```bash
make list-standing      # Standing documents + embedding status
make list-entries       # Recent journal entries
make list-associations  # Entry-to-standing-document similarity links

make psql               # Open psql shell directly
make mqtt-sub           # Watch all journal MQTT traffic (requires mosquitto_sub)
```

## Infrastructure

```bash
make infra              # Start Postgres + Mosquitto
make infra-down         # Stop containers (data preserved)
```

> `docker compose down -v` destroys all data including volumes.

Ports are offset from defaults to avoid conflicts with other local services:
- Postgres: `localhost:5433`
- Mosquitto: `localhost:1884`
- Ollama: `localhost:11434` (standard, running on host)

## What Are Standing Documents

Standing documents are markdown files describing patterns that have been found true across many contexts — your personal epistemology. They define the gravitational field that journal entries are pulled toward. The journal is only as useful as the standing documents that exist before entries start accumulating.

See [concepts/journal-concept.md](concepts/journal-concept.md) for the full model.
