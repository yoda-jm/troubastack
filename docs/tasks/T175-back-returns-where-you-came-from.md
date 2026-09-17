# T175 — Back returns where you came from, not where the song lives

**Lane:** web-core · **Status:** specced, not started · **Origin:** VLL, 2026-09-17: *"when going from
setlist to song, the back is not the setlist, this is not practical, fix it and review other navigations."*

Specced from web-core's survey, which located the defect and inventoried every back affordance before
proposing anything. Their inventory is adopted as the scope.

## 1. The defect

`Viewer.tsx`'s back arrow is a hardcoded `to={`/bands/${bandId}`}` with `aria-label="Back to band"`, and
`SongEditor.tsx`'s error crumb repeats it. **A song has more than one parent** — the band's list, a setlist
row, a bare URL — and the arrow claims one of them in all three cases.

The setlist detail's own crumbs are correct (it has a single parent), and the **chart editor already solves
this exact shape one level down**, returning to the song with `?file=` restored — *"the reader's context"*.

## 2. ⟨D1⟩ — **SUPERSEDED 2026-09-17 by ⟨D5⟩. The origin goes in the PATH.**

Kept as the record of why. It ruled `?from=setlist:<id>`, citing the chart editor as precedent — and took
only half of that precedent: *restore the reader's context*, while missing **how the chart editor carries
it**. See ⟨D5⟩.

## 2b. ⟨D1-original⟩ Carry the origin in the query: `?from=setlist:<id>`

Not `navigate(-1)`: a deep link or a reload has no history, and after switching files inside the editor it
walks back through the reader's own edits. Not router link state: it is silently lost on reload, which is
exactly the state a musician is most likely to be in. A URL parameter survives both, is linkable, and
**matches what this very component already does with `?file=`** through `useSearchParams`.

## 3. ⟨D2⟩ The label says "setlist", not the setlist's name

The question web-core raised — *a back whose text lies is worse than one that goes to a fixed place* — has a
sharper form they half-saw. **A label rendered from the URL is text an attacker chooses.** `?from=` is
user-editable and shareable: parse it into *"Back to <name>"* and a crafted link puts arbitrary words in
someone's chrome, and a valid id from **another band** would render that band's setlist name to someone not
entitled to it if the name is fetched without the viewer's own authorisation.

So: **"Back to setlist"**, generic, no fetch, nothing derived from the parameter except the destination. It
is always true, it cannot leak, and it costs nothing. The name adds little — he was just there.

## 3b. ⟨D5⟩ The origin is a path segment: `/bands/:bandId/setlists/:setlistId/songs/:songId`

**VLL:** *"you used a from, wouldn't a path with the setlist be nicer? even if it is the same, I am looking
for idiomatic"* — and the router already agrees with him. `App.tsx:53` routes the chart editor as
`/bands/:bandId/songs/:songId/chart/:fileId`: **its parent is in the path and its Back is derived from it.**
Two idioms for one concept in one router is a divergence that costs someone a reading later, and the one
already there is the better of the two:

1. **Back stops being resolved and becomes derived.** Strip the segments. The membership fetch, the pending
   state and the whole `useBackTarget` mechanism disappear — the answer is in the URL and there is only ever
   one. My own GO on `bee1ca31` said an invariant that cannot be expressed wrongly beats one held up by
   tests; the query form was one step short of my own standard.
2. **The hostile-input class becomes unrepresentable.** A path segment cannot contain `/`, so
   `../../bands/other` is a different route or a 404, not a `:setlistId`. `parseFrom` defends against
   something the router makes impossible.
3. **The charset contract disappears.** Nothing parses the id, so nothing silently degrades the day the id
   format changes.

**Delete** `parseFrom`, `useBackTarget` and their unit tests. That is not lost coverage — the coverage moves
into the router, where it cannot be bypassed. Say so in the commit, because deleting tests reads as a
regression unless the reason is written down.

**The flat `/bands/:bandId/songs/:songId` route stays, permanently.** It is the song's canonical address,
the band list uses it, and every existing link and bookmark points at it. Two routes render the editor; that
is a fact to keep honest, not a migration.

## 4. ⟨D3⟩ — restated under ⟨D5⟩: the arrow's job ends at the destination

**Bare URL → the band, saying "Back to band".** Unchanged: the flat route has no origin, so there is nothing
to name.

**A deleted or foreign setlist in the path → go there anyway.** ⟨D3⟩ as first written conflated two things:
*the label must not lie about where the arrow goes* (the real property, and now structural — the label says
setlist, the arrow goes to the setlist route, they cannot disagree) and *the reader must not meet an error*,
which was never the arrow's business. The setlist page already owns its own not-found and its own auth;
duplicating that judgement inside a back button is a second copy of a truth that will drift from the first.

And silently retargeting is not neutral: it hides from him that something he was just using is gone. He
**did** come from there. If it has been deleted in another tab, the honest answer is the setlist page saying
so — not the band page appearing for reasons he cannot see.

**Not** the song's first setlist: a song belongs to several, "first" is arbitrary, and guessing is how the
label starts lying again. The band is the song's one unambiguous owner.

## 5. ⟨D4⟩ Scope — three files, and deliberately not a fourth

- **`SetlistDetail.tsx`** — the origin. `item-title-link` must append `?from=setlist:<id>`; the context has
  to be *written at the link*, not inferred at the destination.
- **`Viewer.tsx`** — the arrow reads it.
- **`SongEditor.tsx`** — the error crumb reads it too. Same defect, second site; fixing one and not the other
  is how a bug comes back.

**Do not touch the band song list.** From there the band *is* the origin, so today's default is already
correct, and a `?from=band:` would be noise that the fallback already covers.

## 6. Acceptance

The discriminating test, which passes today only if the fallback happens to coincide:

- **arrive from a setlist, press back, land on that setlist** — not the band;
- arrive from the band list, press back, land on the band;
- **reload the editor first, then press back** — still the setlist (this is the case link state fails);
- a bare `/bands/:b/songs/:s` → band, no error;
- `?from=setlist:<id that does not resolve for this viewer>` → band, no error, **and the label says
  "Back to band"** — the label and the destination never disagree;
- the error crumb obeys the same rules as the arrow.

Existing coverage asserts only that `tb-back` is *visible* (`editor-t66-move-default-chrome.spec.ts`);
nothing asserts where it goes, so every row above is new.
