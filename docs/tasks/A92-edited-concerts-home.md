# A92 — Home: "N concerts edited since their last bake →" (replaces the always-on Re-bake row)

Owner: mobile. Spec: Fable, 2026-10-21. Depends on T201 (server). Pairs with A91.

## Why (VLL)

> "I often have a rebake on home screen, I don't understand how it is picked, I have it even when nothing seems to
> have changed" → "only after edits", and "a list of modified non archived playlists (including 1 song changed)".

## Now (with A91's landing): remove the unconditional row

- **Remove Home's A42② "Re-bake «last opened»" row** in A91's landing commit, or right after it. It is shown to
  every admin with no signal, which is exactly the complaint.
- Re-bake stays available in A91's ⋯ (admin).

## With T201

- **Home, admins only, when ≥1 non-archived concert of their bands has `editedSinceBake == true`:** one quiet line,
  "2 concerts edited since their last bake →". **Nothing at all when there are none.**
- **The tap opens the list:** one row per edited concert. Each row shows its name and date, and the changes in plain
  words ("Sail: content changed · Le Sud added · order changed"), plus a **Re-bake** button with inline progress (the
  A91 bake path). A baked row leaves the list on its own (the next listing says `false`).
- **`null` (unknown, a pre-T201 bake):** not counted, not listed. Optionally an "unknown, re-bake once to start
  tracking" hint in A91's ⋯. Never as an edited claim.
- **A91's row, for admins:** when `editedSinceBake`, the state line gains "· edited since its last bake", with
  Re-bake in ⋯. The primary action does not change.

## Tests (pure)

- the count and the list come only from `editedSinceBake == true` on non-archived concerts of bands where the user
  is admin;
- `null` is never counted;
- zero → no Home line.

## Device pass (VLL)

- edit a song in Studio → Home shows "1 concert edited…" → tap → Re-bake → the line disappears;
- with no edits → no line.
