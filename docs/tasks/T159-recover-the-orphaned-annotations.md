# T159 — recover the annotations orphaned by the September re-seed

**Lane:** core (web-core). **Kind:** bug — silent data loss, already happened.
**Number claimed** in the same push as this file. VLL asked for this after I measured it.

**Status (web-core):** TOOL LANDED + APPLIED-TO-A-COPY 2026-09-06 — at the gate. `cmd/recover-annotations` (+ pure `internal/annrecover`): match by band+title (not id), refuse ambiguous, copy absent-UUID objects EXACTLY, never anchor, report by id/index. RED-first unit tests (idempotent, teeth on already-present + never-anchors + tombstones + ambiguous-abort). Dry-run vs `data.preseed-20260904-191837` restores **3 pointed marks incl. the 150-point freehand** into live song (idempotent re-run: 0 remaining) — the visible recovery Fable's done-criteria names (15→18). Applied to a COPY (`data.t159-*`), not the served store; ready for swap-in. NOTE: I see 1 orphaned stream / 3 visible marks in the CURRENT store, not Fable's 3 streams / 13 objects — the extra are point-less objects and/or streams re-attached by a later re-seed; flagged at the gate.

## What was lost, measured

Comparing the frozen store copy `data.preseed-20260904-191837` against the live store, **by object UUID
across every annotation stream**:

| | archive (04-09) | live | absent |
|---|---|---|---|
| annotation objects | — | — | **13** |
| of which carry points (visible marks) | 18 | 15 | **3** |

The 13 sit in **three** archived streams that did not carry across at all or only partly — 0 of 8, 0 of 2,
and 1 of 4. The other seven streams carried **every** object. One of the three lost marks is a **150-point
freehand**; none is flagged deleted.

**This is not a deletion.** The annotation streams are keyed by **song id**, and the 2026-09-05 re-seed
changed the band ids (`0cf20569`/`b3a2f7b7` → `a0f94a1b`/`db913e1c`) and therefore the song ids. Most
streams were re-attached under the new ids; three were not. T150 stopped the churn **going forward** — it
did nothing for the marks already orphaned by it.

## What makes recovery possible

**The songs still exist.** All ten annotated songs from the archive are present live **by title**, and the
one I traced maps to exactly one live song, which already has its own live stream. So each orphaned stream
has an identifiable target.

## What to build

A one-shot recovery tool, in the same shape as the T145 runner (`cmd/migrate-anchors`): **dry-run by
default**, `--apply` writes, operate on a **copy** with the server stopped (filerepo is single-writer).

1. **Match archived stream → live song by (band, title)**, not by id — ids are exactly what churned. The
   band mapping is one archived band id to one live band id; derive it, do not hardcode it.
2. **Refuse an ambiguous match.** Two live songs sharing a title, or none, must **abort that stream** and
   be reported. Never guess which song a mark belongs to.
3. **Copy only objects whose UUID is absent from the live target stream.** An object already present is
   left untouched — the tool must be safe to run twice.
4. **Preserve the object exactly**: points, page, style, layer, owner. It is VLL's hand, not ours to
   normalise.
5. **The layer must exist on the target.** If the archived object's `LayerID` has no counterpart live,
   create it or map it — and say which in the report. A mark on a layer nobody can see is not recovered.

## ⚠ The trap this shares with T145

**Do not anchor anything.** These marks arrive with frozen coordinates and no `Anchor`, and that is the
correct state for them — T145's migration decides separately whether a mark can be back-anchored. A
recovery pass that also stamped anchors would anchor them against **today's** render, which is precisely
the corruption T145's BLOCKER 2 exists to prevent. Restore the object, nothing more.

## ⟨R1⟩ Red first

- A fixture with an archived stream whose song id is absent live but whose title matches one live song:
  the objects are copied, and **running the tool twice adds nothing the second time**.
- **Teeth:** an archived object whose UUID is ALREADY live must not be duplicated — assert the live object
  count is unchanged, and assert the surviving object's points are untouched.
- An ambiguous title (two live songs, same title) **aborts that stream** and is reported; the unambiguous
  streams in the same run still succeed.
- A recovered object has **no `Anchor` and no `PointsRenderHash`** — reverting that assertion to "anchor
  it while we are here" must redden.

## Done means

The dry-run report is shown to VLL **before** `--apply`, naming counts per stream (not song titles — see
below). After `--apply` on a copy, the live pointed-mark count goes **15 → 18**, and the 150-point freehand
is present with its 150 points.

## One more thing, and it is not optional

**The T145 runner prints song titles in its report** (`cmd/migrate-anchors`, the `↳ … → anchored to "…"`
line). This repository is public, and a lane pasting that output into a gate note publishes the band's
song titles. **Both tools must report by id or by index, never by title.** Fix it in `migrate-anchors`
while you are here.
