// Convex hull algorithm for THREE.js (global namespace, r150 legacy build)
//
// THREE.computeConvexHull(points) → { faces: [[i,j,k], ...], vertices: points }
//
// Returns face index triples. Caller builds BufferGeometry from these —
// avoids the broken BufferGeometry.call(this) pattern in r150 legacy.

(function() {
  'use strict';

  function faceNormal(a, b, c) {
    var ab = new THREE.Vector3().subVectors(b, a);
    var ac = new THREE.Vector3().subVectors(c, a);
    return new THREE.Vector3().crossVectors(ab, ac).normalize();
  }

  THREE.computeConvexHull = function(points) {
    var faces = [];
    if (!points || points.length < 4) return { faces: faces, vertices: points };

    var n = points.length;

    // Find extremal points for initial tetrahedron
    var min = { x: Infinity, y: Infinity, z: Infinity };
    var max = { x: -Infinity, y: -Infinity, z: -Infinity };
    var minIdx = [0, 0, 0], maxIdx = [0, 0, 0];

    for (var i = 0; i < n; i++) {
      var p = points[i];
      if (p.x < min.x) { min.x = p.x; minIdx[0] = i; }
      if (p.y < min.y) { min.y = p.y; minIdx[1] = i; }
      if (p.z < min.z) { min.z = p.z; minIdx[2] = i; }
      if (p.x > max.x) { max.x = p.x; maxIdx[0] = i; }
      if (p.y > max.y) { max.y = p.y; maxIdx[1] = i; }
      if (p.z > max.z) { max.z = p.z; maxIdx[2] = i; }
    }

    // Two most distant points
    var bestDist = 0, a = 0, b = 1;
    var candidates = minIdx.concat(maxIdx);
    for (var i = 0; i < candidates.length; i++) {
      for (var j = i + 1; j < candidates.length; j++) {
        var d = points[candidates[i]].distanceToSquared(points[candidates[j]]);
        if (d > bestDist) { bestDist = d; a = candidates[i]; b = candidates[j]; }
      }
    }

    // Point most distant from line ab
    var ab = new THREE.Vector3().subVectors(points[b], points[a]);
    bestDist = 0;
    var c = 0;
    for (var i = 0; i < n; i++) {
      if (i === a || i === b) continue;
      var ap = new THREE.Vector3().subVectors(points[i], points[a]);
      var cross = new THREE.Vector3().crossVectors(ab, ap);
      var d = cross.lengthSq();
      if (d > bestDist) { bestDist = d; c = i; }
    }

    // Point most distant from plane abc
    var ac = new THREE.Vector3().subVectors(points[c], points[a]);
    var normal = new THREE.Vector3().crossVectors(ab, ac).normalize();
    bestDist = 0;
    var dd = 0;
    for (var i = 0; i < n; i++) {
      if (i === a || i === b || i === c) continue;
      var d = Math.abs(new THREE.Vector3().subVectors(points[i], points[a]).dot(normal));
      if (d > bestDist) { bestDist = d; dd = i; }
    }

    // Orient tetrahedron outward
    var aboveD = new THREE.Vector3().subVectors(points[dd], points[a]).dot(normal);
    if (aboveD > 0) {
      faces = [[a,b,c],[a,c,dd],[a,dd,b],[b,dd,c]];
    } else {
      faces = [[a,c,b],[a,dd,c],[a,b,dd],[b,c,dd]];
    }

    var used = {};
    used[a] = used[b] = used[c] = used[dd] = true;

    // Incrementally add remaining points
    for (var i = 0; i < n; i++) {
      if (used[i]) continue;
      var p = points[i];

      var visible = [];
      for (var f = 0; f < faces.length; f++) {
        var face = faces[f];
        var fn = faceNormal(points[face[0]], points[face[1]], points[face[2]]);
        var d = new THREE.Vector3().subVectors(p, points[face[0]]).dot(fn);
        if (d > 1e-10) visible.push(f);
      }

      if (visible.length === 0) continue;
      used[i] = true;

      var edgeCount = {};
      for (var v = 0; v < visible.length; v++) {
        var face = faces[visible[v]];
        for (var e = 0; e < 3; e++) {
          var e0 = face[e], e1 = face[(e+1)%3];
          var key = Math.min(e0,e1)+':'+Math.max(e0,e1);
          var dirKey = e0+':'+e1;
          if (!edgeCount[key]) edgeCount[key] = [];
          edgeCount[key].push(dirKey);
        }
      }

      var horizon = [];
      for (var key in edgeCount) {
        if (edgeCount[key].length === 1) {
          var parts = edgeCount[key][0].split(':');
          horizon.push([parseInt(parts[0]), parseInt(parts[1])]);
        }
      }

      visible.sort(function(x,y){ return y - x; });
      for (var v = 0; v < visible.length; v++) faces.splice(visible[v], 1);
      for (var h = 0; h < horizon.length; h++) faces.push([horizon[h][0], horizon[h][1], i]);
    }

    return { faces: faces, vertices: points };
  };

})();
