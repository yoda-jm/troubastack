# A61 — give `fixtures/baked/` a real regeneration path, without destroying the frozen one

**Lane:** mobile (with a `Makefile` + `core/` edge). **Size:** S. **Status:** spec, not started.
**Why now:** A59 chose to document `baked/` as frozen and named this as the better end state, deferred
because it needs the bake toolchain to be resolvable at all. **[T128](T128-the-bake-toolchain-fails-at-the-gig-not-at-boot.md)
landed, so that blocker is gone**, and [P207](P207-carry-the-artist-into-the-bake.md) is about to make
a second fixture genuinely useful.

**This task ships nothing into the app binary**, which is why it is offered while the app is frozen
before the concert — see the gate note of 2026-09-03.

## The trap, and it is the whole design

`fixtures/baked/` is a **real `web/bake`-produced bundle, frozen on purpose**. After P207 it becomes
the compatibility arm for the artist field: a bundle from *before* the field, which must still load and
must show no artist. **A regeneration target that writes over it destroys exactly that.**

So the target must not point at `baked/`. Produce a **second** fixture — `baked-current/`, or whatever
name says "regenerated from today's baker" — and leave `baked/` untouched and documented as frozen.
Two fixtures with two jobs:

| fixture | job | regenerated? |
|---|---|---|
| `baked/` | an **old** bundle: proves new readers still load bundles that predate later fields | **never** |
| the new one | a **current** bundle: proves the reader tracks what the baker emits today | by the target |

If you find yourself adding a `--force` to overwrite `baked/`, stop: the value of that fixture is its
age, and age cannot be regenerated.

## Work

1. **`make fixtures-baked`** runs the real pipeline — poppler + the Node `web/bake` worker — into the
   new fixture dir. It is **not** part of plain `make fixtures`, which stays `mkbundle`-only and
   deterministic.
2. **Skip cleanly, never half-write.** Without node, `pdftoppm`, or a built `cli.js`, the target must
   print a readable message and exit **success**, leaving the tree untouched. T128's resolver decides
   where the CLI is; reuse that answer rather than hard-coding a path — a second hard-coded path is
   how the gig server broke.
   **Test that path deliberately** (e.g. `TROUBA_BAKE_CLI=/nonexistent`): it is the one contributors
   will hit, and a target that half-writes a fixture on a machine without the toolchain is worse than
   no target.
3. **Say whether it is reproducible.** A real bake is unlikely to be byte-identical run to run (T97/T98
   timing, ids). If it is not, say so in the README next to the target and treat a refresh as a
   deliberate non-empty diff — the same honesty A59 applied to `baked/`.
4. **Content stays synthetic.** No band data in a fixture, ever — the same rule as everywhere, and
   this one is committed.
5. **Update `fixtures/README.md`** so the two fixtures' jobs are stated side by side, and the
   Makefile comment points at it.

## Done when

- `make fixtures-baked` on a machine **with** the toolchain writes the new fixture and leaves
  `baked/` byte-identical — check `baked/` afterwards, do not assume.
- On a machine **without** it (simulate by pointing the resolver at a missing path), the target skips
  with a readable message, exits 0, and writes nothing. Verify the tree is clean afterwards.
- A test loads the regenerated fixture and asserts the same performable properties `baked/` guards,
  so the new one is a fixture and not just a directory.
- `:shared:testDebugUnitTest` green; match the count.
- Nothing under `app/` that ships in the APK changed.
