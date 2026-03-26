// Journal Space Viz
// X/Z: UMAP projection of lateral standing-doc association coords
// Y:   SinceTimestamp normalized to [0,1] (older=bottom, newer=top)
// Color: soul-speed similarity (blue=low, red=high) or source repo toggle
// Focus: select a standing doc to fade out unrelated entries (fog-by-similarity)

const SOUL_SPEED_SLUG = 'soul-speed'; // must match Go SoulSpeedSlug = "soul-speed"
const UMAP_SEED = 42;
const BG_COLOR = new THREE.Color(0x0a0a0f); // must match scene.background

const state = {
  points:       [],    // EntrySpacePoint[] from /api/points (PascalCase fields)
  standing:     [],    // {slug, title}[] from /api/standing
  lateralSlugs: [],    // all slugs except soul-speed, sorted
  positions:    [],    // [{x, y, z}] Three.js coords after UMAP + time mapping
  colorMode:    'soul-speed', // 'soul-speed' | 'source'
  focusSlug:    null,  // standing doc slug currently in focus, or null
  days:         90,
};

// ── Distinct source palette ───────────────────────────────────────────────────
const SOURCE_PALETTE = [
  0x4fc3f7, // sky blue
  0xf06292, // rose
  0x81c784, // sage green
  0xffb74d, // amber
  0xce93d8, // lavender
  0x4dd0e1, // cyan
  0xff8a65, // coral
  0xa5d6a7, // mint
  0xf48fb1, // pink
  0x90caf9, // periwinkle
];

let sourceColorMap = {};

function buildSourceColorMap() {
  const sources = [...new Set(state.points.map(p => p.Source))].sort((a, b) => a.localeCompare(b));
  sourceColorMap = {};
  sources.forEach((src, i) => {
    sourceColorMap[src] = new THREE.Color(SOURCE_PALETTE[i % SOURCE_PALETTE.length]);
  });
}

// ── Data fetching ─────────────────────────────────────────────────────────────

async function fetchData(days) {
  const [pRes, sRes] = await Promise.all([
    fetch(`/api/points?days=${days}`),
    fetch('/api/standing'),
  ]);
  state.points   = await pRes.json();
  state.standing = await sRes.json();

  const allSlugs = state.standing.map(s => s.slug).sort((a, b) => a.localeCompare(b));
  state.lateralSlugs = allSlugs.filter(s => s !== SOUL_SPEED_SLUG);
}

// ── Seeded PRNG (Mulberry32) ──────────────────────────────────────────────────

function seededRandom(seed) {
  let s = seed;
  return function() {
    s = Math.trunc(s + 0x6D2B79F5);
    let t = Math.imul(s ^ s >>> 15, 1 | s);
    t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t;
    return ((t ^ t >>> 14) >>> 0) / 4294967296;
  };
}

// ── UMAP projection ───────────────────────────────────────────────────────────

function buildMatrix() {
  return state.points.map(pt =>
    state.lateralSlugs.map(slug => pt.Coords?.[slug] ?? 0)
  );
}

function runUMAP(matrix) {
  const n = matrix.length;
  const umap = new UMAP({
    nComponents: 2,
    nNeighbors: Math.min(15, Math.max(2, n - 1)),
    minDist: 0.1,
    random: seededRandom(UMAP_SEED),
  });
  return umap.fit(matrix);
}

// ── Position computation ──────────────────────────────────────────────────────

function computePositions(umapResult) {
  const times = state.points.map(pt => new Date(pt.SinceTimestamp).getTime());
  const tMin = Math.min(...times);
  const tMax = Math.max(...times);
  const tRange = tMax - tMin || 1;

  let xMin = Infinity, xMax = -Infinity, yMin = Infinity, yMax = -Infinity;
  for (const [x, y] of umapResult) {
    if (x < xMin) { xMin = x; }
    if (x > xMax) { xMax = x; }
    if (y < yMin) { yMin = y; }
    if (y > yMax) { yMax = y; }
  }
  const xRange = xMax - xMin || 1;
  const yRange = yMax - yMin || 1;
  const scale = 10;

  state.positions = umapResult.map(([ux, uy], i) => ({
    x: ((ux - xMin) / xRange - 0.5) * scale,
    y: ((times[i] - tMin) / tRange) * scale,
    z: ((uy - yMin) / yRange - 0.5) * scale,
  }));
}

