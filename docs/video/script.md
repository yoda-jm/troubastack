# DEMO-VID — narration script (final)

Per-scene narration for the TroubaShare walkthrough video, finalized from the storyboard in
[`../tasks/DEMO-VID-walkthrough-video-plan.md`](../tasks/DEMO-VID-walkthrough-video-plan.md).
One block per scene = one TTS segment (Part D). Web scenes (0–14) are recorded by the
Playwright walkthrough (Part B); app scenes (15–21) on the emulator (Part C).

Voice: calm, confident, unhurried. `~s` is the target segment length; the audio-first pass
sets real timings and the on-screen action is paced to land under the words.

> **Credits (Part D — required):** the video is a public distribution and includes the
> **Canon in D** (CC-BY-4.0, Michael Fischer v. Mollard / Mutopia) and **Greensleeves**
> (CC-BY-SA-4.0, David Kastrup / Mutopia) editions — the closing credits MUST attribute both
> (creator · license · https://www.mutopiaproject.org/). See the repo `NOTICE`.

## Part 1 — the web app (TroubaStudio)

> Everything in Part 1 is **built live, on camera, from an empty server** — we register the
> band, add the songs, and make every mark ourselves. The end state is exactly the demo you
> can log into (marie / demo). Each annotation is narrated with the *reason* a real band would
> add it — that's the point of the tool.

**S0 — Title card** · ~9s
"This is TroubaShare — a self-hosted app for bands and ensembles. Nothing is pre-loaded here.
We're going to build a real band from an empty server, and everything we make becomes the demo
you can log into."

**S1 — Marie starts a band** · ~13s
"Marie runs the sign-up — no cloud account, no subscription; the server is a box her band
owns. She creates The Troubadours, and as the person who set it up, she's the admin."

**S2 — Invite the bandmates** · ~15s
"She invites the other two by username — Leo, who plays guitar, and Sasha on bass. They each
accept, and now it's a band: three people, one shared space. No email funnel, just the people
in the room."

**S3 — Leo becomes the conductor** · ~12s
"Leo also runs rehearsals, so Marie promotes him to conductor. That's not just a title — it
gives Leo a cue layer of his own that the others can see but can't switch off. More on that in
a minute."

**S4 — A song, typed as plain text** · ~17s
"Now the music. Marie adds their own song, The Open Road, and types the chart as plain text —
chords over lyrics, the way you'd scribble it on a napkin. TroubaShare renders it to a clean
sheet as she types."

**S5 — One song, all its parts** · ~14s
"A song isn't one file. She drops in the real parts beside the chart — a guitar tab, a drum
sheet — all pooled under the one song, so nobody's hunting through email for the right PDF."

**S6 — Everyone tags what they play** · ~17s
"Each player marks what they're actually on. Marie sings and plays the red electric, so she
tags her part with a mic and a red guitar. Sasha tags his as bass, in blue. Now one glance at
the setlist tells every player what to pick up for this song."

**S7 — A mark with a reason: the capo** · ~20s
"Here's the whole idea of the app in one gesture. Leo *always* forgets to put his capo on for
this one. So on his part he swipes a green highlighter over the printed 'Capo two' and writes
himself 'capo on!' right in the margin. It's his note, on his layer — a fix for a real
mistake, not decoration."

**S8 — Layers you can show and hide** · ~19s
"Every mark lives on a layer. Leo's conductor cues, in red, are mandatory — the players can
see them but can't hide them. The shared section markings anyone can edit. And personal notes,
like Marie's, are private to her. Watch: hide a layer, and its ink lifts off the page — show
it again, and it's back. Same sheet, every musician sees only what's theirs."

**S9 — Editing is a canvas** · ~11s
"Editing is direct — pick a tool, a color, and draw straight on the sheet. Move it, resize it,
zoom in. It's a canvas, not a form."

**S10 — Transpose in one click** · ~13s
"Sasha finds it sits low, so Marie transposes the whole chart in one click. The chords rewrite
into the new key — but the layout holds, so every note and highlight stays anchored exactly
where it was drawn."

**S11 — Build the setlist** · ~15s
"For the gig, the songs go into a setlist — The Open Road, then the covers — drag to set the
running order, and you can override a key for just this show."

**S12 — Bake the concert** · ~15s
"When the set is locked, Marie bakes it. TroubaShare freezes every part, every layer, and
every player's cues into one concert bundle — the exact pages the band will play from, ready
to go offline."

**S13 — The same app, at orchestra scale** · ~18s
"And this isn't just for a three-piece. The very same app runs the City Chamber Orchestra —
Mozart's Eine kleine Nachtmusik, from a real published edition: a full score for the conductor,
and a separate part on every desk. It's all in the demo, built the same way we just did."

**S14 — Everyone sees their own view** · ~11s
"Same music, but each musician sees their own layers — the conductor's interpretation on the
score, each player's own bowings on their part. Nothing more, nothing less."

## Part 2 — the mobile app (TroubaStage) — recorded on the emulator (Part C)

**S15 — Connect & download** · ~14s
"On stage, there's no wifi to trust. The app finds the band's server on the local network and
downloads the concert — to perform completely offline."

**S16 — The immersive page** · ~12s
"Tap to perform. The whole screen becomes the page, with the baked annotations composited
right in."

**S17 — Reveal the chrome** · ~10s
"A tap brings back the controls — the song drawer, the live tempo meter, the pager."

**S18 — Page turns & live layers** · ~10s
"Turn pages with a swipe or a foot pedal. Toggle a layer without ever leaving the page."

**S19 — Night mode & count-in** · ~10s
"Night mode for a dark stage. And a silent, visual count-in on the tempo — so the downbeat is
never a surprise."

**S20 — Facing pages & per-role views** · ~14s
"On a tablet, facing pages. And every role gets its own view — the conductor sees the cues
they wrote; the player sees theirs."

**S21 — Outro** · ~8s
"Rehearse together, perform offline, on hardware you own. That's TroubaShare."
