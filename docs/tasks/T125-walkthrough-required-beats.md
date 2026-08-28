# T125 — The walkthrough's REQUIRED beats must be required

**Lane:** Web/Core · **Kind:** correctness in the video script · **Verified against `b6d23b7`**
**Files:** `web/studio/walkthrough/walkthrough.spec.ts`
**Origin:** findings 1 and 2 of the 2026-08-28 full-flow check (`b6d23b7`).
**⚠ This file is the DEMO-VID script.** Changing it changes the video. Re-recording is VLL's call; this
task makes a correct take *possible*, it does not schedule one.

## The defect the file states about itself

Header comment, `walkthrough.spec.ts:12-13`:

> `soft(label, fn)` runs a step but logs + continues on any miss so the tour always finishes. **The two
> REQUIRED beats — the capo mark …**

Both "REQUIRED" beats are wrapped in `soft()`. A step that can only skip cannot report that it stopped
working, so the tour always finishes and always looks fine. Two of its beats have been broken and silent:

**1 — the capo note never lands, and jams the take (`:358-366`).**

```ts
// 3) a bold red "capo on!" note above it (text tool → native prompt)
page.once("dialog", (d) => d.accept("capo on!"));
```

The comment is wrong about the product. Text entry is an **in-app modal**, not `window.prompt` —
`web/studio/src/pages/song-editor/WetCanvas.tsx:729` calls `prompt({ title: "Text annotation", … })`, the
promise-based helper rendered by `components/Dialog.tsx` (`data-testid="app-dialog-input"` `:189`,
`app-dialog-confirm` `:205`). No native dialog event ever fires, the handler never runs, and **the modal
stays open across everything that follows** until `soft()` swallows the wreckage.

**2 — the setlist beat targets testids that do not exist (`:440-465`).**

- `setlist-add-song` — **absent from the studio source.** The real control is a `<select>`
  `add-item-song` (`SetlistDetail.tsx:631`) plus a submit `add-item` (`:644`).
- `new-setlist-btn` — **also absent.** The Setlists page creates inline; `setlist-name` and
  `create-setlist` exist (`Setlists.tsx`), the button does not, and the `getByRole(/new setlist/i)`
  fallback does not match either.

Each is guarded by `if (await has(…))`, so both simply do nothing. The consequence compounds: the setlist
may never be created, so the song is never added, so **S12 bakes an empty setlist** — while the recording
shows a confident tour.

## Deliverable

1. **Fix the capo note**: drive the in-app modal — fill `app-dialog-input`, confirm with
   `app-dialog-confirm`. Fix the stale comment at `:357` too; it is what made the bug plausible.
2. **Fix the setlist beat**: create via the inline form, then select the song in `add-item-song` and
   submit `add-item`.
3. **Make REQUIRED mean required.** The beats the file names as required must fail the run when they fail.
   Keep `soft()` for genuinely optional colour; stop using it where a miss means the video is wrong.
4. **Audit every testid in the file against the studio source.** Two were dead; the two above were found
   by accident while checking something else, and there is no reason to think they are the only ones. A
   selector that matches nothing is indistinguishable from a passing step here.

## Verification

Run the walkthrough and check **artefacts, not gestures**:

- the capo note exists as an annotation afterwards, and no modal is left open;
- the setlist exists and contains the song;
- S12's bundle is non-empty (T124 makes this the product's job too — until it lands, assert it here).

Report the pass/fail count and name every beat still wrapped in `soft()` after the change, so the next
person knows exactly which parts of the tour can still skip silently.

## Not in scope

Re-recording · narration, pacing or annotation styling · the bake's own honesty (**T124**) · the flow-check
harness at `web/studio/flowcheck/` (a separate assertion suite; leave it alone).
