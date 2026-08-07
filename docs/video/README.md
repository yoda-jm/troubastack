# docs/video — DEMO-VID walkthrough

Assets for the TroubaShare walkthrough video (plan:
[`../tasks/DEMO-VID-walkthrough-video-plan.md`](../tasks/DEMO-VID-walkthrough-video-plan.md)).

- **[`script.md`](script.md)** — the finalized per-scene narration (one block = one TTS
  segment). Web scenes 0–14, app scenes 15–21.

## Part B — web walkthrough recording (this lane)

A Playwright config records the **seeded** demo through the web storyboard at 1920×1080:

- `web/studio/playwright.walkthrough.config.ts` — recording config (`video: on`,
  1920×1080). Seeds a file-backed TroubaCore via `walkthrough/global-setup.ts`, serves the
  SPA through Vite, and drives it.
- `web/studio/walkthrough/walkthrough.spec.ts` — the paced tour of scenes 1–14 with LIVE
  interactions: create a song + type a chart that renders live, the multi-file pool, the Open
  Road annotations, drawing on the canvas, transposing a chart, the setlist, baking the concert,
  and the orchestra's per-desk parts + conductor score. `beat(seconds)` holds each frame for its
  narration length; fragile steps are wrapped in `soft()` so a selector miss is skipped, never
  aborting the recording. Produces a ~100s 1920×1080 video.

Produce the video:

```sh
cd web/studio && npx playwright test -c playwright.walkthrough.config.ts
# → test-results/.../video.webm  (1920×1080; a generated artifact, git-ignored)
```

**Known gaps / follow-ups:** the live layer-toggle beat (scene 7–8) is skipped — the Layers
pill is found but not actionable in that editor state (the annotations are still *shown*, just
not toggled on camera); scenes 0/15–21 are Part C/D; and the per-scene dwell times will be
re-timed to the final TTS lengths in the audio-first assembly pass (Part D).

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
