# T97 — A bake spawns two external processes PER SONG, even with nothing to draw

**Priority:** high — VLL: *"updating is long even for a small concert, this is strange"*. It is
strange, and there is a structural reason. · **Size:** S to measure + S/M to fix ·
**Area:** `core/internal/bake`. Lane: Web & Core. **Do this before T96** — see §4.

**CONFIRMED BY VLL, 2026-08-23:** the slow thing is the **server-side bake**, not the app's Update
(download + install). So this task is aimed correctly, and the A39 Home-Update stall is a separate,
unrelated problem already in flight with Mobile. **Approved to build — Web-Core.**

## 1. What I found

`bakeSong` spawns **two external processes for every song in the setlist**:

- `baker.go:351` → `b.raster.Rasterize(ctx, pdf)` → `exec.CommandContext(ctx, r.bin, "-png", …)`
  (`render.go:62`, poppler `pdftoppm`).
- `baker.go:369` → `b.overlays.Render(ctx, …)` → `exec.CommandContext(ctx, r.node, r.cli, …)`
  (`render.go:141`, **node** + `@napi-rs/canvas`).

Both are per **song**, not per page — so the cost scales with *how many songs are in the setlist*,
independent of how much content or annotation each one has.

**And the overlay spawn is unconditional.** At `:369` there is no guard on whether the song's document
contains any layers or objects at all. A concert with **no annotations** still starts `node` once per
song, loads a canvas native module, and draws nothing.

That predicts VLL's symptom exactly: a "small" concert — few pages, few or no annotations — still pays
N × (node startup + poppler startup). Cost tracks song count, which is the one thing a small concert
still has.

## 2. Measure first — do not optimise on my reading

I inspected the structure; I did **not** time it. Step one is a measurement, because the fix depends on
where the time actually goes:

- Instrument a real bake of the demo setlist: total, and per song the time in `Rasterize`, in
  `Render`, and everything else. Report the table at the gate.
- Separate **process startup** from **work**: run the same `node` CLI twice in one process-lifetime if
  you can, or time a no-op invocation, to get the fixed cost per spawn.
- Do it on a quiet box (`uptime` first) — the whole point is a number that means something.

If startup is not the dominant term, **stop and re-present**: the analysis in §1 is a hypothesis about
cause, and the measurement is allowed to refute it.

## 3. The fix, in the order the evidence will probably support it

1. **Skip the overlay spawn when there is nothing to draw.** If the song's snapshot has no layers with
   objects on the baked file, don't start `node`. Biggest win for the smallest change, and it makes an
   un-annotated concert cost nothing extra. Assert it: a no-annotation bake must spawn **zero** overlay
   processes (inject a counting fake — `fakeOverlays` already exists in `baker_test.go`).
2. **Batch the overlay CLI across songs** — one `node` invocation for the whole bake instead of N. The
   request already carries `Doc`/`Pages`/`OverlayWidth`, so the shape generalises to a list. This is
   the real fix if startup dominates, and it turns an O(songs) tax into O(1).
3. **Only then consider parallelism.** Running songs concurrently would hide the cost rather than
   remove it, and it collides with the known concurrency work in `baker.go` — see §4.

## 4. Sequencing, and a hazard

**Do T97 before T96 (bake progress).** Progress on an operation that is slow for no good reason
documents the problem instead of fixing it; and if T97 lands second it changes the very timings T96's
UI is built around. Fix the cost, then report it.

**Hazard:** `TestBake_ConcurrentSameSetlist_distinctRevs` is a known intermittent failure with a real
race behind it in `baker.go`. Any restructuring here touches that path. Run
`go test -race ./internal/bake/` and report what it says — if the race surfaces, it may be the same bug
and worth fixing here rather than dodging.

## 5. Acceptance criteria

- **A measurement table** (per-song `Rasterize` / `Render` / other, plus fixed spawn cost) from a quiet
  box, before any fix — and the same table after, so the improvement is a number rather than a claim.
- A bake of a setlist with **no annotations** spawns **zero** overlay processes; asserted with a
  counting fake, not by timing.
- Byte-identical bundles for the demo setlist before and after (a sha over the `.tstage`) — this is a
  performance change and must not alter output. If it legitimately must change, stop and come to the
  gate first.
- `go test -race ./internal/bake/` green, including the concurrent same-setlist test.
- `gofmt -l core` clean; `go test ./...` green.

## 6. Out of scope

- Progress reporting — **T96**, after this.
- Changing the rasteriser or the DPI, or caching rasters across bakes. Both are real ideas and both
  deserve their own decision; this task is about not paying for work nobody asked for.
