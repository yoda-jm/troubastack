# The life of a band on TroubaStack — user journey, validated & gap-flagged

*Written 2026-07-06 by the Architect/Reviewer. Method: walk a realistic band's life
from formation to touring, and for every beat check what the stack actually does —
**validated** claims cite code/tests/review evidence; **gaps** are classified as
`[spec'd]` (a task file exists), `[needs-task]` (real, unfiled), `[product-call]`
(needs Vincent), or `[accepted]` (deliberate non-goal for now). The gap register at the
bottom is the actionable output.*

*Tidied 2026-07-10 (web-core): gap statuses reconciled to what has landed since —
resolved items are marked ✅ with their task/commit ref, still-open ones stay flagged.
Landings since: T20, T21, A08–A12 (stage-ergonomics trio + facing pages), B06 core
slice, B07, P202 safe slice, and T32 (insecure-context uuid + global error visibility).*

---

## The cast

**Marie** — singer, organizes everything (band admin). **Leo, Sasha** — guitar, bass.
**Anya ("maestro")** — also conducts a community orchestra (second band on the same
server). They rehearse Tuesdays in Marie's garage; the garage laptop runs `troubacore`.
Gigs are pub sets from stand-mounted tablets.

---

## Phase 0 — The band forms

**The story.** After one too many "which version of the chart is this?" nights, Marie
sets up the server, makes accounts happen, and gets everyone in.

**Validated today:**
- Register / login / logout / password change — `POST /api/auth/*`, `/api/me/password`
  (session cookies; hardened storage on mobile since B03).
- Create the band; **role model** (admin/member) with real teeth — admin gates on
  import (T08), bake (B02), verified by endpoint auth tests.
- **Invites both ways**: direct invites (create/list/delete/accept/decline) and
  shareable **invite links** — the realistic "paste it in the band WhatsApp" path.
- Full member lifecycle: list, role change (`PATCH members/{userId}`), removal
  (`DELETE`), voluntary leave. Multi-band per user works (Anya's two groups, seeded
  and exercised daily by the demo).

**Gaps:**
- **Running the server like an adult** — TLS, systemd service, backups, a signed
  release APK: `[spec'd]` **OPS01**, unstarted. Right now the garage laptop runs a dev
  binary over plain HTTP; a backup is "copy `troubadata/`". This is the single biggest
  real-world adoption gap — everything downstream is production-grade before the
  serving is.
- **Finding the server from a phone** — typing `192.168.x.x:8080` is Phase-0 friction:
  **B06** ✅ core slice landed (mDNS advertise + discovery, `ac0066e`); the app's
  Connect-screen prefill/browse (type-no-IP) is the open half — **mobile lane**.
- Password **reset** (forgotten, not change): ✅ **T21** landed — operator
  `troubacore reset-password <user>` mints a one-time link (admin-assisted, shown as a
  QR); admin-assisted reset is enough for self-hosted, as scoped.
- The **git-remote credential rotation** (repo hygiene, not product): `[product-call]`,
  long-flagged, still open.

## Phase 1 — Building the repertoire

**The story.** Marie uploads the charts — full score, a vocals part, Leo's tab. Leo
prefers his tab on top; Marie wants vocals first. Nobody wants to fight about it.

**Validated today:**
- Songs CRUD; **multi-file shared pool per song** with display order
  (`songs/{id}/files`, PATCH to reorder); PDF viewing in Studio.
- **`my-files`** — the quietly excellent bit: each member keeps a *personal ordered
  selection* of a song's pool (`GET/PUT/DELETE …/my-files`, `MyFileSelection` in
  `service.go`) that never affects what others see. Leo's tab-first view is real and
  private.
