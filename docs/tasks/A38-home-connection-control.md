# A38 — Home's connection status becomes a connection *control*

**Priority:** normal · **Size:** M · **Area:** `app/shared` (`home/HomeScreen.kt`), `app/androidApp`
(`MainActivity.kt`, `HttpTransport.kt`, `ConnectScreen.kt`), iOS host. Lane: Mobile.
**After A36** — it settles the palette this reads against.

VLL, 2026-08-21, on device: *"the offline/online status can be better, with connect / disconnect
buttons, with icons or colors to show what can be done — ask Fable to spec this."*

## What's actually there today

`identityLine()` / `identityAction()` in `HomeScreen.kt` are already pure and unit-testable — good
bones, keep them. The problem is the model behind them, not the rendering.

`Presence` (`HttpTransport.kt:43`) produces exactly three outcomes, and `MainActivity.kt:186–192`
folds them into four `Identity` values:

| real outcome | today's Identity | today's action |
|---|---|---|
| `Presence.Online` | `Connected` | **Manage** |
| `Presence.Unreachable` | `Offline(band)` | **Manage** |
| `Presence.Unauthorized` | `Disconnected` (and `me = null`) | **Connect** |
| no session cookie at all (`!isConnected`) | `Disconnected` | **Connect** |

Three problems fall straight out of that table, and they are the task:

1. **The action never matches the state.** Connected offers *Manage* when the obvious action is
   *Disconnect*. Offline offers *Manage* when the obvious action is *Retry*.
2. **`Disconnected` collapses two genuinely different situations** — "you have never set this up" and
   "your session ended on a server you know". Those want different words and different work: the
   first needs a server address, the second needs only a password.
3. **There is no Disconnect anywhere.** I grepped `ConnectScreen.kt`: no disconnect, no sign-out, no
   forget. This is new behaviour, not a relabelling.

## Answers to the three questions you raised

**One Home row, not a new surface.** This is a status plus one action; a dedicated screen for that is
a step backwards, and `ConnectScreen` already exists for the details behind *Manage*. Keep it one row
that shows state + the primary action inline.

**Semantic colour, not the brand.** A36 makes indigo the app's accent. If "connected" were also
indigo, then *connected* and *this is a button* would look identical, and the status would stop
carrying information. Use a small semantic set — a positive hue for online, a warning hue for offline,
muted/neutral for the not-connected states — defined as its own tokens alongside A36's palette, not
derived from `primary`. **Never colour alone**: each state also gets a distinct icon shape, since a
stage is exactly where someone glances at this in bad light.

**The states are the four below.** "No server found" is not a state the system can produce today —
discovery either yields an address or it doesn't, and an unreachable address is `Unreachable`. Don't
invent it.

## The state model — status and action are two different things

**Revised 2026-08-22** after VLL's addendum (*"there is still an offline/disconnect icon that is not
clear — guest / recognized / whatever this feeling"*). His three words are a better model than the one
I first wrote, and the reason is worth stating: **what am I?** and **what can I do?** are separate
questions, and collapsing them is what makes the row illegible.

**Status — three values, and this is what the icon and colour say:**

| status | meaning to a player | arises from |
|---|---|---|
| **Recognized** | the server knows me — *"Performing as Léo · The Troubadours"* | `Presence.Online` |
| **Guest** | I can use what's on this device, but nobody knows who I am | `Presence.Unauthorized`, **or** no session at all |
| **Offline** | there's no server to talk to right now | `Presence.Unreachable` |

*(plus **Checking** while the probe is in flight — a spinner, not a fourth identity.)*

Note that **Guest covers both** "never signed in" and "session expired". As a *status* they are the
same thing — you are unrecognized — which is why one word serves. They differ only in the **action**,
and that is the second, independent question:

| status | primary action | when |
|---|---|---|
| Recognized | **Disconnect** | always |
| Guest | **Sign in** | a server address is known → password only |
| Guest | **Connect** | no address known → the full set-up |
| Offline | **Retry** | always |
| Checking | *(disabled, not hidden)* | — |

"Guest" is also the honest word: it says you can keep working, which is true (I12) and which
"Signed out" and "Disconnected" both fail to convey. Use it in the UI.

Split `Identity.Disconnected` accordingly, and give each state the action it deserves:

| state | how it arises | icon | colour | primary action | also |
|---|---|---|---|---|---|
| **Checking** | probe in flight | spinner | muted | *(none — disabled, not hidden)* | — |
| **Connected** | `Presence.Online` | filled dot | positive | **Disconnect** | Manage |
| **Offline** | `Presence.Unreachable` | slashed/hollow dot | warning | **Retry** | Manage |
| **Signed out** | `Presence.Unauthorized` | hollow dot | muted | **Sign in** | Manage |
| **Not set up** | no session and no known server | hollow dot | muted | **Connect** | — |

Keep the existing reassurance on Offline — *"concerts on device still work"* (I12). It is the whole
reason Offline is not an error state, and it should stay visible.

