/**
 * DEMO-VID Part B — the web walkthrough, BUILT LIVE FROM AN EMPTY SERVER.
 *
 * This does not drive pre-seeded data: it registers The Troubadours from scratch on camera —
 * Marie signs up and creates the band, invites Leo and Sasha, they accept, she promotes Leo to
 * conductor, adds "The Open Road" and types its chart, everyone tags what they play, Leo marks
 * the capo (green highlight + a ⚠ stamp + a "capo on!" note) on his own layer, we show/hide a
 * layer, transpose, build the setlist and bake it. The orchestra (seeded via `seed -only
 * orchestra` in global-setup) is revealed at the end as "the same app, at orchestra scale".
 * After the run the server holds exactly the demo you can log into (marie / demo).
 *
 * Pacing: `beat(s)` holds a frame for its narration. `soft(label, fn)` runs a step but logs + continues
 * on any miss — for OPTIONAL colour ONLY. The beats the video depends on use `required(label, fn)`, which
 * RE-THROWS so a broken beat FAILS the run instead of skipping silently and recording a confident lie
 * (T125): the capo mark, the layer show/hide toggle, the setlist (create + add song), and the S12 bake —
 * each hard-asserted against its ARTEFACT, not the gesture that should have produced it.
 */
import { test, expect, Page, Locator } from "@playwright/test";
import { writeFileSync, mkdirSync } from "node:fs";

const PW = "demo";
const beat = (p: Page, seconds: number) => p.waitForTimeout(seconds * 1000);
const has = async (l: Locator) => (await l.count()) > 0;

// ---- per-scene sync marks ------------------------------------------------
// Record the video-relative time each narration scene begins, so Part D (assemble.sh) can fit
// each scene's footage to its own narration length instead of stretching the whole clip.
let t0 = 0;
const marks: { id: string; t: number }[] = [];
function mark(id: string) {
  marks.push({ id, t: (Date.now() - t0) / 1000 });
}
function writeMarks() {
  // CWD is web/studio when Playwright runs; write to the repo's video output dir. Ensure it exists —
  // a fresh checkout (CI, a new clone) has no untracked output dir, and this used to crash the whole
  // tour at the very end after every beat had passed.
  mkdirSync("../../docs/video/output", { recursive: true });
  writeFileSync("../../docs/video/output/scene-marks.json", JSON.stringify(marks, null, 2));
}

async function soft(label: string, fn: () => Promise<void>) {
  try {
    await fn();
  } catch (e) {
    // eslint-disable-next-line no-console
    console.log(`[walkthrough] soft-skip "${label}": ${(e as Error).message.split("\n")[0]}`);
  }
}

// required(label, fn) runs a beat the video DEPENDS on and RE-THROWS on any miss — the tour must FAIL,
// not skip, so a broken beat is never recorded as a confident take (T125). soft() is for optional colour.
async function required(label: string, fn: () => Promise<void>) {
  // eslint-disable-next-line no-console
  console.log(`[walkthrough] required "${label}"`);
  await fn();
}

// annotationCount reads the live object-count probe (Viewer.tsx) — used to assert a mark actually landed
// as an annotation, not merely that a gesture was performed.
async function annotationCount(p: Page): Promise<number> {
  return parseInt((await p.getByTestId("object-count").innerText().catch(() => "0")) || "0", 10);
}

// ---- account lifecycle ---------------------------------------------------

async function register(p: Page, username: string, displayName: string) {
  await p.goto("/register", { waitUntil: "networkidle" });
  await p.getByTestId("username").fill(username);
  await p.getByTestId("displayName").fill(displayName);
  await p.getByTestId("password").fill(PW);
  await p.getByTestId("submit").click();
  await p.waitForURL(/\/bands/, { timeout: 15_000 });
  await p.waitForTimeout(500);
}

async function login(p: Page, username: string) {
  await p.goto("/login", { waitUntil: "networkidle" });
  await p.getByTestId("username").fill(username);
  await p.getByTestId("password").fill(PW);
  await p.getByTestId("submit").click();
  await p.waitForURL(/\/bands/, { timeout: 15_000 });
  await p.waitForTimeout(500);
}

async function logout(p: Page) {
  // The song-editor route is full-bleed and hides the topbar (account menu). Always return
  // to /bands first so the account trigger is present.
  await p.goto("/bands", { waitUntil: "networkidle" });
  await p.waitForTimeout(200);
  await p.getByTestId("account-trigger").click();
  await p.getByTestId("logout").click();
  await p.waitForURL(/\/login/, { timeout: 10_000 });
  await p.waitForTimeout(300);
}

