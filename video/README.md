# video/ — the build for the series "TroubaStack — From rehearsal room to stage"

The spec is [`docs/video/README.md`](../docs/video/README.md). This folder is its machinery: every episode
is a **build**, re-runnable from a fresh checkout.

```sh
video/tts/setup.sh            # once: the narrator (Kokoro am_eric) in ~/.cache/troubastack-video
video/run.sh 03               # music → narration → filming server → takes → edit → render → master
STEPS="edit render master" video/run.sh 03   # re-cut without re-shooting
```

Output (gitignored) lands in `video/out/`: `music/`, `ep03/audio/` (narration + timings.json),
`takes/` (MP4 + marks.json per take), `ep03/ep03.mp4|webm|srt|vtt` (the master).

| Step | Tool | Source |
|---|---|---|
| music | MIDI in code → fluidsynth + FluidR3_GM | `music/theme.py`, `music/render.sh` |
| narration | Kokoro-82M via kokoro-onnx, read from the spec's narration lines | `tts/synth.py`, `tts/lexicon.tsv` |
| filming server | Studio embedded, seeded demo band, the APK in "Get the app" — `127.0.0.1:18097`, throwaway data | `server.sh` |
| browser takes | Playwright + Chrome's screencast at 1920×1080, drawn cursor, asserted beats | `takes/web-lib.mjs`, `takes/web-*.mjs` |
| tablet takes | the `Trouba_Tablet_1200` emulator (never a real tablet), rehearse-then-film, screenrecord | `takes/emulator.sh`, `takes/tablib.py`, `takes/eNN_tablet.py` |
| edit | the narration is the clock; footage from its marks, sped ≤ 1.6× or held | `episodes/epNN.json`, `build_edit.py` |
| render | Remotion (React), components of spec §3.3 | `remotion/src/` |
| master | two-pass loudnorm −16 LUFS / −1.5 dBTP, MP4 + WebM, checks | `master.sh` |

Needs: ffmpeg, sox, fluidsynth + `/usr/share/sounds/sf2/FluidR3_GM.sf2`, espeak-ng, Node 24, Go, the Android
SDK emulator with the `Trouba_Tablet_1200` AVD, `web/studio` and `web/bake` dependencies installed.
