# T122 — An invite link should expire and be used once, unless you say otherwise

**Lane:** Web/Core · **Kind:** hardening · **Verified against `b6d23b7`**
**Files:** `web/studio/src/components/InviteLinks.tsx` (+ its test). **Independent of A51–A53** — it can
land at any time, and should, whether or not the scanning work ever happens.

## The defect

`CreateInviteLink` (`core/internal/app/service.go:740`) treats both limits as opt-in:

```go
MaxUses: maxUses,          // 0 == unlimited  (InviteLinkValid: `l.MaxUses > 0 && …`)
…
if expiresInHours > 0 { … } // otherwise ExpiresAt stays nil == never
```

The studio's form matches: `placeholder="Expiry (hours)"` and `placeholder="Max uses"`
(`InviteLinks.tsx:87`, `:95`) are empty by default. So **the natural act — fill in nothing, click "Create
link" — mints a credential that grants member or conductor, to anybody, for ever.**

That was defensible when the artefact was a URL behind a copy button. It is not once the same link is a
**QR code on screen** (`InviteLinks.tsx:128`) that anyone in the room can photograph from a distance, and
it is less defensible still once a phone can scan it into a membership in two taps (A51–A53).

## Deliverable

**Change the studio's defaults, not the API's zero-value semantics.** `maxUses == 0 ⇒ unlimited` is a
documented, tested contract with other callers; redefining it is a breaking change for a fix that does not
need one.

1. **Pre-fill the form**: one use, and an expiry on the order of a day. Both fields stay fully editable —
   an admin who wants a standing link for a big ensemble can still have one.
2. **Make the unlimited case say what it is.** When a link has no expiry *and* no use cap, the row should
   read as the standing invitation it is, in plain words, near the QR rather than buried in the meta line
   (`:150-166`).
3. **Unit-test the default-computing seam**, not the rendering. Extract "what does an empty form submit?"
   into something a vitest can call, and assert both defaults.

## Teeth-check

Revert the pre-filled defaults to empty; the named test must redden. Report the count and the vitest total.

## Explicitly not in scope

- **Existing links are not touched.** This changes what gets minted next, nothing retroactive. Say so at
  the gate; someone reading "invite links now expire" would otherwise assume the live ones do.
- No core change, no migration, no revocation sweep.
- Rate limiting and an invite-only registration mode are **audit C6 / F2** and stay deferred; this task
  does not close either, and the sweep should record that negative rather than leave the rows looking
  addressed.
