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
};
export type GridMusician = { id: string; displayName: string; order?: number; instruments?: string[] };
export type GridLineupEntry = { musicianId: string; partIds: string[]; optional?: boolean; note?: string };
export type GridSong = { id: string; title: string; key?: string; tempo?: number; lineup?: GridLineupEntry[] };

export type SubColumn =
  | { kind: "voice"; key: string; musicianId: string; header: string }
  | { kind: "inst"; key: string; musicianId: string; partId: string; header: string }
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

// --- channel keys (match the Go VoiceChannelKey/InstChannelKey/SharedChannelKey) ---
export const voiceKey = (mid: string) => `voice:${mid}`;
export const instKey = (mid: string, pid: string) => `inst:${mid}:${pid}`;
export const sharedKey = (pid: string) => `shared:${pid}`;

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

/** In editor mode every roster musician gets a column (so an empty band can be filled: you can assign a part
 *  to anyone), each gains a Voice sub-column while the band has any voice part, and every shared part in the
 *  vocabulary shows its column. The sound sheet (default, no options) omits all of that — it shows only what
 *  is actually used. */
export type GridOptions = { editor?: boolean };

/** buildGrid projects the person-major channel grid. musicians/parts are the band vocabulary; by default only
 *  columns USED across the given songs appear (an unused sub-column is omitted) — `editor` relaxes that. */
export function buildGrid(songs: GridSong[], musicians: GridMusician[], parts: GridPart[], opts: GridOptions = {}): Grid {
  const editor = !!opts.editor;
  const bandHasVoice = parts.some((p) => p.kind === "voice");
  const partById = new Map(parts.map((p) => [p.id, p]));
  const musById = new Map(musicians.map((m) => [m.id, m]));

  // First-appearance index (running order) for musicians + shared parts, as the default ordering.
  const firstMus = new Map<string, number>();
  const firstShared = new Map<string, number>();
  // Which sub-columns are USED: per musician, whether they sing (voice) + which personal-instrument parts;
  // and which shared parts occur.
  const musSings = new Set<string>();
  const musInst = new Map<string, Set<string>>(); // musicianId → set of personal instrument partIds
  const sharedUsed = new Set<string>();

  songs.forEach((s, si) => {
    for (const e of s.lineup ?? []) {
      if (!firstMus.has(e.musicianId)) firstMus.set(e.musicianId, si * 1000 + (s.lineup ?? []).indexOf(e));
      for (const pid of e.partIds) {
        const p = partById.get(pid);
        if (!p) continue;
        if (p.kind === "voice") {
          musSings.add(e.musicianId);
        } else if (p.shared) {
          sharedUsed.add(pid);
          if (!firstShared.has(pid)) firstShared.set(pid, si);
        } else {
          if (!musInst.has(e.musicianId)) musInst.set(e.musicianId, new Set());
          musInst.get(e.musicianId)!.add(pid);
        }
      }
    }
  });

  // Musician wide columns, ordered. The editor shows every roster musician; the sheet only the used ones.
  const usedMusicianIDs = editor
    ? musicians.map((m) => m.id)
    : [...new Set([...musSings, ...musInst.keys()])];
  usedMusicianIDs.sort((a, b) =>
    cmpOrder(musById.get(a)?.order ?? 0, musById.get(b)?.order ?? 0, firstMus.get(a) ?? 0, firstMus.get(b) ?? 0),
  );
  const columns: WideColumn[] = [];
  for (const mid of usedMusicianIDs) {
    const label = musById.get(mid)?.displayName ?? "?";
    const subs: SubColumn[] = [];
    if (musSings.has(mid) || (editor && bandHasVoice)) subs.push({ kind: "voice", key: voiceKey(mid), musicianId: mid, header: "Voice" });
    // Instrument sub-columns: the ones USED, plus (editor only) the musician's PERSISTED instruments (ruling 1),
    // so a column never vanishes when its last usage is toggled off. Only valid personal (non-shared) instruments.
    const instSet = new Set<string>(musInst.get(mid) ?? []);
    if (editor) {
      for (const pid of musById.get(mid)?.instruments ?? []) {
        const p = partById.get(pid);
        if (p && p.kind === "instrument" && !p.shared) instSet.add(pid);
      }
    }
    const instIDs = [...instSet];
    instIDs.sort((a, b) => cmpOrder(partById.get(a)?.order ?? 0, partById.get(b)?.order ?? 0, 0, 0) || partShort(partById.get(a)!).localeCompare(partShort(partById.get(b)!)));
    for (const pid of instIDs) {
      subs.push({ kind: "inst", key: instKey(mid, pid), musicianId: mid, partId: pid, header: partShort(partById.get(pid)!) });
    }
    columns.push({ kind: "musician", id: mid, label, subs });
  }

  // Shared part wide columns, ordered. The editor shows every shared part in the vocabulary (so one can be
  // assigned); the sheet only shared parts actually used. Header is "Name — P1/P2" (the players across songs).
  const sharedIDs = editor ? parts.filter((p) => p.shared && p.kind !== "voice").map((p) => p.id) : [...sharedUsed];
  sharedIDs.sort((a, b) => cmpOrder(partById.get(a)?.order ?? 0, partById.get(b)?.order ?? 0, firstShared.get(a) ?? 0, firstShared.get(b) ?? 0));
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
  if (sub.kind === "voice") {
    const e = byMus.get(sub.musicianId);
    if (!e) return { text: "" };
    const shorts = e.partIds
      .map((pid) => partById.get(pid))
      .filter((p): p is GridPart => !!p && p.kind === "voice")
      .map((p) => partShort(p));
    return { text: shorts.join("+"), optional: e.optional };
  }
  if (sub.kind === "inst") {
    const e = byMus.get(sub.musicianId);
    const plays = !!e && e.partIds.includes(sub.partId);
    return { text: plays ? partShort(partById.get(sub.partId)!) : "", optional: plays ? e!.optional : undefined };
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
