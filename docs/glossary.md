# Glossary & naming

**Rebuilt 2026-09-13** from an audit of every surface — proto/contracts, the Go core, Studio (web), the
mobile app, and the task specs — 264 tasks after the scaffold version of this file. The scaffold's ten
domain terms are still true; the product has grown about sixty more, and several words now carry two or
more meanings. This file names the **canonical term**, what each surface actually calls it, and where
they disagree (§Divergences at the end, each with an owner).

Conventions: **bold** is the canonical term to use in specs, comments and UI copy. `code` is an
identifier or wire name. "Quoted" is a user-facing string. A term marked ⚠ is overloaded — say which
sense you mean.

The brand names come from the troubadours: *trobar* (Occitan, "to compose verse") is the root of
"troubadour"; a *joglar* performed what the troubadour composed. **Compose = Studio, perform = Stage.**

## 1. Products and surfaces

| Term | Is | Notes |
|---|---|---|
| **TroubaStack** | the monorepo and product family | `@troubastack/*` packages, the OCI image |
| **TroubaCore** / **Core** | the Go server (`core/`) | source of truth, sync hub, bake orchestration, blob store; binary `troubacore` |
| **TroubaStudio** / **Studio** | the web editor SPA (`web/studio`); also a section of the app via WebView | where you compose and annotate |
| **TroubaStage** / **Stage** | the offline concert presenter — the mobile app's performing surface (`app/`) | the app as shipped is called TroubaStage everywhere but this file's scaffold version, which said *TroubaShare*; see D1 |
| **ink** (`web/ink`, `@troubastack/ink`) | the ONE stroke renderer + glyph geometry package | consumed by Studio (dry), the bake worker, and the app (geometry only, via the generated mirror) |
| **bake worker** (`web/bake`, `troubabake`) | the Node process Core shells out to render overlays | also called "the overlay worker"; Core draws nothing itself |
| **presenter** | Stage's compositor + pager | "pure image compositor + pager"; no annotation-model logic on the tablet (I12) |
| ⚠ **stage** (lowercase) | (a) the product above; (b) a bake phase (`stageSong`, "stage 1b"); (c) a Docker build stage; (d) a P206 delivery stage ("Stage 3 — bake") | write "Stage" for the product only |

## 2. Band and people

| Term | Meaning | Surface names |
|---|---|---|
| **band** | the trusted unit of collaboration and the privacy boundary; every member receives all band data | `bandId` everywhere on main; legacy `group_id` on `Song`/`Setlist` in proto (D2) |
| **member** | a person in a band | `MemberView` (Studio), `Membership` (Core), `BundleMember` / **roster** (bundle); `userId` in `/api`, `memberId`/`owner` in the bundle and layer model |
| ⚠ **role** | three distinct things: (a) **membership role** `admin \| conductor \| member` (permissions); (b) **part tag** `Layer.role_tag` (free text: what you play; drives default visibility); (c) **reading role** on Stage ("My role … a reading preference, not a login") | say *membership role*, *part tag*, or *reading role* |
| ⚠ **identity** | (a) **Stage identity** = the roster member the tablet performs as (`StageState.identity`, "Who are you?", "Performing as …"); (b) **account** = the server login (`home.Identity`: Connected / Guest / Offline); (c) a stable band+title key in `annrecover` | never use bare "identity" in copy |
| **taken as** | the Stage identity a rehearsal note was drawn under (`NoteEntry.takenAs`, `RehearsalNoteMeta.TakenAs`); the server never maps it back to an account — **the sender is the owner, always** | shown as "taken as X" (D22: raw id today) |
| **owner** | of a layer/object: `OwnerID` (`_shared_` for shared layers in the domain, `""` in the baked `LayerImage`, see D14); of a rehearsal note: the caller, implicit | |
| **Band / Mine** | the two audiences of any change: 👥 "Shared with the band" vs 👤 "Just for you" | `Audience = band \| mine`; derived from zone (personal → Mine, else Band) |

