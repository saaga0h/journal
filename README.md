# Journal a.k.a Rhino Horn

> A system that imposes geometric order on organic thinking, named after Dalí's obsession with logarithmic spirals

Persistent journal system for capturing thinking in motion. Automatically extracts engineering concepts from commits, links them to standing documents (your personal epistemology), and builds a queryable record of how thinking has moved over time.

MQTT-native, Postgres-backed with pgvector, built in Go.

## Intellectual Property

The mechanisms described in this repository — including the Soul Speed scalar field, manifold-based proximity computation, and GLF-weighted gravity profiles — are original to this project. Prior art documentation is maintained in this repository:

- **[CONCEPTS.md](CONCEPTS.md)** — the ideas: core abstractions, novel mechanisms, design decisions, and the mathematical specificity of each
- **[ARCHITECTURE.md](ARCHITECTURE.md)** — the structure: components, data flows, wire formats, algorithms, and invariants

## Prerequisites

- Go 1.21+
- Docker + Docker Compose
- [Ollama](https://ollama.com) running on the host with `qwen3-embedding:8b` pulled
- `mosquitto_sub` (optional, for MQTT debugging)

```bash
ollama pull qwen3-embedding:8b
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
| `OLLAMA_EMBED_MODEL` | `qwen3-embedding:8b` | Embedding model — must produce 4096-dim vectors |
| `ASSOCIATION_THRESHOLD` | `0.3` | Minimum similarity for entry-standing associations |
| `BRIEF_RELEVANCE_THRESHOLD` | `0.6` | Minimum similarity for brief entry relevance |
| `WEBDAV_URL` | — | WebDAV server URL (e.g., `https://nextcloud.example.com/remote.php/webdav/`) |
| `WEBDAV_USERNAME` | — | WebDAV username |
| `WEBDAV_PASSWORD` | — | WebDAV password |
| `WEBDAV_STANDING_PATH` | `/standing/` | WebDAV directory path for standing documents |
| `WEBDAV_ENTRIES_PATH` | `/entries/` | WebDAV directory path for freeform entries |

## Binaries

| Binary | Purpose |
|---|---|
| `entry-ingest` | Long-running service — listens on MQTT, ingests journal entries, computes embeddings, associates with standing documents |
| `ingest-standing` | CLI — ingest a standing document from a markdown file |
| `ingest-webdav-standing` | CLI — fetch standing documents from a WebDAV server, skip unchanged (content hash), embed and store |
| `ingest-webdav-entries` | CLI — fetch freeform markdown entries from a WebDAV server, skip unchanged, parse date from content, run concept extraction, publish to MQTT |
| `reembed` | CLI — re-embed entries that have null embeddings (run when Ollama was previously unavailable) |
| `concept-extract` | CLI — extract engineering concepts from recent commits in a repository |
| `trend-detect` | CLI — compute gravity profile (per-standing-doc attraction) and soul speed, publish to MQTT |
| `brief-assemble` | Long-running service — MQTT-triggered brief assembler, queries Minerva with trend vector, surfaces one article or silence |
| `brief-feedback` | CLI — record read/skip feedback for a brief session (`--session-id`, `--action read\|skip`) |
| `space-viz` | Web server — interactive 3D visualization of entries in standing-doc space (X/Z: UMAP-reduced semantic clusters, Y: time, color: soul-speed) |

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
# Previous calendar week (Mon-Sun UTC), deep extraction
make extract REPO=/path/to/repo

# Last N days, deep extraction
make extract-days REPO=/path/to/repo DAYS=7

# Last 1 day (default)
make run-concept-extract REPO=/path/to/repo
```

Detect temporal trends:

```bash
# Compute and publish trend to MQTT
make run-trend-detect

# Compute trend, print human-readable summary + JSON (no MQTT publish)
make trend-detect-dry
```

Visualize entries in standing-doc space (interactive 3D):

```bash
# Build and open browser (macOS)
make run-space-viz

# Or run directly with custom flags
./build/space-viz --config .env.dev --days 90 --port 8765 --open
```

Flags: `--port` (default 8765), `--days` (default lookback window in days), `--open` (macOS auto-browser), `--config` (path to .env file)

Assemble and manage morning briefs:

```bash
# Run brief assembler service (keep running in background)
make run-brief-assemble

# Trigger brief generation immediately (development)
make trigger-brief
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

## Minerva Integration

`brief-assemble` queries Minerva for article recommendations based on the current trend. Minerva must implement both sides of this protocol.

### Query — Journal → Minerva

**Topic**: `minerva/query/brief`

```json
{
  "session_id": "189e5c5163294ec0",
  "gravity_profile": {
    "universe-design": 0.647,
    "gradient-lossy-functions": 0.634,
    "two-corpus": 0.626
  },
  "top_k": 5,
  "response_topic": "journal/brief/minerva-response"
}
```

`gravity_profile` is a map of standing_slug → GLF-weighted mean similarity for the current trend window. Soul-speed is excluded. `response_topic` is where Minerva must publish its response. Timeout is 30s (configurable via `--timeout-seconds`).

### Response — Minerva → Journal

**Topic**: `journal/brief/minerva-response`

```json
{
  "session_id": "189e5c5163294ec0",
  "article_url": "https://arxiv.org/abs/...",
  "article_title": "...",
  "score": 0.72
}
```

`session_id` must echo the query exactly — mismatches are discarded. `score` is compared against `BRIEF_RELEVANCE_THRESHOLD` (default 0.6); below threshold produces silence. If Minerva has no candidates, respond with `score: 0.0` rather than letting the timeout fire.

## What Are Standing Documents

Standing documents are markdown files describing patterns that have been found true across many contexts — your personal epistemology. They define the gravitational field that journal entries are pulled toward. The journal is only as useful as the standing documents that exist before entries start accumulating.

See [concepts/journal-concept.md](concepts/journal-concept.md) for the full model.

## Standing-Doc Space

Each journal entry is positioned in an N-dimensional space where axes are standing documents. The system computes similarity between entry embeddings and each standing document embedding, placing entries at coordinates (similarity-to-doc-1, similarity-to-doc-2, ...). An orthogonal axis called "soul speed" measures the aliveness of thinking — how much the entry churns internally, independent of which standing documents it touches.

**Gravity Profile** (`trend-detect` output): A weighted vector showing how strongly recent thinking gravitates toward each standing document. Weights are GLF-decayed by entry age. Values ~0.4–0.8 indicate gravitational pull, <0.1 suggests no attraction.

**Soul Speed**: A scalar from 0–1 measuring internal mental activity. High values (>0.65) indicate intense thinking; low values (<0.45) suggest dormant patterns. Computed separately from lateral standing-doc similarities.

**Cluster Spread**: Standard deviation of all entries' multi-doc vectors. Tight cluster (<0.03) suggests convergent thinking; dispersed (>0.08) suggests exploration across multiple docs.

## WebDAV Ingestion

Pull markdown documents from a WebDAV server (e.g., Nextcloud, any WebDAV-compatible server). Joplin can sync notebooks via WebDAV. Both ingestors are idempotent — unchanged files (same content hash) are skipped on subsequent runs.

**Standing Documents** (`ingest-webdav-standing`): Fetches `.md` files from `WEBDAV_STANDING_PATH`, extracts the title from the first `# ` heading, computes embeddings, and stores as standing documents. Slug is derived from filename (lowercase, spaces/underscores → hyphens).

**Freeform Entries** (`ingest-webdav-entries`): Fetches `.md` files from `WEBDAV_ENTRIES_PATH`, parses date from document content (fallback to WebDAV last-modified timestamp), runs concept extraction on the content, and publishes extracted concepts to MQTT for `entry-ingest` to process.

Configuration:
- `WEBDAV_URL` — Base URL of your WebDAV server
- `WEBDAV_USERNAME` / `WEBDAV_PASSWORD` — Authentication
- `WEBDAV_STANDING_PATH` — Directory for standing documents (default `/standing/`)
- `WEBDAV_ENTRIES_PATH` — Directory for freeform entries (default `/entries/`)

Run:

```bash
make sync-standing                # Ingest standing docs from WebDAV + recompute associations
make run-ingest-webdav-standing   # Ingest standing docs from WebDAV only
make run-ingest-webdav-entries    # Ingest freeform entries from WebDAV
```

Both ingestors skip unchanged files on repeat runs, making them safe to schedule. `sync-standing` treats WebDAV as source of truth — ingest then reassociate in one step.
