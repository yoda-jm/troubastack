# Deploying TroubaStack on a box you own (OPS01)

TroubaStack is one Go binary that serves the API **and** the web editor on a single
origin, with all state in one data directory. This runs it on a small home server or
cheap VPS, over HTTPS, with backups. No cloud services, no database.

> **Status:** the `docker compose` path here is authored to spec but its live bring-up
> (real TLS cert issuance) is the **attended acceptance step** — verify it once on your
> host per *Verify* below. The **backup/restore** path is tested (a band + song survive
> a wipe-and-restore).

## What you need

- A host with Docker + the Compose plugin.
- A domain (or subdomain) whose **A/AAAA record points at the host** — Caddy needs it to
  get a Let's Encrypt cert. Ports **80 and 443** reachable from the internet.
- For baking setlists, nothing extra: the image already bundles `poppler-utils` + Node +
  the bake worker.

## Bring it up

```sh
cd deploy
cp .env.example .env          # set DOMAIN=your.domain
docker compose up -d --build  # builds the image (SPA embed + bake worker), starts Caddy
```

Caddy provisions the TLS cert automatically. Open `https://your.domain`.

**First user / admin.** There is no instance admin and no bootstrap command — the app
ships empty. The first person to open the site **self-registers**, then **creates a
band**, which makes them that band's admin. (Registration is open: anyone who can reach
the site can register. Put it behind a private network / VPN, or an auth proxy at the
Caddy layer, if that matters to you — TroubaStack has no per-instance gate.)

Forgot the only admin's password? `docker compose exec troubacore troubacore reset-password <username>` prints a one-time reset link.

## Config (env, set in the compose file)

| Var | Set to | Why |
|---|---|---|
| `TROUBA_APP_STORE` | `file` | durable users/bands/songs (default `mem` is **ephemeral**) |
| `TROUBA_STORE` | `file` | durable annotations (`git` also works; `pg` needs a DB) |
| `TROUBA_DATA_DIR` | `/data` | the one state dir → the `troubadata` volume |
| `TROUBA_SECURE_COOKIES` | `true` | session cookie is HTTPS-only behind TLS |
| `TROUBA_NO_MDNS` | `1` | no LAN advertising in a container |
| `TROUBA_BAKE_KEEP_REVS` | `3` (see below) | bake retention — `troubacore gc` keeps the newest N revs per setlist (`0` = keep all) |

## Rehearsal live mode (retention) — P201

Rehearsal live mode auto-bakes on every ~10 s quiet period, so a 2-hour rehearsal
mints **dozens of concert revs** per setlist. That's by design (each is a real,
downloadable snapshot), but the disk grows. Set retention and reclaim periodically:

- Set `TROUBA_BAKE_KEEP_REVS` (or `[bake] keep_revs` in the ini) to a small N — e.g.
  **3** keeps the latest few per setlist. `0` (the default) keeps everything.
- Run `troubacore gc` from a cron/timer (or by hand after a rehearsal) to prune to N.
  A **final-locked** rev is never pruned, so the "this is the gig" bundle is safe.

## Backup & restore

The whole state is the data dir. `backup.sh` tars it; restore untars onto a fresh dir.
**Stop the server first** — `app.json` is written as one file and a bake may be mid-write.

```sh
# Filesystem / systemd deploy (data dir on disk):
./backup.sh backup  /var/lib/troubastack/data  /var/backups/troubastack
./backup.sh restore  troubastack-backup-<ts>.tgz  /var/lib/troubastack/data-new

# Docker-volume deploy (data is in the `troubadata` volume):
docker compose stop troubacore
docker run --rm -v deploy_troubadata:/data -v "$PWD":/backup alpine \
  tar czf /backup/troubastack-backup.tgz -C /data .
docker compose start troubacore
# Restore: `docker run … tar xzf … -C /data` into a fresh volume, then compose up.
```

The git-store variant (`TROUBA_STORE=git`) is additionally `git bundle`-able.

## Verify (the attended acceptance)

1. `docker compose up -d --build`; `docker compose ps` shows `troubacore` healthy
   (the compose healthcheck hits `/healthz`).
