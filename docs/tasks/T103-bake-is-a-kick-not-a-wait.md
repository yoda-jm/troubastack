# T103 — Baking must be a kick, not a held-open socket

**Priority:** high — it blocks A42② and it already fails on real venue wifi · **Size:** M
**Area:** `core/internal/httpapi` + `core/internal/bake` + `web/studio` (Web & Core lane).
From Mobile's device evidence (`0757197`). **A42② is blocked on this.**

## 1. The finding that decides the design

Mobile proposed fixing this client-side: fire the POST with a short timeout, treat a read-timeout as
non-fatal, and let the progress poll be the source of truth. **That cannot work today, and it would fail
in the most confusing way possible.**

`bakeapi.go:114` passes the **request** context into the bake:

```go
cb, bakeID, err := a.baker.Bake(r.Context(), bandID, …)
```

So when the client stops waiting, `r.Context()` is cancelled and **the bake is cancelled with it**.
`baker.go`'s own comment already names this case — *"a client disconnect that cancels ctx"* — and its
deferred publish then writes **`BakeFailed`**. A client that hangs up and polls would therefore watch its
own bake die, and the progress record would honestly report the failure it caused.

This is why the fix has to be server-side.

## 2. What's wrong, from the device

- A 4-song setlist bakes in **~12 s** (measured host→rig, `HTTP 200 time=11.9s`). A 25-song setlist runs
  for **minutes**.
- The app's blocking POST **failed twice on-device** with a socket-level error while short GET polls
  sailed through — venue/power-saving wifi kills a connection held open that long.
- When the POST throws, the client cancels its poller, so **"Baking song N of M" never renders** — the
  progress feature T96/T99 built is defeated by the transport shape.
- A39's `socketTimeoutMillis = 30_000` (correct for downloads) would trip on any bake over 30 s and show
  a **false** "couldn't reach the server" while the bake is progressing fine.

The studio has the same exposure — a multi-minute bake holds a browser connection too — it is simply
less likely to be on bad wifi.

## 3. The design

**Make the bake asynchronous, and make the progress record the single source of truth for the outcome.**

- `POST …/bake` **kicks** the bake and returns promptly with the bake id (202 is the natural code).
- The bake runs on a context tied to the **server's** lifetime, not the request's. A disconnected client
  must no longer cancel it.
- Both clients — studio and app — poll `…/bakes/{bakeId}/progress` to a terminal `succeeded`/`failed`
  and drive their UI from that.
- Migrate **both** clients in this task. Do not leave a synchronous path alive as a "compatible" option:
  it is the broken shape, and a second path would rot.

**Do not lose the warnings.** The synchronous response currently carries `Concert.warnings` (T60's
per-song transpose warnings), which the studio surfaces. With the outcome arriving via progress, the
warnings must arrive somewhere too — on the terminal progress record, or by re-fetching the concert.
Losing them silently would be a regression of a deliberate feature.

**Cancellation must remain possible, but explicitly.** Today, hanging up cancels. After this change it
won't — so if a user-initiated cancel is still wanted, it needs to be its own action, not a side effect
of a dropped socket. Decide and state which; do not leave "no way to stop a bake" undocumented.

## 4. Sequencing — T102 first, or with it

On `state:"failed"` the row shows the server's `error`. **That string is the raw Node stack trace today**
(T102). Ship this before T102 and A42② will put a stack trace on VLL's phone — the exact thing he asked
us to stop doing. **T102 lands first, or in the same change.**

## 5. Acceptance criteria

- The POST returns **before** the bake completes, carrying the bake id; a client that closes the
  connection immediately **does not** cancel the bake — asserted with a test that hangs up and then polls
  to `succeeded`. This is the core regression guard and it must fail against today's code.
- Both clients drive their outcome from the poll; the studio's T99 dialog behaviour (song N of M,
  "Finishing…", degrade-to-"Baking…", no leaked timer) is preserved — its e2e stays green.
- Warnings still reach the studio after a successful bake.
- Two concurrent bakes of the same setlist still produce distinct revs (B08/B09) and each reads only its
  own progress (T99's `claim`).
- The bake POST does not inherit the app's download-shaped socket timeout (A39's 30 s).
- `gofmt -l core`, `go vet`, `go test ./...`, full `make e2e`.

## 6. Out of scope

Bake performance (T97/T98 did that; poppler still dominates at ~11 s); the progress contract itself;
mobile-side UI, which is A42②.
