# T105 — Edit a chart from where you're reading it, on a page of its own

**Priority:** normal, after T104 · **Size:** M · **Area:** `web/studio` (Web & Core lane).
VLL, 2026-08-25, asked for this after T104's quick wins: the viewer affordance and the dedicated route.

## 1. What T104 leaves unsolved

T104 makes the action one click **once the files panel is open**. The deeper cost is having to open that
panel at all: when you are *looking at* a rendered chart and spot a wrong chord, the fix lives in a
different part of the UI from the thing you are looking at.

And the editor still borrows whatever space a dialog inside a panel can spare.

## 2. What to build

**(a) An edit affordance in the viewer, for a generated chart.** When the file on screen is a text
chart, offer to edit it from there. Not on a PDF — there is nothing to edit.

**(b) A dedicated editor route**, e.g. `/bands/:bandId/songs/:songId/chart/:fileId`. The editor gets the
whole viewport instead of a dialog's leftovers, and the URL becomes linkable and reloadable — the
existing precedent is T68, which already made a file addressable in the URL.

**These two compose, and that is why they are one task.** Since both were asked for together, the
natural shape is: **the viewer's edit affordance navigates to the route.** One editor, reachable from
the viewer, from the file row (T104), and from a link. If review prefers the affordance to open T104's
in-place dialog instead — keeping the reader's context — argue it at the gate; it is a reasonable
alternative and the choice is reversible.

## 3. Rules

- **Returning must be obvious and lossless.** A full-page editor takes the reader away from what they
  were reading; leaving it should land them back on that song and that file, not at a generic page.
- **Unsaved work must not vanish on navigation.** A route makes back/forward and reloads real events in
  a way a dialog never had to survive. Decide the behaviour explicitly (warn, or preserve) rather than
  discovering it.
- **One editor, not two.** T104's dialog and this route must not fork into divergent implementations —
  same component, different host. If that proves impossible, that is a finding worth reporting before
  building the second copy.
- **T60 transpose and T67 re-render still work** through whichever host is used.

## 4. Acceptance criteria

- From the viewer showing a generated chart, editing is reachable **without opening the files panel**;
  the affordance is absent for a PDF.
- The route loads the editor directly on a cold navigation (paste the URL, reload) and 404s honestly for
  a file that isn't a generated chart.
- Leaving the editor returns to the song/file the user came from.
- Unsaved-changes behaviour on navigation is asserted, whichever way it is decided.
- T104's row affordance still works — this adds a path, it does not replace one.
- `tsc -b` clean; full `make e2e`.

## 5. Out of scope

The chart dialect itself; the preview pipeline; anything in T104.
