// T203 stage 2 — line-up search chips. A chip is a tagged union (tag / musician / part / musician·part /
// "without musician"); chips AND together, the same semantics as the tag filter today. Pure + unit-tested.
import { foldText } from "../foldText";

export type LineupChip =
  | { kind: "tag"; value: string }
  | { kind: "musician"; id: string; label: string }
  | { kind: "part"; id: string; label: string }
  | { kind: "musicianPart"; musicianId: string; partId: string; label: string }
  | { kind: "withoutMusician"; id: string; label: string };

export type LineupSong = {
  id: string;
  title: string;
  artist?: string;
  tags?: string[];
  lineup?: { musicianId: string; partIds: string[]; optional?: boolean }[];
};

function hasTag(s: LineupSong, folded: string): boolean {
  return (s.tags ?? []).some((t) => foldText(t) === folded);
}
function entryFor(s: LineupSong, musicianId: string) {
  return (s.lineup ?? []).find((e) => e.musicianId === musicianId);
}

/** One chip's test against a song. */
export function chipMatches(s: LineupSong, c: LineupChip): boolean {
  switch (c.kind) {
    case "tag":
      return hasTag(s, foldText(c.value));
    case "musician":
      return !!entryFor(s, c.id);
    case "part":
      return (s.lineup ?? []).some((e) => e.partIds.includes(c.id));
    case "musicianPart": {
      const e = entryFor(s, c.musicianId);
      return !!e && e.partIds.includes(c.partId);
    }
    case "withoutMusician": {
      // "what can we play without B?": B absent, OR present but optional.
      const e = entryFor(s, c.id);
      return !e || !!e.optional;
    }
  }
}

/** A song matches when it passes EVERY chip (AND) and contains every loose word (title/artist substring). */
export function matchLineupSong(s: LineupSong, chips: LineupChip[], words: string[]): boolean {
  const text = foldText(`${s.title} ${s.artist ?? ""} ${(s.tags ?? []).join(" ")}`);
  for (const w of words) {
    const fw = foldText(w);
    if (fw && !text.includes(fw)) return false;
  }
  return chips.every((c) => chipMatches(s, c));
}

export function filterLineupSongs<T extends LineupSong>(songs: T[], chips: LineupChip[], words: string[]): T[] {
  return songs.filter((s) => matchLineupSong(s, chips, words));
}

// --- suggestions: musicians, parts, and the musician·part pairs that actually occur ---

export type ChipSuggestion = { chip: LineupChip; label: string; count: number };

/** Suggest chips for the typed word: musicians + parts whose name contains it, the occurring musician·part
 *  pairs, and a "without <musician>" per matching musician. `count` is how many of the currently-listed songs
 *  the result would show if the chip were added (current chips + other words + this chip). */
export function suggestLineupChips(
  allSongs: LineupSong[],
  chips: LineupChip[],
  otherWords: string[],
  typedWord: string,
  musicians: { id: string; displayName: string }[],
  parts: { id: string; name: string }[],
  limit = 8,
): ChipSuggestion[] {
  const fw = foldText(typedWord);
  if (fw === "") return [];
  const out: ChipSuggestion[] = [];
  const add = (chip: LineupChip, label: string) => {
    const count = filterLineupSongs(allSongs, [...chips, chip], otherWords).length;
    out.push({ chip, label, count });
  };
  const withoutPrefix = fw.startsWith("without ") ? fw.slice("without ".length) : null;

  for (const m of musicians) {
    const fm = foldText(m.displayName);
    if (withoutPrefix !== null) {
      if (fm.includes(withoutPrefix)) add({ kind: "withoutMusician", id: m.id, label: m.displayName }, `without ${m.displayName}`);
      continue;
    }
    if (fm.includes(fw)) {
      add({ kind: "musician", id: m.id, label: m.displayName }, m.displayName);
      // musician·part pairs that occur for this musician
      const theirParts = new Set<string>();
      for (const s of allSongs) for (const e of s.lineup ?? []) if (e.musicianId === m.id) for (const pid of e.partIds) theirParts.add(pid);
      for (const p of parts) {
        if (theirParts.has(p.id)) add({ kind: "musicianPart", musicianId: m.id, partId: p.id, label: `${m.displayName} · ${p.name}` }, `${m.displayName} · ${p.name}`);
      }
    }
  }
  if (withoutPrefix === null) {
    for (const p of parts) {
      if (foldText(p.name).includes(fw)) add({ kind: "part", id: p.id, label: p.name }, p.name);
    }
  }
  out.sort((a, b) => b.count - a.count || a.label.localeCompare(b.label));
  return out.slice(0, limit);
}

/** A stable key for a chip (for React keys + de-dup). */
export function chipKey(c: LineupChip): string {
  switch (c.kind) {
    case "tag":
      return `tag:${foldText(c.value)}`;
    case "musician":
      return `m:${c.id}`;
    case "part":
      return `p:${c.id}`;
    case "musicianPart":
      return `mp:${c.musicianId}:${c.partId}`;
    case "withoutMusician":
      return `wo:${c.id}`;
  }
}

export function chipLabel(c: LineupChip): string {
  switch (c.kind) {
    case "tag":
      return c.value;
    case "withoutMusician":
      return `without ${c.label}`;
    default:
      return c.label;
  }
}
