# T102 — A failed bake tells the user what went wrong, not a stack trace

**Priority:** high — VLL hit this today on his own band's concert · **Size:** S–M
**Area:** `core/internal/bake` + `core/internal/httpapi` (Web & Core lane). **Requested by VLL**, 2026-08-24.

> VLL: *"can the error be nicer instead of having a big stacktrace? … I mean for the user."*

## 1. What he saw, and why

When his GVO bake failed (the overlay CLI was missing), the Studio dialog showed him a raw multi-line
Node stack trace. That is the literal server-side error text, shipped to the browser and rendered
verbatim. Three links in the chain, all verified on `origin/main`:

1. **`render.go`** embeds the worker's entire stderr in the Go error:
   ```go
   return nil, fmt.Errorf("web/bake worker (%s %s): %w: %s", r.node, r.cli, err, strings.TrimSpace(stderr.String()))
   ```
   For a missing module that stderr is a full Node stack trace — and `r.node` / `r.cli` are **absolute
   server filesystem paths**.
2. **`writeErr`** (`webapi.go:1187`) ships it as-is: `writeJSON(w, code, map[string]string{"error": err.Error()})`.
3. **`BakeDialog.tsx:144`** renders it: ``setError(`Couldn't bake “${finalP.song}”.${finalP.error ? ` (${finalP.error})` : ""}`)``

**There are TWO channels, and fixing one is not enough.** T99 added a good human wrapper ("Couldn't bake
*Dirty Old Town*") — but it appends the raw text in parentheses, because `baker.go:160` publishes
`Error: err.Error()` into the progress record as well as returning it from the POST. Both carry the
trace. A fix that only touches `writeErr` leaves the stack trace visible through progress.

## 2. What to build

**A user-facing message, produced in core — not patched over in the studio.** The message the user reads
must be composed where the failure is understood, for two reasons: the studio cannot reconstruct meaning
from a stderr blob, and **the mobile app is about to hit these same errors** (A42 ② puts baking on Home),
so a studio-only fix would leave the app showing the trace.

Split every bake failure into two values:

- **A short human message** — what failed, on which song, and what to do next. One sentence, no newlines,
  no file paths, no stack frames. It names the song when the failure is song-scoped (T99's progress
  already carries the title, so use it) and stays honest when it isn't.
- **The full internal detail** — stderr, exit code, resolved binary and script paths — which goes to the
  **server log** and nowhere else.

Both output channels (the POST error body and `BakeProgress.Error`) carry only the human message.

Suggested shapes, to be refined by whoever builds it:
- worker won't start / missing script → *"The annotation renderer isn't available on the server. Ask an
  admin to check the bake setup."*
- worker failed on a song → *"Couldn't render annotations for “Dirty Old Town”. The server log has the
  details."*
- PDF rasterisation failed → *"Couldn't read the sheet music for “Dirty Old Town” — the file may be
  damaged."*

The exact wording matters less than the rule: **a band member reading it should know whether it's their
problem or the server's**, and never see a path.

## 3. Rules

- **Never put `err.Error()` on the wire for a bake failure** — neither channel.
- **Never lose the detail.** Every sanitised error logs its full text server-side, at the point of
  failure. This is not "swallow the error"; it is "put it where it belongs".
- **No absolute paths, no stack frames, no newlines** in anything the user can see.
- **Don't regress T99.** The dialog's existing "Couldn't bake *X*" flow, the degrade-to-"Baking…" path,
  and the terminal-state publishing all stay exactly as they are — this changes the *content* of the
  error string, not the mechanics.

## 4. Acceptance criteria

- **Reproduce VLL's exact failure**: point `TROUBA_BAKE_CLI` at a path that does not exist, bake, and
  assert the user-visible message (a) names the problem in one line, (b) contains **no newline**, (c)
  contains **no `/`-rooted path**, (d) does not contain `at ` stack-frame text. Assert it on **both**
  the POST response body and the progress record — one test each, because they are separate code paths.
- The full stderr **is** present in the server log for that same run.
- Teeth-check: revert the sanitisation and both assertions go red. A test that passes against
  `err.Error()` is guarding nothing.
- T99's bake e2e stays green; `gofmt -l core`, `go vet`, `go test ./...` clean.

## 5. The wider issue — flagged, not scoped in

`writeErr` ships `err.Error()` for **every** unmapped 500 across the whole API, so this class of leak is
not unique to bake — any internal failure anywhere can put server text and paths in front of a band
member. Fixing that globally is the right end state, but it has real blast radius (many tests assert on
error strings), so it is **deliberately out of T102's scope**. Fix bake properly first; if the shape
works, generalising it is a separate task worth filing.

## 6. Out of scope

Changing bake behaviour, retry logic, or the progress contract; the global `writeErr` change (§5); any
mobile-side work (A42 ② will consume whatever core produces here).
