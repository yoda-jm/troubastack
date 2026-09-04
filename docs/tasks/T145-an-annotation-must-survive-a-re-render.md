# T145 — An annotation must stay on the words it was drawn on

**Lane:** to be routed by first stage (core, then studio + app). **Size:** M/L — a data-model change.
**Status:** spec, 2026-09-05. Filed from the rehearsal field report, items #4/#5.

## The failure, in the musician's words

VLL, mid-rehearsal: *"en scrolling les annotations ne sont plus alignés"*, and about one chart:
*"le trait rouge devrait etre sur la fin, alors qu'il est beaucoup plus bas."* Then, decisively, after the
afternoon bake was recovered from the tablet: *"le bake de cet apres midi a la bonne taille et
l'annotation du trait est bien sur le Verse 5, donc quelquechose a changé."*

He was right, and the proof is two bundles of the same song:

| | page 2: where ink stops | the mark |
|---|---|---|
| 17:46 bake | `0.409` | `0.328 → 0.424` — **exactly at the end of the text** |
| 22:20 bake | `0.051` | `0.328 → 0.424` — **orphaned in blank space** |

**The mark never moved. The words moved out from under it.** Every annotated song reflowed the same night;
one chart's mark that sat at the last line (`0.744`, text ending `0.754`) now floats mid-page (text ending
`0.949`), one overlay's own extent changed, and **one overlay vanished from the bundle entirely**.

## Root cause

An annotation stores **`(page index, fractional x/y)` of a particular render**. The render is regenerated
on every bake and every import, and its layout is not pinned (**T144**). So any reflow — a renderer
change, an auto-fit change, one added lyric line — **silently re-points every mark on the page**.

This is the worst shape of bug we have: **it looks like nothing is wrong.** A musician sees a mark in a
plausible place and trusts it.

## The decision this task must make

Pick one and **state why in the commit**:

1. **Anchor to the source** — store a line/character offset into the chart source, project to page
   coordinates at render time. Survives reflow by construction; the most work, and needs an answer for
   marks on an *uploaded* PDF, which has no source.
2. **Re-anchor on re-render** — keep page coordinates, but record the identity of the render they were
   drawn against; when the render changes, remap old coordinates to new ones.
3. **Detect and warn** — store the render identity and flag marks whose render no longer matches.
   Fixes nothing, but replaces a silent wrong answer with a visible one.

**(3) is acceptable only as an explicit stopgap**, and only if it lands with the task that follows it
named. Shipping (3) alone and calling this closed would leave the musician exactly where he started.

Note the asymmetry that makes (1) attractive: **for generated charts the folder bytes ARE the source**
(the standing ruling from T134), so a source anchor is available for every chart in the library. Only
sourceless uploaded PDFs need the fallback.

## ⟨R1⟩ Red first, and it must be the real failure

The assertion has to reproduce **the reflow**, not merely re-read a stored mark:

1. Commit a fixture chart source and a mark anchored to a known word near the end of a page.
2. Render it, then **change a layout metric** (or use T144's two renderer builds) so the content reflows
   onto a different page — the same event as 2026-09-04.
3. Assert the mark still covers **the same words**. On today's code this fails: the mark keeps its page
   fraction and lands on different text, or on none.

**Teeth-check:** the expected value must differ from what the buggy code produces. A test asserting "the
mark is still at 0.328" would pass today and guard nothing — that *is* the bug.

Then, separately: **an overlay must never silently disappear from a bundle**. One did. Assert the bundle
carries an overlay for every song that has one, and watch that fail if the overlay is dropped.

## Acceptance

- The chosen option is implemented and named in the commit, with its reason.
- The reflow test above is **seen red first**, then green.
- A bundle keeps one overlay per annotated song; a missing overlay fails the build, not the rehearsal.
- Existing marks in the live library are addressed explicitly: either migrated, or reported as
  un-migratable with a count. **Do not silently drop them.**
- Studio and Stage agree on the anchor — whatever is stored must render identically on both.

## Evidence — frozen, do not touch

The proof lives in `/home/yoda/troubastack-evidence/rehearsal-2026-09-04/` (the recovered 17:46 bundle),
the running `:8080` instance's data directory, and the tablet itself. **All three are read-only for now**
— see the gate note. Reproduce on fixtures, never by re-baking the live instance.
