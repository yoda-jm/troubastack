# P206 — Jump marks: an author-placed link to another page of the same document

**Status:** **QUEUED — mobile lane** (VLL, 2026-09-02). All four design decisions are settled and
recorded in §Decisions: the feature is a **jump mark**; a tap opens a small "go to" popup above the
mark, with a **direct-goto opt-in** in the Stage section of Parameters; there is **no Return**, so
`MainActivity.kt:628` is not touched; cross-song jumps are out of scope. **Startable now** — the concert that gated it was cancelled (see the sequencing note); it still
touches proto and all three lanes, so stage it accordingly. Originally requested by VLL, 2026-08-30: *"I think I want to spec a
new tool: hyperlink (to somewhere in the same pdf, you would give a page number I suppose, and it goes
back to this page). Spec this in all cases (including scroll (maybe at the top not top of the page),
multipage (landscape), …)"* · **Size:** L, staged across three lanes · **Verified against `a46ecc8`**
**Areas:** `proto/` + generated mirrors · `web/studio/src/annotations` · `core/internal/bake` ·
`app/shared/.../stage` + the Android host.

> ⏱ **Sequencing — the hold is LIFTED (2026-09-04).** This note used to say "nothing here should start
> before the concert on 2026-09-05". **The concert was cancelled**, so the reason for the hold no longer
> exists and P206 is startable now. Recorded rather than deleted, because a stale hold that nobody
> re-reads is how a queued spec sits still for a week.

## The model, in one paragraph

An author draws a **jump mark** on a page: a small rectangle that says "go to page N of this
document". On the Stage a tap inside it opens a small "go to" popup; confirming navigates there.
**There is no Return** — decision 3 struck it; back still leaves the concert. The destination is a page **of the same document** plus an optional
point on that page, so the reader lands *at the passage*, not merely on the sheet containing it. It is
one more annotation object — it lives on a layer, obeys layer visibility, and is baked into the
bundle like everything else.

## Naming — a ruling, overrulable

