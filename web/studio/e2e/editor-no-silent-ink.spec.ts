/**
 * T30 — no silent ink: when the realtime connection is down, the editor presents
 * READ-ONLY (chip + grayed draw tools + blocked gestures) instead of letting a
 * stroke render and silently evaporate. Reconnect restores editing.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { stamp, register } from "./setup-helpers";

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

test("offline: read-only chip, grayed tools, a drag leaves no ink; reconnect restores", async ({
  page,
  context,
}) => {
  // context.setOffline blocks NEW connections but does not kill an established
  // WebSocket — so we registry-patch WebSocket and force-close the live socket
  // after going offline (reconnect attempts then fail until we go back online).
  await page.addInitScript(() => {
    const OW = window.WebSocket;
    const sockets: WebSocket[] = ((window as unknown as { __sockets: WebSocket[] }).__sockets = []);
    // eslint-disable-next-line no-global-assign
    (window as unknown as { WebSocket: unknown }).WebSocket = class extends OW {
      constructor(url: string | URL, protocols?: string | string[]) {
        super(url, protocols);
        sockets.push(this as unknown as WebSocket);
      }
    };
  });
  await register(page, `t30_${stamp()}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`T30 ${stamp()}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").first().click();
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill("No Silent Ink");
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();
  await page.getByTestId("my-files-edit").click(); // T36: upload form is in the Details panel
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });

  // Happy path: NO read-only chip while live; draw tools enabled.
  await expect(page.getByTestId("editor-readonly")).toHaveCount(0);
  await expect(page.getByTestId("tool-rect")).toBeEnabled();

  // Arm a draw tool BEFORE the connection drops — the dangerous case is a user
  // mid-flow when the network dies (tool already active; the button-disable alone
  // wouldn't protect them — the gesture gate must).
  await page.getByTestId("tool-rect").click();

  // Kill the network AND drop the live socket → the editor must present read-only.
  await context.setOffline(true);
  await page.evaluate(() =>
    (window as unknown as { __sockets: WebSocket[] }).__sockets.forEach((s) => s.close()),
  );
  await expect(page.getByTestId("editor-readonly")).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId("editor-readonly")).toContainText("Read-only");
  await expect(page.getByTestId("tool-rect")).toBeDisabled();

  // A drag on the canvas must leave NOTHING: no object, no wet residue.
  const before = await page.getByTestId("object-count").innerText();
  const box = (await page.getByTestId("pdf-page").first().boundingBox())!;
  await page.mouse.move(box.x + box.width * 0.3, box.y + box.height * 0.3);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * 0.5, box.y + box.height * 0.4, { steps: 6 });
  const wetAlpha = await page
    .getByTestId("edit-canvas")
    .first()
    .evaluate(
      (el, at) => {
        const c = el as HTMLCanvasElement;
        return c
          .getContext("2d")!
          .getImageData(Math.floor(at.fx * c.width), Math.floor(at.fy * c.height), 1, 1).data[3];
      },
      { fx: 0.4, fy: 0.35 },
    );
  await page.mouse.up();
  expect(wetAlpha, "no wet ink may render while read-only").toBe(0);
  await expect(page.getByTestId("object-count")).toHaveText(before);

  // Back online → the sync client reconnects (500ms→5s backoff) → chip clears,
  // tools re-enable.
  await context.setOffline(false);
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 20_000 });
  await expect(page.getByTestId("editor-readonly")).toHaveCount(0);
  await expect(page.getByTestId("tool-rect")).toBeEnabled();
});
