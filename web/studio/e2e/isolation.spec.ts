/**
 * T81 — proof that the e2e suite talks to its OWN fresh, in-memory backend, not a seeded preview
 * that happens to be running on :8080. A mis-pointed vite proxy (aimed at a demo or personal-band backend) would
 * otherwise look perfectly green while testing the wrong server — the exact silent-wrong-target class
 * this task exists to remove. Two independent tells, both through the same vite→core path the tests
 * use:
 *   1. a seed-only account (marie / maestro / vincent, all password "demo") cannot log in — it does
 *      not exist on a fresh backend, but WOULD on any seeded preview;
 *   2. a newly-registered user sees an empty band list — no pre-seeded bands exist here.
 */
import { test, expect } from "@playwright/test";
import { stamp } from "./setup-helpers";

test("e2e backend is fresh + isolated, not a seeded preview (T81)", async ({ page }) => {
  // 1) Seed-only accounts must be rejected — proof this is not the :8080 demo or personal-band server.
  for (const username of ["maestro", "marie", "vincent"]) {
    const res = await page.request.post("/api/auth/login", {
      data: { username, password: "demo" },
      failOnStatusCode: false,
    });
    expect(res.ok(), `seed account "${username}" must not exist on the isolated e2e backend`).toBe(
      false,
    );
  }

  // 2) A brand-new user lands on an EMPTY band list (a seeded server would carry demo bands, but a
  // fresh user is nonetheless a member of none — so this is the weaker tell; the login check above
  // is the one that distinguishes fresh-vs-seeded).
  const u = `iso_${stamp()}`;
  await page.goto("/register");
  await page.getByTestId("username").fill(u);
  await page.getByTestId("displayName").fill(u);
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
  await expect(page.getByTestId("band-link")).toHaveCount(0);
});
