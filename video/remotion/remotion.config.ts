import { Config } from "@remotion/cli/config";
import { existsSync, readdirSync } from "node:fs";
import { homedir } from "node:os";
import path from "node:path";

// Prefer a chrome-headless-shell already on the machine (Playwright's cache) over a download; set
// REMOTION_BROWSER to override.
const pw = path.join(homedir(), ".cache/ms-playwright");
const found = process.env.REMOTION_BROWSER ?? (existsSync(pw)
  ? readdirSync(pw).filter((d) => d.startsWith("chromium_headless_shell-")).sort().reverse()
      .map((d) => path.join(pw, d, "chrome-headless-shell-linux64/chrome-headless-shell")).find((p) => existsSync(p))
  : undefined);
if (found) Config.setBrowserExecutable(found);
Config.setVideoImageFormat("jpeg");
Config.setConcurrency(2);