**`MainActivity.kt:192` currently does `me = null` on `Unauthorized`, discarding the known band.**
Stop doing that: a signed-out user should still see whose band they were in, because that is what
makes "Sign in" feel like resuming rather than starting over. The `Unreachable` branch already gets
this right (`me?.band ?: ""`) — match it.

## The Connect flow becomes a modal (VLL, 2026-08-22 addendum)

*"not a modal, and no back button so definitely not a page feel."* Verified — and the real behaviour
is worse than that reads:

- `ConnectScreen.kt:59` is `Surface(Modifier.fillMaxSize(), color = background)` — a full-screen page.
- Its only affordance is `TextButton(onClick = onBack) { Text("Cancel") }` at `:96`, bottom row. No
  top bar, no back arrow.
- `MainActivity.kt:165–168` swaps it in with `if (connecting) { ConnectScreen(…); return }` — it
  **replaces** Home rather than overlaying it.
- **And there is no `BackHandler` in that branch.** The app uses `BackHandler` elsewhere
  (`MainActivity.kt:220`, `:286`, `EditScreen.kt:78`), but the early `return` at `:167` means none is
  composed while Connect is showing. So system Back falls through to the Activity default and **leaves
  the app** instead of dismissing Connect. Please confirm that on-device — it is cheap for you and I
  cannot run it — but the composition structure says it plainly.

**Make it a real dialog** overlaying Home: a titled surface with an explicit dismiss (**✕** or
tap-outside), **Cancel** kept, and a **`BackHandler` that dismisses rather than exits**. Home stays
visible behind it, which is most of what makes it read as a modal rather than a page.

Keep the discovery/server-picking content as-is — this is a container change, not a redesign of what
is inside it.

## Disconnect: what it must and must not do

**It signs you out. It does not forget your server, and it never touches your concerts.**

- Clear the session (cookie), so the server no longer recognises this device.
- **Keep the last server address** so reconnecting is one tap and a password.
- **Never remove downloaded concerts or bundles** (I12 — Perform works fully offline). Say so in the
  confirm, in one short line.
- Forgetting the server entirely is a *different*, rarer action and belongs behind **Manage**, not on
  the Home row.

**The constraint that makes this real work:** today the cookie and the origin are cleared *together*
(`HttpTransport.kt:72–74` wipes `SESSION_COOKIE_KEY` and `SESSION_ORIGIN_KEY` as a pair), and there is
no separately persisted "last server address". So a naive Disconnect leaves the user retyping an IP —
exactly the punishment this task exists to remove. **Persist the last server address independently of
the session** and have *Sign in* / *Retry* use it. Do not weaken `sessionCookieFor`'s origin guard
while doing it: a session must still never be handed to a server other than the one that issued it.

**Confirm before disconnecting.** One tap that silently ends a session mid-gig is worse than one
extra tap.

## Acceptance criteria

- `identityLine()` and `identityAction()` stay **pure and unit-tested**, extended to the five states —
  including the new *Signed out* vs *Not set up* split. Table-test every state's label *and* action.
- **Round-trip on device:** Connected → Disconnect → confirm → state shows *Signed out* with the band
  still named → **Sign in** needs only a password (no address retyped) → Connected again. Screenshot
  the Disconnect confirm and both resulting states.
- **Disconnect leaves concerts intact** — assert the installed-concert count is unchanged across a
  disconnect, and that Perform still opens one. This is the I12 guard and it must be a test, not a
  claim.
- Every state distinguishable **without colour** (icon shape differs) — state it and show it.
- **The three status words appear in the UI**: Recognized (as *"Performing as <name>"*), Guest,
  Offline. Assert the label for each, and that Guest is reached from *both* an expired session and a
  never-signed-in device while offering the right action in each case (Sign in vs Connect).
- **Connect is a dialog**: Home is still visible behind it; ✕/tap-outside and Cancel both dismiss;
  **system Back dismisses it and does NOT leave the app** — assert that one explicitly, it is the
  defect being fixed.
- `Unauthorized` no longer nulls the known band.
- A36's palette unchanged by this task; the semantic colours are added as their own tokens.
- `:shared:check` green, `:androidApp:assembleDebug` green, iOS klibs compile. No new dependencies.

## Out of scope

- Server **discovery** UI (NSD/mDNS picking) — a separate surface, and unchanged here.
- Anything on `ConnectScreen` beyond what *Manage* / *Sign in* already reach.
- Auto-reconnect or background retry. *Retry* is a button the player presses; a background reconnect
  loop on stage is a different design with different risks.
- Multi-server / server switching.

## Known limitation (accepted, not a bug — Fable's review, 2026-08-23)

The retained band on the Guest line comes from the in-memory `me` (`SignedOut(band = me?.band ?: "")`).
On a **cold start while signed out**, `me` is null, so the line is a bare "Guest" and the "Sign in
resumes, not starts over" cue is lost until the next successful probe. The common case (signing out
within a session) keeps the band; persisting the last-known band across process death is out of scope
for A38. Noted here so it isn't rediscovered as a defect.
