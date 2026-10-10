# T191 — A mark you move stays where you put it when the chart text changes

**Lane:** web-core (core) · **Status:** specced 2026-10-10, not started. **High: it moves VLL's marks.** ·
**Origin:** VLL, 2026-10-10: *"j'ai rajouté une section dans le lyrics, et mon capo a bougé (là je l'ai remis
où il doit être), regarde ces problèmes de stabilité des annotations quand on change le texte, ça ne devrait
pas arriver"*.

## 1. What happened (traced on :8080, 2026-10-10)

The song is one of VLL's real charts (`lyrics.txt`), a generated chart with **no** `size:`/`fit:`/`columns:` header, so the
type size is fixed. Auto-size played no part. VLL added an empty `## Intro` section above the first verse.

The mark is a text mark near the top of page 0. Its log (`data/songs/<song>.jsonl`) shows:
- **Create** (rev 1): `Points` y≈0.080. `Anchor` = the first verse line, occurrence 1, with
  `Offset.RelY0 = −2.25`: "2.25 run-heights above the first verse line". That is T172's nearest-run
  fallback.
- **Move** (rev 6, and again today at rev 8 after the text edit): the `Points` change (y≈0.075, then ≈0.074),
  but **the `Anchor` and its `Offset` are byte-identical to the create's.** Only `PointsRenderHash` follows
  the render.

**Why it moved:** `Reproject` (`chartpdf/anchor_project.go`) recomputes a stale mark's position **from its
`Anchor` alone**; it never looks at `Points`. Adding `## Intro` pushed the first verse line down, so the mark
was re-projected 2.25 run-heights above the verse's **new** position. That is lower than where VLL had put
it. **Every manual move is thrown away at the next text edit**, because a move never updates the anchor.

**Why the anchor is never updated:** anchoring happens only at **create**
(`sync/apply.go` `KindCreate` → `anchorer.AnchorMark`, and the import path in `httpapi/annotations.go`).
`AnchorObject` returns early when `o.Anchor != nil`. The move, resize and setStyle branch in `apply.go` takes
the client's object as is, old anchor included.

**How many marks this affects on :8080 today:** **15** live anchored marks across the library, of which
**10** were moved or resized after they were anchored. Each one carries a stale anchor and will jump at the
next text edit of its chart. The rest of VLL's marks sit on uploaded PDFs, which have no anchors and are not
affected.

## 2. The fix

**⟨D1⟩ A move or resize re-anchors the mark from where it now is.** On `KindMove` and `KindResize` (both the
realtime path in `sync/apply.go` and any HTTP path that moves objects), when the mark's file is a generated
chart:
- Recompute the anchor from the **new** `Points` against the render the client was looking at. That render
  is the **current** one when the mutation's `PointsRenderHash` equals the current render hash. Use the same
  `AnchorAt` as create: centre-on-run first, then the T172 nearest-run-with-overlap within 3 run-heights.
- **The new position is anchorable:** replace `Anchor` (and its `Offset`/`Span`) with the recomputed one, and
  stamp `PointsRenderHash`.
- **The new position is not anchorable** (moved into empty space, more than 3 run-heights from any
  overlapping text): **clear the anchor.** The mark keeps its coordinates and never re-projects. The user
  explicitly put it there, and a stale anchor would drag it back to a place they moved it *away* from. That
  is the T172 refusal, applied to an explicit placement.
- **The client's `PointsRenderHash` is not current** (a move made against an older render, for example a
  stale tab): do **not** re-anchor against a render the user never saw. Keep the old anchor and say so in a
  comment. This is an edge case, and the next move fixes it.
- **The server decides the anchor, never the client.** Ignore any `Anchor` a client sends on a move or
  resize, as create already does.

`setStyle` and `setText` keep the anchor: they don't move the mark.

**⟨D2⟩ Check what a header-area mark anchors to.** This mark was placed near the title/artist block, yet
anchored to the first **verse** line, 2.25 heights below. Either the title and artist lines are not runs in
the T95 manifest, or the mark does not overlap them horizontally. **Dump the manifest for this chart and
report which.**
- If the header lines are missing from the manifest, add them as runs (they are text the mark sits beside),
  so a mark by the title follows the title.
- If they are present but don't overlap horizontally, report it and leave it. After ⟨D1⟩, a re-placed mark
  re-anchors to whatever is nearest at its new position anyway.

## 3. The 10 marks already carrying a stale anchor

A **one-off repair**, run against VLL's :8080 data **only with his go-ahead** and with a dated `app.json` +
`songs/` backup first:
- For every anchored mark whose latest `PointsRenderHash` **equals its file's current render hash**, its
  `Points` are exactly what VLL sees today, so **re-anchor from those `Points`** with ⟨D1⟩'s rule. Write it
  as a normal mutation (a new revision), not a log rewrite.
- A mark whose hash is **stale** (its file changed since it was last placed) shows today where `Reproject`
  puts it. **Leave it and list it.** VLL may want to look at those.
- **A dry run first:** print each mark (song, file, object kind, old anchor run → new anchor run, current y)
  without writing. Post the dry run at the gate. **No song titles or lyrics in the gate entry.** Refer to marks
  by id; the titles go to VLL directly.

## 4. Not in this task

- Anchoring marks on uploaded PDFs (they have no source text).
- Changing T172's 3-run-height radius.
- Moving a mark when its own anchor run is edited away. That keeps today's behaviour: frozen coordinates,
  flagged.

## 5. Acceptance

**Go unit (chartpdf / sync):**
- **The reported failure, end to end:**
  1. Create a mark 2 run-heights above line L1 (it anchors to L1 with an offset).
  2. Move it up to beside the title line. It re-anchors to the title (or, per ⟨D2⟩, to whatever is nearest
     there).
  3. Change the source to insert two lines above L1, and re-render.
  4. `Reproject` leaves the mark where it was moved to, relative to the title, **not** 2 heights above L1's new
     position.
- **Teeth:** remove the re-anchor in the move branch, and that test goes red (the mark follows L1). Print the
  removed line.
- A move into empty space (> 3 run-heights from any overlapping run) **clears** the anchor. A later re-render
  leaves the mark exactly in place.
- A move carrying a **stale** `PointsRenderHash` keeps the old anchor.
- A client-sent `Anchor` on a move is ignored. Send one pointing at a different line; the stored anchor is the
  server-computed one.
- A resize re-anchors like a move. `setStyle` and `setText` leave the anchor untouched.

**Studio e2e (realtime path):** on a generated chart:
1. Draw a highlight on line 3.
2. Drag it onto line 6.
3. Edit the source to insert a line above line 3, and save.
4. Reload. The highlight is on line 6's **words**, not on line 3's.

**The repair (§3):** the dry-run listing posted at the gate. After VLL's go, the real run's counts (re-anchored /
left-stale / skipped) and the backup path.

**Regression:** `go test ./...` (with `-race`), gofmt, and the existing anchor tests (T145, T146, T172) are
green.
