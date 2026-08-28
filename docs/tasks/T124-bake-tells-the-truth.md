# T124 — A bake that produced nothing must not report success

**Lane:** Web/Core · **Kind:** correctness / honesty · **Verified against `b6d23b7`**
**Files:** `web/studio/src/pages/SetlistDetail.tsx`, `core/internal/bake/` (+ tests both sides)
**Origin:** findings 3 and 4 of the 2026-08-28 full-flow check (`b6d23b7`).

> **📋 Correction, 2026-08-29 (Fable).** The finding-4 half of this task's premise was **wrong**, and the
> lane proved it by root-causing as this spec demanded. The renderer path was **already fail-loud**:
> `baker.go` maps a `RenderBatch` error to a terminal `BakeFailed` with a user-safe message, and its own
> comment names *"VLL's exact failure (the overlay CLI missing)"*. What I saw in the flow check was my own
> harness asserting the confirm dialog instead of the terminal record — the mistake I had already confessed
> about step 16, without following it through to notice it invalidated the finding. **The real defect was
> the empty setlist (finding 3), and that is what landed in `b2b5302`.**

## What was observed

Running the flow from an empty server, a bake **reported success in the browser while the output
directory was empty**, with the server log carrying a node `Cannot find module` trace for an unbuilt
`web/bake/dist/cli.js`. My own harness passed that step at first, because it asserted the confirmation
dialog rather than the artefact — the same mistake in miniature.

**Be precise about what is and is not established:** the empty-output-plus-success outcome was observed
end to end; the exact path by which a failed overlay render becomes a green UI was **not** isolated. Part
of the machinery is already fail-loud — `render.go:154-159` returns an error on a non-zero CLI exit, and
`core/internal/bake/errors_test.go` asserts the node trace reaches the log. **Root-cause it first; do not
assume this spec's author already did.** If the real defect turns out to be narrower than described here,
say so at the gate and fix the narrow thing.

## The three properties to establish

### 1. An empty setlist cannot be baked

`SetlistDetail.tsx:273`:

```tsx
data-testid="bake-setlist"
disabled={dialog}
```

`dialog` is *"the bake dialog is already open"* — the only condition on the control. `songIds` is right
there in the same component (`:207`), unconsulted. Baking a setlist with no songs produces a bundle of
nothing and calls it a concert. Guard it, and say why the button is disabled rather than just greying it
out — a disabled control with no explanation is its own small failure to communicate.

### 2. The bake's terminal record is derived from the artefact, not from reaching the end of the code

A bake that wrote no bundle, or a zero-byte one, is a **failure**, however cleanly the pipeline returned.
The check belongs where the terminal progress record is written (T103's polling contract), so that every
consumer — studio, tests, anything later — inherits it.

*Assert the artefact, not the gesture.* This is the general rule the flow check earned, and this task is
where it becomes a property of the product rather than of one harness.

### 3. A terminal failure reaches the person who clicked

`BakeCard` already has an `error` state and already clears it per run (`:242-243`). Make a terminal
failure land in it, carrying the server's own words. A missing or unbuilt `TROUBA_BAKE_CLI`
(`core/internal/config/config.go:114`) should read as an operator problem — the renderer is not built —
not as a raw node stack trace. `core/internal/bake/errors_test.go` already maps node traces to friendlier
text; **extend that mapping rather than adding a second one beside it.**

## Tests

- **Go**: a bake whose CLI path does not exist ends in a **terminal failed** record naming the cause. This
  is the load-bearing one — it is the exact configuration that produced the observed false success.
- **Go**: a bake that produces no bundle is not recorded as successful.
- **Studio**: the bake control is unavailable for a setlist with no items, and available with one.

## Teeth-check

Make the terminal record report success when the bundle is absent or empty. A named test must redden.
Report the exact count, and the Go and vitest totals.

## Before landing

`gofmt -l core`. The CI `go` job gates on gofmt **after** vet and test, so a green test run says nothing
about it.

## Not in scope

The DEMO-VID walkthrough script (**T125**) · rebuilding `web/bake` in CI or in `make` · the bake's
performance · retrying a failed bake automatically.
