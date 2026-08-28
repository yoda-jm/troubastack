/**
 * FLOW CHECK — build a TEMPLATE concert from an empty server and assert every step.
 *
 * Deliberately synthetic: band "Riverside Session", song "Sound Check", setlist "Template
 * Concert", members alex/robin/jo. No real band data touches this run or the story it produces.
 *
 * Every step records PASS/FAIL and screenshots either way, then writes a manifest. A break
 * therefore still yields a truthful story instead of nothing.
 */
import { test, expect, Page, Locator } from "@playwright/test";
import { writeFileSync, mkdirSync, existsSync, readdirSync, statSync } from "node:fs";

const PW = "demo";
const BAND = "Riverside Session";
const SONG = "Sound Check";
const SETLIST = "Template Concert";
const OUT = "flowcheck/output";

type Step = { n: number; id: string; title: string; note: string; ok: boolean; error?: string; shot?: string };
const steps: Step[] = [];
let bakeArtifact: { path: string; bytes: number } | null = null;
let n = 0;

const has = async (l: Locator) => (await l.count()) > 0;

async function step(p: Page, id: string, title: string, note: string, fn: () => Promise<void>) {
  n += 1;
  const shot = `${String(n).padStart(2, "0")}-${id}.jpg`;
  let ok = true;
  let error: string | undefined;
  try {
    await fn();
  } catch (e) {
    ok = false;
    error = (e as Error).message.split("\n")[0].slice(0, 300);
  }
  await p.waitForTimeout(350);
  try {
    await p.screenshot({ path: `${OUT}/${shot}`, type: "jpeg", quality: 72 });
  } catch { /* keep going */ }
  steps.push({ n, id, title, note, ok, error, shot });
  // eslint-disable-next-line no-console
  console.log(`[flow] ${ok ? "PASS" : "FAIL"} ${n}. ${title}${error ? " — " + error : ""}`);
}

