# A65 — the Studio screen should say where you are, offer two doors, and show a QR the room can scan

**Lane:** mobile (one small Studio addition). **Size:** M. **Status:** spec, not started.
**After the gig (2026-09-05)** — app binary, so the freeze covers it.
**Raised by:** VLL, 2026-09-03: *"in the native part of the Studio it is just concerts, maybe we can
have 2 tabs inside: Concert and Bands, 2 entry points in Studio webview, and then we can have a … with
show band QR code … also the whole TroubaStudio native page is not easy to understand, it should be
simple and with the right shortcuts"* — and the headline path he wants: *"je suis admin dans l'app, je
veux montrer un QR code pour que tout le monde puisse rejoindre."*

## What is there today

`EditScreen.kt` (A16) hosts Studio in a WebView with `?embedded=1`, so Studio hides its own nav. The
native frame around it is:

- **Title: "Edit".** Not the product, not the band, not the concert.
- **Overflow `⋮`: "Reload" and "Server URL…".** Nothing about where you are or what you came to do.
- **`initialPath` exists, is documented (*"deep-links Studio to a context"*) — and is never passed.**
  `MainActivity.kt:265` calls `EditScreen(storage, onBack = …)`, so Studio **always** opens on the band
  list, whatever you tapped to get there.

That last point is why this task is cheaper than it looks: the deep-link capability is already built
and simply unused.

**And the QR is already minted and drawn by Studio** — `InviteLinks.tsx`, rendered in exactly one
place, `BandSettings.tsx:45`, gated on `myRole === "admin"`. The server has the full set:
`POST/GET/DELETE /api/bands/{bandId}/invite-links`. **So nothing needs to be minted natively.** That
matters: `EditScreen`'s own contract is *"No native editor logic — login, editing and realtime sync are
all Studio's own"*, and this task must not erode it.

## Work

### 1. Two entry points, because there are two things you come here to do

Wire the parameter that already exists:

- **Concerts** → `initialPath = "/bands/{bandId}/setlists"`
- **Bands** → `initialPath = null` (the band list, today's behaviour)

Present them as the Home tile's two actions, or as two tabs inside the screen — the lane's call on the
idiom, but **both must be reachable without going through the other**. Today "Author, import & manage
concerts" lands you on a band list, which is the mismatch VLL is describing.

**The band id has to come from somewhere.** Take it from the app's existing current-band/identity
notion (the same thing that drives "Performing as …"); when there is none, Concerts falls back to the
band list rather than guessing. **Confirm which state holds it before building** — do not add a second
source of truth for "the current band".

### 2. Make the frame say where you are

- **Title**: the band name when known, otherwise "TroubaStudio". "Edit" tells you nothing.
- Keep the server origin visible or one tap away — it already exists in the overflow, but a person who
  has just scanned a QR onto a different server needs to see which one they are on.
- Keep `Reload` and `Server URL…`; they are fine, they were just alone.

### 3. `⋮ → Show band QR` — a deep link, not a new feature

Add an overflow item that deep-links the WebView to the band's invite section. **Hide it for
non-admins** — Studio gates `InviteLinks` on `myRole === "admin"`, so a non-admin would land on a page
with nothing on it. The app already knows enough to gate this (the Studio tile greys itself on
identity); use that, do not invent a second role check.

### 4. ⚠ The headline path: an admin showing a QR to a room

VLL's actual goal is not "reach the QR", it is **"hold this up and let everyone scan it"**. A 128 px SVG
inside a settings page, in a WebView, is not that. What that needs:

- **A presentation view**: the QR large, centred, high contrast, minimal chrome. Build it in **Studio**
  (a present-mode for an invite link) and deep-link to it, rather than rendering natively — that keeps
  invite logic on one side of the seam. ZXing *can* encode on Android, but using it here would put a
  second QR implementation in the codebase for no gain.
- **Keep the screen awake and bright** while it is shown. This is the one genuinely native part of the
  job — a phone that dims mid-room defeats the whole thing.

**And the security trade must be stated, not discovered.** T122 made invite links **expiring and
single-use by default**. A QR shown to a room is the exact opposite: multi-use, and long-lived enough
for everyone to photograph it. Studio's own copy already says so — *"anyone who photographs this QR can
join as {role}, with no expiry"*.

So the presentation view must, on the same screen: **name the role being granted**, **say it has no
expiry** (or show the expiry), and keep **revoke** reachable afterwards. A projected QR is also
photographed by everyone with a camera in the room, including people who were not invited — that is
acceptable *if the admin knows it*, and a defect if they do not. Do not soften the wording to make the
screen prettier.

## Do not

- Do not mint or render invites natively — Studio owns that (I10 boundary, `EditScreen`'s own contract).
- Do not add a second "current band" state, nor a second role check.
- Do not change the invite defaults T122 set. If a room-facing QR needs different terms, that is a
  choice the admin makes per link, visible on screen — not a new default.
- Do not remove `Server URL…` from the overflow.

## Done when

- Concerts and Bands are **both reachable directly**, and Concerts actually lands on a band's setlists —
  verified on device, not by reading the call site.
- The title names the band when one is known.
- `⋮ → Show band QR` reaches the QR in one step, and **is absent for a non-admin** (check with a
  non-admin account, not by reasoning about the gate).
- The presentation view is legible **scanned from across a room** — test it by actually scanning it from
  a few metres with a second device, which is the only test that means anything here.
- The screen stays awake while the QR is shown, and reverts afterwards.
- The role and expiry are stated on the same screen as the QR.
- `:shared:testDebugUnitTest` green, count matched; `tsc -b` clean for the Studio side.

## Sequencing

After the concert. Note it composes with [A57](A57-sign-up-from-an-invite.md) — A57 gave the *joiner* a
direct scan on Home; this gives the *admin* the other half, so the whole join flow becomes describable
in one sentence: **one person shows, the others scan.**
