# T96 — Bake progress: "song 3 of 11", observable while it runs

**Priority:** normal — a bake is the longest thing in the product and it is currently silent ·
**Size:** S (core), UI deferred · **Area:** `core/internal/bake`, `core/internal/httpapi`. Lane: Web & Core.
**Core first, per VLL** — the studio/app surface is a follow-up, not part of this task.
**Sequenced AFTER T97** (bake performance): VLL reports a bake is slow even for a small concert, and
T97 found a structural reason (two external processes per song, one of them unconditional). Fix the
cost before building a readout around it — otherwise the progress UI is designed against timings that
are about to change.

VLL, 2026-08-23: *"having a progress while baking could be nice (status update on the song number of
the total, or something like that (should be core lane first)"*.

## 1. Where it stands

The bake is a **blocking POST**: `bakeapi.go:32` routes
`POST /api/bands/{bandId}/setlists/{setlistId}/bake` straight into `a.baker.Bake(...)` (`:110`), which
returns the finished bundle. While it runs there is no channel of any kind — the client is holding one
request open and gets a single answer at the end.

The good news is that the number VLL wants already exists at the point it is needed. `baker.go:164`:

```go
for si, item := range detail.Items {
    song, berr := b.bakeSong(ctx, si, bandID, actor, item, blobsDir, layerDefaults)
```

`si`, `len(detail.Items)` and `item`'s title are all in hand. Nothing needs to be computed — it needs
to be *published*.

## 2. Shape: a side-channel, not a contract change

Three options were considered; take the third.

- **Job model** (POST returns 202 + id, client polls) — cleanest long-term, but it changes the bake
  contract and every caller, for a progress readout. Too much for the first slice.
- **Streaming response** (NDJSON progress then the bundle) — no new endpoint, but every client's JSON
  decode changes.
- **✅ A progress registry + a read endpoint.** `POST …/bake` keeps its exact current contract —
  same request, same response, same status. It additionally publishes progress to an in-process
  registry, and a new `GET` exposes it. A client polls *while its own POST is in flight*; a client that
  doesn't poll is unaffected. **Nothing breaks if the UI never adopts it.**

## 3. What to build

1. **A progress sink on the Baker.** `Bake` takes (or the Baker holds) a callback invoked once per
   song — before `bakeSong`, so the reported song is the one being worked on, not the one just
   finished — carrying: bake id, `done`/`total`, the current song's title, and a terminal
   `state` (running / succeeded / failed).
   Keep `Bake`'s signature change minimal and keep the callback **non-blocking**: a slow observer must
   never slow a bake.
2. **A bake id, returned from the POST.** Generate it at the start of `Bake` and return it to the
   caller (a response header is fine, e.g. `X-Trouba-Bake-Id`, or a field alongside the bundle).
   **Do not key progress by setlist id.** B08/B09 established that concurrent bakes of the *same*
   setlist are legal and must produce distinct revs — so a setlist key is exactly the thing already
   proven not to be unique, and two simultaneous bakes would overwrite each other's progress.
3. **`GET /api/bands/{bandId}/setlists/{setlistId}/bakes/{bakeId}/progress`** → `{state, done, total,
   song, error}`. Same authorisation as the bake itself (do not invent a new access rule — reuse
   `authed` and the band check). Unknown/expired id → 404, not an empty 200: "no progress" and "I lost
   your progress" are different answers.
4. **Bound the registry.** In-memory, and entries **must expire** — a terminal state retained briefly
   (a few minutes) so a poller can observe the ending, then evicted. State this rule in code; an
   unbounded map keyed by bake id is a leak that only shows up on a long-lived server.

## 4. The things most likely to go wrong — address them, don't discover them

- **Races.** A registry written by bake goroutines and read by HTTP handlers is shared mutable state.
  `TestBake_ConcurrentSameSetlist_distinctRevs` already exercises concurrent bakes and is a **known
  intermittent failure** with a real race behind it in `baker.go` — this task adds writes on that exact
  path. **Run the concurrent bake test under `-race`** and make that part of the acceptance, not an
  afterthought. If `-race` surfaces the pre-existing race, say so at the gate rather than working
  around it; it may be the same bug and worth fixing here.
- **The empty setlist.** `total == 0` must not report "song 1 of 0". Decide and test it: succeeded
  immediately, with `done == total == 0`.
- **Failure names the song.** The loop already wraps errors as `song %s: %w`. The terminal failed state
  must carry which song failed — that is the whole value of progress when something goes wrong.
- **Cancellation.** `Bake` takes a `ctx`; if the client disconnects, the bake may be cancelled. The
  terminal state must not be left `running` forever — that is the same trap as the A39 stall, where a
  UI sat on a state nothing would ever resolve.

## 5. Granularity: per song, and why not finer

Rasterising pages dominates a bake, so per-*page* progress would be smoother. **Per song is still the
right unit for this task:** it is what VLL asked for, it is meaningful to a human ("Sat @ The Anchor,
song 3 of 11" tells you where you are), and it needs no new plumbing inside `bakeSong`. If a long song
makes a single step feel stuck, per-page is a follow-up inside `bakeSong` — do not pre-build it.

## 6. Acceptance criteria

- A bake of an N-song setlist publishes N progress updates with `done` advancing 1..N and `total == N`,
  each naming the song **about to be** baked. Assert the sequence, not just the final value.
- Terminal states: **succeeded** after the last song; **failed** carrying the offending song's name;
  neither left `running`.
- Empty setlist → succeeded, `done == total == 0`, no per-song update.
- **Two concurrent bakes of the same setlist keep separate progress** — distinct bake ids, neither
  observing the other's counter. This is the assertion that makes the bake-id decision load-bearing.
- `GET …/progress` on an unknown or expired id → **404**.
- Authorisation: a member of another band cannot read a bake's progress.
- **`go test -race ./internal/bake/ ./internal/httpapi/` green**, including the existing concurrent
  same-setlist test.
- The existing bake POST is **byte-for-byte unchanged** in request and response — prove it by the
  existing bake tests passing untouched, and say so at the gate.
- `gofmt -l core` clean.

## 7. Out of scope

- Any UI. The studio bake dialog and the app are a follow-up once the endpoint exists.
- Per-page progress (§5).
- Turning the bake into a background job, or persisting progress across a server restart. In-memory is
  correct here: progress is only interesting while someone is waiting.

## 8. Implementation decisions (build-time, per Fable's ef024ea)

- **Anchor (T98 moved it).** T98 split the per-song loop into `stageSong` (phase 1) → one
  `RenderBatch` → `assembleSong` (phase 3). §3.1's "before `bakeSong`" now names the **phase-1 stage
  loop**: `done` advances 1..N there, publishing before each song's (poppler-dominant) work.
- **`done == total` does NOT mean finished — the state field does.** T98 gave the bake a tail (one
  RenderBatch + assembly, ~2.4s of ~13.5s). The per-song counter reaches N-of-N while that tail still
  runs. `succeeded` is published ONLY from a deferred terminal step, never at the end of phase 1, so
  state stays `running` through the tail. Chosen signal for the tail: a `running` update with
  `done == total` and an **empty `song`** ("finishing"), rather than adding a phase field — the wire
  shape stays `{state,done,total,song,error}`. `song` is the song being baked in phase 1, and empty
  during the finalize; a UI reads "finishing", not "still baking song N". The sequence test asserts
  this song-less `done == total` running update precedes the terminal.
- **Auth: admin-only** (mirrors the bake, which is admin-only I11) via the same `GetBand` check; the
  registry is additionally band+setlist scoped so a cross-band admin can't read another band's bake id.
