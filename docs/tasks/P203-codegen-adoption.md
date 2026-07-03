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
