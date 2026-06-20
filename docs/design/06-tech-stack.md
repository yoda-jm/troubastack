# Design: Tech stack

Derives from **I1, I8, I10, I14, I15**.

## TroubaCore — Go
WebSocket hub + REST + a **swappable persistence `Store`** (see Persistence below). Serves the **TroubaStudio static SPA** directly via `embed.FS` +
`http.FileServer` — **no Node runtime in production** (works because Studio is a client-rendered
SPA, *not* SSR; avoid Next.js-style SSR).

### Persistence — swappable `Store` (ADR 0002)
Persistence sits behind the `store.Store` interface; the backend is chosen by `TROUBA_STORE` at
startup. **Postgres is one implementation, not an architectural assumption** — nothing in `core`
outside the composition root knows which backend is live (I14).

| `TROUBA_STORE` | Backend | Use |
|---|---|---|
| `file` *(default)* | plain append-only file tree (`./troubadata`) | simplest zero-infra local dev |
| `git` | go-git repo (pure Go) | versioned dev / small self-host |
| `mem` | in-memory | tests / throwaway runs |
| `pg` | Postgres | production scale |

**Why `git` fits so well:** the domain *is* git's object model — linear append-only revisions =
commits, revert = a revert-commit (I4), content-addressed assets = blobs (dedup for free), pins/head
= tags/branch tip, reachability GC = `git gc` (I7). `go-git` is pure Go, so it preserves the single
static binary (unlike the cgo PDF libs). Granularity: **one commit per completed action** — the
mid-gesture firehose is display-only and never persisted, so there's no flood (ADR 0003). The server
holds authoritative HEAD in memory, applies mutations serialized, and a WAL + async committer keep
git I/O off the editor's hot path; `checkpoint`-tagged milestones + `git gc` keep the timeline
readable. **No git merges ever** (no branch, single writer, LWW upstream). For small self-hosted
bands, `git` can even be production; reserve `pg` for scale and relational queries.

### Baking — Go's weak spot, handled deliberately
- **PDF → raster:** MuPDF (`go-fitz`) / pdfium / poppler (`pdftoppm`). *cgo caveat:* complicates the
  single-static-binary story → may run as a sidecar/subprocess.
- **Overlay → image:** has a **parity requirement (I8)** → render it with `web/ink` inside the
  **`web/bake` Node worker** that core invokes. Do **not** re-implement strokes in Go (would be a
  second renderer → drift). `web/bake` is the *only* legitimate server-side JS runtime.

## TroubaStudio — Vite SPA
- Framework is secondary (Svelte/Solid for lean, or React for ecosystem/tldraw) — the work is canvas
  (PDF.js + `web/ink`).
- Builds to **static assets**, embedded in and served by `core` (I10, I14).

## web/ink — the one renderer (I8)
A tiny TS package: points/handles → stroke geometry → draw onto a canvas/`OffscreenCanvas`.
Consumed by `studio` (browser) and `bake` (Node). The native overlay mirrors it (parity test).

## TroubaShare app — Kotlin Multiplatform + Compose Multiplatform
Android now, iOS later (CMP iOS stable since 2025). The **"mobile library"** is the KMP shared
module. Shared: presenter (image compositor + paging), downloader, sync client, revision logic,
navigation.

**Platform-specific = exactly three `expect/actual` seams (I15):**
1. **WebView host** — Android `WebView` / iOS `WKWebView`.
2. **Low-latency ink overlay** — the one irreducibly per-platform perf piece. Android: Jetpack Ink
   (`androidx.ink`) or `GLFrontBufferedRenderer`. iOS: PencilKit / Metal.
3. **Storage** — paths / secure prefs.

Design all three seams now; implement Android; iOS = fill in the `actual`s.

## troubaproto — the contract (I1)
Schema once, in **protobuf**; codegen Go + TS + Kotlin so the three clients cannot drift. (JSON on
the wire is fine; the *definition* is single-sourced.)

## Versions
Go 1.26+ · Node 24+ / npm 11+ · JDK 25+ · protoc 34+ / buf.
