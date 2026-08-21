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

## The state model

Split `Identity.Disconnected` into two, and give each state the action it deserves:

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
- `Unauthorized` no longer nulls the known band.
- A36's palette unchanged by this task; the semantic colours are added as their own tokens.
- `:shared:check` green, `:androidApp:assembleDebug` green, iOS klibs compile. No new dependencies.

## Out of scope

- Server **discovery** UI (NSD/mDNS picking) — a separate surface, and unchanged here.
- Anything on `ConnectScreen` beyond what *Manage* / *Sign in* already reach.
- Auto-reconnect or background retry. *Retry* is a button the player presses; a background reconnect
  loop on stage is a different design with different risks.
- Multi-server / server switching.
