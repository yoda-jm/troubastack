# A05 — Android Storage actual + `.tstage` bundle import

**Priority:** A-track 5 (after A04) · **Size:** M · **Area:** `app/shared` seams + androidMain, `app/androidApp`

## Context

A04 ships the demo fixture in app assets. Real usage needs bundles to arrive from
outside: this task implements **seam 3 (Storage)** for Android and a manual import flow
(pick a `.tstage` file → it appears in the concerts list). This keeps TroubaStage usable
end-to-end *without* waiting for the server-side bake/distribution pipeline.

The Storage seam contract is already sketched in
`app/shared/src/commonMain/kotlin/com/troubashare/shared/seams/Storage.kt` and its
Android actual (which names the concrete APIs). The `.tstage` container is specified in
`docs/design/08-bundle-container.md` (A02). Import follows the atomic-swap discipline of
invariant I13: **a half-imported bundle must never exist** where the presenter can see it.

## Changes

1. **Storage actual (androidMain)** — implement the existing methods:
   - `bundlesDir()` = `File(context.filesDir, "bundles").path` (created on demand);
     `tempDir()` = `context.cacheDir.path`. Constructor takes an
     `android.content.Context` (application context, passed from `androidApp`).
   - `getSecret`/`putSecret`: nothing needs secrets yet. Implement over plain
     `SharedPreferences` with a doc comment: *"holds nothing sensitive yet; harden
     (EncryptedSharedPreferences/Keychain-class storage) before auth tokens land."* Do
     not pull in extra crypto dependencies for an unused path.
2. **Extend the seam minimally for import** (this stays within seam 3 — it is "the
   where/how of bytes on disk", per the seam's own doc):
   - `expect fun unpackBundle(zipPath: String, destDir: String): UnpackResult` (sealed
     Ok/Failed(reason) — same never-throw discipline as A02's loader).
   - Android actual uses `java.util.zip.ZipInputStream`. **Zip-slip guard is mandatory**:
     resolve each entry against `destDir` and reject any entry whose canonical path
     escapes it (`..`, absolute paths). Reject zips > a sane cap (e.g. 512 MB) and
     entries that would exceed it (zip-bomb guard).
   - Add matching `TODO("iOS-later")` stubs to the iOS actual so both platforms keep
     compiling.
3. **Atomic import (shared code, commonMain)** — `BundleImporter` in `bundle/`:
   unpack into `tempDir()/import-<random>/`, validate by running A02's `BundleLoader`
   on it (a `Failed` load ⇒ delete temp, report the loader's reason), then move the
   validated directory to `bundlesDir()/<concertId>/` — **rename, not copy**; if a bundle
   with that id already exists, rename it aside first and delete it only after the new
   one is in place (that's the I13 swap; an interrupted import leaves either the old
   bundle or none, never a broken one).
4. **Concerts list (androidApp + shared)** — replace A04's asset stopgap: list
   `bundlesDir()` contents (each entry loaded lazily for its name/rev via the loader —
   a directory that fails to load is shown as "damaged bundle" with a delete action, not
   hidden and not crashing), an **Import** button launching the system file picker
   (Storage Access Framework, `ACTION_OPEN_DOCUMENT`, `application/zip` + `*/*` filter —
   `.tstage` has no registered MIME type), copy the picked stream to `tempDir()`, then
   run the importer. Show success/failure as a plain message.
5. **Tests:** commonTest for `BundleImporter` (fake files: valid import lands under the
   concert id; invalid bundle never appears in bundles dir; re-import same id swaps
   cleanly). AndroidTest or a JVM test for the zip-slip guard: a crafted entry
   `../../evil.txt` must be rejected. (Craft the malicious zip in the test with
   `ZipOutputStream` — do not commit a malicious binary fixture.)

## Acceptance criteria

- The whole path — app launch → concerts list → import → Stage — works **offline and
  with no login/account/unlock of any kind** (I12; auth exists only in the Studio
  webview and, later, the downloader).
- On an emulator: `adb push demo.tstage /sdcard/Download/`, Import → pick it → it
  appears in the concerts list and opens in Stage. Import a torture `.tstage`
  (`bad-json` variant zipped) → clear error message, nothing appears in the list,
  nothing crashes.
- Kill the app mid-import (or simulate by making unpack fail halfway in a test): the
  bundles dir contains either the old state or the complete new bundle — never a partial.
- Zip-slip test passes; `./gradlew :shared:check :androidApp:assembleDebug` green; the
  platform-code footprint is still only the seam files (I15) plus `androidApp`.

## Out of scope

- Network download, update offers/freeze UI (blocked on the server-side bake +
  distribution endpoints — see the A-track notes in README), iOS actuals beyond compiling
  stubs, MIME/file-association registration.
