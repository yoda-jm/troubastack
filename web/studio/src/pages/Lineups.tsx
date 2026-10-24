/**
 * T203 stage 2 — the band-level "Line-ups" grid: the main editor for who plays/sings what, per song. It is a
 * pure projection (buildGrid) of the songs' line-ups + the band's musicians + parts, with sub-columns as
 * CHANNELS: one Voice column per person, one per personal instrument, and a shared instrument its own column.
 * Click a Voice cell to cycle its part; click an instrument cell to toggle it; a shared cell picks the player.
 * A "by part" toggle re-projects the same data. The chips filter the rows (tags + line-up chips, AND).
 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ApiError, api, type Song, type MusicianView, type Part, type LineupEntry } from "../api";
import { useBand } from "./BandLayout";
import { ErrorBanner } from "../components/ErrorBanner";
import { useDialogs } from "../components/Dialog";
import { queryWords } from "./song-search";
import {
  buildGrid,
  buildGridByPart,
  inferPartKind,
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
  const { confirm } = useDialogs();

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
  const assignPopupRef = useRef<HTMLDivElement | null>(null);
  const assignTriggerRef = useRef<HTMLButtonElement | null>(null);
  const [focusRC, setFocusRC] = useState<[number, number]>([0, 0]); // the grid cell holding the roving tabstop (M4)
  const gridRef = useRef<HTMLTableElement>(null);

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

  // The "+ part" popup must be closeable (T203-FIX2): ✕, Escape (focus returns to the trigger), and a click
  // outside. closeAssign returns focus to the "+ part" button that opened it.
  const closeAssign = useCallback(() => {
    setAssignFor(null);
    setNewInstName("");
    assignTriggerRef.current?.focus();
  }, []);
  useEffect(() => {
    if (!assignFor) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        closeAssign();
      }
    };
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node;
      if (assignPopupRef.current?.contains(t) || assignTriggerRef.current?.contains(t)) return;
      setAssignFor(null); // outside click: close without stealing focus
      setNewInstName("");
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("pointerdown", onDown);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("pointerdown", onDown);
    };
  }, [assignFor, closeAssign]);

  const voiceParts = useMemo(
    () => parts.filter((p) => p.kind === "voice").sort((a, b) => (a.order || 1e9) - (b.order || 1e9) || a.name.localeCompare(b.name)),
    [parts],
  );
  const words = queryWords(query);
  const shown = filterLineupSongs(songs as LineupSong[], chips, words);
  const gridMus: GridMusician[] = musicians.map((m) => ({ id: m.id, displayName: m.displayName, order: m.order, instruments: m.instruments }));
  const grid = buildGrid(shown as GridSong[], gridMus, parts, { editor: true });
  const byPartGrid = buildGridByPart(shown as GridSong[], gridMus, parts);
  // Flat interactive-column index (voice/inst/shared), stable per render — the x-coordinate for arrow nav (M4).
  const subColIndex = new Map<string, number>();
  {
    let i = 0;
    for (const col of grid.columns) for (const sub of col.subs) subColIndex.set(sub.key, i++);
  }
  // CLAMP the roving tabstop to the current grid shape (M4 binding 1): a chip filter that shrinks the rows must
  // not orphan it, or no cell has tabIndex 0 and the grid becomes unreachable by Tab. The clamp is applied at
  // render; focusCell/onFocus still record the raw position.
  const gridRows = grid.rows.length;
  const gridCols = subColIndex.size;
  const fr = Math.min(Math.max(0, focusRC[0]), Math.max(0, gridRows - 1));
  const fc = Math.min(Math.max(0, focusRC[1]), Math.max(0, gridCols - 1));

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

  // Replace a musician's VOICE parts in a song with the given set (keeping their instruments).
  function setVoice(songId: string, musicianId: string, voiceIds: string[]) {
    const lineup = songLineup(songId);
    const { i } = withEntry(lineup, musicianId);
    const e = lineup[i];
    const nonVoice = e.partIds.filter((pid) => !voiceParts.some((v) => v.id === pid));
    e.partIds = [...nonVoice, ...voiceIds];
    void patchLineup(songId, prune(lineup));
  }

  // M4 binding 2: typing a letter on a focused Voice cell sets that voice part by its short's first letter —
  // L → lead, B → backing; the same key again clears; Shift toggles membership (so Shift-L then Shift-B → L+B).
  // Non-letter keys (arrows) are left to bubble to the grid's navigation handler.
  function voiceKeyDown(e: React.KeyboardEvent, songId: string, musicianId: string) {
    const k = e.key;
    if (k.length !== 1 || !/[a-z]/i.test(k)) return;
    const part = voiceParts.find((p) => (p.short ?? p.name).toLowerCase().startsWith(k.toLowerCase()));
    if (!part) return;
    e.preventDefault();
    e.stopPropagation();
    const entry = songLineup(songId).find((x) => x.musicianId === musicianId);
    const cur = (entry?.partIds ?? []).filter((pid) => voiceParts.some((v) => v.id === pid));
    let next: string[];
    if (e.shiftKey) {
      next = cur.includes(part.id) ? cur.filter((id) => id !== part.id) : [...cur, part.id];
    } else {
      next = cur.length === 1 && cur[0] === part.id ? [] : [part.id];
    }
    setVoice(songId, musicianId, next);
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
      // Infer the kind from the name (T203-FIX2): "Voix"/"Chœurs"/… become a VOICE part (which the Voice
      // column cycles), not an instrument. Only an instrument is attached to this musician's column.
      const kind = inferPartKind(n);
      const p = await api.createPart(bandId, n, { kind });
      if (kind === "instrument") await api.addMusicianInstrument(bandId, musicianId, p.id);
      setAssignFor(null);
      setNewInstName("");
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't add the part.");
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
        const ok = await confirm({
          title: `Remove ${partName}?`,
          body: `It's played in ${uses} song(s). Removing it clears those ${uses} usage(s) from the line-ups.`,
          danger: true,
          confirmLabel: "Remove & clear",
        });
        if (ok) {
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

  // --- M4: reorder (admin). Reassign dense 1-based orders over the NEW sequence, so it works even when the
  // current orders are all unset (0). Both a drag (mouse) and the ◀▶/▲▼ buttons (keyboard + a11y) land here. ---
  async function persistOrder<T extends { id: string }>(seq: T[], patch: (id: string, order: number) => Promise<unknown>) {
    try {
      await Promise.all(seq.map((x, i) => patch(x.id, i + 1)));
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't save the order.");
    }
  }
  function moveInSeq<T>(seq: T[], from: number, to: number): T[] {
    if (to < 0 || to >= seq.length || from === to) return seq;
    const next = seq.slice();
    const [x] = next.splice(from, 1);
    next.splice(to, 0, x);
    return next;
  }
  // The musician columns in current display order (the grid already sorts by order then first appearance).
  const musicianOrder = grid.columns.filter((c) => c.kind === "musician").map((c) => ({ id: c.id }));
  function moveMusician(musicianId: string, dir: -1 | 1) {
    const from = musicianOrder.findIndex((m) => m.id === musicianId);
    void persistOrder(moveInSeq(musicianOrder, from, from + dir), (id, order) => api.updateMusician(bandId, id, { order }));
  }
  function moveMusicianTo(dragId: string, dropId: string) {
    const from = musicianOrder.findIndex((m) => m.id === dragId);
    const to = musicianOrder.findIndex((m) => m.id === dropId);
    if (from < 0 || to < 0) return;
    void persistOrder(moveInSeq(musicianOrder, from, to), (id, order) => api.updateMusician(bandId, id, { order }));
  }
  function movePart(partId: string, dir: -1 | 1) {
    const seq = parts.map((p) => ({ id: p.id }));
    const from = seq.findIndex((p) => p.id === partId);
    void persistOrder(moveInSeq(seq, from, from + dir), (id, order) => api.updatePart(bandId, id, { order }));
  }

  // --- M4: 2D arrow-key navigation across the body cells (roving tabindex). focusCell no-ops when the target
  // cell doesn't exist (a ragged edge or an empty-column gap), so no explicit bounds are needed. ---
  function focusCell(r: number, c: number) {
    const el = gridRef.current?.querySelector<HTMLElement>(`[data-row="${r}"][data-col="${c}"]`);
    if (el) {
      setFocusRC([r, c]);
      el.focus();
    }
  }
  function onGridKey(e: React.KeyboardEvent) {
    const t = e.target as HTMLElement;
    if (t.getAttribute("data-row") === null) return;
    const r = Number(t.getAttribute("data-row"));
    const c = Number(t.getAttribute("data-col"));
    let nr = r;
    let nc = c;
    switch (e.key) {
      case "ArrowRight": nc = c + 1; break;
      case "ArrowLeft": nc = c - 1; break;
      case "ArrowDown": nr = r + 1; break;
      case "ArrowUp": nr = r - 1; break;
      default: return;
    }
    if (nr < 0 || nc < 0) return;
    e.preventDefault();
    focusCell(nr, nc);
  }
  const rc = (r: number, c: number) => ({
    "data-row": r,
    "data-col": c,
    tabIndex: fr === r && fc === c ? 0 : -1, // the CLAMPED tabstop, so a shrunk grid always keeps one reachable cell
    onFocus: () => setFocusRC([r, c]),
  });

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
      {parts.length > 0 && !parts.some((p) => p.kind === "voice") && (
        <p className="muted" data-testid="lineups-no-voice-hint">
          No voice part yet — add one (e.g. “Lead” / “Backing”, or “Chant” / “Chœurs”) to sing in this grid.
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
              <li
                key={p.id}
                data-testid="partlist-item"
                draggable={isAdmin}
                onDragStart={(e) => { if (isAdmin) e.dataTransfer.setData("text/part", p.id); }}
                onDragOver={(e) => { if (isAdmin) e.preventDefault(); }}
                onDrop={(e) => {
                  const id = e.dataTransfer.getData("text/part");
                  if (!id || id === p.id) return;
                  e.preventDefault();
                  const seq = parts.map((x) => ({ id: x.id }));
                  const from = seq.findIndex((x) => x.id === id);
                  const to = seq.findIndex((x) => x.id === p.id);
                  void persistOrder(moveInSeq(seq, from, to), (pid, order) => api.updatePart(bandId, pid, { order }));
                }}
              >
                {p.name} <span className="muted">· {p.kind}{p.shared ? " · shared" : ""}</span>
                {isAdmin && (
                  <span className="lineup-part-move">
                    <button type="button" className="linklike" data-testid="partlist-up" title="Move up" aria-label={`Move ${p.name} up`} onClick={() => movePart(p.id, -1)}>▲</button>
                    <button type="button" className="linklike" data-testid="partlist-down" title="Move down" aria-label={`Move ${p.name} down`} onClick={() => movePart(p.id, 1)}>▼</button>
                  </span>
                )}
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
        <table className="lineup-grid" data-testid="lineup-grid" ref={gridRef} onKeyDown={onGridKey}>
          <thead>
            <tr>
              <th rowSpan={2}>Song</th>
              {grid.columns.map((col) => (
                <th
                  key={col.id}
                  colSpan={Math.max(1, col.subs.length)}
                  data-testid="lineup-col-owner"
                  draggable={isAdmin && col.kind === "musician"}
                  onDragStart={(e) => { if (isAdmin && col.kind === "musician") e.dataTransfer.setData("text/musician", col.id); }}
                  onDragOver={(e) => { if (isAdmin && col.kind === "musician") e.preventDefault(); }}
                  onDrop={(e) => { const id = e.dataTransfer.getData("text/musician"); if (id && col.kind === "musician") { e.preventDefault(); moveMusicianTo(id, col.id); } }}
                >
                  {col.label}
                  {isAdmin && col.kind === "musician" && (
                    <span className="lineup-col-move">
                      <button type="button" className="linklike" data-testid="lineup-musician-left" title="Move left" aria-label={`Move ${col.label} left`} onClick={() => moveMusician(col.id, -1)}>◀</button>
                      <button type="button" className="linklike" data-testid="lineup-musician-right" title="Move right" aria-label={`Move ${col.label} right`} onClick={() => moveMusician(col.id, 1)}>▶</button>
                    </span>
                  )}
                  {col.kind === "musician" && (
                    <span className="lineup-addpart-wrap">
                      <button type="button" className="linklike lineup-addpart" data-testid="lineup-add-part" title="Manage this musician's instruments" onClick={(e) => { assignTriggerRef.current = e.currentTarget; setAssignFor(assignFor === col.id ? null : col.id); setNewInstName(""); }}>+ part</button>
                      {assignFor === col.id && (
                        <div className="lineup-assign" data-testid="lineup-assign" ref={assignPopupRef}>
                          <button type="button" className="linklike lineup-assign-close" data-testid="lineup-assign-close" aria-label="Close" onClick={closeAssign}>✕</button>
                          {/* Create a new instrument inline (members may) */}
                          <form
                            className="lineup-assign-new"
                            onSubmit={(e) => { e.preventDefault(); void createAndAddInstrument(col.id, newInstName); }}
                          >
                            <input list="common-part-names" placeholder="new part…" value={newInstName} onChange={(e) => setNewInstName(e.target.value)} data-testid="lineup-newinst" aria-label="New part" />
                            <button type="submit" className="linklike" data-testid="lineup-newinst-add">add</button>
                            {newInstName.trim() !== "" && (
                              <span className="muted lineup-infer-kind" data-testid="lineup-infer-kind"> as {inferPartKind(newInstName)}</span>
                            )}
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
                    </span>
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
            {grid.rows.map((r, ri) => (
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
                    const ci = subColIndex.get(sub.key)!;
                    if (sub.kind === "voice") {
                      return (
                        <td key={sub.key}>
                          <button type="button" className="lineup-cell" data-testid="lineup-cell-voice" {...rc(ri, ci)} onKeyDown={(e) => voiceKeyDown(e, r.songId, sub.musicianId)} onClick={() => cycleVoice(r.songId, sub.musicianId)}>
                            {txt || "·"}
                          </button>
                        </td>
                      );
                    }
                    if (sub.kind === "inst") {
                      return (
                        <td key={sub.key}>
                          <button type="button" className={"lineup-cell" + (cell.text ? " on" : "")} data-testid="lineup-cell-inst" {...rc(ri, ci)} onClick={() => toggleInst(r.songId, sub.musicianId, sub.partId)}>
                            {txt || "·"}
                          </button>
                        </td>
                      );
                    }
                    // shared: a select of who plays it
                    const playerId = musicians.find((m) => cell.text && m.displayName.startsWith(cell.text))?.id ?? "";
                    return (
                      <td key={sub.key}>
                        <select className="lineup-cell" data-testid="lineup-cell-shared" {...rc(ri, ci)} value={playerId} onChange={(e) => setShared(r.songId, sub.partId, e.target.value)}>
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
