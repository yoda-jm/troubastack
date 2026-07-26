/**
 * The app header must stay on ONE row at phone width — the account trigger sits inline
 * to the right of the brand + nav, not stranded on a second line. (Regression guard for
 * the T47 two-row split that left a lone avatar on its own line after T58 made the
 * trigger avatar-only.)
 */
import { test, expect, type Page } from "@playwright/test";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;

async function register(page: Page, u: string) {
  await page.goto("/register");
  await page.getByTestId("username").fill(u);
  await page.getByTestId("displayName").fill(`D ${u}`);
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}

for (const width of [360, 320]) {
  test(`account trigger stays inline with the brand at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 740 });
    await register(page, `ph_${stamp()}`);

    const brand = await page.locator(".brand").boundingBox();
    const trigger = await page.getByTestId("account-trigger").boundingBox();
    expect(brand).not.toBeNull();
    expect(trigger).not.toBeNull();

    // Same row: the trigger's vertical span overlaps the brand's (not below it).
    expect(trigger!.y).toBeLessThan(brand!.y + brand!.height);
    // Pinned right: the trigger sits to the right of the brand.
    expect(trigger!.x).toBeGreaterThan(brand!.x + brand!.width);
    // The account name is hidden at phone width (avatar-only trigger).
    await expect(page.locator(".account-name")).toBeHidden();

    // No horizontal overflow of the page body.
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    );
    expect(overflow).toBe(true);

    // The menu still opens and is reachable.
    await page.getByTestId("account-trigger").click();
    await expect(page.getByTestId("logout")).toBeVisible();
  });
}
