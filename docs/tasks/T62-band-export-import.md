# T62 — Band export / import (one zip, everything)

**Lane:** web-core · **Size:** M/L · **Status:** SPEC'd 2026-07-25 (VLL: *"export and import a complete band, including everything, no need for versioning, you'll resolve the incompatibility in the future, a zip would be super nice"*) · **Depends on:** nothing open

## What VLL asked for

One zip that carries a **complete band** out of a server and back into one (this or
another instance): backup, migration, band-moves-servers. Explicitly **no versioning
machinery** — incompatibilities get resolved by a human in the future, not by
migration code now.

## Design decisions (resolved here — don't improvise)

1. **Format: one zip** —
   ```
   band.json          the whole relational graph, one manifest (shape below)
   blobs/<sha256>     unique file bytes, content-addressed (dedup by BlobHash)
   ```
   `band.json` starts with `"formatVersion": 1` and `"exportedAt"`. The importer
   rejects `formatVersion != 1` with a clear 400 — **that single integer is the
   entire versioning story** (it's what makes VLL's "resolve it in the future"
   possible; build NO migration logic).

2. **What's in the manifest** (all fields of each struct, by their existing JSON
   tags, except where noted):
   - `band` — Name (ID/OwnerID/CreatedAt are re-minted on import).
   - `members[]` — per member: `{username, displayName, email, avatarKind, role}`
     (User projection + Membership.Role). **Never PasswordHash** (it's `json:"-"`
     everywhere for a reason; hashes don't cross servers).
   - `songs[]` — all Song fields; nested `files[]` (SongFile metadata incl.
     `blobHash`, `generated`, `revision`, `displayOrder`) and, for generated files,
     the **chart source** (`GetChartSource`) inline.
   - `setlists[]` — all Setlist fields; nested `items[]` (song ref, `keyOverride`,
     `tempoOverride`, `notes`, `onCall`, `transposeChords`).
   - `fileSelections[]` and `songCues[]` — the per-member personal layers of the
     data (keyed by member username + song ref in the manifest).
   - `annotations` — per song, the engine **HEAD only** (`Engine.Head(songID)` →
     layers + objects, the same `annotationsJSON` shape T08's GET/import round-trips).
     Revision history is NOT exported — head-only matches the T08 precedent and the
     no-versioning spirit.
   - **NOT exported:** baked concerts (rebake on the target; if a final-locked gig
     bundle matters, download its `.tstage` before migrating — say this in the UI
     copy), invites + invite links (ephemeral, token-bearing), sessions, password
     resets.

3. **Generated charts import as their exported BYTES, not a re-render.** The stored
   PDF blob is imported as-is and the chart source is attached alongside. Rationale:
   a renderer tweak between export and import would shift pagination/geometry and
   silently unanchor every annotation on that chart (the T60 Part A invariant is
   about *transpose*, not about cross-version renderer identity). Re-render happens
   naturally on the next source edit.

4. **ID strategy: re-mint everything relational; keep annotation-internal ids.**
   Band, songs, files, setlists, items get fresh server IDs; the importer keeps a
   translation map. Annotation **layer IDs and object uuids are kept** (each song
   gets a fresh engine keyed by the new songID, so there is nothing to collide
   with — and T08 idempotency semantics key on those ids). Two rewrites are
   mandatory while applying annotations:
   - `Object.FileID` (domain.go:160, the T40 file scoping) → through the file map;
   - `Layer.OwnerID` (personal/conductor zones) → through the member map.

5. **Members: match by username, else create.** For each manifest member: if the
   username exists on the target server, that account is REUSED (membership added
   with the manifest role); otherwise an account is created with displayName /
   email / avatarKind and **no usable password** — the admin hands out credentials
   via the existing T21 reset flow. The import response reports
   `matched[]` / `created[]` explicitly so the admin sees exactly which existing
   accounts were attached. (This is the restore/migration semantics; the importing
   admin is trusted on their own server.) The **importer** always becomes an
   admin member and the band's `OwnerID`, whether or not they appear in the
   manifest — an imported band is never orphaned.

6. **Import creates a NEW band, always.** No merge-into-existing. Name collisions
   are fine (band names aren't unique).

7. **Import is all-or-nothing.** Validate the ENTIRE manifest first — formatVersion,
   every `blobHash` present in `blobs/` and matching its content hash
   (`blob.HashOf`), every item's song ref resolvable, every annotation's file ref
   resolvable — and only then start creating. Any validation failure → 400 with a
   specific error and NOTHING created. (A failed bake warns-not-fails; a failed
   import must fail-not-half-create — restoring half a band is worse than a clear
   error.)

8. **API + auth (I11-consistent):**
   - `GET /api/bands/{bandId}/export` — **admin-only** (the zip contains every
     member's email, personal cues and selections). Streams `application/zip`,
     `Content-Disposition: attachment; filename="<band-name>-<yyyy-mm-dd>.tband.zip"`.
   - `POST /api/bands/import` — any authenticated user (creating a band is open to
     any user; import is just creating a band with content). Multipart zip upload.
     Own size cap — a new `maxImportBytes = 512 << 20` const (the 32 MiB
     `maxUploadBytes` at webapi.go:642 is per-song-file and far too small for a
     band). Response: the new band + the member `matched[]`/`created[]` report +
     counts (songs/files/setlists/annotations).

9. **Studio UI (minimal):**
   - Band **Settings** tab: an "Export band (.zip)" download button (admin-only,
     same gating as the other Settings admin controls), with one line of copy
     noting baked concerts are not included.
   - **Bands** overview page: an "Import band…" file picker next to the create-band
     form → on success, navigate to the new band and show the member report
     (created accounts need a password reset — say it in the copy).

## Out of scope

- Baked concerts / bake revision history (rebake; re-lock finals manually).
- Merge into an existing band; selective/partial export; cross-server user
  identity beyond username matching.
- Any `formatVersion != 1` handling beyond the clear 400.
- Password hash export, sessions, invites, invite links.
- App/iOS work — this is server + studio only.

## Acceptance

1. **httpapi round-trip test (both backends):** build a band with 2 members
   (admin + member), a song with an uploaded PDF **and** a generated chart (with
   source), annotations on both files across 2 layers (one owned by the non-admin
   member, exercising the OwnerID rewrite), personal cues + a my-files selection
   for the member, a setlist with `keyOverride` + `transposeChords` + `onCall` +
   notes. Export → import into a FRESH service (empty repo). Assert: entity graph
   deep-equal modulo re-minted IDs and timestamps; blob bytes byte-identical (same
   `blobHash`); chart source round-trips; annotations HEAD equal modulo
   FileID rewrite; member report shows 1 matched (pre-create the member's username
   on the target) + 1 created.
2. **Auth:** non-admin `GET export` → 403. Import response's band has the importer
   as OwnerID + admin membership.
3. **All-or-nothing:** a zip with a missing blob, a corrupted blob (hash mismatch),
   and a `formatVersion: 2` manifest each → 400 AND a follow-up list shows no new
   band (nothing created).
4. **e2e (studio):** export a demo band from Settings (download completes, zip
   non-empty); import it on the Bands page → the new band appears; open the
   imported song → the chart renders and annotations are visible; the member
   report is shown. (Reviewer pixel-checks the two new surfaces light/dark.)
5. `go test ./...` + gofmt + vet green; tsc + studio build clean; no dist churn.

## Notes for the executor

- Reuse the T08 `annotationsJSON` marshaling for the annotations section and the
  import path's mutation-apply loop (`LayerCreate` + `Create` on behalf of any
  owner — admin-gated exactly like `importAnnotations`, annotations.go:135).
- **`Layer.OwnerID` rewrite exception:** `domain.SharedOwner` (`"_shared_"`,
  domain.go:89) is a synthetic sentinel, not a member — it passes through the
  member map UNCHANGED. Only real member UUIDs remap; a layer whose owner isn't in
  the manifest's member list is a validation error (decision 7), not a guess.
  (Credit: the lane's grounding memo caught the sentinel.)
- Export walks the Repo interface + `blob.Store.Get` — do NOT read the filestore
  layout from disk; the same code must work on mem and file (and any future) repos.
- `archive/zip` from stdlib; stream the response (no temp file needed for export;
  import may buffer — it's capped).
- Present at the gate as usual; cite the work order (VLL 2026-07-25, relayed via
  Fable) in the trailer.
