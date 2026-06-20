# Design: Rendering & ink

Derives from **I3, I8, I9, I10**. This is the riskiest subsystem — read carefully.

## Two kinds of latency (do not conflate)

| | What it is | Fixed by |
|---|---|---|
| **Sync latency** | stroke → server → peers | commit-on-up (02-doc). Solved. |
| **Input→photon latency** | pen tip → line under it, mid-stroke | *local* rendering only. The reason the native overlay exists. |

Commit-on-up does **nothing** for input→photon latency — that is purely where you draw the wet ink.

## Wet / dry split (I9, I10)

- **Dry layer** = PDF + all committed objects. Rendered by the **web** layer always.
- **Wet layer** = the in-progress freehand stroke only.
  - **Pure browser** (laptop/desktop, and the fallback everywhere): wet ink drawn in-browser on a
    second canvas (`desynchronized`, `pointerrawupdate` + `getCoalescedEvents()`). Good enough on
    desktop; this path **always exists** (I10).
  - **Mobile, in the app**: an optional **native overlay** renders the wet stroke for lowest
    stylus latency, then hands it to web on commit. Feature-detected; falls back to in-browser.

**MVP: only freehand is native.** Lines/shapes/text/move/select all preview in the web layer
(coarse gestures, not latency-critical). Promote another type into native later only if a real
tablet shows it lagging ("passthrough" is the per-type dial).

## Rendering: one authoritative renderer, transient wet (I8)

Sharing *geometry* (`web/ink`: points → outline) does **not** guarantee identical *pixels* —
rasterization (anti-aliasing, line joins, text, sub-pixel) differs across canvas backends (browser /
node-canvas / native). So parity is handled by *what must match*, not by geometry tricks:

- **Editor (dry) vs. bake — MUST match** → the **bake reuses studio's dry renderer** (headless
  studio): the *same* code+canvas, so the baked images are **pixel-identical by construction**. No
  separate bake renderer, no parity test needed here.
- **Native wet vs. dry — needn't match exactly** → the wet overlay renders only the *in-progress*
  freehand stroke and is replaced by the authoritative dry render on commit. Share `web/ink`
  geometry so it's *visually close*; accept a sub-second pop at pen-up. (The pure-web wet path
  renders in the browser = same as dry = no pop, but higher latency — the per-platform trade.)

So there is **one authoritative renderer** (dry, reused for bake) and **one transient, approximate**
one (native wet). `web/ink` (geometry) is shared to keep the wet approximation close; it is *not* the
load-bearing guarantee — reusing the dry renderer for the bake is.

## Coordinates (I3)
Both layers render `screen = clamp01(coord) × viewportTransform`. Because storage is `[0,1]`,
committed objects track the page through any zoom/rotation automatically.

## Zoom
- Web owns the viewport transform (scale + offset).
- You **don't zoom mid-stroke**, so the transform is static for a stroke's lifetime → the wet layer
  needs the transform only at stroke-start → **no per-frame native↔web bridge sync**.
- **Re-rasterize the PDF page at the new scale** (PDF.js) for crisp text — never CSS-scale the old
  bitmap. Pattern: instant CSS-scale during the pinch, debounced crisp re-render on settle.
- Touch routing: **stylus draws, fingers pan/zoom** (also gives palm rejection).

## Notification & the wet→dry handoff

The wet layer is always **local and personal** — it only ever holds *your* in-progress stroke.
Others' edits, and your own *committed* strokes, live in dry. So there are two fully **decoupled**
concerns:

**(A) Wet→dry handoff — fully LOCAL and instant (no network).**
1. Native draws the wet stroke live (low latency).
2. On pen-up, native **posts the completed stroke to the webview** (in-process bridge) with a
   transient handoff id, and keeps showing it.
3. Studio **emulates the same drawing in the dry layer locally** — same `web/ink` renderer, so it is
   pixel-identical (I8) — and acks the handoff id.
4. Native **clears the wet stroke on that local ack** — one bridge hop / next frame, *not* a server
   round-trip. Order: dry paints, then wet clears → no gap, no double-draw.

Native is now maximally dumb: draw, post to webview, clear on ack. It knows **nothing** about the
server, UUIDs, echoes, or sync (I9/I15). It tags the handoff with a transient correlation id only to
match the ack; **studio owns the domain UUID.**

**(B) Dry→core sync — separate and async, owned by studio.**
- Studio (now holding the object in dry) posts it to core, reconciles the WebSocket echo by UUID
  (its own object → no-op; others' objects → render into dry), and rolls back on `deleted-remotely`.
- Concurrent editors receive the object only via this echo, into their dry layer (I2, I6).

So **"wet→dry" (local, instant) is fully decoupled from "dry→core" (network, async).** Native talks
only to the webview; the webview is the sole client of core.

## ⚠️ First build step: the web-ink spike
Before committing to the native overlay, build a throwaway `web/studio` spike (PDF.js +
`web/ink` + low-latency canvas) and judge stylus feel **on the real target Android tablet**.
This validates the one assumption the client architecture rests on. Keep the native drawing path
in mind but do not delete the in-browser path — it is the canonical fallback (I10).
