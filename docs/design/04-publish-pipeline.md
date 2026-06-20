# Design: Publish pipeline

Derives from **I4, I11**. Two revision worlds: *live editing* vs *published performance*.

```
Song            live, continuously edited in TroubaStudio (objects/events, realtime)
  │  pins; sync is song-by-song (people edit different songs concurrently)
  ▼
Setlist         authoring/staging: an ordered set of songs, each PINNED to a revision
  • FROZEN mode = hold pins + mute "song updated" notifications
  │
  │  admin explicitly BAKES   ←── the publish gate (I11)
  ▼
TroubaStage     immutable concert bundle (flattened images, 05-doc)
  • the bake MINTS a new TroubaStage revision (NOT every edit)
  • performers see "new version of Concert A" only when an admin bakes
```

## The bake is the only publisher (I11)
Edits never auto-publish. The bake **snapshots the setlist's current composition** (its pinned/held
song revisions). So the workflow is deterministic:

> curate setlist → freeze → review → **bake** → that exact staged state is the release.

## Three freeze/control points — they stack, they don't conflict
1. **Setlist freeze** (authoring) — pin song revisions, mute song-edit notifications.
2. **The bake** (publish gate) — admin decides *when* a performable revision exists at all.
3. **Presenter freeze** (device, 05-doc) — local pin ("keep what I rehearsed") + band-wide admin
   lock.

Net: edits flow continuously into songs, but performers only ever receive **stable, admin-cut
releases**, and can freeze even those for a show.

## Bake = role/scope resolution moves server-side
The bake resolves *what this performer sees* (PERSONAL + shared + promoted, per
ADMIN/PERFORMER/CONDUCTOR rules) and renders it. All access-control logic lives here, **not** on the
device (I12).