## 3. Repertoire

| Term | Meaning | Surface names |
|---|---|---|
| **song** | a titled repertoire entry with one linear annotation history and a **file pool** | `app.Song` (relational) and `domain.Song` (engine) share the id; `Slug` is the stored identity, title is display |
| **song file** / **pool** | one PDF, image, or generated chart attached to a song; the set is the pool | `SongFile`, `fileId`; UI "Files"; CSS `.part`; **"part"** is the musician-facing word in jump copy only (D5) |
| **my files** / **stage selection** | a member's ordered subset of the pool, what Stage shows them | `MyFiles`, `my-files` route, "In my stage selection" |
| **default file** | the ONE file a member gets when they have not chosen (T138 rule) | `DefaultFile`; two Core handler comments still say "all pool files" (D15) |
| **text chart** / **chart** | a plain-text lyrics+chords source rendered server-side into a PDF that joins the pool | `chartpdf`, `chart-source`, directives `np/fn/sot/eot`; UI "Edit chart" |
| ⚠ **source** | (a) the chart TEXT source (`chart-source`, `SourceAnchor`, `sourceRevision`); (b) the jump-mark end that carries `jump_to` (§5) | |
| ⚠ **anchor** | (a) `SourceAnchor` on an object: which text run a mark is attached to (marks follow their words); (b) `chartpdf.Anchor`: the render-side text-run box ("anchor manifest"); (c) "retained anchors" as a GC root | |
| ⚠ **revision** | (a) a point on a song's annotation history (`domain.Revision`, `head`); (b) a chart re-render counter (`SongFile.revision`); (c) the concert **rev** (§7). Four numbered things share the word (D8) | |
| **head** | the latest annotation revision of a song, always retained (I7); also the engine's in-memory live set | `Song.head`, `Engine.Head()` |

## 4. Annotation model

