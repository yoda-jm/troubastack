# T150 — A band folder is the same band every time it is imported

**Lane:** web-core (core). **Size:** M. **Status:** CORE LANDED 2026-09-06 (web-core) @ `27adcc18` — at the
gate for re-verification. Re-import now UPDATES a band instead of minting a twin, via an UPSERT model (safe:
every repo `Create*` is overwrite-by-id, so no destructive deletes). Declared `id` on band.json +
setlists.json (parsed on import, emitted on export = the write-back going forward); deterministic child ids
(song = `uuidV5(bandID,"song:"+slug)`, file = `uuidV5(songID,"file:"+name)`, item = `uuidV5(setlistID,pos)`;
new `uuidV5` helper); `resolveBandIdentity` = declared-id → shortname → unique-name adoption, caller-scoped,
refusing ambiguous name matches (⟨R1⟩). Cross-user sharing preserved (a foreign-owned id ⇒ fresh band, never
an overwrite). RED-first, all ⟨R1⟩ assertions + uuidV5 + annotation-reimport idempotence green; full suite
green; no mirror drift. **Descoped to a follow-up (flagged at the gate):** the on-disk folder write-back
MIGRATION for VLL's PRE-EXISTING id-less folders (export writes the id in going forward, but old folders on
disk need a one-time pass to gain from-scratch stability); and upsert leaves a child REMOVED from the folder
lingering (non-destructive). Original spec below.

## What VLL hit

*"Je pense que les ids de playlist et de groupe changent a chaque re contribution, c'est normal ? c'est
embetant dans mon cas de dev car mes bakes arrivent systematiquement dans des nouveaux groupes avec le
meme nom."*

He is right, and it is not normal. **`app.Band` is `{ID, Name, OwnerID, CreatedAt}`** — it carries nothing
that ties it to the folder it came from, and the import path never looks for an existing band. So every
re-seed mints a new UUID, and T143's by-band accordion — which groups on `bandId`, correctly — shows two
sections with the same name.

**This is T139 one level up.** There we ruled *a slug is stored, not derived*, because regenerating an
identifier silently breaks everything pointing at it. Bands and setlists still have the disease: their
identity is **minted per import** instead of **declared by the folder**.

## The rule

**The folder's `shortname` is the band's durable identity.** It is already the thing a human uses
(`make band=<shortname>`), it is already unique in the library, and it already lives in `band.json`.

- **Store `Shortname` on `Band`**, unique per server.
- **Import resolves by shortname**: an existing band with that shortname is **updated**, never duplicated.
  Its `ID` — and therefore every bake, annotation and membership pointing at it — survives.
- **Setlists need the same treatment.** A setlist declared in `setlists.json` must have a stable key so a
  re-import updates the existing one instead of adding a twin. Use its declared name if there is no id;
  say which, and make it explicit in the folder format rather than implicit in the importer.

## ⟨D2⟩ VLL, same conversation: it must survive a from-scratch re-seed too

*"et aussi quand on reinsere from scratch ce serait cool de garder le meme ID non ?"*

**Yes — and this changes the design, so read it before implementing the section above.** Lookup-by-shortname
makes an import idempotent *within one server*. A from-scratch re-seed has an empty store and nothing to
look up, so the id must not be **found** — it must be **declared by the folder**.

Which is this repo's own standing ruling, applied one level up: **an identity is stored, not derived**
(T139). I specified only half of it.

### The shape

- **`band.json` carries an explicit `id`** — a UUID minted once when the folder is created, then never
  changed. The importer uses it **verbatim**. Two seeds into two empty stores produce the same band id.
- **Each entry in `setlists.json` carries an explicit `id`**, same rule.
- **Songs need no new field.** T139 already gave them a declared, unique-per-band `slug`, so derive the
  song id deterministically as **UUIDv5(band id, slug)**. Namespacing on the band's *declared* id — not on
  the shortname — means two bands that happen to share a shortname on different servers cannot collide.
- Keep the shortname lookup from the section above as a **fallback** for folders that predate `id`, and as
  the uniqueness check.

### The migration this makes possible, which is better than adoption

When adopting an existing band, **write its CURRENT id back into `band.json`** rather than minting a new
one. Then:

- everything already on a device keeps matching — **VLL's existing tablet bundles stay valid**, which the
  shortname-only design could not deliver;
- the folder becomes the durable record from that moment on;
- and the change is visible in his library as a normal file edit, not a hidden server-side mapping.

Do the same for setlists and for the song slugs already stored. **Say how many ids were written back.**

### ⟨R1⟩ additions

- **Two seeds into two EMPTY stores produce identical band, setlist and song ids.** This is the assertion
  that ⟨D2⟩ exists for, and no amount of lookup logic can pass it.
- A folder with no `id` still imports (fallback), and adoption **writes one in**.
- Changing a band's `shortname` does **not** change its id — that is the whole point of declaring it.

## Migration, which is the part that needs care

Existing bands have **no** shortname, so the first import after this lands cannot match on it.

- Adopt by **name** exactly once: if a band with no shortname has the same name, claim it and write the
  shortname in. State the count.
- If two bands share that name — **which is exactly VLL's current server** — do **not** guess. Report both
  and leave them; a wrong adoption merges two histories and cannot be undone.
- Never create a second band when adoption is ambiguous. Failing loudly is correct here.

## ⟨R1⟩ Red first

- **Import the same folder twice ⇒ one band, and the SAME `ID`.** Today this is two bands. Assert the id,
  not the count alone: a test that only counts would pass an implementation that deletes and recreates.
- Same for the setlist: two imports, one setlist, same id.
- A bake made before the second import **still resolves to its band** afterwards.
- Two same-named bands with no shortname ⇒ adoption **refuses and reports**, creating nothing.
- **Teeth-check:** revert the lookup and confirm the id-stability assertions go red, not just the counts.

## What this does NOT fix, and VLL should hear it plainly

Bundles already on his tablet point at the **old** band ids. After this lands they will still show as their
own group — the fix stops new duplicates, it does not retroactively merge the ones already made. Deleting
those stale bundles from the device is the clean way out, which is what T143's ⋮ is for.

## Out of scope

Cross-server band identity (a band that exists on two servers). This task is about one server importing one
folder repeatedly.
