# T82 — "My files": a checkbox must never move or resize the row it is on

**Priority:** high (VLL on the live app: *"clicking/unclicking a file checkbox is not stable — the
order changes and the layout also changes; it's not practical"*) · **Size:** M · **Area:**
`web/studio/src/pages/song-editor/MyFilesEditor.tsx` (+ styles, e2e). **No server change.**

## The diagnosis holds — I verified all three

1. **The row relocates.** The panel renders two groups: `includedFiles` (my `order[]`) then
   `excludedFiles` (pool order). Toggling moves the row you just clicked between groups, and
   *checking* appends to the **end** of the included group (`[...order, id]`), so a file you
   re-include teleports away from where it was.
2. **The row reshapes.** Included rows carry an `actions` span with ↑/↓; excluded rows have none.
   Confirmed in the markup — so the row's height/width changes as you tick it.
3. **Every toggle is a blocking round-trip** that flips `busy` and re-seeds `order` from the
   response.

## Ruling

The instability exists because **position is derived from inclusion**. Fix that, and all three
symptoms go with it. Take **option (B), decoupled** — and note the real prize is that inclusion and
order stop being the same axis:

1. **One list, one order, inclusion orthogonal.** Render **all** pool files in a single list. A
   checkbox toggles membership **in place**: it never moves the row and never changes its geometry.
2. **The display order is computed once per load**, then frozen for the session: my included files
   in my stored order, followed by the remaining pool files in pool order. After that, **only an
   explicit reorder changes positions** — never a toggle. This keeps load-time grouping (which is
   fine and informative) while eliminating toggle-time movement (which is the complaint).
3. **Re-including restores the row where it sits**, not at the end — which falls out of (1)–(2) for
   free and is the half of the bug that most annoys.
4. **Reorder is explicit**, via the shared `useSortable` grip from T78 **plus** the existing ↑/↓.
   Keep `my-files-up` / `my-files-down` attached to the equivalent elements (T78/T80 guard), and
   keep them for the same reason as T78: a working path when drag isn't available.
5. **Uniform rows.** Grip and move controls render on **every** row regardless of checked state
   (disabled at the ends rather than absent). Excluded rows differ **only** in colour/opacity —
   never in geometry.
6. **Optimistic persistence.** Flip the checkbox in local state immediately, fire the PUT, and do
   **not** disable the list while it is in flight. On failure, revert that one row and surface the
   error.

**One correctness point that outranks the flicker:** the current code re-seeds `order` from *every*
response. Under fast toggling that is a **lost update** — a slow earlier response overwrites newer
local state, and since `setMyFiles` replaces the whole list, the user's latest intent is silently
discarded. Sequence the requests (request id / last-write-wins) and **ignore responses from
superseded requests**. Do not treat this as cosmetic.

**No API change.** `setMyFiles` continues to store the ordered list of **included** ids. Positions of
*excluded* files are therefore session-local: after a reload they fall back to pool position. State
that plainly in the code comment rather than implying otherwise — if persistent full ordering is ever
wanted, it is an API change and its own task.

## Acceptance criteria

- **The complaint is the test.** An e2e that captures a row's index **and bounding box**, toggles its
  checkbox, and asserts **both are unchanged** — for a row in the middle of the list, and for one at
  each end. This is the crisp form of "not stable"; it must fail against today's code.
- Re-including a file within a session returns it to its original position, not the end of the list.
- Row geometry is identical checked vs unchecked (assert equal bounding-box height/width on the same
  row across a toggle).
- **No control is disabled during a toggle's round-trip**, and a rapid double-toggle settles on the
  correct final state — assert the persisted selection after the dust settles, with the responses
  arriving out of order if you can force it (this is the lost-update guard, and the reason it exists).
- Drag reorder and ↑/↓ both persist and survive a reload; the ends' move controls are disabled, not
  missing.
- Existing testids (`my-files-panel`, `my-files-row`, `my-files-include`, `my-files-up`,
  `my-files-down`, `my-files-reset`) stay attached to equivalent elements. Run the **dangling-testid
  sweep** and the **full** e2e suite (it runs on the isolated ports since T81 — no excuse for a
  subset).
- Before/after screenshots of the panel, checked and unchecked, in the handoff.
- `tsc -b studio` clean.

## Out of scope

- Any change to the `my-files` API or to what a bake contains.
- The band-wide files × members matrix (parked).
- Changing what "my order" *means* downstream (default part, strip order, per-member bake).
