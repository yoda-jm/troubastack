# A46 — The Stage remembers where the set was, across process death

**Priority:** high — it fails mid-gig, which is the product's whole point · **Size:** S/M
**Area:** `app/` (Mobile lane). Found by **A33 drill 2** on device, 2026-08-25.

## 1. The finding (reproduced, not theorised)

1. Resume a concert → Stage opens at song 1/4, page 1/6.
2. Advance to **song 2/4, page 3/6**.
3. `am force-stop`, relaunch.
4. The app lands on **Home**, not the Stage.
5. *Resume* returns to **song 1/4, page 1/6** — the beginning.

The **concert** survives; the **position within it** does not. In code, `stage/` carries
`stagePositionLabel` for display only — no `rememberSaveable`, no persisted key/value for song/page.

**On stage this is the bad case:** an OS kill or a crash during a 25-song set drops the player to song 1
page 1 and they hunt for their place while the band plays. The longer the set, the worse it gets.

## 2. What to build

Persist the Stage position — **concert id + song index + page index** (and the fit/two-up mode if it is
cheap) — durably enough to survive process death, not merely configuration change. A `rememberSaveable`
alone is not sufficient: it dies with the process. Use the app's existing key/value storage.

Restore it when the concert is resumed. Whether relaunch should jump **straight back into the Stage** or
land on Home with Resume restoring the exact position is a **UX call for VLL** — the spec's title says
"restore INTO the page", but landing in a full-screen performance view on launch may be surprising when
the crash happened hours earlier. **Recommend: Resume restores the exact position; add a
"was here" affordance rather than auto-entering.** Ask before building the auto-enter variant.

## 3. Rules

- **A stale position must never strand the user.** If the saved song/page no longer exists — the concert
  was updated and is now shorter — clamp to the last valid page and continue. Never crash, never show an
  empty view.
- **The position must survive an update.** After a bake lands and the bundle swaps, the saved position
  should still resolve (same concert, possibly different page count) — see the clamp above.
- Writes must be cheap: this updates on **every page turn**, so it cannot do synchronous heavy I/O on the
  turn path. A page turn during a song is the most latency-sensitive interaction in the product.

## 4. Acceptance criteria

- **A test that fails today**: given a saved position, resuming lands on that song/page. Pure, in shared,
  beside the other state-mapping tests.
- Clamp behaviour has its own test: saved page beyond the end ⇒ last valid page, no crash.
- Page-turn latency is not regressed — state the measurement.
- `:shared:check` + `:androidApp:assembleDebug` + **`:shared:compileKotlinIosSimulatorArm64`**.
- **Device pass, non-waivable** (A39's lesson): advance mid-set, `am force-stop`, relaunch, resume, land
  where you were. That is the drill that found it, so it is the drill that closes it — and it goes back
  into `STAGE-WORTHINESS.md` as a PASS.

## 5. Out of scope

The other five A33 drills; whether relaunch auto-enters the Stage (VLL's call, §2).
