# DEMO-VID — TroubaShare walkthrough video (with synced AI voiceover)

**Program** (multi-part, cross-lane) · **Owner routing:** web-core (content + web recording + assembly) · mobile (app recording) · **Status:** PLAN'd 2026-08-04 (VLL request) · **Goal:** one narrated video that shows the whole product — compose → annotate → bake → perform.

## The ask (VLL, verbatim intent)

A single audio-commented video demonstrating how the web app works end-to-end (create a band, invite members with roles, create charts, annotate, layers, conductor role, show/hide layers, …) — **Part 1** — plus a **Part 2** recorded separately on the mobile app (perform offline), stitched into **ONE final video** with a synced **AI-generated / free voiceover**. Use **real tracks and real chord charts / lead sheets / guitar tab / drum** where possible. **One pop/folk band with 2+ songs**, **one orchestra with 2 classical tracks, multiple instrument players + a conductor**. Show annotations placed nicely, conductor-role layers, layer show/hide.

**VLL rulings (2026-08-04):** (1) **public / copyright-safe** content — real public-domain + original songs with real charts (the recommended approach). (2) **Add Amazing Grace** (already a committed PD lead sheet). (3) **The video content BECOMES the shipped demo seed** — reworking the demo to these copyright-safe real songs serves both the video AND fixes the existing demo's copyrighted song titles (Wonderwall/Hallelujah/Black Hole Sun render placeholder PDFs but the *titles* are copyrighted works — off-limits for a public video). So DEMO-VID Part A is also a demo-quality upgrade that ships.


## PLAN UPGRADE (VLL, 2026-08-06) — real published editions + a licensing requirement

Content upgraded beyond the original plan (all landed via the layer-audit stack `e9346d9`):
- **Band (The Troubadours) = 4 songs:** The Open Road (original), House of the Rising Sun + Amazing Grace (traditional/PD, purpose-built charts), **Greensleeves** (real Mutopia voice+guitar edition, **CC-BY-SA 4.0**, David Kastrup — attributed).
- **Orchestra = real engraved editions:** Eine kleine Nachtmusik (real Mozart edition, multi-part Vln I/II/Vla/Cello + full score) and Canon in D (real published edition, **CC-BY 4.0**, Fischer v. Mollard — attributed). No more empty-staff placeholders.
- **CORE fix rode along:** bake now scopes overlays per-file (B11/T40 through the bake) — no cross-part annotation bleed.

**LICENSING REQUIREMENT for Part D (video assembly):** the demo now contains CC-BY (Canon) + CC-BY-SA (Greensleeves) editions. The final video is a public distribution → its **credits MUST attribute** both (creator + license name + link). Repo attributions are in `docs/demo-charts/README.md`; recommend a root-README/NOTICE pointer too. Everything else is PD/original (no obligation).

## Final deliverable

- **One 1080p (1920×1080, 16:9) MP4**, ~7–9 min, **chaptered**: Part 1 web (~5 min) → seamless cut → Part 2 app (~2.5–3 min), with a continuous synced narration track, title cards + lower-thirds, and an optional subtle public-domain music bed.
- **Voiceover:** free/local TTS (**Piper** — high-quality, fully local, no API/cost; Coqui XTTS as a richer alternative). One consistent voice throughout.
- Committed under `docs/video/` (script, scene manifest, generated assets kept out of git-LFS unless VLL wants the raw MP4 versioned; the final MP4 is a release/hosting artifact).

## Content design — the reworked demo (copyright-safe, REAL, doubles as the shipped seed)

Keep the existing band/member/role STRUCTURE (rich annotation/cue/layer showcase already seeded, `core/cmd/seed/`), swap the SONGS to copyright-safe real material. All six users, password `demo` (`main.go:23`); roles admin/conductor/member.

