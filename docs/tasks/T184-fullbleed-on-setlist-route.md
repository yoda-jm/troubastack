# T184 — The editor opened from a setlist is not full-screen: its bars drift off the viewport

**Lane:** web-core · **Status:** specced, not started · **Kind:** regression from T175 ⟨D5⟩ (`5292f280`,
2026-09-17) · **Origin:** VLL, 2026-10-01: *"les barres top et bottom de studio ne sont plus ancrées en haut
et en bas du viewport sur mon desktop"*.

## 1. Cause

`Shell.tsx` decides full-bleed with a regex over the path:

```ts
const fullbleed = /\/bands\/[^/]+\/songs\/[^/]+/.test(location.pathname);
```

T175 ⟨D5⟩ added a **second route to the same editor**, `/bands/:b/setlists/:sl/songs/:s`. That path has
`/setlists/…` between the band and `/songs/`, so the regex does not match. On that route the shell renders the
**normal** layout: the app navbar is shown, the document is not height-locked, and the editor's absolutely
positioned top and bottom pills scroll with the page.

T175 links setlist rows to that route, so **every song opened from a setlist** is affected.

## 2. Measured (1920×1080, Chromium, `origin/main`)

Same song, same PDF, a scroll-wheel over the page:

| route | `.shell-fullbleed` | document height | scrollY | top pill | bottom pill |
|---|---|---|---|---|---|
| `/bands/:b/songs/:s` | yes | 1080 | 0 | 10–57 | 1026–1070 |
| `/bands/:b/setlists/:sl/songs/:s` | **no** | **2673** | **1593** | **−1483** (off-screen) | 936–981 (mid-window) |

The navbar (`.topbar`, 0–57) is drawn only on the second route.

*Harness note:* a hard `page.goto` straight to the setlist route stalled in my scratch vite run
(`ERR_INSUFFICIENT_RESOURCES` on module loads, with no request flood from the app), so I reached it by a
client-side `pushState`. The existing `back-navigation` e2e does `goto` that route successfully, so this
is likely my harness. Note it only if you hit the same thing.

## 3. The fix: the property, not a second regex

**The property:** *every route that renders `SongEditor` is full-bleed, and no other route is.* The two
routes must not be able to disagree again.

Do not add an alternative to the regex. A third route to the editor would break it the same way, and that is
exactly how this one happened: the route table changed and a string 50 lines away did not. Derive the decision
from the **route table** instead, e.g. `matchPath` against the same pattern constants `App.tsx` uses, or a
flag the route carries. The mechanism is yours. The test below is what holds it.

The T105 chart editor (`/bands/:b/songs/:s/chart/:f`) matches the current regex today. **Keep its present
behaviour**, whichever it is; this task is not about it. But say in your submission whether it is full-bleed,
so the rule is written down.

## 4. Acceptance

- **The regression, pinned:** open the editor via the setlist route. `.shell-fullbleed` is present, the
  navbar is absent, and after a wheel scroll over the page `scrollY` is 0 and the top pill's top is ≥ 0.
  Assert the **pill positions**, not only the class: the class is the mechanism, and the anchored pills are
  the symptom VLL reported.
- The same assertions on the flat route, so the two cannot drift. One parametrised test over both routes is
  the right shape.
- **Teeth-check:** restore the old regex, confirm the setlist case goes red, and print the restored line.
- A non-editor page (the band page) is still **not** full-bleed.
