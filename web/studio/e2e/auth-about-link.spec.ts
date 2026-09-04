import { test, expect } from "@playwright/test";
const S = "/tmp/claude-1000/-home-yoda-dev-git-troubastack/72d1f559-04a0-4860-bc97-97f9ef5cf3e3/scratchpad";
const URL = "https://yoda-jm.github.io/troubastack/";

// BRAND11: the logged-out hole. A signed-out visitor never sees the account menu, so the auth
// screens must carry the one route to "what is this?". Checked WHILE LOGGED OUT — the bug's state.
for (const path of ["/login", "/register"]) {
  test(`signed-out visitor reaches the project page from ${path} (BRAND11)`, async ({ page }) => {
    await page.goto(path); // no auth
    const about = page.getByTestId("auth-about");
    await expect(about).toBeVisible();
    await expect(about).toHaveText(/About TroubaStudio/);
    await expect(about).toHaveAttribute("href", URL);
    await expect(about).toHaveAttribute("target", "_blank");
    await expect(about).toHaveAttribute("rel", "noopener noreferrer");
    // it must not navigate the SPA away from itself
    await about.click({ modifiers: ["Control"] }).catch(() => {});
    await expect(page).toHaveURL(new RegExp(`${path}$`));
    if (path === "/login") await page.screenshot({ path: `${S}/brand11-login.png` });
  });
}
