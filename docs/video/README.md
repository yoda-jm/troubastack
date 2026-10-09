# TroubaStack — the video series (spec v2)

**Status:** **specced, decisions taken (§9), parked — nobody is dispatched on it.** VLL reviews this on GitHub and decides when (and in
which order) production starts. No lane picks this up from `docs/tasks/`: it deliberately does **not** live there.
**Supersedes:** the single DEMO-VID walkthrough, archived in [`docs/archive/video-v1/`](../archive/video-v1/README.md)
(its README lists the five lessons this spec is built on).
**Decided (VLL, 2026-10-09):** English · a soft male voice · **12 episodes** · soft **original music composed
for the series** · no real-world footage for now · published as a **YouTube playlist** embedded on the project
site · the demo songs (to be improved, §5.4) · **vertical cuts too** · series name still open. See §9.
**Origin:** VLL, 2026-10-08 — *"a series of a dozen or half of that videos, with AI generated audio comments, text
generated, and recordings of both the tablet and browser … episodes are meant to be in sequence, so probably start
with general presentation, installations, stuffs like this."*

---

## Contents

1. [What this series is for](#1-what-this-series-is-for)
2. [The twelve episodes at a glance](#2-the-twelve-episodes-at-a-glance)
3. [Format, voice and visual language](#3-format-voice-and-visual-language)
4. [How an episode is made — the pipeline](#4-how-an-episode-is-made--the-pipeline)
5. [Data, privacy and reproducibility rules](#5-data-privacy-and-reproducibility-rules)
6. [Repository layout and the episode file format](#6-repository-layout-and-the-episode-file-format)
7. [Production plan, waves and gates](#7-production-plan-waves-and-gates)
8. [Episodes — full specs and narration](#8-episodes--full-specs-and-narration)
9. [Decisions](#9-decisions)
10. [Appendix — glossary for the narrator, pronunciation, credits](#10-appendix)

---

## 1. What this series is for

**One sentence:** a band leader who has never heard of TroubaStack can go from *"what is this?"* to *"my band
rehearses and performs with it"* by watching the episodes in order — and each episode also stands on its own when
someone lands on it from a search or from the project page.

**Audiences, in order of priority:**
1. **The band leader / musical director** who would install and run it: wants to know *what it does for my band,
   what it costs me to run, and how my players will use it*. Comfortable following a `docker compose` recipe on a
   screen, not a developer.
2. **The players** the leader sends a link to: only care about episodes 3, 4 (joining), 9–11 (the tablet).
3. **Curious technical people** (self-hosters, open-source contributors): want the architecture in one picture
   (E01) and an honest install (E02, E12).

**What success looks like:** a viewer who finishes E01–E04 can install the server, get the app on a tablet and
invite a second member **without reading the README**. Every feature claim on screen is shown working, not
described (lesson 2 of the archive: no beat may silently skip).

**Out of scope:** marketing hype, roadmap promises, features that are not on `main` when the episode is recorded,
anything shown with real band data (§5).

---

## 2. The twelve episodes at a glance

The series runs in **four arcs**, coloured after the product layer each is mostly about (brand palette, §3.3).

| # | Title | Arc | Length | Main surfaces | One-line promise |
|---|---|---|---|---|---|
| 01 | **What is TroubaStack?** | Welcome · *stack* | 3:30 | GFX + Studio + Stage | The whole loop in three minutes: compose, annotate, bake, perform. |
| 02 | **Install the server** | Getting started · *core* | 4:15 | terminal + browser | One box you own, HTTPS, a first account and a band — in about ten minutes. |
| 03 | **Get the app on your tablet** | Getting started · *stage* | 3:15 | tablet + browser | Install TroubaStage, find your way around Home, and perform a demo with no server at all. |
| 04 | **Your band: invite, roles, join** | Getting started · *studio* | 3:00 | browser + tablet | Invite players with a QR, give the conductor his role, join from the tablet. |
| 05 | **Songs and charts** | Studio · *studio* | 3:30 | browser | PDFs, charts typed as text, tags and search — the band's repertoire in one place. |
| 06 | **Annotating together** | Studio · *studio* | 4:00 | browser ×2 | Draw, highlight, mark — on shared, conductor and personal layers, live with your bandmates. |
| 07 | **Cues and jump marks** | Studio · *studio* | 3:15 | browser + tablet | Tell each player what to prepare, and turn "D.S. al segno" into one tap. |
| 08 | **Setlists and baking** | Studio→Stage · *core* | 3:30 | browser + tablet | Build the running order, bake it, and watch it land on every tablet. |
| 09 | **On stage: reading** | Stage · *stage* | 3:45 | tablet | Page, Width, Scroll, facing pages, the four colour schemes, the song drawer. |
| 10 | **On stage: hands-free** | Stage · *stage* | 3:30 | tablet (+ pedal) | Pedals, volume keys, the silent count-in, the clock — and nothing that can throw you out mid-song. |
| 11 | **The rehearsal loop** | Rehearsal · *stack* | 4:30 | browser + tablet | Live mode re-bakes as the conductor edits; players scribble rehearsal notes that land back in Studio. |
| 12 | **Keeping it safe** | Running it · *core* | 3:30 | terminal + browser | Backups, restores, exporting a band, updating the server — your repertoire never depends on us. |

**Total ≈ 43 minutes** (lengths recalibrated from the measured narration, §10.4). Every episode ends on a 6-second card pointing at the next one; E01 also offers a
"skip to what you need" map (players → E03, E04, E09–E11).

**If VLL wants half a dozen instead of twelve**, the planned merges are: 02+12 (*Install and keep it safe*),
05+06 (*Songs, charts and annotation*), 07+08 (*Cues, jumps and baking*), 09+10 (*On stage*). E01, E03/04 and
E11 stay as they are. The narration below is written so these merges are mostly concatenation (§8 marks the
seams with `⟂ merge seam`).

---

## 3. Format, voice and visual language

### 3.1 Technical format

| Property | Value |
|---|---|
| Master | 1920×1080, 30 fps, H.264 High (CRF 18) + AAC 48 kHz 192 kb/s, `.mp4` |
| Site copy | VP9 + Opus `.webm`, same resolution, for the project page |
| Loudness | −16 LUFS integrated, true peak ≤ −1.5 dBTP (EBU R128 measured with `ffmpeg loudnorm`, two-pass) |
| Captions | `.srt` + `.vtt` per episode, generated from the narration text (never from speech recognition — the text is known) |
| Thumbnail | 1280×720 PNG, from a template (§3.3) |
| Chapters | one per scene group, `MM:SS Title` list for the video description |
| Language | **English** narration and captions (decided, §9 D3) |

### 3.2 Voice and narration rules

- **One narrator voice for the whole series**, AI-generated (§4.4): **a soft male voice** (decided), calm and
  warm, ~145 words per minute. Same
  voice, same settings, every episode — a series that changes voice reads as patched together.
- **Second person, present tense, short sentences.** *"Tap the pencil. Draw. Tap the check mark."* Not *"the user
  can then proceed to…"*.
- **UI words are said exactly as the screen spells them**, and appear as an on-screen label the moment they are
  said (lower third, §3.3). The strings in §8 are copied from the code on `main` (2026-10-08); the glossary is
  partly stale on the note bar ("Erase note"/"Done" are now **"Clear page"** and the **✓**).
- **Never narrate what is not on screen**, and never show a feature without saying what it is for. The *why*
  comes before the *how*: one sentence of purpose, then the gesture.
- **No hype words** (*revolutionary, seamless, powerful*). The product's own tone is plain and exact.
- **Pauses are part of the script**: `[beat]` = 0.6 s of silence, `[pause]` = 1.2 s (the TTS adapter turns them
  into silence; the timing of the picture follows).
- Pronunciation lexicon in §10.2 (TroubaStack, segno, joglar…).

### 3.3 Visual language

**Palette (from `docs/brand`, never restated by eye):** Core blue `#2A8FE9`, Studio pink `#E13198`, Stage yellow
`#FCCC55`, highlighter gold `#FEE36A`, the dark tile as the ground for cards. Each episode wears its arc colour
(table §2): title card, lower thirds, progress strip.

**Building blocks** (Remotion components, §4.3 — each used across the series, designed once):

| Component | What it is | Used for |
|---|---|---|
| `TitleCard` | 3.5 s: the arc's mark (from `docs/brand/dist`), episode number, title, a one-line promise | episode open |
| `ChapterCard` | 1.5 s: chapter title over a blurred still of what comes next | section breaks |
| `LowerThird` | the UI word being said, in a pill of the arc colour, bottom-left, 2–3 s | every UI term |
| `Callout` | ring + arrow onto a point of the footage, drawn on (300 ms), never over the thing it points at | taps, buttons |
| `Zoom` | eased push-in / pull-out on a region of the footage (max 2.2×) | small text, gestures |
| `DeviceFrame` | a neutral tablet bezel (landscape or portrait) / a minimal browser window chrome around footage | all screen footage |
| `Split` | two frames side by side with a shared caption | browser ↔ tablet, two browsers |
| `Diagram` | animated SVG: the three layers, the bake arrow, the network | E01, E02, E08, E12 |
| `Terminal` | VHS-rendered terminal footage in a frame (§4.2) | E02, E12 |
| `EndCard` | 6 s: "Next: E0n — title", the project URL, the arc strip | episode close |
| `CreditsCard` | music credits + the demo charts' licences (NOTICE) | every episode that shows a CC chart |

**Typography:** the brand wordmark paths for logos (no font dependency); UI captions in **Inter** (OFL) — loaded
locally into Remotion, never from a CDN at render time.

**Motion rules:** nothing moves without a reason; callouts draw on, they don't bounce. Footage is never sped up
past 2× or slowed below 0.8× (a page turn played in slow motion lies about how it feels); freeze-frames are fine.
**The tablet's own touch indicator is on** (`show_touches`), so every gesture is visible without a fake cursor.

### 3.4 Music — composed for the series

**Decided:** soft, original music, composed for TroubaStack — no stock library, so no licence to track and the
series owns its sound. It is written as **text in the repo** and rendered by the build, like everything else.

**Character.** Soft and unhurried, a rehearsal room at the end of the day: felt piano, a warm pad, a nylon-string
guitar picking, a soft upright bass; no drums under narration (a brushed snare at most in the title sting). 76–84
bpm, a major key with a gentle IV–vi colour; nothing that competes with a voice — no melody in the voice's
range (≈ 100–300 Hz) while it speaks.

**One theme, four cues.** A single four-bar motif carries the series identity:

| Cue | Length | Use |
|---|---|---|
| `sting` | 3 s | the motif's first bar, piano + pad — `TitleCard`, `EndCard` |
| `bed` | 64 bars, loops seamlessly | under narration; the motif returns every 16 bars, sparsely |
| `bed-light` | same, piano and pad only | under dense explanation (E02, E12 terminal scenes) |
| `outro` | 8 s | the motif resolved — the end card and the credits |

Each product arc (§3.3) may tint the bed — Core: piano forward, Studio: guitar forward, Stage: pad and bass
forward — the same notes, so twelve episodes still sound like one series.

**Pipeline** (all installed on the dev box; all free software):
- source: `video/music/theme.py` writes Standard MIDI (the notes are code — reviewable, diffable, re-voiceable),
  or `video/music/*.ly` (LilyPond) if a cue is easier to write as notation;
- render: `fluidsynth` with the **FluidR3_GM** SoundFont (MIT licence) → 48 kHz WAV; `sox` for fades, a soft
  room reverb and the loop crossfade; MuseScore only to *look* at a cue as a score at review;
- mix: Remotion places the cues; the ducking is sidechain-style — −30 LUFS under narration, −22 in gaps, 400 ms
  ramps — computed from `timings.json`, so music never fights a word;
- checks (§4.5): the bed's loop point is click-free (no sample discontinuity), the master meets −16 LUFS.

**Review:** the four cues are a gate of their own — VLL listens to them (with a narration line over the bed)
before the pilot render, the same way the voice is chosen. Credited on the `CreditsCard` as *"Music: composed for
TroubaStack (CC-BY-SA 4.0, like the docs)"* — licence to confirm with VLL.

### 3.5 Vertical cuts (decided)

One **vertical cut per episode**, 1080×1920, 30–60 s, for Shorts and the project page: the episode's best beat
(listed per episode below as *Vertical*), **burned-in captions** (large, centre-low), the tablet frame filling the
height or the browser cropped to the action, the `sting` at the head and an end slate *"Full episode: <title>"*.
They are cut **from the same takes and narration** — a Remotion composition per cut, `video/vertical/cuts.yaml`
listing `episode, from-scene, to-scene, crop`; no extra shooting. Suggested beats:

| Ep | Vertical beat |
|---|---|
| 01 | 1.5 — bake, and the tablet picks it up |
| 02 | 2.4–2.5 — one settings line, one command, a padlocked site |
| 03 | 3.5 — import a concert with no server |
| 04 | 4.4 — scan a QR, you're in the band |
| 05 | 5.4 — a chart typed as text |
| 06 | 6.5 — two windows, one mark appears live |
| 07 | 7.4 — D.S. al segno in one tap |
| 08 | 8.6 — "new version" → Update |
| 09 | 9.6 — Night and Amber |
| 10 | 10.4 — the silent count-in |
| 11 | 11.4 — live mode: a mark becomes a new rev on the tablet |
| 12 | 12.2–12.3 — one volume, one tar: the whole backup |

---

## 4. How an episode is made — the pipeline

The archive's biggest failure was **one long take that staled with every UI change**. Here every episode is a
**build**: scripted takes → narration audio → composition → render, re-runnable end to end by one command on a
fresh checkout. Changing a sentence or re-shooting one take re-renders only what depends on it.

```
fixtures ──▶ isolated server (seeded) ──▶ TAKES ─────────────┐
               (web: Playwright · tablet: adb · terminal: VHS)│
episode spec (YAML, §6) ──▶ NARRATION (TTS) ──▶ timings ─────┼──▶ COMPOSITION (Remotion) ──▶ RENDER ──▶ LOUDNESS
                                                              │        (+ captions, chapters)            + CHECKS
music/brand assets ───────────────────────────────────────────┘
```

### 4.1 Takes — recorded by scripts, asserted at every beat

A **take** is a short (≤ 2 min) recording of one surface doing one scene group. Every take is a script, and every
**beat** in it ends with an assertion that the UI actually reached the expected state; a failed assertion fails
the take (no `soft()` — archive lesson 2). Each take writes `take.mp4` + `marks.json` (`{beat: "bake-done", t:
12.84}`), so the composition can cut on beats instead of guessed seconds.

**Web takes — Playwright** (`video/takes/web/*.spec.ts`)
- Chrome at a fixed 1920×1080 viewport, device scale factor 1, `prefers-reduced-motion: no-preference`.
- Capture via the **CDP screencast** (`Page.startScreencast`, PNG frames, every frame) piped to ffmpeg at
  CRF 12 — not Playwright's `recordVideo`, whose VP8 is too soft for small UI text.
- An injected **cursor overlay** (an arrow that eases between targets and pulses on click) — real pointer moves are
  instantaneous and unreadable on video.
- `page.clock.setFixedTime('2026-10-12T19:30:00')` so every date on screen is the same in every take.
- Typing uses `pressSequentially` with a human delay (60–90 ms/key) for anything the viewer should read.
- `TROUBA_NO_HMR=1` for the Vite dev server (the switch the archive added; kept), or the embedded SPA build.

**Tablet takes — emulator + adb** (`video/takes/tablet/*.py`)
- AVD **`Trouba_Tablet_1200`**: 1200×1920 @ 280 dpi, Android 36, **`-gpu host`** (software rendering starved the
  host CPU and ANR'd System UI — measured 2026-10-08), `-no-window`. Same panel class as VLL's Redmi Pad SE.
- Recording: `adb shell screenrecord --bit-rate 20000000 --size 1920x1200` (landscape) or `1200x1920`; the
  3-minute cap is why takes are short. Pulled and re-encoded.
- `settings put system show_touches 1` (touch dots on), SystemUI **demo mode** for the status bar (fixed 10:00,
  full battery, no notification icons), `hide_error_dialogs 1`.
- Gestures by `input tap` / `input motionevent DOWN/MOVE/UP` (handwriting needs real paths — proven for the A80
  frames). The helper library keeps the **chrome-visibility check** (Stage chrome auto-hides: reveal only if
  hidden, then tap) and the landscape/portrait coordinate maps.
- **Physical-hardware inserts** are the exception, used only where the emulator cannot fake it: a Bluetooth/
  BLE-MIDI pedal pressing pages (E10) — shot on a real tablet showing **only the demo concert** (§5), with the
  screen recorded by `adb screenrecord` (no camera footage: §9 D5).

**Terminal takes — VHS** (`video/takes/term/*.tape`)
- [charmbracelet/vhs](https://github.com/charmbracelet/vhs) renders a `.tape` script (typed commands, sleeps, a
  fixed theme and font) to MP4: perfectly reproducible, readable, no real secrets on screen. Commands run for real
  against a throwaway VM/container where the episode needs real output (E02's `docker compose up`), recorded output
  otherwise.

### 4.2 The data the takes run against

Every take runs against an **isolated server** started by the pipeline: the `troubacore` binary built from the
checkout, `TROUBACORE_ADDR=:18080`, `TROUBA_DATA_DIR=<temp>`, file stores, `TROUBA_NO_MDNS=1`, the current
`bake` worker — never VLL's `:8080`, never his data dir. It is seeded by `cmd/seed` and then a **fixture script**
(`video/fixtures/`) adds what the seed lacks. Known gaps today:
- **jump marks** — the committed demo bundle and the seed predate P206; the fixture imports a `segno` pair on
  *The Open Road* through the admin `annotations/import` (the shape `core/internal/bake/baker_p206_test.go` uses),
  then bakes (done by hand for the A80 frames, 2026-10-08);
- **rehearsal notes already in Studio** for E11's opening (sent from the emulator by the take itself);
- **a second band member signed in in a second browser profile** for E06's realtime beat.

Some episodes must **start from empty** (E02 first registration, E04 invites): those use a second, unseeded
isolated server. The archive's *build-from-empty* idea survives here, scoped to the episodes where "empty" is the
point.

### 4.3 Composition — Remotion

[Remotion](https://www.remotion.dev) (React + TypeScript, rendered frame-accurately by headless Chrome + ffmpeg)
assembles each episode from: takes (`<OffthreadVideo>` with `startFrom` on a mark), stills, the components of
§3.3, the narration WAVs and the music. **The narration is the clock**: each scene's duration is the measured
length of its audio plus its pauses; footage is fitted to it by trimming, freeze-framing or bounded speed changes
(§3.3 motion rules) — never the other way round.

- Licence: Remotion is free for individuals and companies of ≤ 3 people; a larger company needs a licence.
  If that ever matters, **Revideo** (MIT, Motion Canvas fork) is the drop-in alternative for this pipeline.
- Captions: `@remotion/captions` from the script lines + timings → burned-in captions are **off** in the master
  (the platform shows the `.srt`), **on** in short social cuts.

### 4.4 Narration — AI text-to-speech

The narration text is in this document (§8) and in each episode's YAML (§6). The TTS step:
1. splits the text into **lines** (one sentence or two; the unit of timing and of re-generation),
2. applies the pronunciation lexicon (§10.2),
3. synthesises each line to `audio/epNN/<line-id>.wav` with a fixed voice and seed, caching by `hash(text,
   voice, settings)` so editing one line re-synthesises one line,
4. measures each line (`ffprobe`), writes `timings.json`,
5. normalises loudness at the end, on the mixed master — not per line (archive lesson 4: Piper clipped, and the
   concat demuxer produced white noise; always the concat **filter**).

**Engine:** an adapter interface (`synth(text, voice, settings) → wav`) with two implementations planned:
- **Kokoro-82M** (Apache-2.0, local, offline, good quality) — the default: free, reproducible, no account;
- **ElevenLabs** or **OpenAI `gpt-4o-mini-tts`** — the premium option if VLL wants a more natural voice (paid,
  online, needs an API key kept out of the repo).
Choice and voice are §9 D1. The archive's Piper `en_US-lessac-medium` is retired (robotic, clipped).

### 4.5 Render and checks

`make video EP=03` (or `video/run.sh 03`): fixtures → takes (skipped if cached and their script/app hash is
unchanged) → TTS (cached per line) → Remotion render → two-pass loudnorm → **checks**, which fail the build:
- duration within ±15 % of the spec;
- every `beat` the storyboard references exists in some take's `marks.json`;
- caption text == narration text (character-exact);
- loudness and true peak in range;
- **a privacy scan**: OCR (Tesseract) on one frame per second, refusing any string on a deny-list of VLL's real
  band, concert, member and song names (kept **outside the repo**, read from `$TROUBA_HOME/video-denylist.txt`).

---

## 5. Data, privacy and reproducibility rules

1. **Synthetic data only, by construction.** Every frame shows the seed (*The Troubadours*, *City Chamber
   Orchestra*, *Sat @ The Anchor*, Marie/Leo/Sasha…) or data the pipeline itself created. VLL's tablet holds real
   concerts — **never film it** except for the pedal insert, and then only inside the demo concert, after the
   frame-by-frame check of §4.5. This is the same rule as the public repo's (no band/concert/member/song names).
2. **Never touch VLL's server or data**: isolated port, temp data dir, `TROUBA_NO_MDNS=1`. On the tablet's Connect
   screen, the network discovery lists his real server (observed 2026-10-08: it showed his host name and LAN
   address) — a take types the isolated URL and never taps a discovered row, and a frame showing that list is only
   allowed on a network where the isolated server is the only advertiser (E04 `tab-signin-discovered`).
3. **Never film secrets**: passwords are the seed's demo password; tokens, invite links and `.env` values on screen
   are throwaway ones minted by the take.
4. **Credits**: any episode that shows *Greensleeves* (CC-BY-SA-4.0) or *Canon in D* (CC-BY-4.0) carries the
   `CreditsCard` with the `NOTICE` attributions. *The Troubadours* set includes *Greensleeves*, so in practice
   every Studio/Stage episode does.
5. **Publish is gated** (§7): nothing is uploaded or put on the site before VLL has watched the final render.

---

### 5.4 The demo content — what the series needs from it (VLL: *"probably with the demo songs (maybe need improvement)"*)

Every frame uses the seeded demo band, **The Troubadours** (Marie admin, Leo conductor, Sasha member) and its
songs, all original or public domain (`core/cmd/seed`, `docs/demo-charts/`, NOTICE). An audit against the
episodes:

| Need | Today | Improvement |
|---|---|---|
| A demo bundle that names its band | `docs/demo/demo-concert.tstage` predates band metadata → the tablet lists it under **"Unknown band"** (seen 2026-10-09) | re-bake the committed bundle from the current seed |
| Jump marks for E07/E09 | the bundle has none; the seed creates none (A80 erratum) | a seeded segno pair on *The Open Road* (the shape `baker_p206_test.go` uses), baked into the bundle |
| A chart typed live (E05) | — | the fixture `video/fixtures/charts/lantern-light.chart` — an **original** song written for the series |
| Search box visible (E05) | it only shows past **12** songs; the band has 4 band songs | either film the tag-click path (as specced) or seed ≥ 13 songs — a few more originals |
| Variety on stage (E09/E10) | 4/4, 3/4, 6/8 present; one tab chart; strings parts (Mozart, Pachelbel) | enough; a **two-column** chart without tab would show `columns: 2` (E05) |
| Look of the pages | the charts are typeset plainly | worth one design pass before filming: they are on screen in every episode |

These are **content** tasks (seed + bake + a few original charts), proposed as one task, *DEMO-CONTENT*, to run
before the pilot — not dispatched; VLL's call.

## 6. Repository layout and the episode file format

```
video/                       ← new top-level dir (production code; outside docs/ and web/ builds)
  README.md                  → points here
  run.sh                     one command per episode: fixtures → takes → tts → render → checks
  fixtures/                  seed extras: jump marks, sent notes, empty server, second member
  takes/
    web/                     Playwright specs, one per take, + cursor overlay + screencast recorder
    tablet/                  python/adb takes + the gesture/chrome helper library + AVD setup
    term/                    VHS .tape scripts
  tts/                       engine adapters (kokoro, elevenlabs/openai), lexicon.yaml, cache
  remotion/                  the Remotion project: components (§3.3), one composition per episode
  episodes/
    ep01.yaml … ep12.yaml    the machine-readable episode (generated from §8 once, then the source of truth)
  music/                     theme.py (+ *.ly) — the composed cues as source (§3.4); renders go to out/
  assets/                    brand exports (copied from docs/brand/dist), fonts
  vertical/                  the 9:16 cut list (§3.5)
  out/  takes/*.mp4  audio/  ← generated, gitignored (see .gitignore)
```

**Episode YAML (schema sketch):**

```yaml
id: ep09
title: "On stage: reading"
arc: stage                     # colour + mark
target: "5:00"
voice: narrator                # §4.4 voice profile
scenes:
  - id: s3
    title: "Two pages at once"            # chapter
    narration:
      - id: s3.1
        text: "Turn the tablet sideways in Page mode, and TroubaStage shows two facing pages."
      - id: s3.2
        text: "[beat] One turn moves the whole spread."
    visual:
      - take: tab-reading-twoup          # video/takes/tablet/tab-reading-twoup.py
        from: beat:rotated
        to:   beat:spread-turned
        frame: device-landscape
        callouts: [{ at: s3.2, target: [1765, 985], label: "›" }]
    lower_thirds: [{ at: s3.1, text: "Page mode" }]
```

The YAML is generated **once** from §8 by hand (or a small script) when production starts; from then on the YAML
is the source of truth and this document's §8 is the original brief. Any narration change goes through VLL (§7).

---

## 7. Production plan, waves and gates

**Nobody starts until VLL says so (status above).** When he does:

### Phase 0 — decisions (VLL, one sitting)
The §9 list: voice engine and voice, music, caption languages, publishing destinations, B-roll yes/no, and
whether twelve or six.

### Phase 1 — the pipeline and a pilot (≈ 1 week of agent time)
Build `video/` end to end with **one** episode as the pilot: **E03** (*Get the app on your tablet*) — it exercises
every source (tablet takes, a browser take, GFX, TTS, captions) and is short. Deliverables:
- isolated-server + fixture runner; AVD bootstrap; the tablet helper library (from the A80 session's `draw.py`
  and chrome check); the web recorder (CDP screencast + cursor overlay);
- TTS adapter (Kokoro first) + lexicon + cache; Remotion skeleton with **all** §3.3 components;
- `run.sh 03` producing a checked master. **Gate P1:** VLL watches E03 and rules on voice, pace and look *before*
  any other episode is made — changing the voice after twelve renders is twelve re-renders.

### Phase 2 — waves
| Wave | Episodes | Why grouped |
|---|---|---|
| A — Getting started | 01, 02, 04 (03 done in P1) | the entry path; E01 last in the wave, so it can quote shots from the others |
| B — Studio | 05, 06, 07, 08 | share the web recorder and the seeded server |
| C — Stage & rehearsal | 09, 10, 11 | share the tablet library; E10 needs the pedal insert |
| D — Running it | 12 | terminal + browser, short |

### Gates per episode (all four, in order)
1. **Script gate** — VLL reads the episode's narration (YAML) and approves the wording. *(Cheap; catches 80 %.)*
2. **Animatic gate** — a render with stills + real TTS + placeholder boxes for missing takes: pacing and story.
3. **Final gate** — Fable reviews the render against this spec (beats present, strings exact, privacy scan clean,
   checks green); VLL watches it.
4. **Publish** — only after 3. Destinations per §9 D6. A re-render after any UI change that a take asserts on is
   automatic: the take fails, the episode is flagged stale instead of silently shipping a wrong picture.

### Lanes (when dispatched)
web-core owns `takes/web`, `takes/term`, `remotion/`, `run.sh`; mobile owns `takes/tablet` and the AVD; Fable
reviews scripts and finals; VLL owns the gates. The narration texts below are the brief — the lanes do not rewrite
them without the script gate.

---

## 8. Episodes — full specs and narration

Conventions: **WEB** = Studio in the browser, **TAB** = the tablet (emulator unless marked *HW*), **TERM** = VHS
terminal, **GFX** = Remotion graphics. Times are budgets (narration-driven, ±15 %). `→ LT "…"` = lower third.
Strings in quotes are the UI's own words.


---

### E01 — What is TroubaStack?

| | |
|---|---|
| **Arc** | Welcome · colour: the stack (three-colour stroke) |
| **Length** | 3:30 (narration 314 words ≈ 2.3 min of speech; the rest is watched action and cards) |
| **Surfaces** | GFX (diagram), WEB (Studio editor), TAB (Stage, two-up, warm) |
| **Goal** | The viewer can say, in their own words, what the three pieces are and what the loop is — and decides whether to keep watching. |
| **Perimeter — in** | The problem (paper, PDFs on WhatsApp, the page turn mid-song), the three layers, the loop compose → annotate → bake → perform, offline on stage, self-hosted, the episode map. |
| **Perimeter — out** | Any how-to. No install, no clicks explained — every shot here is *quoted* from a later episode and says "see E0n". |
| **Made** | **Last of wave A**, from takes shot for E02–E11 (no takes of its own beyond the GFX). |

**Prerequisites:** the takes of E03, E06, E08, E09, E11 exist (it re-uses their best 2–4 s moments).

**Storyboard**

| # | Time | Visual | On screen |
|---|---|---|---|
| 1.1 | 0:00–0:14 | GFX: a music stand, a stack of paper sliding off; a phone with 14 PDFs in a chat | — |
| 1.2 | 0:14–0:23 | `TitleCard`: TroubaStack mark, "01 · What is TroubaStack?" | promise line |
| 1.3 | 0:23–1:04 | `Diagram`: three planes appear one by one — TroubaStudio (pink, pencil chip), TroubaCore (blue, CPU chip), TroubaStage (yellow, play chip); an arrow Studio → Core → Stage | LT "TroubaStudio" / "TroubaCore" / "TroubaStage" |
| 1.4 | 1:04–1:39 | WEB (from E06): Marie highlights "Capo 2", Leo's conductor ring appears live in a second window (`Split`) | LT "annotate together" |
| 1.5 | 1:39–2:03 | WEB (from E08): "Bake setlist" → progress "Baking — song 2 of 4: …" → the concert row now reads "Rev 3" | LT "bake" |
| 1.6 | 2:03–2:43 | TAB (from E09/E10): the concert opens, a pedal turns a spread, the count-in pulses, a jump mark → "Go" → landing glow | LT "TroubaStage — offline" |
| 1.7 | 2:43–3:07 | TAB (from E11) a note scribbled on the tablet → WEB the same note under the score in Studio | LT "rehearsal notes" |
| 1.8 | 3:07–3:23 | GFX: the episode map — 12 tiles in four arc colours; "players: start at 03" highlights 03, 04, 09–11 | — |
| 1.9 | 3:23–3:30 | `EndCard` → "02 · Install the server" | — |

**Narration**

> **1.1** Every band has the same pile. Paper that slides off the stand. Fourteen PDFs in a group chat — and nobody
> is sure which one is the latest. [beat] And somewhere in the middle of a song, a page that has to turn while
> both your hands are busy.
>
> **1.2** This is TroubaStack. [beat] It keeps your band's music in one place, lets everyone mark it up together,
> and puts the result on a tablet that performs it — offline, hands-free.
>
> **1.3** It has three parts. [beat] TroubaStudio is where you work on the music, in a web browser: upload a PDF or
> type a chart, then draw on it. [beat] TroubaCore is the server that holds the one true copy of everything — and it
> runs on a box you own, not in somebody's cloud. [beat] TroubaStage is the app on your tablet. It is where you
> perform. [pause] The name is a troubadour joke: the troubadour composes, the joglar performs.
>
> **1.4** Annotation is shared, and it is live. When the conductor circles a bar, everyone sees it. [beat] But
> each player also keeps personal notes that nobody else has to read.
>
> **1.5** When the setlist is ready, you bake it. Baking freezes the songs, in order, with their annotations, into
> one concert file — and gives it a version number, so everyone knows they have the same one.
>
> **1.6** On the tablet, the concert downloads once and then needs nothing: no network, no account. [beat] Pages
> turn with a foot pedal. A silent count-in sets the tempo. And a repeat sign becomes a button.
>
> **1.7** In rehearsal, players scribble on their tablet — and those notes come back to Studio, so the band can
> make them real.
>
> **1.8** This series follows that loop, in order. If you run the band, start with the next episode: installing
> the server. [beat] If you just play in one, skip to episode three, get the app — and then four, nine, ten and
> eleven.
>
> **1.9** Next: installing the server.

**Acceptance:** every quoted shot exists in the take its scene names; no shot appears that is not later explained.

---

### E02 — Install the server

| | |
|---|---|
| **Arc** | Getting started · colour: Core blue |
| **Length** | 4:15 (narration 382 words ≈ 2.8 min of speech; the rest is watched action and cards) |
| **Surfaces** | GFX, TERM (VHS), WEB (empty server: register, new band) |
| **Goal** | A band leader with a small server or VPS and a domain can follow along and end with TroubaStack running over HTTPS, his account, and an empty band. |
| **Perimeter — in** | What you need (a box with Docker, a domain, ports 80/443); `deploy/.env` + `docker compose up -d`; automatic HTTPS (Caddy); the health check; first visitor registers; "New band" → you are admin; where the data lives (one volume = the backup unit, teaser for E12); the LAN-only variant (`docker run`, plain HTTP) in 30 s. |
| **Perimeter — out** | systemd, building from source, `make demo` (developer path — a one-line mention for contributors), PostgreSQL, arm64 (said once as "amd64 today"), backups in detail (E12). |

**Prerequisites:** a disposable VM/container host for the real `docker compose up -d` run (TERM footage of a real
pull and start), its Caddy in `tls internal` or Let's Encrypt **staging** mode (no real cert on film is needed, but
the padlock shot needs a valid one — see §9 D7), a throwaway domain. WEB against an **unseeded** isolated server.

**Takes**

| Take | Surface | Beats asserted |
|---|---|---|
| `term-compose-up` | TERM (real) | `cp deploy/.env.example deploy/.env`; editor shows `DOMAIN=band.example.org`; `docker compose up -d` → pulls `vincentleligeour/troubastack:latest` → `troubacore` healthy in `docker compose ps`; `curl https://…/healthz` → `ok` |
| `term-docker-run` | TERM (recorded) | `docker pull …` then the 4-line `docker run … -e TROUBA_APP_STORE=file -e TROUBA_STORE=file -v troubadata:/data` |
| `web-first-account` | WEB (empty server) | `/register` → account `marie` created → `/bands` empty → "+ New band" → field "New band name" "The Troubadours" → "Create band" → the band appears in the list (no navigation) → click it → Overview, Marie listed as admin |

**Storyboard**

| # | Time | Visual | On screen |
|---|---|---|---|
| 2.1 | 0:00–0:07 | `TitleCard` | "02 · Install the server" |
| 2.2 | 0:07–0:34 | `Diagram`: your box → Docker → TroubaCore + Caddy → the internet (padlock); the data volume as a drawer | LT "one binary · one data folder" |
| 2.3 | 0:34–0:55 | GFX checklist: a host with Docker · a domain pointing at it · ports 80 and 443 | — |
| 2.4 | 0:55–1:59 | TERM `term-compose-up` (cuts on beats) | `Zoom` on `DOMAIN=`; LT "docker compose up -d" |
| 2.5 | 1:59–2:16 | WEB: the padlock + the login page at `https://band.example.org` | LT "HTTPS, automatic" |
| 2.6 | 2:16–3:07 | WEB `web-first-account` | LT "Register" → "New band" → "admin" |
| 2.7 | 3:07–3:37 | `Diagram`: the `troubadata` volume; "this folder is your band — back it up (E12)" | LT "/data" |
| 2.8 | 3:37–4:02 | TERM `term-docker-run` in a smaller frame | LT "LAN only · plain HTTP" |
| 2.9 | 4:02–4:15 | `EndCard` → "03 · Get the app on your tablet" | — |

**Narration**

> **2.2** TroubaStack runs on a computer you control: a small home server, or a cheap rented one. [beat] It is a
> single program and a single data folder. No database to install, no cloud account, nothing that phones home.
> The recommended way to run it is Docker, with a small web server in front that takes care of HTTPS for you.
>
> **2.3** You need three things. [beat] A machine with Docker and its compose plugin. [beat] A domain name — or a
> sub-domain — pointing at that machine. [beat] And ports eighty and four-four-three open to the internet, so a
> certificate can be issued automatically.
>
> **2.4** In the TroubaStack repository, go to the deploy folder. Copy the example settings file, and set one
> line: your domain. [pause] Then start everything with docker compose up, dash d. [beat] Docker pulls the
> TroubaStack image, starts the server, and starts Caddy, which fetches a certificate for your domain. [pause]
> After a minute, docker compose p s shows the server as healthy. You can ask it directly: the health check
> answers "ok".
>
> **2.5** Open your domain in a browser. You get TroubaStack's "Log in" page, over HTTPS.
>
> **2.6** A fresh server is empty — there is no administrator account built in. [beat] The first person to open
> it registers, here Marie, who leads the band. [pause] She lands on "My bands", which is empty. She clicks "New band",
> types the band's name, and clicks "Create band". The band appears in her list; she opens it. [beat] Creating a
> band makes you its admin. That is the only kind of admin
> TroubaStack has: per band, not per server. [beat] One thing to know: registration is open, so anyone who can
> reach your server can create an account. If that matters to you, keep the server on a private network or
> behind a VPN.
>
> **2.7** Everything this server knows lives in one place: the data volume. Songs, annotations, concerts —
> all of it. [beat] Nothing in there can be rebuilt from anywhere else, so it is the one thing to back up.
> Episode twelve shows how.
>
> **2.8** No domain, or just trying it out on your home network? You can run the same image with a single
> docker run command. This serves plain HTTP on port eight-zero-eight-zero — fine for a rehearsal room, not for
> the open internet. [beat] And set both store variables, as shown: without them, the server forgets everything
> when it stops. [beat] The published image is for regular PC-type servers today; on a Raspberry Pi or another ARM
> machine, you build it from source. [beat] And if you'd rather hack on TroubaStack itself, the README's "make demo"
> runs everything locally with demo data.
>
> **2.9** The server is ready. Next, the app on your tablet.

**Acceptance:** the `docker compose` take is a real run (output not mocked); no real domain, IP or secret on screen;
the LAN variant shows both `TROUBA_*_STORE=file` lines in full.

---

### E03 — Get the app on your tablet  *(the Phase 1 pilot)*

| | |
|---|---|
| **Arc** | Getting started · colour: Stage yellow |
| **Length** | 3:15 (narration 276 words ≈ 2.0 min of speech; the rest is watched action and cards) |
| **Surfaces** | WEB (account menu "Get the app"), TAB (install, Home, import, perform offline) |
| **Goal** | Any player can install TroubaStage, knows the Home screen, and has performed a concert — without any server — before the band is even set up. |
| **Perimeter — in** | Where the APK comes from (your server's "Get the app" card + QR, or the project page's link), allowing installs from unknown sources (said plainly: a debug build, not a store app yet), first launch and Home (the TroubaStage tile, "Perform · import or download a concert", the TroubaStudio tile greyed with "Sign in to manage concerts", the "Guest" chip), importing the demo `.tstage`, opening it, reading a page, airplane mode on, still working; iOS status in one line. |
| **Perimeter — out** | Connecting to the band (E04), reading modes (E09). |

**Prerequisites:** emulator with a **fresh** install (app data cleared); the demo bundle `docs/demo/demo-concert.tstage`
pushed to `/sdcard/Download/`; an isolated server built with the APK in `deploy/apps/` (so the "Get the app" card
exists) and seeded (Marie logged in, WEB).

**Takes**

| Take | Surface | Beats asserted |
|---|---|---|
| `web-get-the-app` | WEB | account menu → "📱 Get the app" → the "Get TroubaStage" panel with a QR (the item exists only when the server was built with an APK in `deploy/apps/` — the fixture builds it so) |
| `tab-install` | TAB | the APK installs (package present), app icon on the launcher, first launch → Home with "Guest" chip, TroubaStage tile "Perform · import or download a concert", TroubaStudio tile disabled "Sign in to manage concerts" |
| `tab-import-demo` | TAB | TroubaStage tile → Concerts → "Import" → system picker → `demo-concert.tstage` → row "Sat @ The Anchor" with "rev N" |
| `tab-perform-offline` | TAB | airplane mode on (status bar icon) → open the concert → "Who are you?" → "Not now" → page 1 shown → swipe → page 2 |

**Storyboard**

| # | Time | Visual | On screen |
|---|---|---|---|
| 3.1 | 0:00–0:06 | `TitleCard` | "03 · Get the app on your tablet" |
| 3.2 | 0:06–0:32 | WEB `web-get-the-app`, `Zoom` on the QR | LT "Get the app" |
| 3.3 | 0:32–0:57 | TAB `tab-install` (download + "unknown sources" dialog) | LT "install from your server" |
| 3.4 | 0:57–1:29 | TAB Home, three `Callout`s: Guest chip · TroubaStage tile · TroubaStudio tile | LT "TroubaStage" / "TroubaStudio" |
| 3.5 | 1:29–2:10 | TAB `tab-import-demo` | LT "Import" · ".tstage" |
| 3.6 | 2:10–2:59 | TAB `tab-perform-offline`, airplane icon `Callout` | LT "Who are you?" · "offline" |
| 3.7 | 2:59–3:15 | `EndCard` → "04 · Your band" | — |

**Narration**

> **3.2** TroubaStage is an Android app, and your own server can hand it out. [beat] In TroubaStudio, open the
> account menu and choose "Get the app": a panel with a download link and a QR code. Point the tablet's camera at it.
> [beat] You can also get the latest build from the project page.
>
> **3.3** Android will ask you to allow installs from this source. Today the app is distributed directly, not
> through a store, so this one-time permission is expected. [beat] An iPhone and iPad version exists and runs in
> the simulator; the store release comes later.
>
> **3.4** Open the app. This is Home. [beat] The big tile is TroubaStage: perform. It works with or without a
> connection. [beat] The second tile, TroubaStudio, is for managing concerts. It stays greyed out until you
> connect to your band — which is the next episode. [beat] And the chip in the corner says who you are: for now,
> a guest.
>
> **3.5** You do not need a server to try it. A concert is a single file — a dot-T-stage file. Copy one onto the
> tablet from anywhere — email, a messenger, a USB stick — and import it. [beat] The project ships a demo concert.
> Open TroubaStage, tap "Import", and pick the file. [beat] "Sat at The Anchor" appears in your library, with its
> version number.
>
> ⚠ **Blocked — product bug, found while checking this spec (2026-10-08):** the "Import" button has been
> unreachable since A65 (`ee3c6a68`, 2026-09-04): `manageIntent` is only ever set to `false`, so the Manage screen
> that holds "Import" never opens, and the Perform list says *"No concerts on device yet. Open TroubaStudio to import
> or download one."* — while TroubaStudio is disabled for a guest. There is also no `.tstage` VIEW intent-filter, so a
> tapped file cannot open the app. **E03 (the pilot) cannot be shot until this is fixed**; the narration above
> describes the intended path (import from the TroubaStage screen) and must be re-checked against the fix.
>
> **3.6** Now put the tablet in airplane mode. [pause] Open the concert. [beat] TroubaStage asks "Who are you?" —
> each player sees their own part, their own layers and their own cues. You can pick a name or say "Not now".
> [pause] And here is the first page. No network, no account — the whole concert is on the tablet. [beat] Swipe to
> turn the page.
>
> **3.7** Next: connecting the tablet to your band, and inviting the others.

**Acceptance (pilot extras):** this episode is the pipeline proof — every component of §3.3 appears at least once
(TitleCard, LowerThird, Callout, Zoom, DeviceFrame both orientations, EndCard, CreditsCard), captions present, checks
of §4.5 green. **Gate P1** (§7) rules on voice, pace and look from this render.

---

### E04 — Your band: invite, roles, join

| | |
|---|---|
| **Arc** | Getting started · colour: Studio pink |
| **Length** | 3:00 (narration 271 words ≈ 2.0 min of speech; the rest is watched action and cards) |
| **Surfaces** | WEB (Studio Settings, Overview, `/join`), TAB (Connect, Join with QR, Home connected) |
| **Goal** | The band leader invites his players; a player joins from the tablet in under a minute; roles are understood. |
| **Perimeter — in** | The three membership roles (admin, conductor, member) and what each may do; Settings → "Invite links": Role, "Expiry (hours)", "Max uses", "Create link", "🔒 Reveal"; the invite by username alternative (one line); joining from the tablet: Home "⧉ Scan a QR to join a band" → QR → "You've been invited to …" → create account → "You've joined …"; the server safety check ("TroubaStage will check this is a TroubaStack server before asking for your password"); signing in on another tablet (Connect → "Sign in", "Servers on this network"); the connected Home ("Performing as … ✓", TroubaStudio tile now active); promoting Leo to conductor; members are band-scoped, the band is the privacy boundary. |
| **Perimeter — out** | Part tags and "Who are you?" depth (E09), "Your invite QR" on `/me` (a one-liner). |

**Prerequisites:** isolated server with **The Troubadours** created with Marie (admin) and Leo (member, joined off camera) only (fixture "empty band");
emulator with the app installed, not connected; a camera feed for the QR scan — the emulator's virtual scene camera
can show an image: the fixture renders the invite QR to a PNG and injects it (`-camera-back virtualscene` +
poster). Fallback: "Paste a link instead" (also a real UI path).

**Takes**

| Take | Surface | Beats asserted |
|---|---|---|
| `web-invite-link` | WEB | Settings → Invite links: Role "member", Expiry 24, Max uses 1 → "Create link" → "🔒 Reveal" (tooltip "Reveal the join link (QR + URL)") → QR and URL visible → "Hide" |
| `tab-join-qr` | TAB | Home "⧉ Scan a QR to join a band" → "Point at the invite QR" → "This invite points at a server you haven't used before." + the TroubaStack-check line → "Continue" → "Verifying …" → sign-in step → "New here? Create an account" → Username/Display name/Password → "Create account" → "You've been invited to" / "The Troubadours" / "as member" → "Join" → "You've joined The Troubadours." → "Done" → Home chip label "The Troubadours" → tap the chip → "Performing as Sasha · The Troubadours ✓" (or " · syncing…" first) |
| `web-members-roles` | WEB | Overview: Sasha listed; Settings → Leo's role → "conductor" → saved (the "empty band" fixture includes Leo as a plain member who joined off camera) |
| `tab-signin-discovered` | TAB (2nd emulator profile) | Home chip → "Join or sign in" → "Connect to your band" (Sign in tab) → "Servers on this network" → tap the **isolated** server row → Username/Password → "Connect" → connected. Note: the E02 compose install sets `TROUBA_NO_MDNS=1` (it never advertises) — discovery is shown as the bare-binary / LAN case, said as such. **Constraint (§5.2):** discovery needs the isolated server's mDNS ON, and on VLL's LAN the list would *also* show his real server — so this take runs only on a network where the isolated server is the only advertiser (a separate Wi-Fi/VLAN or a network namespace bridged to the emulator). If that is not available, the take **types the URL** instead and the narration's "shows up by itself" sentence is cut. |

**Storyboard**

| # | Time | Visual | On screen |
|---|---|---|---|
| 4.1 | 0:00–0:05 | `TitleCard` | "04 · Your band" |
| 4.2 | 0:05–0:30 | GFX: three role badges — admin · conductor · member — with what each can do | LT "roles" |
| 4.3 | 0:30–1:07 | WEB `web-invite-link` | LTs "Invite links" · "Max uses" · "Reveal" |
| 4.4 | 1:07–2:00 | `Split`: WEB QR left, TAB `tab-join-qr` right | LTs "Scan a QR to join a band" · "Create account" |
| 4.5 | 2:00–2:23 | WEB `web-members-roles` | LT "conductor" |
| 4.6 | 2:23–2:50 | TAB `tab-signin-discovered` + Home connected, `Callout` on the TroubaStudio tile now active | LT "Servers on this network" |
| 4.7 | 2:50–3:00 | `EndCard` → "05 · Songs and charts" | — |

**Narration**

> **4.2** A band in TroubaStack has three kinds of members. [beat] Admins manage the band: members,
> invitations, and baking concerts. [beat] Conductors lead the music: their marks can be made compulsory for everyone.
> [beat] And members — the players — read, annotate for themselves, and share what they want to share. [beat] The
> band is also the privacy boundary: nothing in it is ever visible outside the band.
>
> **4.3** To invite someone, the admin opens the band's Settings. Under "Invite links", choose the role the
> invitation gives, how long it stays valid — a day by default — and how many people can use it: one, by default.
> Click "Create link". [beat] The QR code stays blurred and the link hidden until you click "Reveal", so nobody
> reads them over your shoulder.
>
> **4.4** On the tablet, the player taps "Scan a QR to join a band", and points the camera at the code. [pause]
> First, TroubaStage checks that this really is a TroubaStack server — before it ever asks for a password. [beat]
> New here, Sasha creates her account right in the app: a username, a display name, a password. [pause] Then it
> shows which band, and which role. She taps "Join". [pause] "You've joined The Troubadours." [beat] Back on Home,
> the chip in the corner now shows the band, and the TroubaStudio tile is active.
>
> **4.5** Back in Studio, Sasha is in the member list. [beat] Marie makes Leo the conductor: one change in Settings.
>
> **4.6** Already have an account and a new tablet? Tap the chip in the corner, then "Join or sign in". [beat]
> Type your server's address, your username and password, and connect. [beat] If your server advertises itself on
> the local network, it is listed under "Servers on this network" — one tap fills the address. [beat] Your username
> is remembered next time.
>
> **4.7** The band is set. Next: putting music in it.

⟂ merge seam — if 02+12 and 05+06 are merged (six-episode cut), E04 keeps its own episode.

---

### E05 — Songs and charts

| | |
|---|---|
| **Arc** | Studio · colour: Studio pink |
| **Length** | 3:30 (narration 312 words ≈ 2.3 min of speech; the rest is watched action and cards) |
| **Surfaces** | WEB |
| **Goal** | The leader can build the band's repertoire: add songs, attach PDFs, type a chart, find songs again — and understands that one song can hold several files and each player picks theirs. |
| **Perimeter — in** | "Add song" (title, artist); the song's Details (Key, Tempo, **Metre**, Notes, Tags); the **file pool** ("＋ Upload file" → "Add to pool", "＋ New text chart", "＋ New tab", "＋ New chart from lyrics"); the chart editor (`# title`, `## section`, chords over lyrics, "Preview", "Save chart"), `{sot}…{eot}` tab blocks in one line, two columns in one line; **Transpose** ("Transpose…" → "Transpose to key" → "Preview" → "Apply"; "Also update the song key" ticked by default); **My files** (checkboxes per file; the ● on the file tab, tooltip "In my stage selection") — Leo's guitar-tab-only selection; tags (chips, the band's tag cloud) and the Overview search ("Filter by title, artist or tag…" — the box shows by itself only past 12 songs, or once a tag is clicked; tag chips combine with AND); the Tags panel (rename/merge, above the songs) in one beat. |
| **Perimeter — out** | Annotation (E06); lyrics lookup over the network ("Search by song" — shown, but the take uses "…or paste the lyrics here", no third-party site on film); deleting songs. |

**Prerequisites:** seeded isolated server (The Troubadours); for the new chart, **original lyrics written for the
take** ("Lantern Light", two short verses and a chorus, in `video/fixtures/charts/lantern-light.chart`) — never a
copyrighted song, never a lyrics site on film; a PDF fixture for the upload beat (one of the public-domain demo
charts).

**Takes**

| Take | Surface | Beats asserted |
|---|---|---|
| `web-add-song` | WEB | Overview → "Add song" → Title "Lantern Light", Artist "The Troubadours" → song opens → Details: Key "D", Tempo 104, Metre "4/4", Tags `original`, `ballad` |
| `web-new-chart` | WEB | Files → "＋ New text chart" → the editor opens pre-filled with `# Lantern Light` / `## Verse 1` → type two chord-over-lyric lines and `## Chorus` → click "Preview" → the page renders → "Save chart" → pool row named "Lantern Light". A second fixture chart shows `{sot}…{eot}`; `columns: 2` is shown on a chart **without** tab (a chart with tablature always stays in one column) |
| `web-transpose` | WEB | reopen the saved chart → "Transpose…" → "Transpose to key" E → "Preview" → "Apply" → chords changed, lines unchanged, song key now E. ⚠ The editor currently shows a stale caveat ("Editing re-renders the PDF — layout may shift, so existing annotations on this chart can end up off their original spot.") that contradicts anchoring (T145) — fix the copy before shooting, or keep it out of frame |
| `web-upload-pdf` | WEB | Files → "＋ Upload file" → pick a PDF (fixture) → "File name" → "Add to pool" → second file in the pool |
| `web-my-files` | WEB (as leo) | House of the Rising Sun → My files: "Guitar tab" checked, others unchecked → the file tab shows ● (tooltip "In my stage selection") |
| `web-search-tags` | WEB | Overview → click the `folk` tag pill on a song row → the search box appears with chip `folk` → 2 songs → click `public-domain` → AND → 1 song; Tags panel visible above the songs (the seeded band has 4 songs: the box only shows by itself past 12) |

**Storyboard**

| # | Time | Visual | On screen |
|---|---|---|---|
| 5.1 | 0:00–0:06 | `TitleCard` | "05 · Songs and charts" |
| 5.2 | 0:06–0:21 | WEB Overview of the seeded band (four songs, tag pills) | LT "Overview" |
| 5.3 | 0:21–0:49 | WEB `web-add-song` | LTs "Add song" · "Metre" · "Tags" |
| 5.4 | 0:49–1:45 | WEB `web-new-chart` (typing at reading speed, `Zoom` on the preview) | LTs "New text chart" · "## section" · "Preview" |
| 5.5 | 1:45–2:06 | WEB `web-transpose` | LT "Transpose to key" |
| 5.6 | 2:06–2:27 | WEB `web-upload-pdf` | LT "Upload file" |
| 5.7 | 2:27–2:52 | WEB `web-my-files` (Leo's view) | LT "In my stage selection" |
| 5.8 | 2:52–3:20 | WEB `web-search-tags` | LTs "Filter by title, artist or tag" |
| 5.9 | 3:20–3:30 | `EndCard` → "06 · Annotating together" | — |

**Narration**

> **5.2** This is the band's Overview: every song the band plays, with its artist and its tags. [beat] Let's add one.
>
> **5.3** "Add song", a title, an artist. [beat] In the song's details you set what the stage will need: the key,
> the tempo, and the metre — four-four here. [beat] Tags are free words that help you find songs later: original,
> ballad, encore… Type one and press Enter.
>
> **5.4** A song can hold several files: a lead sheet, a guitar part, a drum chart. Together they are the song's file
> pool. [beat] You can upload PDFs or images. Or you can type a chart. [pause] "New text chart" starts with the
> song's title already in place. Write the rest as plain text: two hashes for a section, chords on the line above
> the words. [beat] Click "Preview", and you see the real page your band will read. [beat] Guitar tab goes between
> "start of tab" and "end of tab" markers; and on a chart without tab, one header line splits the page into two
> columns. [beat] Save, and the chart joins the pool, named after its title.
>
> **5.5** Because it is text, a chart can change key. "Transpose", pick the target key, preview, apply — every chord
> moves, and the song's key follows. [beat] The lines
> stay where they were, so any marks already drawn on the page stay on their words.
>
> **5.6** And a PDF you already have: "Upload file", pick it, "Add to pool".
>
> **5.7** Each player then chooses which of the song's files they want on stage. Leo plays guitar: on House of the Rising Sun, he ticks only the guitar tab. [beat] On his tablet, that is all he
> will see.
>
> **5.8** Tags make songs easy to find. Click a tag on any song, and it becomes a filter; click a second one, and only
> songs with both remain. [beat] Once your repertoire passes a dozen songs, a search box finds them by title,
> artist or tag as you type. [beat] The Tags panel above the songs renames or merges tags for the whole band —
> always telling you how many songs will change first.
>
> **5.9** Next: drawing on the music, together.

---

### E06 — Annotating together

| | |
|---|---|
| **Arc** | Studio · colour: Studio pink |
| **Length** | 4:00 (narration 344 words ≈ 2.6 min of speech; the rest is watched action and cards) |
| **Surfaces** | WEB ×2 (Marie and Leo, `Split`), WEB single |
| **Goal** | Every player understands the three kinds of layers and draws their first marks; the leader sees that edits are live and that the conductor's layer can be compulsory. |
| **Perimeter — in** | The full-bleed editor; tools ("Move", "Select", "Pen", "Line", "Rect", "Ellipse", "Text", "Icon" → "Stamp icon", "Jump mark" as a teaser); style ("Colour", "Stroke width", presets ▢ "Outline" / ■ "Box" / ▨ "**Highlight**" (icon buttons, names in tooltips), "Line" Solid/Dashed/Dotted and "Start"/"End" None/Arrow/Circle/Square in the ⋯ style popover); "Drawing on: <layer>" with 👥 Band / 👤 Mine; the Layers panel: shared ("Form / sections" — also **"required"** in the seed), conductor ("(conductor)", "required", editable only by conductors), personal ("My notes" — others see them only if they switch them on; owner shown by the avatar tooltip "Your layer"), show/hide, "New layer", "Lock layer"; realtime (two windows, a mark appears in the other); "Undo" (and its honest refusal "Can't undo — someone else changed this since."); marks stay on their words after a chart re-render (anchoring, one beat). |
| **Perimeter — out** | Cues and jump marks (E07); deleting mandatory layers (danger flow — one sentence at most). |

**Prerequisites:** seeded server; two browser contexts (Marie, Leo) side by side at 960×1080 each for the `Split`
beats, one 1920×1080 context for the rest; *The Open Road* lead sheet open.

**Takes**

| Take | Surface | Beats asserted |
|---|---|---|
| `web-editor-tour` | WEB | song opens full-bleed; the tool pill visible; "Fit page" |
| `web-draw-tools` | WEB (Marie) | Highlight preset over "Sing loud" on p.1 (the seed already highlights "Capo 2" — avoid stacking) → Text "louder!" → Rect ▢ "Outline" around Verse 2 → Line, "Line" Dashed, "End" Arrow → Icon "Stamp icon" ⚠ → each object exists in the layer (API check) |
| `web-layers` | WEB (Marie) | Layers panel: "Form / sections" 👥 Band, "required" (checkbox disabled); "Conductor cues" "(conductor)" "required"; Marie's "My notes"; Sasha's "My notes" listed **off**; hide Marie's "My notes" → its marks vanish; show again |
| `web-realtime` | WEB ×2 | Leo (conductor) draws an ellipse on "Conductor cues" → appears in Marie's window within 2 s; Marie (admin, not conductor) cannot edit it |
| `web-undo` | WEB | draw → Ctrl+Z → gone |
| `web-anchor` | WEB | on the text chart, a highlight on a word → transpose → the highlight still on the same word |

**Storyboard**

| # | Time | Visual | On screen |
|---|---|---|---|
| 6.1 | 0:00–0:05 | `TitleCard` | "06 · Annotating together" |
| 6.2 | 0:05–0:27 | WEB `web-editor-tour`, `Callout`s on the tool pill and the rail | LT "canvas-first" |
| 6.3 | 0:27–1:27 | WEB `web-draw-tools` | LTs "Highlight" · "Text" · "Outline" · "Dashed" · "Arrow" · "Stamp icon" |
| 6.4 | 1:27–2:13 | `Diagram` (three layer bands stacked over the page) then WEB `web-layers` | LTs "shared" · "conductor · required" · "My notes" |
| 6.5 | 2:13–3:00 | `Split` WEB ×2 `web-realtime` | LT "live" · "read-only" |
| 6.6 | 3:00–3:20 | WEB `web-undo` | LT "Undo" |
| 6.7 | 3:20–3:50 | WEB `web-anchor` | LT "marks follow their words" |
| 6.8 | 3:50–4:00 | `EndCard` → "07 · Cues and jump marks" | — |

**Narration**

> **6.2** Open a song and the page fills the screen: the music first, the tools out of the way. [beat] The tool pill
> is at the top; layers and the list of annotations live in the rail on the side.
>
> **6.3** Marie wants the chorus louder. She picks the highlight preset and drags over "Sing loud". [beat] A text note
> next to it. [beat] An outline around the second verse. [beat] A line, dashed, with an arrow at the end, to point
> at the repeat. [beat] And from the stamp palette, a warning sign. [pause] Every mark has a colour, a width, a style;
> and every mark is saved the moment you lift the pen.
>
> **6.4** Where a mark goes matters more than how it looks. TroubaStack draws on layers, stacked over the page in
> three bands. [beat] Shared layers belong to the whole band — like "Form and sections", the song's structure. [beat] Conductor layers
> belong to the conductor. [beat] Any band layer can be marked "required" — like the form here, and the
> conductor's cues: then every player sees it, and nobody can hide it. [beat] And personal layers — "My notes" —
> are yours: only you draw on them, and your bandmates don't see them unless they choose to switch them on. [pause] The "Drawing on" chip always says where your next mark will land, and who will see it:
> the band, or just you. [beat] Any layer can be hidden from your own view with one click — except the required ones.
>
> **6.5** And it is live. Here are Marie and Leo, on two different computers. [beat] Leo, the conductor, circles the
> last chord on his conductor layer. [pause] It appears on Marie's screen at once. [beat] She can see it, and she cannot change it: the conductor's layer is read-only for everyone who isn't a conductor.
>
> **6.6** Made a mistake? Undo takes back your last change. [beat] And if someone else has changed that mark since,
> TroubaStack says so instead of silently overwriting their work.
>
> **6.7** One more thing about typed charts. Marks are attached to the words they were drawn on. [beat] Change the
> key, change the text size, switch to two columns — the page re-renders, and the highlight stays on its word.
>
> **6.8** Next: cues that tell each player what to grab, and jump marks.

---

### E07 — Cues and jump marks

| | |
|---|---|
| **Arc** | Studio · colour: Studio pink (stage payoff in yellow) |
| **Length** | 3:15 (narration 274 words ≈ 2.0 min of speech; the rest is watched action and cards) |
| **Surfaces** | WEB (cues editor, setlist rows, jump tool), TAB (cue flash, jump) |
| **Goal** | Players set their personal song cues; the leader places a jump mark and sees it become one tap on stage. |
| **Perimeter — in** | **"My cues"** ("Add a cue", icons, "Tint") → shown on the setlist rows → flashed on song entry on the tablet; **Jump mark** tool: "next: source" → "next: destination →", "Jump source placed — now place the destination it jumps to (Esc to cancel).", landmarks (segno, coda…), the tie, "Jumps to p.N" / "Jumped to from p.N", "→ p.1" / "← p.2" (cross-page: no dashed tie — the tie is drawn only when both ends are on one page), "Swap jump direction", one sign-and-colour pair per part; on stage: tap → "Go" → landing near the top with a brief full-screen flash; ⚙ "Skip the go-to popup". |
| **Perimeter — out** | Conductor cues as a *layer* (that is E06; said once to avoid the confusion). |

**Prerequisites:** seeded server; the jump-mark fixture **not** applied (the jump is placed on camera); a bake +
download for the tablet beats (done by the take, or reuse E08's bake); emulator connected as Marie.

**Takes**

| Take | Surface | Beats asserted |
|---|---|---|
| `web-my-cues` | WEB (Sasha) | **Amazing Grace** (the seed already gives her a bass cue on The Open Road) → "My cues" → "Color for new cue" Teal → "Add a cue" → keys icon → saved; setlist "Sat @ The Anchor" row shows it |
| `web-jump-place` | WEB (Leo) | lead sheet: "Jump mark" → segno near the end of p.2 (source) → segno at Verse 1 on p.1 (destination) → select the source: "Jumps to p.1" + "→ p.1"; select the destination: "Jumped to from p.2" + "← p.2". (The jump fixture of §4.2 is NOT applied for this episode) |
| `tab-cue-flash` | TAB | enter The Open Road → the cue flash (mic + red electric for Marie) visible ≥ 0.5 s |
| `tab-jump` | TAB | portrait, the source page → tap the segno → "Go" → the destination page with the arrival flash → `marks` "landed" |

**Storyboard**

| # | Time | Visual | On screen |
|---|---|---|---|
| 7.1 | 0:00–0:06 | `TitleCard` | "07 · Cues and jump marks" |
| 7.2 | 0:06–0:57 | WEB `web-my-cues` then TAB `tab-cue-flash` | LTs "My cues" · "Tint" |
| 7.3 | 0:57–2:18 | WEB `web-jump-place` (`Zoom` on both ends, the tie) | LTs "Jump mark" · "source" · "destination" · "Jumps to p.1" · "→ p.1" |
| 7.4 | 2:18–3:03 | TAB `tab-jump` | LT "Go" |
| 7.5 | 3:03–3:15 | `EndCard` → "08 · Setlists and baking" | — |

**Narration**

> **7.2** Before a song starts, every player has a different question: which guitar? Do I sing? Is it my solo?
> [beat] Song cues answer it. In a song, open "My cues", pick a colour, and add a cue — a microphone, an electric guitar, a keyboard. [beat] Your cues show under each song in the setlist. [pause] And on stage, the moment you
> enter the song, they flash big in the middle of the screen. One glance, and you know what to pick up. [beat] Cues
> are personal: each player sets their own. The conductor's marks on the score are something else — a layer, as we
> saw in the last episode.
>
> **7.3** Now, repeats. "Back to the segno", "jump to the coda" — on paper that means flipping pages with a hand you
> don't have. [beat] Pick the "Jump mark" tool. First, place the source: where the jump is made — here, at the end
> of the riff, "play four times". [beat] Then place the destination: back at verse one. [pause] Both ends get the same sign and the same colour. Select either end, and it tells you the way — "jumps to
> page one" — with a small arrow you can click to go there. [beat] Need it the other way round? "Swap jump
> direction". [beat] Each sign-and-colour pair is used once per part, so two jumps can never be confused.
>
> **7.4** On the tablet, the sign becomes a button. Tap it, and confirm with "Go". [pause] You land on the destination, placed near the top of the screen, and the screen flashes briefly so you know
> you've jumped. [beat] Trust it already?
> In settings, "Skip the go-to popup" makes it a single tap.
>
> **7.5** Next: putting the songs in order, and sending them to every tablet.

⟂ merge seam — the six-episode cut joins E07 and E08 here.

---

### E08 — Setlists and baking

| | |
|---|---|
| **Arc** | Studio → Stage · colour: Core blue |
| **Length** | 3:30 (narration 323 words ≈ 2.4 min of speech; the rest is watched action and cards) |
| **Surfaces** | WEB (Setlists, setlist detail, bake), TAB (Home update row, Concerts offers) |
| **Goal** | The leader builds a running order, bakes it, and every tablet picks up the new version; the viewer understands what a bake and a rev are. |
| **Perimeter — in** | Setlists tab: "New concert" (Name, "Venue (optional)"); setlist detail: "Add a song from the band library…" → "Add to order", drag to reorder, per-item ✎ (tooltip "Edit key / tempo / notes": Key, Tempo, "Performance note"; "transpose chords" on a text chart; "Preview chart" opens the saved result in a new tab), "Add an intermission" (no number, own page), "★ Bench · on call" (the ★ button, tooltip "Move to the bench (on call, outside the running order)"); "Bake setlist" → the dialog ("Baking with: …", default layers) → "Baking — song N of M: <title>" → the concert's bake row updated in place ("👥 Band · Rev N · K songs · by <who>" + date); "Download .tstage" / "Download PDF" (one line each); on the tablet: Home "Sat @ The Anchor — new version" → "Update" with progress → "Nothing to update"; Concerts "— update to rev N"; "Freeze (no updates)" / "Pin this version" (one line: keep a version for the gig). |
| **Perimeter — out** | Live mode (E11); band-wide bundle internals (said as "one file for the whole band, each tablet shows its player"). |

**Prerequisites:** seeded server, plus a baked rev 1 already on the emulator (so the bake produces an *update*);
emulator connected as Marie.

**Takes**

| Take | Surface | Beats asserted |
|---|---|---|
| `web-setlist-build` | WEB | "New concert" "Friday at the Mill" → add three songs → drag The Open Road last → "Add an intermission" (appended at the end) → drag it between 2 and 3 → numbers 1, 2, (Intermission), 3 → Greensleeves ★ (Move to the bench). *Illustrative only: this concert is not baked; the bake and update beats use* **Sat @ The Anchor** |
| `web-item-override` | WEB | Sat @ The Anchor → House → ✎ → Key "Bm" (seeded), "transpose chords" ✓, "Performance note" → Save → "Preview chart" (new tab) shows Bm chords |
| `web-bake` | WEB | Sat @ The Anchor (rev 1 already baked by the fixture) → "Bake setlist" → dialog → "Bake" → "Baking — song n of 4: …" seen → the bake row reads "Rev 2" and "Download .tstage (rev 2)" |
| `tab-update` | TAB | Home: "… — new version" → "Update" → "Downloading x / y MB" → "Installing…" → "Nothing to update"; concert opens on rev 2 (title card / Bakes row "rev 2") |

**Storyboard**

| # | Time | Visual | On screen |
|---|---|---|---|
| 8.1 | 0:00–0:06 | `TitleCard` | "08 · Setlists and baking" |
| 8.2 | 0:06–1:10 | WEB `web-setlist-build` | LTs "New concert" · "Running order" · "Intermission" · "★ Bench · on call" |
| 8.3 | 1:10–1:41 | WEB `web-item-override` | LTs "Key" · "transpose chords" |
| 8.4 | 1:41–1:57 | `Diagram`: songs + layers → bake oven → one `.tstage` file, "rev 2" stamp | LT "bake" |
| 8.5 | 1:57–2:36 | WEB `web-bake` | LTs "Bake setlist" · "Rev 2" |
| 8.6 | 2:36–3:18 | TAB `tab-update` | LTs "Update" · "rev 2" |
| 8.7 | 3:18–3:30 | `EndCard` → "09 · On stage: reading" | — |

**Narration**

> **8.2** A gig is a setlist: songs, in order. On the Setlists tab, "New concert", a name, a venue if you like.
> [beat] Add songs from the band's library, and drag them into the order you will play them. [beat] Need a break? "Add an intermission", and drag it into place: it gets its own page on stage, and it doesn't take a number, so the numbering stays the one
> on your paper setlist. [beat] And songs you might play — the encore you're not sure about — go to the bench. They
> travel with the concert, they can be reached on stage, but they stay out of the running order.
>
> **8.3** Each song in a setlist can be adjusted for this gig only. House of the Rising Sun goes up a tone tonight:
> key B minor, and a note for the band. [beat] For a typed chart, tick "transpose chords", and the chords themselves
> are rewritten when the concert is baked.
>
> **8.4** Baking is the moment the setlist becomes a concert. [beat] TroubaStack takes every song, in order, with its
> annotations, and flattens them into one file: every page as an image, every layer as a transparent sheet on top.
> One file for the whole band — each tablet shows its own player's part. [beat] Every bake gets a number, the rev,
> so the band can check they all have the same one.
>
> **8.5** "Bake setlist". [beat] The dialog asks which layers should be switched on by default when the concert
> opens. Required layers are always on. [pause] The bake runs, song by song… [beat] and the concert's row now shows the new rev, who baked it, and when.
>
> **8.6** On the tablets, there is nothing to send. Home simply says: new version. One tap on "Update"… [pause]
> and the concert is current. Players who were offline see "new version" the next time they connect. [beat] For the gig
> itself, you can freeze a concert on a tablet, or pin a version, so nothing changes under you on the night.
>
> **8.7** Next: what the tablet does with it — on stage.

---

### E09 — On stage: reading

| | |
|---|---|
| **Arc** | Stage · colour: Stage yellow |
| **Length** | 3:45 (narration 329 words ≈ 2.4 min of speech; the rest is watched action and cards) |
| **Surfaces** | TAB (landscape and portrait) |
| **Goal** | A player can read a concert comfortably in any light and any position, and find any song in two taps. |
| **Perimeter — in** | Opening a concert (Home "Resume «…»" or the TroubaStage screen); "Who are you?" again in one beat (your part, your layers); immersive reading — **any tap shows or hides the chrome**, it hides itself; the chrome: ☰ song drawer, title card "Song 2/4 · 3–4/12", ✎, ⚙, ✕ (the only exit), ‹ ›; the meta strip on a song's first page (performance note · key; the tempo shows in the song drawer); page turns by swipe; **Reading mode** "Page" / "Width" / "Scroll" (Scroll: one column per song, swipe sideways to the next song); **two-up** facing pages in landscape + Page, a turn moves the whole spread, a spread never mixes two songs; the edge-of-concert glyph; the **song drawer** ("Songs", numbered rows, "On call", intermission divider); **Colour**: Normal · Warm · Night · Amber (Night/Amber invert the page, ink re-tinted to stay readable); Layers dialog ("Layers — <song>", the current song's layers, "required"; "Rehearsal notes · this device" only when this song has a note on this tablet) in one beat; the reading position is remembered. |
| **Perimeter — out** | Pedals, metronome, clock (E10); notes (E11). |

**Prerequisites:** emulator, connected, *Sat @ The Anchor* (rev with the jump fixture and an intermission from
E08) downloaded; identity Marie; demo mode; colour "Normal" at start.

**Takes**

| Take | Surface | Beats asserted |
|---|---|---|
| `tab-open-concert` | TAB portrait | Home "Resume «Sat @ The Anchor»" → page shown; tap → chrome visible; wait → chrome hidden |
| `tab-reading-modes` | TAB portrait | ⚙ → "Reading mode" Width → page fills width, scrolls; Scroll → continuous column; swipe sideways → next song; back to Page |
| `tab-twoup` | TAB landscape | rotate → two facing pages (a spread label like "2–3/…" — fixture-dependent); swipe → next spread; on the last spread, one more swipe → "›\|" glyph |
| `tab-drawer` | TAB landscape | ☰ → "Songs" with numbered rows + intermission divider + "On call" → tap song 3 → it opens |
| `tab-colours` | TAB landscape | ⚙ "Colour" tapped ×3 → Warm, Night, Amber (pixel checks on the page background) |

**Storyboard**

| # | Time | Visual | On screen |
|---|---|---|---|
| 9.1 | 0:00–0:06 | `TitleCard` | "09 · On stage: reading" |
| 9.2 | 0:06–0:45 | TAB `tab-open-concert`, `Callout`s on ☰ · title · ✎ · ⚙ · ✕ | LTs "Resume" · "tap to show / hide" |
| 9.3 | 0:45–1:30 | TAB `tab-reading-modes` | LTs "Page" · "Width" · "Scroll" |
| 9.4 | 1:30–2:11 | TAB `tab-twoup` (`DeviceFrame` rotates with the take) | LT "facing pages" |
| 9.5 | 2:11–2:41 | TAB `tab-drawer` | LTs "Songs" · "On call" |
| 9.6 | 2:41–3:26 | TAB `tab-colours` (a slow cross-fade between the four, then live taps) | LTs "Normal" · "Warm" · "Night" · "Amber" |
| 9.7 | 3:26–3:45 | TAB Layers dialog still + `EndCard` → "10 · On stage: hands-free" | LT "your layers" |

**Narration**

> **9.2** On Home, "Resume" reopens the last concert exactly where you left it — even if the tablet restarted.
> [pause] On stage, the music takes the whole screen. [beat] Tap anywhere to bring back the controls, tap again to
> hide them — and if you don't, they hide by themselves. [beat] At the top: the song list, the title and your
> position, the note pencil, the settings, and the red cross: the only way out of a concert. [beat] On a song's first page, under the title: the note your band left for tonight, and the key.
>
> **9.3** Swipe to turn the page. [beat] How the page fits the screen is up to you. "Page" shows a whole page.
> "Width" fills the width and scrolls down the page — bigger notes on a small tablet. [beat] "Scroll" lays out each
> song as one long column; scroll down to read, swipe sideways to move to the next song.
>
> **9.4** Turn the tablet sideways in Page mode, and you get two facing pages, like an open book. [beat] One turn
> moves the whole spread. A spread never mixes two songs: a new song always starts on the left. [beat] Try to turn past the last page, and this sign tells you there is nothing further — so a turn that does
> nothing never looks like a broken one.
>
> **9.5** The song list jumps anywhere in the concert. Songs are numbered in running order; the intermission and
> the songs on the bench are there too, unnumbered. [beat] Tap one, and you are there.
>
> **9.6** Stages are dark, rehearsal rooms are not. [beat] "Colour" steps through four schemes, darker and back. Normal, the white
> page. Warm, a softer paper. [beat] Night turns the page dark, and Amber turns it dark with amber ink, which keeps
> your eyes used to the dark. [beat] In Night and Amber, annotation colours are adjusted, so the band's red stays a red you can read.
>
> **9.7** And the Layers dialog shows which layers you see in the current song — your part, the band's, the required ones
> — and lets you change it for this song only. [beat] Next: playing without using your hands.

⟂ merge seam — the six-episode cut joins E09 and E10 here (*On stage*).

---

### E10 — On stage: hands-free

| | |
|---|---|
| **Arc** | Stage · colour: Stage yellow |
| **Length** | 3:30 (narration 311 words ≈ 2.3 min of speech; the rest is watched action and cards) |
| **Surfaces** | TAB (emulator) + **HW insert** (real tablet + real pedal, demo concert only) |
| **Goal** | A player sets up a pedal (keyboard-style or MIDI), uses the silent count-in and the clock, and trusts that nothing can throw them out mid-song. |
| **Perimeter — in** | Built-in page turns: Bluetooth pedals that act as a keyboard work as-is (PageDown/arrows/Space), and the **volume keys** turn pages (by spread in two-up); **Parameters → "Foot pedal"**: "MIDI pedal: not connected" → "Connect"; "Learn" → "Learning Next page — press a pedal button." ("nothing received yet") → press → "Learned." → the row reads "learned: …" (the raw token, e.g. "MIDI:192,3"); "Forget learned buttons"; the **metronome capsule**: tap = a silent two-bar count-in in the song's metre (the pulsing border, the big beat number restarting each bar, amber downbeat; starting it hides the chrome), "∞" set **before** starting keeps it going, no sound ever; **Chronometer** "Start"/"Pause"/"Reset" and **"Show clock"** (Analog/Digital/Both), bottom-right; **"Lock swipe"** ("Stop an accidental scroll from changing the song — use ‹ › or a pedal." — it also stops swipe page turns in Page and Width); **Back does nothing in a concert** — an edge swipe meant as a page turn can't leave the song; ✕ leaves; Back still closes the drawer and sheets. |
| **Perimeter — out** | Pedal hardware recommendations (none on film; "any pedal that acts as a keyboard, or a BLE-MIDI controller"). |

**Prerequisites:** emulator as in E09, **reading mode Page** (E10's lock-swipe beat switches to Scroll — the take restores Page at its end); House of the Rising Sun (6/8, ♩.=72) for the count-in; **HW insert**: VLL's tablet with **only the demo concert opened on screen**, a BLE-MIDI pedal paired;
recorded with `adb screenrecord` (no foot B-roll for now, §9 D5 — a `Callout` "foot press" marks the moment); the frames go through the
privacy scan (§4.5) — the insert never shows Home, the concert list or the Notes tab.

**Takes**

| Take | Surface | Beats asserted |
|---|---|---|
| `tab-volume-keys` | TAB | `KEYCODE_VOLUME_DOWN` → next spread (label changes) |
| `tab-learn-midi` | **HW** | Parameters → Foot pedal → "Connect" → "MIDI pedal: connected" → "Learn" (Next page) → "Learning Next page — press a pedal button." / "nothing received yet" → foot press → "Learned." → row "learned: MIDI:…" → back in the concert, foot press → page turns |
| `tab-count-in` | TAB | House (6/8) → metronome → chrome hides, border pulses, numbers 1…6 twice, stops by itself; then ∞ ON first → metronome → keeps running across a page turn |
| `tab-clock-chrono` | TAB | ⚙ "Chronometer" "Start" → readout bottom-right running; "Show clock" on, "Both" |
| `tab-lock-swipe` | TAB | Scroll mode, "Lock swipe" on → sideways swipe does not change the song; ‹ › still do → back to **Page** mode at the end (E11 needs it) |
| `tab-back-guard` | TAB | 3 left-edge + 3 right-edge back gestures → same page label; ☰ open → Back → drawer closes, concert stays; ✕ → Home |

**Storyboard**

| # | Time | Visual | On screen |
|---|---|---|---|
| 10.1 | 0:00–0:06 | `TitleCard` | "10 · On stage: hands-free" |
| 10.2 | 0:06–0:31 | TAB `tab-volume-keys` (a key-press overlay) | LT "volume keys" |
| 10.3 | 0:31–1:26 | HW `tab-learn-midi` (+ a "foot press" `Callout` on each press) | LTs "Foot pedal" · "Learn" · "Learned." |
| 10.4 | 1:26–2:12 | TAB `tab-count-in` | LTs "count-in" · "∞" |
| 10.5 | 2:12–2:36 | TAB `tab-clock-chrono` | LTs "Chronometer" · "Show clock" |
| 10.6 | 2:36–2:55 | TAB `tab-lock-swipe` | LT "Lock swipe" |
| 10.7 | 2:55–3:22 | TAB `tab-back-guard` with an edge-swipe arrow overlay | LT "✕ is the only way out" |
| 10.8 | 3:22–3:30 | `EndCard` → "11 · The rehearsal loop" | — |

**Narration**

> **10.2** Your hands are on your instrument, so pages need to turn without them. [beat] Most Bluetooth page-turn
> pedals act like a keyboard, and TroubaStage understands them out of the box. [beat] No pedal? The tablet's volume
> keys turn pages too — by a whole spread in two-page mode.
>
> **10.3** Some pedals speak MIDI instead, or have more buttons than the defaults cover. [beat] In Parameters,
> under "Foot pedal", connect the MIDI pedal, then tap "Learn" next to "Next page" — and press the button you want.
> [pause] "Learned." — and the row shows exactly what the pedal sends. [beat] Same for "Previous page". From now on, your foot turns the page.
>
> **10.4** The metronome capsule at the top gives you a silent count-in. Tap it: the page border pulses, and a big number counts two bars in the song's metre — in six-eight, one to six,
> twice — and stops by itself. [beat] The
> downbeat glows amber, so you see the one without counting. [beat] Switch on the infinity sign before you start, and the beat keeps going through the song. [beat] It never makes a sound.
>
> **10.5** In settings, a chronometer times the set — it keeps running even if the app is closed. And "Show clock"
> puts the time of day in the corner: analog, digital, or both.
>
> **10.6** In Scroll mode, a sideways swipe changes the song. If you'd rather it couldn't, "Lock swipe" turns swiping off —
> in every reading mode; the arrows, the pedal and the volume keys still work.
>
> **10.7** And one thing you will never do by accident: leave the concert. [beat] On many tablets, a swipe from
> the edge of the screen means "back" — and a page turn that starts near the edge used to throw you out of the
> song. [beat] Inside a concert, back does nothing. You stay on your page. [beat] It still closes the song list or
> the settings sheet. To leave the concert, there is one way: the red cross.
>
> **10.8** Next: rehearsals — where the music keeps changing.

---

### E11 — The rehearsal loop

| | |
|---|---|
| **Arc** | Rehearsal · colour: the stack |
| **Length** | 4:30 (narration 408 words ≈ 3.0 min of speech; the rest is watched action and cards) |
| **Surfaces** | `Split` WEB + TAB, TAB, WEB |
| **Goal** | The band understands the two loops that make rehearsals work: the conductor edits and the tablets follow (live mode), and the players scribble and Studio receives (rehearsal notes). |
| **Perimeter — in** | **Rehearsal live mode** (Studio, admin): "Arm live mode · auto-bakes for 3 h" / "Go live (rehearsal)", the "LIVE" badge, every new mark on its songs auto-bakes ~8 s after the last one (chart, file and metadata edits do not), it switches itself off after 3 hours; on the tablet: ⚙ **"Auto-update"** ("Apply new bakes as they arrive", 👤 Just for you) → "Updated to rev N" with the page unmoved; **rehearsal notes**: ✎ → "Enter note mode?" ("Touch will draw, not turn pages. Pages still turn with a pedal or the volume keys.") → pencil, eraser, widths, colours → "✎ notes" badge → "Clear page" ("Clear this page?"; "Clear which page?" when both facing pages have a note; disabled when none does) → the **✓** ("Finish notes"); the Notes tab (TroubaStage screen → Bakes \| Notes): "Send all (N)" → "1 sent ✓" (a single-note Send says "Sent to Studio ✓"), per-concert "Send all", "Sent", "Clear sent (N)" ("Studio keeps its copies"); in Studio: "✎ N" on the song list, the "Rehearsal notes (N)" underlay, "Go to page N", the viewer ("Look at this note on the page it was drawn on"), "Done, remove"; **auto-upload**: ⚙ "Auto-upload notes to Studio" "ON for 3 h · notes send as you draw" (up to 3 h; off when you leave the concert, when a new bake arrives, or on a conflict), the banner "AUTO-UPLOAD ON — your notes are sending to Studio", "Removed here and in Studio"; the principle: a note is a scribble, *recopied* into a real annotation — it never becomes one by itself. |
| **Perimeter — out** | Notes ageing ("Older than the current bake…") — one line at most. |

**Prerequisites:** seeded server with a baked *Sat @ The Anchor*; tablet reading mode **Page** (note mode refuses in Scroll with "Notes: switch to Page or Width"); emulator connected as Sasha (member) with the
concert; WEB as Leo (conductor) for live mode — **live mode needs an admin to arm it**, so Marie arms it in a short
beat first; the rehearsal-note beats send to the isolated server.

**Takes**

| Take | Surface | Beats asserted |
|---|---|---|
| `web-arm-live` | WEB (Marie) | setlist row ⋯ → "Arm live mode · auto-bakes for 3 h" → the row's pulsing "Live" chip; on the concert page the "LIVE" chip and banner |
| `split-live-edit` | WEB (Leo) + TAB (Sasha, Auto-update on) | Leo adds "breathe here" on the conductor layer → ~8 s later the concert's bake row shows a new rev → within ~30 s (the tablet polls every 15 s) the tablet shows "Updated to rev N" and the mark, **page label unchanged** |
| `tab-note-mode` | TAB | ✎ → "Enter note mode?" → "Enter" → handwriting (motionevent paths) in red → ✓ → "✎ notes" badge |
| `tab-notes-send` | TAB | ✕ → TroubaStage → Notes → "Send all (1)" (title row) → "1 sent ✓" → "Sent" group 1 |
| `web-note-viewer` | WEB (Leo) | Overview "✎ 1" on the song → open → "Rehearsal notes (1)" underlay on → "Go to page 1" → viewer → recopy as a real text mark → "Done, remove" |
| `tab-auto-upload` | TAB | ⚙ "Auto-upload notes to Studio" on → banner under the menu → draw → server note count +1 without a Send → "Clear page" → "Removed here and in Studio" |

**Storyboard**

| # | Time | Visual | On screen |
|---|---|---|---|
| 11.1 | 0:00–0:06 | `TitleCard` | "11 · The rehearsal loop" |
| 11.2 | 0:06–0:22 | `Diagram`: two loops — Studio → tablets (blue arrow), tablets → Studio (yellow arrow) | — |
| 11.3 | 0:22–0:45 | WEB `web-arm-live` | LTs "Arm live mode" · "3 h" |
| 11.4 | 0:45–1:30 | `Split` `split-live-edit` | LTs "Auto-update" · "Updated to rev N" |
| 11.5 | 1:30–2:22 | TAB `tab-note-mode` | LTs "Enter note mode?" · "Clear page" · "✓" |
| 11.6 | 2:22–2:49 | TAB `tab-notes-send` | LTs "Send all" · "Sent" · "Clear sent" |
| 11.7 | 2:49–3:41 | WEB `web-note-viewer` | LTs "✎ 1" · "Go to page 1" · "Done, remove" |
| 11.8 | 3:41–4:19 | TAB `tab-auto-upload` | LTs "Auto-upload" · "ON for 3 h" · "Removed here and in Studio" |
| 11.9 | 4:19–4:30 | `EndCard` → "12 · Keeping it safe" | — |

**Narration**

> **11.2** Rehearsals are where the music changes the most. TroubaStack runs two loops for them: the conductor's
> changes reaching every tablet, and the players' scribbles reaching Studio.
>
> **11.3** First loop. Normally, nothing reaches the tablets until someone bakes. During a rehearsal, an admin can
> arm live mode on the concert. [beat] For the next three hours, every new mark on its songs is baked automatically, a few seconds after the
> last one. [beat] Then it switches itself off — so a live mode you forgot about can never
> follow you to the gig.
>
> **11.4** On the tablet, each player decides: in settings, "Auto-update". [beat] Leo writes "breathe here" on his
> conductor layer… [pause] shortly after, a new rev — and on Sasha's tablet, "Updated to rev" and its number. The mark
> is there. And she is still on her page: an update never moves you.
>
> **11.5** Second loop. In a rehearsal, players need to scribble — fast, on the page, without thinking about layers.
> [beat] Tap the pencil, and confirm: in note mode, touch draws instead of turning pages; the pedal still turns
> them. [pause] Write. A pencil, an eraser, three widths, four colours. [beat] "Clear page" wipes the whole note on this page — and if both facing pages have notes, it asks which one. [beat] When you're done, the check mark. The page shows a small
> "notes" badge from now on.
>
> **11.6** These notes stay on the tablet — they even work offline. When you're connected, send them: in the Notes
> tab, "Send all" — it says how many were sent. [beat] Sent notes move to their own group. "Clear sent" removes them from the tablet; Studio keeps
> its copies.
>
> **11.7** In Studio, the song now shows a pencil and a count. [beat] Open it: the note appears under the score as a
> reference — "Go to page one" takes you right there. The viewer even shows it on the exact page it was drawn on,
> as that page looked at the time. [pause] A rehearsal note is a scribble, not an annotation: someone recopies it
> properly — here Leo turns it into a clean text mark — and then "Done, remove".
>
> **11.8** And when the page behind you won't change for a while, you can skip the "Send" step entirely. "Auto-upload notes to Studio" turns on for up to three hours — until you leave the concert or a new version
> arrives: a banner under the menu says so, and every note goes to Studio as you draw. [beat] While it's on, clearing a page clears it in Studio too — and the message says exactly that. [beat] If
> a send fails, the banner says that as well, and keeps retrying.
>
> **11.9** Last episode: keeping all of this safe.

---

### E12 — Keeping it safe

| | |
|---|---|
| **Arc** | Running it · colour: Core blue |
| **Length** | 3:30 (narration 324 words ≈ 2.3 min of speech; the rest is watched action and cards) |
| **Surfaces** | TERM, WEB |
| **Goal** | The leader knows exactly what to back up, how to restore, how to take the whole band elsewhere, and how to update the server — and that the band's music never depends on the project. |
| **Perimeter — in** | The one thing to back up: the data volume (stop the server, the documented `docker run … alpine tar` one-liner on the compose volume **`deploy_troubadata`**, start — `deploy/backup.sh` covers the plain-directory case only); restore onto a fresh volume; **Export band** (Settings → "Export band (.zip)", the `.tband`: songs, files, annotations, setlists) and **"Import band…"** with the per-member choice ("Invite", "Create account", "Skip"); updating: `docker compose pull && docker compose up -d`, `/api/version` shows the exact build; bake retention (`TROUBA_BAKE_KEEP_REVS`, `troubacore gc`, a final-locked rev is never pruned); "Forgot the only admin's password?" `troubacore reset-password`; the tablets keep working offline whatever happens to the server. |
| **Perimeter — out** | PostgreSQL / git stores; systemd details. |

**Prerequisites:** the E02 host (disposable), with the seeded band; TERM for backup/restore/update; WEB for export/import
(a second, empty isolated server receives the import).

**Takes**

| Take | Surface | Beats asserted |
|---|---|---|
| `term-backup` | TERM (real) | `docker compose stop troubacore` → `docker run --rm -v deploy_troubadata:/data -v "$PWD":/backup alpine tar czf …` → `.tgz` exists → `start` |
| `term-restore` | TERM (real) | fresh volume → untar → `up -d` → healthy → the band is there (curl `/api/bands` as Marie) |
| `web-export-import` | WEB ×2 servers | Settings → "Export band (.zip)" → file → server 2 "Import band…" → member choices → "Import complete" → songs present |
| `term-update` | TERM | `docker compose pull && docker compose up -d` → `curl …/api/version` shows a commit |

**Storyboard**

| # | Time | Visual | On screen |
|---|---|---|---|
| 12.1 | 0:00–0:08 | `TitleCard` | "12 · Keeping it safe" |
| 12.2 | 0:08–0:30 | `Diagram`: the data volume (from E02) | LT "back up this" |
| 12.3 | 0:30–1:20 | TERM `term-backup` + `term-restore` (cut) | LTs "backup" · "restore" |
| 12.4 | 1:20–2:20 | WEB `web-export-import` | LTs "Export band" · "Import band…" |
| 12.5 | 2:20–2:55 | TERM `term-update` | LTs "update" · "/api/version" |
| 12.6 | 2:55–3:15 | GFX: retention + reset-password, two lines of text each | LTs "keep revs" · "reset-password" |
| 12.7 | 3:15–3:30 | `EndCard` (series end): the map of all twelve, the project URL, `CreditsCard` | — |

**Narration**

> **12.2** Your band's music lives in one place: the server's data volume. Everything else — the program, the
> image, even this repository — can be downloaded again. That folder can't. So it is the one thing to back up.
>
> **12.3** Stop the server for a moment, so nothing is half-written, archive the volume with tar, and start it
> again. A few seconds of downtime, one file. [beat] To restore, unpack that file into a fresh volume and start the
> server: the band is back, exactly as it was.
>
> **12.4** You can also take a whole band with you, to another server — or just keep a copy you can read. In the
> band's Settings, "Export band". [beat] The file holds the songs, their files, every annotation and every setlist.
> [beat] On another server, "Import band", and decide for each member: invite them, create their account, or skip
> them. [pause] "Import complete".
>
> **12.5** Updating TroubaStack is two commands: pull the new image, start it again. Your data stays in its volume.
> [beat] The server tells you exactly which build it is running, at slash A-P-I slash version.
>
> **12.6** Two last things. Live mode can create many concert versions during a rehearsal: set a retention number,
> run the clean-up command after the rehearsal, and only the latest few are kept — never a version you locked for a
> gig. [beat] And if the only admin forgets their password: stop the server, run one command, and it prints a
> one-time reset link, valid for a day.
>
> ⚠ **Doc bug found while checking (2026-10-08):** `deploy/README.md` documents `docker compose exec troubacore
> troubacore reset-password <user>`, which runs against the *live* server — on the file backend the running server
> never sees the token and overwrites it on its next flush. The working sequence is stop → `docker compose run --rm
> troubacore reset-password <user>` → start. The GFX must show the working sequence; the README needs the fix.
>
> **12.7** And whatever happens to the server, the tablets keep every concert they downloaded, and keep working
> offline. [pause] That's TroubaStack: your band's music, on a box you own, from the rehearsal room to the stage.
> Thanks for watching.

---

## 9. Decisions

Ruled by VLL on 2026-10-09 unless marked **open**.

| # | Decision | Ruling |
|---|---|---|
| D1 | **Narrator voice** | **A soft male voice**, English, AI-generated. Engine still chosen by ear at the pilot: E03's first minute rendered with a soft male voice from each of Kokoro-82M (local, free — e.g. `am_michael`, `bm_george`), ElevenLabs and OpenAI `gpt-4o-mini-tts`, side by side. |
| D2 | **Twelve or six episodes** | **Twelve.** (The merge seams stay marked; nothing is planned on them.) |
| D3 | **Languages** | **English** narration and captions. |
| D4 | **Music** | **Soft, original, composed for the series** — §3.4 (tools installed: fluidsynth + FluidR3_GM, sox, LilyPond, MuseScore). |
| D5 | **Real-world footage** | **None for now** — screen only. E10's pedal moments are marked with a `Callout`. |
| D6 | **Publishing** | **A YouTube playlist**, embedded on the project site (probably). Unlisted first, public after each episode's final gate. The `.webm` site copy stays in the build in case. |
| D7 | **E02's live server on film** | **open** — default: a throwaway VPS + throwaway domain for one day (a real Let's Encrypt padlock). |
| D8 | **Series name** | **open** — VLL has no idea yet. Candidates: *"TroubaStack — From rehearsal room to stage"* · *"Twelve bars of TroubaStack"* · *"The band book"* · *"Off the stand"* · just *"TroubaStack — episode N"*. Needed only at publish (title cards read the name from one config value). |
| D9 | **Vertical cuts** | **Yes** — one per episode, §3.5. |
| D10 | **Demo content** | **The demo songs**, improved first — §5.4 (*DEMO-CONTENT*, not dispatched). |

---

## 10. Appendix

### 10.1 The narrator's glossary (said this way, always)

| Say | Means | Never say |
|---|---|---|
| TroubaStack | the whole product | "the platform", "the stack" |
| TroubaStudio / Studio | the web editor | "the web app", "the backend" |
| TroubaStage / Stage, "the app" | the tablet presenter | "the client", "the player" |
| TroubaCore / "the server" | the Go server | "the cloud", "the API" |
| band | the group, and its privacy boundary | "workspace", "team", "organisation" |
| admin · conductor · member | the three membership roles | "owner", "user" |
| song · file · chart | a repertoire entry · one PDF/image/chart in it · a chart typed as text | "document", "asset" |
| annotation, mark | something drawn on the music | "object", "shape" (except a rectangle is a "box") |
| layer — shared · conductor · personal ("My notes") | where marks live | "zone" |
| required | a layer nobody can hide | "mandatory" (internal) |
| song cue · "My cues" | a player's icons for a song | "tag", "badge" |
| jump mark · source · destination | a repeat made tappable | "link", "hyperlink", "anchor" |
| setlist · running order · intermission · bench, "on call" | the program | "playlist", "queue" |
| bake · rev · concert | publishing a setlist · its version number · the baked result | "build", "export", "release", "version 7" (say "rev seven") |
| live mode | Studio's auto-bake window | "auto-publish" |
| Auto-update | the tablet's opt-in to live revs | "sync" |
| rehearsal note · note mode · the check mark | the tablet scribble | "annotation" (it isn't one) |
| auto-upload | the tablet's 3-hour note window | "arm" (the UI avoids the verb on the tablet — glossary D28) |

### 10.2 Pronunciation lexicon (`video/tts/lexicon.yaml`)

| Written | Spoken (for the TTS) |
|---|---|
| TroubaStack · TroubaStudio · TroubaStage · TroubaCore | "TROO-bah-stack" · "TROO-bah-stoo-dee-oh" · "TROO-bah-stage" · "TROO-bah-core" |
| troubadour · joglar | "TROO-bah-door" · "zho-GLAR" |
| segno · coda | "SEN-yo" · "KOH-dah" |
| `.tstage` · `.tband` | "dot T stage" · "dot T band" |
| Sat @ The Anchor | "Saturday at The Anchor" |
| 4/4 · 6/8 · 3/4 | "four-four" · "six-eight" · "three-four" |
| ♩=92 | "ninety-two beats per minute" |
| Bm | "B minor" |
| rev 7 | "rev seven" |
| `docker compose up -d` | "docker compose up, dash d" |
| `/api/version` | "slash A-P-I slash version" |
| HTTPS · QR · PDF · VPN | spelled letter by letter |

### 10.3 Credits card (verbatim, from `NOTICE`)

Shown in every episode that shows the demo charts (in practice all but E02 and E12):

> *Greensleeves* (traditional; music: public domain). Typeset edition © 2014 David Kastrup, Mutopia-2014/03/10-1943,
> The Mutopia Project (mutopiaproject.org) — CC BY-SA 4.0.
>
> *Canon in D*, J. Pachelbel (music: public domain). Typeset edition © 2015 Michael Fischer v. Mollard,
> Mutopia-2015/09/02-2047, The Mutopia Project — CC BY 4.0. *(only where the orchestra appears)*
>
> Other demo charts: original or public domain. Music: *(D4)*. Narration voice: *(D1)*.

### 10.4 Narration budget check

At ~145 words per minute plus the scripted pauses, each episode's narration fits its target length (word counts
measured on §8 at commit time; the TTS timings of the pilot will recalibrate the rate):

| Ep | Words | Speech | Target (speech ÷ 0.72 + 20 s cards) |
|---|---|---|---|
| E01 | 314 | 2.3 min | 3:30 |
| E02 | 382 | 2.8 min | 4:15 |
| E03 | 276 | 2.0 min | 3:15 |
| E04 | 271 | 2.0 min | 3:00 |
| E05 | 312 | 2.3 min | 3:30 |
| E06 | 344 | 2.6 min | 4:00 |
| E07 | 274 | 2.0 min | 3:15 |
| E08 | 323 | 2.4 min | 3:30 |
| E09 | 329 | 2.4 min | 3:45 |
| E10 | 311 | 2.3 min | 3:30 |
| E11 | 408 | 3.0 min | 4:30 |
| E12 | 324 | 2.3 min | 3:30 |

*Why ÷ 0.72:* a screen tutorial keeps ~28 % of its running time for the viewer to watch an action happen (a pull, a bake, a gesture) without talk; the pilot measures the real ratio and the targets are re-derived from it.
