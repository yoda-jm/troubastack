/**
 * T203 stage 2 — the band-level "Line-ups" grid: the main editor for who plays/sings what, per song. It is a
 * pure projection (buildGrid) of the songs' line-ups + the band's musicians + parts, with sub-columns as
 * CHANNELS: one Voice column per person, one per personal instrument, and a shared instrument its own column.
 * Click a Voice cell to cycle its part; click an instrument cell to toggle it; a shared cell picks the player.
 * A "by part" toggle re-projects the same data. The chips filter the rows (tags + line-up chips, AND).
 */
import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, api, type Song, type MusicianView, type Part, type LineupEntry } from "../api";
import { useBand } from "./BandLayout";
import { ErrorBanner } from "../components/ErrorBanner";
import { queryWords } from "./song-search";
import {
  buildGrid,
  buildGridByPart,
  type GridSong,
  type GridMusician,
} from "./lineup-grid";
import {
  filterLineupSongs,
  suggestLineupChips,
  chipKey,
  chipLabel,
  type LineupChip,
  type LineupSong,
} from "./lineup-search";

// The one-click "Add common parts" set (ruling 2). Generic roles only — never band/member data. Each is
// [name, kind, shared]; all idempotent by folded name, so a second click is a no-op.
const COMMON_PARTS: [string, "voice" | "instrument" | "other", boolean][] = [
  ["Lead vocal", "voice", false],
  ["Backing vocal", "voice", false],
  ["Guitar", "instrument", false],
  ["Bass", "instrument", false],
  ["Keys", "instrument", false],
  ["Percussion", "instrument", false],
  ["Drums", "instrument", false],
];
const COMMON_PART_NAMES = COMMON_PARTS.map(([n]) => n);

