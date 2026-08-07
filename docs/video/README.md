# docs/video — DEMO-VID walkthrough

Assets for the TroubaShare walkthrough video (plan:
[`../tasks/DEMO-VID-walkthrough-video-plan.md`](../tasks/DEMO-VID-walkthrough-video-plan.md)).

- **[`script.md`](script.md)** — the finalized per-scene narration (one block = one TTS
  segment). Web scenes 0–14, app scenes 15–21.

## Part B — web walkthrough recording (this lane)

The walkthrough **builds The Troubadours live, on camera, from an empty server** (it does not
drive pre-seeded data): register the members, create the band, invite them, add "The Open Road"
and type its chart, tag instrument cues, mark the capo, toggle a layer, transpose, build the
setlist and bake — converging to the demo you can log into (`marie` / `demo`). Every mark is
narrated with the *reason* a real band would add it.

- `web/studio/playwright.walkthrough.config.ts` — recording config (`video: on`, 1920×1080).
  Runs an **isolated** empty TroubaCore on `:8090` (a fresh data dir each run) + its own Vite on
  `:5273` with HMR off — so it never touches the persistent `:8080` demo.
- `web/studio/walkthrough/global-setup.ts` — seeds **only the orchestra** (`seed -only
  orchestra`, real Mutopia editions) for the closing "at orchestra scale" reveal; The Troubadours
  is built live.
- `web/studio/walkthrough/walkthrough.spec.ts` — the paced tour. `beat(seconds)` holds each
  frame for its narration; best-effort steps are wrapped in `soft()`. The two required beats —
  the capo mark (green highlight + a ⚠ stamp + a "capo on!" note) and the layer show/hide toggle
  — are hard-asserted. Produces a ~180s 1920×1080 video.

Produce the video:

```sh
cd web/studio && npx playwright test -c playwright.walkthrough.config.ts
# → test-results/.../video.webm  (1920×1080; a generated artifact, git-ignored)
```

**Follow-ups:** scenes 15–21 are the mobile app (Part C); the per-scene dwell times are re-timed
to the final TTS lengths in the audio-first assembly pass (Part D).

## Part D — voiceover + assembly (this lane)

- `tools/synth.py` — parses `script.md`, synthesizes one WAV per scene with **Piper TTS**, and
  writes `output/narration/timings.json` (scene → seconds).
- `tools/assemble.sh` — joins the web-scene narration, time-aligns the silent walkthrough
  recording to it, prepends a title card, and muxes → `output/walkthrough-web.mp4`.

Produce it (Piper in a venv + a voice model, e.g. `en_US-lessac-medium`):

```sh
python3 docs/video/tools/synth.py --piper <piper> --voice <voice.onnx> --out docs/video/output/narration
cd web/studio && npx playwright test -c playwright.walkthrough.config.ts   # the silent recording
docs/video/tools/assemble.sh docs/video/output/narration <test-results/.../video.webm>
```

**Remaining for the full film:** concat **Part C** (the mobile-lane app capture, scenes 15–21)
onto the end; optional per-scene tight-sync (feed `timings.json` into the walkthrough beats); and
the **closing credits MUST attribute the CC-BY / CC-BY-SA editions** (see the repo `NOTICE`) —
the video is a public distribution. Generated WAVs/MP4 are git-ignored (release artifacts).