| Term | Meaning | Surface names |
|---|---|---|
| ⚠ **annotation** | (a) the whole of a song's layers + objects (`AnnotationDoc`, `/annotations`); (b) one drawn mark, in UI copy | code says *object*, UI says *annotation*; the Studio drawer tab is "Annotations" (was "Notes"; D21) |
| **object** | one drawn mark with a client UUID: `freehand \| line \| rect \| ellipse \| text \| highlight (legacy) \| icon`; per-object LWW; `order` = z within its layer | `AnnotationObject` (TS), `domain.Object` (Go) |
| **layer** | an ordered, owned stack of objects on ONE file, in a zone | `Layer{file_id, owner, zone, order, access, mandatory, role_tag}` |
| ⚠ **layer** (other senses) | repo tiers ("three layers"), storage tier (I7 "referenced by any layer"), the wet/dry surfaces, the raster+overlay pair, Stage's `~notes` reserved id | say *annotation layer* when in doubt |
| **zone** | `conductor \| shared \| personal` — fixed z-bands: PDF < conductor < shared < personal, mine on top | UI shows the *audience* (Band/Mine) and "(conductor)", never the word zone |
| **access** / **mandatory** | RW = any member edits, RO = owner only; mandatory = viewers cannot hide it | UI "required" for `mandatory`; "Read-only layer" |
| **layer visibility** | a per-viewer presentation preference, never a mutation; seeded by (reading role, identity); `default_on` captured at bake | Stage: "Layers — song" dialog, per-song override |
| ⚠ **stroke** | (a) a freehand path; (b) `InkStyle.stroke`, the shape-border flag; (c) "Stroke width" = line width | |
| **freehand** | the point-list object type; the only type the native wet layer renders (I9) | tool label "Pen" |
| **icon** / **stamp** | an object placing a glyph; `glyph id` rides in `Object.text` | tool "Icon", palette "Stamp icon"; jump landmarks are icons too (§5) |
| **glyph** | curated polyline geometry in `glyphs.json`, generated by `gen-glyphs.mjs` into TS and the Kotlin mirror `CueGlyphData.kt` | ⚠ also: the blocked-turn UI marker, the FAB label, the ♩ tempo symbol |
| **glyph kind** | `cue \| landmark` — a partition written by the generator; pickers filter on it, "not a hand list"; only `kind=cue` reaches the app (**cue-only** mirror) | |
| ⚠ **cue** | (a) **song cue**: a member's icon+tint reminder per song, flashed on song entry (`SongCue`, "My cues"); (b) marks on a conductor layer ("Conductor cues"); (c) transient Stage hints (**boundary cue**, **blocked turn**); (d) the `kind=cue` glyph class | |
| **wet** | the in-progress stroke, before commit: a second canvas in Studio (`EditCanvas` in `WetCanvas.tsx`), a native overlay on the tablet | ⚠ file name vs export name (D16) |
| **dry** | all committed objects, painted by ink onto the overlay canvas; the bake reuses the same renderer | ⚠ `dryRun` (transpose) is unrelated |
| ⚠ **overlay** | (a) Studio's dry canvas per page (`.annotation-overlay`); (b) the bake's transparent PNG per (page, layer) (`LayerImage`); (c) the DOM selection overlay; (d) on Stage, anything floating over the score (clock, chrome) | |
| **EditCanvas** | the topmost per-page pointer/wet canvas in Studio | stack: `pdf-canvas` → `rehearsal-underlay` → `annotation-overlay` → `edit-canvas` |
| **tombstone** / **restore** | a delete is terminal and wins LWW (`deleted-remotely`); only an explicit `restore` revives a UUID | ⚠ restore also = chart-draft restore, ops restore, T159 recovery |
| **mutation** / **seq** / **echo** | one action in history; server-assigned total order; the optimistic local apply is confirmed by the server echo by uuid | `reject` reasons: `deleted-remotely \| stale \| forbidden \| jump-duplicate` |
| **live** | ⚠ (a) the realtime sync connection ("LIVE" chip, "Live annotation count"); (b) **rehearsal live mode** = the autobake window (P201: "Go live" / "Arm live mode", D6) | |

## 5. Jump marks (P206)

| Term | Meaning | Surface names |
|---|---|---|
| **jump mark** | a PAIR of placed icon **landmarks** with the same glyph and colour in one file; the **source** carries `jump_to` = the **destination**'s uuid — no page, no coordinate ("nothing to drift"). Musician's word, not "hyperlink" | tool "Jump mark"; code `jump`, `jumpTo`; VLL's quotes say "jumpmark" |
| **landmark** | one end of a jump: a glyph of `kind=landmark` (`segno`, `coda`, circle, square, triangle, diamond, star) | palette shows raw ids (D11) |
| **source** / **destination** | authored roles: source = the end carrying `jump_to`; placed FIRST (⟨D3⟩) | baked side says **target** (`target_page`, `target_anchor_y_permille`) — D9 |
| **relationship label** | the selected end states the relationship, not a role noun: "Jumps to p.7" / "Jumped to from p.5" / "…the other mark"; the label is the go-there button (⟨D4⟩) | step chip still says "next: source/destination" (D10) |
| **the other end** / partner | the counterpart of the selected end | code `partner`; comments "counterpart", "pair" |
| **tie** | the dashed segment + arrowhead drawn source→destination when both ends are co-visible | CSS `.jump-segment`, `.jump-arrow`; site says "dashed arrow" |
| **cross-page hint** | a small "→ p.N" / "← p.N" button on an end whose partner is on another page of the same file; Stage deliberately shows no page numbers | `JumpPageHint` |
| **uniqueness** | one (glyph, colour) per file: two identical landmarks never coexist; refused client-side (palette disables taken ids) AND server-side (`jump-duplicate`) — a legibility rule, not integrity | |
| **swap** | move `jump_to` to the other end; the relationship sentence flips | "Swap jump direction" |
| **broken** (Studio) | a source whose destination is not in THIS file: deleted, on another part, or self-pointing; marked red, refuses to navigate, never guesses | `brokenJumpUuids`, `JumpFlags` |
| **dangling** (bake) | the same condition seen by the bake: a documented INPUT population (pre-sweep songs, survivors on a layer the deleter could not edit); the jump is DROPPED with a warning naming the song, ink untouched, bake succeeds | `resolveJumps` |
| **delete sweep** | when a landmark is deleted, its partner's `jump_to` is cleared in the same atomic mutation, so the surviving end is a plain icon — bounded by layer access (hence the dangling population) | |
| **PageJump** | the baked form: source hotspot rect (permille) + `target_page` (0-based, song-local) + `target_anchor_y_permille` + `layer_id`/`owner` for visibility filtering | `PageImages.jumps` |
| **page-index frame** | the bake resolves within a FILE, then adds the file's first index in the song's page list; the wire frame is song-local; Stage maps song-local to its flat page index | traced correct at the gate; the proto comment says "song", the bake comment says "file" — both true (D12) |
| **Go** / **jump direct** | on Stage a tap inside a hotspot opens the "Go" popup (the whole tooltip is the action) or, with "Skip the go-to popup", jumps at once; landing puts the anchor near the top with a lead-in | `pendingJump`, `jumpDirect` |
| **Return** | the come-back affordance — **STRUCK** by decision 3; kept in P206 §4.5 only as the record of why | do not build |