// ---- band / member helpers ----------------------------------------------

async function openBand(p: Page, name: string) {
  await p.goto("/bands", { waitUntil: "networkidle" });
  const link = p.getByTestId("band-link").filter({ hasText: name }).first();
  // The band list can lag a beat after navigation; wait for it, reloading once if needed.
  await link.waitFor({ state: "visible", timeout: 15_000 }).catch(async () => {
    await p.reload({ waitUntil: "networkidle" });
    await link.waitFor({ state: "visible", timeout: 12_000 });
  });
  await link.click();
  await p.waitForLoadState("networkidle");
  await p.waitForTimeout(400);
}

async function openSong(p: Page, band: string, title: string) {
  await openBand(p, band);
  await p.getByTestId("song-link").filter({ hasText: title }).first().click();
  await p.waitForLoadState("networkidle");
  await p.waitForTimeout(600);
}

// The Details & files panel is a floating overlay toggled by the `my-files-edit` pill; it is
// open by default and covers the middle of the chart. These keep it in a known state.
async function detailsVisible(p: Page) {
  return (await p.getByTestId("details-panel").count()) > 0 && (await p.getByTestId("details-panel").isVisible().catch(() => false));
}
async function openDetails(p: Page) {
  if (!(await detailsVisible(p))) {
    await p.getByTestId("my-files-edit").click();
    await p.getByTestId("details-panel").waitFor({ state: "visible", timeout: 8_000 }).catch(() => {});
  }
}
async function closeDetails(p: Page) {
  if (await detailsVisible(p)) {
    await p.getByTestId("my-files-edit").click();
    await p.getByTestId("details-panel").waitFor({ state: "hidden", timeout: 8_000 }).catch(() => {});
  }
  await p.waitForTimeout(300);
}

// A freshly-created song may open with no file selected in the viewer. Make sure a page is
// on screen (so there's a canvas to annotate): pick the first file tab, or "choose some".
async function ensureFileShown(p: Page) {
  if ((await p.locator("canvas").count()) > 0) return;
  const chooseSome = p.getByRole("button", { name: /choose some/i });
  if (await has(chooseSome)) {
    await chooseSome.first().click();
    await p.waitForTimeout(500);
  }
  const tab = p.getByTestId("file-tab").first();
  if (await has(tab)) {
    await tab.click();
    await p.waitForTimeout(500);
  }
  // include the first pooled file into "my files" if that's the gate
  const include = p.getByTestId("my-files-include").first();
  if ((await p.locator("canvas").count()) === 0 && (await has(include))) {
    await include.click();
    await p.waitForTimeout(500);
  }
  await p.waitForTimeout(500);
}

