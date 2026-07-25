# T63 — Invite-on-import (choose per missing member)

**Lane:** web-core · **Size:** M · **Status:** SPEC'd 2026-07-25 (VLL in-session: "invite on import" — after picking "land T62 as-built + invite popup later") · **Depends on:** T62 (band export/import) landed

## What VLL asked for

When importing a band (T62), members who don't exist on the target server are
currently **created** as passwordless accounts (activated via the T21 reset link).
VLL wants the option, **at import time**, to **invite** a missing member instead of
minting an account — via a preview popup that shows who's missing and lets the admin
choose per person.

## Facts this builds on (verified against current main)

- T62 import (`app.ImportBand`, `core/internal/app/bandio.go`): members matched
  by username else created; annotation layers + personal cues are owned by member
  UUIDs and need a real user-id **at import time** to attach.
- Invites (`Invite`, `IdentifierKind` username/email/uuid; `service.go` CreateInvite /
  PendingInvitesForIdentifiers / AcceptInvite) are **pull-based and email-free** —
  a pending invite keyed by identifier is seen + accepted by the invitee on their
  next login (this app has **no SMTP** — the `[smtp]` config block is an unused
  forward hook; T21 reset is deliberately email-free). So "invite" = create a
  pending in-app invite, NOT send an email.

## Design decisions (resolved here)

1. **Two-step import.** Split the current one-shot import into:
   - `POST /api/bands/import:preview` (multipart zip) → parses + validates the
     manifest (reusing T62's all-or-nothing validation, INCLUDING the created-member
     email pre-check), writes NOTHING, and returns the classified member list:
     `matched[]` (username exists on this server) and `missing[]` (would be created),
     plus the band name + counts. A short-lived server-side handle (or the client
     simply re-uploads the same zip on confirm — executor's call; re-upload is
     simplest and avoids server-side temp state).
   - `POST /api/bands/import` (existing) gains a **per-missing-member disposition**
     map in the request: for each missing username one of `create` (default,
     today's behavior) | `invite` | `skip`.
2. **`invite` disposition:** create a pending `Invite` to the new band with the
   manifest role, `IdentifierKind = username`, `Identifier =` the manifest username
   (username is always present; email is optional). The member is NOT created and NOT
   added to the band now. **Their personal content is dropped** (annotation layers
   they own + their cues + their file selections) — there is no user to own it until
   they accept. Report the dropped counts.
3. **`skip` disposition:** the member is neither created nor invited; their personal
   content is dropped too. (Same drop path as `invite`, minus the invite.)
4. **`create` disposition:** unchanged from T62 (passwordless account, reset link).
5. **Shared/conductor content is never dropped** — only content owned by a
   missing member with `invite`/`skip` disposition. `domain.SharedOwner` layers and
   any layer owned by a `create`d or `matched` member land as in T62.
6. **All-or-nothing still holds:** validate everything (T62 rules) BEFORE writing;
   an unknown disposition or a disposition naming a non-missing member → 400, nothing
   created.
7. **Studio UI:** the Bands-page "Import band…" picker now runs preview first →
   a dialog: "Importing **<band>**. N members already here (listed, will be
   attached). M members aren't — choose for each: **Create account** (default) /
   **Invite** (they join when they next sign in) / **Skip**." A one-line note that
   Invite/Skip drop that member's personal annotations + cues. Confirm → the real
   import with the disposition map → the existing import report, now also listing
   `invited[]` and any dropped-content counts.

## Out of scope

- Email delivery of invites (no SMTP in this app — invites are pull-based; if SMTP
  ever lands, invites ride it then).
- Deferring/holding a missing member's personal content until they accept (complex;
  `invite`/`skip` drop it — revisit only if VLL wants "content waits for the owner").
- Changing T62's export, format, or the `create` path.
- App/iOS work.

## Acceptance

1. httpapi: `import:preview` returns matched/missing correctly (pre-create one
   username, leave one absent) and writes nothing (follow-up list shows no band);
   the email pre-check from T62 still 400s a colliding created-member.
2. httpapi: import with dispositions — `create` (account made, in band),
   `invite` (pending Invite exists for the username with the manifest role; member
   NOT in band; that member's cues/layers absent from the new band), `skip` (no
   invite, content absent); shared/conductor layers present in all cases. The
   invited user then registers with that username → `PendingInvites` shows the band
   → `AcceptInvite` → they're a member.
3. e2e (studio): import a band whose manifest has one already-present member and one
   absent → the dialog lists both, choose Invite for the absent one → new band
   appears, report shows 1 invited; log in as the invited username → the pending
   invite to that band is visible.
4. `go test ./...` + gofmt + vet green; tsc + studio build clean; no dist churn.
5. Red-first on the disposition routing (invite vs create produces observably
   different post-state).

## Notes for the executor

- Reuse T62's validation verbatim for `import:preview` (don't fork the rules).
- Re-uploading the zip on confirm avoids server-side temp-file state — prefer it
  unless you have a reason not to; if you keep a handle, it must expire.
- Present at the gate; cite the work order (VLL 2026-07-25, relayed via Fable).
