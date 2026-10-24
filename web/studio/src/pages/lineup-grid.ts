import { foldText } from "../foldText";

// T203 stage 2 — the person-major line-up grid, as a PURE projection of (songs' line-ups + the band's
// musicians + parts). Sub-columns are CHANNELS (VLL's layout), decided by each part's kind + shared flag:
//   - ONE "Voice" sub-column per musician (one mic): the cell is the joined shorts of their voice parts that
//     song (L / B / "L+B"), or empty.
//   - one sub-column per PERSONAL instrument part a musician plays (one DI/mic): filled with the part short,
//     or empty; optional ⇒ parenthesised.
//   - a SHARED instrument (one physical source passed between players) gets its OWN wide column apart from the
//     people; the cell shows WHO plays it that song.
// A "by part" projection re-reads the same data with part columns and names in the cells. Both are pure +
// unit-tested; the sound sheet (stage 4, Go) applies the same rules server-side.

export type GridPartKind = "voice" | "instrument" | "other";

export type GridPart = {
  id: string;
  name: string;
  kind: GridPartKind;
  short?: string;
  shared?: boolean;
  input?: string;
  order?: number;
  channelId?: string; // T204: a custom console-track group this part belongs to (overrides voice/per-part keying)
};
export type GridChannel = { id: string; name: string; order?: number };
export type GridMusician = { id: string; displayName: string; order?: number; instruments?: string[] };
export type GridLineupEntry = { musicianId: string; partIds: string[]; optional?: boolean; note?: string };
export type GridSong = { id: string; title: string; key?: string; tempo?: number; lineup?: GridLineupEntry[] };

// T204: a personal sub-column is a CHANNEL (track) that groups the parts sharing its key; the cell cycles those
// parts. A shared part keeps its own wide column. `partIds` is the channel's member parts (ordered) — the cycle
// set the editor steps through; `channelKey` is voice | p:<partId> | g:<channelId>.
export type SubColumn =
  | { kind: "channel"; key: string; musicianId: string; channelKey: string; partIds: string[]; header: string }
  | { kind: "shared"; key: string; partId: string; header: string };

/** A wide column: a musician (its voice + personal-instrument sub-columns) or a shared part (one sub-column). */
export type WideColumn = {
  kind: "musician" | "shared";
  id: string; // musicianId or partId
  label: string;
  subs: SubColumn[];
};

export type Cell = { text: string; optional?: boolean };
export type GridRow = { songId: string; title: string; key?: string; tempo?: number; cells: Record<string, Cell> };
export type Grid = { columns: WideColumn[]; rows: GridRow[] };

// --- channel keys (match the Go PartChannelKey/PatchKey/SharedChannelKey) ---
/** A part's channel key: a custom group wins, else "voice" for voice parts, else its own "p:<id>" (T204). For a
 *  band with no custom groups this reduces to one voice channel + one channel per instrument — byte-identical
 *  to T203. */
export const partChannelKey = (p: GridPart): string =>
  p.channelId ? `g:${p.channelId}` : p.kind === "voice" ? "voice" : `p:${p.id}`;
/** The patch/sub-column key for a musician's channel: ch:<musicianId>:<channelKey> (matches Go PatchKey). */
export const channelKey = (mid: string, ck: string) => `ch:${mid}:${ck}`;
// Convenience wrappers for the default-band keys (voice channel; a single-instrument channel).
export const voiceKey = (mid: string) => channelKey(mid, "voice");
export const instKey = (mid: string, pid: string) => channelKey(mid, `p:${pid}`);
export const sharedKey = (pid: string) => `shared:${pid}`;

// T203-FIX2: infer a part's kind from its name on inline create (the row defaulted everything to instrument, so
// "Voix" toggled like a guitar and the L/B cycle had nothing). Matches whole FOLDED WORDS (accents stripped,
// œ/æ expanded, lower-cased) so "Lead guitar" and "Harmonica" stay instruments. "lead" alone counts (it is the
// whole name or sits beside a voice word). The user can still change the kind; the create row shows it first.
const VOICE_WORDS = new Set(["voix", "voice", "vocal", "vocals", "chant", "choeur", "choeurs", "choir", "backing", "harmonie", "harmonies"]);
function partNameWords(name: string): string[] {
  return foldText(name).replace(/œ/g, "oe").replace(/æ/g, "ae").split(/[^a-z0-9]+/).filter(Boolean);
}
export function inferPartKind(name: string): "voice" | "instrument" {
  const words = partNameWords(name);
  if (words.some((w) => VOICE_WORDS.has(w))) return "voice"; // a voice word anywhere (also carries "Chant lead")
  if (words.length === 1 && words[0] === "lead") return "voice"; // "Lead" alone — but not "Lead guitar"
  return "instrument";
}

