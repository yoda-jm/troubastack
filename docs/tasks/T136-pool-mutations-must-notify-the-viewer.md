# T136 — Every pool mutation must notify the Viewer, not just the one that happened to

**Lane:** web-core (studio). **Size:** S. **Status:** spec — bug reported by VLL on the live demo,
2026-09-04, diagnosed against `e6789224`.

## Symptom

**VLL:** *"when renaming a file in the 'Details' it does not change in the bottom menu of the page"*, and
*"same for changing order"*.

## Root cause: the notification is wired to a call site, not to the operation

`SongDetails.tsx` takes an `onFilesChanged?: () => void` prop, and `Viewer.tsx:1545` passes
`refreshMyFiles` into it — that is the bottom strip. The prop's own comment (T67) explains it exists to
bubble *"a pool mutation … up to the Viewer so it refetches 'my files' and the open render updates
in-session, no F5"*.

**It is called in exactly one place** — line 488, the chart re-render path it was added for. Every other
pool mutation reloads `SongDetails`' own `files` state via `load()` and tells the Viewer nothing:

| line | mutation | notifies |
|---|---|---|
| 277 | upload (create) | ❌ |
| 280 | the upload's follow-up rename (T80 name field) | ❌ |
| 301 | **rename** — VLL's report | ❌ |
| 313 | **delete** | ❌ |
| 342 | **reorder / `displayOrder`** — VLL's report | ❌ |
| 488 | chart re-render (T67) | ✅ |

So the feature was never "rename doesn't refresh"; it is **"only the mutation T67 needed refreshes."**
Two were reported; there are five.

## ⚠ Delete is worse than the two that were reported

`refreshMyFiles` does more than relabel — it repairs the selection: *"preserves the current one if it
survives, otherwise falls back to the first viewable"*. Because delete never notifies, **that repair
never runs**: the strip keeps an entry for a file that no longer exists, and `selectedFileId` can still
point at it. A cosmetic bug for rename and reorder; a dangling selection for delete.

## Fix

One helper used by every mutation:

```ts
async function reloadPool() {
  await load();
  onFilesChanged?.();
}
```

…called at all five sites in place of the bare `await load()`.

**Do not move the notification inside `load()`.** `load` also runs on mount (`useEffect`, line 247), so
that would fire a Viewer refetch on every mount of the Details pane — extra requests, and a re-render
loop if the parent's refetch remounts the child.

## Acceptance

- **e2e, the two VLL reported:** rename a file that is in the bottom strip → the strip shows the new name
  without a reload; reorder the pool → the strip's order follows.
- **e2e, the one he did not:** delete the file that is currently selected → it leaves the strip **and**
  the selection falls back to another viewable file rather than pointing at the deleted id.
- Upload and the post-upload rename likewise reach the strip.
- No extra `getMyFiles` request on mounting the Details pane (guards against the `load()` shortcut).

## Why it is worth a task rather than a one-line patch

The same shape produced the band-rename bug already solved in `BandSettings.tsx:23` (*"setBand lets
Rename update the shared copy"*). **Two components each holding their own copy of the same server state,
with one ad-hoc notification between them**, will keep producing this bug per operation. If a shared
source for the file pool is cheap here, prefer it; if not, the helper above at least makes the
notification a property of *every* mutation rather than of the one someone remembered.
