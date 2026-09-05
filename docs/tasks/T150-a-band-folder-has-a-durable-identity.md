# T150 — A band folder is the same band every time it is imported

**Lane:** web-core (core). **Size:** M. **Status:** spec, 2026-09-05, from VLL.

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
