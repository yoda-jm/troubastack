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

**3. No confirm. The consequence goes in the LABEL.**

My first draft asked for a confirm naming the concert. **VLL corrected it and he is right:**
*"armer est dans le menu contextuel d'un concert, donc on sait duquel on parle."* The menu is anchored
to the row you clicked, the name is on that line, and arming is **instantly reversible** — the chip
appears on the row as immediate feedback, and disarming is one click. Naming it in a dialog is
redundant, and a dialog for a reversible toggle is friction.

**What does not go away is the explanation.** `LiveModeCard` tells you what live mode does; a bare menu
item does not, and the 3-hour auto-bake is the one thing nobody guesses. `RowMenuItem` has a `title`,
but its own comment says that is the **hover** explanation — and there is no hover on a tablet.

**So put it in the label:** `Arm live mode · auto-bakes for 3 h` (wording yours), and
`Disarm live mode` when on. A label is read by everyone who reads the item, cannot be dismissed
unread the way a dialog can, and costs no new component API. Keep `title` as the fuller sentence for
mouse users — a bonus, never the only carrier.

**Disarming** stays immediate and unexplained: it is the safe direction.

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
- **No confirm dialog.** The arm label itself states the 3-hour auto-bake consequence, and reads
  correctly on a touch device where `title` never appears.
- Arming from the row and from `LiveModeCard` produce the same state — verify by arming from one and
  observing the other.
- `tsc -b` clean, e2e green.
