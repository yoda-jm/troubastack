# T107 — Secrets on disk should not be world-readable

**Priority:** normal, any time after T106 · **Size:** XS · **Area:** `core/internal` (Web & Core lane).
From the 2026-08-25 project audit, finding C3 (the file-mode half).

## 1. The problem

`app.json` is written `0o644` (`filerepo.go:187`) and holds **every bcrypt hash and every session
token in plaintext**. On a shared or multi-user host, any local account can read it and impersonate
anyone. The blob and store files deserve the same treatment.

This is the cheapest item in the whole audit and it is pure downside if left.

## 2. What to build

`0o600` for every file the server writes that carries credentials or user content — `app.json` first,
then blobs and the store files. Directories `0o700` where that follows.

## 3. Rules

- **Existing deployments must not break.** A file already on disk at `0o644` keeps its mode until
  rewritten; decide explicitly whether to tighten in place on open or only on next write, and say which
  at the gate. Silently leaving old installs wide open is the failure to avoid.
- Do not change *where* anything is written or its format — modes only.
- Umask can only remove bits, never add them; assert the resulting mode, don't assume it.

## 4. Acceptance criteria

- A test asserts the on-disk mode of `app.json` after a write (and of a blob and a store file).
- No file written by the server that carries a hash, a token, or user content is left group/world
  readable.
- The behaviour for pre-existing files is stated and tested.
- `gofmt -l core` clean.

## 5. Out of scope

Session TTL, token hashing, rate limiting — the rest of C3/C6. Encryption at rest.
