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
import { dirname, join } from "node:path";
import { parseArgs } from "node:util";
import { renderOverlays, type AnnotationsDoc, type PageSize } from "./render.js";

interface SongInput {
  doc: AnnotationsDoc;
  pages: PageSize[];
  overlayWidth: number;
}
// T98 — a request is either a single song (back-compat) or a batch of keyed songs. The batch form
// lets CORE render every song's overlays in ONE node process instead of paying the ~0.6s Skia startup
// per song.
interface BakeCliRequest extends Partial<SongInput> {
  songs?: (SongInput & { key: string })[];
}

interface PageManifest {
  index: number;
  overlays: { layerId: string; file: string; order: number; mandatory: boolean; roleTag: string; contentHash: string }[];
}

function log(msg: string): void {
  process.stderr.write(`troubabake: ${msg}\n`);
}

/** Keep on-disk names inside the output dir: layer ids / song keys are opaque strings. */
function safeName(id: string): string {
  return id.replace(/[^A-Za-z0-9._-]/g, "_") || "unnamed";
}

// renderSong renders one song's overlays under rootOut, folding `prefix` (a "<key>/" subdir for batch,
// "" for single) into every manifest `file` path so CORE reads them uniformly from the root.
async function renderSong(song: SongInput, rootOut: string, prefix: string): Promise<PageManifest[]> {
  const overlays = renderOverlays(song.doc, song.pages, { width: song.overlayWidth });
  const pages = new Map<number, PageManifest["overlays"]>();
  for (const ov of overlays) {
    const file = `${prefix}p${ov.page}-${safeName(ov.layerId)}.png`;
    await mkdir(dirname(join(rootOut, file)), { recursive: true });
    await writeFile(join(rootOut, file), ov.png);
    let list = pages.get(ov.page);
    if (!list) pages.set(ov.page, (list = []));
    list.push({ layerId: ov.layerId, file, order: ov.order, mandatory: ov.mandatory, roleTag: ov.roleTag, contentHash: ov.contentHash });
  }
  return [...pages.keys()].sort((a, b) => a - b).map((index) => ({ index, overlays: pages.get(index)! }));
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
  const outDir = values.out;
  await mkdir(outDir, { recursive: true });

  if (Array.isArray(req.songs)) {
    // BATCH: one process renders every song; outputs are namespaced by song key.
    const songs: { key: string; pages: PageManifest[] }[] = [];
    let total = 0;
    for (const s of req.songs) {
      if (!s.key || !s.doc || !Array.isArray(s.pages) || !(s.overlayWidth > 0)) {
        log("each batch song must have { key, doc, pages: [...], overlayWidth > 0 }");
        process.exitCode = 2;
        return;
      }
      const pages = await renderSong(s, outDir, `${safeName(s.key)}/`);
      total += pages.reduce((n, p) => n + p.overlays.length, 0);
      songs.push({ key: s.key, pages });
    }
    await writeFile(join(outDir, "index.json"), JSON.stringify({ songs }, null, 2) + "\n");
    log(`wrote ${total} overlay(s) across ${songs.length} song(s) to ${outDir}`);
    return;
  }

  // SINGLE (back-compat): unchanged manifest shape at the root.
  const { doc, pages: reqPages, overlayWidth } = req;
  if (!doc || !Array.isArray(reqPages) || typeof overlayWidth !== "number" || !(overlayWidth > 0)) {
    log("request.json must have { doc, pages: [...], overlayWidth > 0 } or { songs: [...] }");
    process.exitCode = 2;
    return;
  }
  const pages = await renderSong({ doc, pages: reqPages, overlayWidth }, outDir, "");
  const manifest = { overlayWidth, pages };
  await writeFile(join(outDir, "index.json"), JSON.stringify(manifest, null, 2) + "\n");
  log(`wrote overlays across ${pages.length} page(s) to ${outDir}`);
}

main().catch((err) => {
  log(`error: ${err instanceof Error ? err.stack ?? err.message : String(err)}`);
  process.exitCode = 1;
});
