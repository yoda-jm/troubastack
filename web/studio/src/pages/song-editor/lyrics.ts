// T37 — client mirror of the server's normalizeLyrics (core/internal/httpapi/
// lyricsimport.go), applied to the PASTE path (the fetch path is normalized server-side).
// Kept deliberately minimal and identical in behavior: CRLF→LF, trim outer whitespace,
// collapse 3+ blank lines to one section break, and drop ONLY exact site-cruft lines —
// when in doubt, KEEP. It never touches section labels, chords, brackets, or case.
//
// There is no unit-test runner in studio (Playwright only), so this mirror is exercised
// by the e2e paste path; the authoritative table lives in the Go test.

const CRUFT: RegExp[] = [
  /^submit corrections?\.?$/i,
  /^writer\(s\):.*$/i,
  /^thanks to .* for (these lyrics|correcting these lyrics).*$/i,
  /^\d+ contributors?$/i,
];

export function normalizeLyrics(input: string): string {
  const unified = input.replace(/\r\n/g, "\n").replace(/\r/g, "\n").replace(/[ \t]+\n/g, "\n");
  const kept = unified.split("\n").filter((line) => {
    const t = line.trim();
    return t === "" || !CRUFT.some((re) => re.test(t));
  });
  return kept.join("\n").replace(/\n{3,}/g, "\n\n").trim();
}

// detectSections (T38, OPT-IN — the dialog toggle is default OFF) labels the blank-line
// stanzas that survive import with the T19 dialect's `## Verse N` / `## Chorus` headings.
// SEPARATE from normalizeLyrics on purpose (that stays minimal/keep-when-in-doubt); this
// is the opposite — it invents structure, so it only runs when the user opts in and they
// review the result in the chart editor. Chorus = a stanza whose text repeats verbatim
// (the azlyrics fit); everything else is a numbered verse. Idempotent: text that already
// carries `##` labels is returned untouched, and a single-stanza lyric is left as-is.
export function detectSections(input: string): string {
  const text = input.trim();
  if (text === "" || /^##\s/m.test(text)) return input; // empty or already labeled
  const stanzas = text
    .split(/\n[ \t]*\n/)
    .map((s) => s.replace(/\s+$/, ""))
    .filter((s) => s.trim() !== "");
  if (stanzas.length < 2) return input; // nothing to section

  const key = (s: string) => s.trim().toLowerCase().replace(/\s+/g, " ");
  const counts = new Map<string, number>();
  for (const s of stanzas) counts.set(key(s), (counts.get(key(s)) ?? 0) + 1);

  let verse = 0;
  return stanzas
    .map((s) => {
      const label = (counts.get(key(s)) ?? 0) >= 2 ? "## Chorus" : `## Verse ${++verse}`;
      return `${label}\n${s.trim()}`;
    })
    .join("\n\n");
}
