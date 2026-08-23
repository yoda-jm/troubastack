# T94 — Layers, Notes and Details: two honest classes, one dismissal contract

**Priority:** normal — VLL hit this using the editor on desktop · **Size:** M ·
**Area:** `web/studio` (`pages/song-editor/Viewer.tsx`, `styles.css`, e2e). Lane: Web & Core.

VLL, 2026-08-23: *"Layers, Notes and Details is a little bit of a mess, especially on Desktop, they
should act the same but notes open annotations, Details can be open on top, layers is also a tab
inside the same flyout as notes"*.

Design settled with VLL the same day: **option B** — keep the two classes distinct, make every panel
behave the same way, and stop them overlapping.

## 1. The organising principle (VLL's, and the code already agrees)

> *"Details is like properties of the whole song, whereas Annotation/Layer is something just for the
> current file."*

That is exactly what the data does today — the UI simply doesn't say so:

- `Viewer.tsx:414` — `sortedFileLayers` filters layers to `selectedFileId`. The comment (T40
  follow-up) spells out why: without it the panel showed the Score's cues while you were looking at
  the Vocals part.
- `Viewer.tsx:1366` — `AnnotationList objects={objectsForFile}`. Also per-file.
- The Details panel is song-scoped end to end: metadata, the band file pool, my-files order, my-cues,
  and the admin delete. Nothing in it changes when you switch which file you're viewing.

So this task is not inventing a taxonomy. It is making the chrome tell the truth about one that
already exists: **the rail inspects the file you are looking at; Details is the song's properties.**

## 2. What is wrong today, precisely

| | Layers | Notes | Details |
|---|---|---|---|
| Surface | `.drawer`, right, 300px, **z-7** | **the same `.drawer`** | `.details-panel`, centred, 680px, **z-9** |
| Opened by | `sidebar-toggle` pill | `drawer-notes` pill | `my-files-edit` pill (+ `add-file`, `viewer-no-pdf`) |
| Closed by | re-click the pill · chevron | same | ✕ · Escape · outside-click (T89) |

1. **Two buttons, one surface.** The pills read as two independent panels but share one container:
   opening Notes silently *replaces* Layers, and clicking a lit pill closes the whole rail. The rail
   already has its own tab row (`.drawer-tabs`), so the pills duplicate it.
2. **Three peers, three mechanisms** — two tabs and a floating panel, with different exits.
3. **They overlap.** z-9 centred over z-7 right-anchored, both open at once, no stated relationship.
4. **Two exits vs four.** Details has ✕/Escape/outside-click; the rail has neither Escape nor
   outside-click.

## 3. The change

### 3.1 One pill for the rail, tabs inside it

Replace the two pills (`sidebar-toggle`, `drawer-notes`) with **one** pill that opens and closes the
rail. The `.drawer-tabs` row inside the rail becomes the only place you switch Layers ↔ Notes.

- Label it for what it is — the current file's contents. **"This file"** is my proposal; the lane may
  counter-propose at the gate, but it must name the *scope*, not the widget ("Panels", "Sidebar" and
  "Inspector" all fail that test).
- The pill **remembers the last tab for the session**, so the common case stays one click. Default to
  Layers on a fresh editor.
- **Keep both testids alive.** `sidebar-toggle` moves to the new pill; `drawer-notes` moves to the
  Notes **tab** inside the rail. Existing specs address them, and silently deleting a testid that a
  spec still queries turns a real assertion into a no-op that still passes.

> **SETTLED by VLL, 2026-08-23.** He chose the single chip, in his own words: *"a layer+annotation
> chip that opens both (but a single button), if they are so strongly connected, with 2 tabs inside."*
> That is this section as written — build it. The deciding argument, for the record: the tab row inside
> the rail is **unavoidable** (open the rail and you must see which tab you are on), so two top-bar
> pills would ship a permanent second control for a job the tab row already does. The cost is one
> extra click in exactly one case — rail closed, last tab Layers, you want Notes — and the remembered
> tab shrinks even that. VLL had already spotted the redundancy himself when he reported the bug:
> *"layers is also a tab inside the same flyout as notes."*

### 3.1b The real blast radius is `openDrawer()`, not the two testids

