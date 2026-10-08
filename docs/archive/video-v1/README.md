# Archived — DEMO-VID, the first walkthrough video (2026-08)

**Retired 2026-10-08 by VLL** ("what we currently have is probably pure shit, archive it") and superseded by the
**video series** spec in [`docs/video/`](../../video/README.md). Kept for history and for the lessons it paid for;
nothing here is run, built or type-checked.

## What it was

One narrated web walkthrough that built the demo band live from an empty server:
- `web-studio-walkthrough/` — the Playwright recorder (`walkthrough.spec.ts`, its config and global setup), which
  ran an isolated core on :8090 + Vite on :5273 with `TROUBA_NO_HMR=1`;
- `docs-video/` — the narration script (`script.md`, S0–S21), Piper TTS (`tools/synth.py`) and the ffmpeg
  assembler (`tools/assemble.sh`);
- `DEMO-VID-walkthrough-video-plan.md` — the program plan (Parts A–D) and VLL's rulings.

Only the web half (S0–S14, ~196 s) was ever rendered, to an ignored output dir; the mobile half (Part C) and the
required credits card were never made.

## Why it was retired (the lessons the new spec builds on)

1. **One long film, one take.** Any UI change staled the whole thing; it was never re-shot after B13/T80+.
2. **Silent skips.** Most beats were wrapped in a `soft()` helper, so a broken step recorded an empty scene
   (T125: the capo note and the setlist beat had been dead for weeks, the bake baked an empty setlist).
3. **Web only.** The tablet — half the product — was never filmed.
4. **Robotic voice, clipping audio** (Piper needed −4 dB + a limiter; the concat demuxer produced white noise).
5. **Footage fitted to narration by stretching**, not composed: no callouts, no zooms, no on-screen text.

The pieces that outlived it stay where they are: the seed's `-only` flag (`core/cmd/seed`), Vite's
`TROUBA_NO_HMR` switch, and the demo charts/bundle (now the product's demo seed).