/** The short label for a part. The SERVER fills the effective short into every part it serves (T203 ruling 3:
 *  one derivation, in Go), so Studio never derives — it reads it. The `|| p.name` is only a defensive fallback
 *  for a locally-constructed part that skipped the server. */
export function partShort(p: GridPart): string {
  return (p.short && p.short.trim() !== "") ? p.short : p.name;
}

// orderThen sorts by an explicit order (0 = unset → after), then by a tiebreak index (first appearance).
function cmpOrder(oa: number, ob: number, ia: number, ib: number): number {
  const a = oa === 0 ? Number.MAX_SAFE_INTEGER : oa;
  const b = ob === 0 ? Number.MAX_SAFE_INTEGER : ob;
  return a !== b ? a - b : ia - ib;
}

// orderLess mirrors Go partOrderLess: an explicit order (≠0) sorts before an unset one, else by value.
function orderLess(oa: number, ob: number): boolean {
  if ((oa === 0) !== (ob === 0)) return oa !== 0;
  return oa < ob;
}

// channelHeader is the sub-column header text: "Voice", a group's name (T204), or a single instrument's short.
function channelHeader(ck: string, channelParts: Map<string, GridPart[]>, chanById: Map<string, GridChannel>): string {
  if (ck === "voice") return "Voice";
  if (ck.startsWith("g:")) {
    const c = chanById.get(ck.slice(2));
    return c?.name ?? (channelParts.get(ck) ?? []).map((p) => partShort(p)).join("/");
  }
  const list = channelParts.get(ck) ?? [];
  return list.length ? partShort(list[0]) : ck;
}

/** In editor mode every roster musician gets a column (so an empty band can be filled: you can assign a part
 *  to anyone), each gains a Voice sub-column while the band has any voice part, and every shared part in the
 *  vocabulary shows its column. The sound sheet (default, no options) omits all of that — it shows only what
 *  is actually used. */
export type GridOptions = { editor?: boolean; channels?: GridChannel[] };

/** buildGrid projects the person-major channel grid. musicians/parts are the band vocabulary; by default only
 *  columns USED across the given songs appear (an unused sub-column is omitted) — `editor` relaxes that. */
