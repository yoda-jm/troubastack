# 0001 — Foundational architecture: server-authoritative, web-canonical, thin native

- **Status:** Accepted (2026-06-20)
- **Supersedes:** the legacy native-only TroubaShare app (Kotlin/Compose + Google-Drive-file sync).

## Context
The legacy app put the source of truth on-device and synced annotations as files through Google
Drive + JSONL, with a hand-built conflict resolver that never converged. The maintainer wants:
real-time collaboration, backend control, web reach, and **critical-quality stylus ink**; editing
is online-first, performing is offline.

## Decision
1. **Server-authoritative** (TroubaCore, Go) holds the single truth; clients are optimistic and
   reconcile to echoes. Kills the file-sync/conflict-resolver failure class. → I6
2. **Web editor is canonical** (TroubaStudio SPA); the mobile app embeds it in a webview. Native is
   an *optional accelerator*, never a reimplementation. → I10
3. **Stylus ink** is the one critical risk → a **native wet overlay** renders only in-progress
   freehand for low latency, handed to web on commit; one shared renderer with a parity test.
   → I8, I9
4. **Performance is offline & dumb**: an admin **bakes** a setlist into self-contained flattened
   images (TroubaStage). Bakes are the only publisher. → I11, I12
5. **One contract** in protobuf generates types for all three languages. → I1
6. **Mobile = Kotlin/Compose Multiplatform**, native limited to three `expect/actual` seams. → I15

## Consequences
- The native app shrinks to three jobs (host Studio · perform Stage · download).
- The domain is re-expressed in Go (small: objects, events, LWW, tombstones, linear revisions).
- The riskiest assumption (webview stylus latency) is validated by a spike *before* committing the
  native overlay — see `docs/design/03-rendering-and-ink.md`.

## Rejected
- *Ktor/Spring reusing Kotlin models* — dropped in favor of Go for the sync server.
- *Flatten-to-images for the **editor*** — kept vector there (needs precise zoom + live edit);
  flattening is presenter-only.
- *OT/CRDT* — unnecessary; per-object LWW + terminal tombstones suffice for this domain.
