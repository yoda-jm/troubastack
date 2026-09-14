# A75 — The Stage Notes tab, re-specced: three quarters of the page is headers

**Lane:** mobile · **Status:** specced, not started · **Owner of the design:** architect (handed over by the
mobile lane, 2026-09-14). Supersedes `proposals/stage-notes-tab-redesign.md` and absorbs
`proposals/stage-notes-sent-recycle-bin.md`.

**Origin:** VLL, twice. First *"look at the page, you can only display 2 notes"* — which produced a
single-row stopgap — and then again after it: *"la page troubastage notes est toujours tres tres tres peu
compacte, tout est ecrit de base, a la fin y'a que la place pour 2 ou 3 notes en mode paysage sur ma
tablette."* **The same complaint survived a compaction pass**, which is the reason this spec starts with a
measurement instead of a layout.

## 1. The measurement — his device, his notes, landscape

Taken over adb with his permission, read-only, on the live Notes tab.

| | |
|---|---|
| Panel | 1200×1920 px @ 280 dpi → factor **1.75** |
| **Landscape height** | **686 dp** — a phone in portrait has ~890 |
| Usable list area | ≈ **521 dp** (below the tabs, above the 16 dp nav bar) |

**Cost of ONE note**, measured top-of-band-header to top-of-next-band-header:

| Block | px | dp | |
|---|---:|---:|---|
| band header | 96 | **55** | |
| concert header | 81 | **46** | holds a "Send all" button |
| song title | 60 | **34** | |
| note leaf + gap | 74 | **42** | holds See / Delete / Re-send |
| **total** | **311** | **178** | **the leaf is 24 % of it** |

521 ÷ 178 = **2.9 notes**. His "2 or 3" was exact.

**Measured touch targets: 48.0 dp**, on the buttons in both the concert header and the leaf. That is the
Material minimum interactive size, and it is why the stopgap did not help: the previous pass set
`contentPadding = PaddingValues(horizontal = 10.dp, vertical = 0.dp)` on those `TextButton`s, and
**`contentPadding` cannot take a row below the minimum touch target.** The leaf's text is 16 dp inside a
48 dp floor. Nothing in this repo has ever overridden that floor.

## 2. ⟨D1⟩ A header with one child is not a header — fold it away

On his device **every level had exactly one child**: a band holding one concert, holding one song, holding
one page. Three header rows, 135 dp, conveying nothing a reader could not get from the leaf.

**Rule: a grouping level renders a header only when it has two or more children.** With one child, fold its
label into the child's line (or drop it when the parent above already names the context). This is the single
biggest win available and it costs no legibility — a header that groups one thing is decoration.

Keep the collapsible tree for the case it was built for: several concerts, many notes. The rule must be about
the *data in front of the user*, not a setting.

## 3. ⟨D2⟩ The leaf is a line of text, not a row of buttons

Three buttons hold the leaf at 48 dp while its content is 16 dp. **Do not fix this by shrinking the touch
targets** — the reader is at arm's length under stage light, and 48 dp is already the accessible floor.
Remove the buttons from the row instead:

- **Tapping the row is "See"** — looking at the note is the ordinary act, and it is currently a button
  competing with two others.
- **Send and Delete leave the row.** Selection-reveals-actions, an overflow, or a swipe: the lane's choice.
  What must hold is that **the resting state of a leaf contains no button**.
- Sending is **secondary** — VLL: *"the send to studio are less important"*. The primary content is the note,
  its page, and its state.

## 4. ⟨D3⟩ Sent notes step back — A74 ⟨D2⟩ is hereby revised

A74 ⟨D2⟩ said *dimmed, not collapsed*, reading his *"still displayed probably"*. **I ruled that before I had
measured anything.** On a 686 dp panel a dimmed row costs the same as a bright one, and the notes with no
remaining tablet-side action are the ones paying for the notes that have one.

**Revised: the "Sent" group is collapsed by default, with its count on the header** — *"Sent (4)"*. That is
still displayed, it still opens in one tap, and it stops finished work from consuming a screen that fits
three things. ⟨D1⟩ applies here too: with no sent notes, no header.

Everything else in A74 stands unchanged — the name **"Sent"** and not a bin (⟨D1⟩ there), **no bulk clear**
(⟨D3⟩), membership is `sentAt != null` and nothing new (⟨D4⟩), and the `updatedAt` clock defect (⟨D6⟩).

## 5. ⟨D4⟩ What must not be lost

- **The two lifetimes.** A sent note is still live in Studio; deleting the tablet copy does not delete the
  Studio underlay, and *Done, remove* never reaches the tablet (T170 §7).
- **The nag stays scoped to unsent notes** (`NoteIndex.isOld`), so the division holds: the tablet nags what is
  unsent, Studio surfaces what is unrecopied (T173 ⟨D6⟩).
- **State must read without parsing a row.** *"page 1 · sent"* is currently the only state signal and it sits
  in the same 16 dp line as everything else.

## 6. Acceptance — measured, not judged

The claim to beat is a number, and it is measured the same way it was measured here: `uiautomator dump` on a
686 dp landscape panel, counting notes whose leaf is fully on screen.

- **At least 8 notes visible** where 2.9 fit today, with the notes spread across **two bands, two concerts,
  two songs** — the shape that produces the worst header overhead, and the shape his device is actually in.
- The leaf's resting state contains **no button**, and no touch target anywhere on the page is below 48 dp.
- With a single band, single concert and single song, **no grouping header is drawn at all**.
- Report the before/after dp per note, not an impression.
