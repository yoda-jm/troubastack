# T204 — Channel groups: a grid column is a TRACK whose cell cycles through the parts it can carry

Owner: web-core. Spec: Fable, 2026-10-24. Follows T203 (and its FIX2). **New design: present the model change +
the migration at the gate before building.**

## Why (VLL, 2026-10-24)

> "actuellement les chants sont les lead, mais je veux cycler la colonne entre différentes valeurs qu'on peut
> setter, marchera pour un groupe qui switch les instruments sur une piste"

T203 hard-codes ONE cycling column: the per-musician "Voice" sub-column cycles through every voice-kind part
(L → B → ·). Every instrument part gets its own sub-column, which is filled or empty.

**That misses the general case.** A musician who switches instruments on ONE input (an acoustic guitar or an
autoharp on the same DI, depending on the song) should be ONE column (one console track) whose cell cycles between
those instruments. That is exactly how Voice already works for lead and backing.

## The model change: `Part.channel` (the group)

```
Part { …, channel?: string }   // the track this part is carried on, per musician
```

- **Default:**
  - voice parts → `"Voice"` (today's behaviour);
  - a personal instrument → its own name (today's behaviour: one sub-column per instrument);
  - a shared part keeps its own column, with the cell showing WHO plays it (unchanged, `shared` wins).
- **Parts with the same `channel` share ONE sub-column per musician.**
  - The header is the channel name ("Voice", "DI acoustique").
  - The cell shows the short of the part that musician carries on that track in that song ("Gtr", "AH"), or the
    shorts joined if several ("L+B").
- **Editor:** a click CYCLES · → p1 → p2 → … → · over that channel's parts, in part order. That is today's
  `cycleVoice`, generalised; keyboard letters pick a part by its short.
- **Grouping is band-level** (admin, in the Parts manager): "this part is carried on track X". The new channel
  name can be typed or picked.
- **Validation:** parts in one channel should share an input type. Warn if mic and DI are mixed.

## Patch keys (T203 amendment 3)

The channel key becomes `ch:<musicianId>:<channel>` for a personal channel; `shared:<partId>` is unchanged.

**Migration** (one pass, idempotent):
- `voice:<m>` → `ch:<m>:Voice`;
- `inst:<m>:<partId>` → `ch:<m>:<that part's channel>`.

Existing patches keep their inputs and strip names. If two old keys collapse into one channel, keep the lower input
number and report it in the migration log. It is unlikely, since nobody patches yet.

## Sheet

- The sub-column = (musician, channel).
- The changes column reports a change of the carried part within a channel as "A DI: Gtr → AH (same channel)", and
  a move between channels as "repatch". The T203 wording rules and label list apply.
- **The shared vectors file** gains:
  - a two-part channel (one column, the cell cycles);
  - a default-channel case (identical to T203 output, as a no-regression proof);
  - a migration case for patch keys.

## Tests

- **Default channels:** the output is byte-identical to T203 for a band that never sets `channel` (no
  regression).
- **Two instrument parts in one channel:** one sub-column; the cell shows the carried part; the click cycles.
- **A voice part moved into a custom channel:** honoured.
- **`shared` overrides `channel`.**
- **Patch-key migration**, including the collision rule.
- **`.tband` round-trip** carries `channel`.

## Interim already done (Fable, 2026-10-24, on one of VLL's real bands on :8080)

- The part "Voix" was kind=instrument, created inline. It is now kind=voice, short L, order 1.
- "Chœurs" was added: voice, short B, order 2.
- The Voice cell cycles · → L → B.
- No line-up was changed: the 44 "Voix" entries read as lead.
