# T201 — "Edited since its last bake": an honest staleness signal per concert (server half)

Owner: web-core. Spec: Fable, 2026-10-21. Pairs with A92 (the app's Home list).

## Why (VLL, 2026-10-21)

> "I often have a rebake on home screen, I don't understand how it is picked, I have it even when nothing seems to
> have changed" … "I would probably like a list of modified non archived playlists (including 1 song changed)"

**Today there is no staleness signal.**
- The manifest's per-song `rev` is the source revision **at bake**.
- `BakedSong.SongRev` is a constant 1 (`baker.go`).

So no client can tell whether a concert was edited after its last bake. Home's A42② row offers "Re-bake" to every
admin unconditionally (A42②(b): "no honest staleness signal"). VLL sees it when nothing changed.

## The property (the mechanism is the lane's call)

For each concert with at least one bake, the server answers **"is what would bake now different from the latest
bake?"** If so, it says what changed, and the answer is **true only if a re-bake would actually change the bundle's
inputs.**

**What counts** is exactly what the bake consumes, nothing more:
- **Setlist level:** song membership, order, intermissions and their labels, per-item overrides (key, on-call, …),
  name / date / venue *only if they are baked*.
- **Per song:**
  - the files that would bake (the set, the blob hashes, chart sources, crops, the default file);
  - each member's file selection and cues, if they are baked (they are: `memberPages`, `memberCues`);
  - annotations and layers on the baked files (the engine head);
  - baked song metadata (title, key, tempo, meter, display notes).

**What does not count:**
- a save that changes nothing;
- edits to songs that are not in the setlist;
- archived concerts (never reported);
- anything the bake ignores.

**Recommended mechanism, not binding:**
- At bake, write a small **input fingerprint** into the bake's sidecar (next to `page-sources.json`), per song and for
  the setlist. Build it with the SAME code that gathers the bake inputs, so the two can't drift.
- At query time, compute the fingerprint of the current inputs and compare.
- Comparing two fingerprints from one function avoids hand-listing "what can change" (memory: hand-maintained
  enumerations rot).
- Old bakes without a fingerprint read as **unknown**, not as edited. A re-bake writes one.

## API

- **The concerts listing** (`GET /api/bands/{b}/concerts`, what the app already reads) gains, per concert:
  - `editedSinceBake`: `true | false | null`, where null means unknown (a pre-T201 bake);
  - `changes`: `{songs: [{songId, title, what: "content"|"added"|"removed"|"moved"}], setlist: bool}`.
- Admins see the details. Members get the boolean only, or nothing; your call, but no leak of other members'
  selections.
- **Studio (web), optional but cheap:** an "Edited since last bake" tag on the setlist list and detail, next to the
  bake button.

## Tests (each cause flips it; a no-op does not)

- Each cause in "What counts" → `true`, with the right song and `what`.
- A save that writes identical content → stays `false`. This is THE regression VLL complained about.
- A re-bake → `false`.
- An edit to a song NOT in the setlist → `false` for that concert.
- An archived concert is not reported.
- A pre-T201 bake → `null`.
- **Positive control:** the fingerprint function, given the bake's actual inputs, matches what the bake wrote.

## Out of scope

Auto-bake. Live mode already covers it during rehearsals.
