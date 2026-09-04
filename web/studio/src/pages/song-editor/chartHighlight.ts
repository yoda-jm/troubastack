// T39 — pure tokenizer for the chart dialect, driving the source-pane syntax overlay.
// The rules MIRROR the server renderer (core/internal/chartpdf/chart.go) so the colors
// match what the PDF will do: `# title`, `## section`, a line whose tokens are ALL chords
// is a chord row, `**bold**` inline on a normal line, everything else is plain lyric text.
//
// CRITICAL for the overlay technique: the concatenated token text MUST equal the input
// line character-for-character (bold tokens KEEP their `**` markers) — the highlighted
// <pre> sits behind a transparent-text <textarea>, so any added/dropped character would
// drift the highlight off the caret.

// A single chord like G, Em, C#m7, Dsus4, A7sus4, F/G, N.C. — same regex as chart.go.
const CHORD = /^(N\.C\.|[A-G](#|b)?([0-9]|maj|min|m|dim|aug|sus|add)*(\/[A-G](#|b)?)?)$/;

/** isChordRow: every whitespace-separated token on a non-blank line is a chord. */
export function isChordRow(line: string): boolean {
  const t = line.trim();
  if (t === "") return false;
  return t.split(/\s+/).every((tok) => CHORD.test(tok));
}

export type HlClass =
  | "hl-title"
  | "hl-section"
  | "hl-chord"
  | "hl-bold"
  | "hl-plain"
  | "hl-marker" // T135: a brace directive ({np}/{fn}/{sot}/{eot}) — also fixes {np}/{fn}, plain today
  | "hl-tab"; // T135: verbatim content inside a tab block
export interface HlToken {
  text: string;
  cls: HlClass;
}

/** The line-level class a whole line resolves to (before inline bold splitting). */
export function classifyLine(line: string): "title" | "section" | "chord" | "plain" {
  const t = line.trim();
  if (t === "#" || t.startsWith("# ")) return "title";
  if (t.startsWith("## ")) return "section";
  if (isChordRow(line)) return "chord";
  return "plain";
}

// Brace directives, mirroring the server predicates (chart.go / chart_tab.go): whole line,
// case-insensitive, surrounding whitespace ignored. `{np} x`, `{{sot}}`, `sot` are NOT markers.
const RE_NEW_PAGE = /^\{(new_page|np)\}$/i;
const RE_FOOTNOTE = /^\{(footnote|fn)\}$/i;
const RE_TAB_START = /^\{(start_of_tab|sot)\}$/i;
const RE_TAB_END = /^\{(end_of_tab|eot)\}$/i;

/** isTabStart/isTabEnd: whole-line tab block markers (T135), mirroring the server. */
export function isTabStart(line: string): boolean {
  return RE_TAB_START.test(line.trim());
}
export function isTabEnd(line: string): boolean {
  return RE_TAB_END.test(line.trim());
}

/** hasTabBlock: the source opens at least one tab block (drives the transpose-form note). */
export function hasTabBlock(source: string): boolean {
  return source.split("\n").some((l) => RE_TAB_START.test(l.trim()));
}

// TAB_LINE: a heuristic for a line that looks like tablature — a string label (e|, B|, G|, a|…) then
// fret/technique glyphs. Used ONLY to SUGGEST wrapping (lint), never to auto-detect a block: a wrong
// guess in the renderer would silently change a stage page, so detection lives where it only hints.
const TAB_LINE = /^[A-Ga-g][b#]?\|[-0-9hpbrx/\\~^()|.\s]+$/;

/** looksLikeTabLine: the lint heuristic for a single line. */
export function looksLikeTabLine(line: string): boolean {
  return TAB_LINE.test(line.trim());
}

/** hasUnwrappedTab: a tab-looking line sits OUTSIDE any block — the lint trigger. */
export function hasUnwrappedTab(source: string): boolean {
  let inTab = false;
  for (const line of source.split("\n")) {
    const t = line.trim();
    if (inTab) {
      if (RE_TAB_END.test(t)) inTab = false;
      continue;
    }
    if (RE_TAB_START.test(t)) {
      inTab = true;
      continue;
    }
    if (looksLikeTabLine(line)) return true;
  }
  return false;
}

/**
 * wrapFirstTabRun: wrap the first contiguous run of unwrapped tab-looking lines in {sot}…{eot} — the
 * "Wrap as tab" lint action. Pure (operates on the source), so it is unit-testable and needs no textarea
 * selection. If more runs remain, the lint hint simply reappears. No-op when nothing looks like tab.
 */
export function wrapFirstTabRun(source: string): string {
  const lines = source.split("\n");
  let inTab = false;
  for (let i = 0; i < lines.length; i++) {
    const t = lines[i].trim();
    if (inTab) {
      if (RE_TAB_END.test(t)) inTab = false;
      continue;
    }
    if (RE_TAB_START.test(t)) {
      inTab = true;
      continue;
    }
    if (looksLikeTabLine(lines[i])) {
      let end = i;
      while (end + 1 < lines.length && looksLikeTabLine(lines[end + 1])) end++;
      return [
        ...lines.slice(0, i),
        "{start_of_tab}",
        ...lines.slice(i, end + 1),
        "{end_of_tab}",
        ...lines.slice(end + 1),
      ].join("\n");
    }
  }
  return source;
}

/** hasUnclosedTab: an opener with no matching closer (a quieter lint hint). */
export function hasUnclosedTab(source: string): boolean {
  let inTab = false;
  for (const line of source.split("\n")) {
    const t = line.trim();
    if (inTab) {
      if (RE_TAB_END.test(t)) inTab = false;
    } else if (RE_TAB_START.test(t)) {
      inTab = true;
    }
  }
  return inTab;
}

/**
 * tokenizeChartSource: the STATEFUL highlighter (T135). A tab block ({sot}…{eot}) needs state across
 * lines — inside it every line is verbatim `hl-tab` (a chord row over the strings keeps `hl-chord`,
 * matching how the server draws it), and the markers themselves are `hl-marker`. Outside a block,
 * `{np}`/`{fn}` are also `hl-marker` (they rendered as plain lyric before this). Everything else falls
 * to the per-line tokenizer. Text is preserved line-for-line (each marker/tab line is one whole-line
 * token), so the overlay stays under the caret. An unclosed block runs to EOF, like the renderer.
 */
export function tokenizeChartSource(source: string): HlToken[][] {
  const out: HlToken[][] = [];
  let inTab = false;
  for (const line of source.split("\n")) {
    const t = line.trim();
    if (inTab) {
      if (RE_TAB_END.test(t)) {
        inTab = false;
        out.push([{ text: line, cls: "hl-marker" }]);
      } else if (isChordRow(line)) {
        out.push([{ text: line, cls: "hl-chord" }]);
      } else {
        out.push([{ text: line, cls: "hl-tab" }]);
      }
      continue;
    }
    if (RE_TAB_START.test(t)) {
      inTab = true;
      out.push([{ text: line, cls: "hl-marker" }]);
    } else if (RE_NEW_PAGE.test(t) || RE_FOOTNOTE.test(t)) {
      out.push([{ text: line, cls: "hl-marker" }]);
    } else {
      out.push(tokenizeChartLine(line));
    }
  }
  return out;
}

/** tokenizeChartLine: spans for one line, text-preserving (see the note above). */
export function tokenizeChartLine(line: string): HlToken[] {
  switch (classifyLine(line)) {
    case "title":
      return [{ text: line, cls: "hl-title" }];
    case "section":
      return [{ text: line, cls: "hl-section" }];
    case "chord":
      return [{ text: line, cls: "hl-chord" }];
    default:
      return splitBold(line);
  }
}

// splitBold: a plain line into plain + **bold** runs (markers kept, so length is preserved).
function splitBold(line: string): HlToken[] {
  const out: HlToken[] = [];
  const re = /\*\*[^*]+\*\*/g;
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(line)) !== null) {
    if (m.index > last) out.push({ text: line.slice(last, m.index), cls: "hl-plain" });
    out.push({ text: m[0], cls: "hl-bold" });
    last = m.index + m[0].length;
  }
  if (last < line.length) out.push({ text: line.slice(last), cls: "hl-plain" });
  if (out.length === 0) out.push({ text: line, cls: "hl-plain" }); // blank line
  return out;
}
