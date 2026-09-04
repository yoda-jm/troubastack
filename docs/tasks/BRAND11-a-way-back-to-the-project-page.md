# BRAND11 — a way back to the project page: the auth screens (web) and the account sheet (app)

**Lane:** web-core (§1) + mobile (§2). **Size:** XS each. **Status:** spec, not started. **Not frozen.**
**Raised by:** VLL, 2026-09-04: *"I feel that in Studio (web) we are missing a link to the github Page,
it is the same for the app (though I don't know where I would put it yet)."*

## The finding is not what it looks like

**Studio already has the link** — `AccountMenu.tsx:174`, *"ℹ️ About TroubaStudio ↗"* →
`https://yoda-jm.github.io/troubastack/`. BRAND03 put it in the account menu deliberately:
*"informational, so it lives in the account menu (never the editor chrome), directly above the
build/version footer."* **That reasoning is right and stays** — a marketing link has no business in the
chrome you annotate through.

**The actual gap is the logged-out state.** `Login.tsx` and `Register.tsx` contain **no outgoing link at
all**, and `AccountMenu` lives in the Shell's user area, which a signed-out visitor never sees. So the
one person most likely to ask *"what is this software?"* — someone who has just been handed a URL to a
self-hosted server — **has no route to the answer.**

That is the same argument BRAND08 already accepted for putting the wordmark there: the auth screen is
*"the first thing a user sees, and the only screen with no context."*

**The app has no link anywhere** — verified, zero occurrences of the URL under `app/`.

## Work

### 1. Studio — one link on the auth screens (web-core)

Add it to `Login.tsx` and `Register.tsx`, **below the form**, not above it: someone who came to log in
should not be offered an exit before the field they came for. Reuse the existing label and the
`target="_blank" rel="noopener noreferrer"` treatment from `AccountMenu`.

**Do not** move or duplicate the account-menu entry — it stays where BRAND03 put it, for logged-in users.

### 2. App — the account chip's sheet (mobile)

VLL's open question was where to put it. **The answer already exists structurally: the account chip's
sheet** (A45 — *"the actions live in its sheet"*). It is the app's exact counterpart to Studio's account
menu, so both products answer *"what is this?"* in the same place, and nobody has to invent a new
surface.

Two properties that make it the right choice rather than merely a convenient one:
- **It is reachable in the Guest states** (`accountChipLabel` handles `SignedOut` / `NotSetUp`), so the
  app has no equivalent of Studio's logged-out hole.
- **It is not Home.** A66 has just removed clutter from Home on the principle that the launcher names
  the products and nothing else; adding an informational link there would undo that in the same week.

**Do not** put it in the Connect modal — that surface is a task ("Connect to your band"), and a link out
of a task is an invitation to abandon it.

## Do not

- Do not add it to the Stage performance surface. Ever.
- Do not put it in the editor chrome (BRAND03's ruling).
- Do not hard-code the URL in more than one place per product; it is one constant.

## Done when

- A **signed-out** Studio visitor can reach the project page from the login screen — check while logged
  out, which is the state the bug lives in.
- The logged-in account-menu entry is unchanged.
- The app offers it from the account chip's sheet, **including in a Guest state**.
- Both open in a browser/new tab and neither navigates the app or the SPA away from itself.
