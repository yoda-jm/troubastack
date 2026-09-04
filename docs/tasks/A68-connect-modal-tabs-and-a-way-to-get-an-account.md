# A68 — split the Connect modal into two tabs, and give "no account yet" a real way out

**Lane:** mobile. **Size:** M. **Status:** spec, ruled by VLL. **Not frozen.**
**Raised by:** VLL, 2026-09-04: *"in the app the connect page is very busy … can we split it or simplify
it without loosing features?"*, then **ruled**: *"je verrais bien des tabs"* and *"un lien clair créer un
compte qui ouvre un navigateur serait top, il faut que ce soit clair."*

## The diagnosis

The modal presents **two mutually exclusive journeys at equal weight, simultaneously, inside a
scrolling container** (`heightIn(max = 560.dp).verticalScroll`). In the common case that is three text
fields, four buttons and four text blocks — and on a phone you cannot see them at once, so you scroll
without knowing whether what you need is above or below.

- **Invite journey:** paste or scan → Join. **The link names the server**, which is the entire point of
  A52 — you never type a URL.
- **Manual journey:** Server URL + username + password → Connect.

Nobody does both. **A57 already identified this** and made the modal *"LEAD with the invite … so a
person holding an invite must not read a button that says only Sign in and conclude it isn't for
them."* But leading is expressed **only by vertical order**, inside a scroll — so the lead can be the
only thing visible, or can scroll away. And the most intimidating field, **Server URL, is always on
screen** — the very thing the invite journey exists to avoid.

## Work

### 1. Two tabs: **Invite** | **Sign in**

The app already uses a segmented row for Stage's reading mode, so this is not a new idiom. Choosing a
tab is choosing a journey; you then read only that half.

**Above the tabs, keep the offline sentence** — *"Playing works offline without an account."* It is true
of both journeys and it is **the only line that tells a hesitant person they can ignore all of this.**
It deserves more prominence, not less.

- **Invite tab:** the paste field, *Scan a QR*, *Join*.
- **Sign in tab:** Server URL, username, password, *Connect* — **and the discovered-servers list, which
  belongs here** (it prefills the URL). Today it floats between the two journeys.

### 2. ⚠ "Create an account" — VLL's ruling, and the reason it matters

**There is no way to create an account from the app except through an invite.** Registration lives only
in `JoinDialog` (A57). The Sign in tab is therefore a **dead end** for someone who has a server and no
account: they type a username, fail, and never learn why.

**Tabs make this worse, not better** — the Sign in tab becomes exactly where that person lands.

**VLL's ruling: a clear link that opens a browser.** In the Sign in tab, prominent, not a footnote:

- It opens **`<serverUrl>/register`** in the system browser — Studio's own registration page. The app
  needs no registration code and no band-creation code, both of which it lacks today.
- **It depends on the Server URL field.** Disable it (with a reason) while that field is empty —
  `/register` on nothing is nothing. If a discovered server was tapped, the URL is already there.
- **Say what it does before it does it.** Opening an external browser mid-sign-in is a context switch;
  the label should make that obvious ("Create an account in your browser ↗"), so nobody is surprised to
  leave the app.

**Why a link and not in-app registration:** the app **cannot create a band** (verified — no
band-creation call exists under `app/`). A bare account made in the app would land on a Home with no
band, no concerts and no way forward. Every account created *through an invite* arrives already
attached to a band. Sending account creation to the browser keeps that property.

### 3. Opening a browser is new — and BRAND11 needs it too

The app **receives** `ACTION_VIEW` (`MainActivity.kt:156`) but never **sends** one. Use Compose's
`LocalUriHandler` rather than a hand-rolled `Intent`, and **agree the mechanism with
[BRAND11](BRAND11-a-way-back-to-the-project-page.md) §2**, which also opens an external URL from the
app. Two tasks inventing two ways to leave the app would be the thing to avoid.

## Do not lose (features hiding in the details)

- **The scan → paste fallback** (A53): the camera falls back to pasting when denied, so the paste field
  must stay reachable even with *Scan* as the headline.
- **`dropSessionIfOriginChanged`**: changing the Server URL drops the previous session. It is a
  correctness behaviour attached to that field — carry it across unchanged.
- **Discovery is conditional** (B06) — it renders only when non-empty, so it costs nothing when absent.
- **The modal stays a modal** (A38): ✕, tap-outside, Cancel and Back all dismiss, and **Back must not
  leave the app**. Tabs must not become navigation.

## Done when

- The two journeys are never both expanded at once, and the offline sentence sits above the tabs.
- The Sign in tab offers *Create an account* opening `<serverUrl>/register` in a browser; it is
  **disabled with a stated reason when the URL is empty** — check that state, it is the one a newcomer
  hits first.
- Discovery renders inside the Sign in tab.
- Scan-denied still falls back to paste; the origin-change session drop still fires.
- Back / ✕ / tap-outside still dismiss without leaving the app.
- `:shared:testDebugUnitTest` green, count matched; checked on a phone, which is where the scroll was
  the problem.
