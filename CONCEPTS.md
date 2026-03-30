# Journal — Concepts

> _A system that imposes geometric order on organic thinking._
> _Named after Dalí's obsession with logarithmic spirals._

**Date of initial implementation**: March 2026. Standing document corpus and Soul Speed mechanism described herein are original to this project unless otherwise noted.

---

> **Note on standing documents**: The standing documents referenced throughout this document (e.g. `soul-speed`, `gradient-lossy-functions`, `nonogram-topology`) are personal knowledge artifacts maintained separately from this repository. They are not public. Their names appear here because they define the conceptual axes of the author's Journal space — they are part of the system's design context, not its implementation. The system itself is fully operational with any set of standing documents; the ones named here are the author's own.

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [Standing Documents — Personal Epistemology as Geometric Axes](#2-standing-documents--personal-epistemology-as-geometric-axes)
3. [Semantic Embedding and the High-Dimensional Journal Space](#3-semantic-embedding-and-the-high-dimensional-journal-space)
4. [Manifolds as Standing Document Territories](#4-manifolds-as-standing-document-territories)
5. [Soul Speed — The Scalar Field](#5-soul-speed--the-scalar-field)
6. [Soul Speed's Influence on Manifold Gravity](#6-soul-speeds-influence-on-manifold-gravity)
7. [The Gravity Profile — Detecting Intellectual Drift](#7-the-gravity-profile--detecting-intellectual-drift)
8. [Unexpected Concepts — Phase Transitions and Emergent Territory](#8-unexpected-concepts--phase-transitions-and-emergent-territory)
9. [Git Commit Enrichment — Making Engineering Work Epistemically Legible](#9-git-commit-enrichment--making-engineering-work-epistemically-legible)
10. [Temporal Recency Weighting — The GLF Mechanism](#10-temporal-recency-weighting--the-glf-mechanism)
11. [The Brief — From Trend to External Query](#11-the-brief--from-trend-to-external-query)
12. [Design Decisions and Roads Not Taken](#12-design-decisions-and-roads-not-taken)
13. [Relationships Between Concepts](#13-relationships-between-concepts)

---

## 1. Problem Statement

### What This Solves

Engineering thinking is diffuse and accumulates invisibly. A person writing code across multiple repositories over months is continuously generating insight — design decisions, theoretical connections, pattern recognitions — but almost none of it is captured in a form that is queryable, temporally indexed, or geometrically organized. Commit messages are terse. Notes are unsystematic. The actual shape of thought — what ideas are being explored, how fast they are moving, which established frameworks they are orbiting — is not recorded anywhere.

The standard response is better note-taking. This system takes a different position: the primary input should be the work itself (git commits), not a parallel documentation effort. The engineering artifact already encodes the thinking. The task is to extract that encoding, represent it in a space where its geometry is meaningful, and make that geometry queryable over time.

### Why the Approach is Non-Obvious

The non-obvious move is to represent both journal entries and the human's own knowledge framework as vectors in the same high-dimensional space, and then to define the axes of that space using the human's own standing documents rather than any universal taxonomy. There is no pre-defined ontology. The coordinate system is personal and evolves as the standing documents evolve.

A second non-obvious move: not all entries contribute equally to the geometry. An entry produced during a period of high "intellectual aliveness" — a term defined precisely in §5 — should exert more gravitational pull on the measurement of what the person is thinking about. This is the role of Soul Speed.

---

## 2. Standing Documents — Personal Epistemology as Geometric Axes

### What They Are

Standing documents are markdown files describing patterns the author has found to be true across many distinct contexts. They are not notes about a specific project or problem. They are claims about how the world works — philosophical, technical, or empirical — that have been validated by recurrence.

Examples from the current corpus: `soul-speed`, `gradient-lossy-functions`, `distributed-patterns`, `physics-as-translation-layer`, `universe-design`, `nonogram-topology`, `two-corpus`, `hx-vs-ux`, `wrong-tools`, `conversational-drift`.

### What Motivates This Abstraction

The insight is that the axes of a personal knowledge space should be defined by the recurring patterns in that person's thinking, not by a universal vocabulary. A standing document encodes one axis of this space. When a journal entry has high semantic similarity to a standing document, it means that entry is operating in the domain of that pattern — the thinking is circling that conceptual territory.

### Slug Stability as an Invariant

Standing documents have stable slugs derived from their filename. A slug is a unique identifier for a standing document's conceptual territory across all versions. Renaming a document creates a new territory in the space, not a version of the old one. This invariant is load-bearing: the geometry is only meaningful if axes are stable over time.

### Versioning

When a standing document's content changes, a new version row is created. The old version is preserved. The current version's content is used for all proximity computations. This allows the conceptual territory to evolve while preserving the historical record.

---

## 3. Semantic Embedding and the High-Dimensional Journal Space

### Embedding Model

All semantic representations use `qwen3-embedding:8b` via Ollama, producing 4096-dimensional float32 vectors. The embedding dimension is fixed by database schema; changing the model requires migrating all vector columns.

Similarity is computed as cosine similarity:

```
cos(a, b) = (a · b) / (|a| × |b|)
          = Σ(aᵢ · bᵢ) / (√Σ(aᵢ²) × √Σ(bᵢ²))
```

returning a value in [0, 1] where 1 is identical and 0 is orthogonal.

### What Gets Embedded

Three classes of objects are embedded:

1. **Journal entries** — the text used for embedding is constructed from extracted engineering concepts and, when available, theoretical territory. This is a deliberate choice: the embedding captures the semantic content of the thinking, not the raw commit text.

2. **Standing documents** — the full markdown content is embedded as a single vector. This provides a document-level similarity measure used for entry-to-standing association at ingest time.

3. **Manifold chunks** — standing document content is also split into semantic chunks (see §4), each independently embedded. These chunk embeddings are not stored; they are computed on demand and used for proximity computation.

### The Journal Space

Each journal entry occupies a position in a high-dimensional Euclidean space. The axes of this space are not standing documents directly — that would require projecting a 4096-dimensional vector onto ~12 axes and discarding most of the information. Instead, the full embedding is retained, and proximity to each standing document's territory is computed geometrically via chunk nearest-neighbor distance (see §4).

The result is a richer representation: an entry's relationship to each conceptual territory is not a single dot product against one document vector, but the minimum distance to any semantic unit within that territory.

---

## 4. Manifolds as Standing Document Territories

### The Manifold Concept

In this system, a **manifold** is the semantic territory of a standing document, represented as a finite collection of chunk embeddings in the 4096-dimensional space. This is a pragmatic approximation of a manifold in the mathematical sense: the chunks constitute a discrete sample of the document's semantic surface, grounded in Euclidean geometry rather than the full machinery of Riemannian manifold theory.

This simplification is intentional. Riemannian manifold geometry in high-dimensional embedding spaces is computationally expensive and requires assumptions about curvature that are not warranted for text. The chunk-based approach provides a tractable surrogate: the manifold is the convex hull (approximately) of its chunk embeddings, and proximity is measured as the distance from an entry to the nearest point on that surface.

### Chunking

Standing document content is split into semantic chunks using double-newline paragraph boundaries. Short fragments (< 50 characters, typically headings) are merged into the following paragraph. Chunks exceeding 2000 characters are split on single-newline boundaries. The resulting chunks are individually embedded.

This chunking strategy preserves the semantic coherence of paragraphs — the natural unit of developed argument in markdown — while preventing single overlong embeddings that would dilute meaning.

### Nearest Chunk Distance

The distance from a journal entry to a manifold is defined as:

```
distance(entry, manifold) = 1 − max{ cos(entry, chunk) | chunk ∈ manifold }
```

A distance of 0 means the entry is semantically identical to some chunk of the standing document. A distance of 1 means the entry has no cosine similarity to any chunk. **Proximity** is the complement:

```
proximity(entry, manifold) = 1 − distance(entry, manifold)
                            = max{ cos(entry, chunk) | chunk ∈ manifold }
```

### Why Chunks Rather Than Document Vectors

A single document vector would represent the average of the document's semantic content. A long, complex standing document covers many ideas, and its averaged vector may be close to nothing in particular. Chunk embeddings preserve the internal differentiation of the document: an entry that strongly resonates with one section of a standing document will have high proximity even if it is unrelated to the rest of the document.

---

## 5. Soul Speed — The Scalar Field

### The Concept

Soul Speed names the pace at which understanding actually forms, as distinct from the pace at which information moves or work gets produced. The insight is that certain kinds of friction are not obstacles to understanding but the mechanism by which understanding occurs. A person who cycles through winter rather than taking a car is choosing the speed at which the city acts on them. A student who navigates dead ends and wrong turns understands differently than one who is routed around them.

The full articulation is in `standing-documents/soul-speed.md`. The conceptual territory is broad, covering physical travel, pedagogy, technical architecture, political economy, and the history of fundamental science. The thread connecting all of them: structured resistance at the appropriate scale is generative; removing it destroys the conditions for genuine understanding.

### As a Geometric Mechanism

Within this system, Soul Speed functions as a **scalar field over the journal space**: it assigns a scalar value to each entry, representing how much "intellectual aliveness" — genuine engagement with ideas at the speed at which they can act on the thinker — is present in that entry's thinking.

The original design intuition: soul-speed is not another territory in the lateral space (not another dimension of "what are you thinking about") but an orthogonal quantity (a measure of "how alive is the thinking"). This led to the architectural decision to treat soul-speed as a modifier rather than a coordinate. It is the dark matter of the space: it does not show up as a gravitational attractor in the lateral profile, but it modifies the effective gravity of every entry.

### Implementation

Soul Speed is operationalized as the proximity of an entry's embedding to the soul-speed standing document's manifold (chunk embeddings). A high value means the entry's semantic content is close to the ideas expressed in the soul-speed document — the entry is engaging with questions of friction, pace of understanding, structured resistance, epistemic value of effort.

As of the current implementation (March 2026), soul-speed is computed via `ManifoldSoulSpeed`: the GLF-weighted mean proximity to the soul-speed manifold chunks across all entries in the window. This replaces an earlier version that used entry-to-document association similarity (a single-vector dot product) rather than manifold geometry.

The scalar is in [0, 1]. Empirical thresholds (subject to calibration):
- ≥ 0.65: high aliveness
- ≥ 0.55: moderate aliveness
- ≥ 0.45: low aliveness
- < 0.45: dormant

These thresholds are starting points. They require calibration against a sufficiently large corpus.

### What Is Novel Here

The use of a living philosophical document (the soul-speed standing document) as the definition of what "intellectual aliveness" means — rather than a mechanically defined proxy such as edit frequency, word count, or commit density — is the distinctive claim. The system measures aliveness by semantic proximity to a carefully articulated description of what aliveness consists of. The measurement is only as good as the standing document, and the standing document is a living artifact that can be refined as the concept develops.

---

## 6. Soul Speed's Influence on Manifold Gravity

### The Mechanism

The core architectural claim: soul-speed does not measure territory (it is not a lateral axis) but it **modulates the effective gravitational weight** of every entry when computing each lateral manifold's attraction.

Formally, when computing the contribution of entry `e` to manifold `m`, the weight is:

```
weight(e) = GLF(age(e)) × (0.5 + 0.5 × soulSpeedProximity(e))
```

where:
- `GLF(age)` is the recency weight (see §10)
- `soulSpeedProximity(e)` is the entry's proximity to the soul-speed manifold, in [0, 1]

The factor `(0.5 + 0.5 × ssProximity)` maps soul-speed proximity from [0, 1] to [0.5, 1.0]. This means:
- An entry with soul-speed proximity 1.0 contributes at full GLF weight
- An entry with soul-speed proximity 0.0 contributes at half GLF weight
- No entry is ever zeroed out — soul-speed is a modifier, not a gate

### The Dark Matter Analogy

The design intuition is that soul-speed operates like a scalar field that deforms the effective gravitational potential of the space without itself being a gravitational source. In the gravitational analogy: lateral standing documents are the visible matter (they attract entries and are attracted by them); soul-speed is the dark matter (it does not appear in the gravity profile but modifies the effective mass of every entry, thus altering the shape of the profile).

In practice this means: if a period of high soul-speed thinking was concentrated around a particular territory (e.g., `distributed-patterns`), that territory's gravitational attraction in the profile will be elevated not only because entries are proximate to it, but because the entries proximate to it were themselves more alive. A period of low-soul-speed thinking around the same territory will produce a weaker gravitational signal.

### What This Enables

This mechanism allows the system to distinguish between two qualitatively different kinds of engagement:
1. Entries that touch a territory because it is formally relevant (high proximity, low soul-speed) — rote application of a known pattern
2. Entries that touch a territory because the thinking is genuinely alive in it (high proximity, high soul-speed) — active development of understanding

Only the second kind exerts full gravitational weight on the profile. The profile therefore preferentially reflects the territory of genuine intellectual movement rather than the territory of routine application.

---

## 7. The Gravity Profile — Detecting Intellectual Drift

### Definition

The **Gravity Profile** (computed as `ManifoldProfile`) is a map from standing document slug to a scalar in [0, 1], representing the GLF+soul-speed-weighted mean proximity of recent entries to each lateral manifold. It is the primary output of trend detection.

Formally:

```
profile[slug] = Σ(proximity(e, manifold_slug) × weight(e)) / Σ(weight(e))
              for e in entries_in_window
```

where `weight(e) = GLF(age(e)) × (0.5 + 0.5 × soulSpeedProximity(e))`.

### What It Represents

The gravity profile is a measurement of intellectual drift: where is the center of gravity of recent thinking, modulated by recency and aliveness? High values (empirically ~0.4–0.8) indicate strong gravitational pull toward that standing document's territory. Low values (< 0.1) indicate the thinking is not currently operating in that domain.

The profile changes over time as new entries are added and older entries decay in weight. This gives it the character of a temporally-smoothed indicator of what the person is actually thinking about, rather than what they have ever thought about.

### Distinction from Simple Association

Entry-to-standing association (computed at ingest, stored in `entry_standing_associations`) is a binary threshold: entries are associated with documents whose cosine similarity exceeds 0.3. This is a document-level similarity.

The gravity profile uses manifold geometry (chunk nearest-neighbor proximity) and is weighted. It is a continuous, recency-sensitive, soul-speed-modulated aggregate — a fundamentally different computation that answers a different question: not "is this entry related to this document" but "how strongly is current thinking gravitating toward this territory."

---

## 8. Unexpected Concepts — Phase Transitions and Emergent Territory

### Definition

Unexpected concepts are engineering concepts extracted from entries that have **low maximum proximity to any lateral manifold**. An entry whose embedding is far from all known standing document territories is an entry whose thinking is in unexplored space — not yet part of the established personal epistemology.

Formally, for each entry `e`:

```
unexpectedness(e) = 1 − max{ proximity(e, m) | m is a lateral manifold }
```

Concepts appearing in high-unexpectedness entries, weighted by GLF and soul-speed, are candidates for new standing document territory — the system's way of detecting when the author's thinking has moved into genuinely new space.

### Why This Matters

The gravity profile tells you where you are. Unexpected concepts tell you where you are going. An entry with high unexpectedness and high soul-speed is a particularly strong signal: it is thinking that is alive and in unmapped territory. These are the entries most likely to eventually generate new standing documents.

### Exclusion from Trending

In the older `UnexpectedConcepts` function (still present, now superseded by `UnexpectedConceptsFromManifold`), concepts that also appeared in the trending list were explicitly excluded from unexpected concepts. The logic: if a concept is both high-frequency and high-spread-distance, it is not truly unexpected — it is an established concept being applied in edge-case situations. Only concepts that appear in distant entries but not in trending are genuinely emergent.

---

## 9. Git Commit Enrichment — Making Engineering Work Epistemically Legible

### The Problem This Solves

Engineering work generates thinking continuously, but that thinking is encoded in a form (code + commit messages) that is not immediately semantically queryable. The commit captures *what changed*; it does not capture *why*, *what theoretical framework justified the approach*, or *what adjacent ideas were activated during the work*.

The enrichment pipeline transforms git commits into epistemically legible journal entries.

### Two-Pass LLM Extraction

**Pass 1 — Engineering Extraction** (`ExtractConcepts`):

Input: commit messages and non-test file diffs for a calendar day.

Output: structured JSON containing:
- `concepts`: engineering concepts, algorithms, data structures, design patterns directly present in the work
- `summary`: one-sentence characterization of the day's engineering focus
- `search_terms`: terms that would retrieve relevant technical literature

This pass produces the surface representation: what the code is doing.

**Pass 2 — Deep Extraction** (`DeepExtract`, activated by `--deep` flag):

Input: the engineering concepts and summary from Pass 1.

Output: structured JSON containing:
- `theoretical_territory`: the underlying theoretical frameworks the engineering draws on
- `adjacent_fields`: related fields of study not directly present in the work
- `arxiv_search_terms`: queries for academic literature
- `research_questions`: open questions surfaced by the work

This pass produces the epistemic representation: what the code *means* in a broader intellectual context. This is the core differentiating step. Pass 1 answers "what did you build." Pass 2 answers "what are you actually thinking about."

### Why Deep Extraction is Central

The embedding that drives all downstream geometry is built preferentially from the `theoretical_territory` output of Pass 2 when it is available. This means the position of an entry in the journal space is determined by its theoretical content — the abstract ideas — rather than its surface engineering content.

An entry about building a distributed message queue will, via Pass 2, produce theoretical territory touching distributed systems theory, consensus mechanisms, fault tolerance models. That entry's embedding will be proximate to standing documents that address those theoretical concerns, even if those documents never mention MQTT or message queues by name.

This is the key mechanism for connecting engineering work to philosophical and theoretical standing documents. Without it, only entries about topics literally mentioned in standing documents would show proximity. With it, entries connect to standing documents through shared theoretical ancestry.

### Per-Day Granularity

One journal entry is produced per calendar day per repository. The `since_timestamp` and `until_timestamp` of the entry are the commit day boundaries (midnight to midnight UTC), not the time of extraction. This means the temporal indexing of entries reflects when the work was done, not when it was processed.

### Freeform Entry Ingestion

In addition to git commit extraction, the system supports freeform markdown entries ingested from WebDAV (e.g., Nextcloud, Joplin notebooks). These go through the same concept extraction pipeline before embedding, with the markdown content as input instead of commit data.

---

## 10. Temporal Recency Weighting — The GLF Mechanism

### The Generalized Logistic Function

All aggregations over a window of entries use GLF-based recency weighting:

```
GLF(age) = 1 / (1 + exp(k × (age − midpoint)))
```

Default parameters: `k = 0.3`, `midpoint = 14` days.

This is a sigmoid function centered at `midpoint` days. Properties:
- At age 0: weight ≈ 1.0
- At age 7 days: weight ≈ 0.93
- At age 14 days: weight = 0.5 (midpoint)
- At age 21 days: weight ≈ 0.07
- At age 28 days: weight ≈ 0.02

The exponential falloff gives sharp recency preference without hard cutoff. Entries beyond ~28 days contribute negligibly without being excluded.

### Why GLF Rather Than Linear Decay or Hard Window

The gravity profile is intended to reflect "what am I thinking about now," not "what have I ever thought about." Linear decay would give excessive weight to slightly old entries. A hard window would create sharp discontinuities when significant entries drop out. GLF provides a smooth, physiologically natural recency curve — the same mathematical form used in the system's broader gradient lossy function philosophy (`gradient-lossy-functions` standing document).

The GLF and Soul Speed are related by more than coincidence. The soul-speed standing document explicitly notes this: "GLF is the technical implementation of Soul Speed. Both are based on the same insight: that structured, proportional loss across scale is not degradation but the mechanism by which a system can operate coherently across multiple time scales simultaneously."

---

## 11. The Brief — From Trend to External Query

### Purpose

The brief is the output-facing mechanism: given the current trend (gravity profile + soul speed), surface one piece of external material (article, paper, reference) that is relevant to what the person is currently thinking about.

### Trend Embeddings as Query Vectors

Rather than sending the gravity profile scalar map to an external system and asking it to interpret the slugs, the brief query includes the actual embedding vectors from which the profile was derived — specifically, the chunk embeddings of the top-N manifolds by proximity score. These are selected by highest L2 norm within each manifold (a proxy for most semantically loaded chunks).

This means the query to Minerva (the external recommendation system) is grounded in the same embedding space as the journal entries themselves. Minerva can perform approximate nearest-neighbor search against its article corpus using the trend embeddings directly, without needing to understand what "distributed-patterns" or "soul-speed" mean as labels.

### Unexpected Embeddings

Separately, the top unexpected concepts (from §8) are individually embedded and included in the query as `unexpected_embeddings`. These allow Minerva to retrieve material at the frontier of current thinking — content that is not in the established territory but is adjacent to what the analysis identifies as emergent.

### Soul Speed as Query Modulator

The soul-speed scalar is included in the Minerva query. The intended semantics: high soul speed signals that the thinker is in an active, generative state and should be surfaced novel or challenging material; low soul speed signals consolidation and should surface synthetic or clarifying material. The actual implementation of this modulation is Minerva's responsibility.

---

## 12. Design Decisions and Roads Not Taken

### MQTT-Native Architecture

Components communicate via MQTT rather than direct function calls, HTTP, or shared state. This is a Soul Speed choice: the latency and friction of message passing are the mechanism that makes the system comprehensible over time. Each component can be understood, replaced, or extended in isolation. The architecture embeds the philosophical framework it measures.

### Ollama on Host, Not in Docker

Embedding is performed by Ollama running on the host, not in Docker. This is a practical constraint arising from GPU access requirements, but it has an architectural implication: all embedding calls must be serialized (via mutex) because concurrent Ollama requests time out. This serialization is explicit in the codebase — it is a constraint, not a choice, but it shapes the design of all embedding-dependent paths.

### Storing Embeddings for Entries, Not for Chunks

Entry embeddings are stored in Postgres (pgvector). Manifold chunk embeddings are computed on demand and not stored. The rationale: entry embeddings are permanent (an entry's semantic representation does not change once computed, modulo re-embedding after model changes). Chunk embeddings change whenever a standing document is updated, which means stored chunk embeddings would require invalidation logic. On-demand computation is simpler and correct.

### Document-Level Association vs. Manifold Proximity

Two distinct similarity computations coexist in the system:

1. **Entry-to-standing association** (stored): computed at ingest as cosine similarity between the entry's 4096-dim embedding and the standing document's 4096-dim embedding. Binary threshold (0.3). Used for visualization and legacy queries.

2. **Manifold proximity** (computed on demand): computed via nearest chunk distance. Continuous [0,1]. Used for trend detection and brief assembly.

The association is cheaper but coarser. The proximity is more expensive but captures intra-document differentiation. Both are retained because association provides a queryable index for navigation (via `space-viz`) while proximity drives the geometric computations.

### Standing Doc Slug as Constant, Not Foreign Key

The soul-speed slug (`"soul-speed"`) is a string constant in the code, matched against standing document slugs at runtime. This is a deliberate coupling: changing the filename of the soul-speed document would break the constant, and the code makes this explicit rather than hiding it in a configuration layer. The fragility is intentional — it makes the dependency visible.

---

## 13. Relationships Between Concepts

The concepts in this system form a dependency chain rather than a flat list:

```
Standing Documents
    │
    ├──► (embedded as) ──► Document Vectors (entry association)
    │
    └──► (chunked and embedded as) ──► Manifold Chunk Collections
                                              │
                              ┌───────────────┼────────────────────┐
                              │               │                    │
                        Nearest Chunk    Soul Speed           Unexpected
                        Distance         Proximity            Concepts
                              │               │                    │
                              └───────────────┼────────────────────┘
                                              │
                                   Weighted by GLF × Soul Speed
                                              │
                                       Gravity Profile
                                              │
                              ┌───────────────┴────────────────────┐
                              │                                    │
                        Trend Detection                    Brief Assembly
                        (MQTT publish)                    (Minerva query)
```

Journal entries enter the system via git commit enrichment (two-pass LLM extraction) or freeform WebDAV ingestion. Their embeddings are the point masses that the manifold geometry acts upon. The gravity profile is the net result. Soul speed is the field that modifies how those point masses are weighted.