// ── Color functions ───────────────────────────────────────────────────────────

function soulSpeedColor(pt) {
  const ss = pt.Coords?.[SOUL_SPEED_SLUG] ?? 0;
  const hue = Math.round(240 * (1 - ss));
  return new THREE.Color(`hsl(${hue}, 80%, 55%)`);
}

function sourceColor(pt) {
  return sourceColorMap[pt.Source] || new THREE.Color(0xffffff);
}

function getColor(pt) {
  return state.colorMode === 'soul-speed' ? soulSpeedColor(pt) : sourceColor(pt);
}

// ── Focus fog ────────────────────────────────────────────────────────────────
// When a doc is focused, lerp each point's color toward the background based
// on how weakly it associates with that doc. High similarity = full color.
// Low similarity = fades toward near-black (background).
// fog strength: 0 = full color, 1 = fully faded to background

const FOG_MIN_ALPHA = 0.06; // dimmest a faded point gets (not fully invisible)

function applyFog(baseColor, pt) {
  if (!state.focusSlug) { return baseColor; }
  const sim = pt.Coords?.[state.focusSlug] ?? 0;
  // Remap sim to fog: sim=1 → no fog, sim=0 → max fog
  const visible = Math.max(FOG_MIN_ALPHA, sim);
  return new THREE.Color().lerpColors(BG_COLOR, baseColor, visible);
}

// ── Three.js scene ────────────────────────────────────────────────────────────

let activeTab = 'space'; // 'space' | 'manifold'

// ── Manifold state ──────────────────────────────────────────────────────────

const MANIFOLD_HULL_COLOR = 0x4fc3f7;
const MANIFOLD_CHUNK_COLOR = 0x6ad0f7; // desaturated surface blue — blends with hull

const mf = {
  slug:       null,
  title:      '',
  days:       90,
  chunks:     [],   // {index, text, position: {x,y,z}, embedding}
  entries:    [],   // {entry_id, source, since_timestamp, concepts, position: {x,y,z}}
  hullMesh:   null,
  wireframe:  null,
  entryCloud: null,
  chunkCloud: null,
};

function clearManifoldScene() {
  [mf.hullMesh, mf.wireframe, mf.entryCloud, mf.chunkCloud].forEach(obj => {
    if (obj) scene.remove(obj);
  });
  mf.hullMesh = mf.wireframe = mf.entryCloud = mf.chunkCloud = null;
}

// ── Manifold data fetch + UMAP 3D ──────────────────────────────────────────

async function computeManifold(slug, days) {
  const statusEl = document.getElementById('manifold-status');
  statusEl.textContent = 'Embedding chunks...';

  const res = await fetch(`/api/manifold?slug=${encodeURIComponent(slug)}&days=${days}`);
  if (!res.ok) {
    statusEl.textContent = 'Error: ' + (await res.text());
    return;
  }
  const data = await res.json();
  mf.slug = data.slug;
  mf.title = data.title;

  const nChunks = data.chunks.length;
  const nEntries = data.entries.length;
  const total = nChunks + nEntries;

  if (total < 3) {
    statusEl.textContent = `Not enough data (${nChunks} chunks + ${nEntries} entries, need >= 3)`;
    return;
  }

  statusEl.textContent = `Projecting ${nChunks} chunks + ${nEntries} entries...`;

  // Build combined embedding matrix: chunks first, then entries
  const allEmbeddings = [
    ...data.chunks.map(c => c.embedding),
    ...data.entries.map(e => e.embedding),
  ];

  // UMAP into 3D (raw embedding space, not association space)
  const umap = new UMAP({
    nComponents: 3,
    nNeighbors: Math.min(15, Math.max(2, total - 1)),
    minDist: 0.1,
    random: seededRandom(UMAP_SEED),
  });
  const projected = umap.fit(allEmbeddings);

  // Normalize all 3 axes to scene scale
  let mins = [Infinity, Infinity, Infinity];
  let maxs = [-Infinity, -Infinity, -Infinity];
  for (const pt of projected) {
    for (let d = 0; d < 3; d++) {
      if (pt[d] < mins[d]) mins[d] = pt[d];
      if (pt[d] > maxs[d]) maxs[d] = pt[d];
    }
  }
  const ranges = mins.map((mn, d) => maxs[d] - mn || 1);
  const scale = 10;

  function normalize(pt) {
    return {
      x: ((pt[0] - mins[0]) / ranges[0] - 0.5) * scale,
      y: ((pt[1] - mins[1]) / ranges[1] - 0.5) * scale,
      z: ((pt[2] - mins[2]) / ranges[2] - 0.5) * scale,
    };
  }

  mf.chunks = data.chunks.map((c, i) => ({
    ...c,
    position: normalize(projected[i]),
  }));

  mf.entries = data.entries.map((e, i) => ({
    ...e,
    position: normalize(projected[nChunks + i]),
  }));

  renderManifold();
}

