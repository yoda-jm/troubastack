# T129 — get the live data out of the source tree

**Lane:** web-core (`core/cmd/seed`, `core/internal/config`, `Makefile`, `docs/`). **Size:** S/M.
**Status:** **CODE LANDED** — 3 commit(s), `93c007f6`…`889dec47` (last 2026-09-03). This line previously said "spec, not started"; it is a SECONDARY copy of a fact the review gate owns (`docs/handoff/reviews.md`), and it rotted. Corrected 2026-09-04 from the git history. **Not re-verified against this spec's own done-when** — "code landed" is what was checked, not "every criterion met".
**Asked by:** VLL, 2026-09-03, after noticing runtime directories scattered through his tree.

## The finding: 822 MB of irreplaceable data lives inside a git worktree

Measured on the primary checkout:

| path | size | what it is |
|---|---|---|
| `bands/` | **822 MB** | real bands' sheet music, lyrics and manifests — **irreplaceable** |
| `core/troubadata/` | 3.7 MB | the demo server's live state |
| `core/troubadata-walkthrough/` | 1.5 MB | the walkthrough recording's state |
| `core/troubadata-<shortname>/` | 280 KB | a local band's seeded state |

All four are gitignored (`.gitignore:12-14, 69`), so **a branch switch does not touch them** — that
part is safe and always was. The hazard is narrower and sharper: **`git clean -xdf` removes ignored
files, and these are ignored files sitting in a worktree.** One cleanup command, one agent tidying a
tree, and the band's library is gone. There is no backup of `bands/` anywhere on this machine.

This is not a tidiness task. Tidiness is the *reason it was noticed*; the reason to do it is that a
routine git command can destroy data that cannot be recreated.

**The pattern is already set.** The live server was moved to `~/dev/git/troubastack-demo/`
(outside any worktree) on 2026-09-03 and works there unchanged, because `start.sh` is `$PWD`-relative.
This task finishes the same move for the rest.

## Why it is small: the machinery already exists

Nothing new has to be invented — only the **defaults** change.

- `TROUBA_DATA_DIR` already drives every data dir (`core/internal/config`). The Docker image sets it
  to `/data`; the moved server sets it absolutely. Only `make demo` / `make band=` still point inside
  the tree (`Makefile:115,126,141`).
- `TROUBA_BANDS_DIR` already overrides the bands folder. The default is the search in
  `localBandsDir()` (`core/cmd/seed/main.go:396`): `../bands`, `bands`, `../../bands`,
  `../../../bands` — **relative to the process's cwd**, which is exactly what plants it in the tree.

## Work

1. **Define one runtime root**, resolved once: `TROUBA_HOME`, defaulting to
   `${XDG_DATA_HOME:-$HOME/.local/share}/troubastack`. Every runtime path derives from it unless
   individually overridden. Document it beside the existing `TROUBA_*` table.
2. **`localBandsDir()` looks there first.** Keep the existing cwd-relative candidates **after** it, so
   an existing checkout keeps working, and keep `TROUBA_BANDS_DIR` winning outright. Same shape as
   T128's resolver, and for the same reason — do not break what works today, just stop *defaulting*
   into the tree.
3. **`Makefile`**: `make demo` and `make band=<shortname>` write under the runtime root. **Update the
   reset hints in the same edit** — `Makefile:24,111,125,133` all tell the reader
   `rm -rf core/troubadata…`, and a stale reset instruction pointed at the wrong path is its own
   small disaster.
4. **Migrate what exists**, with `git mv`-style care:
   - **`mv`, never copy-then-delete.** A copy that half-fails followed by a delete is how data is
     lost; a `mv` on the same filesystem is atomic per entry.
   - **Verify before removing anything**: compare file counts and total bytes, and re-run
     `go run ./cmd/seed -band <shortname>` against a throwaway server to prove the folder still
     seeds. Only then consider the old path gone. (This is the verify-before-delete rule; it has been
     broken here before.)
   - The live server at `~/dev/git/troubastack-demo/` already points at `bands/` by absolute path —
     **its `start.sh` must be updated in the same change**, or the band server silently seeds from a
     path that no longer exists.
5. **Keep the `.gitignore` entries after the move.** They cost nothing and they stop a re-created
   `bands/` from ever being committed. Removing them is the one edit that could turn this task into
   the leak it is meant to prevent.
6. **Document the layout** in `deploy/README.md` and the root `README.md`: source tree here, runtime
   root there, and the one sentence that matters — *nothing under the runtime root is ever
   regenerable from the repository*.

## Explicitly out of scope

- **Build outputs stay where they are.** `node_modules` must sit beside its `package.json` (Node
  resolves upward from the importing file, and this repo installs `--no-workspaces` deliberately);
  Gradle's `build/` dirs and `web/studio/dist` are consumed by the Go embed at compile time. Moving
  them fights the toolchains for directories nobody looks at.
- The bare-repo / worktree-container reorganisation. Separate decision, and **not before the concert
  on 2026-09-05** — it touches 42 registered worktrees and the server that has to work that night.
- Any change to what `bands/` *contains*. It stays gitignored, and no part of it is ever committed.

## Done when

- `rm -rf` of a fresh clone's working tree destroys **no** band data — verify by checking that
  `bands/` and `core/troubadata*` are absent from the tree after the migration, not by assuming.
- `make demo` and `make band=<shortname>` work from a clean checkout with **no** environment set, and
  write outside the source tree. Check where the files actually landed.
- `TROUBA_BANDS_DIR` and `TROUBA_DATA_DIR` still win when set — assert with a path that does not
  exist, so a silent fallback is visible.
- An existing checkout that still has `bands/` in the old place **keeps working** (the candidate list
  is ordered, not replaced). This is the compatibility arm; a change that only works on a fresh setup
  has broken every current one.
- The live band server still starts and still seeds after the move — start it and look, do not reason
  about it.
- Every `rm -rf core/troubadata…` hint in the `Makefile` and docs names the new location.
- `gofmt -l core` clean, `go vet`, `make test` green.
