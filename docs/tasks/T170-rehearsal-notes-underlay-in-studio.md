# T170 — Rehearsal notes reach Studio as a reference underlay, never as an annotation

**Lane:** web-core (core + studio), then a small **mobile slice** (§6) once the server side is on main.
**Size:** M (core + studio) + S (app). **Filed:** 2026-09-10 by the architect. This is **A70 Part B**
(`A70-rehearsal-notes-on-stage.md` §7), written out as its own task at VLL's request.

**Status: TAKEABLE.** VLL lifted the defer on 2026-09-11 (he is using notes and wants them in Studio); dispatched to web-core at the gate the same day. Read ⟨D1⟩ below before building §6.

Not in any lane's queue. It depends on A70 Part A being on main, and on VLL having used rehearsal notes
for a few rehearsals — if the notes turn out to live and die on the tablet, this is never built. It
becomes takeable only when a Fable entry in `docs/handoff/reviews.md` dispatches it after VLL says so.

## 1. What this is, and the one rule that shapes all of it

A rehearsal note (A70) is a per-page, per-device bitmap drawn on Stage. VLL's design principle, verbatim:

> *"the bitmap is especially done in order to be sure it is not mixed for an annotation, the fact that you
> cannot directly import it as a layer that you can do something is precisely for that (for example you
> print it between the pdf and the other layers and then you recopy manually in annotation layers the
> notes you took)."*

And the reason the friction is wanted (A70 §1.4): a note cannot follow a lyric change, cannot be shared,
and nags — every one of those is an argument for recopying it into a real annotation while it still means
something.

So this task moves pixels from the tablet to Studio and **prints them under the annotation layers as a
reference**. It does not create an object, an object type, a layer, or anything the bake can see. When the
musician has recopied, they remove the underlay. **If an implementation of this task makes the note
selectable, movable, bakeable, or visible to another member, it has implemented the wrong feature.**

## 2. What exists today

> **Every anchor below is a grep, not a line number.** Numbers rot: four of these moved in a single day
> (2026-09-10 → 09-11), one by ~250 lines, from ⟨D2⟩/⟨D4⟩ work in the same file, and a wrong hint costs a
> wrong jump plus doubt about every other line. Run the grep; it is a second and it never rots. What is
> load-bearing here is the STRUCTURE each anchor describes — if a grep finds nothing, the symbol was
> renamed and you want the new name, but if the *structure* is gone, stop and post it at the gate (Fable,
> `80ce7cca`).

- **Routing style** — one line per route, `mux.HandleFunc("METHOD /path", a.auth(handler))`. The song-file
  routes are the shape to copy, and the new ones go beside them:
  `git grep -n 'songs/{songId}/files' core/internal/httpapi/webapi.go`
- **The upload precedent** — multipart field `"file"`, member-only, guarded three ways
  (`http.MaxBytesReader`, `io.LimitReader`, a `len` check) against one `const` cap of 32 MiB:
  `git grep -n 'func (a \*WebAPI) uploadFile\|maxUploadBytes' core/internal/httpapi/webapi.go`
  **T141's rule** for any binary path: `Content-Length` on the way out is `len(data)`, never a stored field.
- **Blob store** — `app.Service` holds a `blob.Store`; `Put` returns the sha256, `Get`/`Delete` take it.
  Content-addressed, so the same PNG twice is one blob:
  `git grep -n 'blob.Store\|s.blobs.\(Put\|Get\|Delete\)' core/internal/app/service.go`
- **Records** — the `app.Repo` interface with **two** implementations, each a `Repo` type in its own
  package; a new record type touches the interface and both:
  `git grep -n 'type Repo interface' core/internal/app/app.go` ·
  `git grep -n 'func (r \*Repo) CreateSongFile' core/internal/app/filerepo core/internal/app/memrepo`
  `filerepo` persists one JSON document of maps (`Files map[string]app.SongFile` and its siblings) —
  `git grep -n 'map\[string\]app\.' core/internal/app/filerepo/filerepo.go`. The `storetest`-style
  parametrised suite is the pattern for testing both.
- **"Which bake is current"** — reads `<bakesDir>/<concertId>/<latestRev>/bundle.json` and returns the
  latest `ConcertBundle` per concert; `ConcertBundle.Songs[].Pages[].RasterHash` is the per-page hash the
  tablet keys notes by, and `concertId == setlistId`:
  `git grep -n 'func (b \*Baker) ListConcerts' core/internal/bake/baker.go`