// ── Manifold rendering ─────────────────────────────────────────────────────
// Proximity triangulation: connect chunk points that are nearby in projected
// space, forming a surface that reveals manifold topology (islands, bridges,
// holes) rather than one convex blob wrapping everything.

function buildIslandHulls(positions) {
  // Cluster chunk positions into islands using alpha radius (median NN * 3.5),
  // then compute a separate convex hull per island. This produces clean,
  // non-intersecting surface faces — no interior cross-hatching.
  const n = positions.length;
  if (n < 3) return { islands: [], alpha: 0 };

  // Pairwise distances
  const dist = [];
  for (let i = 0; i < n; i++) {
    dist[i] = new Float32Array(n);
    const pi = positions[i];
    for (let j = 0; j < n; j++) {
      const pj = positions[j];
      const dx = pi.x - pj.x, dy = pi.y - pj.y, dz = pi.z - pj.z;
      dist[i][j] = Math.sqrt(dx*dx + dy*dy + dz*dz);
    }
  }

  // Alpha from median nearest-neighbor distance
  const nnDists = [];
  for (let i = 0; i < n; i++) {
    let minD = Infinity;
    for (let j = 0; j < n; j++) {
      if (i !== j && dist[i][j] < minD) minD = dist[i][j];
    }
    nnDists.push(minD);
  }
  nnDists.sort((a, b) => a - b);
  const median = nnDists[Math.floor(n / 2)];
  const alpha = median * 3.5;

  // Union-find to cluster into islands
  const parent = Array.from({length: n}, (_, i) => i);
  function find(x) { while (parent[x] !== x) { parent[x] = parent[parent[x]]; x = parent[x]; } return x; }
  for (let i = 0; i < n; i++) {
    for (let j = i + 1; j < n; j++) {
      if (dist[i][j] <= alpha) parent[find(i)] = find(j);
    }
  }

  // Group points by island
  const groups = {};
  for (let i = 0; i < n; i++) {
    const root = find(i);
    if (!groups[root]) groups[root] = [];
    groups[root].push(i);
  }

  // Build convex hull per island (via ConvexGeometry which produces clean faces)
  const islands = [];
  for (const root of Object.keys(groups)) {
    const members = groups[root];
    const vecs = members.map(i =>
      new THREE.Vector3(positions[i].x, positions[i].y, positions[i].z)
    );
    islands.push({ indices: members, vectors: vecs });
  }

  console.log('Manifold: alpha=' + alpha.toFixed(2), 'median NN=' + median.toFixed(2),
    'chunks=' + n, 'islands=' + islands.length,
    'sizes=[' + islands.map(g => g.indices.length).join(',') + ']');

  return { islands, alpha };
}