I under-specified this in §3.1 and found it while reviewing T71. `e2e/fullscreen-helpers.ts:69` is a
**shared helper used by 22 spec files, ~48 call sites**:

```ts
export async function openDrawer(page: Page, tab: "layers" | "annotations" = "layers") {
  const pill = tab === "layers" ? page.getByTestId("sidebar-toggle") : page.getByTestId("drawer-notes");
  const alreadyOnTab = (await pill.getAttribute("aria-pressed")) === "true";
  if (!alreadyOnTab) await pill.click();
  await expect(page.getByTestId("viewer-drawer")).toBeVisible();
}
```

It encodes the exact model this task removes: one pill *per tab*, `aria-pressed` meaning "this tab is
showing".

**Update the helper; do not touch the 48 call sites.** `openDrawer(page, tab)` should open the single
chip if the rail is closed, then select the requested tab inside it. Every existing call keeps its
meaning for free, and the diff stays reviewable.

**The trap, and the guard I want:** the lazy repair is to click the chip and ignore `tab`. Then every
`openDrawer(page, "annotations")` call silently exercises the **Layers** tab, ~20 specs keep passing,
and their annotation assertions test nothing. That is the dangling-testid failure in helper form.

So the helper must **assert it landed on the requested tab before returning** — `annotation-list`
visible for `"annotations"`, `layers-panel` for `"layers"` — not merely that `viewer-drawer` is
visible. Prove it: make the helper ignore its `tab` argument and confirm a batch of annotation specs
go **red**. If they stay green, the helper isn't guarding and the suite is lying.

`closeDrawer()` (`:78`) needs the same pass against the new close contract.

Also note `editor-layers.spec.ts:532` — *"both drawer tabs (Layers, Annotations) are reachable"* — is
a behavioural assertion about the old two-pill model. Rewrite it for the chip+tabs model; **do not
delete it.** Reachability of both tabs is still exactly what it should assert.

### 3.2 Details becomes the song's properties, and looks like it

- The pill takes a **gear/⚙ icon** (VLL: *"properties (wheel)"*) with the title **"Song properties &
  files"**. Today it is a list-with-dots glyph titled "Song details & files" — indistinguishable in
  kind from the two rail pills next to it.
- **Check for a gear collision first.** If another ⚙ already exists in the studio chrome, report it
  at the gate before picking a different glyph — do not silently choose something else.
- Its three tabs (Shared with the band / Just for you / Admin) are unchanged. They are an *audience*
  split within the song's properties, which is coherent once the panel is clearly song-scoped.
- The other two entry points (`add-file` at `Viewer.tsx:1425`, `viewer-no-pdf` at `:1200`) keep
  working and must open the same panel.

### 3.3 One dismissal contract for all three surfaces

Every panel closes by **its own ✕**, **Escape**, and an **outside click**.

- The rail gains a ✕ (it has only a collapse chevron today — keep or replace it, but the affordance
  must read as "close"), plus Escape and outside-click.
- **Extract T89's dismiss logic into ONE shared helper and use it for both surfaces.** Do not copy
  it. `Viewer.tsx:123-149` contains the portal-aware version: Escape is ceded to any open
  `[data-portal]` overlay, and a click inside a `[data-portal]` node counts as *inside* the panel.
  That exemption exists because T87 portals a file row's ⋯ menu out of the Details panel and T91's
  dialogs live there too — a second, hand-written copy for the rail is exactly how that regression
  comes back. One helper, both callers, one place to fix.
- Escape and outside-click must respect the same portal rule for the rail. **A delete-layer
  confirmation (T83) opens from inside the rail** — pressing Escape on it must close the dialog and
  leave the rail open, not collapse the rail underneath it. Test this specific case.

### 3.4 Mutual exclusion — no more overlap

**At most one of {rail, Details} is open at any time.** Opening Details closes the rail; opening the
rail closes Details. This is the whole fix for the overlap, it is one line of state, and it is
directly testable.

