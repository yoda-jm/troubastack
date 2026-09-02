# T126 — main is red: two bake e2e specs bake an empty setlist

**Lane:** web-core.
**Status:** **LANDED** `03c42b3a` — `e2e` is green at 200/0. (The lane correctly refused this
spec's claim that nothing unit-tested `bakeSetlistDisabled`: `web/studio/test/setlist-bake-guard.test.ts`
had existed since T124 with the exact three cases. That error was mine.)
**Raised by:** VLL — "spec aussi le fix pour le rouge pour la bonne lane".

## What is failing

`CI` → job `e2e`, **198 passed / 2 failed in 11.4m**, the same two every time:

- `e2e/bake.spec.ts:12` — *admin bakes a setlist → download link + history appear* `@smoke`
- `e2e/bake-pdf.spec.ts:11` — *admin downloads a concert PDF (paper fallback)*

Both die on the **same single line**, `bake.spec.ts:29` and `bake-pdf.spec.ts:25`:

```ts
await page.getByTestId("bake-setlist").click();
// locator.click: Test timeout of 30000ms exceeded.
//   56 × waiting for element to be visible, enabled and stable
```

Not flaky, not load: **deterministic on three separate commits** (`b64d254` and `a46ecc8` on
08-29, `fb5eb01` on 08-30). Every other job is green, including on today's tip `d39cba48`.

## Why

`SetlistDetail.tsx:149`:

```ts
export function bakeSetlistDisabled(dialogOpen: boolean, songCount: number): boolean {
  return dialogOpen || songCount === 0;
}
```

…wired at line 281, with a title that spells out the intent: *"Add at least one song to this
setlist before baking."*

**Both specs create a setlist and click Bake without ever adding a song.** They are clicking
a permanently disabled button, so Playwright waits the full 30s and gives up. The element is
visible — the preceding `expect(bake-card).toBeVisible()` passes — it is simply never
*enabled*.

The guard came in with **T124 `b2b5302`**, *"a bake that produced nothing must not report
success"*. **The product behaviour is correct.** Baking an empty setlist producing a
"success" was the bug T124 fixed; the specs are the stale party.

**This is a miss in my own review.** I verdicted T124 and did not sweep for the e2e specs
that exercise the button it disabled. The sweep discipline exists for exactly this, and I
did not apply it.

## The deeper gap: T124's behaviour has no test at all

Worth noticing before touching anything. `bakeSetlistDisabled` is **exported and pure** —
the cheapest possible unit test — and **nothing tests it**. So the only signal that T124's
rule works is two e2e specs that fail *because* it works. Fixing the specs without adding a
guard would leave the rule protected by nothing.

## Work

1. **Make both specs add a song before baking.** Reuse whatever `setup-helpers` already
   offers rather than hand-rolling; `add-item` is the existing test id. The specs are about
   *bake output*, so the setup should be as short as it can be.
2. **Unit-test `bakeSetlistDisabled`** — three cases, and they must be discriminating:
   `(false, 0) === true`, `(true, 3) === true`, `(false, 3) === false`. A vector where the
   correct and the naive-wrong answer agree guards nothing.
3. **Add one e2e assertion for the rule itself**: on a setlist with no songs, `bake-setlist`
   is disabled and carries the explanatory title. That is T124's behaviour, and after this
   task nothing else covers it end to end.

## Done when

- `e2e` is green, and the run reports **200 passed / 0 failed** — match the count; a green
  run with fewer tests than before is not a fix.
- The three unit cases exist and the suite total moved by exactly three.
- Reverting `bakeSetlistDisabled` to `return dialogOpen;` makes the new unit test **and** the
  new e2e assertion fail. Teeth-check it that way — by reintroducing the regression, not by
  editing the assertion.

## Notes

- Playwright cannot be run in a scratch worktree here: there is no `node_modules`, and in
  this repo a worktree's `node_modules` is shared with `main`, so installing writes through
  and prunes the shared tree. Run it from the primary checkout, or let CI answer.
- While `main` is red, a red `e2e` tells nobody anything — every genuine regression lands
  invisible behind it. That is the real cost of leaving this, and why it outranks its size.