## 6. Setlists and concerts

| Term | Meaning | Surface names |
|---|---|---|
| **setlist** | an ordered program of songs, each **pinned** to a revision; bakes to ONE concert | `Setlist`, `SetlistItem{position, onCall, kind, label, transposeChords}` — `SetlistItem` has no proto message (acknowledged I1 divergence) |
| **running order** | the numbered main-order songs, excluding on-call and intermissions; the numbering rule is a shared contract vector | `runningorder` (Go), `runningOrder.ts`, `runningOrderNumbers()` (Kotlin) |
| **on call** | a song baked and jumpable but outside the running order, unnumbered | wire `onCall`; Studio "Bench · on call", "To bench"; Stage drawer header "On call"; code/comments "bench", "encore" |
| **intermission** | a break entry: one separator page, no overlays, no number | `kind = intermission`; Studio tooltip says "break" |
| ⚠ **pin** | (a) **setlist pin** = a setlist's reference to a song revision, a GC root (I7); (b) **device pin** = a performer freezing their downloaded rev ("Pin this version", `LocalPin`); (c) a test "pins" a rule | |
| **frozen** / **freeze** / **admin lock** | three stacking control points: setlist freeze (holds pins), the bake, and the presenter freeze (device pin + `final_locked`) | Stage: "Freeze (no updates)" |
| **concert** | a baked setlist as performed; `concertId == setlistId` for the band-wide bake | Studio's Setlists page says "New concert" for an unbaked setlist (D3) |
| **variant** | the per-member concert `<setlistId>~<userId>` (B07) — declared **retired**, parser still live (D17) | `?scope=mine`, "Bake my parts" |
| **rehearsal live mode** | a window during which every edit autobakes and Stage may auto-update transiently (P201) | "Go live (rehearsal)" vs "Arm live mode" (D6) |
| **offer** | server-side availability surfaced in the concerts list ("New", "update to rev N"), applied only on tap | |

## 7. Bake and bundle

