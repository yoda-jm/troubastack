/**
 * T43 guard: in SELECT mode on touch, a one-finger drag on EMPTY space draws a marquee
 * (rubber-band select) — matching the desktop mouse grammar — instead of panning. A
 * one-finger drag ON an object still MOVES it (no regression). Two-finger pan/zoom is
 * unchanged (covered by editor-touch.spec). VLL: "a single finger moves the page, it
 * should open a select area?" Ruling: option (b), T27 author — two fingers always
 * navigate, so one-finger-pan in Select mode was redundant.
 *
 * Touch is driven via CDP Input.dispatchTouchEvent (real touch → pointerType:"touch").
 * Red-first: pre-fix a one-finger empty-space drag PANS (selects nothing).
 */
import { test, expect, type Page, type CDPSession } from "@playwright/test";
import { fileURLToPath } from "node:url";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));
const STYLE = { color: "#e11d48", opacity: 1, width: 0.004, fontSize: 0.04 };

async function register(page: Page, u: string) {
  await page.goto("/register");
  await page.getByTestId("username").fill(u);
  await page.getByTestId("displayName").fill(`D ${u}`);
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}
async function myUserId(page: Page) {
  return page.evaluate(async () => {
    const r = await fetch("/api/me", { credentials: "include" });
    return ((await r.json()) as { user: { id: string } }).user.id;
  });
}
async function setup(page: Page, prefix: string) {
  await register(page, `${prefix}_${stamp()}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`${prefix}B ${stamp()}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").first().click();
  const bandId = page.url().split("/bands/")[1];
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(`${prefix}S ${stamp()}`);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();
  const songId = page.url().split("/songs/")[1];
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
  const fileId = await page.evaluate(async (b) => {
    const r = await fetch(`/api/bands/${b}/songs/${location.pathname.split("/songs/")[1]}/files`, {
      credentials: "include",
    });
    return ((await r.json()) as { files: { id: string }[] }).files[0].id;
  }, bandId);
  const me = await myUserId(page);
  return { bandId, songId, fileId, me };
}
async function importTwoRects(page: Page, bandId: string, songId: string, fileId: string, me: string) {
  const doc = {
    layers: [
      { id: "layer-mine", fileId, name: "My notes", ownerId: me, zone: "personal", order: 0, access: "rw", mandatory: false, roleTag: "" },
    ],
    objects: [
      { uuid: "a", layerId: "layer-mine", type: "rect", points: [{ x: 0.15, y: 0.15 }, { x: 0.3, y: 0.3 }], page: 0, text: "", style: STYLE },
      { uuid: "b", layerId: "layer-mine", type: "rect", points: [{ x: 0.4, y: 0.15 }, { x: 0.55, y: 0.3 }], page: 0, text: "", style: STYLE },
    ],
  };
  const res = await page.evaluate(
    async ([b, s, body]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations/import`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      return r.ok;
    },
    [bandId, songId, doc] as const,
  );
  expect(res).toBeTruthy();
}
async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}

/** A one-finger touch drag in absolute client coords, via real CDP touch events. */
async function touchDrag(cdp: CDPSession, x0: number, y0: number, x1: number, y1: number, steps = 10) {
  await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x: x0, y: y0 }] });
  for (let i = 1; i <= steps; i++) {
    const x = x0 + ((x1 - x0) * i) / steps;
    const y = y0 + ((y1 - y0) * i) / steps;
    await cdp.send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ x, y }] });
  }
  await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
}

test("Select mode: one-finger marquee selects both objects, does NOT scroll (T43)", async ({ page }) => {
  const { bandId, songId, fileId, me } = await setup(page, "TM");
  await importTwoRects(page, bandId, songId, fileId, me);
  await page.reload();
  await openEditorReady(page);
  await page.getByTestId("tool-select").click();

  const box = (await page.getByTestId("pdf-page").first().boundingBox())!;
  const abs = (fx: number, fy: number) => ({ x: Math.round(box.x + box.width * fx), y: Math.round(box.y + box.height * fy) });
  const scrollBefore = await page.getByTestId("viewer-scroll").evaluate((el) => el.scrollTop);
  const cdp = await page.context().newCDPSession(page);

  // One-finger drag across EMPTY space enclosing both rects (0.15..0.55 x, 0.15..0.3 y).
  const a = abs(0.1, 0.08);
  const b = abs(0.62, 0.36);
  await touchDrag(cdp, a.x, a.y, b.x, b.y);

  // Marquee selected BOTH — not a pan.
  await expect(page.getByTestId("selected-bbox")).toHaveCount(2);
  // The page did NOT scroll (one-finger in select mode no longer pans).
  const scrollAfter = await page.getByTestId("viewer-scroll").evaluate((el) => el.scrollTop);
  expect(scrollAfter).toBe(scrollBefore);
});

test("Select mode: one-finger drag ON an object still MOVES it (no regression) (T43)", async ({ page }) => {
  const { bandId, songId, fileId, me } = await setup(page, "TMove");
  await importTwoRects(page, bandId, songId, fileId, me);
  await page.reload();
  await openEditorReady(page);
  await page.getByTestId("tool-select").click();

  const box = (await page.getByTestId("pdf-page").first().boundingBox())!;
  const abs = (fx: number, fy: number) => ({ x: Math.round(box.x + box.width * fx), y: Math.round(box.y + box.height * fy) });
  const cdp = await page.context().newCDPSession(page);

  // Start ON rect "a" (centre ~0.225,0.225) and drag right — it should MOVE, not marquee.
  const start = abs(0.225, 0.225);
  const end = abs(0.7, 0.5);
  await touchDrag(cdp, start.x, start.y, end.x, end.y);

  const moved = await page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
      const doc = (await r.json()) as { objects: { uuid: string; points: { x: number }[] }[] };
      const a = doc.objects.find((o) => o.uuid === "a");
      return a ? a.points[0].x : null;
    },
    [bandId, songId],
  );
  // rect "a" started at x=0.15; a move to the right pushes it well past that.
  expect(moved).not.toBeNull();
  expect(moved!).toBeGreaterThan(0.15);
});