### Band "The Troubadours" (pop/folk) — admin **Marie** (singer); **Leo** (guitar → **conductor**); **Sasha** (bass)
1. **The Open Road** — *original* (VLL's own demo song). Already real: `docs/demo-charts/open-road-leadsheet.pdf` (p1 chords-over-lyrics lead sheet, p2 intro riff as guitar **tab**) + the text chart `open-road-lyrics.chart` (rendered live) + the full 3-layer annotation showcase (`buildOpenRoadAnnotations`). **The flagship annotation scene.**
2. **House of the Rising Sun** — *traditional, PUBLIC DOMAIN* — real, iconic guitar chords (Am · C · D · F · E). NEW real charts: a chords-over-lyrics **text chart** (the dialect) + a one-page **guitar tab** PDF + a simple **drum groove** chart PDF (the 6/8 arpeggio feel) — generate via **LilyPond** (real notation, free) or `cmd/mkcharts`. Covers VLL's "real … guitar or drum."
3. **Amazing Grace** — *public domain* (Newton 1779) — already committed real chart `docs/demo-charts/amazing-grace.pdf` (lead sheet + demo accompaniment). Wire it into the seed (it's committed but currently unused) + a chord text-chart.

### Orchestra "City Chamber Orchestra" (classical, PUBLIC DOMAIN) — admin **Anya/maestro** (conductor-admin); **Flora** (flute → **conductor** layer owner); **Cory** (cello)
1. **Eine kleine Nachtmusik** — Mozart K.525 (PD) — real **per-instrument parts**: Violin I, Violin II, Viola, Cello (IMSLP PD part PDFs, or LilyPond-rendered). Multiple players, distinct parts = the "multiple instrument players" showcase.
2. **Canon in D** — Pachelbel (PD) — real parts (Violin, Cello, Flute) — the most legible multi-part piece. (Existing seed uses Bach *Air on the G String* which is already PD-fetched — keep as fallback if Pachelbel parts are harder to source; Pachelbel preferred for distinct visible parts.)

### Annotations / layers / roles / cues (already richly seeded — extend to the new songs)
- **Conductor cues** — zone `conductor`, `Mandatory:true`, `RoleTag:"conductor"`, owned by the promoted conductor (Leo band / Flora orchestra): brackets, "Watch me — pickup", "rit.", "D.C. al Fine", pointer ellipses (`annotations.go:178-182`). **This is the "conductor role on layers" showcase.**
- **Section markings** — zone `shared`, `_shared_`, amber Verse/Chorus/Bridge/Outro highlights + labels.
- **Personal layers** — Leo's "Chords & capo" (band), Flora's "Bowing/Breath marks" (orchestra, green ticks + "dolce"), Marie's "My notes" (Open Road: "capo 2 OK", circled change, "breathe", freehand flourish).
- **Per-part layers (B11)** — per-file annotation scoping (a layer that only shows on the Vocals PDF vs the Guitar PDF).
- **Personal cues** — per-member instrument icons (mic / guitar / bass / keys / tambourine) shown on setlist rows + flashed on song entry in the app.

All of the above already exist in `annotations.go` for the current songs — Part A re-points them at the new songs (Open Road's is verbatim; the others get the same 3-layer treatment + per-part on House of the Rising Sun's multi-file pool).

## Production pipeline (audio-first, for tight sync)

The crux of "synced voiceover": **generate the narration first, pace the visuals to it.**
1. **Script** → segment the narration by scene (below). One text block per scene.
2. **TTS** → Piper renders each segment to a WAV; record each segment's exact duration.
3. **Web capture** → a Playwright "walkthrough" spec drives the seeded app through the storyboard deterministically, with **`video: { mode: 'on', size: {width:1920,height:1080} }`** added to a dedicated recording config (the base config has no `video:` today — `playwright.config.ts:21`). Each scene's on-screen actions are paced (waits / `slowMo`) to match its narration WAV length, so the action lands under the words.
4. **App capture** → on the **Pixel_7 AVD** (and a **tablet AVD** for facing-pages), `adb shell screenrecord` (a standard emulator capability — not yet used in-repo; capture is stills-only today via `adb exec-out screencap`) records the perform flow, driven by `adb shell input`, paced to its narration segments.
5. **Assemble** → **ffmpeg**: concat the web-part and app-part video, concat the narration WAVs into one track, mux narration over video, add title cards / lower-thirds (member names + roles, chapter titles), optional PD music bed at low gain, export the single MP4.

Everything is scripted + re-runnable, so the video regenerates when the UI or seed changes.

## Storyboard — Part 1: the web app (TroubaStudio)

Each scene: on-screen action → *narration draft (the lane refines wording)* → ~seconds.

| # | On screen | Narration (draft) | ~s |
|---|---|---|---|
| 0 | Title card: "TroubaShare — rehearse together, perform offline" | *"TroubaShare is a self-hosted app for bands and ensembles — from the rehearsal-room edit to the on-stage page turn. Let's build a band from scratch."* | 8 |
| 1 | Register/login as Marie → Bands page → **New band** "The Troubadours" | *"Marie signs in and creates her band, The Troubadours. She's the admin."* | 12 |
| 2 | Band → **Invite member** → invite Leo (guitar) and Sasha (bass); show the pending invites; (cut) Leo accepts on sign-in | *"She invites her bandmates by username — Leo on guitar, Sasha on bass. Each joins by accepting the invite; no email needed."* | 16 |
| 3 | Members list → promote **Leo to conductor** (role chip changes) | *"Roles matter here. Marie promotes Leo to conductor — that unlocks the conductor's own annotation layer, which we'll see in a moment."* | 12 |
| 4 | **New song** "House of the Rising Sun" → open editor → **New text chart** → type/show chords-over-lyrics → Save → it renders in place | *"A song is more than a title. Leo writes a chord chart in plain text — chords over lyrics — and TroubaShare renders it to a clean sheet instantly."* | 18 |
| 5 | Same song → upload the **guitar tab** PDF and the **drum groove** PDF into the shared file pool; show the multi-file tabs | *"Real parts live together: the lead sheet, a guitar tab, even a drum chart — one shared pool, each player picks their view."* | 14 |
| 6 | Open **The Open Road** → the editor with the real lead sheet; toggle the **layers** panel | *"Open Road is the band's original. Watch the annotations — every mark lives on a layer you can show or hide."* | 12 |
| 7 | Show **conductor layer** (red, mandatory): "Watch me — pickup", "rit.", bracket, pointer — toggle it off then on | *"The conductor's cues, in red — 'watch me', 'rit.' — are mandatory: players can't hide them. Leo owns this layer because he's the conductor."* | 16 |
| 8 | Show **shared section markings** (amber Verse/Chorus/Bridge) → toggle; then **Marie's personal 'My notes'** (capo 2 OK, circled change, "breathe", flourish) → toggle | *"Shared section highlights help everyone navigate. And each player keeps private notes — Marie's 'breathe' and capo reminder are hers alone."* | 18 |
| 9 | Draw a quick annotation live (rectangle/marker) on a section, pick a color, move it with the **Move tool**, double-tap to **zoom** in on it | *"Editing is direct — draw, color, move, zoom. It's a canvas, not a form."* | 12 |
| 10 | **Transpose** the House-of-the-Rising-Sun chart (T60) — pick a key, preview, apply; chords change | *"Need a different key? Transpose the whole chart in one click — the chords rewrite, the layout stays put so annotations stay anchored."* | 12 |
| 11 | **Setlists** → create "Sat @ The Anchor" → add the 3 songs → drag-reorder → set a per-item **key override + 'transpose chords'** | *"Songs go into a setlist — drag to reorder, override a key per gig, and the chart transposes just for that show."* | 16 |
| 12 | **Bake** the setlist → the bake dialog (layer defaults) → confirm → a concert bundle is produced, downloadable | *"When it's ready, Marie bakes the setlist into a concert bundle — every part, every layer, frozen for the stage."* | 14 |
| 13 | (cut) Sign in as **Anya** → **City Chamber Orchestra** → open **Eine kleine Nachtmusik** → show the **per-instrument parts** (Vln I/II, Viola, Cello) and Flora's conductor cues + bowing layer | *"It scales to an orchestra too — Mozart, with real parts for each desk, the conductor's cues on top, and each player's own bowing marks."* | 18 |
| 14 | Toggle layers on the orchestral part; show a second player's different view of the same song | *"Same score, every musician sees their own layers — nothing more, nothing less."* | 10 |

## Storyboard — Part 2: the mobile app (TroubaStage), recorded on the emulator/tablet

| # | On screen | Narration (draft) | ~s |
|---|---|---|---|
| 15 | Cut / title card: "On stage — offline." App Home → **Connect** to the band's server (mDNS) → browse offered concerts | *"On stage, there's no wifi to trust. The app connects to the band's server, and downloads the concert to perform completely offline."* | 14 |
| 16 | **Download** "Sat @ The Anchor" → open it → the **immersive Stage page** (whole screen is the score) | *"Tap to perform. The whole screen becomes the page — the baked annotations composited right in."* | 12 |
| 17 | Tap to reveal **chrome**: song drawer, title + position, **♩ tempo meter**, page arrows | *"A tap brings back the controls — the song drawer, the live tempo meter, the pager."* | 10 |
| 18 | **Page turn** (swipe / pedal), then **layer toggle** — hide/show a layer live on stage | *"Turn pages with a swipe or a pedal. Toggle a layer without leaving the page."* | 10 |
| 19 | **Night mode** toggle; **count-in** on the tempo chip (visual pulse) | *"Night mode for a dark stage. A silent visual count-in on the tempo."* | 10 |
| 20 | (tablet AVD) **Facing pages** two-up spread; **per-role layers** — the conductor's view vs a player's | *"On a tablet, facing pages. And every role gets its own view — the conductor sees the cues they wrote; the player sees theirs."* | 14 |
| 21 | Outro card: logo + "Self-hosted. Open source. Yours." + repo/site | *"Rehearse together, perform offline, on hardware you own. That's TroubaShare."* | 8 |

*(Durations are estimates; the audio-first pass sets the real timings.)*

## Parts & lane routing

- **Part A — Rework the demo seed to copyright-safe real content + generate the real charts** — **web-core.** Replace the copyrighted-title songs with Open Road + House of the Rising Sun + Amazing Grace (band) and Mozart + Pachelbel (orchestra); source/generate real charts (committed PD PDFs + LilyPond for tab/drum/parts); re-point the annotation/layer/role/cue showcase onto the new songs; regenerate the demo `.tstage` bundle. **This ships as the new demo** (VLL ruling) — so it also updates any e2e/screenshots that reference old titles (e.g. specs asserting "Wonderwall"). Gate as a normal task; pixel/render-verify the charts.
- **Part B — Web walkthrough: narration script + Playwright recording** — **web-core.** Finalize the script (scenes 0–14) from this storyboard; a dedicated recording Playwright config (`video: on`, 1920×1080) + a walkthrough spec paced to the TTS segments; produce the Part-1 video.
- **Part C — App walkthrough: recording** — **mobile.** Scenes 15–21 on the Pixel_7 AVD + a tablet AVD (facing pages); `adb screenrecord` driven by `adb input`, paced to its narration segments; deliver the raw Part-2 capture + the per-scene timing to web-core for assembly.
- **Part D — Voiceover synthesis + final assembly** — **web-core (integrator).** Piper TTS for all segments (scenes 0–21); ffmpeg concat (web + app) + narration mux + title cards/lower-thirds + optional PD music bed → the single MP4. Publish under `docs/video/`.

Dependency: A before B/C (they record on the new content). D after B+C. A is independently valuable (ships as the improved demo) and should go first.

## Settled defaults (VLL can override any)

- Public / copyright-safe real content ✅ (ruled). Video content = the shipped demo ✅ (ruled). Amazing Grace added ✅.
- Voice: Piper, one clear neutral-English voice (swappable). Length: ~7–9 min comprehensive ("everything"). Format: 1080p 16:9 MP4, chaptered. Music bed: subtle PD instrumental, optional (easy to drop).
- Band/member names kept (not copyrighted); only songs change.

## Open questions for VLL (non-blocking — defaults chosen above)

1. Voice preference (accent/gender) — default neutral English; say the word to pick a specific Piper voice.
2. Music bed under narration — yes (subtle PD) / no. Default: yes, low.
3. Where the final MP4 lives — repo `docs/video/` + a release asset, or an external host (YouTube)? Default: committed reference + release asset.

## Notes for the lanes

- Part A is the load-bearing content step and ships as the demo — treat it as a real feature (regenerate `.tstage`, fix title-referencing e2e/screenshots, verify the real charts render). LilyPond (`lilypond`, free) is the recommended generator for guitar tab / drum / orchestral parts where a direct PD PDF isn't at hand; `cmd/mkcharts` covers the chords dialect.
- Keep the recording harnesses scripted (Playwright spec + adb script) so the video is reproducible, not a one-off manual capture — same discipline as the e2e suite.
- Present each Part at the gate; cite VLL 2026-08-04 (via Fable). — Fable (architect/reviewer)
