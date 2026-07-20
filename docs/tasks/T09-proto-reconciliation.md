# T09 — Proto codegen: generated mirrors, drift-guarded (REWRITTEN 2026-07-20, VLL green-light)

**Priority:** high (roadmap #1) · **Size:** M/L, staged · **Area:** `proto/`, a small
generator, `core/internal/bake/bundle.go` + type maps, `web/studio/src/api.ts`
(bundle types), `app/shared/.../BundleModel.kt`. Web-core lane.
*(This file's original reconciliation items are absorbed: highlight/scope cleanup
happened along the way; `buf lint` runs since P205 s1. What remains is THE debt:
hand-written mirrors.)*

## Why (the honest evidence)

Three real bugs this week were mirror-class: the T51 REST-vs-WS **type-string map
divergence** (proto enum names, duplicated twice, one updated), and the general
review-only discipline on five new P205 fields across three languages. (The
`/api/me` wrapper bug was REST-DTO drift — SAME class, but REST DTOs are not
proto-defined; they're a possible follow-up, out of scope here.) Every future
field multiplies exposure. The fix: **mirrors become generated artifacts.**

## Design — the glyphs.json philosophy, not three protobuf runtimes

Do NOT adopt protobuf runtime libraries in three languages (churns every usage,
new deps, JSON-shape risk). Instead: **ONE small generator** (node or Go, reading
`buf build`'s descriptor output) emits the mirrors in each language's EXISTING
idiom:
- Go: the `bundle.go` structs with the exact current json tags/omitempty style.
- TS: the `api.ts` bundle-side interfaces.
- Kotlin: the `@Serializable` BundleModel data classes (kotlinx stays).
- **Plus the type-string maps** (ObjectType ⇄ string — the T51 dup dies) for
  httpapi AND sync from the one enum.
All four outputs committed + **CI drift-guarded** (`generate && git diff
--exit-code` — the CueGlyphData pattern, proven twice). Editing a mirror by hand
becomes a CI failure; adding a proto field regenerates all three languages in one
command.

## The crux: byte-compatibility

The bundle format on disk must NOT change. **Stage 0 (red-first): golden tests** —
the committed `demo-concert.tstage`'s `bundle.json` must round-trip IDENTICALLY
through each language's current mirror; these goldens then gate every generated
replacement. A generated mirror that changes one json key fails before review.

## Stages

0. Golden round-trip tests against the committed demo bundle (Go + TS parse +
   Kotlin decode), red-first proof they'd catch a key rename.
1. The generator + Go output; delete the hand `bundle.go` mirror; type maps
   generated; goldens green.
2. TS output replacing api.ts's bundle types (studio compiles, e2e untouched).
3. Kotlin output replacing BundleModel.kt (`:shared:check` + A29 vectors green —
   the vectors are the semantic cross-check riding on top).
4. `buf breaking` in CI against main (field-number safety forever), + the I1
   section flips from "🎯 spec only" to "✅ enforced (generated + guarded)" — that
   doc edit is MINE at the end.

## Acceptance

Goldens green pre/post per stage; all suites green; the AUTHORITY comments replaced
by "GENERATED — do not edit" headers; drift-guard proven (touch a mirror → CI red);
no runtime dependency changes; old .tstage files load unchanged (parse the
committed bundle as the final check).

## Out of scope

REST DTO generation (Me/Bands etc. — a candidate follow-up), any wire-format
change, gen/ relocation debates (placement per the I1 scope note: generated-and-
guarded is the invariant).
