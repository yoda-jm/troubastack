# T08 — Close the REST-import authorization gap

**Priority:** 8 · **Size:** S/M · **Area:** `core/internal/sync`, `core/internal/httpapi`

## Context

Write-access policy for annotations (layer locks, conductor-zone rules, RO/RW checks)
is implemented at the WebSocket transport edge — `authorizeWrite`/`canWriteLayer` in
`core/internal/sync/apply.go` (~lines 119–239). The comments there state this is
deliberate ("the engine stays mechanical — authority lives here").

The problem: there is a second write path into the same engine — the REST import
endpoint — and it **bypasses those checks entirely**. This is not an accident waiting to
be found; it's already codified by a test named `TestImportNotBlockedByWriteAccess` in
`core/internal/httpapi/ws_test.go` (~line 500). Whatever the original intent (seeding?),
"two write paths, one gated" is a latent authorization bug: any authenticated band
member can push mutations through import that the WS gate would reject (e.g. writing to
a locked layer or another member's personal layer).

## Changes

1. **Decide the policy and implement it** (preferred option first):
   - **(a) Share the gate.** Extract the authorization logic from `sync/apply.go` into a
     function/small package usable by both transports (it currently takes role +
     layer/zone info; keep it transport-agnostic). Apply it on the REST import path with
     the caller's authenticated role. Admin-role callers may still be allowed to import
     into locked layers if the product needs that — if so, make it an explicit
     `role == admin` allowance inside the shared gate, not a bypass of it.
   - **(b) Restrict the endpoint.** If import is genuinely an admin/seed-only tool,
     enforce `admin` role on the route and document that on the handler.
   Look at how `core/cmd/seed` uses the endpoint before choosing — the seeder logs in as
   the band admin, so option (a) with an admin allowance or option (b) both keep seeding
   working. State the chosen policy in the PR description.
2. Update `TestImportNotBlockedByWriteAccess` to assert the **new** behavior (rename it
   accordingly, e.g. `TestImportEnforcesWriteAccess` / `TestImportRequiresAdmin`), and
   add the negative case: a non-admin member importing into a locked/foreign layer gets
   a 403 (or per-mutation rejection — match how the endpoint reports errors today).
3. Re-run the seed flow end-to-end to prove no regression:
   `rm -rf core/troubadata && make demo` must still produce the fully seeded dataset.

## Acceptance criteria

- A non-admin authenticated user can no longer mutate, via REST import, anything the WS
  gate would reject for them — covered by a Go test.
- `make test` green; `make demo` still seeds successfully; `make e2e` green.
- No duplicated policy code: WS and REST call the same authorization function (unless
  option (b) was chosen, in which case the route enforces admin and says so in a comment).

## Out of scope

- Redesigning roles/permissions, adding new roles.
- Moving authorization into the engine (bigger architectural discussion; keep the
  current "authority at the edge" placement, just make it cover every edge).