**Ruled (Fable), on VLL's *"Details opens on top of everything whatever"*:** his point is that Details
is unmistakably the higher class — song properties, not a peer of the file inspector — and I agree.
But "higher class" is expressed by the ⚙, by taking over the screen, and by being the thing nothing
else clips; it does **not** require the rail to stay open underneath. Two overlapping panels was
defect #3 in his own report (a 680px centred panel sitting over a 300px right rail leaves a half-hidden
strip poking out, which is precisely the "mess"). So: Details takes over, and closing it does **not**
reopen the rail — the state is one panel or none, never two.

If VLL wants the rail preserved and restored behind Details, that is a one-line change to this
paragraph; ask before building it differently.

No new backdrop or scrim. The score stays visible behind whichever panel is open — that is the point
of a panel rather than a modal, and adding a scrim would make Details feel like the blocking dialogs
T91 just removed.

### 3.5 Naming

The user-facing word for the annotations tab is **Notes**, everywhere: pill, tab, and any heading.
The internal `drawerTab === "annotations"` identifier and `AnnotationList` may stay as they are —
add a one-line comment at the state declaration recording that "annotations" is the code name for
what the UI calls Notes, so the next reader doesn't take the mismatch for a bug.

## 4. Mobile

Below **760px** the rail already goes full-width (`styles.css:667`) and Details already shrinks via
`width: min(680px, calc(100% - 1.75rem))`, so option B and option A converge on a phone: **one
full-width sheet at a time**, which §3.4 now guarantees outright.

Two mobile-specific requirements, both learned from T89:

- **The ✕ is mandatory on both surfaces and must stay reachable.** The top-bar pill strip scrolls
  horizontally on a phone, so the pill that opened a panel can scroll out of view — that was the
  original T89 dead end. The ✕ must sit in a **sticky** row that survives scrolling the panel body
  (Details already does this; the rail must match).
- **Escape does not exist on a phone**, so the ✕ and the outside tap are the only exits there. Both
  must work with **touch** events, not only `mousedown`. The current T89 handler listens for
  `mousedown` only; verify on a touch emulation profile that an outside tap closes both panels, and
  if it doesn't, fix it as part of this task.

The single pill from §3.1 also buys back a slot in the phone's chrome row-count budget (the
constraint T66 recorded) — a small side benefit, not a justification.

## 5. Acceptance criteria

- **Mutual exclusion:** with the rail open, opening Details closes the rail (`viewer-drawer` count 0)
  and vice versa. Assert both directions.
- **One contract, all three exits, both surfaces:** ✕ closes; Escape closes; an outside click closes.
  Six assertions, rail and Details.
- **The portal rule survives, on the rail too:** open the rail → Layers → trigger the T83
  delete-layer confirmation → press **Escape** → the dialog closes and **the rail is still open**.
  Same for an outside click landing inside a portalled surface. This is the T89 regression in its new
  home, and it is the assertion I care most about.
- **Details' other entry points still work:** `add-file` and `viewer-no-pdf` both open the panel.
- **Tab memory:** open the rail → Notes → close → reopen → it is on Notes. Fresh editor → Layers.
- **Dangling-testid sweep:** `sidebar-toggle` and `drawer-notes` both still resolve to a real,
  clickable element after the move, and **every** spec that references them passes unmodified where
  the behaviour is unchanged. Grep the e2e directory for both ids and list what you found at the
  gate — I will check that list against my own grep.
- **Teeth-check, and name the test for each:** (a) remove the mutual-exclusion line → the overlap
  test reddens, and only it; (b) point the rail at a private copy of the dismiss helper instead of
  the shared one and delete its portal exemption → the delete-layer-Escape test reddens, and only it.
  Report the blast radius as a count, e.g. "1 of N".
- `tsc -b studio` clean; **full `make e2e`** on isolated ports, one suite at a time.
- **Screenshots** of the final desktop layout (rail open; Details open) and the phone layout
  (each full-width), from the build under review.

## 6. Out of scope

- The contents of any panel — Layers' rows, the annotation list, metadata, the file pool, the chart
  editor. This task moves and unifies chrome; it changes nothing inside.
- Making Details reachable from the concert/stage surfaces.
- Resizable or dockable panels, and any persistence of panel state beyond the session-scoped last
  tab in §3.1.
- The text-tool re-arm question (T90 follow-up) — separate, and **not yet filed**; it takes the next
  free number if VLL takes it.
