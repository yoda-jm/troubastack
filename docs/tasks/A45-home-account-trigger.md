# A45 — Home's connection controls move to a top-right account trigger (not a hamburger)

**Priority:** normal · **Size:** S/M (mostly relocating landed pieces) · **Area:** `app/` (Mobile lane).
VLL, 2026-08-24: *"connect/disconnect and stuffs, might be better at the top right next to parameters?
what do you think? or maybe something more mobile aware like a hamburger."*

## 1. The answer, and the reason

**Yes to top-right. No to a hamburger. And move the ACTIONS, not the STATE.**

**A hamburger (☰) is the wrong idiom here.** It means "a navigation drawer lives behind this". Home has
no navigation to hide — two tiles and a status line. A hamburger would promise a menu of destinations
and deliver a settings sheet. The mobile-aware control for *account and chrome* is a top-right
**account trigger**, which is also what this product already chose.

**The prior art settles the shape.** T58 ruled exactly this for the studio: ONE account trigger,
top-right, **avatar + display name, avatar-only at phone width**, opening a dropdown. Home has the same
problem — a top-right gear plus a separate identity row — and should not invent a second answer to it.
Matching T58 is the point, not a coincidence.

**But the connection is not chrome.** `ConnectionRow` carries *state* — connected / offline / guest /
checking — and two things depend on it: a player glancing at the phone before a gig wants "am I
connected, to which band" without a tap, and `UpdateRow` is **hidden** whenever identity is Offline or
SignedOut. Burying state behind a tap costs something real; burying *actions* costs nothing.

So: the **status stays visible and gets smaller**; the **actions move into the trigger**.

## 2. What to build

**A single top-right trigger** replacing today's standalone `⚙ Parameters` text button, following T58:
display name + a state-tinted status dot, collapsing to the dot/avatar alone at narrow width. It opens
a sheet or menu carrying:

- **Parameters** (today's `onSettings`).
- **Manage** (today's `onManage`).
- **The primary identity action** — Connect / Sign in / Disconnect, whatever `identityAction(identity)`
  currently yields, with its existing enable/disable behaviour while `checking`.

**What stays on the surface:** a compact connection line — the existing `StatusIcon` plus the identity
text (band / display name). No buttons. It keeps the at-a-glance answer and keeps `UpdateRow` visually
attached to the thing that gates it.

**Burying Disconnect is a feature, not a side effect.** A38 already treats a mid-gig sign-out as the
dangerous case — that's why it confirms. Moving it one level in makes an accidental tap harder, which
is the same instinct. **Keep A38's confirmation dialog exactly as it is**; this task must not become a
reason to drop it because "it's already behind a menu".

## 3. What must NOT regress

- **`UpdateRow`'s gating.** It is Hidden on Offline / SignedOut today. Whatever the surface line
  becomes, that relationship must survive — A43 is separately making the landing state honest, and
  these two must not disagree.
- **A38's disconnect confirmation**, verbatim.
- **The `checking` state** — the trigger must not offer an action mid-probe any more than the row does.
- **Guest / Offline reachability.** "Sign in" must remain reachable in one obvious step from Home; it is
  the recovery path when a session expires, and a menu that hides it fails the person who most needs it.

## 4. Acceptance criteria

- Every identity state (Connected / Guest / Offline / Checking) renders a correct surface line **and** a
  correct trigger menu — pure state-mapping tests, no device, in the shared layer beside `inFlightStatus`
  and `updateOutcomeStatus`.
- **A test that the update affordance still appears/hides on the same identity states as today** — the
  regression this refactor could most easily cause.
- Disconnect still confirms (A38).
- Narrow-width behaviour is asserted: the trigger collapses without the name, and the surface line does
  not overflow at 320 dp.
- `:shared:check` + `:androidApp:assembleDebug` + **`:shared:compileKotlinIosSimulatorArm64`**.
- **Device pass**: the four identity states seen on a real phone, plus one disconnect-and-reconnect
  round trip. Per A39's lesson, a state no human has executed is not waivable.

## 5. Where I'm guessing, and what would change my mind

I am reasoning from the code, not from looking at the screen — **VLL is.** My claim is that Home's
problem is *action clutter*, not *status clutter*, and the whole design follows from that. If what
actually bothers him is the status line itself — that Home shows identity at all when he just wants two
big buttons — then §2 is wrong and the right task is "put everything behind the trigger and let Home be
the two tiles". Worth one sentence from him before the lane builds.

## 6. Out of scope

The studio's topbar (T58 landed); a navigation drawer; changing what Manage or Parameters *do*; A43's
landing-status wording, which is a separate ruling.
