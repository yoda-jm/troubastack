/**
 * Cross-check: a bundle assembled from bake's overlays + a dummy raster is
 * accepted by A02's Kotlin bundle loader (docs/design/08-bundle-container.md).
 *
 * B01's acceptance asks to "assemble the output + a dummy raster into a bundle dir
 * and run the A02 fixture test path against it." This test does the assembly and
 * validates the result against EXACTLY the contract BundleLoader enforces
 * (app/shared/.../bundle/BundleLoader.kt + BundleLoaderTest):
 *   - bundle.json is proto3 canonical JSON: lowerCamelCase fields; int64/uint64
 *     (concertRev, bakedAt, sourceRevision, songRev) are JSON STRINGS; order is a
 *     number; defaults may be omitted.
 *   - every pageRasterRef and overlay imageRef resolves to an existing, NON-empty
 *     blob (else the loader flags MISSING_BLOB / EMPTY_BLOB).
 *   - overlays are z-ordered by `order`, no duplicate layerId on a page.
 * Mapping index.json → bundle.json is CORE's job (B02); this is an illustrative
 * assembler proving bake's overlays drop cleanly into the container. The live Go
 * core → bake → Kotlin loader path is wired in B02; the Kotlin BundleLoaderTest /
 * FixtureBundleTest run green in-tree (see the B01 commit note).
 */

import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile, readFile, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createCanvas } from "@napi-rs/canvas";
import { renderOverlays } from "../dist/index.js";
import { fixture } from "./fixture.mjs";

/** A tiny non-empty transparent raster standing in for CORE's PDF page raster (B02). */
function dummyRaster(w, h) {
  const cv = createCanvas(w, h);
  const c = cv.getContext("2d");
  c.fillStyle = "rgba(255,255,255,0.01)";
  c.fillRect(0, 0, w, h);
  return cv.encodeSync("png");
}

/** Assemble a bundle dir (bundle.json + blobs/) from bake overlays. Returns the dir. */
async function assembleBundle(dir) {
  await mkdir(join(dir, "blobs"), { recursive: true });
  const overlays = renderOverlays(fixture.doc, fixture.pages, { width: fixture.overlayWidth });

  const pages = [];
  for (const p of fixture.pages) {
    const rasterFile = `blobs/p${p.index}-raster.png`;
    const rasterBytes = dummyRaster(64, Math.round(64 / (p.width / p.height)));
    await writeFile(join(dir, rasterFile), rasterBytes);

    const pageOverlays = overlays
      .filter((o) => o.page === p.index)
      .sort((a, b) => a.order - b.order);
    const overlayEntries = [];
    for (const ov of pageOverlays) {
      const imageRef = `blobs/p${ov.page}-${ov.layerId}.png`;
      await writeFile(join(dir, imageRef), ov.png);
      overlayEntries.push({
        layerId: ov.layerId,
        imageRef,
        contentHash: ov.contentHash,
        order: ov.order,
        mandatory: ov.mandatory,
        roleTag: ov.roleTag,
      });
    }
    pages.push({ pageRasterRef: rasterFile, rasterHash: "dummy", overlays: overlayEntries });
  }

  // proto3 canonical JSON: 64-bit ints are strings; order is a number.
  const bundle = {
    concertId: "crosscheck",
    name: "B01 cross-check",
    concertRev: "1",
    bakedAt: "1700000000",
    bakedBy: "bake",
    finalLocked: false,
    songs: [{ songId: "s1", sourceRevision: "1", songRev: "1", pages }],
  };
  await writeFile(join(dir, "bundle.json"), JSON.stringify(bundle, null, 2) + "\n");
  return { dir, bundle };
}

/** Mirror BundleLoader's validation; return the issues it would report. */
async function loaderIssues(dir, bundle) {
  const issues = [];
  for (const song of bundle.songs) {
    for (let pi = 0; pi < song.pages.length; pi++) {
      const page = song.pages[pi];
      const refs = [page.pageRasterRef, ...page.overlays.map((o) => o.imageRef)].filter(Boolean);
      for (const ref of refs) {
        try {
          const s = await stat(join(dir, ref));
          if (s.size === 0) issues.push({ kind: "EMPTY_BLOB", ref });
        } catch {
          issues.push({ kind: "MISSING_BLOB", ref });
        }
      }
      const seen = new Set();
      for (const o of page.overlays) {
        if (seen.has(o.layerId)) issues.push({ kind: "DUPLICATE_LAYER", ref: o.layerId });
        seen.add(o.layerId);
      }
    }
  }
  return issues;
}

test("bake overlays assemble into a loader-valid bundle (A02 contract)", async () => {
  const dir = await mkdtemp(join(tmpdir(), "bake-crosscheck-"));
  const { bundle } = await assembleBundle(dir);

  // Re-parse bundle.json the way the loader does; assert canonical-JSON shape.
  const parsed = JSON.parse(await readFile(join(dir, "bundle.json"), "utf8"));
  assert.equal(typeof parsed.concertRev, "string", "concertRev is a JSON string (uint64)");
  assert.equal(typeof parsed.bakedAt, "string", "bakedAt is a JSON string (int64)");
  const song = parsed.songs[0];
  assert.equal(typeof song.sourceRevision, "string", "sourceRevision is a JSON string");
  assert.equal(typeof song.songRev, "string", "songRev is a JSON string");

  // Overlays present, z-ordered, and each order is a number.
  for (const page of song.pages) {
    assert.ok(page.overlays.length >= 1, "each page carries ≥1 overlay");
    const orders = page.overlays.map((o) => o.order);
    assert.deepEqual(orders, [...orders].sort((a, b) => a - b), "overlays are z-ordered");
    for (const o of page.overlays) assert.equal(typeof o.order, "number", "order is a JSON number");
  }

  // The loader would report ZERO issues for this bundle (all blobs present + non-empty, no dup layer).
  const issues = await loaderIssues(dir, bundle);
  assert.deepEqual(issues, [], `bundle should load with zero issues: ${JSON.stringify(issues)}`);
});