| Term | Meaning | Surface names |
|---|---|---|
| **bake** | the explicit publish gate that flattens a setlist into a bundle (I11); manual by default; a noun for one produced version | `POST …/setlists/{id}/bake`; `bakeId` (progress key) ≠ `concertId`/`rev` (output key) |
| **re-bake** | bake again from a list row (Studio) or the ⋮ menu (app) | |
| **rev** (concert rev) | the monotonically bumped bake number per concert; what performers compare | `concertRev`, `currentRev`; UI "rev N" and "version" (D8) |
| **bundle** | the self-contained flattened concert: `bundle.json` **manifest** + `blobs/`, zipped as **`.tstage`** | app UI says "concert"; only the delete dialog says "bundle" |
| ⚠ **manifest** | (a) `bundle.json`; (b) the server's what's-available response | |
| **raster** | the baked page image, black ink on white, no scheme variant; **grey pages** are stored single-channel since T169 (bundle roughly halved) | `pageRasterRef`; proto and design docs do not record the channel change (D13) |
| **raster hash** / **content hash** | sha256 of the raster bytes / of an overlay PNG: dedups blobs, drives the viewport-preserving swap on update, and is the key rehearsal notes hang on | `rasterHash` vs `contentHash` vs cache `rasterVer` — three names, one idea |
| **overlay image** | one transparent PNG per (page, layer) | `LayerImage{layerId, owner, contentHash, defaultOn}` |
| **member pages** | per-member index sequences into the song's shared page pool | `MemberPages.page[]` |
| **permille** | the bundle-side integer stand-in for I3's [0,1] coordinates (codegen constraint); `content_bottom_permille`, `PageJump.*_permille` | no constitutional note yet (D13) |
| **content bottom** | how far drawn content reaches on a page; scroll mode stops there on a song's last page | |
| ⚠ **blob** | content-addressed bytes in three places: the app blob store (`<data>/blobs/<sha256>`), the file store, and a bundle's `blobs/` dir; plus git blobs | |
| **render cache** | content-keyed cache of rasters and overlays | `TROUBA_RENDER_CACHE`, `purge-render-cache` |
| ⚠ **GC** / prune | (a) store-level reachability GC (tiers, pins as roots); (b) the `gc` subcommand, which prunes only bake outputs (`keep_revs`); (c) `deleteSongCascade` | |
| **orphan** (Core sense) | an annotation whose text run vanished after re-render (T145 reflow orphan); unreferenced blobs — distinct from the rehearsal-note orphan (§9) | |

## 8. Stage (presenter) UX

| Term | Meaning | Surface names |
|---|---|---|
| **reading mode** | **Page \| Width \| Scroll**: whole page fitted / fit width, scrolls within the page / one continuous column | code `FitMode {FIT_PAGE, FIT_WIDTH, SCROLL}`; README says "fit modes" (D4) |
| **two-up** | landscape spread in Page mode | also "facing pages", "spread"; never in UI |
| **chrome** | the overlaid controls: FABs, title card, meta strip, sheets, drawer; any tap toggles it, never navigates (except inside a jump hotspot, N3) | |
| ⚠ **drawer** | (a) Stage's **song drawer**: the only way to jump to a song, running order + On call; (b) Studio's right-hand panel (layers/annotations) | |
| ⚠ **picker** | Stage: the "Who are you?" identity picker, the concerts list, the file picker; Studio: cue, glyph, key, avatar pickers | say which |
| **swipe lock** | disables the horizontal song-crossing swipe in Scroll mode; ‹ › and pedal still work | "Lock swipe" |
| **finger-follow** | the pager drag following the finger in Scroll mode (N10) | comments only |
| **colour mode** | the draw-time reading palette **Normal \| Warm \| Night \| Amber**; Night is the inversion | `StageColorMode`; UI "Colour" / "Colour mode"; README "paper"; **theme** (Light/Dark/System) is separate |
| **Parameters** | the settings screen | code `SettingsScreen` (D19) |
| **library** | the on-device concerts grouped by band | UI "Concerts" heading, "Bakes" tab, "N on device" (D18) |
| **auto-update** / **update notice** | transient rehearsal polling that swaps in a new rev without moving the page; "Updated to rev N" | Home "Update" is a different action |
| **chrono** / **clock** | session timer vs time-of-day overlay | |
| **beat** / **count-in** | the visual beat driven by baked tempo/meter; `CountIn.kt` holds beat-phase math | |
| **meta strip** | the first-page line "notes · key · ♩=tempo" from setlist metadata (A08) — these "notes" are setlist text, not rehearsal notes | |
| **title card** / **boundary cue** / **blocked turn** | transient song-entry card; edge-of-concert hints | |
| **pedal** | a Bluetooth page-turner presenting as a keyboard | |
| **FAB** | the round Stage buttons ☰ ⚙ ‹ › ✎ ✕ | |
| **cue flash** | the member's song cue shown on song entry | |