- Seed proves the flow end to end with real content, incl. an original chart ("The
  Open Road", committed PDF → seed → pipeline).

**Gaps:**
- Non-PDF assets (audio references, lyrics-only text): `[accepted]` — the product is
  chart-centric by design (I12's "flattened images" presenter); revisit only on demand.
- Bulk upload / drag-a-folder: `[needs-task]` polish, low priority.

## Phase 2 — Rehearsals (the collaborative heart)

**The story.** Tuesday night: laptops and tablets open, everyone marks their own parts
while Marie writes section cues everyone should see; Anya scribbles conductor marks
that only matter when she's directing.

**Validated today:**
- **Realtime collaborative annotation** in Studio: freehand (with the T06 fast wet-ink
  path, ~3 ms event→paint measured), line/rect/ellipse/text/highlight (T07 registry),
  per-object LWW + terminal tombstones (I5, engine-tested), server-authoritative echo
  with client outbox (I6, e2e-exercised).
- **Layer model matches band reality**: personal layers (private-by-default),
  shared/mandatory layers, conductor `roleTag` — all of which survive into the bake
  and Stage's Role/Layers toggles (proven in the shipped demo: Form / conductor Cues /
  Marie's notes).
- Tablet editing via the app's Studio WebView (A06); song history is linear and
  append-only, revert = new head (I4) — no rehearsal can destroy history.

**Gaps:**
- **Rehearsal live mode** — autobake + the red "your edits are auto-publishing" banner
  + transient auto-update on stage devices, so a mid-rehearsal edit shows up on
  everyone's stand *without* manual bake/download rounds: `[spec'd]` **P201**. This is
  the biggest missing *workflow*; today rehearsal-to-stand requires the full
  bake→offer→download loop each time (works, but is ceremony at rehearsal cadence).
- **Presence** ("Sasha is editing bar 12"): `[needs-task]` nice-to-have; the sync layer
  has the connections to support a cheap version.
- **Native stylus ink** on tablets: `[product-call]` — A07 is built-or-closed by
  Vincent's real-tablet spike; the measured web path may already be enough.

## Phase 3 — Cutting the concert

**The story.** Week before the gig, Marie drags songs into "Sat @ The Anchor", sets
per-song keys/tempo/notes ("Encore — everyone in"), bakes it, and everyone's devices
offer the download.

**Validated today:**
- Setlists CRUD + item overrides (key/tempo/notes) + reorder — and those overrides
  **ride the bake as metadata** (B02 decision 1, proven end to end into the Kotlin
  loader and onto the Stage screen via A08 ✅ landed).
- **One-click bake** (Studio card, admin-gated), atomic + concurrency-safe publication
  (B04, verified under `-race`), deterministic `.tstage`, canonical manifest; overlay
  pixels provably identical to the editor within AA tolerance (B01 golden test).
- **In-app distribution** (B03): offers ("New" / "update to rev N") applied only on
  tap, atomic swap, download-failure leaves the old bundle intact (tested), FROZEN /
  local pin / server `final_locked` all suppress offers.

**Gaps:**
- **Who may bake** — rule I11 permits "admin *or* member", v1 ships admin-only:
  `[product-call]` (documented in ARCHITECTURE).
- **Per-member bake** — bake *my-files* views so Leo's stand shows his tab, not the
  default score: ✅ **B07** landed — per-member bakes are real (`2a53bfe`), the
  most-requested-by-the-story feature delivered.
- **"New bake is up" notification** — offers appear when the app is opened; nothing
  pushes: `[needs-task]` later (self-hosted push is nontrivial; a poll-on-app-open is
  what exists and is honest).
- Bake **history** (list/download old revs): server keeps every rev on disk, API
  exposes latest-only: `[accepted]` for v1 (noted in B04's out-of-scope). Retention now
  has teeth — **P202** ✅ landed the `troubacore gc` bake-output prune (keep newest N
  per concert, never a `final_locked` rev).
- Setlist **duplication** ("same as last month's gig, swap two songs"): ✅ **T20**
  landed — duplicate endpoint + Studio action.

## Phase 4 — Show night

**The story.** Tablets on stands, house lights down. No Wi-Fi trust, no accounts, no
surprises: page turns, role-appropriate layers, and nothing else.

**Validated today (the strongest part of the stack):**
- **Offline, login-free, read-only presenter** (I12 — flipped ✅ this weekend):
  performs locally-imported bundles with zero server dependency, never-crash loader
  contract (torture-fixture tested), missing blob ⇒ placeholder page not a crash.
- Screen **stays awake** on both platforms (Android `FLAG_KEEP_SCREEN_ON`, iOS IOS04).
- Role-based layer defaults + manual Layers toggles; fit page/width; song-jump; the
  4-song demo performing on a **portrait Pixel Tablet** is pixel-documented in the
  README.
- Works on **iOS** too — simulator-proven with real Stage pixels (IOS02); real-device
  is credential-blocked (IOS03 runbook ready).

**Gaps:**
- **Pedal page turns**: ✅ **A09** landed — BT pedals (keyboards) drive turns, with a
  volume-keys fallback.
- **Night mode**: ✅ **A10** landed — dark-gig reading without the floodlight.
- **Metadata strip** (key/tempo/encore note at the top of a song): ✅ **A08** landed —
  the setlist overrides now show on the Stage screen.
- **Facing pages / landscape two-up** on wide tablets: ✅ **A12** landed — two-up on
  wide/landscape tablets. (Half-page turns remain the deluxe idea-list version.)
- **Kiosk hardening** (accidental Back/Home mid-song): partially mitigated (chrome is
  small; OS Guided Access documented for iOS QA) — `[accepted]`, revisit after real
  gig feedback.
- Devices are updated *before* the show by I13 design; a mid-show "rev 3 available"
  can never interrupt (offers render only in the concerts list — validated).

## Phase 5 — The day after, and the months after

**The story.** Setlist tweaks for the next gig, a re-bake, everyone updates at
soundcheck. A year later there are 40 bakes on the laptop and a new drummer joining.

**Validated today:**
- Re-bake bumps `concert_rev` monotonically; devices see "update to rev N"; pins keep
  a performer on the version they rehearsed (all tested).
- New members join via invite link and download the current concerts — no history
  ceremony needed.
- Song history is immortal-by-reference (I4/I7 keep-all default).

**Gaps:**
- **Bake retention/GC**: 40 bakes × ~0.5 MB is fine; 400 with real scans is not.
  ◐ **P202** ✅ landed the safe slice — `troubacore gc` prunes old bake-output revs
  (the real disk-growth source) + an I7 proof suite. The invariant-invasive half
  (revision-history compaction with baseline snapshots) is deferred as **P204**
  ("until real pressure").
- **Server migration/backup story**: part of OPS01 — "the garage laptop died" must be
  a 10-minute restore, not a tragedy.
- Codegen (**P203**): invisible to the band, existential to the maintainers — the
  hand-mirrored types held this weekend only through review discipline.
  `[product-call]`: promote it.

---

## Gap register (actionable summary, rough priority)

**Still open (actionable):**

| # | Gap | Class | Where |
|---|---|---|---|
| 1 | Production serving: TLS, service, backup/restore, release APK | spec'd | **OPS01** |
| 2 | Rehearsal live mode (autobake + banner + transient auto-update) | spec'd | **P201** |
| 5 | LAN discovery — the app's Connect-screen prefill (core mDNS ✅ done) | app half | **B06** (mobile) |
| 9 | Bake **history-compaction** GC (the deferred P202 half) | spec'd | **P204** (until real pressure) |
| 10 | Proto codegen (maintainer risk, not user-visible) | product-call | **P203** promote? |
| 11 | Widen bake to members (admin-only today) | product-call | I11 note |
| 12 | Presence, push notifications, bulk upload, kiosk hardening | accepted/later | idea list |

**Resolved since 2026-07-06** (moved out of the actionable list):

| # | Gap | Landed as |
|---|---|---|
| 3 | Pedal page turns / night mode / metadata strip | ✅ **A09 / A10 / A08** |
| 4 | Per-member (my-files) bake — "Leo sees his tab on stage" | ✅ **B07** (`2a53bfe`) |
| 6 | Facing-pages / landscape two-up on Stage | ✅ **A12** |
| 7 | Setlist duplication | ✅ **T20** (`8257d54`) |
| 8 | Password reset (admin-assisted) | ✅ **T21** (`troubacore reset-password`, QR link) |
| 9a | Bake retention — prune old bake outputs | ✅ **P202** safe slice (`troubacore gc`) |
| — | Silent client failures (insecure-context `crypto.randomUUID`, any uncaught error) | ✅ **T32** (`newUuid` + global error visibility) |

**Bottom line:** Phases 1–4 are real, tested, and evidenced — including the full
stage-ergonomics arc (A08–A12) and per-member bakes (B07), which have all landed since
this doc was written; a band could run a rehearsal and play a gig on this stack *today*
if someone technical babysits the server. The **one** thing now standing between "demo"
and "my band actually uses this" is **OPS01** — a server normal humans can keep alive
(TLS, service, backup, signed APK). After that, **P201** (live rehearsal mode) is the
big *lovable* workflow. T32 also closed a real adoption blocker this week: the product
now works on a plain-HTTP self-hosted origin, and no client error dies silently.
