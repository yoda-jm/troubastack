import { test, expect } from "@playwright/test";
import { stamp, register, createBandAndOpen } from "./setup-helpers";

// T130: Overview / Setlists / Settings are tabs of ONE band under a shared layout. Switching tabs must
// NOT unmount the crumb + tab strip or refetch the band (the "refresh" VLL saw), and the crumb must be
// identical across all three.
test("band sections share one layout — strip stays mounted, crumb consistent, band not refetched per tab", async ({
  page,
}) => {
  await register(page, `t130_${stamp()}`);

  // Count GET /api/bands/:id EXACTLY (one segment after /bands — not /members, /songs, /setlists).
  let getBand = 0;
  page.on("request", (r) => {
    if (r.method() === "GET" && /^\/api\/bands\/[^/]+$/.test(new URL(r.url()).pathname)) getBand++;
  });

  const { id } = await createBandAndOpen(page, `Layout ${stamp()}`);

  const strip = page.locator("nav.section-tabs");
  await expect(strip).toBeVisible();
  await page.waitForLoadState("networkidle"); // let the mount fetch(es) settle (StrictMode doubles in dev)
  const baseline = getBand; // 1 in prod, 2 under dev StrictMode — either way, switching must add 0
  expect(baseline).toBeGreaterThanOrEqual(1);

  const stripEl = await strip.elementHandle();
  const stillMounted = async () => (await stripEl!.evaluate((el) => el.isConnected)) === true;
  const crumbHref = await page.locator("a.crumb").getAttribute("href");
  await expect(page.locator("a.crumb")).toHaveText(/Bands/);

  // Overview -> Setlists: SAME strip element still connected, crumb unchanged, band NOT refetched.
  await page.getByTestId("nav-setlists").click();
  await expect(page).toHaveURL(new RegExp(`/bands/${id}/setlists$`));
  await expect(page.getByTestId("setlists-title")).toBeVisible();
  await page.waitForLoadState("networkidle");
  expect(await stillMounted()).toBe(true);
  expect(await page.locator("a.crumb").getAttribute("href")).toBe(crumbHref);
  expect(getBand).toBe(baseline);

  // Setlists -> Settings.
  await page.getByTestId("nav-settings").click();
  await expect(page).toHaveURL(new RegExp(`/bands/${id}/settings$`));
  await expect(page.getByTestId("settings-title")).toBeVisible();
  await page.waitForLoadState("networkidle");
  expect(await stillMounted()).toBe(true);
  expect(await page.locator("a.crumb").getAttribute("href")).toBe(crumbHref);
  expect(getBand).toBe(baseline);

  // Settings -> Overview.
  await page.getByTestId("nav-overview").click();
  await expect(page).toHaveURL(new RegExp(`/bands/${id}$`));
  await expect(page.getByTestId("band-title")).toBeVisible();
  await page.waitForLoadState("networkidle");
  expect(await stillMounted()).toBe(true);
  expect(getBand).toBe(baseline); // three tab switches added ZERO band fetches
});
