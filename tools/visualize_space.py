#!/usr/bin/env python3
"""
Visualize journal entries in the standing document space.

Lateral position (X/Y): PCA over all standing doc similarities except soul-speed.
Vertical position  (Z): soul-speed similarity — aliveness of the thinking.
"""

import argparse
import os
import subprocess

import numpy as np
import matplotlib.pyplot as plt
from mpl_toolkits.mplot3d import Axes3D  # noqa: F401
from sklearn.decomposition import PCA

# ── Arguments ─────────────────────────────────────────────────────────────────
parser = argparse.ArgumentParser(description="Visualize journal entries in standing-doc space")
parser.add_argument("--db", default=None, help="Postgres connection string (or set JOURNAL_DB env)")
args = parser.parse_args()

DB_CONN = args.db or os.environ.get(
    "JOURNAL_DB",
    "postgresql://journal:journal@localhost:5433/journal?sslmode=disable"
)

# ── Fetch data from database ───────────────────────────────────────────────────
def fetch_data(conn_str):
    """Fetch entry-standing associations from the database.

    Tries psql directly first; falls back to docker exec journal_postgres psql
    if psql is not on PATH (common on macOS where Postgres runs in Docker).
    """
    query = (
        "SELECT je.id, je.repository, esa.standing_slug, esa.similarity "
        "FROM entry_standing_associations esa "
        "JOIN journal_entries je ON je.id = esa.entry_id "
        "ORDER BY je.id, esa.standing_slug"
    )

    # Try host psql first
    import shutil
    if shutil.which("psql"):
        cmd = ["psql", conn_str, "-t", "-A", "-F", ",", "-c", query]
    else:
        # Fall back to Docker
        cmd = ["docker", "exec", "journal_postgres",
               "psql", "-U", "journal", "-d", "journal",
               "-t", "-A", "-F", ",", "-c", query]

    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0:
        print(f"psql error: {result.stderr}")
        raise SystemExit(1)

    rows = []
    for line in result.stdout.strip().split("\n"):
        if not line:
            continue
        parts = line.split(",")
        rows.append((int(parts[0]), parts[1], parts[2], float(parts[3])))
    return rows

RAW = fetch_data(DB_CONN)

# Derive lateral docs dynamically — all slugs except soul-speed, sorted
SOUL_SPEED_SLUG = "soul-speed"
all_slugs = sorted({row[2] for row in RAW if row[2] != SOUL_SPEED_SLUG})
LATERAL_DOCS = all_slugs

# ── Build feature matrix ───────────────────────────────────────────────────────
entries = {}  # id -> {slug: sim, ...}
repos   = {}  # id -> repo name

for eid, repo, slug, sim in RAW:
    entries.setdefault(eid, {})[slug] = sim
    repos[eid] = repo

entry_ids = sorted(entries.keys())

# Lateral matrix: rows = entries, cols = lateral standing docs
X = np.array([[entries[eid].get(doc, 0.0) for doc in LATERAL_DOCS] for eid in entry_ids])

# Z axis: soul-speed similarity
z = np.array([entries[eid].get("soul-speed", 0.0) for eid in entry_ids])

# Reduce lateral dims to 2 via PCA
pca = PCA(n_components=2)
xy  = pca.fit_transform(X)

print(f"PCA explained variance: {pca.explained_variance_ratio_}")
print(f"PC1 top loadings: {list(zip(LATERAL_DOCS, pca.components_[0].round(3)))}")
print(f"PC2 top loadings: {list(zip(LATERAL_DOCS, pca.components_[1].round(3)))}")

# ── Plot ───────────────────────────────────────────────────────────────────────
repo_names  = [repos[eid] for eid in entry_ids]
unique_repos = sorted(set(repo_names))
colors = plt.cm.tab10(np.linspace(0, 1, len(unique_repos)))
color_map = dict(zip(unique_repos, colors))

fig = plt.figure(figsize=(11, 8))
ax  = fig.add_subplot(111, projection="3d")

for eid, (x, y), zv, repo in zip(entry_ids, xy, z, repo_names):
    c = color_map[repo]
    ax.scatter(x, y, zv, color=c, s=120, depthshade=True)
    ax.text(x, y, zv + 0.004, f"{repo}\n#{eid}", fontsize=7, ha="center")

# Legend
for repo, c in color_map.items():
    ax.scatter([], [], [], color=c, label=repo, s=60)
ax.legend(loc="upper left", fontsize=8)

# Axis labels — annotate PC1/PC2 with dominant loading
def dominant(loadings, docs):
    idx = np.argmax(np.abs(loadings))
    return docs[idx]

pc1_label = dominant(pca.components_[0], LATERAL_DOCS)
pc2_label = dominant(pca.components_[1], LATERAL_DOCS)

ax.set_xlabel(f"PC1 (∝ {pc1_label})", fontsize=9, labelpad=8)
ax.set_ylabel(f"PC2 (∝ {pc2_label})", fontsize=9, labelpad=8)
ax.set_zlabel("Soul Speed", fontsize=9, labelpad=8)
ax.set_title("Journal entries in standing document space", fontsize=11, pad=12)

plt.tight_layout()
plt.savefig("tools/journal_space.png", dpi=150)
print("Saved tools/journal_space.png")
plt.show()
