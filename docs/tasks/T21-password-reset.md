# T21 — Password reset (admin-assisted; self-hosted honest)

**Priority:** filler, real-human necessity (USER-JOURNEY #8) · **Size:** S · **Area:** `core/httpapi`, `web/studio`

## Context

`POST /api/me/password` handles *change* (knows the old one). A forgotten password on
a self-hosted server with no email pipeline needs the honest small answer: **a band
admin (or the server operator) issues a one-time reset link**, same trust model as
invite links (which already exist and set the pattern).

**Design decisions (resolved):**
1. **No email, no SMS** — the reset is a link/code the admin hands over out-of-band
   (in person at rehearsal, in the band chat), exactly like invite links.
2. Scope: a band admin can issue resets **only for members of their band**; the server
   operator can do anyone via a CLI (`troubacore reset-password <username>` printing a
   one-time URL) — covers the "only admin forgot" bootstrap case.
3. Token: single-use, 24h expiry, stored hashed (same discipline as sessions);
   consuming it sets the new password and **invalidates all existing sessions** for
   that user.

## Changes

1. Core: token store + `POST /api/bands/{b}/members/{u}/password-reset` (admin) →
   one-time URL; `GET/POST /api/password-reset/{token}` (anonymous — validate, set);
   the CLI subcommand.
2. Studio: "Reset password…" in the member row overflow (admin), and the plain
   set-new-password page the token URL lands on.
3. Tests: token single-use/expiry/hash, session invalidation, authz (member can't
   issue, admin can't cross bands), CLI happy path; e2e: admin issues → member sets →
   old session dead, new login works.

## Acceptance criteria

- The full admin-issue → out-of-band link → set → old-sessions-dead flow passes in e2e;
  tokens are single-use and expire; `make test` green.

## Out of scope

- Email/SMS delivery; self-service "forgot password" (needs email — revisit if a mail
  pipeline ever exists); rate limiting beyond the token's single-use (note it for OPS01's
  hardening pass).
