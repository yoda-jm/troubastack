# T178 — The bake drops z-order, its tiebreak, and stylus pressure

**Lane:** web-core · **Status:** specced, not started · **Needs a VLL decision before it is built.**
**Origin:** found by the field-completeness guard web-core added to `core/internal/bake/annotations.go`
during T177 — the mirror that had none.

## 1. What is dropped

The bake builds the renderer's doc from `domain.Object` by hand. Three members never make the crossing:

| field | consequence a musician sees |
|---|---|
| `Order` | T27 per-object z-order. A **bring-to-front in Studio is absent from the bake** — marks stack in a different order on the stand than on the screen. |
| `CreatedAt` | the z-order tiebreak after `Order`, dropped with it |
| `Point.Pressure` | the bake **simulates** pressure (ink turns simulation on when every point lacks it) instead of drawing the pressure the stylus recorded |

**The z-order one hid behind a green test**, which is the part worth remembering: `web/bake`'s zorder test
proves the **renderer** honours `order` — it is fed one by hand — and is blind to this **producer** never
sending it. A seam test on the consumer cannot see a producer that stays silent.

**The pressure one is a sweep that stopped short.** The REST and realtime wires were fixed for exactly this
(the reflection-guard campaign that also caught `Anchor` and `PointsRenderHash`); that campaign covered three
mirrors and there are **five**.

## 2b. MEASURED 2026-09-23 — and §2/§3 below were the wrong question

**Pressure: closed, no decision needed.** 0 of 1062 points in his library carry a recorded pressure, so
ink's simulation already *is* what he reads. Carry the field with the fix and do not put it in front of him.

**Z-order: not a choice to offer — a defect to fix.** `Order` is non-default on **0 of 517** live objects,
which by §3's framing would mean "changes nothing he can see". That conclusion is wrong, and the number
beside it shows why: **506 of 517 carry `CreatedAt`.** The baker's comparator is `order → createdAt →
uuid`; core sends **neither** of the first two, so both read 0 and the sort falls through to **UUID**.
Studio, holding the real values, sorts by drawing time.

So the bake does not merely lose a bring-to-front nobody used: **it stacks overlapping marks by an internal
identifier where the screen stacks them by drawing order**, on every page with more than one mark. Active on
15 of his songs; **3 actual crossings** change when it is fixed.

**Nobody chose "stack by uuid"** — it is a fallthrough, not a design. So "do you want your charts to change?"
is malformed. **Tell him, do not ask him:** his charts currently stack overlapping marks in an order that
matches nothing, the fix makes the page match the screen, and three crossings in the whole library move.

**My framing error, recorded because it is reusable (Fable):** I asked how often the *producer sets* the
field. The defect lives in what the *consumer does without it* — and absence was not neutral, it collapsed a
three-key sort onto a meaningless tiebreak.

## 2. Why this is not a drive-by fix — SUPERSEDED by §2b for z-order

All three change **what an existing chart bakes to**. Correcting z-order re-stacks marks on concerts already
baked and already read from; correcting pressure changes the weight of every stylus stroke. That is a
visible change to VLL's own charts, and it is the same shape of decision as the 13 pt re-render he chose
deliberately with the numbers in front of him.

**So it is his call, not ours**, and it should reach him as three separate questions — the z-order pair and
pressure are independent, and he may want one and not the other.

## 3. What to put in front of him

Measured, not described:

- **How many objects in his library actually carry a non-default `Order`** — if the answer is near zero, the
  z-order fix changes nothing he can see and is simply correctness;
- **How many points carry a real `Pressure`** — the same question for the second fix. If his marks were all
  drawn with a finger, pressure is absent everywhere and the simulation is already what he sees;
- **A before/after of one affected page**, rendered both ways.

If the populations are empty the decision is trivial and we should say so rather than asking him to rule on
a hypothetical.

## 4. Not in scope

The guard itself already landed with T177, with each defect recorded as a named skip so the suite is green
and the gap is written down. **Un-skipping any one of them fails the guard by name** — do not remove a skip
until its fix lands, and do not "fix" one by deleting its entry.
