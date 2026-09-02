#!/usr/bin/env node
// T50/T51 glyph generator: flatten the hand-authored SVG-ish primitives in
// glyphs.authoring.mjs into the runtime contract glyphs.json — normalized polyline
// strokes + polygon fills, so no consumer (TS ink, Go bake, the studio picker) ever
// parses SVG at runtime. Deterministic (fixed tolerance + rounding) so the CI guard
// `node gen-glyphs.mjs && git diff --exit-code glyphs.json` is stable.
//
// Run: node gen-glyphs.mjs   (writes ./glyphs.json)
import { writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { BOX, STROKE_WIDTH, GLYPHS } from "./glyphs.authoring.mjs";

const HERE = dirname(fileURLToPath(import.meta.url));
// Max chord deviation from the true curve, in BOX (24) units. Fable's ruling pins
// ≤0.002 in the 1×1 box → ≤0.002·BOX here. Coords are rounded to ROUND decimals,
// finer than the tolerance so rounding never dominates.
const TOL = 0.002 * BOX;
const ROUND = 4;

// ---- SVG path parsing (subset: M L H V C S Q T A Z, absolute + relative) ----

// Tokenize a path `d` into [{cmd, params:number[]}], reading arc flags as single
// 0/1 digits (they may abut the next coordinate).
function parsePath(d) {
  let i = 0;
  const n = d.length;
  const out = [];
  const isWs = (c) => c === " " || c === "\t" || c === "\n" || c === "\r" || c === ",";
  function skipWs() {
    while (i < n && isWs(d[i])) i++;
  }
  function readNumber() {
    skipWs();
    const start = i;
    if (d[i] === "+" || d[i] === "-") i++;
    while (i < n && d[i] >= "0" && d[i] <= "9") i++;
    if (d[i] === ".") {
      i++;
      while (i < n && d[i] >= "0" && d[i] <= "9") i++;
    }
    if (d[i] === "e" || d[i] === "E") {
      i++;
      if (d[i] === "+" || d[i] === "-") i++;
      while (i < n && d[i] >= "0" && d[i] <= "9") i++;
    }
    return parseFloat(d.slice(start, i));
  }
  function readFlag() {
    skipWs();
    const c = d[i];
    i++;
    return c === "1" ? 1 : 0;
  }
  const argc = { M: 2, L: 2, H: 1, V: 1, C: 6, S: 4, Q: 4, T: 2, A: 7, Z: 0 };
  while (i < n) {
    skipWs();
    if (i >= n) break;
    const cmd = d[i++];
    const up = cmd.toUpperCase();
    if (!(up in argc)) throw new Error(`bad path command ${cmd} in "${d}"`);
    if (up === "Z") {
      out.push({ cmd, params: [] });
      continue;
    }
    // Implicit repeat: keep consuming parameter groups until the next command.
    do {
      const params = [];
      for (let k = 0; k < argc[up]; k++) {
        // Arc: params 3 (large-arc) and 4 (sweep) are single-digit flags.
        if (up === "A" && (k === 3 || k === 4)) params.push(readFlag());
        else params.push(readNumber());
      }
      out.push({ cmd, params });
      skipWs();
    } while (i < n && (d[i] === "." || d[i] === "-" || d[i] === "+" || (d[i] >= "0" && d[i] <= "9")));
  }
  return out;
}

// ---- flattening ----

const dist = (a, b) => Math.hypot(a[0] - b[0], a[1] - b[1]);

// Recursively subdivide a cubic until flat within TOL; push interior+end points.
function flattenCubic(p0, p1, p2, p3, out) {
  // Flatness: max distance of the two control points from the chord p0→p3.
  const d1 = segDist(p1, p0, p3);
  const d2 = segDist(p2, p0, p3);
  if (Math.max(d1, d2) <= TOL) {
    out.push(p3);
    return;
  }
  // de Casteljau split at t=0.5.
  const p01 = mid(p0, p1),
    p12 = mid(p1, p2),
    p23 = mid(p2, p3);
  const p012 = mid(p01, p12),
    p123 = mid(p12, p23);
  const m = mid(p012, p123);
  flattenCubic(p0, p01, p012, m, out);
  flattenCubic(m, p123, p23, p3, out);
}
function flattenQuad(p0, p1, p2, out) {
  // Elevate to a cubic and reuse.
  const c1 = [p0[0] + (2 / 3) * (p1[0] - p0[0]), p0[1] + (2 / 3) * (p1[1] - p0[1])];
  const c2 = [p2[0] + (2 / 3) * (p1[0] - p2[0]), p2[1] + (2 / 3) * (p1[1] - p2[1])];
  flattenCubic(p0, c1, c2, p2, out);
}
const mid = (a, b) => [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2];
// Perpendicular distance of point p from the segment a→b.
function segDist(p, a, b) {
  const dx = b[0] - a[0],
    dy = b[1] - a[1];
  const len = Math.hypot(dx, dy);
  if (len === 0) return dist(p, a);
  return Math.abs((p[0] - a[0]) * dy - (p[1] - a[1]) * dx) / len;
}

// Endpoint arc → sampled points (SVG implementation notes F.6.5/F.6.6), excluding
// the start point (the caller already holds it).
function flattenArc(x1, y1, rx, ry, phiDeg, large, sweep, x2, y2, out) {
  if (rx === 0 || ry === 0 || (x1 === x2 && y1 === y2)) {
    out.push([x2, y2]);
    return;
  }
  rx = Math.abs(rx);
  ry = Math.abs(ry);
  const phi = (phiDeg * Math.PI) / 180;
  const cosP = Math.cos(phi),
    sinP = Math.sin(phi);
  const dx = (x1 - x2) / 2,
    dy = (y1 - y2) / 2;
  const x1p = cosP * dx + sinP * dy;
  const y1p = -sinP * dx + cosP * dy;
  // Correct out-of-range radii.
  let lambda = (x1p * x1p) / (rx * rx) + (y1p * y1p) / (ry * ry);
  if (lambda > 1) {
    const s = Math.sqrt(lambda);
    rx *= s;
    ry *= s;
  }
  const sign = large === sweep ? -1 : 1;
  let num = rx * rx * ry * ry - rx * rx * y1p * y1p - ry * ry * x1p * x1p;
  num = Math.max(0, num);
  const den = rx * rx * y1p * y1p + ry * ry * x1p * x1p;
  const co = sign * Math.sqrt(num / den);
  const cxp = (co * rx * y1p) / ry;
  const cyp = (-co * ry * x1p) / rx;
  const cx = cosP * cxp - sinP * cyp + (x1 + x2) / 2;
  const cy = sinP * cxp + cosP * cyp + (y1 + y2) / 2;
  const angle = (ux, uy, vx, vy) => {
    const dot = ux * vx + uy * vy;
    const len = Math.hypot(ux, uy) * Math.hypot(vx, vy);
    let a = Math.acos(Math.min(1, Math.max(-1, dot / len)));
    if (ux * vy - uy * vx < 0) a = -a;
    return a;
  };
  const theta1 = angle(1, 0, (x1p - cxp) / rx, (y1p - cyp) / ry);
  let dTheta = angle((x1p - cxp) / rx, (y1p - cyp) / ry, (-x1p - cxp) / rx, (-y1p - cyp) / ry);
  if (!sweep && dTheta > 0) dTheta -= 2 * Math.PI;
  else if (sweep && dTheta < 0) dTheta += 2 * Math.PI;
  // Segment count so chord error < TOL: err = r(1-cos(step/2)).
  const rmax = Math.max(rx, ry);
  const maxStep = 2 * Math.acos(Math.max(0, 1 - TOL / rmax));
  const segs = Math.max(2, Math.ceil(Math.abs(dTheta) / maxStep));
  for (let s = 1; s <= segs; s++) {
    const t = theta1 + (dTheta * s) / segs;
    const px = cx + rx * Math.cos(t) * cosP - ry * Math.sin(t) * sinP;
    const py = cy + rx * Math.cos(t) * sinP + ry * Math.sin(t) * cosP;
    out.push([px, py]);
  }
}

// Flatten a path `d` to an array of subpath polylines (points in BOX units).
function flattenPathD(d) {
  const cmds = parsePath(d);
  const subpaths = [];
  let cur = null; // current polyline
  let x = 0,
    y = 0,
    startX = 0,
    startY = 0;
  let prevCubic = null,
    prevQuad = null,
    prevCmd = "";
  const push = (nx, ny) => {
    cur.push([nx, ny]);
    x = nx;
    y = ny;
  };
  for (const { cmd, params: p } of cmds) {
    const up = cmd.toUpperCase();
    const rel = cmd !== up;
    if (up === "M") {
      // First pair is a moveto (starts a subpath); extra pairs are implicit L.
      let mx = rel ? x + p[0] : p[0];
      let my = rel ? y + p[1] : p[1];
      cur = [[mx, my]];
      subpaths.push(cur);
      x = mx;
      y = my;
      startX = mx;
      startY = my;
      for (let k = 2; k + 1 < p.length; k += 2) {
        push(rel ? x + p[k] : p[k], rel ? y + p[k + 1] : p[k + 1]);
      }
    } else if (up === "L") {
      for (let k = 0; k + 1 < p.length; k += 2) push(rel ? x + p[k] : p[k], rel ? y + p[k + 1] : p[k + 1]);
    } else if (up === "H") {
      for (const v of p) push(rel ? x + v : v, y);
    } else if (up === "V") {
      for (const v of p) push(x, rel ? y + v : v);
    } else if (up === "C") {
      for (let k = 0; k + 5 < p.length; k += 6) {
        const c1 = [rel ? x + p[k] : p[k], rel ? y + p[k + 1] : p[k + 1]];
        const c2 = [rel ? x + p[k + 2] : p[k + 2], rel ? y + p[k + 3] : p[k + 3]];
        const e = [rel ? x + p[k + 4] : p[k + 4], rel ? y + p[k + 5] : p[k + 5]];
        flattenCubic([x, y], c1, c2, e, cur);
        x = e[0];
        y = e[1];
        prevCubic = c2;
      }
    } else if (up === "S") {
      for (let k = 0; k + 3 < p.length; k += 4) {
        const c1 = prevCmd === "C" || prevCmd === "S" ? [2 * x - prevCubic[0], 2 * y - prevCubic[1]] : [x, y];
        const c2 = [rel ? x + p[k] : p[k], rel ? y + p[k + 1] : p[k + 1]];
        const e = [rel ? x + p[k + 2] : p[k + 2], rel ? y + p[k + 3] : p[k + 3]];
        flattenCubic([x, y], c1, c2, e, cur);
        x = e[0];
        y = e[1];
        prevCubic = c2;
        prevCmd = "S";
      }
      continue;
    } else if (up === "Q") {
      for (let k = 0; k + 3 < p.length; k += 4) {
        const c1 = [rel ? x + p[k] : p[k], rel ? y + p[k + 1] : p[k + 1]];
        const e = [rel ? x + p[k + 2] : p[k + 2], rel ? y + p[k + 3] : p[k + 3]];
        flattenQuad([x, y], c1, e, cur);
        x = e[0];
        y = e[1];
        prevQuad = c1;
      }
    } else if (up === "T") {
      for (let k = 0; k + 1 < p.length; k += 2) {
        const c1 = prevCmd === "Q" || prevCmd === "T" ? [2 * x - prevQuad[0], 2 * y - prevQuad[1]] : [x, y];
        const e = [rel ? x + p[k] : p[k], rel ? y + p[k + 1] : p[k + 1]];
        flattenQuad([x, y], c1, e, cur);
        x = e[0];
        y = e[1];
        prevQuad = c1;
        prevCmd = "T";
      }
      continue;
    } else if (up === "A") {
      for (let k = 0; k + 6 < p.length; k += 7) {
        const ex = rel ? x + p[k + 5] : p[k + 5];
        const ey = rel ? y + p[k + 6] : p[k + 6];
        flattenArc(x, y, p[k], p[k + 1], p[k + 2], p[k + 3], p[k + 4], ex, ey, cur);
        x = ex;
        y = ey;
      }
    } else if (up === "Z") {
      push(startX, startY);
    }
    prevCmd = up;
  }
  return subpaths;
}

// Sample a circle as a closed polygon (chord error < TOL).
function circlePoly(cx, cy, r) {
  const maxStep = 2 * Math.acos(Math.max(0, 1 - TOL / r));
  const segs = Math.max(8, Math.ceil((2 * Math.PI) / maxStep));
  const pts = [];
  for (let s = 0; s < segs; s++) {
    const t = (2 * Math.PI * s) / segs;
    pts.push([cx + r * Math.cos(t), cy + r * Math.sin(t)]);
  }
  pts.push(pts[0].slice());
  return pts;
}

// Rounded rectangle as a closed polygon.
function rectPoly(x, y, w, h, rx) {
  rx = Math.min(rx || 0, w / 2, h / 2);
  const pts = [];
  const arc = (cx, cy, a0, a1) => {
    const maxStep = 2 * Math.acos(Math.max(0, 1 - TOL / rx));
    const segs = rx > 0 ? Math.max(2, Math.ceil(Math.abs(a1 - a0) / maxStep)) : 0;
    for (let s = 0; s <= segs; s++) {
      const t = a0 + ((a1 - a0) * s) / segs;
      pts.push([cx + rx * Math.cos(t), cy + rx * Math.sin(t)]);
    }
  };
  if (rx <= 0) {
    return [
      [x, y],
      [x + w, y],
      [x + w, y + h],
      [x, y + h],
      [x, y],
    ];
  }
  // Corners clockwise from top-left; angles in the y-down system.
  arc(x + rx, y + rx, Math.PI, 1.5 * Math.PI); // top-left
  arc(x + w - rx, y + rx, 1.5 * Math.PI, 2 * Math.PI); // top-right
  arc(x + w - rx, y + h - rx, 0, 0.5 * Math.PI); // bottom-right
  arc(x + rx, y + h - rx, 0.5 * Math.PI, Math.PI); // bottom-left
  pts.push(pts[0].slice());
  return pts;
}

// ---- build ----

const round = (v) => Math.round(v * 10 ** ROUND) / 10 ** ROUND;
const norm = (poly) => poly.map(([px, py]) => [round(px / BOX), round(py / BOX)]);

function buildGlyph(shapes) {
  const strokes = [];
  const fills = [];
  for (const shape of shapes) {
    const [kind, ...a] = shape;
    let polys, filled;
    if (kind === "path") {
      const [d, fill] = a;
      polys = flattenPathD(d);
      filled = !!fill;
    } else if (kind === "circle") {
      const [cx, cy, r, fill] = a;
      polys = [circlePoly(cx, cy, r)];
      filled = !!fill;
    } else if (kind === "rect") {
      const [x, y, w, h, r, fill] = a;
      polys = [rectPoly(x, y, w, h, r)];
      filled = !!fill;
    } else {
      throw new Error(`unknown shape kind ${kind}`);
    }
    for (const poly of polys) (filled ? fills : strokes).push(norm(poly));
  }
  return { strokes, fills, strokeWidth: round(STROKE_WIDTH / BOX) };
}

const glyphs = {};
for (const [id, shapes] of Object.entries(GLYPHS)) glyphs[id] = buildGlyph(shapes);

const outPath = join(HERE, "glyphs.json");
writeFileSync(outPath, JSON.stringify({ version: 1, glyphs }, null, 2) + "\n");
console.log(`wrote ${outPath}: ${Object.keys(glyphs).length} glyphs`);

// ---- Kotlin app mirror (T50/A20) ----
// One generator, TWO outputs: the same flattened geometry also emits the app's
// CueGlyphData.kt, so a new glyph lands in glyphs.json AND the app in one run — the CI
// `node gen-glyphs.mjs && git diff --exit-code` guard covers both, no drift folklore.
function emitKotlin(glyphs) {
  const num = (v) => `${v}f`;
  const poly = (p) => "listOf(" + p.map(([x, y]) => `O(${num(x)},${num(y)})`).join(", ") + ")";
  const polys = (ps) => (ps.length ? "listOf(" + ps.map(poly).join(", ") + ")" : "emptyList()");
  const L = [];
  L.push("// GENERATED from web/ink/glyphs.authoring.mjs by web/ink/gen-glyphs.mjs (T50 shared glyph");
  L.push("// contract v1) — DO NOT EDIT BY HAND. Regenerate with `node web/ink/gen-glyphs.mjs` when the");
  L.push("// glyph set changes; the same run also rewrites web/ink/glyphs.json. One source. See docs/tasks/T50.");
  L.push("package com.troubastack.shared.stage");
  L.push("");
  L.push("import androidx.compose.ui.geometry.Offset");
  L.push("");
  L.push("/** One cue glyph's geometry, normalized to a 1x1 box (y-down): [strokes] are stroked (round");
  L.push(" *  cap/join at [strokeWidth]*renderSize), [fills] are filled non-zero. Tinted at draw time. */");
  L.push("internal class CueGlyph(val strokes: List<List<Offset>>, val fills: List<List<Offset>>, val strokeWidth: Float)");
  L.push("");
  L.push("private fun O(x: Float, y: Float) = Offset(x, y)");
  L.push("");
  L.push("/** The curated cue glyph set (T50), in authoring/picker order. Unknown ids resolve to `note`. */");
  L.push("internal val CUE_GLYPHS: Map<String, CueGlyph> = mapOf(");
  for (const [id, g] of Object.entries(glyphs)) {
    L.push(`    "${id}" to CueGlyph(strokes = ${polys(g.strokes)}, fills = ${polys(g.fills)}, strokeWidth = ${num(g.strokeWidth)}),`);
  }
  L.push(")");
  L.push("");
  L.push("/** T50 fallback id for unknown/future icons. */");
  L.push('internal const val CUE_FALLBACK_ID = "note"');
  L.push("");
  L.push("/** Resolve an icon id to a known glyph, falling back to [CUE_FALLBACK_ID] (never null). */");
  L.push("internal fun cueGlyph(icon: String): CueGlyph = CUE_GLYPHS[icon] ?: CUE_GLYPHS.getValue(CUE_FALLBACK_ID)");
  return L.join("\n") + "\n";
}

const ktPath = join(HERE, "..", "..", "app", "shared", "src", "commonMain", "kotlin", "com", "troubastack", "shared", "stage", "CueGlyphData.kt");
writeFileSync(ktPath, emitKotlin(glyphs));
console.log(`wrote ${ktPath}: ${Object.keys(glyphs).length} glyphs`);
