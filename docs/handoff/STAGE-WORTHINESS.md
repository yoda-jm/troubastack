# Stage-worthiness drill checklist (A33)

The product's point is the 90 minutes on stage. This is the re-runnable drill sheet for that — the
ACCEPTANCE-P205 pattern applied to failure modes rather than features.

**Re-run whenever the Stage, the update path, or the bundle format changes.** Record the date, the
device, and a PASS or a finding per drill. **A drill left "assumed" is not a PASS** — that rule is why
this file exists.

---

## Run log

| Date | Device | Drills run | Result |
|---|---|---|---|
| 2026-08-25 | Redmi `23073RPBFG`, wireless adb | 2, 3 (partial) | **1 finding** (drill 2 → A46); drill 3's predicted finding **disproved** |

---

## Drill 1 — Airplane mode mid-set

Toggle mid-song. Page turns, layers, cues, drawer keep working (I12); live-mode toggles degrade
**silently** (no error toasts mid-performance); recovery when connectivity returns.

**Status: NOT RUN.** ⚠ **This drill cannot be driven over wireless adb** — airplane mode kills wifi and
therefore the adb connection the harness runs on, so it disconnects itself mid-drill and the result is
unobservable. Use **USB**, or script an on-device sequence that re-enables wifi after a fixed delay.
Budget that before starting.

## Drill 2 — Process death → restore INTO the page

Kill the app mid-song; reopen; land back in the same concert, same song/page/fit.

**Status: 🔴 FINDING — the position is not restored.** Reproduced on device, 2026-08-25:

1. Home → *Resume* → Stage opens at song 1/4, page 1/6.
2. Advanced to **song 2/4, page 3/6**.
3. `adb shell am force-stop com.troubashare.app`, relaunch.
4. **Lands on Home**, not the Stage.
5. *Resume* returns to **song 1/4, page 1/6** — the start, not where the set was.

The **concert** is remembered; the **position inside it** is not. Corroborated in code: `stage/` has
`stagePositionLabel` for *display* only — no `rememberSaveable`, no persisted key/value for song/page.

**Why it matters:** a crash or OS kill during a 25-song set drops the player to song 1 page 1, to find
their place by hand while the band plays. Filed as **A46**.

## Drill 3 — Battery saver + screen dim

Throttled schedulers vs the P201 poll and N9 prefetch; keep-screen-on policy in Stage.

**Status: 🟡 PARTIAL — the predicted finding is DISPROVED.** The spec guessed *"keep-screen-on in Stage
(is there one? probably the finding)"*. **There is one, on both platforms:** `StageHost.kt` adds
`FLAG_KEEP_SCREEN_ON` on entry and clears it on exit; iOS has the analog (`KeepScreenAwake`, I13).
Recorded so it is not re-opened.

**Still to run:** battery-saver ON, then verify the poll and prefetch behave (or degrade honestly) under
Doze/throttling.

## Drill 4 — Big bundle

A generated 25-song / 60-page bundle: open time, drawer scroll, memory, page-turn latency.

**Status: NOT RUN.** Needs the bundle-generator script (an A33 output, not yet written). The demo bundle
is 4 songs / 6 pages — an order of magnitude below a real gig, so nothing has been measured at size.

## Drill 5 — Reconnect storm during live mode

Server restarts mid-rehearsal; the poll loop and probe recover; no duplicate-apply (R10 remap under
repeated rev bumps).

**Status: NOT RUN.** ⚠ Target an **isolated rig**, never the :8080 instance.

## Drill 6 — Storage pressure

Import with low disk: a clean failure, never a corrupted bundle — the atomic-swap promise under its real
failure mode.

**Status: NOT RUN.** Use a scratch partition or a quota shim; **do not** fill the real device.

---

## Findings filed

| Drill | Finding | Task |
|---|---|---|
| 2 | Stage position (song/page) not persisted across process death | **A46** |

---

## A42② success-path drill (Fable's request, 2026-08-25)

The terminal **success** path of the Home re-bake (`bakePollStep` succeeded ⇒ clear + re-list) — the one
state the A42② device demo didn't show, because the rig sandbox has no annotation/overlay renderer.

**Status: NOT RUN — blocked on an annotation-free concert.** The only setlist on the rig is the annotated
"Sat @ The Anchor"; its bake fails at the overlay step (a direct curl bake fails identically —
`{"state":"failed",…,"error":"The annotation renderer isn't available…"}`), which is how I confirmed it's
environmental, not the client. T97's zero-annotation guarantee means a concert whose songs carry no
annotations would bake green here, but no such concert exists on the rig yet and authoring one (songs +
setlist, server-side) is more than the 5-minute drill it looks like from a warm rig. The mapping is
unit-tested + teeth-checked (`succeeded_isTerminal_clearsRow`; `finishingTail_isNotTerminal` reddens only
on the naive terminal-by-counts), and the live + failure paths WERE device-shown. To run: seed/author an
annotation-free concert, make it the resume target, tap Re-bake from Home, confirm the row clears and the
rev bumps, and record it here.

**Update (2026-08-25, after Fable pressed with the A42① precedent):** still NOT RUN — and it costs more
than the ~15 min Fable budgeted, so per her explicit out I'm not forcing it. There is no annotation-free
concert on the rig and no cheap one to borrow: marie admins only the annotated "Sat @ The Anchor" (fails
at the overlay step), and the T100 local band `good-vibes-only` both excludes marie (so she couldn't
re-bake it) and carries its own annotations. Any real run means authoring a new object-free song + setlist
server-side (uncertain payloads) AND a device download→open→re-bake cycle. **Structural argument that this
is lower-risk than A42①:** A42①'s deadlock was a success that stayed `InFlight` because the row relied on a
*guarded* re-diff to clear it. A42②'s `onReBake` sets `homeBake = BakeStatus.Hidden` **directly** on a
terminal `succeeded` (then bumps refresh) — there is no guarded-clear dependency, so it cannot reproduce
that hang; the A44 lesson is already applied. Will run the moment an annotation-free concert exists
naturally (a demo/seed change), and log the line then.
