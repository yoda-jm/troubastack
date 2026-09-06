# T160 — every restart logs the whole band out, and the session file fills with husks

**Lane:** core. **Kind:** bug (not a security hole — verified). **Number claimed** in the same push.
Found while restarting the live server for the T145 migration: after the restart, **not one** of the 11
persisted sessions authenticated.

## What is happening

`app.Session` carries the user a token belongs to — and marks **every field** as not-serialised:

```go
// Session is an opaque bearer token bound to a user. The token is stored as the
// map key in the repo; this record carries who it belongs to.
type Session struct {
	Token     string    `json:"-"`
	UserID    string    `json:"-"`
	CreatedAt time.Time `json:"-"`
}
```

**The doc comment and the tags contradict each other.** The file store writes `"sessions": {"<token>": {}}`
— the token survives as a map key, the user does not. On the live store right now: **11 sessions, every
record `{}`.**

So after a restart `GetSession` succeeds (the key is there), `sess.UserID` is `""`, and
`GetUser("")` fails. Two consequences:

1. **Everyone is logged out by every restart** — including, if it shares this mechanism, the tablet. That
   is a redeploy turning into "please sign in again", which on a gig day is the wrong moment to discover.
2. **Dead tokens accumulate forever.** `DeleteSession` runs only on an explicit logout, and nothing prunes
   a session that can never authenticate again.

## What is NOT happening, so nobody re-raises it

**This is not an auth bypass.** I checked `UserForToken` before writing any of this down: an emptied
record resolves to `GetUser("")`, which fails, so the request is rejected with `ErrUnauthorized`. A husk
grants nothing. The bug is availability and hygiene, not access.

## The decision this needs, and it is a real one

`json:"-"` on `UserID` may well have been deliberate — persisting *token → user* means a stolen copy of
`app.json` is a set of live credentials. If that was the intent, then **the current design is not "broken",
it is undocumented**, and the fix is to stop pretending: don't write the map at all, and say in the
comment that sessions are memory-only and a restart signs everyone out.

If instead sessions are meant to survive, persist the **hash** of the token as the key (never the token
itself) with the user id beside it, which is the shape `PasswordReset` already uses in this same file —
"only its SHA-256 hash is stored, so a leaked dataset yields no usable tokens". **Follow the precedent
that already exists in the codebase rather than inventing a third pattern.**

**Pick one deliberately and write down why.** What must not survive is a struct whose comment says it
carries the user while its tags guarantee it cannot.

## ⟨R1⟩ Red first

- A repo round-trip test: create a session, **reload the store from disk**, and resolve the token.
  Under "memory-only", the reload must yield **no session at all** (not a husk). Under "persisted", it
  must yield the same user.
- **Teeth:** the test must fail on today's code, which returns a record that exists but authenticates
  nobody — the state that is neither of the two designs.
- Whichever is chosen: **no unusable record may remain in the file.** Assert the count after a reload.

## Out of scope

Session expiry, rotation, and any change to the cookie itself. This task is about one contradiction:
a record that claims to carry a user and does not.
