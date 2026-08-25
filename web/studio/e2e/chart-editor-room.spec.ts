/**
 * T104 §2(c) — editing a chart gets real room.
 *
 * The editor used to sit at a fixed `min-height: 22rem` (~17 lines) with two `min-width: 16rem` panes
 * forced side-by-side at every width, so on a narrow viewport they squeezed instead of stacking, and on
 * a tall one you had to drag the textarea bigger every time. These assert the two behaviours the CSS now
 * guarantees — and the NARROW case is the one that silently regresses if someone reverts the media query.
 */
import { test, expect, type Page } from "@playwright/test";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;

async function register(page: Page, u: string) {
  await page.goto("/register");
  await page.getByTestId("username").fill(u);
  await page.getByTestId("displayName").fill(`Display ${u}`);
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}
async function createBandAndOpen(page: Page, name: string) {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(name);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: name }).click();
  await expect(page.getByTestId("band-title")).toHaveText(name);
}
async function createSongAndOpen(page: Page, title: string) {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
}
// Open the details panel and a fresh chart editor (the create flow opens ChartEditor directly).
async function openChartEditor(page: Page, tag: string) {
  await register(page, `t104${tag}_${stamp()}`);
  await createBandAndOpen(page, `T104${tag} ${stamp()}`);
  await createSongAndOpen(page, `T104${tag}Song ${stamp()}`);
  await page.getByTestId("my-files-edit").click();
  const panel = page.getByTestId("details-panel");
  await expect(panel).toBeVisible();
  await panel.getByTestId("new-text-chart").click();
  await expect(panel.getByTestId("chart-editor")).toBeVisible();
  return panel;
}

test("chart editor: on a WIDE, tall viewport the source pane fills far more than the old fixed 22rem (T104)", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1200, height: 1000 });
  await openChartEditor(page, "wide");

  const ta = await page.getByTestId("chart-source").boundingBox();
  // Old behaviour: a fixed min-height of 22rem ≈ 352px. Filling ~62vh of a 1000px viewport is ~620px;
  // assert comfortably above the old fixed floor so a regression to 22rem reddens this.
  expect(ta!.height).toBeGreaterThan(460);
});

test("chart editor: on a NARROW viewport the panes stack and the editor keeps real height (T104)", async ({
  page,
}) => {
  await page.setViewportSize({ width: 600, height: 900 });
  const panel = await openChartEditor(page, "narrow");

  const src = (await panel.getByTestId("chart-source").boundingBox())!;
  // Before Preview is clicked the preview pane shows its empty hint; measure that as the pane's position.
  const preview = (await panel.getByTestId("chart-preview-empty").boundingBox())!;

  // (1) BEHAVIOUR — at narrow width the panes STACK (the preview begins below the source), not
  // side-by-side. NOTE (corrected per review): at 600px the details panel's chrome leaves the panes
  // only ~460px, so `flex-wrap` already stacks them here; the `max-width: 640px` media query REINFORCES
  // this. This assertion guards the behaviour a user sees, via whichever mechanism delivers it — it does
  // NOT isolate the query (two 16rem panes need a ~664px viewport to sit side by side, above the 640
  // breakpoint, so below 640 they always wrap first). The next assertion is the query's own guard.
  expect(preview.y).toBeGreaterThan(src.y + src.height - 8);

  // (2) THE QUERY, ISOLATED — at ≤640px it raises the source textarea's `min-height` to 55vh. That is a
  // direct property (not flex fill), distinguishable from the base 22rem (352px): 55vh of 900 ≈ 495px.
  // Removing the whole media query drops it back to 352 and reddens this — the guard the geometry check
  // above cannot give, because flex-wrap would still stack the panes.
  const minH = await panel
    .getByTestId("chart-source")
    .evaluate((el) => parseFloat(getComputedStyle(el).minHeight));
  expect(minH).toBeGreaterThan(430);
});
