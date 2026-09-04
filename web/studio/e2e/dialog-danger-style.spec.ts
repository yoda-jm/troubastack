import { test, expect } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSetlist } from "./setup-helpers";
const S = "/tmp/claude-1000/-home-yoda-dev-git-troubastack/72d1f559-04a0-4860-bc97-97f9ef5cf3e3/scratchpad";

test("the confirm dialog's destructive button is styled destructive, not like OK (T133)", async ({ page }) => {
  await register(page, `dd_${stamp()}`);
  const { id } = await createBandAndOpen(page, `DD ${stamp()}`);
  await page.goto(`/bands/${id}/setlists`);
  const name = `Gig ${stamp()}`;
  await createSetlist(page, name);
  await page
    .locator("li", { has: page.getByTestId("setlist-link").filter({ hasText: name }) })
    .getByTestId("setlist-menu")
    .click();
  await page.getByTestId("setlist-delete").click(); // danger confirm
  const confirm = page.getByTestId("app-dialog-confirm");
  await expect(confirm).toBeVisible();
  // Computed-style check: the button is the ERROR tint, not the neutral base (the class alone would
  // pass on today's broken version where nothing styles the dialog's danger button).
  const r = await confirm.evaluate((btn) => {
    const probe = document.createElement("div");
    document.body.appendChild(probe);
    const tok = (n: string) => {
      probe.style.color = getComputedStyle(document.documentElement).getPropertyValue(n);
      return getComputedStyle(probe).color;
    };
    const surface = tok("--surface");
    const errbg = tok("--error-bg");
    probe.remove();
    return { bg: getComputedStyle(btn).backgroundColor, surface, errbg };
  });
  expect(r.bg).toBe(r.errbg); // styled with --error-bg
  expect(r.bg).not.toBe(r.surface); // NOT the neutral base — the bug this fixes
  await page.getByTestId("app-dialog").screenshot({ path: `${S}/dialog-danger.png` });
});