function renderManifold() {
  clearManifoldScene();

  const chunkPositions = mf.chunks.map(c => c.position);
  const result = buildIslandHulls(chunkPositions);

  const statusEl = document.getElementById('manifold-status');
  const islandLabel = result.islands.length === 1 ? '1 island' : result.islands.length + ' islands';

  // Render a convex hull per island — clean non-intersecting faces
  const hullGroup = new THREE.Group();
  let totalFaces = 0;

  for (const island of result.islands) {
    if (island.vectors.length >= 4) {
      // Convex hull → face indices, then build geometry manually
      try {
        const hull = THREE.computeConvexHull(island.vectors);
        if (hull.faces.length === 0) {
          console.warn('Hull produced 0 faces for island of', island.vectors.length);
          continue;
        }

        const verts = new Float32Array(hull.faces.length * 9);
        let vi = 0;
        for (const face of hull.faces) {
          for (const idx of face) {
            const p = hull.vertices[idx];
            verts[vi++] = p.x; verts[vi++] = p.y; verts[vi++] = p.z;
          }
        }

        const geo = new THREE.BufferGeometry();
        geo.setAttribute('position', new THREE.BufferAttribute(verts, 3));
        geo.computeVertexNormals();

        hullGroup.add(new THREE.Mesh(geo, new THREE.MeshBasicMaterial({
          color: MANIFOLD_HULL_COLOR,
          transparent: true,
          opacity: 0.07,
          side: THREE.DoubleSide,
          depthWrite: false,
        })));

        const wireGeo = new THREE.WireframeGeometry(geo);
        hullGroup.add(new THREE.LineSegments(wireGeo, new THREE.LineBasicMaterial({
          color: MANIFOLD_HULL_COLOR, opacity: 0.3, transparent: true,
        })));

        totalFaces += hull.faces.length;
      } catch (e) {
        console.warn('Hull failed for island of', island.vectors.length, 'points:', e);
      }
    } else if (island.vectors.length === 3) {
      // Triangle from 3 points
      const v = island.vectors;
      const pos = new Float32Array([
        v[0].x, v[0].y, v[0].z, v[1].x, v[1].y, v[1].z, v[2].x, v[2].y, v[2].z,
      ]);
      const geo = new THREE.BufferGeometry();
      geo.setAttribute('position', new THREE.BufferAttribute(pos, 3));
      geo.computeVertexNormals();
      hullGroup.add(new THREE.Mesh(geo, new THREE.MeshBasicMaterial({
        color: MANIFOLD_HULL_COLOR, transparent: true, opacity: 0.07,
        side: THREE.DoubleSide, depthWrite: false,
      })));
      totalFaces += 1;
    } else if (island.vectors.length === 2) {
      // Edge between 2 points
      const v = island.vectors;
      const pos = new Float32Array([v[0].x, v[0].y, v[0].z, v[1].x, v[1].y, v[1].z]);
      const geo = new THREE.BufferGeometry();
      geo.setAttribute('position', new THREE.BufferAttribute(pos, 3));
      hullGroup.add(new THREE.LineSegments(geo, new THREE.LineBasicMaterial({
        color: MANIFOLD_HULL_COLOR, opacity: 0.4, transparent: true,
      })));
    }
    // Single-point islands are just rendered as chunk dots (below)
  }

  if (hullGroup.children.length > 0) {
    mf.hullMesh = hullGroup;
    scene.add(hullGroup);
  }

  statusEl.textContent = `${mf.title}: ${mf.chunks.length} chunks, ${mf.entries.length} entries — ${totalFaces} faces, ${islandLabel}`;

  // Chunk dots (larger, orange)
  {
    const n = mf.chunks.length;
    const pos = new Float32Array(n * 3);
    const col = new Float32Array(n * 3);
    const chunkColor = new THREE.Color(MANIFOLD_CHUNK_COLOR);
    for (let i = 0; i < n; i++) {
      const p = mf.chunks[i].position;
      pos[i*3] = p.x; pos[i*3+1] = p.y; pos[i*3+2] = p.z;
      col[i*3] = chunkColor.r; col[i*3+1] = chunkColor.g; col[i*3+2] = chunkColor.b;
    }
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.BufferAttribute(pos, 3));
    geo.setAttribute('color', new THREE.BufferAttribute(col, 3));
    mf.chunkCloud = new THREE.Points(geo,
      new THREE.PointsMaterial({ size: 0.2, vertexColors: true, sizeAttenuation: true,
        transparent: true, opacity: 0.4 }));
    scene.add(mf.chunkCloud);
  }

  // Entry dots (smaller, tinted by source)
  {
    const n = mf.entries.length;
    const pos = new Float32Array(n * 3);
    const col = new Float32Array(n * 3);
    const sources = [...new Set(mf.entries.map(e => e.source))].sort((a, b) => a.localeCompare(b));
    const srcMap = {};
    sources.forEach((s, i) => { srcMap[s] = new THREE.Color(SOURCE_PALETTE[i % SOURCE_PALETTE.length]); });

    for (let i = 0; i < n; i++) {
      const p = mf.entries[i].position;
      const c = srcMap[mf.entries[i].source] || new THREE.Color(0xffffff);
      pos[i*3] = p.x; pos[i*3+1] = p.y; pos[i*3+2] = p.z;
      col[i*3] = c.r; col[i*3+1] = c.g; col[i*3+2] = c.b;
    }
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.BufferAttribute(pos, 3));
    geo.setAttribute('color', new THREE.BufferAttribute(col, 3));
    mf.entryCloud = new THREE.Points(geo,
      new THREE.PointsMaterial({ size: 0.18, vertexColors: true, sizeAttenuation: true }));
    scene.add(mf.entryCloud);
  }
}

