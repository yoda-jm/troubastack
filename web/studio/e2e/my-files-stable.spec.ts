/**
 * T82 — "My files": a checkbox must never move or resize the row it is on. The complaint made
 * testable: capture a row's index AND bounding box, toggle its checkbox, assert BOTH unchanged — for
 * a row in the middle and at each end. Also: uniform geometry checked-vs-unchecked, re-include returns
 * to position (not the end), and the end move-controls are disabled (not absent). Fails on pre-T82.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function register(page: Page, u: string) {
  await page.goto("/register");
  await page.getByTestId("username").fill(u);
  await page.getByTestId("displayName").fill(u);
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}

type Metrics = { dx: number; dy: number; width: number; height: number };

// Measure the row's offset within the LIST <ul> (which holds only the frozen rows) — NOT absolute page
// coords and NOT relative to the panel. Excluding a file legitimately shrinks the viewer strip above the
// panel AND can reflow the panel's hint text; both move the whole list, which is not the row jumping among
// its siblings. Offset-within-the-list isolates exactly the complaint: did THIS row move relative to the
// others. Read the list + row rects in ONE browser turn so a reflow can't slip between the two reads.
async function rowMetrics(page: Page, name: string): Promise<Metrics> {
  // Use the LAYOUT box (offsetTop/offsetLeft), not getBoundingClientRect: the latter includes CSS
  // transforms, so a transient FLIP reorder animation would read as a "move". offsetTop is the row's
  // layout position within its offsetParent (the position:relative list), immune to transforms and to
  // the strip/hint above shifting the page — exactly "did this row move among its siblings".
  return page.evaluate((n) => {
    const rows = Array.from(document.querySelectorAll('[data-testid="my-files-row"]')) as HTMLElement[];
    const el = rows.find((r) => r.textContent?.includes(n))!;
    return { dx: el.offsetLeft, dy: el.offsetTop, width: el.offsetWidth, height: el.offsetHeight };
  }, name);
}
// Assert the row SETTLES to its before-metrics. A single instantaneous read can catch a sub-frame
// transient during the async write/refresh settle (e.g. a proxy ECONNRESET under test load triggers a
// reconcile re-render); polling asserts the steady state — "the row does not move or resize" — which is
// the actual complaint (a permanent jump), while tolerating a momentary reflow.
async function pollSame(page: Page, name: string, before: Metrics) {
  await expect
    .poll(async () => {
      const a = await rowMetrics(page, name);
      return (
        Math.abs(a.dx - before.dx) < 1.5 &&
        Math.abs(a.dy - before.dy) < 1.5 &&
        Math.abs(a.width - before.width) < 1.5 &&
        Math.abs(a.height - before.height) < 1.5
      );
    }, { timeout: 6000 })
    .toBe(true);
}

test("My files: ticking a checkbox never moves or resizes its row (T82)", async ({ page }) => {
  await register(page, `myf_${stamp()}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`MyfBand ${stamp()}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").first().click();
  const bandId = page.url().split("/bands/")[1];
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(`MyfSong ${stamp()}`);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();
  const songId = page.url().split("/songs/")[1];

  // Three pool files, named + ordered A/B/C deterministically.
  await page.getByTestId("my-files-edit").click();
  for (let i = 0; i < 3; i++) {
    await page.getByTestId("file-input").setInputFiles(PDF);
    await page.getByTestId("file-upload").click();
    await expect(page.getByTestId("file-row")).toHaveCount(i + 1);
  }
  const ids = await page.evaluate(async ([b, s]) => {
    const r = await fetch(`/api/bands/${b}/songs/${s}/files`, { credentials: "include" });
    return ((await r.json()) as { files: { id: string }[] }).files.map((f) => f.id);
  }, [bandId, songId]);
  await page.evaluate(
    async ([b, s, fids]) => {
      const ns = ["fileA", "fileB", "fileC"];
      for (let i = 0; i < fids.length; i++) {
        await fetch(`/api/bands/${b}/songs/${s}/files/${fids[i]}`, {
          method: "PATCH",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ filename: ns[i], displayOrder: i }),
        });
      }
    },
    [bandId, songId, ids] as const,
  );
  await page.reload();
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("details-tab-mine").click();
  await expect(page.getByTestId("my-files-panel")).toBeVisible();
  await expect(page.getByTestId("my-files-row")).toHaveCount(3);

  const row = (name: string) => page.getByTestId("my-files-row").filter({ hasText: name });
  const idxOf = async (name: string) =>
    page.getByTestId("my-files-row").evaluateAll(
      (els, n) => els.findIndex((e) => e.textContent?.includes(n)),
      name,
    );

  // Toggle the MIDDLE row (fileB) → index + bbox must be unchanged.
  expect(await idxOf("fileB")).toBe(1);
  const beforeB = await rowMetrics(page, "fileB");
  await row("fileB").getByTestId("my-files-include").click();
  await expect(row("fileB").locator(".my-files-name")).toHaveClass(/muted/); // now excluded (colour only)
  expect(await idxOf("fileB")).toBe(1); // did not move
  await pollSame(page, "fileB", beforeB); // did not move or resize

  // Re-include → returns to its position (index 1), not the end.
  await row("fileB").getByTestId("my-files-include").click();
  expect(await idxOf("fileB")).toBe(1);
  await pollSame(page, "fileB", beforeB);

  // The ENDS: toggle first (fileA) and last (fileC) — index + bbox unchanged too.
  const beforeA = await rowMetrics(page, "fileA");
  await row("fileA").getByTestId("my-files-include").click();
  expect(await idxOf("fileA")).toBe(0);
  await pollSame(page, "fileA", beforeA);

  const beforeC = await rowMetrics(page, "fileC");
  await row("fileC").getByTestId("my-files-include").click();
  expect(await idxOf("fileC")).toBe(2);
await pollSame(page, "fileC", beforeC);

  // End move-controls are DISABLED, not missing (uniform rows).
  await expect(row("fileA").getByTestId("my-files-up")).toBeDisabled();
  await expect(row("fileC").getByTestId("my-files-down")).toBeDisabled();
  await expect(row("fileB").getByTestId("my-files-up")).toBeEnabled();
});

// T82 lost-update guard: force two selection writes to COMPLETE out of order (delay the first PUT),
// then assert the PERSISTED state after a reload matches the final UI. On the ticket-only version the
// superseded EXCLUDE lands at the server last and reload flips the box back — this must fail there.
test("My files: out-of-order writes never lose the last toggle (T82 lost-update)", async ({ page }) => {
  await register(page, `myfr_${stamp()}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`RaceBand ${stamp()}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").first().click();
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(`RaceSong ${stamp()}`);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();

  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.reload();
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("details-tab-mine").click();
  await expect(page.getByTestId("my-files-panel")).toBeVisible();

  // Delay ONLY the first my-files PUT by 2.5s; let the rest through immediately.
  let n = 0;
  await page.route("**/my-files", async (route) => {
    if (route.request().method() === "PUT") {
      n += 1;
      if (n === 1) await new Promise((r) => setTimeout(r, 2500));
    }
    await route.continue();
  });

  const cb = page.getByTestId("my-files-row").first().getByTestId("my-files-include");
  await expect(cb).toBeChecked();
  await cb.click(); // EXCLUDE — write #1 (delayed 2.5s)
  await cb.click(); // INCLUDE — write #2 (final intent)
  await page.waitForTimeout(4000); // let both writes settle

  await expect(cb).toBeChecked(); // UI holds the final intent
  await page.reload(); // and the SERVER must agree
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("details-tab-mine").click();
  await expect(page.getByTestId("my-files-row").first().getByTestId("my-files-include")).toBeChecked();
});

