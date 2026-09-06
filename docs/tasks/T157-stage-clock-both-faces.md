# T157 — Stage clock: a third choice, both faces stacked

**Surface:** TroubaStage, the bottom-right time overlay + the ⚙ settings sheet.
**Lane:** mobile. **Kind:** feature (extends T147). **Number claimed** in the same push as this file.

VLL, 2026-09-06: he wants the choice of having the analog and the digital clock **one above the other**,
the digital **below** the analog.

Today `ClockStyle` is a two-value enum (`ANALOG` default, `DIGITAL`) and the two faces are mutually
exclusive at `StageScreen.kt:696-703`. This adds a third value — call it `BOTH` — and nothing else about
the clock changes: not the default, not the tick loop, not the chrono, not the 12/24h host formatting.

## What to build

1. `ClockStyle.BOTH` (`stage/Clock.kt`). **`ANALOG` stays the default.**
2. The overlay draws the analog face, then the digital text **underneath it**, inside the Column that is
   already there (`Arrangement.spacedBy(6.dp)` already gives the gap).
3. The ⚙ sheet's segmented row gains a third segment. Label it `Both`, after `Analog` and `Digital`.

**No separator between the two faces.** The 44dp rule at `StageScreen.kt:706` separates the *clock* from
the *chrono*; with `BOTH` the two faces are **one clock**, so the rule stays where it is — below the pair,
above the chrono. A second rule inside the clock would read as two independent readouts.

## Two places this breaks silently — check both, they are the red-first

⟨1⟩ **The restore is a two-way string compare, so a stored `BOTH` comes back as `ANALOG`.**
`MainActivity.kt:644` reads:

```kotlin
initialClockStyle = if (storage.getSecret(STAGE_CLOCK_STYLE_KEY) == "DIGITAL") ClockStyle.DIGITAL else ClockStyle.ANALOG,
```

The write side (`:689`) already persists `.name`, so `"BOTH"` lands in storage correctly and is then
thrown away on the next launch. Restore by **name over the enum's entries**, with the default as the
fallback for anything unrecognised — that also makes an unknown value written by a newer build degrade to
the default instead of to a wrong face.

**RED FIRST:** set `BOTH`, kill and relaunch the app, assert the overlay still shows both faces. It must
fail on today's code. Teeth-check: the same test with `DIGITAL` passes today, so the test is specific to
the new value and not to persistence in general.

⟨2⟩ **The visibility gate hard-codes `ANALOG` as "the style that needs no text".**
`StageScreen.kt:696`:

```kotlin
val clockShown = state.clockVisible && (state.clockStyle == ClockStyle.ANALOG || clockText.isNotEmpty())
```

`clockText` is `""` on any host that provides no formatter (iOS, tests — see `:200`). With `BOTH` on such
a host the whole clock would vanish, **including the analog face that has everything it needs**. Restate
it as two independent decisions — show the analog part when the style is `ANALOG` or `BOTH`; show the
digital line when the style is `DIGITAL` or `BOTH` **and** `clockText` is non-empty; the surface shows
when either part does.

**RED FIRST:** `BOTH` with an empty `clockText` must still render the analog face.

## Out of scope

The chrono, the tick loop (`:262` already wakes on `clockVisible`), the analog geometry
(`clockHandAngles` is unchanged and its tests stay green), and the bundle/baker. This is a view
preference — **it must never touch page geometry or the sheet's content**, exactly like `clockVisible`.

## Done means

Both faces stacked with the digital underneath; the choice survives a relaunch; `ANALOG` is still what a
fresh install shows; `:shared:compileKotlinIosSimulatorArm64` compiles (CI builds iOS separately); and the
device pass is owed — a seam test proves the enum plumbing, never that the pair is legible at arm's length
on a dark stage. Report the device check, do not let the green suite stand in for it.
