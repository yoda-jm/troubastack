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

## 2. ⟨D1⟩ Carry the origin in the URL: `?from=setlist:<id>`

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

## 4. ⟨D3⟩ Unknown or unresolvable origin falls back to the band, silently

A bare URL, a deleted setlist, a `?from=` pointing somewhere this viewer cannot see: **the arrow reverts to
the band and says "Back to band"**. No error, no toast — a back button is not the place to report a problem.

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
