# B06 — LAN auto-discovery: find the band's server without typing an IP

**Priority:** B-track, after B03 lands · nice-to-have, good filler · **Size:** S/M ·
**Area:** `core/cmd/troubacore`, `app` (Connect screen), `app/iosApp/project.yml`

## Context

Connecting the app today means typing the server URL by hand (A06's settings dialog,
reused by B03's Connect screen). The product's deployment story is a self-hosted core
on the rehearsal-room / venue LAN — exactly the environment mDNS/DNS-SD was made for.
Discovery turns "read the laptop's IP off the screen" into "tap the server that
appeared".

**Design decisions (resolved here):**

1. **Discovery is a convenience PREFILL, never trust.** The user still taps the
   discovered entry (showing name + host:port clearly) and still logs in. Nothing
   auto-connects and no credential is sent without explicit user action. Be honest in
   the code comments: mDNS is unauthenticated, so a spoofed advertisement is possible —
   the risk equals typing a wrong URL, and the real mitigation is TLS (OPS01), not
   discovery logic.
2. **Service type `_troubacore._tcp`**, instance name = a configurable friendly name
   (default: the host name), TXT records: `version`, `path=/` (reserved). Advertised
   **on by default**, opt-out via `TROUBA_NO_MDNS=1` (a LAN tool should be findable;
   servers behind real DNS/TLS set the opt-out).
3. **Not a new I15 seam.** Discovery is connectivity glue like `HttpTransport`: Android
   uses the built-in `NsdManager` (no new dependency) in `androidApp`; iOS uses
   `NWBrowser`/Bonjour in the entrypoint layer. Common code only sees a
   `List<DiscoveredServer(name, url)>` via app DI.

## Changes

1. **Core**: advertise `_troubacore._tcp` via a small zeroconf lib (e.g.
   `grandcat/zeroconf` — pick the maintained one, justify in the PR) on startup,
   using the actual listen port; skip cleanly when disabled or when the socket fails
   (advertising must never break serving). Log one line: `mDNS: advertising as "<name>"`.
2. **Android**: `NsdManager` discovery while the Connect screen is open (start/stop
   with the screen's lifecycle — don't scan in the background). Results render as
   tappable rows above the URL field: "🎵 <name> — 192.168.1.23:8080"; tapping fills
   the URL field only.
3. **iOS**: same UX via `NWBrowser`. **Gotcha to handle:** iOS requires
   `NSLocalNetworkUsageDescription` + `NSBonjourServices` (`_troubacore._tcp`) in the
   Info.plist — add them to `app/iosApp/project.yml`'s `info.properties` (the plist is
   generated; see the IOS02 plist history for why).
4. **Tests**: Go — advertising starts/stops with the flag and never fails startup
   (bind-failure tolerated); app — the discovery→prefill mapping is plain logic,
   unit-test the parsing/dedup; platform browse/advertise itself is verified manually
   (document the two-machine or emulator-host check used).

## Acceptance criteria

- `troubacore` on a LAN host is discovered by the app's Connect screen on the same
  network, and tapping the row prefills the URL (evidence: screenshot or screen
  recording; the Android emulator cannot see host mDNS — use a real device or document
  the limitation and verify Android with two hosts or `avahi-publish` simulation).
- `TROUBA_NO_MDNS=1` disables advertising; startup and serving are unaffected either way.
- No auto-connect, no credential sent without a user tap; the row always shows host:port.
- `make test`, `:shared:check`, `:androidApp:assembleDebug`, iOS klib compiles green;
  ubuntu CI unaffected.

## Out of scope

- TLS / certificate pinning (OPS01 and later); WAN discovery; auto-login; advertising
  additional services; the iOS simulator proof (Bonjour on the CI simulator is not
  worth the macOS minutes — code-review + Android evidence suffices).