test("web walkthrough — build The Troubadours from an empty server", async ({ page }) => {
  test.setTimeout(300_000);
  const BAND = "The Troubadours";
  const SONG = "The Open Road";
  t0 = Date.now();

  // ── S1: the band signs up — accounts, then Marie creates the band ───────────
  // (Sasha & Leo need accounts before Marie can invite them.)
  mark("S1");
  await register(page, "sasha", "Sasha");
  await logout(page);
  await register(page, "leo", "Leo");
  await logout(page);

  // Marie signs up last and stays logged in to create the band.
  await register(page, "marie", "Marie");
  await beat(page, 3); // her empty /bands — nothing pre-loaded
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(BAND);
  await page.getByTestId("create-band").click();
  await page.waitForLoadState("networkidle");
  await beat(page, 4); // the band exists; Marie is the admin

  // ── S2: invite Leo and Sasha by username ───────────────────────────────────
  mark("S2");
  await openBand(page, BAND);
  await soft("invite bandmates", async () => {
    for (const who of ["leo", "sasha"]) {
      const toggle = page.getByTestId("invite-toggle");
      if (await has(toggle)) await toggle.click();
      await page.getByTestId("invite-identifier").fill(who);
      await page.getByTestId("invite-submit").click();
      await beat(page, 2);
    }
  });
  await beat(page, 2);
  await logout(page);

  // Leo and Sasha accept.
  for (const who of ["leo", "sasha"]) {
    await login(page, who);
    await soft(`${who} accepts invite`, async () => {
      await page.goto("/invites", { waitUntil: "networkidle" });
      const accept = page.getByTestId("invite-accept").first();
      if (await has(accept)) {
        await accept.click();
        await beat(page, 2);
      }
    });
    await logout(page);
  }

  // ── S3: Marie promotes Leo to conductor (Settings) ─────────────────────────
  mark("S3");
  await login(page, "marie");
  await openBand(page, BAND);
  await soft("promote Leo to conductor", async () => {
    const settings = page.getByRole("link", { name: /settings/i }).or(page.getByText("Settings", { exact: true }));
    if (await has(settings)) {
      await settings.first().click();
      await page.waitForLoadState("networkidle");
      await beat(page, 1);
    }
    const leoRow = page.getByTestId("settings-member-row").filter({ hasText: "leo" });
    if (await has(leoRow)) {
      await leoRow.getByTestId("member-role-select").selectOption("conductor");
      await beat(page, 3); // Leo is now the conductor
    }
  });

  // ── S4: add "The Open Road" and type its chart as plain text ───────────────
  mark("S4");
  await openBand(page, BAND);
  await soft("create song", async () => {
    await page.getByTestId("new-song-btn").click();
    await page.getByTestId("song-title").fill(SONG);
    await page.getByTestId("create-song").click();
    await page.waitForLoadState("networkidle");
    await beat(page, 2);
  });
  // Ensure we're in the editor, then type the chart via the text-chart source editor.
  await openSong(page, BAND, SONG);
  await soft("type a live text chart", async () => {
    const details = page.getByRole("button", { name: "Details" }).or(page.getByText("Details", { exact: true }));
    if (await has(details)) {
      await details.first().click();
      await beat(page, 1);
    }
    const newChart = page.getByTestId("new-text-chart");
    if (await has(newChart)) {
      await newChart.click();
      const src = page.getByTestId("chart-source");
      await src.click();
      await src.press("Control+a");
      await src.press("Delete");
      await src.type(
        "# The Open Road\n## Capo 2\n\nG               D\nWe were born to run the open road,\nEm              C\nheadlights on the highway home.\n\nG            D            C        G\nMiles roll under a restless sky.\n",
        { delay: 16 },
      );
      await beat(page, 3); // the live preview renders as she types
      const save = page.getByTestId("chart-save");
      if (await has(save)) {
        await save.click();
        await page.waitForLoadState("networkidle");
        await beat(page, 3); // the rendered sheet appears in the pool
      }
    }
  });

  // ── S5: the chart is one file in a shared pool ─────────────────────────────
  mark("S5");
  await soft("show the file pool", async () => {
    await openDetails(page);
    await beat(page, 3); // the Files pool — add tabs, upload PDFs, reorder
  });

  // ── S6: Marie tags what she plays — mic + the RED electric guitar ───────────
  mark("S6");
  await soft("Marie's cues: mic + red guitar", async () => {
    await openDetails(page);
    const mine = page.getByTestId("details-tab-mine");
    if (await has(mine)) await mine.click();
    await beat(page, 1);
    const panel = page.getByTestId("my-cues-panel");
    if (await has(panel)) {
      await panel.getByTestId("cue-add-mic").click();
      await beat(page, 1);
      // tint the NEXT glyph red, then add the electric guitar
      const tint = panel.getByTestId("cue-addcolor");
      if (await has(tint)) await tint.fill("#e11d48").catch(() => {});
      await panel.getByTestId("cue-add-guitar-electric").click();
      await beat(page, 3); // 🎤 + red guitar on her part
    }
  });
  await closeDetails(page);

  // ── S7: THE CAPO — Leo marks his part (REQUIRED: green highlight + ⚠ + note) ─
  // The actor-switch + navigation (blank pages) belongs to S6's tail; S7 (the capo narration)
  // must start on the ready canvas, so mark("S7") is placed AFTER the setup, at the drawing.
  await logout(page);
  await login(page, "leo");
  await openSong(page, BAND, SONG);
  await ensureFileShown(page);
  // Draw with a clean canvas: the Details overlay must be closed so clicks land on the page.
  await closeDetails(page);
  mark("S7");

  // Leo needs an editable personal layer that is the ACTIVE draw target — the Pen is disabled
  // until an editable layer is focused for editing.
  await soft("new + active personal layer", async () => {
    const newLayer = page.getByTestId("new-layer");
    if (await has(newLayer)) {
      await newLayer.click();
      await beat(page, 1);
    }
    const editThis = page.getByTestId("edit-this-layer");
    if (await has(editThis)) {
      await editThis.first().click();
      await beat(page, 1);
    }
  });

  // Draw coordinates are VIEWPORT pixels (the page canvas is taller than the fold, so a
  // canvas-box fraction would map off-screen). "Capo 2" prints near viewport (0.23, 0.43).
  const vp = page.viewportSize();
  if (vp) {
    const at = (fx: number, fy: number) => ({ x: vp.width * fx, y: vp.height * fy });
    const drag = async (a: { x: number; y: number }, b: { x: number; y: number }, steps = 12) => {
      await page.mouse.move(a.x, a.y);
      await page.mouse.down();
      await page.mouse.move(b.x, b.y, { steps });
      await page.mouse.up();
    };
    const setColor = async (hex: string) => {
      const c = page.getByTestId("style-color");
      if (await has(c)) await c.fill(hex).catch(() => {});
    };
    const preset = async (id: string) => {
      const p = page.getByTestId(id);
      if (await has(p)) await p.click().catch(() => {});
      await page.waitForTimeout(120);
    };
    // Set a React-controlled range slider (the ctx-bar opacity/width) to an exact value.
    const setRange = async (testid: string, value: number) => {
      const el = page.getByTestId(testid);
      if (!(await has(el))) return;
      await el.evaluate((node: HTMLInputElement, v: string) => {
        const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")!.set!;
        setter.call(node, v);
        node.dispatchEvent(new Event("input", { bubbles: true }));
      }, String(value));
      await page.waitForTimeout(80);
    };
    // 1) a GREEN highlighter block over the printed "Capo 2": a filled box at low opacity so
    //    the text still reads through it (a real marker swipe, not an opaque block).
    await soft("green capo highlight", async () => {
      const rect = page.getByTestId("tool-rect");
      if (await has(rect)) await rect.click();
      await preset("preset-box"); // filled
      await setColor("#2E9E4F");
      await setRange("style-opacity", 0.32);
      await drag(at(0.168, 0.408), at(0.318, 0.475), 8);
      await beat(page, 2);
    });
    // 2) a big ⚠ warning stamp right of it (the new glyph; dragged bbox, full opacity)
    await soft("warning stamp", async () => {
      const icon = page.getByTestId("tool-icon");
      if (await has(icon)) await icon.click();
      await setColor("#EA580C");
      await setRange("style-opacity", 1);
      const warn = page.getByTestId("icon-pick-warning");
      await expect(warn).toBeVisible({ timeout: 8_000 }); // the ⚠ glyph exists in the palette
      await warn.click();
      await drag(at(0.335, 0.385), at(0.408, 0.5), 10);
      await beat(page, 2);
    });
    // 3) a bold red "capo on!" note above it — text tool → the IN-APP text modal (Dialog.tsx's
    //    app-dialog-input/confirm), NOT window.prompt. REQUIRED: the note must land as an annotation and
    //    the modal must close, or the tour is a lie (and, pre-fix, the open modal jammed everything after).
    await required("capo note", async () => {
      await page.getByTestId("tool-text").click();
      await setColor("#D32F2F");
      await setRange("style-opacity", 1);
      const before = await annotationCount(page);
      const p = at(0.20, 0.34); // on the page (its left edge is ~0.15), above "Capo 2"
      await page.mouse.click(p.x, p.y);
      await page.getByTestId("app-dialog-input").fill("capo on!");
      await page.getByTestId("app-dialog-confirm").click();
      // Artefact, not gesture: the modal closed (no wreckage) AND a new annotation exists.
      await expect(page.getByTestId("app-dialog-input")).toHaveCount(0);
      await expect.poll(() => annotationCount(page)).toBe(before + 1);
      await beat(page, 3);
    });
  }

  // ── S8: layers you can show and hide (REQUIRED beat, VLL) ──────────────────
  mark("S8");
  // The show/hide toggle MUST be on camera: hide Leo's personal layer so the capo ink lifts
  // off the page, then show it again. Mandatory layers (conductor cues) are disabled by design.
  await page.getByTestId("sidebar-toggle").click();
  await expect(page.getByTestId("layers-panel")).toBeVisible();
  await beat(page, 2);
  const toggles = page.getByTestId("layers-panel").getByTestId("layer-toggle");
  const nToggles = await toggles.count();
  let toggled = false;
  for (let i = 0; i < nToggles; i++) {
    const cb = toggles.nth(i);
    if (!(await cb.isEnabled().catch(() => false))) continue;
    if (!(await cb.isChecked().catch(() => false))) continue;
    await cb.scrollIntoViewIfNeeded().catch(() => {});
    await cb.click({ force: true, noWaitAfter: true }); // hide
    await beat(page, 3); // camera: the ink disappears from the canvas
    await expect(cb).not.toBeChecked({ timeout: 8_000 });
    await cb.click({ force: true, noWaitAfter: true }); // show
    await beat(page, 2); // camera: the ink returns
    await expect(cb).toBeChecked({ timeout: 8_000 });
    toggled = true;
    break;
  }
  expect(toggled, "an enabled personal layer to show/hide on camera").toBe(true);

  // Leo's cue (last, so it doesn't interfere with drawing): acoustic guitar.
  await soft("Leo's cue: acoustic guitar", async () => {
    await openDetails(page);
    const mine = page.getByTestId("details-tab-mine");
    if (await has(mine)) await mine.click();
    const panel = page.getByTestId("my-cues-panel");
    if (await has(panel)) {
      await panel.getByTestId("cue-add-guitar-acoustic").click();
      await beat(page, 2);
    }
    await closeDetails(page);
  });
  await logout(page);

  // ── S9: back to Marie — the canvas with everyone's marks ───────────────────
  await login(page, "marie");
  await openSong(page, BAND, SONG);
  mark("S9");
  await beat(page, 4); // the canvas with everyone's marks

  // ── S10: transpose ─────────────────────────────────────────────────────────
  mark("S10");
  // Optional colour (soft), but its entry was dead: `file-menu-source` doesn't exist (the file-menu items
  // are rename/up/down/delete). The chart source opens via the one-click `file-chart-edit` row control (T104).
  await soft("transpose", async () => {
    await openDetails(page);
    const edit = page.getByTestId("file-chart-edit").first();
    if (await has(edit)) {
      await edit.click();
      await expect(page.getByTestId("chart-editor")).toBeVisible({ timeout: 8_000 });
      await beat(page, 1);
    }
    const key = page.getByTestId("transpose-target-key");
    if (await has(key)) {
      await key.selectOption({ index: 2 }).catch(() => {});
      await beat(page, 2);
      const apply = page.getByTestId("transpose-update-key");
      if (await has(apply)) await apply.click();
      await beat(page, 3);
    }
    await closeDetails(page);
  });

  // ── S11: build the setlist ─────────────────────────────────────────────────
  mark("S11");
  await openBand(page, BAND);
  // REQUIRED: the setlist must be created AND contain the song — else S12 bakes nothing (and, since
  // T124, an empty setlist won't bake at all). The old testids here (new-setlist-btn, setlist-add-song)
  // never existed, so each `if (has(...))` silently did nothing and the running order stayed empty.
  await required("setlist", async () => {
    const setlists = page.getByRole("link", { name: /setlists/i }).or(page.getByText("Setlists", { exact: true }));
    await setlists.first().click();
    await page.waitForLoadState("networkidle");
    await beat(page, 1);
    // Create INLINE — the Setlists page has no "new setlist" button; it's a name field + submit.
    await page.getByTestId("setlist-name").fill("Sat @ The Anchor");
    await page.getByTestId("create-setlist").click();
    await page.waitForLoadState("networkidle");
    await beat(page, 2);
    // Open it, then add the song via the REAL controls: the add-item-song <select> + the add-item submit.
    await page.getByTestId("setlist-link").filter({ hasText: "Sat @ The Anchor" }).first().click();
    await page.getByTestId("add-item-song").selectOption({ label: SONG });
    await page.getByTestId("add-item").click();
    // Artefact, not gesture: the running order now contains the song.
    await expect(page.getByTestId("item-row").filter({ hasText: SONG })).toHaveCount(1);
    await beat(page, 3);
  });

  // ── S12: bake the concert ──────────────────────────────────────────────────
  mark("S12");
  // REQUIRED: the finale must actually produce a concert. bake-setlist is disabled for an empty setlist
  // (T124), so this depends on S11 having added the song; a successful bake surfaces a download link.
  await required("bake", async () => {
    await page.getByTestId("bake-setlist").click();
    await beat(page, 2);
    await page.getByTestId("bake-dialog-confirm").click();
    // Artefact, not gesture: a downloadable bundle appears (the bake is a T103 kick — poll for it).
    await expect(page.getByTestId("bake-download")).toBeVisible({ timeout: 30_000 });
    await beat(page, 6); // the concert bundle is produced
  });
  await logout(page);

  // ── S13: the same app, at orchestra scale (seeded) ─────────────────────────
  await login(page, "maestro");
  await openBand(page, "City Chamber Orchestra");
  mark("S13");
  await beat(page, 3);
  await soft("orchestra part", async () => {
    await page.getByTestId("song-link").filter({ hasText: "Eine kleine" }).first().click();
    await page.waitForLoadState("networkidle");
    await beat(page, 6); // a player's part
  });

  // ── S14: everyone sees their own view — the conductor's full score ──────────
  mark("S14");
  await soft("orchestra score", async () => {
    const score = page.getByText("Full score", { exact: false }).first();
    if (await has(score)) {
      await score.click();
      await beat(page, 7); // the conductor's score
    }
  });

  mark("END");
  await beat(page, 1);
  writeMarks();
  expect(true).toBe(true);
});
