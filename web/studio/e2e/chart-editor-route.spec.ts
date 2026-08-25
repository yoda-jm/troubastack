/**
 * T105 — edit a chart from where you're reading it, on a page of its own.
 *
 * Covers the route host `/bands/:bandId/songs/:songId/chart/:fileId`: the viewer affordance (generated
 * charts only), cold navigation + honest 404, lossless return to the reader's file, and — the part that
 * replaced sub-decision 1 — DRAFT PERSISTENCE across leaving/reloading (a react-router blocker isn't
 * available under <BrowserRouter>, and Back is the obvious exit, so we persist rather than warn).
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

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

// Register → band → song → upload a PDF → create a text chart. Returns the ids so the route/API can be
// addressed directly (cold navigation and the honest-404 both need a real file id).
async function setup(page: Page): Promise<{ bandId: string; songId: string; chartId: string; pdfId: string }> {
  await register(page, `t105_${stamp()}`);
  await createBandAndOpen(page, `T105 ${stamp()}`);
  await createSongAndOpen(page, `T105 Song ${stamp()}`);
  const m = page.url().match(/\/bands\/([^/]+)\/songs\/([^/?]+)/)!;
  const bandId = m[1];
  const songId = m[2];

  // Upload a PDF (a non-chart file, for the affordance-absent + honest-404 cases).
  await page.getByTestId("my-files-edit").click();
  const panel = page.getByTestId("details-panel");
  await panel.getByTestId("file-input").setInputFiles(PDF_PATH);
  await panel.getByTestId("file-upload").click();
  await expect(panel.getByTestId("file-row")).toHaveCount(1);

  // Create a generated text chart.
  await panel.getByTestId("new-text-chart").click();
  await panel.getByTestId("chart-source").fill("# Road Song\n\n## Verse\nthe original words\n");
  await panel.getByTestId("chart-save").click();
  await expect(panel.getByTestId("file-row")).toHaveCount(2);

  const files = await page
    .request.get(`/api/bands/${bandId}/songs/${songId}/files`)
    .then((r) => r.json())
    .then((j: { files: { id: string; generated?: boolean }[] }) => j.files);
  const chartId = files.find((f) => f.generated)!.id;
  const pdfId = files.find((f) => !f.generated)!.id;
  return { bandId, songId, chartId, pdfId };
}

test("viewer: a generated chart offers Edit in the viewer chrome; a PDF does not (T105)", async ({ page }) => {
  const { bandId, songId, chartId, pdfId } = await setup(page);

  // Reading the chart → the Edit affordance is there, without opening the files panel.
  await page.goto(`/bands/${bandId}/songs/${songId}?file=${chartId}`);
  await expect(page.getByTestId("viewer-chrome")).toBeVisible();
  await expect(page.getByTestId("viewer-edit-chart")).toBeVisible();

  // Reading the PDF → nothing to edit, so no affordance. Wait for the page to actually render (so the
  // absence is "PDF selected, no affordance", not "nothing loaded yet").
  await page.goto(`/bands/${bandId}/songs/${songId}?file=${pdfId}`);
  await expect(page.getByTestId("viewer-chrome")).toBeVisible();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("viewer-edit-chart")).toHaveCount(0);

  // The affordance navigates to the dedicated route and loads the editor.
  await page.goto(`/bands/${bandId}/songs/${songId}?file=${chartId}`);
  await page.getByTestId("viewer-edit-chart").click();
  await expect(page).toHaveURL(new RegExp(`/songs/${songId}/chart/${chartId}$`));
  await expect(page.getByTestId("chart-editor")).toBeVisible();
  await expect(page.getByTestId("chart-source")).toHaveValue(/Road Song/);
});

test("route: cold navigation loads the editor; a non-chart file 404s honestly (T105)", async ({ page }) => {
  const { bandId, songId, chartId, pdfId } = await setup(page);

  // Cold nav (as if pasted / reloaded): the editor loads directly, source and all.
  await page.goto(`/bands/${bandId}/songs/${songId}/chart/${chartId}`);
  await expect(page.getByTestId("chart-editor")).toBeVisible();
  await expect(page.getByTestId("chart-source")).toHaveValue(/Road Song/);

  // A PDF id is a real file but not a chart → honest not-found, not an empty editor over nothing.
  await page.goto(`/bands/${bandId}/songs/${songId}/chart/${pdfId}`);
  await expect(page.getByTestId("chart-route-notfound")).toBeVisible();
  await expect(page.getByTestId("chart-editor")).toHaveCount(0);
});

test("route: leaving returns to the song with that file selected (T105)", async ({ page }) => {
  const { bandId, songId, chartId } = await setup(page);
  await page.goto(`/bands/${bandId}/songs/${songId}/chart/${chartId}`);
  await expect(page.getByTestId("chart-editor")).toBeVisible();

  await page.getByTestId("chart-cancel").click(); // "Back to song"
  await expect(page).toHaveURL(new RegExp(`/songs/${songId}\\?file=${chartId}$`));
  await expect(page.getByTestId("viewer-edit-chart")).toBeVisible();
});

test("route: an unsaved edit PERSISTS across leaving and reloading, and Discard drops it (T105)", async ({
  page,
}) => {
  const { bandId, songId, chartId } = await setup(page);
  const routeUrl = `/bands/${bandId}/songs/${songId}/chart/${chartId}`;

  await page.goto(routeUrl);
  await page.getByTestId("chart-source").fill("# Road Song\n\n## Verse\nUNSAVED DRAFT EDIT\n");

  // Leave without saving — Back is the obvious exit, and it must not lose the draft.
  await page.getByTestId("chart-cancel").click();
  await expect(page).toHaveURL(new RegExp(`\\?file=${chartId}$`));

  // Re-enter cold (full load, like a reload): the draft is restored, with a visible hint.
  await page.goto(routeUrl);
  await expect(page.getByTestId("chart-restored")).toBeVisible();
  await expect(page.getByTestId("chart-source")).toHaveValue(/UNSAVED DRAFT EDIT/);

  // Discard drops the draft and falls back to the loaded source.
  await page.getByTestId("chart-restored-discard").click();
  await expect(page.getByTestId("chart-restored")).toHaveCount(0);
  await expect(page.getByTestId("chart-source")).toHaveValue(/the original words/);
  await expect(page.getByTestId("chart-source")).not.toHaveValue(/UNSAVED DRAFT EDIT/);

  // And a discarded draft does not resurrect on the next cold navigation.
  await page.goto(routeUrl);
  await expect(page.getByTestId("chart-restored")).toHaveCount(0);
  await expect(page.getByTestId("chart-source")).toHaveValue(/the original words/);
});

test("route: Save persists to the server, returns to the song, and clears the draft (T105)", async ({
  page,
}) => {
  const { bandId, songId, chartId } = await setup(page);
  const routeUrl = `/bands/${bandId}/songs/${songId}/chart/${chartId}`;

  await page.goto(routeUrl);
  await page.getByTestId("chart-source").fill("# Road Song v2\n\n## Verse\nsaved for real\n");
  await page.getByTestId("chart-save").click();

  // Returns to the reader's file…
  await expect(page).toHaveURL(new RegExp(`/songs/${songId}\\?file=${chartId}$`));
  // …and the saved source is what loads next time, with NO restored-draft hint.
  await page.goto(routeUrl);
  await expect(page.getByTestId("chart-restored")).toHaveCount(0);
  await expect(page.getByTestId("chart-source")).toHaveValue(/saved for real/);
});

test("route: a draft is DROPPED (not restored) when the source moved underneath it (revision key, T105)", async ({
  page,
}) => {
  const { bandId, songId, chartId } = await setup(page);
  const routeUrl = `/bands/${bandId}/songs/${songId}/chart/${chartId}`;

  // A draft at revision 1, then leave — it persists in sessionStorage keyed by revision 1.
  await page.goto(routeUrl);
  await page.getByTestId("chart-source").fill("# Road Song\n\n## Verse\nSTALE DRAFT must be dropped\n");
  await page.getByTestId("chart-cancel").click();
  await expect(page).toHaveURL(new RegExp(`\\?file=${chartId}$`));

  // The source MOVES underneath — a save from elsewhere, which is exactly what a T60 transpose / T67
  // refresh does: it bumps the file revision. Driven via the API (base revision 1 → server writes 2) for
  // determinism, per the reviewer's steer.
  const put = await page.request.put(
    `/api/bands/${bandId}/songs/${songId}/files/${chartId}/chart-source`,
    { data: { source: "# Road Song\n\n## Verse\nMOVED UNDERNEATH from elsewhere\n", baseRevision: 1 } },
  );
  expect(put.ok()).toBeTruthy();

  // Come back cold. The draft was keyed at revision 1; the file is revision 2 now → a different (absent)
  // key → the stale draft must NOT resurrect on top of the moved source. The editor shows the SERVER's
  // source and there is no "Restored your unsaved edits" reassurance. Without `:${rev}` in the key, the
  // key would match and this reverses on both counts — the discriminating vector the guard needs.
  await page.goto(routeUrl);
  await expect(page.getByTestId("chart-editor")).toBeVisible();
  await expect(page.getByTestId("chart-source")).toHaveValue(/MOVED UNDERNEATH/);
  await expect(page.getByTestId("chart-source")).not.toHaveValue(/STALE DRAFT/);
  await expect(page.getByTestId("chart-restored")).toHaveCount(0);
});
