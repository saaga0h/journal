// Journal Space Viz
// X/Z: UMAP projection of lateral standing-doc association coords
// Y:   SinceTimestamp normalized to [0,1] (older=bottom, newer=top)
// Color: soul-speed similarity (blue=low, red=high) or source repo toggle

const SOUL_SPEED_SLUG = 'soul-speed'; // must match Go SoulSpeedSlug = "soul-speed"
const UMAP_SEED = 42;

const state = {
  points:       [],    // EntrySpacePoint[] from /api/points (PascalCase fields)
  standing:     [],    // {slug, title}[] from /api/standing
  lateralSlugs: [],    // all slugs except soul-speed, sorted
  positions:    [],    // [{x, y, z}] Three.js coords after UMAP + time mapping
  colorMode:    'soul-speed', // 'soul-speed' | 'source'
  days:         90,
};

// ── Data fetching ─────────────────────────────────────────────────────────────

async function fetchData(days) {
  const [pRes, sRes] = await Promise.all([
    fetch(`/api/points?days=${days}`),
    fetch('/api/standing'),
  ]);
  state.points   = await pRes.json();
  state.standing = await sRes.json();

  const allSlugs = state.standing.map(s => s.slug).sort();
  state.lateralSlugs = allSlugs.filter(s => s !== SOUL_SPEED_SLUG);
}

// ── Seeded PRNG (Mulberry32) ──────────────────────────────────────────────────

function seededRandom(seed) {
  let s = seed;
  return function() {
    s |= 0; s = s + 0x6D2B79F5 | 0;
    let t = Math.imul(s ^ s >>> 15, 1 | s);
    t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t;
    return ((t ^ t >>> 14) >>> 0) / 4294967296;
  };
}

// ── UMAP projection ───────────────────────────────────────────────────────────

