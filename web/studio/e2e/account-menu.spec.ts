/**
 * T58 — topbar account menu mechanics: one avatar+name trigger opens a dropdown that
 * consolidates My account / Get the app / build line / Log out. This spec covers the
 * open/close behaviour and the two nav entries not exercised by get-app.spec (the app
 * panel) and version.spec (the build line + mismatch dot).
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register } from "./setup-helpers";

test("account menu opens, shows the display name, and closes (click-again + Escape + outside)", async ({
  page,
}) => {
  const u = `am_${stamp()}`;
  await register(page, u);

  const trigger = page.getByTestId("account-trigger");
  // The trigger always shows the display name (avatar-only styling is width-driven).
  await expect(page.getByTestId("current-user")).toHaveText(`Display ${u}`);

  // Open → menu visible with all resting entries.
  await trigger.click();
  await expect(page.getByTestId("account-menu")).toBeVisible();
  await expect(page.getByTestId("menu-account")).toBeVisible();
  await expect(page.getByTestId("logout")).toBeVisible();
  await expect(page.getByTestId("version-popover")).toBeVisible(); // build-line footer

  // Click the trigger again → closes.
  await trigger.click();
  await expect(page.getByTestId("account-menu")).toHaveCount(0);

  // Escape closes.
  await trigger.click();
  await expect(page.getByTestId("account-menu")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByTestId("account-menu")).toHaveCount(0);

  // Click outside closes.
  await trigger.click();
  await expect(page.getByTestId("account-menu")).toBeVisible();
  await page.mouse.click(5, 400); // empty page area, away from the menu
  await expect(page.getByTestId("account-menu")).toHaveCount(0);
});

test("My account navigates to /me", async ({ page }) => {
  await register(page, `ma_${stamp()}`);
  await page.getByTestId("account-trigger").click();
  await page.getByTestId("menu-account").click();
  await expect(page).toHaveURL(/\/me$/);
});

test("About TroubaStudio links to the project page in a new tab (BRAND03)", async ({ page }) => {
  await register(page, `ab_${stamp()}`);
  await page.getByTestId("account-trigger").click();
  const about = page.getByTestId("menu-about");
  await expect(about).toBeVisible();
  await expect(about).toHaveText(/About TroubaStudio/);
  await expect(about).toHaveAttribute("href", "https://yoda-jm.github.io/troubastack/");
  await expect(about).toHaveAttribute("target", "_blank");
  // noopener so the opened page can't reach back through window.opener.
  await expect(about).toHaveAttribute("rel", /noopener/);
});

test("Log out returns to /login and clears the session", async ({ page }) => {
  await register(page, `lo_${stamp()}`);
  await page.getByTestId("account-trigger").click();
  await page.getByTestId("logout").click();
  await expect(page).toHaveURL(/\/login/);
  // Session cleared: hitting a guarded route bounces back to /login.
  await page.goto("/bands");
  await expect(page).toHaveURL(/\/login/);
});
