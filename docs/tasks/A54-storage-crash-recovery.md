# A54 — A device that cannot open its own secrets must recover, not brick

**Lane:** Mobile · **Severity:** VLL, on hardware — *"the crash was not acceptable"* · **Verified against `3386b58`**
**Files:** `app/shared/src/androidMain/kotlin/com/troubashare/shared/seams/Storage.kt`,
`app/androidApp/src/main/AndroidManifest.xml` (+ a backup rule), and the pure seam's test.

## The defect, verified at source

`Storage.kt:25-36` builds the encrypted prefs in a `by lazy` with **no `try`/`catch`**:

```kotlin
private val prefs by lazy {
    val masterKey = MasterKey.Builder(context)…build()
    EncryptedSharedPreferences.create(context, "troubashare.secrets.enc", masterKey, …)
}
```

If `create` throws — the KeyStore master key cannot decrypt the existing `troubashare.secrets.enc` — the
exception propagates to the **first** `getSecret`, which is the theme read at `MainActivity.kt:122`, during
composition. **The app crashes on every launch, with no route out from the UI.** The lane hit exactly this
on the tablet; `pm clear` was the only way back.

## Why this is not just a test artifact

**`android:allowBackup="true"` (`AndroidManifest.xml:16`).** Android therefore backs up `secrets.enc` and
can restore it onto a device whose KeyStore holds **no matching master key** — a guaranteed crash-loop on
first launch, for a real person, on a new phone. The same failure occurs when an OEM invalidates KeyStore
keys on a lock-screen credential change.

Untouched by A51–A53 (this code dates to the B03/A05 era), so **pre-existing** — but reachable in the
field, and reachable on the machine a set is played from.

## What is actually at risk — this shapes the fix

The encrypted prefs hold the session cookie, the server URL, the last username, the theme, the Stage
reading/colour modes, the last concert, and the distribution policies. **Concerts are files on disk and are
not in this store.** So the worst case of discarding it is *"signed out, settings reset"* — never lost music.

## Deliverable

### 1. Open-or-heal, with the failure injectable so it can be tested

Wrap the creation. On a `GeneralSecurityException` / `IOException`: **delete `troubashare.secrets.enc` and
the master key, then retry once.** If the retry also fails, fail into a state the UI can render — never an
uncaught throw reaching composition.

**Make the creator a constructor-injectable lambda** (default: the real `EncryptedSharedPreferences.create`)
so a test can supply one that throws the first time and succeeds the second. That is the whole point:
today this path is unreachable from any test, which is why it shipped.

### 2. The ruling you asked for: self-heal, then say so — do not ask permission first

VLL's words were *"I need to delete your local data to restore … , Exit / OK"*. **Implement the recovery,
but as an after-the-fact notice rather than a blocking pre-flight prompt** — a non-modal line on Home:
*"Your saved settings were reset and you'll need to sign in again."*

Reasoning, so it can be overruled knowingly: a permission dialog is the right shape when the alternative to
consent is *keeping* the data. Here the alternative is a **brick** — declining leaves the app unusable — so
the prompt asks a question with only one real answer, and it asks it at launch, which is exactly when
someone is walking on stage. And the thing being discarded is settings and a session, not their music.
**Being honest afterwards satisfies "make it clear"; blocking first does not make it safer.**

**✅ SETTLED — VLL, 2026-08-29: *"ok for the self-heal, keep going."*** This diverged from his literal
wording ("Exit / OK" prompt) and he has confirmed the divergence. **Build the self-heal plus the
after-the-fact notice; do not add a blocking pre-flight dialog.** No longer an open question.

### 3. Stop the keyless blob arriving in the first place

Add a backup rule that **excludes `troubashare.secrets.enc`** from backup/transfer (`data_extraction_rules`
plus the legacy `full_backup_content` for older API levels), rather than turning `allowBackup` off wholesale.

**Both, not either.** The exclusion removes the restore trigger; it does **not** cover KeyStore invalidation
on the same device, so the try/catch is required regardless.

### 4. Tests

- the injected creator throws once ⇒ the store recovers, the file is gone, and reads work;
- it throws **always** ⇒ a recoverable failure state, **no exception escapes**;
- the happy path is unchanged (no deletion when `create` succeeds).

## Teeth-check

Make the recovery path retry **without deleting** the corrupt file. A named test must redden. Report the
count. Also state whether any existing test covered this path before (I expect: none — say so explicitly,
it is the finding).

## Out of scope

Migrating or preserving the old prefs · encrypting anything new · the bundles directory · iOS.
