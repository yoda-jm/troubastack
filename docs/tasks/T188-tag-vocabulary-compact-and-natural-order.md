# T188 — The band's tags: a compact panel, and one natural order everywhere

**Lane:** web-core · **Status:** specced 2026-10-09, not started; the three choices in §2 were taken by VLL on
2026-10-09 · **Origin:** VLL, 2026-10-09, after loading a real choir repertoire with many tags: *"when we have
a lot of tags the vocabulary of a band takes too much space, maybe one line per tag is not good, also are they
ordered properly? same question for the order of the tags displayed in the list of songs"*.

**Builds on:** T182 (the Tags panel, landed) and T181 (row pills, chips, refine strip, landed). This task
changes their **layout and ordering only**. Rename, merge, delete and their confirmations (T182 §4–§6) are
unchanged.

## 1. What is wrong today (measured, not guessed)

Measured on a real band: 67 songs and 19 tags, 13 of them "kind" tags (`tv`, `movie`, `anime`, …) plus a
**season family** `s1`…`s6`, one season per song.

- **The panel is one full-width row per tag**, each row with its own Rename and Delete buttons
  (`TagsPanel.tsx`, `.tag-list` is `flex-direction: column`). With 19 tags it is 19 rows, the tallest panel on
  the band page, and that height is spent on buttons the reader rarely uses.
- **The panel order is count-descending** (server `TagCounts`, `core/internal/app/tags.go`). That order
  scatters any family of tags: the panel reads `tv 24, movie 21, s2 15, s1 14, video-game 14, pop 11, s3 11,
  s4 10, s6 9, anime 8, cartoon 8, s5 8, …`. The seasons come out as 2, 1, 3, 4, 6, 5.
- **Every tie-break is raw byte order** (Go `<`, and TS `a.label < b.label` in `song-search.ts`, `tagInput.ts`).
  So `s10` sorts before `s2`, `Encore` and `encore` are not neighbours, and `écran` sorts after `zoo`.
- **Row pills are ordered by band-wide count** (`rowPills` with `bandTagRank`, `song-search.ts`). Each tag
  keeps a stable place, but a family does not. One row reads `tv, s2, video-game`, another `tv, video-game, s4`.
  The season moves between the second and third column depending on its count.

## 2. VLL's choices (2026-10-09)

He was given three options for each question. He chose:

- **⟨D1⟩ Panel layout: a chip cloud, plus a Manage mode.** Tags wrap as compact chips, `name count`. A click
  filters the song list, as T182 §3 does today. A **Manage** toggle in the panel head switches to the
  T182 rows with Rename and Delete.
- **⟨D2⟩ Panel order: natural A–Z by default, with a toggle to "Most used".**
- **⟨D3⟩ Row pills: natural A–Z**, the same order as the panel's default.

He did **not** choose any grouping of tag families: no `season:` prefixes and no detecting `s1…s6` as a
series. Natural A–Z already keeps `s1…s6` together and in order. **Do not add family detection.** It would be a
heuristic standing in for an intent nobody stated.

## 3. ⟨D4⟩ One comparator: "natural order"

Add **one** exported function, for example `compareTagNames(a, b)`. Put it next to `foldText` or in
`band-tags.ts`. **Every tag order below uses it.** Its properties:

1. **Case- and accent-insensitive first.** Compare the `foldText` forms, so `Encore`/`encore` and
   `Écran`/`ecran` sit next to each other, and `écran` sorts among the e's.
2. **Number-aware.** A run of digits compares by value: `s2 < s10`, `track 9 < track 10`.
3. **Deterministic on a fold tie.** When two different stored spellings fold equal (`Encore`, `encore`), break
   the tie on the raw string's code units, so the order never depends on input order.
4. **The same result in every browser and in CI.** If you use `Intl.Collator`, pin the locale (for example
   `"en"`, `{ numeric: true, sensitivity: "base" }`). Do not use the default locale, because CI and VLL's
   browser may differ.

Where it applies:

| Place | Primary key | Tie-break |
|---|---|---|
| Tags panel, A–Z mode (default) | `compareTagNames` | (property 3) |
| Tags panel, Most-used mode | count, descending | `compareTagNames` |
| Row pills (`rowPills`) | `compareTagNames` (**replaces band count**) | (property 3) |
| Search suggestions (`suggestForWord`) | count, descending (unchanged) | `compareTagNames` |
| Refine strip (`refineStrip`) | count, descending (unchanged) | `compareTagNames` |
| Song-details cloud (`tagInput.ts` `cloud`) | count, descending (unchanged) | `compareTagNames` |

Suggestions, the refine strip and the song-details cloud stay count-first on purpose. There the count answers
"how many results will I get" or "what do we usually use", and these lists are bounded. Only their byte-order
tie-break changes.

**The server's `TagCounts` order is not changed.** The panel sorts on the client. Leave the Go sort as it is,
and state in a comment in the panel that it does not rely on the server's order.

`bandTagRank` loses its only caller (`rowPills`). Delete it rather than keep dead code. `rowPills` no longer
needs a rank argument.

## 4. ⟨D1⟩ The panel: chips by default, rows in Manage mode

**Head:** `Tags`, the count (as today), then on the right:
- the **order toggle**: a two-option segmented control, *A–Z* | *Most used*, default *A–Z*;
- **Manage**: a toggle button (`aria-pressed`). *Done* (or the pressed state) returns to chips.

