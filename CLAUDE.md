# Journal

Persistent journal system for capturing thinking in motion. MQTT-native, Postgres-backed
with pgvector, built in Go following Minerva's independent-primitive architecture.

## Build & Run

```bash
make infra          # Start Postgres (5433) + Mosquitto (1884)
make build-primitives  # Build all cmd/ binaries to ./build/
make test           # Run tests
make psql           # Open psql shell via docker exec
make mqtt-sub       # Watch all journal MQTT topics
```

## Architecture

- Independent Go binaries communicating via MQTT (Eclipse Paho, QoS 1)
- Postgres 16 + pgvector for persistence and vector similarity
- Ollama (nomic-embed-text for embeddings, configurable chat model for extraction)
- Config via environment variables + godotenv
- Structured logging via logrus (JSON format)

## Development Ports

Ports are offset from defaults to avoid conflicts with Minerva:
- **Postgres**: localhost:5433 (not 5432)
- **Mosquitto**: localhost:1884 (not 1883)
- **Ollama**: localhost:11434 (standard, shared)

## Constraints

- **pgvector dimension is 768** — all `vector(768)` columns must match nomic-embed-text output.
  Changing the embedding model requires migrating every vector column.
- **Ollama embed endpoint is `/api/embed`** (not `/api/embeddings`). Request uses `"input"` not `"prompt"`.
- **Docker Compose volumes**: `docker compose down -v` destroys all data.
- **Migrations run automatically** on service startup via embedded SQL files.
  Migration files in `internal/database/migrations/` must be numbered sequentially.
- **Standing document slugs are stable** — derived from filename, used for cross-references.
  Renaming a source file creates a new slug (new document), not a new version. Use `--slug` override if filenames change.
- **Standing documents require embeddings** — the CLI fails if Ollama is unavailable rather than storing without an embedding.
- **Journal entries tolerate missing embeddings** — stored with null embedding if Ollama is down, re-embedded later via `reembed` CLI.
- **Ollama mutex** — serialize embedding calls in long-running services. Concurrent requests cause timeouts. One embedding at a time.
- **Paho payload copy** — MQTT handlers must copy `payload` before spawning goroutines (`data := make([]byte, len(payload)); copy(data, payload)`). Paho reuses the buffer.
- **Association threshold** — `ASSOCIATION_THRESHOLD` env var (default 0.3). With nomic-embed-text, unrelated texts ~0.1-0.3, related ~0.4-0.8. May need tuning with real data.
- **Ollama chat endpoint is `/api/chat`** (not `/api/generate`). Request uses `messages` array with `{role, content}` objects.
- **Chat model config** — `OLLAMA_CHAT_MODEL` (default `qwen2.5:7b`) and `OLLAMA_CHAT_NUM_CTX` (default `32768`) control concept extraction. Chat timeout shares `OLLAMA_TIMEOUT`.
- **Concept extractor** — `cmd/concept-extract` is a one-shot CLI like `ingest-standing`. Use `make extract REPO=<path> DAYS=7`. The `--deep` flag is recommended for meaningful embeddings (produces theoretical territory).
- **`--week` flag** resolves to the previous calendar week (Mon 00:00:00 UTC — Sun 23:59:59 UTC). Deterministic — running any day of the week produces the same window for the prior week.
- **`until_timestamp` is `*time.Time` in `JournalEntry`** — pointer handles NULL for entries created before migration 004. Check `.IsZero()` before assigning from MQTT messages or you will store zero-time as a real timestamp.
- **`trend-detect` needs ≥3 entries** with embeddings to produce output. With fewer it logs a message and exits 0 — this is intentional, not a bug.
- **`BRIEF_RELEVANCE_THRESHOLD`** env var (default 0.6) — start high to prefer silence. Calibrate down only based on evidence from `brief_feedback` data.
- **`brief-assemble` depends on Minerva** — publishes to `minerva/query/brief`, which has no handler on the Minerva side yet. Until Minerva implements it, `brief-assemble` will always produce `silence_reason = "minerva_timeout"`.
