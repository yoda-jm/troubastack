/**
 * T196 — anchoring stage 3: Studio shows each mark's link to its chart line (Linked / Check / Lost /
 * Fixed), and the bake dialog lists the marks that need a look before baking.
 *
 * Everything runs on a GENERATED demo chart (synthetic words). Marks are placed through the annotation
 * import, which the server links exactly like a drawn mark (T195 P31); their positions come from a probe
 * copy of the same chart, read back from the server's own `link.at` — so no coordinate here is guessed.
 * Source edits go through the chart-source API, then a reload (a fresh join snapshot), as a real save does.
 *
 * Set T196_SHOTS=<dir> to also write the gate screenshots (light + dark) there.
 */
import { test, expect, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import { stamp, register, createBandAndOpen } from "./setup-helpers";
import { closeDrawer, openDrawer, scrollFracIntoBand } from "./fullscreen-helpers";

const SHOTS = process.env.T196_SHOTS ?? "";

const SOURCE = [
  "# Harbour Lights Demo",
  "",
  "## Verse",
  "G            D",
  "Pack a little light for the road ahead,",
  "",
  "C            G",
  "lanterns swinging by the water's edge",
  "every rope is coiled and ready now",
  "",
  "## Chorus",
  "hold the line until the morning comes",
  "",
].join("\n");

const L1 = "Pack a little light for the road ahead,";
const L2 = "lanterns swinging by the water's edge";
const L3 = "every rope is coiled and ready now";
const L4 = "hold the line until the morning comes";

type At = { page: number; x: number; y: number; h: number };
type Ctx = { bandId: string; songId: string; fileId: string; me: string; geo: Map<string, At> };

async function api<T>(page: Page, method: "GET" | "POST" | "PUT", url: string, data?: unknown): Promise<T> {
  const r = await page.request.fetch(url, { method, data });
  if (!r.ok()) throw new Error(`${method} ${url}: ${r.status()} ${await r.text()}`);
  return (await r.json()) as T;
}

async function newChartSong(page: Page, bandId: string, title: string, source: string) {
  const { song } = await api<{ song: { id: string } }>(page, "POST", `/api/bands/${bandId}/songs`, { title });
  const { file } = await api<{ file: { id: string } }>(page, "POST", `/api/bands/${bandId}/songs/${song.id}/text-charts`, { source });
  return { songId: song.id, fileId: file.id };
}

const layer = (id: string, fileId: string, me: string) => ({
  id, fileId, name: "My marks", ownerId: me, zone: "personal", order: 0, access: "rw", mandatory: false, roleTag: "",
});

const obj = (uuid: string, layerId: string, type: string, pts: { x: number; y: number }[], extra: Record<string, unknown> = {}) => ({
  uuid, layerId, type, page: 0, text: "", order: 0, createdAt: 0, points: pts,
  style: { color: "#2563eb", opacity: 1, width: 0.003, fontSize: 0.02 },
  ...extra,
});

async function importMarks(page: Page, bandId: string, songId: string, layers: unknown[], objects: unknown[]) {
  await api(page, "POST", `/api/bands/${bandId}/songs/${songId}/annotations/import`, { layers, objects });
}

type ServedObj = { uuid: string; points: { x: number; y: number }[]; link?: { state: string; at?: At; text?: string } };
async function served(page: Page, bandId: string, songId: string): Promise<Map<string, ServedObj>> {
  const doc = await api<{ objects: ServedObj[] }>(page, "GET", `/api/bands/${bandId}/songs/${songId}/annotations`);
  return new Map(doc.objects.map((o) => [o.uuid, o]));
}

/** Every line's geometry on this chart, from the server: tiny probes in the left margin each link to
 *  the nearest line, and a margin link's `link.at` is that line's start (x0, top, height). */
async function lineGeometry(page: Page, bandId: string, me: string, source: string): Promise<Map<string, At>> {
  const probe = await newChartSong(page, bandId, `Probe ${stamp()}`, source);
  const probes = [];
  for (let i = 0; i < 160; i++) {
    const y = 0.03 + i * 0.004;
    probes.push(obj(`p${i}`, "probe", "rect", [{ x: 0.006, y }, { x: 0.012, y: y + 0.002 }]));
  }
  await importMarks(page, bandId, probe.songId, [layer("probe", probe.fileId, me)], probes);
  const geo = new Map<string, At>();
  for (const o of (await served(page, bandId, probe.songId)).values()) {
    if (o.link?.at && o.link.text && !geo.has(o.link.text)) geo.set(o.link.text, o.link.at);
  }
  for (const t of [L1, L2, L3, L4]) if (!geo.has(t)) throw new Error(`probe: line ${t} not found`);
  return geo;
}

/** The demo song: a generated chart with four marks on one personal layer. */
async function setup(page: Page): Promise<Ctx> {
  await register(page, `ls_${stamp()}`);
  const { id: bandId } = await createBandAndOpen(page, `LinkBand ${stamp()}`);
  const me = await page.evaluate(async () => ((await (await fetch("/api/me")).json()) as { user: { id: string } }).user.id);
  const geo = await lineGeometry(page, bandId, me, SOURCE);
  const { songId, fileId } = await newChartSong(page, bandId, "Harbour Lights Demo", SOURCE);
  const a1 = geo.get(L1)!, a2 = geo.get(L2)!, a3 = geo.get(L3)!, a4 = geo.get(L4)!;
  await importMarks(page, bandId, songId, [layer("marks", fileId, me)], [
    // a highlight BESIDE line 1 (just under it, in the blank line's room): linked below its line, so the
    // dot is on the words and the thin line crosses to the mark
    obj("hl", "marks", "rect", [{ x: a1.x + 0.06, y: a1.y + 1.15 * a1.h }, { x: a1.x + 0.22, y: a1.y + 1.45 * a1.h }], {
      style: { color: "#facc15", opacity: 0.45, width: 0.003, fontSize: 0.02, fill: true, stroke: false, blend: "multiply" },
    }),
    // a circle around the first word of line 2 (a typo there makes it Check)
    obj("circ", "marks", "ellipse", [{ x: a2.x - 0.004, y: a2.y - 0.15 * a2.h }, { x: a2.x + 0.07, y: a2.y + 1.1 * a2.h }]),
    // a box on line 3 (deleting that line makes it Lost)
    obj("del", "marks", "rect", [{ x: a3.x + 0.1, y: a3.y - 0.1 * a3.h }, { x: a3.x + 0.18, y: a3.y + 1.1 * a3.h }]),
    // an OLD-STYLE mark (a T145 anchor, no link): kept as it is by the import (P30), shown as Fixed
    obj("old", "marks", "rect", [{ x: a4.x + 0.15, y: a4.y - 0.1 * a4.h }, { x: a4.x + 0.25, y: a4.y + 1.1 * a4.h }], {
      anchor: { runText: L4, occurrence: 1, charStart: 0, charEnd: 4 },
    }),
  ]);
  const s = await served(page, bandId, songId);
  expect(s.get("hl")?.link?.state, "fixture: the highlight is linked").toBe("linked");
  expect(s.get("circ")?.link?.state).toBe("linked");
  expect(s.get("del")?.link?.state).toBe("linked");
  expect(s.get("old")?.link, "fixture: the old-style mark has no link").toBeUndefined();
  return { bandId, songId, fileId, me, geo };
}

async function openSong(page: Page, c: Ctx) {
  await page.goto(`/bands/${c.bandId}/songs/${c.songId}?file=${c.fileId}`);
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
}

async function saveSource(page: Page, c: Ctx, edit: (src: string) => string) {
  const cur = await api<{ file: { revision: number }; source: string }>(
    page, "GET", `/api/bands/${c.bandId}/songs/${c.songId}/files/${c.fileId}/chart-source`,
  );
  await api(page, "PUT", `/api/bands/${c.bandId}/songs/${c.songId}/files/${c.fileId}/chart-source`, {
    source: edit(cur.source), baseRevision: cur.file.revision,
  });
}

/** Click a mark (Select tool) at the centre of its served box. */
async function selectMark(page: Page, c: Ctx, uuid: string) {
  const o = (await served(page, c.bandId, c.songId)).get(uuid)!;
  const cx = (o.points[0].x + o.points[1].x) / 2, cy = (o.points[0].y + o.points[1].y) / 2;
  await page.getByTestId("tool-select").click();
  const box = await scrollFracIntoBand(page, cy);
  await page.mouse.click(box.x + cx * box.width, box.y + cy * box.height);
  await expect(page.locator(`[data-testid="selected-bbox"][data-uuid="${uuid}"]`)).toBeVisible();
}

async function deselect(page: Page) {
  await page.keyboard.press("Escape");
  if (await page.getByTestId("selected-bbox").count()) {
    // Escape is not bound in every mode: click empty paper (the far right margin, mid-page).
    const box = await scrollFracIntoBand(page, 0.7);
    await page.mouse.click(box.x + 0.97 * box.width, box.y + 0.7 * box.height);
  }
  await expect(page.getByTestId("selected-bbox")).toHaveCount(0);
}

/** The mean RGB of a 3×3 px patch of the SCREEN at (x, y): a real pixel probe of what the user sees. */
async function pixelAt(page: Page, x: number, y: number): Promise<[number, number, number]> {
  const png = await page.screenshot({ clip: { x: Math.round(x) - 1, y: Math.round(y) - 1, width: 3, height: 3 } });
  return page.evaluate(async (b64) => {
    const img = new Image();
    img.src = `data:image/png;base64,${b64}`;
    await img.decode();
    const cv = document.createElement("canvas");
    cv.width = img.width;
    cv.height = img.height;
    const ctx = cv.getContext("2d")!;
    ctx.drawImage(img, 0, 0);
    const d = ctx.getImageData(0, 0, cv.width, cv.height).data;
    let r = 0, g = 0, b = 0;
    const n = d.length / 4;
    for (let i = 0; i < d.length; i += 4) {
      r += d[i];
      g += d[i + 1];
      b += d[i + 2];
    }
    return [r / n, g / n, b / n] as [number, number, number];
  }, png.toString("base64"));
}
/** The Studio accent is a magenta: far more red than green. Paper, black ink, blue chords and the yellow
 *  highlight are not. */
const isAccent = ([r, g]: [number, number, number]) => r - g > 70;

async function shot(page: Page, name: string) {
  if (!SHOTS) return;
  mkdirSync(SHOTS, { recursive: true });
  for (const scheme of ["light", "dark"] as const) {
    await page.emulateMedia({ colorScheme: scheme });
    await page.waitForTimeout(150);
    await page.screenshot({ path: join(SHOTS, `${name}-${scheme}.png`) });
  }
  await page.emulateMedia({ colorScheme: "light" });
}

test.describe("T196 link states", () => {
  test("a selected linked mark shows a dot on its text and a line to it; deselect removes them", async ({ page }) => {
    const c = await setup(page);
    await openSong(page, c);
    await expect(page.getByTestId("link-dot")).toHaveCount(0); // nothing without a selection

    await selectMark(page, c, "hl");
    const dot = page.getByTestId("link-dot");
    await expect(dot).toHaveCount(1);
    await expect(page.getByTestId("link-line")).toHaveCount(1);
    await expect(page.getByTestId("link-details")).toHaveAttribute("data-state", "linked");
    await expect(page.getByTestId("link-details-text")).toHaveText("Linked to “Pack a little light for the…”");

    // The dot is ON line 1's text (where the server says), not on the mark.
    const pg = (await page.getByTestId("pdf-page").first().boundingBox())!;
    const db = (await dot.boundingBox())!;
    const dx = db.x + db.width / 2, dy = db.y + db.height / 2;
    const a1 = c.geo.get(L1)!;
    expect(Math.abs((dy - pg.y) / pg.height - (a1.y + a1.h / 2))).toBeLessThan(0.004);
    // pixel probe: accent pixels at the dot…
    expect(isAccent(await pixelAt(page, dx, dy)), "the dot is drawn in the accent").toBe(true);
    await shot(page, "selected-dot");

    // …and none once deselected.
    await deselect(page);
    await expect(page.getByTestId("link-dot")).toHaveCount(0);
    expect(isAccent(await pixelAt(page, dx, dy)), "no accent left after deselect").toBe(false);
    await expect(page.getByTestId("link-details")).toHaveCount(0);
  });

  test("Show links draws every link and persists across a reload; it never writes anything", async ({ page }) => {
    const c = await setup(page);
    await openSong(page, c);
    const sent: string[] = [];
    page.on("websocket", (ws) => ws.on("framesent", (f) => typeof f.payload === "string" && sent.push(f.payload)));
    const toggle = page.getByTestId("show-links");
    await expect(toggle).toHaveAttribute("aria-pressed", "false"); // off by default
    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-pressed", "true");
    // hl, circ, del are linked; old (Fixed) has no link to draw
    await expect(page.getByTestId("link-tie")).toHaveCount(3);
    await shot(page, "show-links");
    await page.reload();
    await expect(page.getByTestId("show-links")).toHaveAttribute("aria-pressed", "true");
    await expect(page.getByTestId("link-tie")).toHaveCount(3);
    await page.getByTestId("show-links").click();
    await expect(page.getByTestId("link-tie")).toHaveCount(0);
    expect(sent.filter((p) => p.includes('"mutation"')), "toggling Show links sent a mutation").toEqual([]);
  });

  test("deleting a linked line: the counter shows it, a click selects it as Lost; moving it re-links", async ({ page }) => {
    const c = await setup(page);
    await saveSource(page, c, (s) => s.replace(`${L3}\n`, ""));
    await openSong(page, c);
    const counter = page.getByTestId("link-attention");
    await expect(counter).toHaveText("1 to check");
    await counter.click();
    await expect(page.locator('[data-testid="selected-bbox"][data-uuid="del"]')).toBeVisible();
    await expect(page.getByTestId("link-details")).toHaveAttribute("data-state", "lost");
    await expect(page.getByTestId("link-details-text")).toHaveText(
      "Lost: the line it was on was deleted. Move it onto the right words to link it again.",
    );
    await expect(page.getByTestId("link-dot")).toHaveCount(0); // a lost mark has no linked text to point at
    await shot(page, "counter-lost");

    // Move it onto line 2's words (⟨D3⟩: moving a mark re-links it).
    const o = (await served(page, c.bandId, c.songId)).get("del")!;
    const a2 = c.geo.get(L2)!;
    const cx = (o.points[0].x + o.points[1].x) / 2, cy = (o.points[0].y + o.points[1].y) / 2;
    const ty = a2.y + a2.h / 2;
    const box = await scrollFracIntoBand(page, cy, ty);
    await page.mouse.move(box.x + cx * box.width, box.y + cy * box.height);
    await page.mouse.down();
    await page.mouse.move(box.x + cx * box.width, box.y + ((cy + ty) / 2) * box.height, { steps: 4 });
    await page.mouse.move(box.x + cx * box.width, box.y + ty * box.height, { steps: 4 });
    await page.mouse.up();
    await expect(page.getByTestId("link-details")).toHaveAttribute("data-state", "linked");
    await expect(page.getByTestId("link-details-text")).toHaveText("Linked to “lanterns swinging by the…”");
    await expect(page.getByTestId("link-dot")).toHaveCount(1); // the echo carried the new link.at
    await expect(page.getByTestId("link-attention")).toHaveCount(0);
  });

  test("a typo under a circle makes it Check; an old-style mark is Fixed with its note", async ({ page }) => {
    const c = await setup(page);
    await saveSource(page, c, (s) => s.replace("lanterns swinging", "lantrens swinging"));
    await openSong(page, c);
    await expect(page.getByTestId("link-attention")).toHaveText("1 to check");
    await page.getByTestId("link-attention").click();
    await expect(page.locator('[data-testid="selected-bbox"][data-uuid="circ"]')).toBeVisible();
    await expect(page.getByTestId("link-details")).toHaveAttribute("data-state", "check");
    await expect(page.getByTestId("link-details-text")).toHaveText(
      "Check: the text under this mark changed, or the lines it spans moved apart. Move it to confirm.",
    );
    await expect(page.getByTestId("link-dot")).toHaveCount(1); // a Check mark still follows: it has a dot

    await selectMark(page, c, "old");
    await expect(page.getByTestId("link-details")).toHaveAttribute("data-state", "fixed");
    await expect(page.getByTestId("link-details")).toHaveAttribute("data-legacy", "true");
    await expect(page.getByTestId("link-state")).toHaveText("Fixed");
    await expect(page.getByTestId("link-details-text")).toHaveText(
      "Fixed position on the page. Placed before linking existed; move it to link it.",
    );
    await expect(page.getByTestId("link-dot")).toHaveCount(0);
  });

  test("the bake dialog lists the Check and Lost marks, and does not block the bake", async ({ page }) => {
    const c = await setup(page);
    await saveSource(page, c, (s) => s.replace(`${L3}\n`, "").replace("lanterns swinging", "lantrens swinging"));
    const { setlist } = await api<{ setlist: { id: string } }>(page, "POST", `/api/bands/${c.bandId}/setlists`, { name: "Harbour Night" });
    await api(page, "POST", `/api/bands/${c.bandId}/setlists/${setlist.id}/items`, { songId: c.songId });
    await page.goto(`/bands/${c.bandId}/setlists/${setlist.id}`);
    await page.getByTestId("bake-setlist").click();
    const list = page.getByTestId("bake-link-check");
    await expect(list).toBeVisible();
    await expect(list).toContainText("the tablet cannot show that");
    const rows = page.getByTestId("bake-link-check-row");
    await expect(rows).toHaveCount(1);
    await expect(rows.first()).toContainText("Harbour Lights Demo");
    await expect(page.getByTestId("bake-link-check-counts")).toHaveText("1 Check · 1 Lost");
    await expect(page.getByTestId("bake-dialog-confirm")).toBeEnabled(); // informs, never blocks
    await shot(page, "bake-list");
  });

  test("no indicator reaches a bake: the file PDF is the same bytes with Show links on and off", async ({ page }) => {
    const c = await setup(page);
    await openSong(page, c);
    await selectMark(page, c, "hl"); // a selected mark is showing its dot, too
    // Download THIS file's PDF through the rail's button, the way a writer prints it.
    const download = async () => {
      await openDrawer(page, "layers");
      const resp = page.waitForResponse((r) => /\/files\/[^/]+\/pdf\?/.test(r.url()));
      await page.getByTestId("file-pdf-download").click();
      const r = await resp;
      const body = r.ok() ? await r.body() : Buffer.alloc(0);
      await closeDrawer(page);
      return { status: r.status(), url: r.url(), body };
    };
    const off = await download();
    await page.getByTestId("show-links").click();
    await expect(page.getByTestId("link-tie")).toHaveCount(3);
    const on = await download();
    // What the client asks the server to print is the same request either way: Show links is browser-only
    // state, and nothing about it is ever sent (the previous test proves no mutation is).
    expect(on.url).toBe(off.url);
    if (off.status !== 200) {
      // The server-side print needs the bake toolchain (poppler + the web/bake worker); the CI e2e job and
      // a bare worktree have neither. Say so instead of passing silently on the bytes.
      test.info().annotations.push({ type: "bytes-not-compared", description: `file PDF answered ${off.status}: no bake toolchain on this server` });
      return;
    }
    expect(on.status).toBe(200);
    expect(on.body.length).toBeGreaterThan(1000);
    expect(Buffer.compare(on.body, off.body), "the PDF changed with Show links").toBe(0);
  });
});