function buildMatrix() {
  return state.points.map(pt =>
    state.lateralSlugs.map(slug =>
      (pt.Coords && pt.Coords[slug]) ? pt.Coords[slug] : 0.0
    )
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
  return umap.fit(matrix); // returns number[][]
}

// ── Position computation ──────────────────────────────────────────────────────
// UMAP-1 → Three.js X
// Time   → Three.js Y (up, Y-up convention)
// UMAP-2 → Three.js Z

function computePositions(umapResult) {
  const times = state.points.map(pt => new Date(pt.SinceTimestamp).getTime());
  const tMin = Math.min(...times);
  const tMax = Math.max(...times);
  const tRange = tMax - tMin || 1;

  let xMin = Infinity, xMax = -Infinity, yMin = Infinity, yMax = -Infinity;
  for (const [x, y] of umapResult) {
    if (x < xMin) xMin = x; if (x > xMax) xMax = x;
    if (y < yMin) yMin = y; if (y > yMax) yMax = y;
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
  const ss = (pt.Coords && pt.Coords[SOUL_SPEED_SLUG]) ? pt.Coords[SOUL_SPEED_SLUG] : 0;
  const hue = Math.round(240 * (1 - ss)); // blue (240) = low, red (0) = high
  return new THREE.Color(`hsl(${hue}, 80%, 55%)`);
}

function sourceColor(pt) {
  let hash = 0;
  for (let i = 0; i < pt.Source.length; i++) {
    hash = ((hash << 5) - hash) + pt.Source.charCodeAt(i);
    hash |= 0;
  }
  return new THREE.Color(`hsl(${Math.abs(hash) % 360}, 70%, 55%)`);
}

function getColor(pt) {
  return state.colorMode === 'soul-speed' ? soulSpeedColor(pt) : sourceColor(pt);
}

// ── Three.js scene ────────────────────────────────────────────────────────────

let scene, camera, renderer, controls, pointCloud, raycaster, mouse;

function initScene() {
  const canvas = document.getElementById('scene');
  renderer = new THREE.WebGLRenderer({ canvas, antialias: true });
  renderer.setSize(window.innerWidth, window.innerHeight);
  renderer.setPixelRatio(window.devicePixelRatio);

  scene = new THREE.Scene();
  scene.background = new THREE.Color(0x0a0a0f);

  camera = new THREE.PerspectiveCamera(60, window.innerWidth / window.innerHeight, 0.1, 1000);
  camera.position.set(0, 22, 2); // near top-down, slight forward tilt to show depth

  controls = new THREE.OrbitControls(camera, renderer.domElement);
  controls.enableDamping = true;
  controls.dampingFactor = 0.05;
  controls.target.set(0, 5, 0); // look at mid-height of data

  raycaster = new THREE.Raycaster();
  raycaster.params.Points = { threshold: 0.25 };
  mouse = new THREE.Vector2();

  addAxisArrows();

  window.addEventListener('resize', onResize);
  renderer.domElement.addEventListener('mousemove', onMouseMove);
}

// ── Point cloud ───────────────────────────────────────────────────────────────

function renderPoints() {
  if (pointCloud) scene.remove(pointCloud);

  const n = state.positions.length;
  const positions = new Float32Array(n * 3);
  const colors    = new Float32Array(n * 3);

  for (let i = 0; i < n; i++) {
    const p = state.positions[i];
    positions[i * 3]     = p.x;
    positions[i * 3 + 1] = p.y;
    positions[i * 3 + 2] = p.z;

    const c = getColor(state.points[i]);
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
  scene.add(new THREE.ArrowHelper(new THREE.Vector3(1,0,0), new THREE.Vector3(-5,0,0), len,      col, hLen, hW));
  scene.add(new THREE.ArrowHelper(new THREE.Vector3(0,1,0), new THREE.Vector3(0,0,0),  len*1.5,  col, hLen, hW));
  scene.add(new THREE.ArrowHelper(new THREE.Vector3(0,0,1), new THREE.Vector3(0,0,-5), len,      col, hLen, hW));
  addLabel('UMAP-1', 2,          -0.8,        0);
  addLabel('Time ↑', -0.8,       len*1.5+0.5, 0);
  addLabel('UMAP-2', 0,          -0.8,        2);
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

// ── Tooltip via raycasting ────────────────────────────────────────────────────

function onMouseMove(event) {
  mouse.x =  (event.clientX / window.innerWidth)  * 2 - 1;
  mouse.y = -(event.clientY / window.innerHeight) * 2 + 1;

  raycaster.setFromCamera(mouse, camera);

  const tip = document.getElementById('tooltip');
  if (!pointCloud) { tip.style.display = 'none'; return; }

  const hits = raycaster.intersectObject(pointCloud);
  if (hits.length > 0) {
    const pt       = state.points[hits[0].index];
    const date     = pt.SinceTimestamp ? pt.SinceTimestamp.substring(0, 10) : '—';
    const concepts = (pt.Concepts || []).slice(0, 3).join(', ') || '(no concepts)';

    tip.querySelector('.source').textContent   = pt.Source;
    tip.querySelector('.date').textContent     = date;
    tip.querySelector('.concepts').textContent = concepts;
    tip.style.display = 'block';
    tip.style.left    = (event.clientX + 14) + 'px';
    tip.style.top     = (event.clientY + 14) + 'px';
  } else {
    tip.style.display = 'none';
  }
}

// ── Controls wiring ───────────────────────────────────────────────────────────

function initControls() {
  const slider    = document.getElementById('days-slider');
  const daysValue = document.getElementById('days-value');

  slider.addEventListener('input', () => {
    state.days = parseInt(slider.value);
    daysValue.textContent = slider.value;
  });
  slider.addEventListener('change', () => reloadAndRender());

  document.getElementById('btn-recompute').addEventListener('click', () => reloadAndRender());

  document.getElementById('btn-color-toggle').addEventListener('click', () => {
    state.colorMode = state.colorMode === 'soul-speed' ? 'source' : 'soul-speed';
    document.getElementById('btn-color-toggle').textContent = 'Color: ' + state.colorMode;
    renderPoints(); // re-color without recomputing UMAP
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
