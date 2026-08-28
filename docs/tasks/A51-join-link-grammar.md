# A51 — Join links: a grammar the app can trust, and a decision it can test

**Lane:** Mobile · **Kind:** new feature, foundation slice · **Verified against `b6d23b7`**
**Files:** new pure seam under `app/shared/src/commonMain/kotlin/com/troubashare/shared/join/` + its test
**Blocks:** A52 (deep link), A53 (camera scanner). **Nothing in this task touches UI, the network, or the manifest.**

## Why this exists

Today the studio hands a person a link — `<origin>/join/<token>` (`core/internal/httpapi/webapi.go:444`) —
and already renders it as a QR (`web/studio/src/components/InviteLinks.tsx:128`). The app cannot do
anything with it: `git grep -i invite -- app/` returns exactly one hit and it is a comment
(`HttpTransport.kt:57`). There is no join flow, no deep link, no scanner.

A51 builds the part that can be tested off-device: **turn an arbitrary scanned or pasted string into a
decision.** A52 and A53 then supply the two ways a string arrives (an intent, a camera frame) and the UI
that acts on the decision. This is the A47/A48/A49/A50 pattern — four for four — and here it matters more
than usual, because the input is **hostile by construction**: a QR code is a string handed to you by
someone else, and the person scanning it is by definition not reading it.

## The threat this seam exists to contain

A scanned link names a **server**. Acting on it means pointing the app at a host chosen by whoever printed
the code, and then — because both token routes are `a.auth(...)`-wrapped (`webapi.go:102-103`) — asking the
person to **type their password into that host**. That is the whole risk of the feature in one sentence.

Two consequences the parser must enforce, not the UI:

1. **Userinfo is rejected outright.** `http://trusted-looking-host@192.0.2.9/join/xyz` has the host
   `192.0.2.9`. Any UI that displays "the host" after naive parsing will display the wrong one. Reject the
   URL rather than trying to display it safely.
2. **Only `http` and `https`.** `javascript:`, `file:`, `intent:`, `content:` and everything else are
   refused before any other consideration.

## Deliverable

### 1. `parseTroubaLink(raw: String): TroubaLink`

A pure function in `commonMain`. Shape the types as reads best; the behaviour below is the contract.

```kotlin
sealed interface TroubaLink {
    data class Join(val origin: String, val token: String) : TroubaLink
    data class PasswordReset(val origin: String, val token: String) : TroubaLink
    data class Unsupported(val reason: String) : TroubaLink
}
```

Rules:

| Input | Result |
|---|---|
| `https://h/join/<tok>` · `http://h:8080/join/<tok>` | `Join(origin, tok)` |
| `…/join/<tok>/` (one trailing slash) · `…/join/<tok>?x=1` · `…#frag` | `Join(origin, tok)` — query and fragment ignored |
| `…/reset-password/<tok>` | `PasswordReset` — the app has no reset UI; A52 tells the user to open it in a browser |
| any other scheme, or no scheme | `Unsupported` |
| userinfo present (`@` before the host) | `Unsupported` |
| no host | `Unsupported` |
| path is not `/join/…` or `/reset-password/…` | `Unsupported` |
| token empty, or containing anything outside `[A-Za-z0-9_-]`, or longer than 512 chars | `Unsupported` |

**Origin normalisation** — this is what A52/A53 compare and display, so it has to be canonical:
lowercase the scheme and host, keep an explicit non-default port, **drop a default port** (`http://h:80`
≡ `http://h`, `https://h:443` ≡ `https://h`), drop path, query and fragment. IPv6 literals
(`http://[::1]:8080/join/x`) must survive intact.

**Do not hardcode the token length.** Today it is 32 chars (24 `crypto/rand` bytes, base64url, no padding
— `core/internal/app/service.go:2219`). Pinning 32 in the client would break every app the day the server
widens the token. Charset and a sane upper bound are the right constraints; length is the server's business.

### 2. `joinDecision(link, currentOrigin: String?, hasSession: Boolean): JoinAction`

Also pure. This is where the security posture lives.

| link | currentOrigin | hasSession | action |
|---|---|---|---|
| `Join(o, t)` | `o` (normalised-equal) | true | `Redeem(o, t)` |
| `Join(o, t)` | `o` | false | `SignIn(o, t)` |
| `Join(o, t)` | different origin | either | `ConfirmServer(current = it, target = o, token = t)` |
| `Join(o, t)` | `null` (never connected) | — | `ConfirmServer(current = null, target = o, token = t)` |
| `PasswordReset` | — | — | `Blocked(<open in a browser>)` |
| `Unsupported(r)` | — | — | `Blocked(r)` |

**`ConfirmServer` is never skippable, including the first-run case.** A person who has never connected has
no basis for trusting the host either — they should see it. Comparison uses the normalised origin, so
`http://Host:80` and `http://host` are the *same* server and must not trigger a spurious confirm.

### 3. Tests

In the existing shared test source set (the suite is at **272 executions**; report the new total).

Cover, at minimum:

- each row of both tables above;
- **the two hostile vectors by name** — a userinfo URL whose visible prefix looks like a trusted host, and
  a `javascript:` payload;
- `http://h:80/join/t` vs `http://h/join/t` deciding `Redeem`, not `ConfirmServer` (the normalisation bug
  that would otherwise nag the user on every scan);
- an IPv6 origin round-tripping;
- a `/join/` URL on a **foreign host** (a plausible unrelated service) deciding `ConfirmServer` — this is
  the case A52's wildcard intent-filter makes reachable, and it must be provably handled here.

## Teeth-check

Two named mutations, each run separately, each must redden **at least one named test**; report the exact
count for both:

1. Delete the userinfo rejection.
2. Make the origin comparison in `joinDecision` a plain case-sensitive string equality.

## Out of scope

Manifest changes · any network call · any UI · the camera · iOS. `parseTroubaLink` must stay free of
platform APIs so it compiles on every target the shared module builds.