VLL said *"hyperlink"*. **I recommend against shipping that word.** Musicians have had this exact
device for three centuries: *D.S.*, *D.C.*, *Segno*, *to Coda* — a written mark that says jump there,
and a convention for coming back. "Hyperlink" imports a browser into a music stand. The tool is a
**Jump mark**; the affordance that comes back is **Return**. (A36 already established that this
product picks the musician's word over the software word.) The code name in proto/geometry is `jump`.
**If VLL prefers "Link", it is one string change and I will make it — the design below is unaffected.**

## What is actually new

Everything else in this repo that appears on a page is **pixels**. `PageImages` carries a page raster
plus one transparent overlay per layer (`bundle.proto`), and `StageModel.kt` opens with *"a pure image
compositor + pager… No annotation model"*. **A jump mark is the first thing on a page that a reader
can interact with**, so it cannot be flattened: it needs geometry and a destination in the manifest,
next to the pixels rather than inside them. That is the whole reason this is an L and not an
afternoon.

Two consequences fall straight out and both are load-bearing:

1. **The mark's ink and the mark's hotspot are different things.** The visible box is drawn by the
   normal annotation pipeline (so it rasterizes, tints under night mode via `colorFilter` (A10), and
   sits in the right z-band for free); the hotspot is invisible geometry the Stage hit-tests.
2. **A second data channel is a second place to forget a filter.** Overlays are already filtered twice
   — by layer visibility (`StageState.visibleFor`, A1) and by owner identity
   (`visibleToIdentity`, P205 Stage 3b, which *drops other members' personal layers at load*). A
   `jumps` list that skips those filters would give the reader an **invisible tappable region** —
   worse, one that leaks the existence of another member's private annotation. **§4.4 is not optional
   polish; it is the correctness core of this feature.**

## Why the destination is not a page number

VLL said *"you would give a page number I suppose"* — that is right for the **author**, and wrong for
the **wire**. A raw index is fragile against exactly the thing this project does constantly: re-baking.
A46 already ruled on this shape and the reasoning transfers verbatim — `resolveStartPage` keys the
saved reading position on the *logical* `(songId, pageInSong)` "so it survives a re-bake that reorders
songs/pages", and clamps to the song's **last** page when a shorter re-bake removed the target.

So: **the author types a page number; the wire carries a page-in-document index; the Stage resolves it
with the same clamp discipline as A46 and never dangles.** The three coordinate spaces, which must not
be confused:

| space | who uses it | note |
|---|---|---|
| page-in-file | the author, in Studio | what the person typed |
| page-in-song | the bundle (`target_page`) | the baker translates; see below |
| global page index | `StageState.current` only | never crosses the wire |

**The translation is free, and here is why it is safe:** `Baker.defaultFile` bakes exactly **one file
per song** (lowest `DisplayOrder` viewable PDF), and `annotations.go:88` already drops any layer whose
`FileID` is not that file. So for the file that got baked, page-in-file **is** page-in-song, 1:1. Marks
authored on a part-file that was not baked vanish with the rest of that file's annotations — the
behaviour they already have, not a new rule.

## Scope: within one document, and why that is not a limitation

VLL's own words are *"somewhere in the same pdf"* and the data model agrees hard. A song is authored
**independently of any concert**: layers hang off a file, and the song has no idea which setlist will
carry it. A cross-song jump would therefore be a reference that is **valid or dangling depending on
which bundle it lands in** — the same mark would work in Friday's set and break in Saturday's. There is
no place to author it and no honest way to render the broken case mid-performance.

**Cross-song jumps are out of scope, permanently unless the model changes.** The song drawer (A15)
already crosses songs, and it is the right tool for it.

---

## Stage 1 — proto + mirrors (core lane)

Additive; every mirror carries `AUTHORITY: proto/…`.

**`object.proto`** — the authored form:

```proto
  // P206: an in-document jump mark — a tappable rect that navigates to another page
  // of the SAME file. `points` are the bbox corners (as OBJECT_TYPE_ICON does).
  OBJECT_TYPE_LINK = 8;
```

and on `Object`:

```proto
  // P206: set only for OBJECT_TYPE_LINK; the destination, within the same file.
  JumpTarget jump = 12;

message JumpTarget {
  int32 page = 1;      // 0-based page index within the SAME file
  float anchor_y = 2;  // [0,1] normalized y on that page (I3). 0 = the page top, which
                       // is also the proto3 default — absent and "top" are the same
                       // thing here, so no presence wrapper is needed.
}
```

**No `anchor_x`.** Deliberate: no Stage mode can pan horizontally — `FIT_PAGE` fits, `FIT_WIDTH` and
`SCROLL` fill the width — so an x would be carried, tested and never read. It is additive if a zoom
mode ever lands.

**`bundle.proto`** — the baked form, on `PageImages`:

```proto
  repeated PageJump jumps = 4;  // P206: tappable jump marks on this page

message PageJump {
  float x0 = 1; float y0 = 2; float x1 = 3; float y1 = 4; // normalized bbox [0,1] (I3)
  int32 target_page = 5;        // 0-based page index WITHIN THIS SONG
  float target_anchor_y = 6;    // [0,1] on the target page; 0 = top
  string layer_id = 7;          // visibility follows THIS layer, exactly (A1/R7)
  string owner = 8;             // P205: "" = shared; a member id = personal
}
```

`layer_id` and `owner` are **not** conveniences — they are the two filters of §4.4. A `PageJump`
without them cannot be filtered and must not exist.

### The enumeration nobody will remember

I enumerated this by reading every site rather than trusting the comments, and **the comments are
wrong in both directions** — so here is the real map:

| site | kind | needs an edit? |
|---|---|---|
| `proto/troubastack/v1/object.proto` | source of truth | ✅ |
| `core/internal/domain/domain.go:23-32` | hand-written `iota` block | ✅ **append only** |
| `core/internal/domain/objecttype_gen.go` | generated, both directions | — regenerates |
| `web/ink/src/objecttype.gen.ts` | generated | — regenerates |
| `core/internal/bake/annotations.go` `objectTypeString` | hand-written duplicate, `default: return ""` | ⚠️ ✅ |
| `web/studio/src/annotations/link.tsx` + `registry.ts` | the tool itself (Stage 2) | ✅ |

Three things to take from that table:

- **`domain.go` is `iota`, so position IS the wire number.** Append `TypeLink` at the end. Inserting
  it anywhere else silently renumbers every existing type — every stored annotation in the band's
  history changes meaning, with nothing failing loudly.
- ⚠️ **`bake/annotations.go` is a silent drop.** An unmapped type returns `""` and the object bakes as
  *nothing*: no error, no warning, a mark that exists in Studio and is simply absent from the bundle.
  **Assert the emitted string in a test** — a test that merely bakes without crashing passes while the
  mark is being discarded.
- **`httpapi` needs no change**, contrary to the comment above `objectTypeString`, which says it
  *"mirrors httpapi's objectTypeToString (kept in sync by review)"*. **There is no such function.**
  httpapi routes through the generated `domain.ObjectTypeToString/FromString`
  (`httpapi/annotations.go:267,293`); bake is now the **last** hand-written duplicate, and its comment
  points at a mirror that no longer exists.

> 📋 **Two stale docs found while writing this — recorded, not queued.**
> (a) The comment above `bake/annotations.go`'s `objectTypeString` names a function that no longer
> exists; the honest fix is for bake to call `domain.ObjectTypeToString` and delete the switch, which
> would retire this whole hazard. (b) `web/studio/src/annotations/README.md` step 3 says to *"add the
> type to `InkObjectType` in `web/ink/src/index.ts`"* — that is stale too: `InkObjectType` is
> `ObjectType | "arrow"`, derived from the **generated** union, so a proto type arrives there for free
> and only the dev-only `arrow` is manual. Neither is a defect; both would cost a lane an hour.

Also: `web/studio/src/annotations/README.md` warns that **the server rejects mutations of a type it
does not know** — which is why the `arrow` demo is still `localStorage.devArrow` gated. Stage 1 is what
stops jump marks from being a dev-flag toy; **it must land before Stage 2 is worth running.**

**Verification:** `cd core && go run ./cmd/gen-mirrors` regenerates `objecttype_gen.go`,
`bundle_gen.go`, `api.gen.ts`, `objecttype.gen.ts` and `BundleModel.kt`; **CI drift-guards it**.
`gofmt -l core` before landing.

## Stage 2 — authoring (web lane)

`web/studio/src/annotations/` is a clean registry (T07): **one descriptor file + one line in
`registry.ts` + one string in ink**, no edits to `editor.ts` or `SongEditor.tsx`. Follow `icon.tsx`
(also a bbox-shaped, non-stroke type) rather than `arrow.tsx`.

- **Drawing** a mark is a drag-rect, then a small popover: *"Go to page ___"*, defaulting to the
  current page + 1. Validate against the file's real page count and refuse out-of-range **at
  authoring time** — the one moment a human can fix it.
- **The anchor** is the answer to VLL's *"maybe at the top not top of the page"*. Offer it, do not
  demand it: after choosing the page, the author may click a point on the destination preview to say
  *land here*. No click ⇒ `anchor_y = 0` ⇒ the page top. **A jump with no anchor must be a complete,
  useful jump** — the anchor is precision, not a prerequisite.
- **Minimum drawn size.** Enforce one (a percentage of the page's short edge). §4.1 gives the hotspot
  *no* touch padding on purpose, so an author who draws a 3 mm box has made an untappable mark. Catch
  it here, where it is fixable.
- **The mark reads as a jump** without the app's help: the default style is a labelled box (e.g.
  `→ 3`) so it is legible on paper and in the printed concert PDF, where nothing is tappable.
- **`make e2e` is mandatory** for this stage — a new toolbar button is a user-visible change, and
  README ground rule 6 is explicit that a UI change gated only by `make test` is how a stale e2e
  assertion once rode `main` red for a whole task window.

## Stage 3 — bake (core lane)

Translate authored marks into `PageJump`s on the page they sit on, carrying `layer_id` and the layer's
`owner`. Then the rule that keeps the bundle honest:

> **A jump whose `target_page` is not a real page of the baked file is DROPPED, with a bake warning
> naming the song and the page.**

This is not defensive padding, it is a case that **will** occur: the D1 transpose path bakes the
*generated chart* instead of the PDF (`Baker.generatedChart`), and the chart's pagination is its own
(T75 compaction, T76 auto-fit, T77 explicit breaks). A mark authored against the PDF's page 4 means
nothing there. The same check covers song-level layers (empty `FileID`), which `annotations.go`
composites onto **whatever** is baked.

Dropping is right and asking is wrong: a warning at bake time reaches an admin at a desk; a dangling
jump reaches a musician on stage.

## Stage 4 — the Stage (mobile lane)

This is the stage VLL actually asked about: *"in all cases"*.

### 4.1 Activation, against N3

N3 is explicit and was a considered decision: *"EVERY tap, in EVERY mode, toggles the chrome… Page
turns are swipe + ‹ › FABs + pedals/keys only — edge-tap-turn was dropped because an accidental turn
reads as a rendering glitch mid-performance."*

**A jump mark is allowed to break that, and here is the argument.** N3 rejected *invisible, huge,
implicit* tap zones. A jump mark is *visible, small, and put there on purpose by a member of the band*.
The mis-tap cost is also different: an accidental page turn leaves you hunting for your place; an
accidental jump is undone by the Return that the jump itself just armed.

Conditions, all required:

- A tap inside a **visible** mark activates it and **does not** toggle chrome. Every other tap behaves
  exactly as N3 says. `tapAction()` gains one branch and keeps its pure-function contract.
- **No touch-slop padding.** The hotspot is the drawn rect, full stop. Padding a hit target is the
  normal instinct and it is wrong here — it converts near-misses into unwanted jumps, which is the
  exact failure N3 was protecting against. Studio's minimum size (Stage 2) is the counterweight.
- Activation is on **tap**, not drag, so `pointerInputSwipe` still gets its page turns — a drag that
  begins on a mark is a page turn, unchanged.
- The hit test is a **pure function** — `jumpAt(page, nx, ny, visibleLayers, identity): PageJump?` —
  unit-tested off-device, exactly as `tapAction` pins N3 today. Zero instrumented tests exist in this
  repo (A58 §"Explicitly NOT"), so a pure seam is the *only* thing that can guard this.

### 4.2 Landing — every mode, which is the point of the task

Common rule first: **`state.current` becomes the target page in every mode.** It is the single source
of truth (A12/A13/N6 all derive from it) and the pager label, the A46 saved position and the drawer
highlight all read it. What differs is what else has to move.

| mode | what moves | where it lands | anchor |
|---|---|---|---|
| `FIT_PAGE`, portrait (one-up) | `current` | the whole target page | ignored for position — cue only |
| `FIT_PAGE`, landscape (two-up) | `current` | the **spread containing** the target — `spreadFor(target, songStarts, pageCount)` | ignored for position — cue only |
| `FIT_WIDTH` | `current` + that page's own `verticalScroll` offset | the anchor near the top of the viewport | **yes** |
| `SCROLL` | `scrollListState` (+ `current`) | the anchor near the top of the viewport | **yes** |

**Two-up (`multipage (landscape)`).** Land on the target's spread and **do not force the target to the
left half.** Pairing is song-aligned (N6) and forcing would either straddle a song boundary or shift
every subsequent spread — you would fix a cosmetic preference by breaking the invariant. So a jump to
an odd offset legitimately shows the target on the **right**, which makes §4.3's arrival cue
load-bearing, not decorative.

> ⚠ **The case that will look broken.** A mark on the left page pointing at the right page of the
> *same spread* moves nothing at all. The reader taps, the screen is identical, the mark reads as dead.
> This is the same class of bug `nextSpreadPage` already carries a comment about (*"extra swipe with
> animation, same page"*). **The arrival cue is the fix and the test for this case is mandatory.**

**`FIT_WIDTH` and `SCROLL` — "maybe at the top not top of the page".** This is the sentence the whole
feature turns on, and it is right: scrolling to the *page* top when the passage is two thirds down the
sheet has moved the reader to the correct piece of paper and left them to find the line themselves.

Landing offset, as a total function:

```
offset = anchorY * pageHeightPx − LEAD_IN
LEAD_IN = a small named fraction of the viewport height   (a sliver of context above,
                                                           so the target reads as "here"
                                                           and not as "cut off at the edge")
clamp offset into the scrollable range
```

Three things that must be true and are each a test:

1. **Clamping is not a failure.** An anchor near the end of the column cannot reach the top; it lands
   as high as the column allows and stays visible. Never bounce, never overscroll, never refuse.
2. **`anchor_y = 0` lands at the page top** — identical to the no-anchor case.
3. **In `SCROLL`, the offset is measured from the PAGE's top, not the list item's top.** `ScrollReader`
   builds each item as `Column { MetaStrip(page); ScrollPage(…) }`, and `MetaStrip` **renders only on a
   song's first page** (A08). A naive `scrollToItem(index, offset)` is therefore correct on every page
   except the first of each song, where it lands short by the strip's height. **The first page of a
   song with a metadata strip is a required test case** — the bug it catches is invisible everywhere
   else.

**The `SCROLL` gap you cannot skip.** In scroll mode the column — not `state.current` — is the source
of truth for the topmost page (`StageScreen.kt:276-277`), and `ScrollReader` repositions on
`LaunchedEffect(state.currentSong)`, keyed on the **song**. So setting `current` to another page **of
the same song** repositions nothing: the label changes and the page does not move. The jump must drive
`scrollListState` explicitly, exactly as the scroll-mode turn handlers already do
(`animateScrollToItem`, `StageScreen.kt:362/371`). Cross-song is not reachable here (§"Scope"), so
same-song is the *only* case — which is precisely the one the existing effect does not cover.

### 4.3 Arrival cue

Reuse the existing transient-cue machinery (`cueEpoch` / `autoHideChrome`, as N1's boundary cue and
N7's blocked-turn cue do). A brief pulse at the anchor point — or on the target page in two-up.

It earns its place three times over: it is the **only** feedback in the same-spread case (§4.2), the
only way to identify which half of a spread you were sent to, and the only signal that a clamped
landing put the target mid-screen rather than at the top.

### 4.4 Filtering — the bypass

A `PageJump` is tappable **iff** its ink is visible. Both existing filters apply, unchanged:

1. `jump.layerId ∈ state.visibleFor(page.songId)` — per-song layer visibility (A1), which unions
   mandatory layers at read.
2. `visibleToIdentity(jump.owner, identity)` — P205 Stage 3b.

A jump that fails either is **not hit-tested, not drawn, not counted**. Filter it at the same place
overlays are filtered, so the two cannot drift: `buildLoaded` already drops other members' overlays at
load, and jumps must be dropped **in the same pass, from the same predicate**. Two call sites doing the
"same" check is how this goes wrong six months from now.

**Guard it with the test that would actually fail:** hide the mark's layer, tap where it was, assert
**nothing happens** — not merely that the mark is not drawn. A test that only checks rendering passes
happily while the hotspot is live.

### 4.5 The Return — ~~STRUCK by decision 3~~ (kept only as the record of why)

> ⛔ **DO NOT IMPLEMENT ANY OF THIS SECTION.** VLL struck the Return on 2026-09-02: the presenter has
> no back, and a jump is navigation *within* a document. **Do not touch `MainActivity.kt`'s
> `BackHandler`** — its current behaviour (back leaves the concert) is correct and stays.
>
> It is kept, quarantined, because it records the riskiest part of the original design and why it was
> removed. Everything below is history, not instruction. The **arrival cue survives** — it is what
> tells a reader the jump happened, and it is specified in §4.3, not here.


*"and it goes back to this page"* — this is the second half of the feature, not a nicety.

**What is stored is a viewport, not a page.** In `FIT_WIDTH` and `SCROLL` the page index does not
describe where you were; returning to the top of a page you were halfway down is not returning. Capture
at jump time: `(songId, pageInSong, scroll offset, fitMode)`. On Return, restore the position; if the
reader changed fit mode in between, restore the page and let that mode's own rule (§4.2) place it —
the stored offset is meaningless across a mode change and must be discarded, not reinterpreted.

**One slot, not a stack.** A jump replaces any armed Return. Nesting a "back" history into a
performance surface buys a case nobody can predict mid-set at the cost of one the reader can always
predict: *the last jump is what comes back.* The musical model agrees — after a *D.S.* you play on, you
do not unwind.

Lifetime:

- **Armed** by a jump.
- **Survives ordinary page turns within the destination song** — "jump ahead, read a bit, come back"
  is the real use, and clearing on the first turn would break it.
- **Cleared** by crossing songs, by a new jump (replaced), and by leaving the Stage.
- **Never persisted** (I12 — the A46 saved position is a *reading position*, not a jump history). A
  cold start has no Return, by design.

**Presentation:** a Return control in the chrome while armed, following the existing chrome rules
(A50's `stageHoldsKeyFocus` is untouched — a pill is not a focus stealer).

**System back — a real defect today, verified.** `MainActivity.kt:628` is
`BackHandler { selectedDir = null }`: **system back leaves the performance, unconditionally.** Ship the
Return without touching it and a reader who taps a mark and presses back is thrown out of the concert.
It must become "return if armed, else exit".

That handler lives in the **Android host**, while the Return state lives in **shared** `commonMain`
(`BackHandler` is `androidx.activity.compose` and there is an iOS host too). **Use the registrar
pattern the codebase already uses for exactly this shape**: `LocalVolumeTurnRegistrar` exists because
Android volume keys cannot reach Compose, so the host intercepts and calls back in, and *"iOS provides
no registrar → the default no-op"*. Return is the same problem and takes the same answer. **Do not
invent a second mechanism.**

### 4.6 Degenerate cases — each one a test

- **Target page's raster is `UNAVAILABLE`** — navigate anyway. `BundleLoader` is total by design (I12,
  *"one bad page must never take down a performance"*); the reader gets the placeholder card at the
  right index, which is honest. Refusing to navigate would be the app hiding a page that exists.
- **Target page is beyond the song's pages** (a shorter re-bake, or a bundle predating Stage 3's
  validation) — **clamp to the song's last page**, the identical rule and rationale as
  `resolveStartPage`. Never dangle, never no-op silently.
- **A jump to the page you are already on** (with an anchor) — a legitimate move; it scrolls. Only a
  jump to the current page *with no anchor* is a true no-op, and it still flashes the cue.
- **Old bundles** carry no `jumps` — proto3 default-empty, nothing to do, no version gate.
- **Old app, new bundle** — unknown field, ignored; the mark's ink still renders. **The feature
  degrades to a drawing, which is exactly right**: the reader sees `→ 3` and turns to page 3.

---

## Explicitly NOT in scope

Cross-song jumps (§"Scope") · links to another concert, file, or URL · a back/forward **history** ·
jumps in the Studio viewer or the exported concert PDF beyond rendering the ink · editing a jump's
destination from the Stage · iOS host back wiring beyond the no-op default · instrumented/Compose UI
tests (there are none in the repo; that gap is the standing audit §5 row and is its own task) ·
anything that makes `StageModel.kt` stop being a pure compositor + pager beyond the one hit test.

## Decisions — settled by VLL, 2026-09-02

1. **The word is "Jump mark."** UI and wire format both; the proto field is `jumps`.

2. **A tap does NOT navigate directly.** It opens a **small popup anchored above the mark**,
   reading *"go to …"*; navigation happens on confirming that. **Direct navigation is opt-in**, a
   setting the reader turns on for themselves.

   This is better than the N3 exception I proposed, and it is worth saying why: N3 forbade
   tap-navigation because invisible, large, implicit zones cause accidental moves. A confirm step
   removes that risk *by construction* rather than by argument, so **N3 is not reversed at all** —
   the default behaviour never navigates on a bare tap. My §4.1 exception is withdrawn; the popup
   replaces it.

   **Where the setting lives — already answered by the code, not a new decision.** There is one
   settings screen (`ui/SettingsScreen.kt`, titled "Parameters") and it carries a **"Stage"
   section** whose subtitle reads *"Also on the ⚙ in concert mode — same setting, whichever is
   handier."* `StageScreen` opens the same sheet from its ⚙ FAB. "Direct goto" belongs in that
   Stage section, and therefore appears in both places, like reading mode and colour mode.

3. **No Return at all.** VLL: the presenter has no back, and this is navigation *within* a page.

   **This deletes the riskiest part of the feature, and that is the point.** §4.5 required changing
   `MainActivity.kt:628` — `BackHandler { selectedDir = null }`, which exits the performance
   unconditionally — into "return if armed, else exit", wired through `LocalVolumeTurnRegistrar`
   because the handler is in the Android host while the state would be in `commonMain`. **None of
   that is needed now.** Do not touch the `BackHandler`. Its current behaviour is unchanged and
   correct: back leaves the concert.

   Consequently §4.5, the Return slot, the arming rules and the iOS no-op default are all struck.
   The arrival cue stays — it is what tells a reader the jump happened.

4. **Cross-song jumps stay out of scope**, as specified.

Everything else in this document I rule on at the gate.
