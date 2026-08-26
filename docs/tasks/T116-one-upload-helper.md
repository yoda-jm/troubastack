# T116 — One upload helper, not twenty-seven

**Priority:** normal · **Size:** S · **Area:** `web/studio/e2e` (Web & Core lane).
The remainder T108 declared and T114 left. **Filed at VLL's request on 2026-08-27 — note that I ruled
"leave it" at T114's gate; he asked for the follow-up, so that ruling is superseded.**

## 1. What's actually there — measured today, not carried forward

Both T108 and T114 reported **47** local copies. **It is 27.** Measured on `origin/main`:

- **27 spec files define their own `uploadPdf`**; `setup-helpers.ts` already exports one (unused by them).
- Those 27 bodies reduce to **4 distinct forms**, and they are not evenly spread:

```
  20×  identical (the T36 "open the Details panel, upload, close" form)
   5×  identical (same, without the closing step)
   1×  editor-annotation-fileid.spec.ts
   1×  editor-song-details.spec.ts
```

**25 of 27 collapse into two forms.** That makes this materially smaller than "four variants" implied —
which is why the number is in the spec rather than the estimate.

## 2. What to build

Converge the 27 onto the exported helper. Give it whatever small parameter distinguishes the two
dominant forms (whether the panel is left open) rather than forking a second helper; the two singletons
either fit that shape or are named as deliberate exceptions.

The `PDF_PATH` / `fileURLToPath` cascade moves into the helper module with it.

## 3. Rules

- **Behaviour-neutral, and provable.** This is T108's shape: the assertion set of each migrated spec must
  reconcile — relocated into the helper, never dropped. Report the arithmetic the way T108 did
  (`N assertions left the specs, N accounted for in the helper`), because that is what makes a
  same-green refactor trustworthy rather than merely green.
- **A spec that is ABOUT uploading keeps driving the UI.** Same rule as T108's registration carve-out —
  and name which specs those are.
- **Don't fold in an API-driven upload.** That is a different change with a different failure mode
  (coverage, not correctness), and T114 already established it needs its own gate.
- Count reconciles at **199 e2e + 27 vitest**. If it moves, say why in the same sentence.

## 4. Acceptance criteria

- No spec defines its own `uploadPdf`; any deliberate exception is listed with its reason.
- Assertion reconciliation reported and closing.
- Full `make e2e` green, count reconciled.
- Before/after line count — this is a deletion task; the number is the point.

## 5. Out of scope

API-driven upload. `retries`/`workers` config (T117). Adding or changing assertions.
