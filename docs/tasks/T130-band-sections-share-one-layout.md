# T130 — the band's three sections should share one layout (fixes both the wrong crumb and the "refresh")

**Lane:** web-core. **Size:** M. **Status:** spec, **dispatched — take it NOW**.
**Sequencing: not frozen.** VLL, 2026-09-03: *"T130 now, it's web so not frozen."* The pre-gig freeze
covers the **app binary** only; this is Studio, so it does not apply. It is also a defect VLL is
looking at today.
**Raised by:** VLL, 2026-09-03: *"in Studio back to bands is not coherent between Overview, Setlist and
Settings pages, this is super odd, of course fix it and also look if the same mistake is not made
elsewhere. Also the whole page seems to refresh, is it normal?"*

**Both of his observations have one root cause**, which is why they belong in one task.

## The root cause

Overview, Setlists and Settings present as **tabs of one band** — all three render `SectionTabs`. But
`App.tsx:42-44` registers them as **three unrelated sibling routes**, and each page imports and renders
the tab strip and the crumb **itself**:

```
<Route path="/bands/:bandId"          element={<BandDetail />} />
<Route path="/bands/:bandId/settings" element={<BandSettings />} />
<Route path="/bands/:bandId/setlists" element={<Setlists />} />
```

So switching tabs **unmounts the whole page** — crumb, tab strip and all — mounts a different one,
which **refetches the same band** and **gates its entire render** behind a full-page placeholder
(`BandDetail.tsx:61`, `Setlists.tsx:102`, `BandSettings.tsx:44` and `:53` — all
`return <div className="page">Loading…</div>`).

**Answering VLL's question directly: no, it is not a browser refresh.** `SectionTabs` uses `<Link>`, so
routing is client-side. What he sees is the tab strip he just clicked vanishing, the content blanking
to `Loading…`, and everything rebuilding — visually indistinguishable from a reload. The Settings tab
can even flicker in and out, because `showSettings` depends on `myRole` from each page's own fetch.

**And the same cause produced the incoherent crumb: it is written three times, so it diverged.**

| page | crumb label | target | `crumb` class |
|---|---|---|---|
| BandDetail (Overview) | "← Bands" | `/bands` | ✔ |
| Setlists | "← Back to band" | `/bands/:id` | **✘ missing** |
| BandSettings | "← Back to band" | `/bands/:id` | ✔ |

Two different destinations and two different labels across what look like tabs of one thing — and
Setlists' link is missing `className="crumb"`, so it is styled differently too. That is the "super
odd". From Setlists, "back to band" lands on Overview, whose crumb then reads "← Bands": two clicks to
leave, with the label changing under you.

## The same mistake elsewhere — VLL asked, so here is the sweep

Missing `className="crumb"` on a back link (6 occurrences):

- `Setlists.tsx:114`
- `SongEditor.tsx:58`
- `Join.tsx:63` and `Join.tsx:74`
- `BandDetail.tsx:54` and `BandSettings.tsx:48` — **both are error-state crumbs**, and both differ from
  their own page's normal-state crumb. `BandSettings` is the sharp one: on error it says "← Bands" to
  `/bands`, and on success "← Back to band" to `/bands/:id`. **The destination changes depending on
  whether the fetch failed** — nothing about an error should move where "up" is.

`SetlistDetail.tsx:72,93` is the **well-formed example**: same label, same target, `crumb` class, in
both branches. Copy that shape.

## Work

**1. Add a shared band layout route.** One element that owns the crumb, the tab strip, and **one** band
fetch, with `<Outlet/>` for the section:

```
<Route path="/bands/:bandId" element={<BandLayout />}>
  <Route index          element={<BandDetail />} />
  <Route path="setlists" element={<Setlists />} />
  <Route path="settings" element={<BandSettings />} />
</Route>
```

The crumb and strip then **persist across tab switches** instead of unmounting, the band is fetched
once, and `myRole` stops flickering. This is the fix for the "refresh".

**2. The crumb is defined once, in the layout — decide its meaning and stop repeating it.**
Since the three are tabs of one band, "up" from any of them is the **bands list**: `← Bands` →
`/bands`. Going "back to band" from a tab is meaningless once the tabs persist — you are already in the
band. Deleting two of the three copies is what makes divergence impossible again.

**3. Loading must not blank the page.** Keep the crumb and tab strip rendered while the section loads;
only the section body shows a placeholder. A layout that disappears while loading is the whole
complaint.

**4. Fix the six stray links** listed above — `crumb` class, and error-state crumbs matching their own
page's normal-state destination.

## Do not

- Do not change what the tabs *are*, their order, or their admin-gating.
- Do not convert `<Link>` to `<a>` anywhere; client-side routing is correct and is not the problem.
- Do not "fix" the flicker by caching `myRole` in a module-level variable — the layout fetch is the fix.

## Done when

- Switching Overview ⇄ Setlists ⇄ Settings **keeps the crumb and tab strip mounted** — assert in an
  e2e that the tab strip element is present continuously across a switch, not merely present after it.
  A test that only checks the end state would pass today.
- The crumb reads the **same label and points to the same place** from all three sections.
- The band is fetched **once** per band visit, not once per tab. Assert it, e.g. by counting requests.
- No back link anywhere is missing `className="crumb"`; error-state crumbs match their page's
  normal-state destination.
- `tsc -b` clean; e2e green.

## Sequencing

**Now.** The 2026-09-05 freeze protects the **app binary**; T130 is entirely `web/studio`, so nothing
here can affect what VLL performs with. Land it the moment it is verified — do not hold it for the
concert.

One caution that is about the gig rather than the freeze: this touches the routes behind the band
Overview, Setlists and Settings, which is the path used to prepare a concert. **Verify the three
sections still load and the setlist flow still works end to end** before landing, not only that the
crumb reads the same. A broken band route two days before a gig would be worse than an odd crumb.
