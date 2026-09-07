/**
 * Setlist running order: pointer-drag reordering via the row grip (T142 stage 2 — Pointer Events replace
 * HTML5 DnD). Dropping in a row's TOP half lands above it; dropping past the LAST row hits the END gap that
 * the old top-edge model could not reach ("on ne peut pas deplacer un morceau en dernier"). The ↑/↓ buttons
 * remain (flows.spec test 8).
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createSetlist } from "./setup-helpers";

// Drag grip[gripIndex] and release at absolute clientY (a real pointer drag: pointerdown on the grip,
// move, pointerup — the path Pointer Events drive).
async function dragGripToY(page: Page, gripIndex: number, clientY: number) {
  const grip = page.getByTestId("item-grip").nth(gripIndex);
  const gb = (await grip.boundingBox())!;
  const x = gb.x + gb.width / 2;
  await page.mouse.move(x, gb.y + gb.height / 2);
  await page.mouse.down();
  await page.mouse.move(x, clientY, { steps: 10 });
  await page.mouse.up();
}

const rowLowerY = async (page: Page, i: number) => {
  const b = (await page.getByTestId("item-row").nth(i).boundingBox())!;
  return b.y + b.height - 3; // the row's LOWER half (below its midpoint) → the gap AFTER it
};

// Build a band + a setlist with the given songs, landing on the setlist detail page.
async function seedSetlist(page: Page, titles: string[]) {
  const s = stamp();
  await register(page, `dnd_${s}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`DndBand ${s}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: `DndBand ${s}` }).click();
  const bandUrl = page.url();
  for (const t of titles) {
    await page.goto(bandUrl);
    await page.getByTestId("new-song-btn").click();
    await page.getByTestId("song-title").fill(t);
    await page.getByTestId("create-song").click();
    await expect(page.getByTestId("song-link").filter({ hasText: t })).toBeVisible();
  }
  await page.goto(bandUrl);
  await page.getByTestId("nav-setlists").click();
  await createSetlist(page, "Gig");
  await page.getByTestId("setlist-link").first().click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);
  for (let i = 0; i < titles.length; i++) {
    await page.getByTestId("add-item-song").selectOption({ label: titles[i] });
    await page.getByTestId("add-item").click();
    await expect(page.getByTestId("item-row")).toHaveCount(i + 1);
  }
}

// Any inline drag-visual styling left on a row AFTER a drag — a residual pointerEvents:none would make the
// row permanently unclickable ("the app is broken"), so this must always be empty once a drag ends.
async function residualDragStyles(page: Page): Promise<string[]> {
  return page.$$eval('[data-testid="item-row"]', (rows) =>
    rows.flatMap((r, i) => {
      const s = (r as HTMLElement).style;
      return (["transform", "pointerEvents", "zIndex", "boxShadow", "opacity", "position"] as const)
        .filter((p) => s[p])
        .map((p) => `row${i}.${p}=${s[p]}`);
    }),
  );
}

test("pointer-drag drops a running-order song at the very end", async ({ page }) => {
  const s = stamp();
  await register(page, `dnd_${s}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`DndBand ${s}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: `DndBand ${s}` }).click();
  const bandUrl = page.url();

  for (const t of ["Aaa", "Bbb", "Ccc"]) {
    await page.goto(bandUrl);
    await page.getByTestId("new-song-btn").click();
    await page.getByTestId("song-title").fill(t);
    await page.getByTestId("create-song").click();
    await expect(page.getByTestId("song-link").filter({ hasText: t })).toBeVisible();
  }

  await page.goto(bandUrl);
  await page.getByTestId("nav-setlists").click();
  await createSetlist(page, "Gig");
  await page.getByTestId("setlist-link").first().click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);
  for (let i = 0; i < 3; i++) {
    await page.getByTestId("add-item-song").selectOption({ label: ["Aaa", "Bbb", "Ccc"][i] });
    await page.getByTestId("add-item").click();
    await expect(page.getByTestId("item-row")).toHaveCount(i + 1);
  }
  await expect(page.getByTestId("item-title").nth(0)).toContainText("1. Aaa");

  // THE FLAGSHIP: drag Bbb (2nd) into the last row's LOWER half → it lands at the very END — the gap the
  // old top-edge model could not reach ("on ne peut pas deplacer un morceau en dernier"). [Aaa, Ccc, Bbb].
  await dragGripToY(page, 1, await rowLowerY(page, 2));
  await expect(page.getByTestId("item-title").nth(0)).toContainText("1. Aaa");
  await expect(page.getByTestId("item-title").nth(1)).toContainText("2. Ccc");
  await expect(page.getByTestId("item-title").nth(2)).toContainText("3. Bbb");

  // Persists across a reload.
  await page.reload();
  await expect(page.getByTestId("item-title").nth(2)).toContainText("3. Bbb");
});

test("the grip's arrow keys reorder and keep focus (no page jump)", async ({ page }) => {
  const s = stamp();
  await register(page, `dndkb_${s}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`KbBand ${s}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: `KbBand ${s}` }).click();
  const bandUrl = page.url();
  for (const t of ["Aaa", "Bbb"]) {
    await page.goto(bandUrl);
    await page.getByTestId("new-song-btn").click();
    await page.getByTestId("song-title").fill(t);
    await page.getByTestId("create-song").click();
    await expect(page.getByTestId("song-link").filter({ hasText: t })).toBeVisible();
  }
  await page.goto(bandUrl);
  await page.getByTestId("nav-setlists").click();
  await createSetlist(page, "Gig");
  await page.getByTestId("setlist-link").first().click();
  for (let i = 0; i < 2; i++) {
    await page.getByTestId("add-item-song").selectOption({ label: ["Aaa", "Bbb"][i] });
    await page.getByTestId("add-item").click();
    await expect(page.getByTestId("item-row")).toHaveCount(i + 1);
  }

  // Focus the first grip and press ArrowDown → Aaa moves to position 2, and focus stays on the grip.
  await page.getByTestId("item-grip").nth(0).focus();
  await page.keyboard.press("ArrowDown");
  await expect(page.getByTestId("item-title").nth(0)).toContainText("1. Bbb");
  await expect(page.getByTestId("item-title").nth(1)).toContainText("2. Aaa");
  // Focus is on a grip (the moved row's), not lost to <body>.
  const focusedTestId = await page.evaluate(() => document.activeElement?.getAttribute("data-testid"));
  expect(focusedTestId).toBe("item-grip");
});

// T142 follow-up ⟨R1⟩ (Fable): the imperative lift/part styles MUST be cleared once a drag ends — a
// surviving pointerEvents:none freezes the row. clearDragVisual runs from the single endDrag exit (drop AND
// cancel). Teeth: delete that call and both assertions below go red.
test("a completed drag leaves no residual inline styles on any row", async ({ page }) => {
  await seedSetlist(page, ["Aaa", "Bbb", "Ccc"]);
  await dragGripToY(page, 1, await rowLowerY(page, 2)); // Bbb → end
  await expect(page.getByTestId("item-title").nth(2)).toContainText("3. Bbb"); // the drag took effect
  expect(await residualDragStyles(page)).toEqual([]);
});

test("a cancelled drag (pointercancel) also leaves no residual inline styles", async ({ page }) => {
  await seedSetlist(page, ["Aaa", "Bbb", "Ccc"]);
  const grip = page.getByTestId("item-grip").nth(0);
  const gb = (await grip.boundingBox())!;
  await page.mouse.move(gb.x + gb.width / 2, gb.y + gb.height / 2);
  await page.mouse.down();
  await page.mouse.move(gb.x + gb.width / 2, gb.y + gb.height / 2 + 70, { steps: 6 });
  // Mid-drag the grabbed row IS lifted (has a transform) — so "clean after" below is not vacuously true.
  const midTransform = await page.$eval('[data-testid="item-row"]', (r) => (r as HTMLElement).style.transform);
  expect(midTransform).not.toBe("");
  // The browser takes over → pointercancel. endDrag(false) must clear every row's inline styling.
  await page.evaluate(() => document.dispatchEvent(new PointerEvent("pointercancel", { pointerId: 1 })));
  await page.mouse.up(); // harmless — the drag already ended
  expect(await residualDragStyles(page)).toEqual([]);
  await expect(page.getByTestId("item-title").nth(0)).toContainText("1. Aaa"); // a cancel does not reorder
});
