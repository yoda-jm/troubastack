/**
 * T203 stage 2 — the band-level "Line-ups" grid: the main editor for who plays/sings what, per song. It is a
 * pure projection (buildGrid) of the songs' line-ups + the band's musicians + parts, with sub-columns as
 * CHANNELS: one Voice column per person, one per personal instrument, and a shared instrument its own column.
 * Click a Voice cell to cycle its part; click an instrument cell to toggle it; a shared cell picks the player.
 * A "by part" toggle re-projects the same data. The chips filter the rows (tags + line-up chips, AND).
 */
import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, api, type Song, type MusicianView, type Part, type LineupEntry, type Channel, type MergeNote } from "../api";
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
  const [channels, setChannels] = useState<Channel[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [mergeNotes, setMergeNotes] = useState<MergeNote[]>([]); // T204 s2: surfaced after a track change (ruling 5)
  const [newTrackFor, setNewTrackFor] = useState<string | null>(null); // partId whose inline "+ New track…" field is open
  const [newTrackName, setNewTrackName] = useState("");
  const [renameTrack, setRenameTrack] = useState<string | null>(null); // channelId being renamed inline
  const [confirmDeleteTrack, setConfirmDeleteTrack] = useState<{ id: string; count: number } | null>(null);
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
      const [sg, ms, ps, chs] = await Promise.all([
        api.listSongs(bandId),
        api.listMusicians(bandId),
        api.listParts(bandId),
        api.listChannels(bandId),
      ]);
      setSongs(sg);
      setMusicians(ms);
      setParts(ps);
      setChannels(chs);
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

  const words = queryWords(query);
  const shown = filterLineupSongs(songs as LineupSong[], chips, words);
  const gridMus: GridMusician[] = musicians.map((m) => ({ id: m.id, displayName: m.displayName, order: m.order, instruments: m.instruments }));
  const grid = buildGrid(shown as GridSong[], gridMus, parts, { editor: true, channels });
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

  // T204: one cell = one CHANNEL, carrying the channel's member parts (memberIds, ordered). Clicking cycles
  // none → p0 → p1 → … → (if the channel has 2+ members) ALL members together → none. This generalises the old
  // per-kind handlers: a single-instrument channel reproduces the on/off toggle, a custom group steps through
  // its parts, and the Voice channel reaches L+B by click (T207 binding 1) — the state the keyboard already
  // reached with Shift-L/Shift-B but the mouse/touch user never could. A partial multi-selection resets to none.
  function cycleChannel(songId: string, musicianId: string, memberIds: string[]) {
    const lineup = songLineup(songId);
    const { i } = withEntry(lineup, musicianId);
    const e = lineup[i];
    const member = new Set(memberIds);
    const other = e.partIds.filter((pid) => !member.has(pid));
    const cur = e.partIds.filter((pid) => member.has(pid));
    const multi = memberIds.length > 1;
    let next: string[] = [];
    if (cur.length === 0) {
      next = memberIds.length ? [memberIds[0]] : [];
    } else if (cur.length === 1) {
      const idx = memberIds.indexOf(cur[0]);
      if (idx + 1 < memberIds.length) next = [memberIds[idx + 1]]; // next member alone
      else if (multi) next = [...memberIds]; // last single → all members together
      // else (single-member channel): fall through to none
    } else if (multi && cur.length === memberIds.length) {
      next = []; // all → none
    } // any partial multi-selection → none
    e.partIds = [...other, ...next];
    void patchLineup(songId, prune(lineup));
  }

  // Replace a musician's parts WITHIN this channel with the given set (keeping their other channels).
  function setChannel(songId: string, musicianId: string, memberIds: string[], picked: string[]) {
    const member = new Set(memberIds);
    const lineup = songLineup(songId);
    const { i } = withEntry(lineup, musicianId);
    const e = lineup[i];
    const other = e.partIds.filter((pid) => !member.has(pid));
    e.partIds = [...other, ...picked];
    void patchLineup(songId, prune(lineup));
  }

  // M4 binding 2: typing a letter on a focused channel cell sets that channel's part by its short's first
  // letter (L → lead, B → backing); the same key again clears; Shift toggles membership (Shift-L then Shift-B →
  // L+B). Non-letter keys (arrows) bubble to the grid's navigation handler.
  function channelKeyDown(e: React.KeyboardEvent, songId: string, musicianId: string, memberIds: string[]) {
    const k = e.key;
    if (k.length !== 1 || !/[a-z]/i.test(k)) return;
    const members = memberIds.map((id) => parts.find((p) => p.id === id)).filter((p): p is Part => !!p);
    const part = members.find((p) => (p.short ?? p.name).toLowerCase().startsWith(k.toLowerCase()));
    if (!part) return;
    e.preventDefault();
    e.stopPropagation();
    const entry = songLineup(songId).find((x) => x.musicianId === musicianId);
    const cur = (entry?.partIds ?? []).filter((pid) => memberIds.includes(pid));
    let next: string[];
    if (e.shiftKey) {
      next = cur.includes(part.id) ? cur.filter((id) => id !== part.id) : [...cur, part.id];
    } else {
      next = cur.length === 1 && cur[0] === part.id ? [] : [part.id];
    }
    setChannel(songId, musicianId, memberIds, next);
  }

  function setShared(songId: string, partId: string, musicianId: string) {
    const lineup = songLineup(songId).map((e) => ({ ...e, partIds: e.partIds.filter((p) => p !== partId) }));
    if (musicianId) {
      const { i } = withEntry(lineup, musicianId);
      lineup[i].partIds = [...lineup[i].partIds, partId];
    }
    void patchLineup(songId, prune(lineup));
  }

  // T204 slice 2 — tracks (channels). Assign/create/rename/delete a track; every track change that merges
  // patches surfaces its notes (ruling 5). All of these are admin-only server-side.
  async function assignTrack(partId: string, channelId: string) {
    try {
      setMergeNotes(await api.setPartChannel(bandId, partId, channelId));
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't change the track.");
    }
  }
  async function createTrackInline(partId: string) {
    const name = newTrackName.trim();
    if (!name) return;
    try {
      const ch = await api.createChannel(bandId, name);
      setMergeNotes(await api.setPartChannel(bandId, partId, ch.id));
      setNewTrackFor(null);
      setNewTrackName("");
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't create the track.");
    }
  }
  async function renameChannelSave(id: string, name: string) {
    const n = name.trim();
    setRenameTrack(null);
    if (!n) return;
    try {
      await api.renameChannel(bandId, id, n);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't rename the track.");
    }
  }
  async function deleteTrack(id: string) {
    try {
      setMergeNotes(await api.deleteChannel(bandId, id)); // no force: 409 if it still has parts
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setConfirmDeleteTrack({ id, count: Number(err.body?.parts ?? 0) });
      } else {
        setError(err instanceof ApiError ? err.message : "Couldn't delete the track.");
      }
    }
  }
  async function confirmDeleteTrackNow() {
    if (!confirmDeleteTrack) return;
    try {
      setMergeNotes(await api.deleteChannel(bandId, confirmDeleteTrack.id, true));
      setConfirmDeleteTrack(null);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't delete the track.");
    }
  }
  // Non-blocking warnings for a track (ruling 4): a mix of console input types, or of voice + instrument parts.
  function trackWarnings(channelId: string): string[] {
    const ps = parts.filter((p) => p.channelId === channelId);
    const w: string[] = [];
    const inputs = new Set(ps.map((p) => p.input).filter((x): x is string => !!x && x !== "none"));
    if (inputs.size > 1) w.push("mixes mic and DI");
    const kinds = new Set(ps.map((p) => p.kind));
    if (kinds.has("voice") && kinds.has("instrument")) w.push("mixes voice and instrument");
    return w;
  }
  // Resolve a merge note's channel key (ch:<musicianId>:<channelKey>) to WHOSE track and WHICH track, so the
  // notice reads "<concert>: <owner> — <track>: kept In N (In M dropped)" rather than a raw key (ruling, slice 2).
  function mergeNoteParties(channel: string): { owner: string; track: string } {
    const m = /^ch:([^:]+):(.+)$/.exec(channel);
    if (!m) return { owner: "?", track: channel };
    const [, mid, ck] = m;
    const owner = musicians.find((x) => x.id === mid)?.displayName ?? "?";
    let track = ck;
    if (ck === "voice") track = "Voice";
    else if (ck.startsWith("g:")) track = channels.find((c) => c.id === ck.slice(2))?.name ?? "track";
    else if (ck.startsWith("p:")) {
      const p = parts.find((x) => x.id === ck.slice(2));
      track = p ? (p.short || p.name) : "track";
    }
    return { owner, track };
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
          {mergeNotes.length > 0 && (
            <div className="merge-notice" data-testid="merge-notice">
              <button type="button" className="linklike notice-dismiss" data-testid="merge-notice-dismiss" aria-label="Dismiss" onClick={() => setMergeNotes([])}>✕</button>
              <strong>Patches merged:</strong>
              <ul>
                {mergeNotes.map((n, i) => {
                  const { owner, track } = mergeNoteParties(n.channel);
                  return (
                    <li key={i}>
                      <a data-testid="merge-notice-link" href={`/bands/${bandId}/setlists/${n.setlistId}/sheet`}>{n.setlistName}</a>
                      {`: ${owner} — ${track}: kept In ${n.keptInput} (In ${n.droppedInput} dropped).`}
                    </li>
                  );
                })}
              </ul>
            </div>
          )}
          {confirmDeleteTrack && (
            <div className="confirm-inline" data-testid="track-delete-confirm">
              Revert {confirmDeleteTrack.count} part{confirmDeleteTrack.count === 1 ? "" : "s"} to their own track?
              <button type="button" className="btn-sm" data-testid="track-delete-confirm-yes" onClick={() => void confirmDeleteTrackNow()}>Revert</button>
              <button type="button" className="linklike" onClick={() => setConfirmDeleteTrack(null)}>cancel</button>
            </div>
          )}
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
                {/* T204 s2: the Track control. Shared parts keep their own who-plays column (ruling 2); voice may
                    join a track (ruling 3); members see the track name read-only (ruling 7). */}
                {p.shared ? (
                  <span className="muted" data-testid="part-track-shared"> · track n/a — shared parts keep their own who-plays column</span>
                ) : isAdmin ? (
                  <span className="lineup-part-track">
                    {" · "}
                    <select
                      data-testid="part-track"
                      aria-label={`Track for ${p.name}`}
                      value={newTrackFor === p.id ? "__new__" : (p.channelId ?? "")}
                      onChange={(e) => {
                        const v = e.target.value;
                        if (v === "__new__") { setNewTrackFor(p.id); setNewTrackName(""); }
                        else { setNewTrackFor(null); void assignTrack(p.id, v); }
                      }}
                    >
                      <option value="">— own track —</option>
                      {channels.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
                      <option value="__new__">+ New track…</option>
                    </select>
                    {newTrackFor === p.id && (
                      <form className="inline-form" onSubmit={(e) => { e.preventDefault(); void createTrackInline(p.id); }}>
                        <input autoFocus data-testid="newtrack-name" aria-label="New track name" value={newTrackName} onChange={(e) => setNewTrackName(e.target.value)} placeholder="Track name" />
                        <button type="submit" className="btn-sm" data-testid="newtrack-create">Create</button>
                        <button type="button" className="linklike" onClick={() => { setNewTrackFor(null); setNewTrackName(""); }}>cancel</button>
                      </form>
                    )}
                  </span>
                ) : (
                  p.channelId && <span className="muted" data-testid="part-track-ro"> · {channels.find((c) => c.id === p.channelId)?.name ?? "track"}</span>
                )}
                {isAdmin && (
                  <span className="lineup-part-move">
                    <button type="button" className="linklike" data-testid="partlist-up" title="Move up" aria-label={`Move ${p.name} up`} onClick={() => movePart(p.id, -1)}>▲</button>
                    <button type="button" className="linklike" data-testid="partlist-down" title="Move down" aria-label={`Move ${p.name} down`} onClick={() => movePart(p.id, 1)}>▼</button>
                  </span>
                )}
              </li>
            ))}
          </ul>
          {channels.length > 0 && (
            <div className="lineups-tracks" data-testid="lineups-tracks">
              <h4>Tracks</h4>
              <ul>
                {channels.map((c) => {
                  const count = parts.filter((p) => p.channelId === c.id).length;
                  const warns = trackWarnings(c.id);
                  return (
                    <li key={c.id} data-testid="track-item">
                      {renameTrack === c.id && isAdmin ? (
                        <input
                          autoFocus
                          data-testid="track-rename"
                          aria-label={`Rename ${c.name}`}
                          defaultValue={c.name}
                          onBlur={(e) => void renameChannelSave(c.id, e.target.value)}
                          onKeyDown={(e) => {
                            if (e.key === "Enter") void renameChannelSave(c.id, (e.target as HTMLInputElement).value);
                            if (e.key === "Escape") setRenameTrack(null);
                          }}
                        />
                      ) : (
                        <span
                          data-testid="track-name"
                          className={isAdmin ? "linklike" : undefined}
                          onClick={() => { if (isAdmin) setRenameTrack(c.id); }}
                        >
                          {c.name}
                        </span>
                      )}
                      <span className="muted"> · {count} part{count === 1 ? "" : "s"}</span>
                      {warns.map((w) => <span key={w} className="warn" data-testid="track-warning"> ⚠ {w}</span>)}
                      {isAdmin && <button type="button" className="linklike" data-testid="track-delete" onClick={() => void deleteTrack(c.id)}>delete</button>}
                    </li>
                  );
                })}
              </ul>
            </div>
          )}
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
              <th className="lineup-corner">Song</th>
              {byPartGrid.columns.map((c) => <th key={c.partId}>{c.label}</th>)}
            </tr>
          </thead>
          <tbody>
            {byPartGrid.rows.map((r) => (
              <tr key={r.songId} data-testid="lineup-row">
                <td className="lineup-songcell">{r.title}</td>
                {byPartGrid.columns.map((c) => <td key={c.partId}>{r.cells[c.partId]}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <table className="lineup-grid" data-testid="lineup-grid" ref={gridRef} onKeyDown={onGridKey}>
          <thead>
            <tr>
              <th rowSpan={2} className="lineup-corner">Song</th>
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
                <td className="lineup-songcell">
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
                    if (sub.kind === "channel") {
                      // One cell per channel: click cycles its parts, a letter picks by short. The testid keeps
                      // the voice/inst split so the M4 + line-up e2e selectors still resolve on a default band.
                      return (
                        <td key={sub.key}>
                          <button
                            type="button"
                            className={"lineup-cell" + (cell.text ? " on" : "")}
                            data-testid={sub.channelKey === "voice" ? "lineup-cell-voice" : "lineup-cell-inst"}
                            {...rc(ri, ci)}
                            onKeyDown={(e) => channelKeyDown(e, r.songId, sub.musicianId, sub.partIds)}
                            onClick={() => cycleChannel(r.songId, sub.musicianId, sub.partIds)}
                          >
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
