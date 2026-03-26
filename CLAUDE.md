# Journal

## Build & Test

```bash
make infra              # Start Postgres (5433) + Mosquitto (1884)
make build-primitives   # Build all 8 binaries to ./build/
make test               # go test ./...
make psql               # psql shell via docker exec
make mqtt-sub           # Watch journal/# MQTT traffic
```

## Production Infrastructure

- **Nomad**: `http://192.168.10.42:4646` — set `NOMAD_ADDR=http://192.168.10.42:4646`
- **Vault**: `https://192.168.10.42:8200` — set `VAULT_ADDR=https://192.168.10.42:8200` + `VAULT_SKIP_VERIFY=true`
- **Artifact server**: `http://192.168.10.50:8080/api/binaries/journal/<arch>/<binary>`
- **Nomad jobs**: `deploy/nomad/` — run on `the-collective` DC, GPU nodes excluded
- **Secrets**: `vault kv get secret/nomad/journal`

## Constraints

- **Ports are offset** — Postgres: 5433 (not 5432), Mosquitto: 1884 (not 1883). Avoid conflict with Minerva.
- **Port 8765 is used by `space-viz`** — the interactive visualization server. Avoid using this port for future services.
- **Ollama runs on host**, not Docker. `OLLAMA_BASE_URL` must point to host. Embed endpoint is `/api/embed` with `"input"` field (not `/api/embeddings`, not `"prompt"`). Chat endpoint is `/api/chat` with `messages` array.
- **Ollama mutex** — serialize all embedding calls in long-running services. Concurrent requests time out.
- **Paho payload copy** — MQTT handlers must copy payload before goroutines: `data := make([]byte, len(payload)); copy(data, payload)`. Paho reuses the buffer.
- **pgvector dimension is 4096** — matches qwen3-embedding:8b. Changing model requires migrating all vector columns.
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
- **`InsertEntry` upsert excludes `annotation` and `created_at` intentionally** — `annotation` is user-written and must survive re-extraction; `created_at` must reflect first ingestion. Do not add them to the `DO UPDATE SET` list.
- **Partial index predicate must match exactly in `ON CONFLICT`** — `ON CONFLICT (repository, since_timestamp) WHERE since_timestamp IS NOT NULL` — Postgres requires the predicate wording to be a textual match of the index definition. Any new `ON CONFLICT` against this index must use the identical clause.
- **NULL `since_timestamp` rows are not protected by the upsert** — pre-004 entries with NULL `since_timestamp` are exempt from the unique constraint. Two rows with NULL `since_timestamp` for the same repo do not conflict. This is intentional; all entries from `concept-extract` always set `since_timestamp`.
- **`flag.Visit` not `flag.VisitAll` for mutual exclusion** — `--days` defaults to 1, so checking `*days != 1` would incorrectly block `--days 1`. `flag.Visit` iterates only flags explicitly set on the command line.

## Minerva Integration Protocol

See `README.md` — Minerva Integration section.
