# T132 — live mode from the concert row: a status dot, and an arm/disarm line in the ⋯ menu

**Lane:** web-core. **Size:** S. **Status:** spec, ruled by VLL. **Not frozen.**
**Ruled by VLL, 2026-09-03**, answering the question T131 left open: *"on garde sur la ligne un 'live'
avec un point (façon youtube) éventuellement pulsating et rouge, en revanche on a une ligne activer (ou
armer) et désactiver (qui bascule entre les 2) dans le menu ⋯ quand on est admin."*

**Why this shape is right** (recorded so it is not re-litigated): it keeps **one grammar**. The `⋯` menu
is already where actions live; the chip stays purely a status. The alternative considered — making the
chip itself clickable — put the same concept in two places under two rules, which is harder to explain
than to build.

## Background, because the consequence is not obvious from the UI

Live mode (**P201**) is *rehearsal* mode. While armed, **every annotation commit to that concert's songs
auto-bakes it**, debounced **8 s** after the last edit (`LiveBakeWindow`). Three properties matter here:

- It **self-expires after 3 hours** (`LiveModeWindow`), and the code says why: *"so a forgotten live
  mode can never survive to the gig."* **That window exists because people forget** — which is exactly
  the risk a one-click shortcut increases.
- It acts **as the admin who armed it** (`LiveBy`) — auto-bakes are attributed to them.
- Liveness is computed **at read time**, so there is no sweeper; the list computes it from `liveUntil`.

Today it is armed from `LiveModeCard` on the concert detail page, admin-only, **and that card carries
the explanation**. A menu shortcut strips that explanation — see Work §3.

## Work

**1. The chip: a dot beside the word, not instead of it.**
T131 already renders a `Live` chip. Add a YouTube-style dot, optionally pulsing.

- **Keep the word `Live`.** A pulsing red dot signals through **colour and motion only** — the two
  channels that fail for colour-blind users and for anyone with reduced motion. The dot is decoration;
  the word is the signal.
- **Respect `prefers-reduced-motion`: no animation under it.** `styles.css` uses that query exactly
  **once** today, so the pattern exists but is not reflexive — it will be forgotten unless it is
  asserted.
- **Do not reuse `--error-fg`** (`#b42318` / `#fca5a5`). A rehearsal in progress is **not an error**,
  and borrowing the error token makes a healthy state read as a failure. Add a token of its own.

**2. The menu line: one item that toggles, admin only.**
`Arm live mode` ⇄ `Disarm live mode` (or VLL's "activer/désactiver"), in the `⋯`, gated on
`myRole === "admin"` like `Delete` and the bake action. It must call **the same endpoint the detail
page calls** (`POST /api/bands/{bandId}/setlists/{setlistId}/live`) — two toggle sites, one behaviour;
do not fork the logic.

**3. Arming confirms and explains. Disarming does not.**
Asymmetric on purpose:

- **Arming** → confirm, **naming the concert** (on a row the real risk is arming the *wrong* one), and
  state the consequence in one sentence: *edits to this concert's songs will auto-bake for the next
  3 hours*. This is the sentence `LiveModeCard` gives you and the shortcut otherwise removes.
- **Disarming** → immediate. It is safe and reversible; a confirm would be friction for nothing.

## Do not

- Do not make the chip itself the control — the menu is the one place for actions.
- Do not reuse the error colour token.
- Do not animate under `prefers-reduced-motion`.
- Do not fork the toggle logic from `LiveModeCard`.
- Do not change `LiveModeWindow` (3 h) or the 8 s debounce.

## Done when

- The row shows a dot **and** the word `Live`; under `prefers-reduced-motion` it is static — assert
  that, do not eyeball it.
- The colour comes from a token that is not `--error-*`.
- The `⋯` offers one item that reads the current state and toggles it, **absent for a non-admin** —
  checked with a non-admin account, alongside the coverage T131's follow-up just added.
- Arming confirms, names the concert, and states the 3-hour window; disarming does not confirm.
- Arming from the row and from `LiveModeCard` produce the same state — verify by arming from one and
  observing the other.
- `tsc -b` clean, e2e green.
