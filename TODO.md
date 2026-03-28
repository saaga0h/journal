# TODO

## What Minerva Gets (minerva/query/brief)

Current state: `brief-assemble` sends `manifold_profile: {slug: proximity}` where proximity
is the GLF+soul-speed-weighted mean proximity [0,1] of recent entries to each standing doc
manifold. Higher = recent work is semantically close to that doc's territory.

The original intent was two distinct signals:

- **Trend**: what recent work has primarily been about — the top manifolds by proximity.
  This is the "fallback" signal when nothing extraordinary is found.
- **Unexpected**: something that genuinely rises up — a positive surprise, not expected
  to be there. Entries that don't strongly belong to any known manifold territory.
  Currently computed as `UnexpectedConceptsFromManifold` (concepts from low-max-proximity
  entries), but not yet surfaced to Minerva separately.

### Questions to resolve before redesigning the Minerva query

- [ ] What does Minerva actually do with the profile? Does it embed the slug names/titles
      and find articles near those embeddings? Or does it use the proximity scores as
      importance weights when ranking candidates?

- [ ] Should "unexpected" bypass Minerva entirely and surface as the brief result directly?
      If an entry cluster doesn't fit any known manifold, Minerva may not have relevant
      articles for it — the surprise *is* the signal, not a query for more content.

- [ ] Should the query distinguish trajectory from snapshot?
      "I've been deep in this manifold for 3 weeks" vs "this manifold just appeared this week"
      carry different meaning. A rising manifold proximity is more actionable than a stable one.
      Consider sending `{slug: {proximity, delta_7d}}` instead of a flat scalar.

- [ ] Is the manifold proximity scalar the right unit for Minerva?
      Minerva presumably works with embeddings. The standing doc content itself (or its
      embedding) may be a better query than a derived scalar — send top-N slug embeddings
      weighted by proximity rather than the scores.

- [ ] How should soul-speed interact with the Minerva query?
      High overall soul-speed = current state is "alive" — Minerva could surface more
      challenging/novel material. Low soul-speed = surface consolidating/reinforcing material.
      Should soul-speed modulate the query strategy, not just the entry weights?

### Current workaround

Both trend and unexpected signals are computed but only `manifold_profile` (trend) is sent
to Minerva. Unexpected concepts are in `TrendResult.UnexpectedConcepts` on
`journal/trend/current` but not used in the brief query.