- **Studio page stack — the load-bearing structural claim.** Inside each `.pdf-page`, in DOM order: the PDF
  raster (`<canvas className="pdf-canvas">`, or `<img className="pdf-canvas image-page">` for image files),
  then `<canvas className="annotation-overlay">` (the dry layer), then `<EditCanvas>` (wet + hit-testing).
  **The underlay goes between the first and the second**, in *both* stacks:
  `git grep -n 'pdf-page\|pdf-canvas\|annotation-overlay\|<EditCanvas' web/studio/src/pages/song-editor/Viewer.tsx`
  That ORDER has survived every change to this file; the two occurrences of the stack have not.
- **Studio API helpers** — a shared `upload<T>(path, FormData)` plus the `api.uploadFile` / `api.fileUrl`
  shapes to mirror: `git grep -n 'function upload<\|uploadFile:\|fileUrl:' web/studio/src/api.ts`
- **The app talks to core only through `HttpTransport.kt`** — `suspend fun` helpers over a session cookie;
  `reBake` is the POST precedent and there is **no multipart helper today**:
  `git grep -n 'suspend fun reBake' app/androidApp/src/main/kotlin/com/troubastack/app/HttpTransport.kt`
- **A70's note index entry** — the `NoteEntry` data class, mirrored verbatim into each concert's
  `files/notes/<concertId>/index.json` on the tablet:
  `git grep -n -A 14 'data class NoteEntry' app/shared/src/commonMain/kotlin/com/troubastack/shared/stage/notes/NoteModel.kt`
  It carries `{ songId, rasterHash, file, pageInSong, songTitle, bandName, concertRev, takenAs, width,
  height, updatedAt, sentAt?, bakesSinceTouched }`. `takenAs` is the Stage identity (a roster member id);
  `sentAt` is **set by the tablet on its own successful send**, never derived from the server. Two things
  this task must not assume (both checked against a real note on VLL's tablet, 2026-09-11 — see §2.1):
  there is **no `bandId`** and **no `concertId`** field, and `updatedAt` is **not a wall clock**.

### 2.1 Two facts from the real artefact, not from the model

A note VLL drew on 2026-09-11 (`files/notes/<concertId>/index.json` + one PNG), read off the tablet:

- **`updatedAt` is `SystemClock.elapsedRealtime()`** — boot-relative milliseconds, wired at
  `git grep -n 'monotonicNow = ' app/androidApp/src/main/kotlin/com/troubastack/app/MainActivity.kt` and
  consumed at `git grep -n 'updatedAt = monotonicNow' app/shared/src/commonMain/kotlin/com/troubastack/shared/stage/NotePad.kt`.
  The real note's value was `4564568383` — 52.8 days of uptime, not a date. It is **write-only today**
  (nothing on the tablet sorts or renders it; the nag counts bakes, not time), so no A70 behaviour depends
  on it — but §3.1's `CapturedAt`, §3.2's `capturedAt` field and §3.5's *"<date>"* have no true value to
  carry until §6 makes the tablet send a wall clock. See §3.2's note on the field.
  **Ruled** (Fable, `dd6e359c`): mobile switches it to a wall clock — but the field then holds **two eras**,
  because notes already on the tablet carry boot counters and nothing marks which is which. The
  discriminator is free and must be used: a boot counter is thousands of seconds, a wall clock ~1.7 × 10⁹,
  so a value below ~10⁸ is an old one and renders as **"date unknown"** — never as 1970, never as a
  plausible-looking wrong date. No migration: a date that was never recorded cannot be recovered, and
  inventing one is worse than admitting it. This branch is deliberate, not defensive.
- **`bandId` is absent.** An earlier draft of this section said it had been "added for this task,
  2026-09-10"; it was not, and is not on `main`. §6's PUT is addressed `/api/bands/{bandId}/…`, so the
  mobile slice has to obtain a band id (add the field, or resolve it from the concert) — it cannot read one
  out of the index. `concertId` is likewise not a field: it is the **directory name** the index lives under.
- Shape of the artefact, for §3.2's cap and §3.5's compositing: **1600×2261, 8-bit RGBA, 22,664 bytes**,
  **99.93 % fully transparent** (ink bbox 158×195 px). The matching page raster in the bake is 1241×1754
  greyscale — a uniform 1.289× scale, so §3.5's "absolutely positioned to the page box exactly as the
  overlay canvas is" needs no scaling code, and the alpha means the underlay shows the chart through it.

## 3. Decisions

### 3.1 Model — one note per (owner, song, page index)

```go
// core/internal/app — a reference image, NOT a domain.Object. No layer, no uuid in the annotation sense.
type RehearsalNote struct {
    ID          string    // server id
    BandID      string
    SongID      string
    OwnerUserID string    // the signed-in user who sent it — the only reader
    PageInSong  int       // 0-based, the page the note was drawn on, in the bake it came from
    RasterHash  string    // that page's raster hash in that bake
    ConcertID   string
    ConcertRev  uint64
    TakenAs     string    // the Stage identity on the tablet (roster member id), for the label only
    BlobHash    string    // content-addressed PNG
    Width, Height int
    CapturedAt  time.Time // when it was drawn — ZERO until §6 sends a wall clock (§2.1)
    UploadedAt  time.Time
}
```

Unique key **(OwnerUserID, SongID, PageInSong)** — *"if there is already a bitmap for this song/page it
asks to overwrite"* (VLL). Page **index**, not raster hash, is the key on purpose: the musician thinks
"page 2 of this song", and two sends after a reflow should collide and ask, not accumulate.

### 3.2 Endpoints (all `a.auth`, band membership required; owner-only beyond that)

| method | path | behaviour |
|---|---|---|
| `PUT` | `/api/bands/{b}/songs/{s}/rehearsal-notes/{page}` | multipart: `file` (PNG) + fields `rasterHash, concertId, concertRev, takenAs, width, height, capturedAt`. `capturedAt` is **optional** and, when present, RFC 3339 — the tablet has no wall clock for a note today (§2.1), so core stores the zero time and §3.5 shows the **upload** date until §6 provides one; it must never render a boot-relative number as a date. **409** if a note exists for (me, song, page) and `?overwrite=1` is absent. Sniff the bytes: `image/png` only. Cap **4 MiB** (a 1600-wide transparent PNG of handwriting is tens of KB; 4 MiB is generous and stops a mistake). |
| `GET` | `/api/bands/{b}/songs/{s}/rehearsal-notes` | **my** notes for the song, each with `pageChanged: true/false/unknown` (§3.4) |
| `GET` | `/api/bands/{b}/songs/{s}/rehearsal-notes/{page}` | the PNG bytes; `Content-Length = len(data)` (T141) |
| `DELETE` | `/api/bands/{b}/songs/{s}/rehearsal-notes/{page}` | *Done, remove*. Deletes the record; the blob is deleted only if no other note references the hash. |

Another member's notes are **404**, not 403 — their existence is not information a non-owner gets.

### 3.3 Identity — the sender is the owner, always

The tablet sends as the signed-in user; that user becomes `OwnerUserID`. `TakenAs` is stored for the
label only. The **prompt** when they differ lives on the tablet (§6), before the request is made. The
server does not try to map a roster member id to a user; it stores what it was told.

### 3.4 "This page isn't in the current bake"

For each note, compare `RasterHash` against the **current** bake of `ConcertID` (`Baker.ListConcerts()`
or a narrower `latest(concertID)` helper — add one rather than listing everything): the song's page at
`PageInSong` exists and has the same hash → `false`; exists with a different hash, or the page index no
longer exists → `true`; no bake on disk for that concert → `"unknown"`. This is a **label for the human**,
who decides whether the reference still helps. Nothing is re-placed, re-anchored or hidden on the strength
of it — that would be the T145 bug in a new coat.

### 3.5 Studio — an underlay, and a way to say "done"

- A chip **`Rehearsal notes (N)`** in the editor top bar, present only when `GET` returns N > 0 for the
  signed-in user. Toggles the underlay; the toggle is per song, in session, default **on** the first time
  notes exist (the musician sent them to see them).
- The underlay is an `<img className="rehearsal-underlay">` inserted **between `.pdf-canvas` and
  `.annotation-overlay`** in the page's stack (both stacks — §2's `Viewer.tsx` grep finds them), absolutely
  positioned to the page box exactly as the overlay canvas is, `pointer-events: none`, full opacity
  (VLL: no transparency), drawn at page `PageInSong`. A note whose page index exceeds the current page
  count is listed but not drawn (nowhere to draw it).