export function Lineups() {
  const { band, myRole } = useBand();
  const bandId = band.id;
  const isAdmin = myRole === "admin";

  const [songs, setSongs] = useState<Song[]>([]);
  const [musicians, setMusicians] = useState<MusicianView[]>([]);
  const [parts, setParts] = useState<Part[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [byPart, setByPart] = useState(false);
  const [chips, setChips] = useState<LineupChip[]>([]);
  const [query, setQuery] = useState("");
  const [showParts, setShowParts] = useState(false);
  const [newPartName, setNewPartName] = useState("");
  const [newPartKind, setNewPartKind] = useState<"voice" | "instrument" | "other">("instrument");
  const [newPartShort, setNewPartShort] = useState("");
  const [newPartShared, setNewPartShared] = useState(false);
  const [assignFor, setAssignFor] = useState<string | null>(null); // musicianId whose +part picker is open
  const [newInstName, setNewInstName] = useState(""); // the inline "create a new instrument" field

  const load = useCallback(async () => {
    if (!bandId) return;
    try {
      const [sg, ms, ps] = await Promise.all([
        api.listSongs(bandId),
        api.listMusicians(bandId),
        api.listParts(bandId),
      ]);
      setSongs(sg);
      setMusicians(ms);
      setParts(ps);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't load the line-ups.");
    }
  }, [bandId]);
  useEffect(() => {
    void load();
  }, [load]);

  const voiceParts = useMemo(
    () => parts.filter((p) => p.kind === "voice").sort((a, b) => (a.order || 1e9) - (b.order || 1e9) || a.name.localeCompare(b.name)),
    [parts],
  );
  const words = queryWords(query);
  const shown = filterLineupSongs(songs as LineupSong[], chips, words);
  const gridMus: GridMusician[] = musicians.map((m) => ({ id: m.id, displayName: m.displayName, order: m.order, instruments: m.instruments }));
  const grid = buildGrid(shown as GridSong[], gridMus, parts, { editor: true });
  const byPartGrid = buildGridByPart(shown as GridSong[], gridMus, parts);

  const suggestions = suggestLineupChips(
    songs as LineupSong[],
    chips,
    [],
    query.trim(),
    musicians.map((m) => ({ id: m.id, displayName: m.displayName })),
    parts,
  );

  // --- editing: PATCH one song's line-up, then reload ---
  async function patchLineup(songId: string, lineup: LineupEntry[]) {
    try {
      await api.updateSong(bandId, songId, { lineup });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't save the line-up.");
    }
  }
  function songLineup(songId: string): LineupEntry[] {
    return (songs.find((s) => s.id === songId)?.lineup ?? []).map((e) => ({ ...e, partIds: [...e.partIds] }));
  }
  function withEntry(lineup: LineupEntry[], musicianId: string): { lineup: LineupEntry[]; i: number } {
    const i = lineup.findIndex((e) => e.musicianId === musicianId);
    if (i >= 0) return { lineup, i };
    lineup.push({ musicianId, partIds: [] });
    return { lineup, i: lineup.length - 1 };
  }
  function prune(lineup: LineupEntry[]): LineupEntry[] {
    return lineup.filter((e) => e.partIds.length > 0 || (e.note ?? "") !== "");
  }

  function cycleVoice(songId: string, musicianId: string) {
    const lineup = songLineup(songId);
    const { i } = withEntry(lineup, musicianId);
    const e = lineup[i];
    const nonVoice = e.partIds.filter((pid) => !voiceParts.some((v) => v.id === pid));
    const curVoice = e.partIds.filter((pid) => voiceParts.some((v) => v.id === pid));
    // cycle: none → v0 → v1 → … → none (any multi-selection resets to none)
    let next: string[] = [];
    if (curVoice.length === 1) {
      const idx = voiceParts.findIndex((v) => v.id === curVoice[0]);
      next = idx + 1 < voiceParts.length ? [voiceParts[idx + 1].id] : [];
    } else if (curVoice.length === 0) {
      next = voiceParts.length ? [voiceParts[0].id] : [];
    }
    e.partIds = [...nonVoice, ...next];
    void patchLineup(songId, prune(lineup));
  }

  function toggleInst(songId: string, musicianId: string, partId: string) {
    const lineup = songLineup(songId);
    const { i } = withEntry(lineup, musicianId);
    const e = lineup[i];
    e.partIds = e.partIds.includes(partId) ? e.partIds.filter((p) => p !== partId) : [...e.partIds, partId];
    void patchLineup(songId, prune(lineup));
  }

  function setShared(songId: string, partId: string, musicianId: string) {
    const lineup = songLineup(songId).map((e) => ({ ...e, partIds: e.partIds.filter((p) => p !== partId) }));
    if (musicianId) {
      const { i } = withEntry(lineup, musicianId);
      lineup[i].partIds = [...lineup[i].partIds, partId];
    }
    void patchLineup(songId, prune(lineup));
  }

  async function addMusician() {
    const name = window.prompt("Add a guest musician by name:");
    if (!name) return;
    try {
      await api.addMusician(bandId, name);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't add the musician.");
    }
  }
  // Seed a part into the band vocabulary. Any member may create a part WITH its kind/short/shared in one call
  // (ruling 2); later metadata edits stay admin (not offered here).
  async function addPartToVocab() {
    const name = newPartName.trim();
    if (!name) return;
    try {
      await api.createPart(bandId, name, { kind: newPartKind, shared: newPartShared, short: newPartShort.trim() || undefined });
      setNewPartName("");
      setNewPartKind("instrument");
      setNewPartShort("");
      setNewPartShared(false);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't add the part.");
    }
  }

  // The empty-state one-click: create a common set of parts (no data is injected unless the user asks).
  async function addCommonParts() {
    try {
      for (const [name, kind, shared] of COMMON_PARTS) {
        await api.createPart(bandId, name, { kind, shared }); // idempotent by folded name
      }
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't add the common parts.");
    }
  }

  // Per-musician "+ part": add a PERSISTED personal instrument to the musician (ruling 1), so the column stays
  // even with no usage. Either an existing part id, or a new name (member-created inline).
  async function addInstrumentToMusician(musicianId: string, partId: string) {
    setAssignFor(null);
    try {
      await api.addMusicianInstrument(bandId, musicianId, partId);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't add the instrument.");
    }
  }
  async function createAndAddInstrument(musicianId: string, name: string) {
    const n = name.trim();
    if (!n) return;
    try {
      const p = await api.createPart(bandId, n); // defaults to a personal instrument
      await api.addMusicianInstrument(bandId, musicianId, p.id);
      setAssignFor(null);
      setNewInstName("");
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't add the instrument.");
    }
  }
  // Explicit removal from the header menu: refused while used (409), then offer to clear the N usages.
  async function removeInstrument(musicianId: string, partId: string, partName: string) {
    try {
      await api.removeMusicianInstrument(bandId, musicianId, partId, false);
      await load();
    } catch (err) {
      const uses = err instanceof ApiError && typeof err.body?.songs === "number" ? err.body.songs : 0;
      if (uses > 0) {
        if (window.confirm(`${partName} is used in ${uses} song(s). Remove it and clear those ${uses} usage(s)?`)) {
          try {
            await api.removeMusicianInstrument(bandId, musicianId, partId, true);
            await load();
          } catch (e2) {
            setError(e2 instanceof ApiError ? e2.message : "Couldn't remove the instrument.");
          }
        }
        return;
      }
      setError(err instanceof ApiError ? err.message : "Couldn't remove the instrument.");
    }
  }
  // Personal-instrument parts a musician does NOT yet list (candidates to add).
  function addableInstruments(musicianId: string): Part[] {
    const m = musicians.find((x) => x.id === musicianId);
    const have = new Set(m?.instruments ?? []);
    return parts.filter((p) => p.kind === "instrument" && !p.shared && !have.has(p.id));
  }
  // A musician's current personal instruments (for the header remove menu).
  function musicianInstruments(musicianId: string): Part[] {
    const m = musicians.find((x) => x.id === musicianId);
    const ids = new Set(m?.instruments ?? []);
    return parts.filter((p) => ids.has(p.id));
  }

  return (
    <>
      <h1 data-testid="lineups-title">Line-ups</h1>
      <ErrorBanner message={error} />

      <div className="lineups-controls">
        <input
          className="song-filter"
          placeholder="Filter: a name, a part, or “without …”"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          data-testid="lineups-filter"
          aria-label="Filter line-ups"
        />
        <button type="button" className="btn-sm" data-testid="lineups-bypart" aria-pressed={byPart} onClick={() => setByPart((v) => !v)}>
          {byPart ? "By musician" : "By part"}
        </button>
        {isAdmin && (
          <button type="button" className="btn-sm" data-testid="lineups-add-musician" onClick={() => void addMusician()}>
            + musician
          </button>
        )}
        {/* Any member may manage the part vocabulary (ruling 2). */}
        <button type="button" className="btn-sm" data-testid="lineups-parts-toggle" aria-pressed={showParts} onClick={() => setShowParts((v) => !v)}>
          Parts
        </button>
      </div>

      {parts.length === 0 && (
        <p className="lineups-empty-seed" data-testid="lineups-empty-seed">
          This band has no parts yet.{" "}
          <button type="button" className="linklike" data-testid="lineups-add-common" onClick={() => void addCommonParts()}>
            Add common parts
          </button>{" "}
          (vocals, guitar, bass, keys, percussion, drums) — or add your own under “Parts”.
        </p>
      )}

      <datalist id="common-part-names">
        {COMMON_PART_NAMES.map((n) => (
          <option key={n} value={n} />
        ))}
      </datalist>

      {showParts && (
        <div className="lineups-parts" data-testid="lineups-parts">
          <form
            className="lineups-addpart-form"
            onSubmit={(e) => {
              e.preventDefault();
              void addPartToVocab();
            }}
          >
            <input list="common-part-names" placeholder="New part (e.g. Guitar)" value={newPartName} onChange={(e) => setNewPartName(e.target.value)} data-testid="newpart-name" aria-label="New part name" />
            <input placeholder="short" className="newpart-short" value={newPartShort} onChange={(e) => setNewPartShort(e.target.value)} data-testid="newpart-short-text" aria-label="Short label" />
            <select value={newPartKind} onChange={(e) => setNewPartKind(e.target.value as "voice" | "instrument" | "other")} data-testid="newpart-kind" aria-label="Part kind">
              <option value="instrument">instrument</option>
              <option value="voice">voice</option>
              <option value="other">other</option>
            </select>
            <label>
              <input type="checkbox" checked={newPartShared} onChange={(e) => setNewPartShared(e.target.checked)} data-testid="newpart-shared" /> shared
            </label>
            <button type="submit" className="btn-sm" data-testid="newpart-add">Add part</button>
          </form>
          <ul className="lineups-partlist">
            {parts.map((p) => (
              <li key={p.id} data-testid="partlist-item">
                {p.name} <span className="muted">· {p.kind}{p.shared ? " · shared" : ""}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {chips.length > 0 && (
        <div className="lineups-chips" data-testid="lineups-chips">
          {chips.map((c) => (
            <button key={chipKey(c)} type="button" className="chip" data-testid="lineup-chip" onClick={() => setChips((cs) => cs.filter((x) => chipKey(x) !== chipKey(c)))}>
              {chipLabel(c)} ✕
            </button>
          ))}
        </div>
      )}
      {query.trim() !== "" && suggestions.length > 0 && (
        <ul className="lineups-suggest" data-testid="lineups-suggest">
          {suggestions.map((s) => (
            <li key={chipKey(s.chip)}>
              <button type="button" data-testid="lineups-suggest-item" onClick={() => { setChips((cs) => [...cs, s.chip]); setQuery(""); }}>
                {s.label} <span className="muted">({s.count})</span>
              </button>
            </li>
          ))}
        </ul>
      )}

      {byPart ? (
        <table className="lineup-grid" data-testid="lineup-grid-bypart">
          <thead>
            <tr>
              <th>Song</th>
              {byPartGrid.columns.map((c) => <th key={c.partId}>{c.label}</th>)}
            </tr>
          </thead>
          <tbody>
            {byPartGrid.rows.map((r) => (
              <tr key={r.songId} data-testid="lineup-row">
                <td>{r.title}</td>
                {byPartGrid.columns.map((c) => <td key={c.partId}>{r.cells[c.partId]}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <table className="lineup-grid" data-testid="lineup-grid">
          <thead>
            <tr>
              <th rowSpan={2}>Song</th>
              {grid.columns.map((col) => (
                <th key={col.id} colSpan={Math.max(1, col.subs.length)} data-testid="lineup-col-owner">
                  {col.label}
                  {col.kind === "musician" && (
                    <>
                      <button type="button" className="linklike lineup-addpart" data-testid="lineup-add-part" title="Manage this musician's instruments" onClick={() => { setAssignFor(assignFor === col.id ? null : col.id); setNewInstName(""); }}>+ part</button>
                      {assignFor === col.id && (
                        <div className="lineup-assign" data-testid="lineup-assign">
                          {/* Create a new instrument inline (members may) */}
                          <form
                            className="lineup-assign-new"
                            onSubmit={(e) => { e.preventDefault(); void createAndAddInstrument(col.id, newInstName); }}
                          >
                            <input list="common-part-names" placeholder="new instrument…" value={newInstName} onChange={(e) => setNewInstName(e.target.value)} data-testid="lineup-newinst" aria-label="New instrument" />
                            <button type="submit" className="linklike" data-testid="lineup-newinst-add">add</button>
                          </form>
                          {/* Add an existing personal instrument */}
                          {addableInstruments(col.id).map((p) => (
                            <button key={p.id} type="button" className="linklike" data-testid="lineup-assign-opt" onClick={() => void addInstrumentToMusician(col.id, p.id)}>+ {p.name}</button>
                          ))}
                          {/* Remove one the musician already has (refused while used; offers clear-N) */}
                          {musicianInstruments(col.id).map((p) => (
                            <button key={`rm-${p.id}`} type="button" className="linklike lineup-remove-inst" data-testid="lineup-remove-inst" title={`Remove ${p.name}`} onClick={() => void removeInstrument(col.id, p.id, p.name)}>✕ {p.name}</button>
                          ))}
                        </div>
                      )}
                    </>
                  )}
                </th>
              ))}
            </tr>
            <tr>
              {grid.columns.flatMap((col) =>
                col.subs.length === 0
                  ? [<th key={`empty:${col.id}`} className="muted" data-testid="lineup-subcol">—</th>]
                  : col.subs.map((sub) => <th key={sub.key} data-testid="lineup-subcol">{sub.header}</th>),
              )}
            </tr>
          </thead>
          <tbody>
            {grid.rows.map((r) => (
              <tr key={r.songId} data-testid="lineup-row">
                <td>
                  {r.title}
                  {r.key ? <span className="muted"> · {r.key}</span> : null}
                </td>
                {grid.columns.flatMap((col) =>
                  col.subs.length === 0
                    ? [<td key={`empty:${col.id}`} className="muted">·</td>]
                    : col.subs.map((sub) => {
                    const cell = r.cells[sub.key];
                    const txt = cell.optional && cell.text ? `(${cell.text})` : cell.text;
                    if (sub.kind === "voice") {
                      return (
                        <td key={sub.key}>
                          <button type="button" className="lineup-cell" data-testid="lineup-cell-voice" onClick={() => cycleVoice(r.songId, sub.musicianId)}>
                            {txt || "·"}
                          </button>
                        </td>
                      );
                    }
                    if (sub.kind === "inst") {
                      return (
                        <td key={sub.key}>
                          <button type="button" className={"lineup-cell" + (cell.text ? " on" : "")} data-testid="lineup-cell-inst" onClick={() => toggleInst(r.songId, sub.musicianId, sub.partId)}>
                            {txt || "·"}
                          </button>
                        </td>
                      );
                    }
                    // shared: a select of who plays it
                    const playerId = musicians.find((m) => cell.text && m.displayName.startsWith(cell.text))?.id ?? "";
                    return (
                      <td key={sub.key}>
                        <select className="lineup-cell" data-testid="lineup-cell-shared" value={playerId} onChange={(e) => setShared(r.songId, sub.partId, e.target.value)}>
                          <option value="">·</option>
                          {musicians.map((m) => <option key={m.id} value={m.id}>{m.displayName}</option>)}
                        </select>
                      </td>
                    );
                  }),
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {shown.length === 0 && <p className="muted" data-testid="lineups-empty">No songs match.</p>}
    </>
  );
}