**Chip mode (default):**
- One wrapping line of chips per row of panel width (`flex-wrap: wrap`), each a button `name` plus a quiet
  count. The look should match the existing `.tag-cloud-item` / `.tag-cloud-count` used by the refine strip,
  so the page has **one** chip look, not two.
- A click does what a tag name does today: it adds a T181 chip to the song search and scrolls to the list
  (T182 §3, `onPick`).
- **No Rename/Delete buttons in this mode.**
- **No cap and no "+N more" in this task.** At this size the chips are the compact form. Revisit only with a
  measured case.

**Manage mode:** the existing T182 rows (name, count, Rename, Delete), **in the order the toggle selects**.
Rename, merge, delete, their dialogs, the server's number reported afterwards, and the refetch all stay
exactly as T182 specifies.

**The empty state** (*No tags yet — add them from a song's details.*) is unchanged and shows in both modes. Hide
the Manage and order controls when there are no tags.

**Remembering the choices.** Remember the order toggle per browser in `localStorage` (one key, wrapped in
`try/catch`, with a broken storage meaning "default A–Z"). **Do not remember Manage**: every page load opens
in chip mode, because Manage is a cleanup mode and not a way of reading the panel.

**Test IDs.** Keep `tags-panel`, `tag-list`, `tag-row`, `tag-row-name`, `tag-row-count`, `tag-rename` and
`tag-delete` on the **Manage-mode** rows, so T182's e2e keeps its meaning. Add `tag-chip` (and `tag-chip-count`)
for chips, `tags-manage` for the toggle, and `tags-order-az` / `tags-order-used` for the segmented control.
`band-tags.spec.ts` must click `tags-manage` before using `rowFor(…)`. **Update that spec. Do not loosen it.**

## 5. ⟨D3⟩ Row pills

`rowPills` orders a song's tags with `compareTagNames`. The rest stays as T181 ⟨D6⟩ specifies: at most 3
pills, then `+N` reveals the rest on that row, the active-chip tint, and click toggles the filter. Update the
code comment that justifies the current order ("ordered by BAND usage … so the same tag sits in the same place
row to row"). A–Z keeps that property, because it is also one global order.

**Known consequence, accepted:** with a 3-pill cap, the tag hidden behind `+N` is now the alphabetically last
one, not the least used. Making the cap width-based instead of a fixed count is **not in this task**.

## 6. Not in this task

- Tag families, prefixes or categories (§2).
- Changing the server's `GET /tags` order.
- A width-based pill cap, and any cap or "+N more" on the panel's chip cloud.
- Tags on Stage (mobile).
- Persisting Manage mode, or remembering the order toggle per account (in `localStorage` is enough).

## 7. Acceptance

**Comparator (`web/studio/test/`, unit):**
- **These vectors tell natural order apart from the naive orders:**
  `["s10", "s2", "S1", "tv", "écran", "Encore", "encore", "zoo"]` sorts to
  `["écran", "Encore", "encore", "S1", "s2", "s10", "tv", "zoo"]`. Checked with
  `Intl.Collator("en", { numeric: true, sensitivity: "base" })` over the folded forms, plus the raw
  tie-break.
  - Plain `<` gives `["Encore", "S1", "encore", "s10", "s2", "tv", "zoo", "écran"]`: three wrong.
  - Folding without numeric collation gives `… "S1", "s10", "s2" …`: `s10` vs `s2` wrong.
  - Property 3 settles `Encore` before `encore`.
- The same vector **shuffled** (at least two different input orders) gives the identical output (property 3).
- **Teeth.** Swap `compareTagNames` for `(a, b) => (a < b ? -1 : a > b ? 1 : 0)`, confirm the test goes **red**,
  and print the swapped line to prove the swap landed. Restore it.

**Row pills (unit, `song-search.test.ts`):**
- A song tagged `[tv, s2, video-game, cartoon]` in a band where `tv` is the most used tag shows
  `cartoon, s2, tv` and `+1`. The old rule would show `tv` first, so this vector tells the two rules apart.
  **Assert in the fixture that `tv` is the band's most-used tag**, or the vector proves nothing.
- Update the existing `rowPills` cases that encode band-count order. Do not delete them.

**Panel (e2e, `band-tags.spec.ts`):**
- Use a fixture with a family that count order would scatter: at least `s1`, `s2` and `s10`, with counts
  chosen so that count-descending order is **not** `s1, s2, s10` (for example `s2` > `s1` > `s10`).
  - In the default mode, the chips read in the order `s1, s2, s10` among the rest.
  - After switching to *Most used*, they read in count order.
  - After a reload, the *Most used* choice is still selected (localStorage).
- In chip mode, **no** `tag-rename`/`tag-delete` is in the DOM. After clicking `tags-manage`, the rows appear in
  the selected order.
- The existing T182 rename, merge and delete flows pass through Manage mode, still asserting the **stored**
  tags, as T182 §8 requires.
- A chip click adds the search chip and filters the list (T182 §3), including in a band with fewer than 12 songs.

**Size (pixels, not theory).** With a 20-tag fixture at a 1280×800 viewport, light and dark themes:
- The chip-mode panel body is **≤ 40 %** of the Manage-mode panel body's height. Measure both in the same run
  with `boundingBox()`, and print both numbers in the PR.
- Attach the two screenshots, chip mode and Manage mode, to the gate entry.

**Regression.** `npm run test` (studio unit), the studio typecheck, and the `band-tags` and `song-search` e2e
are green.
