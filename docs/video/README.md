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
- `web/studio/walkthrough/walkthrough.spec.ts` — the paced tour of the built demo (Open Road
  annotations → guitar chord chart → setlist → the orchestra). `beat(seconds)` holds each frame
  for its narration length so the action lands under the words.

Produce the video:

```sh
cd web/studio && npx playwright test -c playwright.walkthrough.config.ts
# → test-results/.../video.webm  (1920×1080; a generated artifact, git-ignored)
```

**First-cut scope:** the deterministic "show the built demo" spine (scenes 6–14). Follow-ups:
the build-from-scratch scenes (1–5: create band / invite / write a chart), live interactions
(draw / transpose / bake), and tightening each scene's dwell to the final TTS timings.

## Part D — assembly (this lane, later)

Piper TTS for every segment in `script.md` → ffmpeg concat (web + the mobile-lane app capture)
+ narration mux + title cards → the single MP4 under `docs/video/`. **The closing credits MUST
attribute the CC-BY / CC-BY-SA editions** (see the repo `NOTICE`) — the video is a public
distribution.