## 9. Rehearsal notes (A70, A71, T170)

| Term | Meaning | Surface names |
|---|---|---|
| **rehearsal note** | one per-page, per-device transparent bitmap drawn on Stage, keyed by `(songId, rasterHash)`, never in a bundle; the only write Stage makes (the I12 carve-out). **Pure bitmap, deliberately not an annotation**: it cannot follow a lyric change, cannot be shared, and nags — so you recopy it into a real annotation while it still means something | `RehearsalNotes` port, `NoteEntry`, `NoteKey`; UI "Rehearsal notes" |
| ⚠ **note** | (a) rehearsal note; (b) setlist/song free-text `notes`; (c) glyph id `note`; (d) the ♩ tempo unit. The Studio annotations tab no longer says "Notes" | |
| **note mode** | the touch-owning drawing state on the current page, entered from the ✎ FAB behind a confirmation ("Touch will draw, not turn pages"), left with **Done**; refused in Scroll mode | refusal string says "page mode" (D20) |
| **note layer** | the note bitmap composited above every baked overlay; toggles like a layer ("Rehearsal notes · this device") but is not a `LayerInfo`; reserved bake-layer id `~notes` is a loader guard only | `NoteLayer` composable |
| **note pad** | the per-page drawing context threaded host → page (`NotePad`), plus the finger reader (`StrokeReader`, A71: no slop, touchdown is the first point) | not a composable |
| **note bar** | the bottom strip in note mode: pencil ✎, eraser ⌫, three widths, four colours, "Erase note", "Done" | ⚠ "Erase note" deletes the whole note; ⌫ is the eraser tool (D23) |
| **orphan** (note sense) | a stored note whose `(songId, rasterHash)` no longer exists in the bundle after a re-bake; kept, never re-placed, counted in the update notice | UI never says "orphan" |
| **Notes ⚠** / **bakes since touched** | the nag: an unsent note ages by one per bake; orange at 3, red at 6 | |
| **sent** / **Send to Studio** | `sentAt` is set by the tablet on its own successful send, never derived from the server; the button is a disabled placeholder until the §6 mobile slice lands | |
| **underlay** (Studio) | the note `<img>` printed between `pdf-canvas` and `annotation-overlay` as a reference to recopy from by hand; non-hit-testable, never an object, never baked, never visible to another member | `RehearsalUnderlay`, chip "Rehearsal notes (N)" |
| **pageChanged** | server-derived, three-valued: the note's raster hash is / is not / cannot be checked against the current bake; UI states the fact "not in the current bake", never a cause | |
| **Done, remove** | delete the note in Studio for this owner; the tablet keeps its copy | `deleteRehearsalNote` |
| **overwrite** | a PUT for an existing (owner, song, page) is 409 unless `?overwrite=1`; a single send prompts on the 409; bulk does NOT ask — it counts them *"N need overwrite"*, writes no `sentAt`, and leaves each retryable individually (settled 2026-09-14) | |

## 10. Process vocabulary (the gate)

