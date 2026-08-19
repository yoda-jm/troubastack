# T80 — One add-file dialog shell: unify the three entries' interaction model

**Priority:** normal · **Size:** M · **Area:** `web/studio` only. Split from **T79** at its gate
(2026-08-20): T79 landed the naming/server half; Fable ruled the shell unification becomes its own
task so one task = one commit and the naming work — which fixes a wart biting today — ships now.

## Why

T79 landed good default names + clean pool names and kept the three add-file entries, but they are
still three different interaction models — the exact heterogeneity T79's design set out to remove:

| entry | today (post-T79) |
|---|---|
| `new-text-chart` | jumps straight into the editor with a stub (now defaulted to the song title) |
| `new-lyrics-chart` | opens a dialog (name, URL+fetch, paste, sections, Create) |
| upload | an inline form with a file input + Upload button |

All three produce the same thing — a new part in the pool — so they should look and land the same.

## Design (carried from T79 §1, deferred)

1. **One button group, one dialog shell.** The three entries sit together in the Files header,
   styled identically, and each opens the *same* dialog shell: a **name field**, a source area that
   differs per entry (editor stub / lyrics fetch+paste / file picker), one primary action, and the
   same landing — the new file is appended to the pool, visible immediately in the T78 list.
2. **Surface T79's defaults in the (editable) name field** — upload → filename without extension;
   from lyrics → typed name else fetched title; from scratch → the song's title. The default is
   pre-filled; the user may rename before creating. Do NOT reintroduce title-follow after create
   (T72/T79 guard).
3. **Preserve the lyrics dialog's behaviour + testids** (fetch / paste / sections / create); the T71
   search row, when it lands, drops into the same shell.

## Acceptance criteria

- The three entries are visually and behaviourally homogeneous: same placement, styling, dialog
  shell, primary action, and landing (appended, visible, named).
- Each entry, end-to-end: create → appears in the T78 list with the expected default name, which is
  editable in the shell before create.
- Existing lyrics fetch/paste/sections behaviour and its testids survive.
- Testids for the unified affordances; e2e covering each of the three entries end-to-end.
- `tsc -b studio` clean; `make e2e` green.
- Before/after screenshots of the Files header + each dialog in the handoff.

## Out of scope

- The naming/extension work (landed in **T79**) and the list/row presentation (landed in **T78**).
- The matrix view (parked) and per-member `my-files`.
- Making the e2e port configurable (a separate infra follow-up flagged at the T79 gate — the
  hardcoded `:8080` / `reuseExistingServer:false` blocks running e2e while a local preview holds the
  port).
