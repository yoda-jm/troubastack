# T38 — Auto-label verses/choruses on lyrics import (opt-in, client-side)

**Priority:** normal (VLL follow-on to T37, 2026-07-12) · **Size:** S ·
**Area:** `web/studio` (the lyrics-import dialog + a pure TS `detectSections`) +
e2e. **Depends on T37** (the import dialog).

## Decision (RULED, VLL-confirmed 2026-07-12)

VLL wants imported lyrics auto-structured into the T19 dialect's section
headers. This is **separate from and must NOT touch `normalizeLyrics`** — the
normalizer's "never invent structure / keep-when-in-doubt" contract stands
(T37). Auto-sectioning INVENTS structure and is sometimes wrong (bridge,
pre-chorus, intro), so:

- **A toggle, default OFF** (VLL's settled call, 2026-07-12 — reconciled from an
  earlier "default ON"). The import dialog gets a **"Label verses & choruses"**
  checkbox, UNCHECKED by default: the safe/minimal path is grouping-only
  (blank-line stanzas, no invented headers), and labeling is explicitly opt-in.
  Checked → run `detectSections` before the text becomes the chart source.
  Either way the user then edits in the T19 editor.
- **Client-side only (TS).** It runs on BOTH the paste text and the fetched
  text in one place (the create-chart handler in the dialog), so **no Go /
  endpoint change** — the `/lyrics-import` endpoint keeps returning
  normalized-but-unlabeled text. Confirmed the right seam.

## `detectSections(text: string): string` — pure, e2e-covered

1. If the text already contains any `## ` section header, RETURN IT UNCHANGED
   (don't relabel a chart the user/site already structured — keep-when-in-doubt).
2. Preserve a leading `# {title}` heading if present; operate on the body.
3. Split the body on blank lines → stanzas (trim each; drop empties).
4. **Chorus by exact verbatim repeat** (VLL's pick — the right azlyrics fit,
   choruses repeat word-for-word): normalize a stanza for comparison
   (trim + collapse inner whitespace + lowercase); any stanza whose normalized
   text appears **2+ times** → `## Chorus` before EVERY occurrence.
5. All other stanzas → `## Verse 1`, `## Verse 2`, … numbered in document order
   (verses only; choruses don't consume a verse number).
6. Re-emit: `# title` (if any) + each stanza under its header, blank line
   between sections. Idempotent (running it on its own output is a no-op — the
   §1 early-return guarantees it).

Edge cases (spec them in the test): a single stanza → `## Verse 1`; no repeats →
all verses, zero `## Chorus` (no false chorus); a stanza repeated 3× → `## Chorus`
×3; already-labeled input → untouched.

## Tests

- e2e (studio has no unit runner): with the toggle ON, paste a 3-stanza lyric
  where stanza 2 repeats → the created chart source shows `## Verse 1`,
  `## Chorus`, `## Verse 2`, `## Chorus` (assert on the chart-source textarea in
  the T19 editor). With the toggle OFF, the same paste yields NO `## ` headers.
  Already-`##`-labeled paste is unchanged either way. Red-first (the toggle
  doesn't exist pre-fix).

## Acceptance criteria

- Toggle present, default OFF (unchecked); `detectSections` behaves per the cases above;
  `normalizeLyrics` is UNCHANGED (verify the diff touches neither it nor Go);
  full suite green; `tsc -b studio` clean; dialog pixel (with the checkbox) at
  the gate.

## Out of scope

- Detecting bridge/pre-chorus/intro/outro (verse/chorus only — the reliable
  signal; the user labels the rest); chord-aware sectioning; any Go/endpoint
  change; touching `normalizeLyrics`.
