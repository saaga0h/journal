# TODO

## What Minerva Gets (minerva/query/brief)

Current state: `brief-assemble` sends a `MinervaQuery` with:
- `manifold_profile`: `{slug: proximity}` — GLF+soul-speed-weighted mean proximity [0,1] per manifold
- `trend_embeddings`: top-N manifold chunk vectors (by proximity rank, highest-norm chunks selected)
- `unexpected_embeddings`: embeddings of top unexpected concept strings (from low-max-proximity entries)
- `soul_speed`: GLF-weighted proximity to soul-speed manifold [0,1]

The original intent was two distinct signals:

- **Trend**: what recent work has primarily been about — the top manifolds by proximity.
  This is the "fallback" signal when nothing extraordinary is found.
- **Unexpected**: something that genuinely rises up — a positive surprise, not expected
  to be there. Entries that don't strongly belong to any known manifold territory.
  Computed as `UnexpectedConceptsFromManifold` and sent as embedded vectors.

### Resolved questions

- [x] **Is the manifold proximity scalar the right unit for Minerva?**
  Resolved: both scalar `manifold_profile` and embedding vectors (`trend_embeddings`) are now sent.
  Minerva can use scalars immediately and upgrade to ANN vector search when its corpus is embedded.

- [x] **Should "unexpected" bypass Minerva entirely?**
  Resolved for now: `unexpected_embeddings` are sent alongside trend embeddings. Minerva decides
  which signal to act on. If Minerva finds no match for unexpected concepts, it can fall back to
  trend. True bypass (surfacing surprise directly without Minerva) is a future option.

- [x] **How should soul-speed interact with the Minerva query?**
  Resolved: `soul_speed` scalar is sent so Minerva can modulate query strategy (high = surface
  novel/challenging material, low = surface consolidating/reinforcing material). Minerva
  implementation is pending.

### Open questions

- [ ] Should the query distinguish trajectory from snapshot?
      "I've been deep in this manifold for 3 weeks" vs "this manifold just appeared this week"
      carry different meaning. A rising manifold proximity is more actionable than a stable one.
      Consider sending `{slug: {proximity, delta_7d}}` instead of a flat scalar.

- [ ] What does Minerva actually do with the profile and vectors?
      Minerva needs to implement: ANN search using `trend_embeddings` and `unexpected_embeddings`,
      `soul_speed`-modulated ranking, and fallback behavior when no strong match is found.
      Currently Minerva times out — expected behavior until implemented.
