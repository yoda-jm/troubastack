// T117 — make a retried pass VISIBLE. Reads Playwright's JSON report (web/studio/playwright-report.json,
// written only on CI) and emits a GitHub ::warning:: naming every test that PASSED ONLY ON RETRY
// (Playwright status "flaky"). A flaky green is not a passing test — this keeps an otherwise-green run
// from silently swallowing it. A name that recurs across runs is a TASK, not a note (the retry exists
// for infra blips, not to quiet a real flake). Never fails the build — retries exist so a blip doesn't
// red the push; a hard gate here would just re-create that problem.
import { readFileSync } from "node:fs";

let report;
try {
  report = JSON.parse(readFileSync("playwright-report.json", "utf8"));
} catch {
  // No report (suite didn't run, or not CI so the json reporter was off) — nothing to say.
  process.exit(0);
}

const flaky = [];
const walk = (suite) => {
  for (const spec of suite.specs ?? []) {
    for (const t of spec.tests ?? []) {
      if (t.status === "flaky") flaky.push(`${spec.file}:${spec.line} › ${spec.title}`);
    }
  }
  for (const child of suite.suites ?? []) walk(child);
};
for (const s of report.suites ?? []) walk(s);

if (flaky.length) {
  console.log(
    `::warning title=Flaky tests (passed only on retry)::${flaky.length} flaky — a name recurring across runs is a bug to file, not noise: ${flaky.join(" | ")}`,
  );
  for (const f of flaky) console.log(`  flaky: ${f}`);
} else {
  console.log("flaky-warn: no flaky tests in this run.");
}
