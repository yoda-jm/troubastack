/**
 * troubabake CLI — the process CORE spawns to render annotation overlays (B02).
 *
 *   node dist/cli.js --in request.json --out <dir>
 *
 * request.json:
 *   {
 *     "doc":   <annotations JSON, exactly as GET .../annotations returns>,
 *     "pages": [ { "index": 0, "width": 1240, "height": 1754 }, ... ],
 *     "overlayWidth": 1600
 *   }
 *
 * The output dir receives one transparent PNG per (page, layer) named
 * `p<index>-<layerId>.png`, plus `index.json` — the manifest CORE reads to
 * assemble bundle.json (08-bundle-container.md). Manifest shape:
 *   {
 *     "overlayWidth": 1600,
 *     "pages": [
 *       { "index": 0, "overlays": [
 *           { "layerId", "file", "order", "mandatory", "roleTag", "contentHash" }
 *       ] }
 *     ]
 *   }
 * (overlays are z-ordered by `order`; `contentHash` is sha256 of the PNG bytes.)
 *
 * Contract with the caller: stdout stays CLEAN (nothing but nothing) so the parent
 * can pipe/ignore it; all logging goes to stderr; a nonzero exit signals failure.
 */

import { mkdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { parseArgs } from "node:util";
import { renderOverlays, type AnnotationsDoc, type PageSize } from "./render.js";

interface BakeCliRequest {
  doc: AnnotationsDoc;
  pages: PageSize[];
  overlayWidth: number;
}

function log(msg: string): void {
  process.stderr.write(`troubabake: ${msg}\n`);
}

/** Keep on-disk names inside the output dir: layer ids are opaque strings. */
function safeLayerId(id: string): string {
  return id.replace(/[^A-Za-z0-9._-]/g, "_") || "unnamed";
}

async function main(): Promise<void> {
  const { values } = parseArgs({
    options: {
      in: { type: "string" },
      out: { type: "string" },
    },
  });
  if (!values.in || !values.out) {
    log("usage: troubabake --in <request.json> --out <dir>");
    process.exitCode = 2;
    return;
  }

  const req = JSON.parse(await readFile(values.in, "utf8")) as BakeCliRequest;
  if (!req.doc || !Array.isArray(req.pages) || !(req.overlayWidth > 0)) {
    log("request.json must have { doc, pages: [...], overlayWidth > 0 }");
    process.exitCode = 2;
    return;
  }

  const overlays = renderOverlays(req.doc, req.pages, { width: req.overlayWidth });

  const outDir = values.out;
  await mkdir(outDir, { recursive: true });

  // Group overlays by page for the manifest, preserving render (z) order.
  const pages = new Map<number, { layerId: string; file: string; order: number; mandatory: boolean; roleTag: string; contentHash: string }[]>();
  for (const ov of overlays) {
    const file = `p${ov.page}-${safeLayerId(ov.layerId)}.png`;
    await writeFile(join(outDir, file), ov.png);
    let list = pages.get(ov.page);
    if (!list) pages.set(ov.page, (list = []));
    list.push({
      layerId: ov.layerId,
      file,
      order: ov.order,
      mandatory: ov.mandatory,
      roleTag: ov.roleTag,
      contentHash: ov.contentHash,
    });
  }

  const manifest = {
    overlayWidth: req.overlayWidth,
    pages: [...pages.keys()]
      .sort((a, b) => a - b)
      .map((index) => ({ index, overlays: pages.get(index)! })),
  };
  await writeFile(join(outDir, "index.json"), JSON.stringify(manifest, null, 2) + "\n");

  log(`wrote ${overlays.length} overlay(s) across ${manifest.pages.length} page(s) to ${outDir}`);
}

main().catch((err) => {
  log(`error: ${err instanceof Error ? err.stack ?? err.message : String(err)}`);
  process.exitCode = 1;
});