- Each note in the chip's popover: *"page P · from rev R · taken as X · <date>"*, where <date> is
  `capturedAt` when it is non-zero and the **upload** date otherwise, labelled as such (§2.1), a
  **`not in the current bake`** tag when §3.4 says so, and **Done, remove** → `DELETE` → the underlay
  disappears. No undo: it was a reference, and the tablet still has the original.
  The tag states the FACT and never the cause (Fable, `dd6e359c`): a re-encode, a re-render, an edit and a
  reflow are indistinguishable from a hash, and the label's first real mass firing was a RENDERER change —
  two consecutive bakes of one concert moved **every** raster hash (103.4 → 57.7 → 59.0 MB, T169 and its
  follow-up) without one page's content changing. *"The chart changed under your note"* would have been
  false at the exact moment it first mattered.
- **Nothing in `web/ink`, nothing in `web/bake`, no `ObjectType`, no proto change.** A source guard
  (§5.3) pins that.

### 3.6 Retention

Server notes belong to their owner and are removed only by *Done, remove*, by deleting the owner's
account, or by deleting the song. They are not baked, not exported in a band archive (`.tband`), not
part of any GC root. Sizes are small; no quota beyond the per-file cap.

## 4. Exact changes

### 4.1 Core

- `core/internal/app`: the `RehearsalNote` type (§3.1); `Repo` gains `CreateOrReplaceRehearsalNote`,
  `GetRehearsalNote(owner, song, page)`, `ListRehearsalNotes(owner, song)`, `DeleteRehearsalNote`,
  `CountRehearsalNotesByBlob(hash)` (for the blob-delete decision) — in the interface, `filerepo`
  (new map + persisted in the document), `memrepo`.