// ── Tab switching ───────────────────────────────────────────────────────────

function switchTab(tab) {
  activeTab = tab;

  // Update tab bar
  document.querySelectorAll('#tab-bar button').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.tab === tab);
  });

  // Toggle control panels
  document.getElementById('space-controls').style.display = tab === 'space' ? '' : 'none';
  document.getElementById('manifold-controls').style.display = tab === 'manifold' ? '' : 'none';

  // Toggle scene objects
  if (tab === 'space') {
    clearManifoldScene();
    if (pointCloud) scene.add(pointCloud);
  } else {
    if (pointCloud) scene.remove(pointCloud);
    if (mf.hullMesh)   scene.add(mf.hullMesh);
    if (mf.wireframe)  scene.add(mf.wireframe);
    if (mf.entryCloud) scene.add(mf.entryCloud);
    if (mf.chunkCloud) scene.add(mf.chunkCloud);
  }
}

let scene, camera, renderer, controls, pointCloud, raycaster, mouse;

function initScene() {
  const canvas = document.getElementById('scene');
  renderer = new THREE.WebGLRenderer({ canvas, antialias: true });
  renderer.setSize(window.innerWidth, window.innerHeight);
  renderer.setPixelRatio(window.devicePixelRatio);

  scene = new THREE.Scene();
  scene.background = BG_COLOR.clone();

  camera = new THREE.PerspectiveCamera(60, window.innerWidth / window.innerHeight, 0.1, 1000);
  camera.position.set(0, 22, 2);

  controls = new THREE.OrbitControls(camera, renderer.domElement);
  controls.enableDamping = true;
  controls.dampingFactor = 0.05;
  controls.target.set(0, 5, 0);

  raycaster = new THREE.Raycaster();
  raycaster.params.Points = { threshold: 0.25 };
  mouse = new THREE.Vector2();

  addAxisArrows();

  window.addEventListener('resize', onResize);
  renderer.domElement.addEventListener('mousemove', onMouseMove);
}

// ── Point cloud ───────────────────────────────────────────────────────────────

function renderPoints() {
  if (pointCloud) { scene.remove(pointCloud); }

  const n = state.positions.length;
  const positions = new Float32Array(n * 3);
  const colors    = new Float32Array(n * 3);

  for (let i = 0; i < n; i++) {
    const p = state.positions[i];
    positions[i * 3]     = p.x;
    positions[i * 3 + 1] = p.y;
    positions[i * 3 + 2] = p.z;

    const c = applyFog(getColor(state.points[i]), state.points[i]);
    colors[i * 3]     = c.r;
    colors[i * 3 + 1] = c.g;
    colors[i * 3 + 2] = c.b;
  }

  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
  geo.setAttribute('color',    new THREE.BufferAttribute(colors, 3));

  const mat = new THREE.PointsMaterial({ size: 0.25, vertexColors: true, sizeAttenuation: true });
  pointCloud = new THREE.Points(geo, mat);
  scene.add(pointCloud);
}

