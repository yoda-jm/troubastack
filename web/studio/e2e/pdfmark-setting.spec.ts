import { test, expect } from "@playwright/test";
import { register, createBandAndOpen, stamp } from "./setup-helpers";

// T205 — the Band settings checkbox for the "Made with TroubaStack" PDF mark: on by default, admin-toggleable,
// and persisted (it writes Band.HidePdfMark through PATCH /api/bands/{id}).
test("pdf-mark checkbox: shown by default, toggles off, persists across reload", async ({ page }) => {
  const u = `pm${stamp()}`;
  await register(page, u);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);

  await page.getByTestId("nav-settings").click();
  await expect(page).toHaveURL(new RegExp(`/bands/${id}/settings$`));
  const toggle = page.getByTestId("pdfmark-toggle");
  await expect(toggle).toBeChecked(); // default: the mark is shown

  // Controlled checkbox backed by an async PATCH — click and let the assertion poll for the settled state.
  await toggle.click();
  await expect(toggle).not.toBeChecked();

  // Persisted: reload and it is still off.
  await page.reload();
  await expect(page.getByTestId("pdfmark-toggle")).not.toBeChecked();

  // And can be turned back on.
  await page.getByTestId("pdfmark-toggle").click();
  await expect(page.getByTestId("pdfmark-toggle")).toBeChecked();
  await page.reload();
  await expect(page.getByTestId("pdfmark-toggle")).toBeChecked();
});