export function buildGrid(songs: GridSong[], musicians: GridMusician[], parts: GridPart[], opts: GridOptions = {}): Grid {
  const editor = !!opts.editor;
  const bandHasVoice = parts.some((p) => p.kind === "voice");
  const partById = new Map(parts.map((p) => [p.id, p]));
  const musById = new Map(musicians.map((m) => [m.id, m]));
  const chanById = new Map((opts.channels ?? []).map((c) => [c.id, c]));

  // The STABLE tie-break (T203-FIX2): a musician's/part's index in the roster/vocabulary — never usage. A
  // usage-based tie-break (first appearance in the line-ups) made clicking a cell reorder the columns, because
  // the click changed when that musician first appeared. The roster/vocab order never changes on a cell edit.
  const rosterIndex = new Map(musicians.map((m, i) => [m.id, i]));
  const partIndex = new Map(parts.map((p, i) => [p.id, i]));
  // T204: band-wide non-shared parts grouped by channel key (ordered by part order then vocab index) — a
  // channel's member set (the editor's cycle list) + its order/amplified. Mirrors Go projectColumns' channelParts.
  const channelParts = new Map<string, GridPart[]>();
  for (const p of parts) {
    if (p.shared) continue;
    const k = partChannelKey(p);
    if (!channelParts.has(k)) channelParts.set(k, []);
    channelParts.get(k)!.push(p);
  }
  for (const list of channelParts.values())
    list.sort((a, b) => cmpOrder(a.order ?? 0, b.order ?? 0, partIndex.get(a.id) ?? 1e9, partIndex.get(b.id) ?? 1e9));

  // Channel ordering (mirror Go channelLess): the voice channel first, then (amplified, order, short) so a
  // default band — one "p:<id>" channel per instrument — matches T203's instrument order exactly.
  const isAmplified = (inp?: string) => !!inp && inp !== "none";
  const chanAmplified = (ck: string) => (channelParts.get(ck) ?? []).some((p) => isAmplified(p.input));
  const chanOrder = (ck: string): number => {
    if (ck.startsWith("g:")) {
      const c = chanById.get(ck.slice(2));
      if (c && (c.order ?? 0) !== 0) return c.order!;
    }
    let min = 0;
    (channelParts.get(ck) ?? []).forEach((p, i) => { if (i === 0 || orderLess(p.order ?? 0, min)) min = p.order ?? 0; });
    return min;
  };
  const chanShort = (ck: string): string => {
    let min = "";
    (channelParts.get(ck) ?? []).forEach((p, i) => { const s = partShort(p); if (i === 0 || s < min) min = s; });
    return min;
  };
  const channelLess = (a: string, b: string): number => {
    if ((a === "voice") !== (b === "voice")) return a === "voice" ? -1 : 1;
    const aa = chanAmplified(a), ba = chanAmplified(b);
    if (aa !== ba) return aa ? -1 : 1;
    const ao = chanOrder(a), bo = chanOrder(b);
    if (ao !== bo) return orderLess(ao, bo) ? -1 : 1;
    const sa = chanShort(a), sb = chanShort(b);
    return sa < sb ? -1 : sa > sb ? 1 : 0;
  };

  // Which CHANNELS are used: per musician, the set of channel keys they play across the songs; plus shared parts.
  const musChan = new Map<string, Set<string>>();
  const sharedUsed = new Set<string>();
  const addChan = (mid: string, ck: string) => {
    if (!musChan.has(mid)) musChan.set(mid, new Set());
    musChan.get(mid)!.add(ck);
  };
  songs.forEach((s) => {
    for (const e of s.lineup ?? []) {
      for (const pid of e.partIds) {
        const p = partById.get(pid);
        if (!p) continue;
        if (p.shared) sharedUsed.add(pid);
        else addChan(e.musicianId, partChannelKey(p));
      }
    }
  });
  // Editor relaxation (ruling 1): every roster musician present; a Voice channel when the band has voice; and a
  // channel for each PERSISTED instrument, so a column never vanishes when its last usage is toggled off.
  if (editor) {
    for (const m of musicians) {
      if (!musChan.has(m.id)) musChan.set(m.id, new Set());
      if (bandHasVoice) musChan.get(m.id)!.add("voice");
      for (const pid of m.instruments ?? []) {
        const p = partById.get(pid);
        if (p && p.kind === "instrument" && !p.shared) musChan.get(m.id)!.add(partChannelKey(p));
      }
    }
  }

  // Musician wide columns, ordered (explicit order, then roster index). Each carries its channel sub-columns.
  const usedMusicianIDs = [...musChan.keys()];
  usedMusicianIDs.sort((a, b) =>
    cmpOrder(musById.get(a)?.order ?? 0, musById.get(b)?.order ?? 0, rosterIndex.get(a) ?? 1e9, rosterIndex.get(b) ?? 1e9),
  );
  const columns: WideColumn[] = [];
  for (const mid of usedMusicianIDs) {
    const label = musById.get(mid)?.displayName ?? "?";
    const cks = [...musChan.get(mid)!].sort(channelLess);
    const subs: SubColumn[] = cks.map((ck) => ({
      kind: "channel",
      key: channelKey(mid, ck),
      musicianId: mid,
      channelKey: ck,
      partIds: (channelParts.get(ck) ?? []).map((p) => p.id),
      header: channelHeader(ck, channelParts, chanById),
    }));
    columns.push({ kind: "musician", id: mid, label, subs });
  }

  // Shared part wide columns, ordered. The editor shows every shared part in the vocabulary (so one can be
  // assigned); the sheet only shared parts actually used. Header is "Name — P1/P2" (the players across songs).
  const sharedIDs = editor ? parts.filter((p) => p.shared && p.kind !== "voice").map((p) => p.id) : [...sharedUsed];
  sharedIDs.sort((a, b) => cmpOrder(partById.get(a)?.order ?? 0, partById.get(b)?.order ?? 0, partIndex.get(a) ?? 1e9, partIndex.get(b) ?? 1e9));
  for (const pid of sharedIDs) {
    const p = partById.get(pid)!;
    const players = sharedPlayers(songs, pid).map((mid) => shortName(musById.get(mid)?.displayName ?? "?"));
    const header = players.length ? `${p.name} — ${players.join("/")}` : p.name;
    columns.push({ kind: "shared", id: pid, label: p.name, subs: [{ kind: "shared", key: sharedKey(pid), partId: pid, header }] });
  }

  // Rows.
  const rows: GridRow[] = songs.map((s) => {
    const cells: Record<string, Cell> = {};
    const byMus = new Map<string, GridLineupEntry>();
    for (const e of s.lineup ?? []) byMus.set(e.musicianId, e);
    for (const col of columns) {
      for (const sub of col.subs) {
        cells[sub.key] = cellFor(sub, byMus, partById, musById);
      }
    }
    return { songId: s.id, title: s.title, key: s.key, tempo: s.tempo, cells };
  });

  return { columns, rows };
}

