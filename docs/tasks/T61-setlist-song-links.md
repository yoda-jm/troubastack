# T61 — Setlist item → song page hyperlink

**Lane:** web-core · **Size:** S · **Status:** SPEC'd 2026-07-21 (VLL: *"an hyperlink from a song playlist to the song would be super nice"*)

## What

In the studio setlist page (`web/studio/src/pages/SetlistDetail.tsx`), each item's **song title links to that song's editor page** (the existing song route the library uses — reuse it, don't invent a new one).

## Constraints

- Must not break drag-to-reorder (T52): a drag that starts on the title must still reorder, and a plain click must navigate — the usual click-vs-drag discrimination. If the row already discriminates, an inner `<Link>` with `draggable=false` is likely enough; verify by driving the existing reorder e2e over the link.
- Keyboard/middle-click behavior comes free with a real anchor — use a router `Link`, not an onClick handler.
- Visual: keep the row's current look; the link may take link affordance on hover only (no permanent blue-underline noise in the setlist).

## Acceptance

1. e2e: click a setlist item's title → lands on that song's editor page; reorder via drag on the same row still works (existing reorder spec extended, red-first on the nav assertion).
2. Middle-click/ctrl-click opens in a new tab (real href present — assert the anchor's `href`, no click-handler-only nav).