| Term | Meaning |
|---|---|
| **gate** | `docs/handoff/reviews.md`: a submission is an entry there; a branch push plus a message is not presenting |
| **lane** | a worker role: web-core (T/B/OPS tracks), mobile (A track); the **architect** (Fable) specs, reviews, rules |
| **task prefixes** | **P** platform/cross-cutting (P201–P206), **T** web-core/core tasks, **A** app track, **B** bake·publish·distribute, **N** Stage navigation items, **CFG**/**OPS** config and ops; **⟨D n⟩** inside a spec = a dated decision/respec; **I n** = an architectural invariant in `ARCHITECTURE.md`; **R n** = a numbered risk in `risks.md` or a requirement inside a ⟨D⟩ |
| **RED FIRST** | every acceptance row must be shown failing before the fix |
| **positive control** | a guard or probe must prove it saw what it was supposed to scan; an empty offender list from the wrong directory is not evidence |
| **source guard** | a unit test that reads source files to forbid an import or a write path (e.g. nothing in `web/ink`, `web/bake`, `proto/`; Stage reaches I/O only through the notes port) |
| **seam** | the boundary a pure test cannot see (a Compose modifier, a browser viewport); a seam is proven on the device or in e2e, not by a pure test |
| **discriminating vector** | a test input that the naive-wrong implementation fails and the correct one passes |
| **device pass** | a gate row executed on the tablet by a human and reported as seen |
| **placeholder column** | any per-song / per-concert / per-member table names his real repertoire by construction, so the identifier column is written as `song A/B/C`, `<lyric run>`, `band A/B` **from the start** — not redacted after review. The repo is public and `docs/handoff/` ships. Whoever *commissions* such a table states this in the same sentence as the request; evidence never depends on the real name. **And grep the diff before the push** — the discipline while writing attaches to the word "report" and a later answer arrives shaped as "evidence", which is how a lane that knew the rule walked past it (2026-09-15) |
| **instrument, not reasoning** | when a measurement surprises, suspect the probe before the conclusion. Three times in the T172/T174 family the instrument lied: a synthetic fixture with a gap real charts lack, 11 pt marks measured on a 13 pt render, and a filter (`Y1 <= cy`) that excluded by construction the very rows being asked about. **Reconcile two measurements of one population individually** — a matching total hides which members moved |

## Divergences — where the surfaces disagree today

Ordered roughly by how often a reader will trip on them. "Owner" is who fixes it; "VLL" means a product word that needs his ruling, not a lane's.

| # | Term | What disagrees | Suggested resolution | Owner |
|---|---|---|---|---|
| D1 | TroubaShare | scaffold glossary (this file, before today) and one design doc call the app TroubaShare; README, the Dockerfile (`troubashare.apk`), ARCHITECTURE and all UI say TroubaStage | retire TroubaShare; rename the APK artefact | VLL, then web-core |
| D2 | band vs group | proto `group_id` on Song/Setlist, `band_id` on the bundle; `domain.Song.GroupID`; USER-JOURNEY says "groups" | proto is I1: keep the wire names, add a comment; fix the prose | web-core |
| D3 | concert vs setlist | Studio's Setlists page: heading "Setlists", buttons "New concert" / "Filter concerts"; Core comment "listSetlists returns the band's concerts"; app ⋮ says "Setlist <id>" | concert = baked setlist, only; unbaked is a setlist | VLL |
| D4 | reading mode | contract and app UI say Page / Width / Scroll; README and USER-JOURNEY say "fit modes", "fit page/width"; code `FitMode` | docs follow the UI | web-core |
| D5 | file vs part | jump-mark and uniqueness copy says "part" for what every other Studio string calls "file" | one word in copy | VLL |
| D6 | live mode verbs | setlist detail "Go live (rehearsal)" / "Stop live mode"; list row "Arm live mode" / "Disarm live mode" | one pair | web-core |
| D7 | role vocabulary | USER-JOURNEY "admin/member"; design/04 "ADMIN/PERFORMER/CONDUCTOR"; proto and code "admin / conductor / member"; role also = part tag and reading role | proto wins; fix both docs; qualify "role" in prose | web-core |
| D8 | rev / revision / version | four counters share the word; Stage says "rev N" and "Pin this version"; `NoteEntry.concertRev: Long` vs `ConcertBundle.concertRev: ULong` | "rev" for the concert bake, "revision" for annotation history, "render count" for charts | mobile (types), web-core (docs) |
| D9 | destination vs target | authored side `jump_to`/"destination"; baked `target_page`/`target_anchor`; app `targetPage` | acceptable: authored vs resolved; state it in proto comments | web-core |
| D10 | role nouns in the jump toolbar | ⟨D4⟩ says the toolbar states the relationship, never a role noun; the step chip and notices say "next: source" / "next: destination" | placement chip may name the step being placed; selection toolbar may not — write that down in P206 | architect |
| D11 | landmark labels | landmark glyphs appear as raw ids ("segno", "coda", "circle") because the label table only covers cue glyphs | add labels; the generator should refuse an unlabelled glyph | web-core |
| D12 | page-index frame | proto says `target_page` is "within this song"; the bake's intermediate is "within this file"; the app's is a flat index | correct, but three frames — one paragraph in ARCHITECTURE | architect |
| D13 | contract gaps | grey single-channel rasters (T169) recorded in README only; permille-vs-I3 has no invariant note; design/01's Object sketch lacks `order` and `jump_to` and still lists `scope`/`geometry`; design/05 says one overlay per layer-group; design/11 and design/05 still say Stage never writes | update the design docs; add the I3 permille note | architect |
| D14 | shared-owner sentinel | domain `OwnerID == "_shared_"`, baked `LayerImage.owner == ""`, export `owner: "_shared_"` | document, or unify on one | web-core |
| D15 | stale Core comments | `webapi.go` my-files handlers say "default = all pool files" (T138 changed it); `sync/doc.go` says the package holds the apply engine (it lives in `internal/engine`); `TypeHighlight` comment says "replaced" while still accepted; `KindLayerReorder` has no wire kind | fix comments | web-core |
| D16 | WetCanvas vs EditCanvas | file `WetCanvas.tsx` exports `EditCanvas` | rename the file | web-core |
| D17 | per-member variant | declared retired in two places; `ParseConcertID`, the `~` separator and variant filtering are still live | delete or un-retire | web-core |
| D18 | the on-device library | "Bakes" (tab), "Concerts" (heading), "N on device" (Home), "bundle" (delete dialog) | one word | VLL |
| D19 | Parameters vs Settings | UI "Parameters"; code `SettingsScreen`, `showSettings` | code follows UI, or UI says Settings | VLL |
| D20 | "page mode" | the note-mode refusal says "switch to page mode"; the UI calls it Reading mode → Page, and Width also allows notes | "Notes: switch to Page or Width" | mobile |
| D21 | stale "Notes" comments | the Studio drawer tab was renamed "Annotations"; five comments in `Viewer.tsx` still say "Notes" | sweep | web-core |
| D22 | taken as | the Notes tab prints the raw member id; the Stage sheet says "Performing as <displayName>" for the same value | resolve to displayName | mobile |
| D23 | Erase | "Erase note" (delete the whole note) vs the ⌫ eraser tool | "Clear page" or "Delete note" for the button | VLL |
| D24 | Undo | README says Studio has Undo (appends the inverse edit); risks R3 and design/01 say "no undo/redo, only revert" | README is current; update risks and design/01 | architect |
| D25 | jump-mark spec header | `jump-mark.spec.ts` header says the destination is placed first; ⟨D3⟩ and the spec body say source first | fix the header | web-core |
| D26 | "kind" | six unrelated discriminators (`Glyph.kind`, `SetlistItem.kind`, `Invite.kind`, `AvatarKind`, mutation kind, undo kind) | leave; qualify in prose | — |
| D27 | who bakes | glossary said "admin action"; ARCHITECTURE I11 says "admin or member (v1 admin-only)" | this file now says "explicit publish gate"; ARCHITECTURE is authoritative | — |
