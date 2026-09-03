import { test, expect } from "@playwright/test";

test("BRAND08 login wordmark swaps with the colour scheme", async ({ page }) => {
  // light (also the default / un-stamped "system" state — the Studio follows the OS)
  await page.emulateMedia({ colorScheme: "light" });
  await page.goto("/login");
  // the accessible name is on the wrapper (the outlined SVGs carry no text of their own)
  await expect(page.getByRole("img", { name: "TroubaStudio" })).toBeVisible();
  await expect(page.locator(".auth-wordmark .wm-light")).toBeVisible();
  await expect(page.locator(".auth-wordmark .wm-dark")).toBeHidden();

  // dark
  await page.emulateMedia({ colorScheme: "dark" });
  await expect(page.locator(".auth-wordmark .wm-dark")).toBeVisible();
  await expect(page.locator(".auth-wordmark .wm-light")).toBeHidden();
  // the visible image actually loaded (real natural size), not a broken <img>
  const shown = page.locator(".auth-wordmark .wm-dark");
  expect(await shown.evaluate((n: HTMLImageElement) => n.naturalWidth)).toBeGreaterThan(0);
});
