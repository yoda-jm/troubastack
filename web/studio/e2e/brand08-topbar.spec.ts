import { test, expect } from "@playwright/test";
import { stamp, register } from "./setup-helpers";
test("BRAND08 topbar: compact mark sits beside the name", async ({ page }) => {
  await register(page, `b8_${stamp()}`);
  const mark = page.locator(".brand .brand-mark");
  await expect(mark).toBeVisible();
  await expect(mark).toHaveAttribute("src", "/troubastudio-compact.svg");
  // decorative: the accessible name is the text, not the image
  await expect(mark).toHaveAttribute("aria-hidden", "true");
  await expect(page.locator(".brand .brand-name")).toHaveText("TroubaStudio");
  // the mark actually loaded (natural size > 0), not a broken img
  // the mark actually rendered (real natural size), not a broken/empty <img>
  expect(await mark.evaluate((n: HTMLImageElement) => n.naturalWidth)).toBeGreaterThan(0);
});
