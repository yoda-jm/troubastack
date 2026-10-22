# T202 — The concert PDF is printed for ONE member (me by default, changeable), with a progress bar

Owner: web-core. Spec: Fable, 2026-10-22.

## Why (VLL)

> "download bake pdf should be user scoped, by default logged in user but changeable. also a progressbar while making
> it."

## What exists (read in code)

- **`GET /api/bands/{b}/concerts/{c}/pdf`** (`bakeapi.go concertPDF` → `bake.ConcertPDF`) prints a baked concert to
  A4.
  - Its viewer is already the caller, and an **admin** may pass `?member=` to print for someone else.
  - **The member only selects which PERSONAL layers are painted.**
- **It ignores the member's file selection.** `ConcertPDF` prints every page of every song (`s.Pages`), but the bundle
  already carries per-member page lists (`BakedSong.memberPages`: a default entry, plus one entry per member with a
  selection).
  - A bassist who chose "bass only" gets lyrics + bass on paper and only the bass on the tablet.
  - The paper fallback (T57: "in case of tablet malfunction") must match the tablet.
- **It is synchronous.** A long concert (100+ composed JPEG sheets) blocks the request with no feedback, and Studio's
  link just waits.

## Required

### 1. Print exactly what that member's tablet shows

For the chosen member, print **their pages**:
- the `memberPages` entry for that member if present, otherwise the default entry;
- in that order;
- on-call songs last, as today.

Their personal layers, the shared and mandatory layers as today, and the explicit `role` param keep their current
semantics. Header and footer as today, plus the member's display name in the footer
("<concert> · for <name> · 12/87").

**Pin it with a test:** a two-file song where member A selected only file 2 → A's PDF has only file 2's pages. Member B
with no selection → the default pages. Show it red on today's `s.Pages` loop.

### 2. Who can be chosen

- **Default: the logged-in user.**
- **An admin** can choose any member of the band (the current `?member=` gate, kept).
- **A non-admin can only print their own view.** The picker shows their own name, disabled, with a one-line reason.
- **Ruling:** someone else's personal annotations are theirs. This keeps the existing T57 rule; it is not a new
  restriction.

### 3. Asynchronous, with progress

Reuse the bake pattern (kick + poll):

| Call | Effect |
|---|---|
| `POST …/concerts/{c}/pdf-jobs {member?, role?}` | same gate as above → `202 {jobId}` |
| `GET …/pdf-jobs/{id}` | `{state: queued\|running\|done\|failed, done, total, error?}`; `done/total` = sheets composed |
| `GET …/pdf-jobs/{id}/file` | the PDF, `Content-Disposition` filename `<concert> — <member>.pdf` |

- **Job lifetime:** in memory, a job and its file expire after 10 min.
- **One running job per (concert, member, role)**: a second request returns the same jobId.
- Jobs are visible only to their creator, or to an admin of the band.
- **Determinism is kept:** the job's bytes are identical to the sync endpoint for the same viewer.
- **Keep the sync `GET …/pdf`,** with the member-scoped fix of §1, for compatibility. Studio stops using it.

### 4. Studio

"Download PDF" on the setlist detail opens a small dialog:
- **"Print for: [Me (Vincent) ▾]"**: admins get the roster, members a disabled field. A "Layers: as on my tablet" note.
- **[Make PDF]**, then a progress bar "Composing page 12 of 87…", then the file downloads by itself (a blob), and a
  "Download again" link stays until the dialog closes.
- **On failure:** the server's message, and Retry.
- The labels go on the docs-stage label list.

## Tests

- §1 member pages: the red-first test above, plus the variant concert id (`setlistId~member`), which still prints
  that member's variant.
- §2 gate: a member choosing another → 403, an admin → OK, default = the caller.
- §3:
  - progress is monotonic, reaches `total`, and `total` = the number of sheets;
  - the job file bytes == the sync endpoint's bytes for the same viewer;
  - an expired job → 404;
  - the second identical request reuses the job.
- Studio e2e: open the dialog, check the default member, the progress (mocked), the download fires, and the member
  sees a disabled picker.

## Out of scope

- The tablet app's own print (none today).
- Choosing layers per print (FILEPDF has `?layers=`; the concert PDF keeps "as on the tablet").
