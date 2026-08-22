# T93 — de-flake `fourFileRows` (files-list-menu.spec.ts)

**Priority:** normal (quality — but it guards a fix VLL actually hit)
**Filed by:** Web-Core, at Fable's request in the T90 verdict (2026-08-22)

## The problem

`web/studio/e2e/files-list-menu.spec.ts` is intermittently red in its **shared fixture**, not its
assertions. The helper `fourFileRows(page)` uploads one PDF then creates three text charts in a loop:

```
for (const t of ["AAA", "BBB", "CCC"]) {
  await panel.getByTestId("new-text-chart").click();
  await panel.getByTestId("chart-source").fill(`# ${t} Chart\n\n## Verse\nla\n`);
  await panel.getByTestId("chart-save").click();
}
await expect(panel.getByTestId("file-row")).toHaveCount(4);
```

The failure is always `chart-save` (or the following `file-row` count) timing out — the chart editor
for the next iteration hasn't mounted `chart-save` yet when the click fires, i.e. a **fixture race in
the create→save→reload loop**, not the behaviour under test.

Because every test in the spec calls `fourFileRows`, the flake surfaces on whichever test draws the
short straw (`:36`, `:108`, `:137` have all been seen).

## Measured rate — it reproduces on clean `origin/main`, T90 not involved

`files-list-menu.spec.ts --repeat-each=8` on a clean `origin/main` worktree (ink node_modules linked):

| run | result |
|---|---|
| Web-Core, on main | 23 passed / 1 failed (`:108`) |
| Fable, on main | 22 passed / 2 failed (`:108`) |

**≈ 3 failures in 48 runs (~6%).** The test it most often hits is *"the last row's ⋯ menu is
in-viewport and actionable, not clipped (T87)"* — the **regression guard for the dead control T87
fixed**. A ~6% flaky guard on a real fix is the alarm that fails to arm one run in sixteen: someone
reruns, gets green, and lands a regression through it.

## Fix direction (Fable: "fixed, not characterised")

The cause looks like the helper, not the assertion. Make the create-chart loop deterministic:

1. After each `chart-save`, **await the observable post-condition before the next iteration** — e.g.
   `await expect(panel.getByTestId("file-row")).toHaveCount(n)` growing 2 → 3 → 4, so each save is
   confirmed landed+reloaded before the next `new-text-chart` opens. (The current code only asserts
   the final count of 4, so the loop races ahead of the reload.)
2. Confirm the chart editor is actually mounted before filling — `await expect(chart-source)
   .toBeVisible()` (and/or `chart-save` enabled) rather than assuming the click synchronously mounts it.
3. If `new-text-chart` → editor open is itself async (state round-trip), gate on the editor's own
   testid appearing, not on a fixed tick.

## Acceptance

- `files-list-menu.spec.ts --repeat-each=20` green on a quiet box (0 failures / 60 runs).
- No production code change — this is a test-fixture fix. If a *product* race is found instead
  (chart-save genuinely not mounting), stop and re-spec: that would be a real bug, not a flake.
- The T87/T78 behaviours the spec guards are unchanged (same assertions, same testids).

## Notes

Sibling of the known `TestBake_ConcurrentSameSetlist_distinctRevs` baker race — both are "the
environment/fixture flakes, not the code". Keep the log as the evidence: quote the reproduced number.