function cellFor(
  sub: SubColumn,
  byMus: Map<string, GridLineupEntry>,
  partById: Map<string, GridPart>,
  musById: Map<string, GridMusician>,
): Cell {
  if (sub.kind === "channel") {
    // The shorts of the channel's member parts this musician plays in the song (in their line-up order), joined
    // "+". A voice channel reads "L"/"B"/"L+B"; a single-instrument channel reads its short or empty; a group
    // channel reads whichever of its parts is carried.
    const e = byMus.get(sub.musicianId);
    if (!e) return { text: "" };
    const members = new Set(sub.partIds);
    const shorts = e.partIds.filter((pid) => members.has(pid)).map((pid) => partShort(partById.get(pid)!));
    if (shorts.length === 0) return { text: "" };
    return { text: shorts.join("+"), optional: e.optional };
  }
  // shared: EVERY musician who plays this part in the song, joined (ruling 4 — two bongo players read "R+V",
  // never a silent first-only). Optional when every player of it is optional.
  const players: string[] = [];
  let allOptional = true;
  for (const [mid, e] of byMus) {
    if (e.partIds.includes(sub.partId)) {
      players.push(shortName(musById.get(mid)?.displayName ?? "?"));
      if (!e.optional) allOptional = false;
    }
  }
  if (players.length === 0) return { text: "" };
  return { text: players.join("+"), optional: allOptional };
}

/** sharedPlayers lists the distinct musicians who play a shared part across the songs, in first appearance. */
export function sharedPlayers(songs: GridSong[], partId: string): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const s of songs) {
    for (const e of s.lineup ?? []) {
      if (e.partIds.includes(partId) && !seen.has(e.musicianId)) {
        seen.add(e.musicianId);
        out.push(e.musicianId);
      }
    }
  }
  return out;
}

/** shortName is a compact label for a grid/sheet cell — the first name (first whitespace token). */
export function shortName(name: string): string {
  return name.trim().split(/\s+/)[0] || name;
}

// --- "by part" projection: columns are parts, cells are the players ---

export type ByPartColumn = { partId: string; label: string };
export type ByPartGrid = { columns: ByPartColumn[]; rows: { songId: string; title: string; cells: Record<string, string> }[] };

export function buildGridByPart(songs: GridSong[], musicians: GridMusician[], parts: GridPart[]): ByPartGrid {
  const musById = new Map(musicians.map((m) => [m.id, m]));
  const used = new Map<string, GridPart>();
  songs.forEach((s) => {
    for (const e of s.lineup ?? []) for (const pid of e.partIds) {
      const p = parts.find((x) => x.id === pid);
      if (p) used.set(pid, p);
    }
  });
  const columns = [...used.values()]
    .sort((a, b) => cmpOrder(a.order ?? 0, b.order ?? 0, 0, 0) || a.name.localeCompare(b.name))
    .map((p) => ({ partId: p.id, label: p.name }));
  const rows = songs.map((s) => {
    const cells: Record<string, string> = {};
    for (const col of columns) {
      const players = (s.lineup ?? [])
        .filter((e) => e.partIds.includes(col.partId))
        .map((e) => shortName(musById.get(e.musicianId)?.displayName ?? "?"));
      cells[col.partId] = players.join(", ");
    }
    return { songId: s.id, title: s.title, cells };
  });
  return { columns, rows };
}
