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
- Ollama (nomic-embed-text) for embedding computation
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
