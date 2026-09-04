# T139 — A song's slug is STORED, not derived from its title

**Lane:** web-core (core). **Size:** S/M. **Status:** implemented 2026-09-04 (web-core) — `Slug` on
`Song`, import stores it verbatim, export emits it (lazy-derive only when empty), `CreateSong` derives
once & unique-in-band, `UpdateSong` leaves it alone, the two `slugify` copies merged to `app.Slugify`.
Migration = option 2 (existing songs keep `Slug=""` → lazy-derive at export until a re-import supplies
one; NOT derive-backfilled). Tests green; awaiting reviewer re-verify at the gate.

## The question that produced this

VLL, after I reported that a band's export and the folder that seeded it disagree on names:
*"pour les apostrophes il faudrait faire en sorte que ce soit stable, une recommandation ? le sluggage
est different dans differents endroits ?"*

**Answered by looking:** the two Go implementations — `app.slugify` and `cmd/seed.slugifySeed` — are
**character-for-character identical**; the second even says *"mirrors app.slugify"*. **There is no
code-vs-code divergence, and my earlier gate note saying "slugify disagrees" was wrong.**

## What is actually happening

`app.Song` is `{ID, BandID, Title, Artist, Key, Tempo, Meter, Tags, Notes, CreatedAt}` — **there is no
`Slug`**. So the exporter has nothing to emit and must **re-derive** the slug from the title. Meanwhile
the folder carries slugs that were **chosen**, not computed. Measured on the real library — 8 of 46
songs differ, and the differences are editorial:

| title | folder | what the rule computes |
|---|---|---|
| `L'Ete Indien` | `lete-indien` | `l-ete-indien` |
| `Ce Vieux Refrain` | `refrain` | `ce-vieux-refrain` |
| `A Long Title / With A Slash?` | `long-title` | `a-long-title-with-a-slash` |

`refrain` is **deliberately shorter than its title**. These are an author's identifiers, and a
derivation cannot reproduce them — not with a better apostrophe rule, not with any rule.

## So the fix is not a better slugify. It is: stop deriving.

**A slug is an identifier; a title is a display field.** Deriving the first from the second has two
consequences, and the second is the dangerous one:

1. The author's choice is discarded on every round-trip through the server (what VLL hit).
2. **Editing a title silently renames the identifier** — and the identifier is what
   `annotations/<slug>.json` and `setlists[].items[].song` point at. A rename today is a quiet reference
   break waiting for someone to notice.

### Required

- **Add `Slug` to `Song`**, unique within a band.
- **Import stores the folder's declared slug verbatim.** No re-derivation, no "tidying".
- **Export emits the stored slug.** The round-trip becomes stable, and `refrain` survives.
- **Derive only when creating a song that has no slug** (the Studio "new song" path) — one call site.
- **Merge the two identical `slugify` copies into one exported function.** They agree today; two copies
  of one rule is how they stop agreeing later. The apostrophe question then has exactly one answer and
  one place to change it.
- **A title edit must NOT change the slug**, and that must be stated where someone would otherwise
  "fix" the drift between them.

### Migration, which needs a decision rather than a default

Songs already on the server have no slug. Backfilling by derivation would **bake in the wrong slug for
exactly the 8 songs this task is about**. Options, in order of preference:

1. Backfill from the band folder on the next import (the folder is the authority for imported bands).
2. Leave empty and derive lazily at export, as today, until an import supplies one — correct, but the
   instability persists until then, so say so rather than let it look fixed.

**Do not backfill by deriving from titles.** That would silently overwrite the author's identifiers with
the very values this task exists to stop producing.

## Acceptance

- **Round-trip:** import a folder whose slugs are hand-chosen (include `refrain` and an apostrophe case),
  export, and the slugs come back **identical**. This is the test that fails today.
- **A title edit leaves the slug alone**, and annotation + setlist references still resolve afterwards.
- Two songs in one band cannot take the same slug.
- `slugify` exists **once**, called only on create-without-slug, with vectors covering apostrophes,
  slashes, punctuation runs and an empty result.
- The demo and `-band` seeding paths still round-trip (they exercise import → export).

## Out of scope

Which apostrophe rule to use. Once nothing re-derives an existing slug, the choice only affects newly
created songs, and any consistent answer is fine.
