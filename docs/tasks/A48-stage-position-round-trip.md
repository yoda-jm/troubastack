# A48 — the Stage position round-trip is the one untested seam in A46

**Priority:** normal · **Size:** XS–S · **Area:** `app/androidApp` (Mobile lane).
Follow-on to **A46** (`a889c4b`), unblocked by **A47** (`:androidApp` now has a test source set).

## 1. Measured today

A46 persists the reading position as a single string and reads it back on open. The encode lives in
`MainActivity.kt` (`"$s#$p"`, in the `onPositionChange` lambda) and the decode a few lines above it:

```kotlin
(if (concertId.isEmpty()) null else storage.getSecret(posKey))
    ?.split('#', limit = 2)?.takeIf { it.size == 2 }
```

`resolveStartPage` — the part that maps a logical position to a page index — **is** covered, by
`StagePositionTest` (6 tests, both teeth-checked at the gate). The **string round-trip is not.** It was
untestable when A46 landed because the module had no test source set; A47 removed that reason.

This is the seam that runs **at app open, on data written by a previous install**. That is the one place
where a malformed value is not hypothetical.

## 2. Why this is worth a task

The decode indexes `[1]` after a `takeIf { it.size == 2 }`. Drop or weaken that guard and a stored value
with no `#` — a truncated write, or a value from a future/older encoding — makes `get(1)` throw
**at composition time, on launch, before the Stage renders**. The performer's failure mode is the app
dying on open at a gig, and the current guard is the only thing preventing it.

That is exactly the T110/A47 bar: code whose wrong answer is silent or catastrophic, and which nothing
executes.

## 3. What to build

**(a) Extract the encode and decode into named pure functions** in the app module — the A47 pattern
(`sessionCookieFor` taking `getSecret`). Something like `encodeStagePosition(songId, pageInSong): String`
and `decodeStagePosition(raw: String?): Pair<String, Int>?`. Both call sites use them; behaviour identical.

**(b) A suite covering the round trip and, above all, what it REJECTS.**

## 4. Rules

- **Per-test teeth-check, reported** — the wrong implementation you tried and that it reddened. Same
  standard as A47.
- **The load-bearing test is the malformed-input one.** Removing the `size == 2` guard must redden it,
  and the assertion must be that the decode *returns a fallback*, not that it throws. Verify the
  catastrophic direction yourself: with the guard removed, the naive decode throws.
- **Pin the known-safe degradations** rather than leaving them to chance. At minimum: no separator; a
  non-numeric page; an empty stored value; and a `#` **inside** the songId (today this decodes to a null
  page and falls back to the top — safe, and it should stay a *decision*, not an accident).
- **Round-trip property:** `decode(encode(id, n)) == (id, n)` for ordinary ids.
- **Read the test-results XML, not the exit code.** Two variants double the counts.
- **Use generic ids in tests** (`song-1`, a UUID) — no real band data in tracked files.

## 5. Acceptance criteria

- Encode and decode are named, testable functions; both A46 call sites go through them; behaviour
  unchanged (a signature change makes a missed call site a compile error — prefer that).
- The malformed-input tests exist and the guard-removal mutation reddens them.
- `:androidApp:test` and `:shared:check` green; counts read from XML.
- No change to `resolveStartPage` or to A46's persisted format — this task adds a seam and its tests,
  and must not silently migrate stored values.

## 6. Out of scope

Changing the encoding (if you think `#` is the wrong separator, raise it — **do not** change it here; a
format change strands every already-persisted position). Compose UI tests. Storage/Robolectric rigs —
keep to pure functions.