// ── Axis arrows and labels ────────────────────────────────────────────────────

function addAxisArrows() {
  const len = 6, hLen = 0.4, hW = 0.15, col = 0x444444;
  scene.add(new THREE.ArrowHelper(new THREE.Vector3(1,0,0), new THREE.Vector3(-5,0,0), len,     col, hLen, hW));
  scene.add(new THREE.ArrowHelper(new THREE.Vector3(0,1,0), new THREE.Vector3(0,0,0),  len*1.5, col, hLen, hW));
  scene.add(new THREE.ArrowHelper(new THREE.Vector3(0,0,1), new THREE.Vector3(0,0,-5), len,     col, hLen, hW));
  addLabel('UMAP-1', 2,        -0.8,        0);
  addLabel('Time ↑', -0.8,     len*1.5+0.5, 0);
  addLabel('UMAP-2', 0,        -0.8,        2);
}

function addLabel(text, x, y, z) {
  const c = document.createElement('canvas');
  c.width = 256; c.height = 64;
  const ctx = c.getContext('2d');
  ctx.fillStyle = '#555';
  ctx.font = '24px monospace';
  ctx.fillText(text, 4, 40);
  const sprite = new THREE.Sprite(new THREE.SpriteMaterial({
    map: new THREE.CanvasTexture(c), transparent: true,
  }));
  sprite.position.set(x, y, z);
  sprite.scale.set(2, 0.5, 1);
  scene.add(sprite);
}

// ── Focus dropdown ────────────────────────────────────────────────────────────

function populateFocusDropdown() {
  const sel = document.getElementById('focus-select');
  // Clear existing options beyond the first (— none —)
  while (sel.options.length > 1) { sel.remove(1); }

  state.lateralSlugs.forEach(slug => {
    const opt = document.createElement('option');
    opt.value = slug;
    opt.textContent = slug;
    sel.appendChild(opt);
  });
}

// ── Tooltip via raycasting ────────────────────────────────────────────────────

function onMouseMove(event) {
  mouse.x =  (event.clientX / window.innerWidth)  * 2 - 1;
  mouse.y = -(event.clientY / window.innerHeight) * 2 + 1;

  raycaster.setFromCamera(mouse, camera);

  const tip = document.getElementById('tooltip');

  if (activeTab === 'manifold') {
    // Manifold tooltip: check chunk cloud first, then entry cloud
    if (mf.chunkCloud) {
      const hits = raycaster.intersectObject(mf.chunkCloud);
      if (hits.length > 0) {
        const chunk = mf.chunks[hits[0].index];
        tip.querySelector('.source').textContent   = 'chunk ' + chunk.index;
        tip.querySelector('.date').textContent     = mf.title;
        tip.querySelector('.concepts').textContent = chunk.text;
        tip.style.display = 'block';
        tip.style.left = (event.clientX + 14) + 'px';
        tip.style.top  = (event.clientY + 14) + 'px';
        return;
      }
    }
    if (mf.entryCloud) {
      const hits = raycaster.intersectObject(mf.entryCloud);
      if (hits.length > 0) {
        const entry = mf.entries[hits[0].index];
        const date = entry.since_timestamp ? entry.since_timestamp.substring(0, 10) : '—';
        const concepts = (entry.concepts || []).slice(0, 3).join(', ') || '(no concepts)';
        tip.querySelector('.source').textContent   = entry.source;
        tip.querySelector('.date').textContent     = date;
        tip.querySelector('.concepts').textContent = concepts;
        tip.style.display = 'block';
        tip.style.left = (event.clientX + 14) + 'px';
        tip.style.top  = (event.clientY + 14) + 'px';
        return;
      }
    }
    tip.style.display = 'none';
    return;
  }

  // Space tab tooltip
  if (!pointCloud) { tip.style.display = 'none'; return; }

  const hits = raycaster.intersectObject(pointCloud);
  if (hits.length > 0) {
    const pt       = state.points[hits[0].index];
    const date     = pt.SinceTimestamp ? pt.SinceTimestamp.substring(0, 10) : '—';
    const concepts = (pt.Concepts || []).slice(0, 3).join(', ') || '(no concepts)';

    // Show similarity to focused doc if one is selected
    let focusLine = '';
    if (state.focusSlug) {
      const sim = (pt.Coords?.[state.focusSlug] ?? 0).toFixed(3);
      focusLine = `${state.focusSlug}: ${sim}`;
    }

    tip.querySelector('.source').textContent   = pt.Source;
    tip.querySelector('.date').textContent     = date;
    tip.querySelector('.concepts').textContent = focusLine ? `${focusLine}\n${concepts}` : concepts;
    tip.style.display = 'block';
    tip.style.left    = (event.clientX + 14) + 'px';
    tip.style.top     = (event.clientY + 14) + 'px';
  } else {
    tip.style.display = 'none';
  }
}

