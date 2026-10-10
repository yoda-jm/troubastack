# Should annotations follow the text? A critical analysis

**For:** VLL and the engineering lanes · **Date:** 2026-10-11 · **Base:** origin/main `a936c93d`
**Evidence:**
- `anchoring-spec.md` Part A, which describes the code as built, with citations;
- `red-first-baseline.md`: 225 executable cells, run against main;
- the live measurements on VLL's data (no names).

## 1. The question

VLL: *would it be simpler if nothing moved (never link)?* He also says the "it follows" feature is very attractive, if it can be defined properly for every shape. Both halves deserve a straight answer:

- **Is today's linking worth keeping as it is?** No. It is worse than not linking on several common paths: it actively moves marks to wrong places.
- **Is "follow the text" worth having, if defined properly?** Probably yes, in a narrower, rigid form. But "never link" is a defensible answer at VLL's current scale, and §6 says when to choose it.

## 2. What the evidence says about today

From the matrix (`ANCHOR_RED=1`), on a synthetic one-page chart, with realistic geometry for each shape:

| family | cells red | what the musician sees |
|---|---|---|
| box, icon and scribble collapse to the line box (incl. 2 vanishing stamps, 1 text snap) | 44 | a rectangle drawn around 3 lines becomes one line; a stamp shrinks by about 40%; a small stamp beside a line **vanishes** (zero width) |
| line / arrow re-fitted to a box | 11 | the arrow **points the other way**; a straight stroke turns diagonal |
| identity by text + occurrence | 40 | a typo fix **unlinks** the mark (it freezes); a repeated chorus line inserted above makes the mark **jump to the copy**; `size:`, `fit:` or `columns:` re-wraps a long line and **unlinks** it |
| move / resize not re-linked; restyle trusts the client | 15 (+6 sync) | a mark you moved **snaps back** at the next edit; a resize is undone |
| margin marks never link | 16 | margin icons stay put while the verse moves |
| band-folder import never links | 8 | coordinates-only marks are fixed for ever (VLL's 5 icons) |

What already works:
- creating a link on or just beside text;
- far marks stay fixed;
- transposition is geometry-neutral (exact);
- deleting the linked line freezes the mark rather than moving it;
- export round trips;
- highlights follow, because their shape happens to equal the line box.

Live data agrees, and is worse:
- **13 of 15** anchored marks already disagree with their own anchor by 0.008–0.133 of the page. **Every one will move at the next edit of its chart.**
- None of this is visible in Studio.

**So the current state is not "a feature with bugs". It is a mechanism that fails silently and actively in most cells.** Whatever VLL decides, it should not stay switched on as is (see Stage 0 in §7).

## 3. The realities that decide it

1. **A solo musician edits lyrics between rehearsals.** The usual edits are:
   - fixing a typo;
   - adding or removing a line or a section;
   - adding chords;
   - occasionally changing `size:` or `fit:`.

   Any added or removed line shifts **every line below it on that page, and the pages after.**
2. **Typo fixes are frequent.** An identity that breaks on any change to a line's text breaks constantly.
3. **Repeated lines are normal:** choruses, "Oh the open road" twice, identical chord rows, identical tab staves. An identity of "the Nth line with this exact text" is wrong by design whenever a copy is inserted or removed before it.
4. **The tablet shows baked rasters.** There is no indicator on stage, and no chance to notice live. Whatever is wrong in Studio at bake time is wrong at the gig. So **silent failures are the expensive ones**, and pre-bake visibility is worth more than in-editor cleverness.
5. **The population is small today.** About 15 anchored marks plus 5 unlinked icons sit on generated charts; most of VLL's marks are on uploaded PDFs, which never move. The cost of any option has to be weighed against that, and against how much he will use generated charts in future.
6. **Layout changes don't only come from VLL.** A renderer change (the T145 incident, the T146 margin, the 13 pt migration) reflows every chart. Today the stored PDF is re-rendered only on a source save or a deliberate migration. One exception: a **transposed bake re-renders with the current binary** (`baker.go:715-729`). After a renderer change, that bake's raster can diverge from the stored blob that the marks were placed on. This is open whatever the option.

## 4. The options

The cost scale used below is S, M, L: a small, medium or large task for one lane. The **test surface** is the number of meaningfully distinct cells the option needs. The 202-cell chartpdf matrix in the patch still runs cheaply under any option; the question is how many of its cells are genuinely different behaviours.

### A. Fix the current elastic model (T191 + T191-D3 + more)

**What it is:** keep text + occurrence, and keep fitting marks into run boxes. Add:
- re-link on move (T191 ⟨D1⟩, implemented and parked);
- extent and `Span` (D3);
- special cases for lines and stamps;
- margin linking;
- import linking.

**What it still fails at:** typos (lost), duplicates (wrong copy) and re-wrap (lost), unless the identity is also replaced, and that replacement is the expensive part (option G's stage 1). It also stays exposed to:
- the "box versus centre" edge (behaviour depends on whether the centre falls on a run or in a gap; see the green-by-luck cells in the baseline);
- chord-row and lyric overlap;
- character snapping;
- one remap rule per shape.

**Failure mode:** mostly **silent**. Wrong shape, or wrong place, displayed as "fine".

**Cost:**
- without a new identity: 2M + 3S, and the typo, duplicate and re-wrap reds stay;
- with one: L + 3M + 3S.

**Test surface:** about 200 genuinely different cells, because each shape has its own projection path, and every cell must be green.

**Verdict:** the most expensive way to the least predictable result.

### B. Never link

**What it is:** delete linking. Every mark keeps its page coordinates.

**Cost:** S (switch off create-time linking and re-projection; keep the data for later).

**Predictability:** perfect for "what moves", because nothing does.

**Failure mode:**
- **silent and passive.** After any edit that adds or removes a line, every mark below it sits one line off.
- That is exactly the T145 incident shape: a mark in a plausible place on the wrong words. T145 itself calls it *"the worst shape of bug we have: it looks like nothing is wrong."*
- At gig time, the tablet shows it baked.

**Fit with the realities:**
- It fails on the most common edit (adding or removing a line).
- It is immune to typos and duplicates, because it never had an identity to lose.

**Test surface:** about 5 properties: nothing moves; the orphan guard; the round trip; P1; P32.

**Verdict:** honest and cheap, but it **moves the work to VLL's hands and his attention**, with nothing telling him when to look.

### B′. Never link + flag

**What it is:** never link, but on every source save, compare each mark's surroundings between the old and new renders. The server has both manifests at `SaveChartSource`. Flag the marks whose words moved or changed: "4 marks may no longer sit on their words", with each mark outlined in Studio. Repeat the count in a pre-bake warning.

**Cost:** M. It needs the old render's manifest at save (it can be re-rendered from the previous source), a per-mark comparison, the flag state, a Studio counter and a bake warning. It does **not** need a line identity: "the text under this box changed, or moved" is a geometric check.

**Failure mode:** **visible.** Every misplacement is announced. The fix is manual: one drag per mark.

**Test surface:** about 12 cells: each layout-changing event flags, each layout-neutral event does not, plus pre-bake.

**Verdict:** **the right answer if VLL prefers control over convenience**, or if generated charts stay a minority of his library.

### C. Freeze + one-click "re-place" review after an edit

**What it is:** B′, plus a proposed new position for each flagged mark (a ghost outline), with "apply all", "apply" or "leave".

**Cost:** B′ + the full projection machinery (G's stage 1 and 2) + a review UI and its pending state. M + L.

**Failure mode:** visible, but with friction on every edit. A solo musician will learn to click "apply all", which is automatic following with an extra step. If he skips the review, the bake shows frozen, wrong marks (so the pre-bake warning is still needed).

**Verdict:** the cost of G plus a dialog. It is worth it only for the **uncertain** cases, and that is exactly G's "to check" flag. So it is folded into G rather than built separately.

### D. Link only some shapes (for example stamps and highlights, never boxes, scribbles or lines)

**What it is:** decide per shape type whether a mark follows.

**Cost:** S on top of A or G.

**Failure mode:** the unlinked shapes fail exactly as in B (silent, on the common edit). The linked ones fail as in A or G. It trades one silent failure for a mix of two, and it is a rule VLL has to remember per tool.

**Verdict:** **no.** The per-shape problems today come from the elastic remap, not from the shapes. Under the rigid model (G), every shape behaves the same way, so there is nothing to exclude.

### E. Opt-in "pin to text" per mark

**What it is:** marks are fixed by default; the user pins the ones that should follow.

**Cost:** M (a UI affordance + B's default).

**Failure mode:** every unpinned mark behaves as in B. Discoverability is poor: he would pin only after being burned.

**Verdict:** **no as opt-in.** As **opt-out** ("unpin": this mark stays where it is), it is cheap and useful later, once states are visible (T192's dot).

### F. Link by section rather than by line

**What it is:** a mark records its section (the label plus an occurrence) and an offset inside it.

**Failure mode:**
- It survives typos and inserts in other sections.
- **Inserting a line inside the section, above the mark, shifts it silently.**
- Repeated section names ("Chorus" ×3) bring back the occurrence problem one level up.

**Verdict:** **no** as the primary identity. It is a weaker version of G's line identity.

### G. Rigid follow by line, flag when uncertain (recommended)

**What it is** (`anchoring-spec.md` Part B):
- Each mark is attached to one **line**, by a stable line id carried through edits by a line diff at save time. It is never attached by text + occurrence.
- The mark **moves rigidly** with the line: translated, and scaled only by the type-size ratio.
- It is **never reshaped.** This removes the collapse, flip, vanish and clamp families by construction.
- A typo in its own line shifts it sideways by its left-edge character.
- Anything uncertain becomes **"to check"**. A deleted line makes it **"lost"**.
- Margin marks follow vertically.

**Failure mode:**
- **Visible** for everything uncertain.
- **Silent** only where the diff mis-pairs lines. In practice that is between two identical adjacent lines, where following the "wrong" twin lands on identical words.
- Also silent where a mark straddles a lyric and the next chord row. P22's runner-up flag covers that.

**Cost:** L (line ids + manifest source index + diff at save) + M (rigid projection, linking rule, states in Studio) + 3S (re-link on move, reusing the parked T191 branch; import linking; server decides) + S–M (migration). That is about the same as A-with-identity. But each part is simpler, and the result is one rule.

**Test surface:** the 202 chartpdf cells collapse into about 26 event classes, plus 8 shape-invariance checks ("one transform, every point"), 7 sync cells, 16 bandio cells, and about 6 Studio e2e cells. That is about 60 meaningfully distinct cells, against about 230 for A.

### Comparison

| option | cost | predictability ("can VLL say where a mark will be?") | common edit: add a line | typo in the linked line | repeated chorus | `size:` change | failure when wrong | test surface |
|---|---|---|---|---|---|---|---|---|
| today | — | no | collapses / snaps back | lost, silent | wrong copy | collapse or lost | **silent, active** | (202 cells, 126 red) |
| A elastic fixed | L+3M+3S | low (per-shape rules) | ok | ok only with new identity | ok only with new identity | ok | mostly silent | ~230 |
| B never link | S | **total** | **one line off, silent** | unaffected | unaffected | **off, silent** | **silent, passive** | ~5 |
| B′ never link + flag | M | total | flagged, manual fix | flagged if moved | flagged | flagged | visible | ~12 |
| C freeze + review | L+2M | high | proposed, confirm | proposed | ok | proposed | visible, friction | ~65 |
| D some shapes | +S | low | mixed | mixed | mixed | mixed | silent | A or G + per-shape split |
| E opt-in pin | M | medium | unpinned off | — | — | — | silent for unpinned | B + G |
| F by section | M | medium | **off if inside the section** | ok | ambiguous | ok | silent | ~40 |
| **G rigid + flag** | L+M+3S | **high: "it moves up and down with its line, never changes shape"** | ok | ok, flagged if under the mark | ok | ok (scaled) | **visible** when unsure | **~60** |

## 5. The per-shape design questions, answered

1. **Should an arrow or line link its two ends independently?** **No.** Independent ends make a line stretch and rotate when the lines between its ends move. That is the elastic model again, with two identities to lose instead of one, and a third state ("one end lost"). Rigid: the line keeps its direction and length, and moves with its top line. If the lines under its two ends move apart (an insert between them), it becomes "to check" (P12/P22). VLL sees it and re-draws, which takes 2 seconds.
2. **Should a freehand scribble across 3 lines stretch with them?** **No (DECISION).** A scribble is handwriting. Stretched, it no longer looks like what he drew, and he will not recognise it. Rigid + "to check" when the lines inside it separate. The red-first test asserts this answer (cell `*/multiline_insert_inside`, marked DECISION).
3. **Margin marks** (brackets, the jump icons, "capo 2" right of a short title): **follow vertically, keep x (DECISION).**
   - Today's T172 rule refuses them, because "beside a block is not beside a run". That refusal was reasoned for an elastic projection that needs a character span.
   - A rigid follow needs only a vertical reference, so the refusal has nothing left to protect.
   - This also resolves web-core's finding: the title **is** a run, and a mark right of a short title was refused only because the title's box is as narrow as its text.
4. **When the type size changes, do marks scale?** **Yes.** With uniform scaling, a circle around a word still rings the word at 16 pt. Otherwise the circle stays 13 pt-sized around a 16 pt word, and every mark needs re-drawing after a `fit: page`.
5. **Stamps and text labels:** they keep their own size, times the type ratio. They are never fitted to the line.

## 6. When to choose "never link" instead

Choose **B′** (never link + flag), and delete the anchoring code, if any of these is true:
- VLL expects to annotate mostly **uploaded PDFs**. Today's split points that way: about 20 marks on generated charts, against several hundred on PDFs.
- He values "nothing ever moves by itself" above saving drags.
- The lanes cannot fund the L-sized line-identity work soon. **A half-built G is worse than B′:** it is today's situation again.

B′ is not a failure. It is the honest version of "never link": nothing moves, and the server says which marks to look at. Its downside is real but bounded: after adding a verse above 6 marks, he drags 6 marks.

Choose **G** if generated charts are becoming his main medium, which the size, fit, columns and tab work suggests. In that case, adding a section above a dozen marks, three times a week, is the case the feature exists for.

## 7. Recommendation

**Do Stage 0 now, whatever is chosen. Then G, staged. Fall back to B′ if VLL answers "nothing should move".**

### Stage 0: stop the active damage (S, now)

- **Freeze re-projection** at serve and bake, behind one switch, until Stage 2 lands. The data is kept. Also stop creating new old-style links.
  - **Before freezing,** persist the currently served coordinates of the **2 stale-hash marks** as their stored coordinates, with a normal mutation and a backup. Otherwise freezing would show them where they were before the last edit, which is a visible move. The other 13 already show their stored (correct) coordinates.
- **Effect:**
  - The 13 diverged marks stop being a time bomb.
  - Edits now shift text under marks below the edit (B's behaviour) until Stage 2.
  - VLL should be told this in plain words.
- **Tests:** a cell "freeze: an edit leaves every mark's coordinates exact" (shape-agnostic), and the bake orphan guard still green.

### Stage 1: line identity (L)

- Runs carry {source line, segment, character offset} (`chart.go` `rec`, inside `layout()`).
- Each file keeps stable line ids. `SaveChartSource` diffs the old and new source and carries the ids plus a per-line character map.
- Writes that bypass it (folder import, migrations) re-derive ids and mark the affected marks "to check".
- **Tests:** the identity half of P14, P15, P19 and P20 at chartpdf and app level. New cells:
  - a word inserted **before** a circled word on the same line (the P19 shift);
  - two identical adjacent lines, with one deleted.

### Stage 2: rigid projection + linking rule (M) + re-link on move and resize (S) + server decides (S) + import links (S)

- Replace `AnchorAt`, `Project` and `remap`'s box branch with P2–P7 and P8–P13.
- Reuse the parked T191 branch for the apply.go seam, re-pointed at the new link.
- **Tests:** delete the `knownRed` lines of every cell this flips. The expected result is all non-DECISION cells green, and the DECISION cells green or rewritten per VLL's answers. Add P22 (runner-up) and P27 (stale view, at the httpapi seam).

### Stage 3: visible states (M, T192 simplified)

- The four states (linked / to check / lost / fixed); VLL names the words.
- The per-file counter.
- A **pre-bake list** of lost, to-check and orphaned marks (P29, P34). The tablet cannot show them, so the bake must.
- **Tests:** T192's e2e, plus a bake-warning cell.

### Stage 4: migration (S–M)

- Re-link every mark on a generated chart from its current served coordinates with the new rule: the 15 anchored marks, the 5 unlinked icons, and any others.
- Use T191 §3b's per-mark review page (VLL approves each one), a dated backup, and a dry run first. No names at the gate.
- Old-format anchors are not interpreted after migration; their fields are dropped from the wire in a later cleanup.

### Existing tests that pin today's behaviour and must change

- `TestOnRunAnchorIsUnchanged` (`anchor_offset_t172_test.go:123`) pins on-run → the line box, which is the collapse.
- `TestSpanCoversBothEnds` (`:148`) pins elastic span stretching. It is obsolete under P12.
- `TestNoHorizontalOverlapStaysUnanchorable` (`:111`) pins the margin refusal. It inverts if P4 is accepted.
- The T146 occurrence tests (`anchor_columns_t146_test.go:113-205`) become line-id tests.
- `TestMigrateObjects_ReverseAnchorsFromCorrectRender` covers dormant code; delete both.

### What I would not do

- Land T191 ⟨D1⟩ alone. It fixes snap-back but keeps collapse, flip, typo, duplicate and wrap (web-core's own finding 3).
- Land T191-D3 alone. It is more elastic machinery that Stage 2 replaces.

## 8. Open questions for VLL (to be answered in that order)

1. **Follow or never?** G (follow rigidly, flag doubts), or B′ (nothing moves, the server tells you what to check)?
2. **Margin marks** follow their line vertically? (Recommended: yes.)
3. **Multi-line marks** never stretch, and are flagged instead? (Recommended: yes.)
4. **When the type size changes,** marks scale with the text? (Recommended: yes.)
5. **Freeze now** (Stage 0), knowing edits shift text under marks until Stage 2? (Recommended: yes.)
6. **The words** for the four states (for example "lié / à vérifier / perdu / fixe").

## 9. Unknowns, and the experiment that settles each

- Transposed bake **after a renderer change** (§3.6). **Experiment:** render the transposed source with the current binary against a blob from an older binary, and compare the run geometry.
- A chord row at the T168 wrap limit whose chords grow on transposition. **Experiment:** a chord row of exactly `monoColChars` characters, transposed C→D♭, then compare the segment count.
- Whether any non-Studio client (the tablet app) sends object mutations without the `anchor` field. That would unlink marks (sync cell `setStyle/missing_link_not_dropped`). **Experiment:** grep `app/` for the sync mutation shape.
- Diff quality on real edits: how often a line-level diff pairs a "rewritten" line VLL considers the same line. **Experiment:** replay the source revisions of his charts (`songs/*.jsonl` + chart sources, on a copy) through the diff, and count paired, unpaired and moved lines.
