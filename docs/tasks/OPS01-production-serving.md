# OPS01 — Production serving: TLS, service, backup, release APK

**Priority:** after B02 (useful), before real-band use (required) · **Size:** M · **Area:** `deploy/` (new), `app`, docs

## Context

Everything runs on `http://localhost` today; the app allows cleartext only in debug
builds. A real band needs: a reachable HTTPS origin, the server as a managed service,
backups, and an installable release APK. No cloud assumptions — target is "a small box
the band admin owns" (home server / cheap VPS).

## Changes

1. **`deploy/` directory**: a `docker-compose.yml` (or plain systemd unit — pick one,
   document the other as a variant) running the single `troubacore` binary with
   `TROUBA_*` env config + a **Caddy** reverse proxy for automatic TLS (Let's Encrypt;
   Caddyfile with the domain as the only variable). Volumes: the data dir. Healthcheck
   on `/healthz`.
2. **Backup**: the data dir is the whole state — document (and script,
   `deploy/backup.sh`) a `tar` of it with the server stopped or via filesystem snapshot;
   note the git-store variant is `git bundle`-able. Test the restore path once and say so.
3. **Release APK**: a `release` build type with a self-managed keystore (documented
   `keytool` one-liner; keystore + passwords stay OUT of the repo — CI signs only if
   secrets are configured, otherwise skips), minify off for now. The README's sideload
   section gains the release variant; debug cleartext config is untouched (release
   builds already refuse cleartext — verify, don't assume).
4. **Docs**: `deploy/README.md` — DNS + Caddy + compose up + first-admin bootstrap +
   backup/restore + APK signing, ~1 page. Root README links it.

## Acceptance criteria

- From a clean Linux host (or a local container): `docker compose up` (or systemd unit)
  serves the seeded app over HTTPS via Caddy with a real or self-signed cert (staging
  ACME is fine for the test); the Android release APK, pointed at the HTTPS origin,
  logs in and edits without the cleartext config.
- `deploy/backup.sh` produces an archive; restoring it onto a fresh data dir brings the
  same bands/songs back (demonstrated once in the PR).
- No secrets committed (keystore, passwords, domains stay in env/gitignored files).

## Out of scope

- Multi-tenant/SaaS anything, monitoring stacks, CDN, autoscaling. Postgres (pgstore is
  a stub by design until someone needs scale).