// ── Controls wiring ───────────────────────────────────────────────────────────

function initControls() {
  // ── Space tab controls ──
  const slider    = document.getElementById('days-slider');
  const daysValue = document.getElementById('days-value');

  slider.addEventListener('input', () => {
    state.days = Number.parseInt(slider.value);
    daysValue.textContent = slider.value;
  });
  slider.addEventListener('change', () => reloadAndRender());

  document.getElementById('btn-recompute').addEventListener('click', () => reloadAndRender());

  document.getElementById('btn-color-toggle').addEventListener('click', () => {
    state.colorMode = state.colorMode === 'soul-speed' ? 'source' : 'soul-speed';
    document.getElementById('btn-color-toggle').textContent = 'Color: ' + state.colorMode;
    buildSourceColorMap();
    renderPoints();
  });

  document.getElementById('focus-select').addEventListener('change', (e) => {
    state.focusSlug = e.target.value || null;
    renderPoints(); // re-color only, no UMAP recompute
  });

  // ── Tab bar ──
  document.querySelectorAll('#tab-bar button').forEach(btn => {
    btn.addEventListener('click', () => switchTab(btn.dataset.tab));
  });

  // ── Manifold tab controls ──
  const mfSlider = document.getElementById('mf-days-slider');
  const mfDaysValue = document.getElementById('mf-days-value');

  mfSlider.addEventListener('input', () => {
    mf.days = Number.parseInt(mfSlider.value);
    mfDaysValue.textContent = mfSlider.value;
  });

  document.getElementById('btn-compute-manifold').addEventListener('click', () => {
    const slug = document.getElementById('manifold-select').value;
    if (!slug) {
      document.getElementById('manifold-status').textContent = 'Select a document first';
      return;
    }
    computeManifold(slug, mf.days);
  });
}

function populateManifoldDropdown() {
  const sel = document.getElementById('manifold-select');
  while (sel.options.length > 1) { sel.remove(1); }

  state.lateralSlugs.forEach(slug => {
    const doc = state.standing.find(s => s.slug === slug);
    const opt = document.createElement('option');
    opt.value = slug;
    opt.textContent = doc ? doc.title : slug;
    sel.appendChild(opt);
  });
}

// ── Reload + render ───────────────────────────────────────────────────────────

async function reloadAndRender() {
  const loading = document.getElementById('loading');
  loading.textContent = 'Loading...';
  loading.style.display = 'block';

  await fetchData(state.days);

  if (state.points.length < 3) {
    loading.textContent = 'Not enough data (need ≥ 3 entries with associations)';
    return;
  }

  buildSourceColorMap();
  populateFocusDropdown();
  populateManifoldDropdown();
  const matrix     = buildMatrix();
  const umapResult = runUMAP(matrix);
  computePositions(umapResult);
  renderPoints();
  loading.style.display = 'none';
}

// ── Resize + animation loop ───────────────────────────────────────────────────

function onResize() {
  camera.aspect = window.innerWidth / window.innerHeight;
  camera.updateProjectionMatrix();
  renderer.setSize(window.innerWidth, window.innerHeight);
}

function animate() {
  requestAnimationFrame(animate);
  controls.update();
  renderer.render(scene, camera);
}

// ── Entry point ───────────────────────────────────────────────────────────────

async function init() {
  initScene();
  initControls();
  animate();
  await reloadAndRender();
}

init();