async function register(p: Page, username: string, displayName: string) {
  await p.goto("/register", { waitUntil: "networkidle" });
  await p.getByTestId("username").fill(username);
  await p.getByTestId("displayName").fill(displayName);
  await p.getByTestId("password").fill(PW);
  await p.getByTestId("submit").click();
  await p.waitForURL(/\/bands/, { timeout: 20_000 });
  await p.waitForTimeout(400);
}
async function login(p: Page, username: string) {
  await p.goto("/login", { waitUntil: "networkidle" });
  await p.getByTestId("username").fill(username);
  await p.getByTestId("password").fill(PW);
  await p.getByTestId("submit").click();
  await p.waitForURL(/\/bands/, { timeout: 20_000 });
  await p.waitForTimeout(400);
}
async function logout(p: Page) {
  const t = p.getByTestId("account-trigger");
  if (await has(t)) { await t.click(); await p.waitForTimeout(200); }
  const l = p.getByTestId("logout");
  if (await has(l)) { await l.click(); await p.waitForTimeout(600); }
  await p.goto("/login", { waitUntil: "networkidle" });
}
async function openBand(p: Page, name: string) {
  await p.goto("/bands", { waitUntil: "networkidle" });
  const link = p.getByTestId("band-link").filter({ hasText: name }).first();
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
async function detailsVisible(p: Page) {
  return (await p.getByTestId("details-panel").count()) > 0 &&
    (await p.getByTestId("details-panel").isVisible().catch(() => false));
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
async function ensureFileShown(p: Page) {
  if ((await p.locator("canvas").count()) > 0) return;
  const chooseSome = p.getByRole("button", { name: /choose some/i });
  if (await has(chooseSome)) { await chooseSome.first().click(); await p.waitForTimeout(500); }
  const tab = p.getByTestId("file-tab").first();
  if (await has(tab)) { await tab.click(); await p.waitForTimeout(500); }
  const include = p.getByTestId("my-files-include").first();
  if ((await p.locator("canvas").count()) === 0 && (await has(include))) {
    await include.click(); await p.waitForTimeout(500);
  }
  await p.waitForTimeout(400);
}

test("flow check — build a template concert from an empty server", async ({ page }) => {
  test.setTimeout(600_000);
  mkdirSync(OUT, { recursive: true });

  await step(page, "empty", "An empty server", "Nothing exists yet — no accounts, no bands, no charts.", async () => {
    await page.goto("/login", { waitUntil: "networkidle" });
    await expect(page.getByTestId("username")).toBeVisible();
  });

  await step(page, "accounts", "Three musicians sign up", "robin and jo register first so they can be invited; alex registers last and stays on to run the band.", async () => {
    await register(page, "robin", "Robin");
    await logout(page);
    await register(page, "jo", "Jo");
    await logout(page);
    await register(page, "alex", "Alex");
    await expect(page).toHaveURL(/\/bands/);
  });

  await step(page, "band", "Alex creates the band", `"${BAND}" — Alex is its admin.`, async () => {
    await page.getByTestId("new-band-btn").click();
    await page.getByTestId("band-name").fill(BAND);
    await page.getByTestId("create-band").click();
    await page.waitForLoadState("networkidle");
    await openBand(page, BAND);
    await expect(page.getByText(BAND).first()).toBeVisible();
  });

  await step(page, "invite", "Alex invites robin and jo", "Invitations go out by username — nobody is added without consenting.", async () => {
    await openBand(page, BAND);
    for (const who of ["robin", "jo"]) {
      const toggle = page.getByTestId("invite-toggle");
      if (await has(toggle)) await toggle.click();
      await page.getByTestId("invite-identifier").fill(who);
      await page.getByTestId("invite-submit").click();
      await page.waitForTimeout(800);
    }
  });

  await step(page, "accept", "They accept", "Each invitee accepts from their own account — the band now has three members.", async () => {
    await logout(page);
    for (const who of ["robin", "jo"]) {
      await login(page, who);
      await page.goto("/invites", { waitUntil: "networkidle" });
      const accept = page.getByTestId("invite-accept").first();
      if (await has(accept)) { await accept.click(); await page.waitForTimeout(900); }
      await logout(page);
    }
    await login(page, "alex");
    await expect(page).toHaveURL(/\/bands/);
  });

  await step(page, "role", "Robin is promoted to conductor", "Roles are per band, set in Settings.", async () => {
    await openBand(page, BAND);
    const settings = page.getByRole("link", { name: /settings/i }).or(page.getByText("Settings", { exact: true }));
    if (await has(settings)) { await settings.first().click(); await page.waitForLoadState("networkidle"); }
    const row = page.getByTestId("settings-member-row").filter({ hasText: "robin" });
    await expect(row.first()).toBeVisible({ timeout: 10_000 });
    await row.getByTestId("member-role-select").selectOption("conductor");
    await page.waitForTimeout(700);
  });

  await step(page, "song", "A song is added", `"${SONG}" — an empty song, ready for a chart.`, async () => {
    await openBand(page, BAND);
    await page.getByTestId("new-song-btn").click();
    await page.getByTestId("song-title").fill(SONG);
    await page.getByTestId("create-song").click();
    await page.waitForLoadState("networkidle");
    await expect(page.getByTestId("song-link").filter({ hasText: SONG }).first()).toBeVisible({ timeout: 10_000 });
  });

  await step(page, "chart", "The chart is typed as plain text", "Chords over lyrics in a text source; the sheet renders live from it.", async () => {
    await openSong(page, BAND, SONG);
    const details = page.getByRole("button", { name: "Details" }).or(page.getByText("Details", { exact: true }));
    if (await has(details)) { await details.first().click(); await page.waitForTimeout(600); }
    await page.getByTestId("new-text-chart").click();
    const src = page.getByTestId("chart-source");
    await src.click();
    await src.press("Control+a");
    await src.press("Delete");
    await src.type(
      "# Sound Check\n## Capo 2\n\nG               D\nOne two, one two, the room is warm,\nEm              C\nthe cable hums, the levels form.\n\nG            D            C        G\nCount it in and let the verse begin.\n",
      { delay: 8 },
    );
    await page.waitForTimeout(800);
    await page.getByTestId("chart-save").click();
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(1200);
  });

  await step(page, "pool", "The chart joins a shared file pool", "One song can hold many files; each player chooses which they read.", async () => {
    await openDetails(page);
    await expect(page.getByTestId("details-panel")).toBeVisible({ timeout: 8_000 });
  });

  await step(page, "cues", "Alex tags what he plays", "Per-player instrument cues, so a part knows who it belongs to.", async () => {
    await openDetails(page);
    const mine = page.getByTestId("details-tab-mine");
    if (await has(mine)) { await mine.first().click(); await page.waitForTimeout(400); }
    for (const id of ["cue-add-mic", "cue-add-guitar-electric"]) {
      const b = page.getByTestId(id);
      if (await has(b)) { await b.first().click(); await page.waitForTimeout(500); }
    }
  });

  await step(page, "layer", "Robin opens a personal annotation layer", "Annotations live on layers — yours are yours, and can be shown or hidden.", async () => {
    await logout(page);
    await login(page, "robin");
    await openSong(page, BAND, SONG);
    await ensureFileShown(page);
    await closeDetails(page);
    const newLayer = page.getByTestId("new-layer");
    if (await has(newLayer)) { await newLayer.click(); await page.waitForTimeout(600); }
    const editThis = page.getByTestId("edit-this-layer");
    if (await has(editThis)) { await editThis.first().click(); await page.waitForTimeout(600); }
    await expect(page.locator("canvas").first()).toBeVisible({ timeout: 10_000 });
  });

  await step(page, "annotate", "Robin marks the capo", "A green highlight over “Capo 2”, a ⚠ stamp beside it, and a red “capo on!” note.", async () => {
    const vp = page.viewportSize();
    if (!vp) throw new Error("no viewport");
    const at = (fx: number, fy: number) => ({ x: vp.width * fx, y: vp.height * fy });
    const drag = async (a: {x:number;y:number}, b: {x:number;y:number}, s = 10) => {
      await page.mouse.move(a.x, a.y); await page.mouse.down();
      await page.mouse.move(b.x, b.y, { steps: s }); await page.mouse.up();
    };
    const setColor = async (hex: string) => {
      const c = page.getByTestId("style-color");
      if (await has(c)) await c.fill(hex).catch(() => {});
    };
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
    const rect = page.getByTestId("tool-rect");
    if (await has(rect)) await rect.click();
    const boxPreset = page.getByTestId("preset-box");
    if (await has(boxPreset)) await boxPreset.click().catch(() => {});
    await setColor("#2E9E4F");
    await setRange("style-opacity", 0.32);
    await drag(at(0.168, 0.408), at(0.318, 0.475), 8);
    await page.waitForTimeout(500);

    const icon = page.getByTestId("tool-icon");
    if (await has(icon)) await icon.click();
    await setColor("#EA580C");
    await setRange("style-opacity", 1);
    const warn = page.getByTestId("icon-pick-warning");
    await expect(warn).toBeVisible({ timeout: 8_000 });
    await warn.click();
    await drag(at(0.335, 0.385), at(0.408, 0.5), 10);
    await page.waitForTimeout(500);

    const text = page.getByTestId("tool-text");
    if (await has(text)) await text.click();
    await setColor("#D32F2F");
    await setRange("style-opacity", 1);
    // The text tool opens an IN-PAGE modal (app-level prompt provider, title "Text annotation"),
    // NOT window.prompt — a page.on("dialog") handler never fires and the modal stays open,
    // blocking everything after it. Drive the modal itself.
    const pt = at(0.20, 0.34);
    await page.mouse.click(pt.x, pt.y);
    const noteField = page.getByPlaceholder(/Type your note/i);
    await expect(noteField).toBeVisible({ timeout: 8_000 });
    await noteField.fill("capo on!");
    await page.getByRole("button", { name: "Add", exact: true }).click();
    await expect(noteField).toBeHidden({ timeout: 8_000 });
    await page.waitForTimeout(900);
    // all three marks must be on the layer: highlight + stamp + note
    await expect(page.getByText(/3 objects/)).toBeVisible({ timeout: 8_000 });
  });

  // The toggle lives INSIDE layers-panel, which is collapsed until sidebar-toggle opens it.
  // Mandatory layers (conductor cues) render disabled by design, so pick an enabled+checked one.
  let personalToggle: Locator | null = null;
  await step(page, "layer-hide", "The layer can be hidden", "Toggling it lifts the ink off the page — the printed chart is untouched underneath.", async () => {
    await page.getByTestId("sidebar-toggle").click();
    await expect(page.getByTestId("layers-panel")).toBeVisible({ timeout: 8_000 });
    const toggles = page.getByTestId("layers-panel").getByTestId("layer-toggle");
    const count = await toggles.count();
    for (let i = 0; i < count; i++) {
      const cb = toggles.nth(i);
      if (!(await cb.isEnabled().catch(() => false))) continue;
      if (!(await cb.isChecked().catch(() => false))) continue;
      personalToggle = cb;
      break;
    }
    if (!personalToggle) throw new Error("no enabled, checked personal layer toggle found");
    await personalToggle.scrollIntoViewIfNeeded().catch(() => {});
    await personalToggle.click({ force: true, noWaitAfter: true });
    await page.waitForTimeout(900);
    await expect(personalToggle).not.toBeChecked({ timeout: 8_000 });
  });

  await step(page, "layer-show", "…and shown again", "Non-destructive by construction.", async () => {
    if (!personalToggle) throw new Error("no toggle captured by the previous step");
    await personalToggle.click({ force: true, noWaitAfter: true });
    await page.waitForTimeout(900);
    await expect(personalToggle).toBeChecked({ timeout: 8_000 });
  });

  await step(page, "setlist", "A setlist is built", `"${SETLIST}" — the running order for the night.`, async () => {
    await logout(page);
    await login(page, "alex");
    await openBand(page, BAND);
    const setlists = page.getByRole("link", { name: /setlists/i }).or(page.getByText("Setlists", { exact: true }));
    if (await has(setlists)) { await setlists.first().click(); await page.waitForLoadState("networkidle"); }
    // The Setlists tab renders the create form INLINE (Name / date / Venue / Create) — there is
    // no "new setlist" button to open first. Click one only if this build happens to show one.
    const newSl = page.getByTestId("new-setlist-btn");
    if (await has(newSl)) { await newSl.first().click(); await page.waitForTimeout(400); }
    const nm = page.getByTestId("setlist-name");
    await expect(nm).toBeVisible({ timeout: 10_000 });
    await nm.fill(SETLIST);
    const date = page.locator('input[type="date"]').first();
    if (await has(date)) await date.fill("2026-09-05").catch(() => {});
    await page.getByTestId("create-setlist").click();
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(1200);
    // open the setlist we just made, then add the song to its running order
    const made = page.getByText(SETLIST, { exact: false }).first();
    if (await has(made)) { await made.click().catch(() => {}); await page.waitForTimeout(900); }
    // The running-order control is `add-item` (a submit button) fed by the `add-item-song` <select>.
    // NOTE: there is no `setlist-add-song` testid anywhere in the studio source.
    const songSelect = page.getByTestId("add-item-song");
    await songSelect.scrollIntoViewIfNeeded().catch(() => {});
    await expect(songSelect).toBeVisible({ timeout: 10_000 });
    await songSelect.selectOption({ label: SONG });
    await page.getByTestId("add-item").click();
    await page.waitForTimeout(1500);
    // the running order must actually contain the song — an empty setlist is not a built setlist.
    // (The "1 song" chip splits the count and unit across elements, so assert the ORDER ITEM.)
    await expect(page.getByText(`1. ${SONG}`)).toBeVisible({ timeout: 10_000 });
  });

  await step(page, "bake", "The concert is baked", "Everything collapses into one offline bundle for the tablet — charts, layers, running order.", async () => {
    // Guard against a false pass: baking an EMPTY setlist is permitted by the UI (bake-setlist is
    // only disabled while the dialog is open), so assert the running order is non-empty first.
    await expect(page.getByText(`1. ${SONG}`)).toBeVisible({ timeout: 10_000 });
    const bake = page.getByTestId("bake-setlist");
    await expect(bake).toBeVisible({ timeout: 10_000 });
    await bake.click();
    await page.waitForTimeout(1200);
    const confirm = page.getByTestId("bake-dialog-confirm");
    if (await has(confirm)) { await confirm.click(); }
    // Bakes are detached (202 + poll), so waiting on the UI alone can pass while the worker fails
    // — as it did when web/bake/dist/cli.js was unbuilt: the dir was created and stayed EMPTY.
    // Assert the ARTIFACT, by polling the bake output for a non-empty bundle.
    const bakesDir = "../../core/troubadata-flowcheck/bakes";
    const deadline = Date.now() + 120_000;
    let produced = "";
    while (Date.now() < deadline && !produced) {
      if (existsSync(bakesDir)) {
        for (const d of readdirSync(bakesDir)) {
          const dir = `${bakesDir}/${d}`;
          if (!statSync(dir).isDirectory()) continue;
          for (const f of readdirSync(dir)) {
            const full = `${dir}/${f}`;
            if (statSync(full).isFile() && statSync(full).size > 0) { produced = full; break; }
          }
        }
      }
      if (!produced) await page.waitForTimeout(2000);
    }
    if (!produced) throw new Error("bake produced no artifact within 120s (empty bakes dir)");
    bakeArtifact = { path: produced, bytes: statSync(produced).size };
    await page.waitForTimeout(1500);
  });

  writeFileSync(`${OUT}/manifest.json`, JSON.stringify({
    band: BAND, song: SONG, setlist: SETLIST,
    ranAt: new Date().toISOString(),
    bakeArtifact,
    passed: steps.filter((s) => s.ok).length,
    total: steps.length,
    steps,
  }, null, 2));
  console.log(`[flow] ${steps.filter((s) => s.ok).length}/${steps.length} steps passed`);
});