2. `curl https://your.domain/healthz` → `ok` over a valid cert (use the `tls internal`
   or Let's-Encrypt-**staging** variant in `Caddyfile` while testing to avoid rate
   limits).
3. Register a user, create a band + song; the Android **release** APK (below) pointed at
   `https://your.domain` logs in and edits with no cleartext config.
4. Backup, wipe, restore → the band + song return (already demonstrated in the PR).

## systemd variant (no Docker)

Prefer a plain service? Build the binary (`make dist` → `core/bin/troubacore`), copy it
to the host, and run it under systemd, with Caddy (or your reverse proxy) terminating
TLS in front. Baking needs `poppler-utils` + Node on the host (`TROUBA_PDFTOPPM` /
`TROUBA_NODE` default to `pdftoppm` / `node` on `PATH`) plus the built **`web/bake`
worker**.

**Put the worker next to the binary and it resolves with no env var (T128).** Core searches
for `bake/dist/cli.js` beside its own executable first, so the supported layout is:

```
/usr/local/bin/troubacore              # the binary
/usr/local/bin/bake/dist/cli.js        # copied from web/bake/dist/
/usr/local/bin/bake/assets/            # copied from web/bake/assets/ (Roboto-Regular.ttf, loaded
                                       #   relative to cli.js)
/usr/local/bin/bake/node_modules/@napi-rs/canvas               # native addon, kept OUT of the bundle
/usr/local/bin/bake/node_modules/@napi-rs/canvas-linux-x64-gnu # its platform package — BOTH are needed
```

Copy `web/bake/dist`, `web/bake/assets` and those two `node_modules` packages into a `bake/`
directory beside the binary. Copying only `@napi-rs/canvas` and not its platform package is the
"Cannot find native binding" failure — which is why the server runs the worker at boot and warns,
naming the absolute path, if it can't. To keep the worker elsewhere, set `TROUBA_BAKE_CLI` to its
`cli.js` explicitly (then no search happens).

```ini
# /etc/systemd/system/troubacore.service
[Unit]
Description=TroubaCore
After=network-online.target
[Service]
User=trouba
Environment=TROUBA_APP_STORE=file TROUBA_STORE=file TROUBA_DATA_DIR=/var/lib/troubastack/data
Environment=TROUBA_SECURE_COOKIES=true TROUBA_NO_MDNS=1 TROUBACORE_ADDR=127.0.0.1:8080
ExecStart=/usr/local/bin/troubacore
Restart=on-failure
[Install]
WantedBy=multi-user.target
```

## Embed the app in the image (OPS02) — self-serve install

The image can carry the native app so a new band member installs it straight from your
server — point their phone's browser at the server, or scan the QR on the **band page** →
download → install (Android "unknown sources"). No store, no other infrastructure.

The APK is an **optional build input**: `deploy/apps/` is committed with only a
`.gitkeep`, so the image always builds. To embed the app, drop the APK there before
building — the CI `android` job already publishes a debug-signed one (installable via
unknown-sources; fine for band use until the signed release APK lands):

```sh
# grab the debug APK from the latest CI run (Actions → "android" job → troubastage-debug-apk),
# then:
cp androidApp-debug.apk deploy/apps/troubastage.apk
cd deploy && docker compose build && docker compose up -d
```

The server then exposes `GET /api/apps` (a tiny manifest) and `GET /apps/troubastage.apk`
(the download, correct MIME + versioned filename), and Studio's band page shows a **"Get
the app"** card with the download button + a QR of the absolute APK URL. Build **without**
an APK (empty `deploy/apps/`) ⇒ the manifest is empty and the card is hidden — no errors.
`deploy/apps/*.apk` is gitignored (it's a build artifact, never committed).

> Signed **release** APK (vs. the debug one above) is the mobile lane's job (`app/`) and
> needs a self-managed keystore kept **out of the repo**; it's a drop-in replacement for
> the file above. See the mobile app's build docs for the `keytool` one-liner + Gradle
> `release` build type; CI signs only if keystore secrets are configured.

## Secrets

Never commit secrets. `deploy/.env` (your domain), any keystore, and passwords stay
gitignored / in env only. This directory commits no secrets.
