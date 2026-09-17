# T176 — The chart editor's route drops the setlist context one level further in

**Lane:** web-core · **Status:** specced, not started · **Priority:** low — consistency, not a report.
**Origin:** web-core flagged it while reworking T175 ⟨D5⟩, deliberately out of that scope.

## The gap

T175 ⟨D5⟩ put the reader's origin in the path: a song reached from a setlist lives at
`/bands/:b/setlists/:sl/songs/:s`, and Back is the path minus two segments.

`/bands/:b/songs/:s/chart/:fileId` nests under the song's **flat** address. So:

```
setlist → song (nested, Back = setlist ✓) → edit chart → Back → song at its FLAT address
                                                              → Back → band ✗
```

The context survives one hop and dies at the second. It is the same defect VLL reported, one level down.

## The decision this needs

**Not** simply a fourth route (`/bands/:b/setlists/:sl/songs/:s/chart/:f`). That multiplies: every future
parent doubles the route table, which is how two idioms became a divergence in the first place.

The question is whether the chart route should be **relative to the song's current address** — a nested
route definition that inherits whatever addressing the song has — so the idiom *composes* instead of
multiplying. That is a router-structure decision, not a link fix, which is why it is its own task.

## Constraint

The flat chart address stays reachable, for the same reason the flat song route did: existing links and
bookmarks point at it.

## Acceptance

- From a setlist, open a song, edit a chart, press Back twice: **the setlist**, not the band.
- The flat path still resolves.
- The route table does not gain a route per parent — if the chosen design does, say why it is the
  cheaper of the two evils.