- `Service`: `PutRehearsalNote(caller, bandID, songID, page, meta, data, overwrite) (RehearsalNote,
  error)` — membership check (the same one `UploadSongFile` does), PNG sniff via `http.DetectContentType`
  (declared type ignored — the T141 family of bugs starts with trusting a field), size cap, blob `Put`,
  `ErrConflict` when exists and `!overwrite`; `ListRehearsalNotes` decorated with `pageChanged` through a
  narrow `bake` lookup interface injected like the anchorer is (`httpapi/anchorer.go` is the shape);
  `GetRehearsalNoteBytes`; `DeleteRehearsalNote` with the shared-blob rule.
- `httpapi`: the four routes (§3.2) beside the file routes (§2's `webapi.go` grep); JSON view `{ page,
  rasterHash, concertId, concertRev, takenAs, width, height, capturedAt, uploadedAt, pageChanged }`.
- `gofmt -l .` clean; `go vet`; the `-race` gate.

### 4.2 Studio

- `api.ts`: `listRehearsalNotes`, `rehearsalNoteUrl(page)`, `deleteRehearsalNote`.
- `Viewer.tsx`: the underlay `<img>` in both page stacks; the chip + popover; `data-testid`s:
  `rehearsal-notes-chip`, `rehearsal-underlay`, `rehearsal-note-row`, `rehearsal-note-done`,
  `rehearsal-note-changed`.
- `styles.css`: `.rehearsal-underlay { position:absolute; inset:0; pointer-events:none; }` at the page
  box, **no opacity rule**.
- The page count / zoom / re-raster machinery already re-lays the overlay canvas; the underlay follows the
  same box (it is the same absolute inset), so no new layout code.

### 4.3 Seed / demo

No seeded notes. The demo must not ship a rehearsal note: it is personal by definition.

## 5. Acceptance — RED FIRST

### 5.1 Core (`make test`, both repos through the parametrised suite)

| assertion | fails against |
|---|---|
| PUT by a band member stores the note; GET lists it; bytes round-trip byte-for-byte and `Content-Length == len(bytes)` | a `Size` field (T141) |
| PUT twice for the same (me, song, page) → **409**; with `?overwrite=1` → 200 and the old blob is gone when unreferenced, kept when another note shares the hash | overwrite by default / blob leak |
| Another member: their PUT creates **their own** note; my GET does not list it; their GET of mine → **404** | a per-song list without an owner filter |
| A non-member → 403 (the existing membership rule) | — |
| A JPEG with a `.png` name → 415; 4 MiB + 1 byte → 413 | trusting the declared type / no cap |
| `pageChanged`: fixture bake dir with the same hash → `false`; different hash → `true`; page index beyond the song → `true`; no bake → `"unknown"` | comparing by page index only |
| DELETE removes the record and the underlay disappears from the list; DELETE of a non-existent → 404 | — |
| A note is absent from a `.tband` export and from the bake's inputs (grep the bake batch request in a test) | — |

### 5.2 Studio (`make e2e`, new `rehearsal-notes.spec.ts`, set up through the API per T114)

- The chip is **absent** with no notes and **present** with one; the underlay `<img>` exists **before**
  `.annotation-overlay` in DOM order inside the same `.pdf-page`, and its bounding box equals the
  overlay's within 1 px.
- **Occlusion, measured, not `toBeVisible()`** (the lesson of `d70fdb14`): draw a freehand mark over the
  underlay; a pixel probe on the annotation overlay canvas finds the mark's colour where the underlay is
  opaque beneath it — the mark is above; and a pixel of the rendered page where only the underlay has ink
  shows the underlay's colour — it is above the PDF.
- The underlay is **not** hit-testable: clicking on underlay ink where no object is selects nothing.
- the `not in the current bake` tag shows for a note whose fixture hash differs; **Done, remove** deletes and the row
  and underlay are gone after reload.
- A second user signed in on the same song sees **no** chip.

### 5.3 Source guards

- `web/ink/src` and `web/bake/src` contain no reference to `rehearsal` (grep, in `web/bake/test`).
- `proto/` unchanged by this task (`git diff --stat origin/main -- proto/` empty at the gate).

## 6. The mobile slice (after §4 is on main — a separate small mobile task, filed then)

- `HttpTransport.putRehearsalNote(bandId, songId, page, png, meta, overwrite): SendResult` — a multipart
  PUT (first multipart helper in the transport; keep it minimal), mapping 200 → `Sent`, 409 → `Exists`,
  else `Failed(reason)`.
- Notes tab **Send to Studio** (A70 §5.3, currently disabled): for each note of the concert, orphans
  included:
  1. if `takenAs` ≠ the signed-in member for that band → prompt *"Taken as X · you are signed in as Y.
     Send as Y?"* Send / Cancel (once per batch, not per note);
  2. PUT; on `Exists` → prompt *"A note already exists in Studio for this song/page. Overwrite?"* → retry
     with overwrite;
  3. on `Sent` → `markSent(now)`; the row shows *"sent <date>"* and the `Notes ⚠` nag stops counting it.
- **Pure tests:** the send state machine with a fake transport — `sentAt` is set **only** on `Sent`
  (never on `Exists`, never on `Failed`, never before the call); the identity prompt fires iff
  `takenAs != me`; a batch with one failure leaves the other notes' `sentAt` untouched.
- Removal in Studio never reaches the tablet (VLL #9); the tablet's note lives until deleted there or
  with its bake (A70 §3.3).

## 7. Out of scope

Syncing notes between tablets · showing a note to any other member · any object type, ink or bake change
· auto-deleting the tablet's note after a send or after Studio's *Done, remove* · an opacity slider on the
underlay (VLL: no transparency) · re-placing or re-anchoring a note whose page changed.

---

## ⟨D1⟩ 2026-09-11 — VLL's two Notes-tab requirements, and the thing neither of them names

VLL, at the tablet with Part A in hand: the flat band → song·page list does not scale, he wants a
**collapsible tree** (band → concert → song → page), and he wants **bulk send at a node** — a whole song's
or a whole concert's notes in one action.

Mobile is right that these belong with the send design rather than after it: the tree decides *what the send
affordances hang off*. Folding them in, with the consequence that follows from bulk and that neither the
request nor the reply states:

### R1 — the tree

Band → concert → song → page, collapsible. Orphans are a node too (they have a concert and a last-known
song); they must not be stranded outside the tree just because their page is gone.

### R2 — bulk send is a PARTIAL-FAILURE operation, and that is its whole design

A per-note send has two outcomes. **A node send has thirty**, and the interesting cases are the middle:

- **Some succeed, some fail.** The result is not "sent" or "failed" — it is *"24 sent, 3 failed, 3 skipped"*,
  and the three failures must stay identifiable and retryable **individually**. A node that reports a single
  aggregate verdict hides which notes are still only on the tablet, which is the one fact the musician needs.
- **Already-sent notes must be SKIPPED, not re-sent.** The per-note state already exists (the tab shows
  *"sent <date>"* / *"not sent"*). Bulk over a node the user sends twice — because the first attempt half
  failed — must not duplicate the successes in Studio. Skipping is the behaviour; *saying* it was skipped is
  what stops it reading as a silent no-op.
- **Failure must not be all-or-nothing at the node.** Do not wrap a node send in a transaction that rolls
  back 24 successes because 3 failed. The notes are independent; treat them so.

This is the same shape as the delete sweep (⟨GO⟩ `ef24ec4c`): an operation over a set, where the honest
report is per-item and the tempting report is one verdict.

### R3 — what does NOT change

The send is still explicit and per-device. No auto-send, no background sync, nothing that makes a note leave
the tablet without the musician pressing something. §7's "explicit, separate action outside `stage/`" is the
I12 carve-out's own wording and bulk does not widen it — one press may move thirty notes, but it is still
one press the user chose.
