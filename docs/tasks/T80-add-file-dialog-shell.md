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

### Amendments (Fable, at spec review 2026-08-20)

The draft carries T79 §1 faithfully. These four points are things a shared shell forces you to
decide, and which cost far more to discover mid-implementation than to rule now.

4. **What happens after Create — "same landing" means the same *pool* landing.** All three append
   the row identically and it is immediately visible. **From-scratch additionally opens the editor**,
   as it does today: its body is empty by definition, so creating it and stopping in the Files list
   is a dead end the user would immediately click through. Upload and from-lyrics do **not** open the
   editor. State this in the UI copy if the difference is ever surprising; do not "simplify" it into
   uniformity.
5. **Validation is per-entry, not per-shell.** Today `lyrics-create` is disabled while the text is
   empty. A shared shell tends to acquire shared validation — but an empty body is **legal and
   normal** for from-scratch (it starts from a stub) and **not** for from-lyrics. Keep the rule
   attached to the entry, not the shell.
6. **Upload's name field populates from the chosen file, and must not clobber the user.** The default
   (filename minus extension) can only be known after the file is picked, so: pre-fill on selection
   **only if the user has not typed a name**. Typing then choosing a file must not silently discard
   what they typed.
7. **Cancel creates nothing.** One shared dismissal path (escape / cancel) that uploads nothing,
   creates no chart, and leaves the pool untouched — worth naming because three entries now share it.

## Acceptance criteria

- The three entries are visually and behaviourally homogeneous: same placement, styling, dialog
  shell, primary action, and landing (appended, visible, named).
- Each entry, end-to-end: create → appears in the T78 list with the expected default name, which is
  editable in the shell before create.
- Existing lyrics fetch/paste/sections behaviour and its testids survive.
- Testids for the unified affordances; e2e covering each of the three entries end-to-end.
- Post-create behaviour per §4: from-scratch opens the editor; upload and from-lyrics do not; all
  three append a visible row. Empty body creatable from-scratch, still blocked from-lyrics (§5).
  Upload's name field pre-fills on selection but never overwrites typed input (§6). Cancel from any
  entry leaves the pool unchanged (§7).
- **Blast-radius guard — this is the criterion that matters.** This task restructures exactly the
  affordances that **14 e2e specs plus `walkthrough.spec.ts`** currently reach for
  (`new-text-chart`, `new-lyrics-chart`, `file-upload-form`, `lyrics-*`). T78 retired one testid and
  broke five specs, so before presenting:
  1. run the **dangling-testid sweep** — every `data-testid` removed from `src` must have no
     surviving reference in `web/studio/e2e` or `web/studio/walkthrough`;
  2. run the **full** `make e2e`, not a subset (it needs :8080 free — the port friction is the
     separate follow-up below);
  3. repoint `walkthrough.spec.ts` too, not just the e2e specs.
  Prefer **keeping the existing testids attached to the equivalent new elements** over renaming
  them: the cheapest way to pass this criterion is not to churn them at all.
- `tsc -b studio` clean; `make e2e` green (full suite, count reported).
- Before/after screenshots of the Files header + each dialog in the handoff.

## Out of scope

- The naming/extension work (landed in **T79**) and the list/row presentation (landed in **T78**).
- The matrix view (parked) and per-member `my-files`.
- Making the e2e port configurable (a separate infra follow-up flagged at the T79 gate — the
  hardcoded `:8080` / `reuseExistingServer:false` blocks running e2e while a local preview holds the
  port).
