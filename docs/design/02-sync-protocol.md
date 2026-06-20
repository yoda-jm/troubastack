# Design: Sync protocol

Derives from **I2, I5, I6**. Wire messages are defined in [`proto/`](../../proto).

## Model: optimistic local + server reconciliation

The server is authoritative (I6); clients render immediately and reconcile to the echo.

```
pen down/move      → render locally (wet layer on tablet, or in-browser canvas)
pen up             → object is final WITH its client uuid; keep it on screen,
                     POST it to the server in the background (commit-on-up)
server accepts     → broadcasts the object over WebSocket
echo received      → reconcile BY UUID → already present → no-op / in-place replace (I2)
                     ⇒ no flicker, no blank-and-redraw, no duplicate
```

The **same ingest path** handles your own echoes and peers' objects — both are "apply object by
uuid". There is no separate "remote" code path.

### Why commit-on-up
Network latency only ever affects *sync*, never *wet-ink feel* (which is local render latency,
I9/03-doc). A stroke is immutable, so one POST per stroke on release is enough. Live point-streaming
("watch the conductor draw") can be added later as the *same* append — no model change.

## Mutations & tombstones (I5)
- `create` (append), `move`, `resize`, `setStyle`, `setText`, `delete`, `restore`.
- A mutation for a tombstoned uuid → server replies `deleted-remotely` → client **rolls back** the
  optimistic change and drops the object.

## Outbox (I6)
Unconfirmed changes live in a client **outbox** and retry. Editing is online-first (see scope
below); the outbox is transient-failure resilience, **not** a full offline-edit engine.

## Transport
- **WebSocket** for the realtime object/mutation stream (per song/session room).
- **REST** for non-realtime ops (auth, list, download bundles, trigger bake).
- Wire encoding: protobuf-generated types (I1). JSON-on-the-wire is acceptable if preferred, but
  the *schema* is still single-sourced in `proto/`.

## Audit & multi-editor — where authorship lives
Concurrency is resolved **here**, not in storage. Two people editing the same song produce
object-level races the server settles by per-object **LWW (I5)**; a single writer per song applies
accepted mutations in a **total order**. The persistence layer only ever sees already-reconciled
state, so gitstore never hits a merge conflict (06-doc).

**Lock scope.** "Single writer per song" is a per-song serialization (an owning goroutine/actor, not
a global lock). Its critical section is short and in-memory: *apply to HEAD → LWW → assign `seq` →
make durable → ack*, then **release**. "Durable" is the release point and is backend-specific — WAL
append for git/file (the store commit is **async, after release**), the txn write for pg,
the in-RAM update for mem. The lock is **never held across the git commit**: doing so would put
commit I/O on every edit's hot path and throttle collaboration — which is exactly why the WAL
exists.

**The append-only mutation log is the audit source of truth.** Each accepted mutation carries
`author_id` (the actor — may differ from the object's owner), `client_ts`, and a server-assigned
`seq` (the total-order spine). This log is **pure append — never amended, never rewritten.**

**Auditability is independent of commit boundaries.** With **one commit per completed action**
(06-doc / ADR 0003), each commit is naturally single-author — clean native `git blame` — even under
simultaneous editing, because actions are interleaved by `seq`, never merged. The append-only log
stays the audit source of truth regardless; `git log` is its per-action mirror, with
`checkpoint`-tagged milestones for a coarse human timeline.

## Scope of "online"
Editing assumes connectivity (online-first). *Presenting* is fully offline (I12). These are
different runtimes with different guarantees — do not blur them.
