# Journal

## Build & Test

```bash
make infra              # Start Postgres (5433) + Mosquitto (1884)
make build-primitives   # Build all 8 binaries to ./build/
make test               # go test ./...
make psql               # psql shell via docker exec
make mqtt-sub           # Watch journal/# MQTT traffic
```

## Constraints

- **Ports are offset** — Postgres: 5433 (not 5432), Mosquitto: 1884 (not 1883). Avoid conflict with Minerva.
- **Ollama runs on host**, not Docker. `OLLAMA_BASE_URL` must point to host. Embed endpoint is `/api/embed` with `"input"` field (not `/api/embeddings`, not `"prompt"`). Chat endpoint is `/api/chat` with `messages` array.
- **Ollama mutex** — serialize all embedding calls in long-running services. Concurrent requests time out.
- **Paho payload copy** — MQTT handlers must copy payload before goroutines: `data := make([]byte, len(payload)); copy(data, payload)`. Paho reuses the buffer.
- **pgvector dimension is 768** — matches nomic-embed-text. Changing model requires migrating all vector columns.
- **Migrations auto-run on service startup** — files in `internal/database/migrations/` must be numbered sequentially.
- **Standing doc slugs are stable** — derived from filename. Renaming creates a new slug (new doc), not a new version.
- **`until_timestamp` is `*time.Time`** — NULL for pre-migration-004 entries. Guard with `.IsZero()` before storing.
- **`GetAllCurrentEmbeddings` and `GetRecentEntriesWithEmbeddings` must NOT be removed** — used by `entry-ingest` and `reembed` respectively. Not referenced by `trend-detect` or `brief-assemble` anymore but still needed.
- **`SoulSpeedSlug = "soul-speed"` constant** in `internal/services/space.go` must match the actual standing doc slug. If that doc is renamed, update the constant.
- **`brief-assemble` always times out** until Minerva implements `minerva/query/brief`. Expected behavior — not a bug.
- **Concept extractor produces per-day entries** — one entry per calendar day with commits. `since_timestamp`/`until_timestamp` are the commit day boundaries, not extraction time.
- **`ASSOCIATION_THRESHOLD`** (default 0.3) and **`BRIEF_RELEVANCE_THRESHOLD`** (default 0.6) need calibration with real data. Start high on brief threshold.
- **Spread/Soul Speed labels are guesses** — thresholds in `buildHumanSummary` and `soulSpeedLabel` in `cmd/trend-detect/main.go` are empirical starting points.
- **PROPFIND Depth header required** — without `Depth: 1`, most WebDAV servers return only the collection itself and no children. The `internal/webdav/client.go` `List` method sets this, but any new PROPFIND call must include it explicitly.
- **`UpsertWebDAVState` timing** — must be called only after a successful MQTT publish, not after content extraction. Calling it earlier marks a file as ingested even if the publish failed.

## Minerva Integration Protocol

See `README.md` — Minerva Integration section.
