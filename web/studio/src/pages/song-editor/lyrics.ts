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
