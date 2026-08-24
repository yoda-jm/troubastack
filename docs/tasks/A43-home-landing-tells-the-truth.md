# A43 — The Home landing must not claim "Up to date" when it doesn't know, or when you have nothing

**Priority:** normal, but it guards a pre-gig failure · **Size:** S · **Area:** `app/` (Mobile lane).
From Mobile's gate question (`85a1bdd`), itself from VLL: *"if I delete the latest bake I downloaded, is
the 'Up to date' on Home still there?"*

## 1. The finding is broader than the reported case

Mobile traced the reported behaviour correctly (verified against `origin/main`): the landing computes
from `UpdateOffered` only, `diff()` classifies a not-on-disk concert as `NewlyAvailable`, and the landing
filters those out — so a deleted concert leaves the landing's radar.

But the same line produces **three** states that all render "Up to date", and the other two are worse:

| State | What the user has | What Home says |
|---|---|---|
| A concert deleted | the rest of their set | "Up to date" |
| **Nothing installed** (fresh install, cleared storage, deleted everything) | **nothing at all** | **"Up to date"** |
| **Manifest failed to load** (`manifest == null` → `UpToDate`) | unknown — it couldn't check | **"Up to date"** |

The empty-device case is the one that matters: a player reinstalls the app or clears storage, connects,
glances at Home before the gig, reads "Up to date", and walks on stage with **no charts**. The
manifest-null case is the same defect in a different disguise — the app couldn't check, and says the one
thing that means "I checked, you're fine."

## 2. Ruling

**Mobile's option (C) as the base, plus a narrowly-scoped (B). Blanket (B) is rejected.**

Their instinct in (A) is right and I'm preserving it: re-offering one concert you deliberately deleted,
while you still hold the other ten, **is** nagware. That is not what this fixes.

1. **Narrow the reassurance (C).** The landing's positive state speaks only about what you have —
   "Nothing to update" or equivalent. It must stop asserting completeness it cannot verify. Exact
   wording is Mobile's call; the constraint is that it must not read as "you have everything."
2. **The empty state is not a reassurance (B, bounded).** When the band's manifest lists concerts and
   **zero** are installed, the landing must say so and offer the download. "Up to date" with an empty
   device is false by any reading, and this is the only case where surfacing `NewlyAvailable` on the
   landing is warranted.
3. **Unknown is not "up to date".** When the manifest can't be fetched, the landing must not claim
   currency. Say nothing (Hidden) or say it couldn't check — Mobile's call — but the honest options are
   silence or ignorance, never a green light. Keep the existing intent of not nagging on a transient
   failure.

Everything else stays: `NewlyAvailable` remains a **Manage-screen** affordance, and a partial set with
one deleted concert still reads as "nothing to update".

## 3. Why this shape

The landing is a **pre-gig glance**. Its job is to answer "am I ready?" in one look. A label that
overstates what the app knows is worse than no label, because it converts an unknown into a false
assurance at exactly the moment nobody has time to check. The distinction that matters is not
*update vs. download* — it's **"I checked and you're fine"** vs **"I don't know"** vs **"you have
nothing."** Those must not share a rendering.

## 4. Acceptance criteria

- **Zero installed + a non-empty manifest ⇒ the landing offers the download**, not a reassurance. Its own
  test.
- **Manifest unavailable ⇒ no currency claim.** Its own test, asserting the reassuring state is *not*
  produced.
- **One concert deleted, others installed and current ⇒ still the quiet state** — this is the nagware
  guard, and it must be a test so a later change can't turn the landing into a re-download nag.
- An installed-but-stale concert still produces the update affordance (A39 unregressed).
- Tests are **pure state-mapping tests** off a fake manifest + fake installed set — no device needed.
- `:shared:check` + APK + **`:shared:compileKotlinIosSimulatorArm64`** (neither of the first two covers
  the iOS klib).
- Device pass on the empty-install case: clear storage, connect, confirm the landing does not say you're
  current. **Per A39's lesson, this state is not waivable** — it is the whole point of the task.

## 5. Out of scope

Auto-downloading anything; changing `diff()`'s classification; the Manage screen; A42's progress work.
