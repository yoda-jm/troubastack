# B03 — Distribution: in-app downloads, offers, freeze (I13)

**Priority:** B-track 3 (after B02) · **Size:** L · **Area:** `app/shared`, `app/androidApp`, `core/internal/httpapi`

## Context

With B02, bundles exist server-side and the app can import files manually. This task
makes distribution first-class per invariant **I13**: the app fetches a cheap manifest,
surfaces **offers** ("New version of X", "Y is available"), and applies them **only on
explicit user action**, atomically. `distribution/Updates.kt` has carried the design
(policy/freeze/availability types) since the scaffold — now it gets bodies.

**Design decision made here: app auth = the existing session login.** The app gets a
minimal "Connect" flow (server URL — already persisted by A06 — plus username/password)
hitting the existing `POST /api/auth/login`; the session cookie is persisted via the
Storage seam. **This is the moment the A05 secrets caveat comes due**: before a session
token is stored, `getSecret`/`putSecret` must move to `EncryptedSharedPreferences` (the
androidx.security.crypto dependency the catalog already anticipates) — that hardening is
part of this task, not a note.

Presenter sanctity (I12): everything here lives in `distribution/` + the concerts-list
UI. The `stage/` package keeps its no-network gate — offers appear in the list, never
mid-performance.

## Changes

1. **Server** — B02 already added list/download endpoints; extend the list response to
   the full `AvailableConcerts` shape from `bundle.proto` (per-song revs for future
   granular updates; `final_locked` passthrough). Keep it a few KB.
2. **App networking**: add ktor-client (okhttp engine, androidMain; the catalog comment
   anticipated it) with a cookie storage bridged to the Storage seam. `Connect` screen +
   sign-out; unauthenticated ⇒ concerts list simply shows local bundles (offline-first —
   Stage never requires an account, I12).
3. **`Updates` implementation** (commonMain, unit-testable with a fake transport):
   - `fetchManifest()` → typed `AvailableConcerts` (reuse A02's Kotlin mirrors).
   - `diff(manifest)` vs installed bundles (read each installed dir's `bundle.json`
     `concertRev` via the A02 loader): `UpdateOffered` / `NewlyAvailable`; suppress
     offers for `FROZEN` policy, `LocalPin`, or server `final_locked` per I13.
   - `apply(offer)`: download `.tstage` to `tempDir()` (stream, no full-file in memory),
     then hand to **A05's `BundleImporter`** — the atomic swap is already built and
     tested; do not reimplement it.
   - Policy store: per-concert `UpdatePolicy` + optional `LocalPin`, persisted as a small
     JSON via Storage. **`AUTO` stays inert in this task** (P201 wires it, transiently,
     for rehearsal — I13 says it must never persist; leave the enum + a pointer).
4. **UI**: offer chips on the concerts list ("Update to rev 3" / "New: Saturday @ The
   Anchor — Download"), per-concert overflow menu (Freeze/Unfreeze, Pin this version),
   progress + failure states (a failed download must leave the old bundle untouched —
   the importer already guarantees it; surface the reason).
5. **Tests**: commonTest for `diff` (offered/newly/frozen/pinned/final-locked matrix)
   and for apply-failure leaving state intact (fake transport + fake fs); Go test for
   the manifest endpoint shape.

## Acceptance criteria

- End-to-end on emulator against `make demo` + a B02 bake: Connect as `marie` → concert
  appears as *available* → Download → performs offline; re-bake server-side → *update
  offered* → apply → new rev performs; **airplane-mode Stage never notices any of it**.
- Freeze and LocalPin each suppress the offer (unit-tested + one manual check).
- Secrets hardening landed: session cookie lives in EncryptedSharedPreferences;
  `grep` shows no plaintext-prefs writes of it.
- No network imports in `stage/` (the A04 gate still passes verbatim).
- `:shared:check` + `assembleDebug` + `make test` green; I15 intact (ktor is commonMain/
  androidMain *dependency*, not a new seam; the six actual files remain the only
  platform code).

## Out of scope

- Auto-update / rehearsal live mode (P201); per-song granular apply (`SongChanged` —
  keep the type, file a follow-up when someone wants it); iOS actuals (IOS track);
  push notifications.
