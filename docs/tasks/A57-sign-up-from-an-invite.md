# A57 — An invitation should be able to bring in someone who has no account

**Lane:** Mobile · **Origin:** VLL — *"I am invited but I have no account, is this possible?"* ·
**Verified against `c331f85`** · **Gig-relevant: inviting new members before 2026-09-05.**
**Files:** `JoinDialog.kt` (a branch on the sign-in step) + a pure seam in `shared/join` and its test.

## This gap was written down before it was hit

A51's spec said it in the constraints: *"Both `/api/invite-links/{token}` routes are `a.auth(...)`-wrapped
⇒ redeeming REQUIRES an existing session; a scan can never be a brand-new person's first step. The app also
has no register screen — only Connect."* That was accepted as a limit of the first slice. VLL has now walked
into it from the other side, which is the right time to close it: **an invite whose entire purpose is
bringing in new people cannot be redeemed by a new person.**

## What already exists — this adds a UI, not a capability

`POST /api/auth/register` is **unauthenticated and open** (`service.go:71` — no invite requirement, no
gate). Anyone who can reach the server can already create an account with `curl`; the lane made its test
users that way. **So A57 does not widen what is possible — it makes the supported path reachable by a
human.** Say that in the submission so nobody reads this as opening a door.

## Deliverable

### 1. A "New here? Create an account" branch on the join sheet's sign-in step

**Join sheet only.** The invite is what motivates and scopes account creation — *"you were invited to
&lt;band&gt; as &lt;role&gt;; create an account to join"*. **Do not add it to `ConnectDialog` in this task**:
outside an invite there is no context that justifies it, and Connect is the screen a confused person lands
on when something else is wrong.

### 2. Fields: username, display name, password. **No email.**

The server takes email as optional and nothing in the app uses it — no verification, no email-based reset
(reset links are admin-issued). A field that does nothing is a field someone mistypes at a rehearsal.

### 3. After registering, keep going — do not restart the flow

Register → sign in automatically → **continue the same join with the same token**. `PendingToken` already
holds it across the sign-in round-trip; this is the same journey with one more step, and landing the person
back at "paste your invite" having just made an account would be its own small betrayal.

### 4. A pure `registerOutcome(status, …)` seam

Mirror `acceptOutcome`/`previewOutcome` in `shared/join`. **`409` (username taken) is the outcome that
matters** — it is the common real failure, it is recoverable, and it must say so ("that name is taken") and
leave the person in the form. Treating it as a generic failure is the defect to avoid. Cover 200, 409, and
an unexpected status.

### 5. Do not make audit C6 worse, and do not pretend to fix it

**C6 records that `minPasswordLen = 1` is not enforced on register, and that there is no rate limiting.** A
sign-up form is where weak passwords arrive. Apply a **client-side minimum** and a clear message — and state
plainly in the submission that **server-side enforcement and rate limiting remain open under C6**, untouched
by this task. Record the negative rather than letting a new form imply the gap closed.

## 6. The door has to match the room — VLL, on device

*"Seems to go to sign in when clicking on it as guest? Not easy to know where to go to join. Maybe some
button next to guest could be nice? Direct to QR code?"*

He is right, and the diagnosis is narrower than "hard to find". **The content is already correct**:
`ConnectScreen.kt:110-121` leads with *"Have an invite link?"* → paste / **Scan a QR** / Join, and only
then *"or sign in manually"*. What is wrong is the **label on the door**: `identityAction`
(`HomeScreen.kt:156-162`) offers a Guest either **"Sign in"** (`SignedOut`) or **"Connect"**
(`NotSetUp`) — neither says *join* — and Home offers nothing else. Someone holding an invite reads the one
button available, concludes it is not for them, and stops.

**Without this, A57 is a register form nobody can find.** That is why it belongs here and not in a task of
its own.

1. **Rename the Guest primary to "Join or sign in"** — for **both** `SignedOut` and `NotSetUp`, since both
   open the same dialog and that dialog leads with the invite. `identityAction` is already a pure,
   unit-tested function; extend its test.
2. **Add the direct scan entry**, on **Home in the Guest state** — where a person holding a QR actually is,
   not behind the account panel. Straight into A53's scanner, which already falls back to paste when the
   camera is denied, so the affordance cannot dead-end.
3. **No third product tile**, and do not touch the two-tile layout. This is a secondary action on the
   identity row, not a new product.

**Do not** solve it by moving the invite section around inside the dialog — it is already first.

## Teeth-check

Make `registerOutcome` map `409` to the generic failure branch. A named test must redden. Report the count.

## Verification

The pure seam off-device. Then **one device pass**: from a fresh install, scan or deep-link an invite,
create an account, and **assert the artefact — the new user is a member on the server**, not that the sheet
said "joined". The rig and tablet are already set up for exactly this.

## Out of scope

`ConnectDialog` · email · password reset · rate limiting or password policy on the server (**C6**) ·
invite-only registration modes (**F2** in the audit).
