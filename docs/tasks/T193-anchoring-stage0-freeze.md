# T193 — Anchoring stage 0: freeze re-projection and stop creating old-style links

**Lane:** web-core (core) · **Status:** specced 2026-10-11, not started. **Priority: ahead of T190.**
**Size:** S. · **Origin:** VLL, 2026-10-11, on `docs/design/anchoring/`: *"B, oui pour tes recommandations,
lance le stage 0"*.

## 0. The decision this implements

VLL chose **option G, rigid follow-by-line with flags** (`critique.md` §4 G; `anchoring-spec.md` Part B),
with every recommendation in `critique.md` §8:
- margin marks follow their line vertically;
- multi-line marks never stretch, and are flagged instead;
- marks scale with the type size;
- the anchoring is frozen **now**;
- the state words are the recommended four (Studio is English: *Linked / Check / Lost / Fixed*).

Stages 1–4 follow as their own tasks. This is **stage 0**: stop today's mechanism from damaging marks while
the replacement is built.

## 1. What to change

1. **Freeze re-projection everywhere.** One switch, a package-level constant in `chartpdf`, e.g.
   `anchoringFrozen = true` (no env or config: this is not a user setting). While it is on, every serve and
   bake path returns each mark's **stored** `Points` and `Page` unchanged. Today those paths are
   `httpapi/annotations.go` (serve), `bake/annotations.go` (bake), and any other caller of
   `chartpdf.Reproject`. **Grep for all of them, and for any other place stored points are replaced by
   projected ones (the sync/ws snapshot included).** Keep `Reproject` and `Project` themselves intact:
   stage 2 replaces them.
2. **Stop creating old-style links.** With the switch on:
   - create-time anchoring is a no-op: `sync/apply.go` `KindCreate` → `anchorer.AnchorMark`, and the HTTP
     import anchoring in `httpapi/annotations.go`;
   - a new mark is stored with **no** `Anchor` and **no** `PointsRenderHash` change.
3. **Keep the data.** Existing `Anchor` fields stay stored, exported and imported exactly as today
   (`bandio_v2.go` untouched), so stage 4 can still read them.
4. **The bake orphan guard stays** (a mark on a page the chart no longer has fails the bake with its
   message).
5. **The Studio chart-editor warning** ("Editing re-renders the PDF — layout may shift…") is still true; keep
   it. Do not add any new UI in this task.

## 2. Before the freeze goes live on :8080: keep what VLL sees

Freezing switches every mark to its **stored** coordinates. For a mark whose stored hash is stale, the
position served today is its *projection*, so freezing would visibly move it. Your T191 dry run counted
**2** such marks. One of them (an anchored line whose run is gone) is already served at its stored
coordinates, so nothing changes for it. **Check each one; do not assume.**

Provide a small one-off tool (by object id, no titles in its output). It must:
- for each mark whose served position differs from its stored one, write the **served** coordinates as the
  stored ones, through a normal mutation (a new revision), with `PointsRenderHash` set to the current blob
  hash;
- **dry run first**, printing the id, the stored and served y, and the delta;
- **refuse to run against a live server** (the same lock check as `reset-password`, OPS07), or go through
  the admin socket.

Fable runs it on :8080 at deploy: stop the server, dated backup, dry run, apply, deploy the frozen binary.
Post the dry-run output (ids and deltas only) at the gate.

## 3. Tests

- **The red-first matrix lands with this task.** Apply `docs/design/anchoring/red-first.patch`.
  - Under the freeze, some cells that are green today turn red: every "follows after an edit" cell, because
    marks no longer follow until stage 2. Add exactly those to the `knownRed` tables, tagged `T193-freeze`.
  - Plain `go test ./...` must stay green, and `ANCHOR_RED=1` must show the full picture. **Report both
    counts** (today's baseline is 85 pass / 140 red).
- **New cell, shape-agnostic, not gated:** with the freeze on, an edit of the chart source (insert a line
  above, change `size:`) leaves every mark's served and baked coordinates **exactly** equal to its stored
  ones, for each of the 8 shapes.
- **New cell:** a mark created with the freeze on carries no `Anchor`.
- **Teeth:** set the switch to false. The freeze cell goes red. Print the line.
- **The existing tests that pin projection** (`TestOnRunAnchorIsUnchanged` and the others listed in
  `critique.md` §7) test `chartpdf` functions directly, so they stay green. Leave them; stage 2 rewrites them.
- **Regression:** `go test -race ./...`, gofmt, go vet, and the studio e2e for annotations and bake are green.

## 4. Not in this task

- Line identity (stage 1), the rigid projection (stage 2), the states UI (stage 3), and the migration (stage
  4). Each gets its own task.
- Landing the parked T191 branch. Stage 2 reuses its apply.go seam.