// T82b — a checkbox toggle must re-render the panel ONLY for its own optimistic update. Before the
// memo wrapper, the parent's post-toggle onChanged (viewer-strip refresh) re-rendered the panel a
// second time ~200ms later — the reflow the user reported as a flicker. StrictMode double-invokes, so
// one logical render == +2; the panel must gain exactly its own toggle's render, not the parent's too.
test("My files: a toggle re-renders the panel once — no parent-driven flicker (T82b)", async ({ page }) => {
  await register(page, `myfk_${stamp()}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`FlkBand ${stamp()}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").first().click();
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(`FlkSong ${stamp()}`);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();

  await page.getByTestId("my-files-edit").click();
  for (let i = 0; i < 2; i++) {
    await page.getByTestId("file-input").setInputFiles(PDF);
    await page.getByTestId("file-upload").click();
    await expect(page.getByTestId("file-row")).toHaveCount(i + 1);
  }
  await page.reload();
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("details-tab-mine").click();
  const panel = page.getByTestId("my-files-panel");
  await expect(panel).toBeVisible();
  await expect(page.getByTestId("my-files-row")).toHaveCount(2);

  const before = Number(await panel.getAttribute("data-renders"));
  await page.getByTestId("my-files-row").first().getByTestId("my-files-include").click();
  await page.waitForTimeout(800); // let the async onChanged (strip refresh) fully settle
  const after = Number(await panel.getAttribute("data-renders"));
  expect(
    after - before,
    `panel re-rendered ${after - before}× after one toggle (want <=2: its own optimistic render; more = a parent-driven flicker)`,
  ).toBeLessThanOrEqual(2);
});
