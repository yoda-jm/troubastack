# BRAND02 — retire the TroubaShare name

**Status:** done, landed with the change itself. This file is the record, not a plan.
**Raised by:** VLL, on reading the project page — "il y a des references a troubashare a
certains endroits ?"

## Why

The repository is `troubastack`, the brand family is TroubaStack / TroubaStudio /
TroubaCore / TroubaStage, and the project page says TroubaStack. But `README.md` opened
with "One product (`TroubaShare`)", the launcher icon on the tablet said **TroubaShare**,
and 431 occurrences of the old name were spread over 163 tracked files. The page was not
inconsistent with itself — it was consistent with a name the rest of the project had not
adopted.

## The target names, and why these

| Thing | Now | Reasoning |
|---|---|---|
| The product | **TroubaStack** | the sum; what the repo and the page already call it |
| The mobile app | **TroubaStage** | the app *is* the presenter, and the brand family already names it |
| Package root | **`com.troubastack`** | namespace the product, keep the module names (`.app`, `.shared`) |
| APK / artifact base | **`troubastage`** | it is the app's artifact, not the product's |

The app is **not** called TroubaStack. That is the one substitution a global `sed` would
have got wrong in both directions: `TroubaShare` → TroubaStage everywhere *except* the
one line in `README.md` that names the product, which becomes TroubaStack.

## The four lockstep pairs

Each of these is two or more places that must move together. Renaming one side of any of
them leaves a build that compiles and a behaviour that is silently wrong — which is the
whole reason this was not a one-line command.

1. **`SECRETS_FILE` ↔ the backup rules.** `Storage.kt` names the encrypted prefs file;
   `backup_rules.xml` and `data_extraction_rules.xml` exclude that exact filename from
   cloud backup and device transfer. Rename the constant alone and the exclusion stops
   matching — the secrets start being backed up. **This one is a security regression, not
   a cosmetic bug.** All four moved to `troubastage.secrets.enc`.
2. **The WebView shell contract.** `bridge.ts` reads `window.TroubaStageShell` and defines
   `window.__troubastageShell`; Android registers `BRIDGE_NAME` via
   `addJavascriptInterface`; iOS installs a shim of the same name. Three sides, one name.
3. **The served APK.** `appsapi.go`'s allow-list holds the exact filename on disk
   (`troubastage.apk`) and the download basename; `deploy/README.md` tells the operator
   which filename to copy into `deploy/apps/`. A mismatch serves a 404 to the in-app
   "get the app" screen.
4. **The generator paths.** `core/cmd/gen-mirrors/main.go` and `web/ink/gen-glyphs.mjs`
   both build the path to a generated Kotlin file out of string components including the
   package directory. Miss either and CI's mirror/glyph guards go red, because the
   generator writes beside the moved file instead of onto it.

## Scope boundary — what was deliberately NOT renamed

Live surfaces were renamed: `app/`, `core/`, `web/`, `deploy/`, `.github/`, `README.md`.

**Historical records were left alone**: `docs/handoff/reviews.md`, already-delivered task
specs under `docs/tasks/`, and `docs/video/` (a script for a video that is already shot).
A review log records what was said at the time; rewriting it to match today's vocabulary
falsifies the record. Those files still say TroubaShare, correctly.

## ⚠ Consequence on installed devices — read before the next gig

`applicationId` changed from `com.troubashare.app` to `com.troubastack.app`. To Android
that is **a different application**, not an upgrade:

- the new APK **installs alongside** the old one rather than replacing it;
- it gets a **fresh `filesDir`**, so **downloaded concert bundles do not carry over**
  (`filesDir/bundles/<concertId>/`);
- the saved server cookie does not carry over either — the device re-registers;
- the old app stays on the device, under the old name, until it is uninstalled by hand.

The same applies on iOS via `PRODUCT_BUNDLE_IDENTIFIER`.

**Before the concert of 2026-09-05, on each performing device:** install the new APK,
uninstall the old TroubaShare, re-point it at the server, and **re-download the concert
bundle** — then verify offline, force-stopped, exactly as A58 specifies. Do not assume the
tablet is ready because it was ready last week.

A silver lining worth naming: because the app must be reinstalled anyway, the shell-bridge
skew (an *old* installed app loading a *new* studio, whose JS looks for a global the old
shell never registers) cannot bite anyone who follows the step above.

## Verification actually run

| Check | Result |
|---|---|
| Residual `troubashare` on live surfaces | **0** |
| `go vet ./...` | clean |
| `go test ./internal/httpapi ./internal/bake` | ok — 96.3s / 5.8s |
| `gofmt -l core` | clean |
| `:shared:testDebugUnitTest` | **302 run, 0 failed** — matches the known count, so the package move dropped no tests |
| `:androidApp:testDebugUnitTest` | 13 run, 0 failed |
| Mirror + glyph generators re-run | write to the new path; tree agrees |
| Secrets-file ↔ backup-rules pair | both sides read `troubastage.secrets.enc` |

## What was NOT verified, and why

- **The studio vitest suite did not run.** This worktree has no `node_modules`, and in this
  repo a worktree's `node_modules` is shared with `main` — installing here writes through
  and prunes the shared tree. Worth knowing anyway: **no unit test imports `bridge.ts`**,
  so that suite would not have covered the contract that changed.
- **The Playwright e2e did not run** — `get-app.spec.ts` is the only test that asserts the
  `/apps/troubastage.apk` path end to end. It should be run before this is relied on.
- **iOS was not built.** `project.yml` was edited (bundle id, display name, usage string);
  nothing on this machine can compile it.
- **Nothing was checked on hardware.** Device state is shared and goes stale; the last read
  of the tablet predates this change.
