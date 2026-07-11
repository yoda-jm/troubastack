# P203 — Adopt proto codegen (retire the hand-written mirrors) — I1's last debt

**Priority:** phase 2 (decision + staged execution) · **Size:** L (staged) · **Area:** proto, core, web, app

## Context

I1's target state: every client *generates* its types from `proto/`. Reality (honest
per T09/T12): four hand-written mirror sets, each with an AUTHORITY comment, kept
aligned by discipline and review. That has held — but every new message family (bundle
metadata in B02, future protocol work) adds mirror surface, and the risk compounds.

This task is **first a decision**: adopt codegen now, or re-affirm mirrors-with-
discipline for another phase. If adoption is chosen, execute in stages — never a
big-bang.

## Stage 0 — the decision (cheap, do first)

Prototype `buf generate` (the committed `buf.gen.yaml` targets Go/TS/Kotlin) and
inventory the delta per client: how far are generated shapes from the hand-written ones
(JSON tags, ULong vs uint64, oneofs)? Write the verdict + migration order into this file
and get it reviewed before touching clients.

### Stage 0 — findings + proposed verdict (2026-07-11, web-core — for VLL/arch review)

**Prototype run.** `buf generate` (buf 1.71.0, remote `protocolbuffers/go`) into a
scratch tree. Two frictions before any type comparison:

- **The protos carry no `option go_package`** — `protoc-gen-go` emits *nothing* without
  it (managed mode or a per-proto `go_package` is required). Adding it to all 5 protos is
  a Stage-1 prerequisite (a proto edit).
- Generated shapes below confirm the decisive delta.

**The delta that decides it — canonical JSON.** The mirrors exist to serialize the
proto3 **canonical JSON** pinned by `docs/design/08` (lowerCamelCase names, 64-bit ints
as JSON **strings**) using plain `encoding/json` / kotlinx.serialization — the committed
fixtures + `docs/demo` bundle are the byte-for-byte oracle. Generated types do **not**
reproduce this with the standard runtimes. `BakedSong.source_revision` (a `uint64`)
generates as:

```go
SourceRevision uint64 `protobuf:"varint,2,...,json=sourceRevision,proto3" json:"source_revision,omitempty"`
```

| axis | canonical (required) | hand mirror | generated + `encoding/json` |
|---|---|---|---|
| JSON name | `sourceRevision` | `json:"sourceRevision,…"` ✓ | `json:"source_revision"` ✗ snake_case |
| uint64 value | `"5"` (string) | `,string` tag ✓ | `5` (number) ✗ |

The camelCase name lives only in the `protobuf:` tag, which **`protojson`** reads —
`encoding/json` does not. So canonical JSON from generated types requires switching
serialization to `protojson`, a **transport/runtime change P203 explicitly excludes**
("JSON stays the wire format; codegen is about *types*, not transport"). Same per client:
TS mirrors type 64-bit as `string` (`api.ts` `currentRev: string`); Kotlin uses custom
`ProtoUInt64Serializer`/`ProtoInt64Serializer` — hand layers that `protobuf-es` (→
`bigint`) and `protobuf-kotlin` (runtime JSON) replace, not the pinned string shape.

**oneofs.** `sync.proto`'s two `oneof payload`s generate Go interface-wrappers
(`isClientMessage_Payload`) + Kotlin sealed classes; the mirrors model them as string
`Kind` discriminators (`sync/mapping.go`). The adapter seam absorbs this but is rewritten.

**Proposed verdict: RE-AFFIRM mirrors-with-discipline for another phase.** Because:
1. The mirrors' whole job is the canonical-JSON encoding that generated types can't
   produce without `protojson`/runtime JSON — and moving to `protojson` is the transport
   change P203 draws out of scope. Types-only adoption leaves you re-adding the same
   name/64-bit layer on generated structs, saving little.
2. The compatibility oracle (fixtures + demo bundle byte-for-byte) makes any serialization
   swap high-stakes — not worth ~5 well-behaved message families.
3. The discipline has held (T09/T12 AUTHORITY comments + review); marginal new-message
   cost is low.

**If VLL chooses to adopt anyway** (e.g. to also move the bundle onto `protojson`), the
gate must settle the encoding policy FIRST — it is the whole decision. Least-risky order:
`(0)` add `go_package` + a drift-checked `buf generate` CI step → `(1)` **Go-first**:
generate into `internal/gen`, pick either **(a)** switch bundle/wire serialization to
`protojson` and re-verify byte-for-byte vs the fixtures, or **(b)** keep `encoding/json`
+ a hand name/64-bit adapter (little saved); delete Go mirrors only after fixtures pass →
`(2)` TS → `(3)` Kotlin (verify `protobuf-kotlin` JSON vs `docs/design/08`) → `(4)` flip I1.

## Stages (if adopting) — one PR each, mirrors deleted only at the end of each stage

1. **CI foundation**: `buf generate` in CI + a drift check (generated output committed
   or diff-checked — pick one policy repo-wide).
2. **Go first** (`core`): generate into `internal/gen`, adapt at the edges (the wire
   mapping files T09 annotated are the natural adapter seam), delete the Go mirrors.
3. **TS** (`web`): generated types replace `api.ts`'s hand unions where they're wire
   types (view-model types stay hand-written by design).
4. **Kotlin** (`app`): generated `gen/` replaces `BundleModel.kt`'s data classes; the
   canonical-JSON serializers go away if the generator's JSON support matches
   `docs/design/08` (verify against the committed fixtures — they are the compatibility
   oracle).
5. **Constitution**: flip I1 to ✅ in ARCHITECTURE.md (T12's tagging scheme).

## Acceptance criteria (per stage)

- The committed fixtures and `docs/demo` bundle still load byte-for-byte compatibly
  (they are the cross-language contract tests).
- No stage leaves a type defined twice; each stage's client builds + full CI green.
- Stage 0's decision text lives in this file, dated, before stage 1 starts.

## Out of scope

- Changing any message; new protocol features; runtime protobuf (JSON stays the wire
  format for the existing REST/WS APIs — codegen here is about *types*, not transport).
