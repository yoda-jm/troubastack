# T99 — Show the bake progress in the dialog ("song 3 of 11")

**Priority:** normal · **Size:** S · **Area:** `web/studio` (`pages/BakeDialog.tsx`, `api.ts`, e2e).
Lane: Web & Core. **The visible half of T96**, whose core endpoint landed `6f4e05c`.

VLL, 2026-08-24: *"spec the progress UI in the bake dialog"* — following T96, which deliberately built
the server side only.

## 1. Where it stands

`BakeDialog.tsx` has exactly one piece of feedback while a bake runs: the submit button's label flips
to **"Baking…"** and everything is disabled (`:150`, `busy` at `:33`). That is the whole readout for a
bake that takes **~13.5 s** on a 6-song annotated setlist. No count, no song, no sense of whether it is
moving.

T96 already publishes everything needed, per song, and `GET
/api/bands/{bandId}/setlists/{setlistId}/bakes/{bakeId}/progress` returns
`{state, done, total, song, error}`.

## 2. The one real constraint: the bake POST is synchronous, so its RESPONSE header is too late

**⚠️ AMENDED after the gate (Fable's RULING, 2026-08-24): the original framing below was wrong.**
It assumed the client could get the bake id by *reading the `X-Trouba-Bake-Id` response header*. But the
bake POST is synchronous (`bakeapi.go` blocks in `Bake()` for ~13.5 s, then sets the header at `:118`),
so `await fetch()` doesn't resolve — the header doesn't exist — until the bake is **over**. There is
nothing to poll during flight. And there's no listing endpoint to learn the id another way (T96, no
existence oracle). Reading the response header can *identify a finished bake*; it cannot *watch a
running one*.

**Resolution — (B) client-supplied id.** The client mints its own UUID and sends it as an **optional
`X-Trouba-Bake-Id` *request* header** on the bake POST; the server uses it as the bake id if it's a
well-formed UUID and currently free, else mints one (malformed/colliding is ignored, never a 400, never
a clobber of another bake's readout). The client polls that id from the instant it fires the POST. The
**response — status, body, echoed id header, 4xx-on-failure — is byte-for-byte unchanged**; the only
delta is one optional request input, so an old server ignoring it behaves exactly as today (the poll
404s → degrade to "Baking…"). This preserves everything §4 cares about (the POST stays the source of
truth for success/failure).

Implementation: a **dedicated `bakeSetlistWithProgress(bandId, setlistId, bakeId, layerDefaults?)`** —
its own `fetch` that sets the request header (the shared `request()` discards headers and is left
alone), plus `bakeProgress(...)` that GETs the endpoint and resolves `null` on any failure. `bakeSetlist`
stays for any other caller.

*(Original three options — widen `request()` ❌, id-in-body ❌, read the response header ✅ — are
superseded: the third couldn't work against a synchronous POST.)*

## 3. What to show

Poll the progress endpoint **while the POST is in flight**, and render three shapes:

| server state | dialog shows |
|---|---|
| `running`, `song` non-empty | **"Baking — song {done} of {total}: {song}"** |
| `running`, `done == total`, `song` empty | **"Finishing…"** |
| `failed` | the error, **naming the song** the server reported |

**The middle row is the one to get right.** After T98 the bake is: per-song staging (~11 s of the
13.5 s) → one overlay batch + assembly (~2.4 s). So the counter genuinely reaches N-of-N with seconds
left, and T96 publishes a song-less `running` update precisely so a client can say *finishing* instead
of showing a full bar that appears stuck. **Do not render `done == total` as "11 of 11"** — that reads
as frozen, and "why did it hang at the end" is the exact complaint this task exists to prevent.

Success needs no new state: the POST resolving is what closes the dialog, as today.

## 4. Degrade to today's behaviour, always

Progress is **decoration over an unchanged flow**. The bake POST remains the source of truth for
success and failure. Therefore:

- **No `X-Trouba-Bake-Id` header** (older server, proxy stripping it) → don't poll; show today's
  "Baking…". Not an error.
- **Progress GET fails or 404s** (expired entry, restarted server) → **stop polling**, keep the last
  line or fall back to "Baking…". A progress request failing must never fail a bake, and must never
  surface an error for a bake that is fine.
- **The dialog must never end up worse than it is now.** That is the non-regression, and it should be
  a test: with the endpoint unavailable, the dialog still bakes and still closes.

## 5. Polling details

- **Interval ~1 s.** A song takes roughly 2 s, so 1 s is responsive without hammering; anything faster
  buys nothing a human can read.
- **Start** once the id is known; **stop** the moment the POST settles (either way), on Cancel, and on
  unmount. **No interval may outlive the dialog** — assert it, don't assume it.
- Don't queue overlapping requests: skip a tick if the previous one is still in flight.

## 6. Acceptance criteria

- All three rows of §3 render from real server payloads, driven by intercepted responses.
- **The finishing tail is asserted specifically**: a `running` update with `done == total` and an empty
  `song` shows **"Finishing…"**, not "N of N". This is the assertion I care most about.
- **Graceful degradation, both cases**: missing header, and a 404/500 from the progress GET — the bake
  still completes and the dialog still closes.
- **A failed bake shows the song name** the server named.
- **No leaked timer**: closing or cancelling the dialog mid-bake stops the polling. Prove it (e.g. count
  intercepted progress requests and assert it stops climbing after the dialog goes).
- e2e is **network-free** — intercept both the bake POST and the progress GET with `page.route`, the way
  `lyrics-search.spec.ts` does. Do not stand up a real bake in the suite.
- `tsc -b studio` clean; full `make e2e` on isolated ports, announced at the gate first.

## 7. Out of scope

- Cancelling a bake from the dialog. T96's server side resolves a cancelled bake's terminal state, but
  a user-facing cancel is a product decision (what happens to a half-written rev?) and needs its own
  spec.
- Progress anywhere but this dialog — no Home surface, no app.
- Per-page granularity, and any change to T96's wire shape or auth.
