/**
 * T203 stage 2 — the band-level "Line-ups" grid: the main editor for who plays/sings what, per song. It is a
 * pure projection (buildGrid) of the songs' line-ups + the band's musicians + parts, with sub-columns as
 * CHANNELS: one Voice column per person, one per personal instrument, and a shared instrument its own column.
 * Click a Voice cell to cycle its part; click an instrument cell to toggle it; a shared cell picks the player.
 * A "by part" toggle re-projects the same data. The chips filter the rows (tags + line-up chips, AND).
 */
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { ApiError, api, type Song, type MusicianView, type Part, type LineupEntry, type Channel, type MergeNote } from "../api";
import { useBand } from "./BandLayout";
import { ErrorBanner } from "../components/ErrorBanner";
import { useDialogs } from "../components/Dialog";
import { LineupScroll, revealInLineupScroll } from "../components/LineupScroll";
import { queryWords } from "./song-search";
import {
  buildGrid,
  buildGridByPart,
  inferPartKind,
  assignSharedPart,
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

// T211 stage 2: the ✎ before each song title opens the song-detail dialog (edit mode only).
const PENCIL = (
  <svg viewBox="0 0 20 20" width="15" height="15" aria-hidden="true">
    <path d="M12.8 3.6l3.6 3.6L7.2 16.4H3.6v-3.6z M10.8 5.6l3.6 3.6" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" strokeLinecap="round" />
  </svg>
);
// T211 stage 2: the voice part's label for the chip — "Lead vocal" → "Lead", else the short or the name.
const voiceLabel = (p: Part): string => p.name.replace(/\s*(voices?|vocals?)$/i, "").trim() || (p.short?.trim() || p.name);
const sameSet = (a: string[], b: string[]): boolean => a.length === b.length && a.every((x) => b.includes(x));

export function Lineups() {
  const { band, myRole } = useBand();
  const bandId = band.id;
  const isAdmin = myRole === "admin";
  const { confirm } = useDialogs();

  // T211: the page opens READ-ONLY on every visit (VLL: "ro by default … to avoid modification while
  // scrolling"); the mode is not remembered. The Edit/Done toggle turns editing on — pure UI state, so flipping
  // it keeps the scroll position and never re-fetches. In read-only the cells are plain text, and the editing
  // affordances (✎, ⋯, the ↺, + musician, Parts, drag reorder) are hidden.
  const [editing, setEditing] = useState(false);
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
  // T211: the grid box (exposed so Jump to can scroll it), whether it overflows sideways (Jump to shows only
  // then), and the per-musician ⋯ menu (fixed-positioned, placed from its trigger so the box never clips it).
  const scrollRef = useRef<HTMLDivElement>(null);
  const [overflowX, setOverflowX] = useState(false);
  const [menuFor, setMenuFor] = useState<string | null>(null); // musician id whose ⋯ menu is open
  const menuTriggerRef = useRef<HTMLButtonElement | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);
  // T211 stage 2: the ✎ song-detail dialog. dialogSong is the open song's id (null = closed); the ref drives
  // showModal/close; the trigger ref returns focus to the ✎ on close; voiceMenuFor is the musician whose voice
  // list is open inside the dialog.
  const [dialogSong, setDialogSong] = useState<string | null>(null);
  const [voiceMenuFor, setVoiceMenuFor] = useState<string | null>(null);
  const [voiceListUp, setVoiceListUp] = useState(false); // open the voice list above the chip when there's no room below
  const dialogRef = useRef<HTMLDialogElement | null>(null);
  const songEditTriggerRef = useRef<HTMLButtonElement | null>(null);
  const dlgBodyRef = useRef<HTMLDivElement | null>(null); // the scrolling sheet body (reset on song change)
  const choiceRef = useRef<HTMLDivElement | null>(null); // the open voice list (scrolled into view so it isn't clipped)
  const voiceRefocusRef = useRef<string | null>(null); // after a list choice, refocus this musician's voice chip

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
  // T210: the grid scrolls inside its box, and the box would clip an absolute popup. So the popup is
  // position:fixed (still in its header cell, so the tab order is unchanged) and placed here from the trigger's
  // rect: below it, kept on screen at the right-hand column, and placed again whenever anything scrolls.
  useLayoutEffect(() => {
    if (!assignFor) return;
    const place = () => {
      const pop = assignPopupRef.current;
      const trig = assignTriggerRef.current;
      if (!pop || !trig) return;
      const t = trig.getBoundingClientRect();
      const vw = document.documentElement.clientWidth;
      pop.style.left = `${Math.max(4, Math.min(t.left, vw - pop.offsetWidth - 4))}px`;
      pop.style.top = `${t.bottom}px`;
      pop.style.maxHeight = `${Math.max(120, window.innerHeight - t.bottom - 4)}px`;
    };
    place();
    window.addEventListener("scroll", place, true); // capture: the box's own scroll doesn't bubble
    window.addEventListener("resize", place);
    return () => {
      window.removeEventListener("scroll", place, true);
      window.removeEventListener("resize", place);
    };
  }, [assignFor]);

  // T211 — the per-musician ⋯ menu. Like the "+ part" popup it is position:fixed and placed from its trigger's
  // rect (the scroll box would clip an absolute child), and closes on Escape (focus back to the ⋯), on an
  // outside click, and after an action.
  const closeMenu = useCallback(() => {
    setMenuFor(null);
    menuTriggerRef.current?.focus();
  }, []);
  useEffect(() => {
    if (!menuFor) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        closeMenu();
      }
    };
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node;
      if (menuRef.current?.contains(t) || menuTriggerRef.current?.contains(t)) return;
      setMenuFor(null); // outside click: close without stealing focus
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("pointerdown", onDown);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("pointerdown", onDown);
    };
  }, [menuFor, closeMenu]);
  useLayoutEffect(() => {
    if (!menuFor) return;
    const place = () => {
      const pop = menuRef.current;
      const trig = menuTriggerRef.current;
      if (!pop || !trig) return;
      const t = trig.getBoundingClientRect();
      const vw = document.documentElement.clientWidth;
      pop.style.left = `${Math.max(4, Math.min(t.left, vw - pop.offsetWidth - 4))}px`;
      pop.style.top = `${t.bottom + 2}px`;
    };
    place();
    window.addEventListener("scroll", place, true);
    window.addEventListener("resize", place);
    return () => {
      window.removeEventListener("scroll", place, true);
      window.removeEventListener("resize", place);
    };
  }, [menuFor]);

  // T211 stage 2: open/close the native <dialog> as dialogSong changes. showModal gives the modal + backdrop +
  // Escape for free; the native close event (Escape, .close(), backdrop) clears the state and returns focus.
  // On a song change (Previous/Next) the body scrolls back to the top so a new song isn't shown mid-scroll (1d).
  useEffect(() => {
    const d = dialogRef.current;
    if (!d) return;
    if (dialogSong && !d.open) d.showModal();
    else if (!dialogSong && d.open) d.close();
    if (dialogSong && dlgBodyRef.current) dlgBodyRef.current.scrollTop = 0;
  }, [dialogSong]);

  // T211 stage 2 (1d): the voice list must open fully visible — for the LAST musician, opening below the chip
  // spills past the scrolling body (and the list is absolute, so the body can't scroll to reveal it). Measure
  // the down position and flip it ABOVE the chip when it would spill; then nudge it into view. useLayoutEffect
  // so the flip happens before paint (no flash).
  useLayoutEffect(() => {
    if (!voiceMenuFor) { setVoiceListUp(false); return; }
    const list = choiceRef.current, body = dlgBodyRef.current, chip = choiceRef.current?.parentElement;
    if (!list || !body || !chip) return;
    const br = body.getBoundingClientRect(), cr = chip.getBoundingClientRect(), h = list.getBoundingClientRect().height;
    // Flip up only when there isn't room below AND there IS room above (so the last musician flips up, but a
    // short dialog whose chip is near the top stays down rather than overlapping the head).
    setVoiceListUp(h > br.bottom - cr.bottom - 4 && cr.top - br.top - 4 >= h);
  }, [voiceMenuFor]);
  useEffect(() => {
    if (voiceMenuFor) choiceRef.current?.scrollIntoView?.({ block: "nearest" });
  }, [voiceMenuFor, voiceListUp]);

  // T211 stage 2 (1d): after choosing from the list the chosen button unmounts and focus falls to <body>; return
  // it to that musician's voice chip. Runs after the re-render that re-adds the value button.
  useEffect(() => {
    const mid = voiceRefocusRef.current;
    if (!mid) return;
    const btn = dialogRef.current?.querySelector<HTMLElement>(`[data-mus-id="${mid}"] [data-testid="song-dialog-voice-value"]`);
    if (btn) {
      btn.focus();
      voiceRefocusRef.current = null;
    }
  });

  const words = queryWords(query);
  const shown = filterLineupSongs(songs as LineupSong[], chips, words);
  const gridMus: GridMusician[] = musicians.map((m) => ({ id: m.id, displayName: m.displayName, order: m.order, instruments: m.instruments, hidden: m.hidden }));
  // T210: band-hidden musicians, for the "Hidden: …" restore bar and for setShared (which must leave their
  // stored entries untouched). The Line-ups page is band scope, so the only exclusion here is `hidden`.
  const hiddenMus = musicians.filter((m) => m.hidden);
  const bandHiddenIds = new Set(hiddenMus.map((m) => m.id));
  const grid = buildGrid(shown as GridSong[], gridMus, parts, { editor: true, channels });
  const byPartGrid = buildGridByPart(shown as GridSong[], gridMus, parts);
  // Flat interactive-column index (voice/inst/shared), stable per render — the x-coordinate for arrow nav (M4).
  const subColIndex = new Map<string, number>();
  {
    let i = 0;
    for (const col of grid.columns) for (const sub of col.subs) subColIndex.set(sub.key, i++);
  }
  // T211: a musician's colour. Stage 1 derives it client-side — musicians in display order take the palette of
  // 8 in order, modulo 8 (no data change; stage 3 makes it a saved, swappable choice). The header cell, the
  // tags in that column, the Jump-to chip and the menu all read it.
  const musColorIndex = new Map<string, number>();
  {
    let i = 0;
    for (const col of grid.columns) if (col.kind === "musician") musColorIndex.set(col.id, i++ % 8);
  }
  const colorOf = (musicianId: string) => musColorIndex.get(musicianId) ?? 0;
  const musicianColumns = grid.columns.filter((c) => c.kind === "musician");

  // Jump to a musician: scroll the box so that musician's header sits just right of the frozen song column.
  function jumpTo(musicianId: string) {
    const box = scrollRef.current;
    if (!box) return;
    const th = box.querySelector<HTMLElement>(`[data-mus-id="${musicianId}"]`);
    const corner = box.querySelector<HTMLElement>(".lineup-corner");
    if (!th || !corner) return;
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    box.scrollTo({ left: Math.max(0, th.offsetLeft - corner.offsetWidth), behavior: reduce ? "auto" : "smooth" });
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
    // T210: suggestions skip hidden players ("as if they don't exist"). A chip already added for someone later
    // hidden still filters, because filterLineupSongs reads the stored line-ups, not this view.
    musicians.filter((m) => !m.hidden).map((m) => ({ id: m.id, displayName: m.displayName })),
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

  // T211 stage 2: toggle one instrument part on/off for a musician in a song (the dialog's instrument switches).
  function toggleInstrument(songId: string, musicianId: string, partId: string) {
    const lineup = songLineup(songId);
    const { i } = withEntry(lineup, musicianId);
    const e = lineup[i];
    e.partIds = e.partIds.includes(partId) ? e.partIds.filter((p) => p !== partId) : [...e.partIds, partId];
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
    // T210 binding 1: assigning a shared part replaces the holder among VISIBLE players only — a band-hidden
    // player's stored entry is left untouched, so bringing them back restores it (the cell then shows both).
    const lineup = assignSharedPart(songLineup(songId), partId, musicianId, bandHiddenIds);
    void patchLineup(songId, prune(lineup));
  }

  // T210: hide/un-hide a musician band-wide (admin-only; the server also gates it). Clearing restores them
  // everywhere with nothing lost.
  async function setMusicianHidden(id: string, hidden: boolean) {
    try {
      await api.updateMusician(bandId, id, { hidden });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't change who's hidden.");
    }
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
      revealInLineupScroll(el); // T210: not left under the stuck header or song column
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
      {/* T211: the active tab names the page, so the visible <h1> goes; a screen-reader one stays (and the
          existing specs still find lineups-title). */}
      <h1 data-testid="lineups-title" className="sr-only">Line-ups</h1>
      <ErrorBanner message={error} />

      {/* T211: one toolbar row — the filter takes the free width, then the buttons and the hidden chip. It
          wraps on a narrow screen. */}
      <div className="lineups-toolbar" data-testid="lineups-toolbar">
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
        {/* T211: Edit/Done. The page is read-only by default; this turns editing on (it reads Done while on, and
            is highlighted). Any band member can edit cells today, so everyone sees it; each control keeps its own
            permission inside edit mode. */}
        <button type="button" className={"btn-sm lineups-edit-toggle" + (editing ? " editing" : "")} data-testid="lineups-edit-toggle" aria-pressed={editing} onClick={() => { setShowParts(false); setAssignFor(null); setMenuFor(null); setEditing((v) => !v); }}>
          {editing ? "Done" : "Edit"}
        </button>
        {editing && isAdmin && (
          <button type="button" className="btn-sm" data-testid="lineups-add-musician" onClick={() => void addMusician()}>
            + musician
          </button>
        )}
        {/* Any member may manage the part vocabulary (ruling 2) — but only in edit mode (T211). */}
        {editing && (
          <button type="button" className="btn-sm" data-testid="lineups-parts-toggle" aria-pressed={showParts} onClick={() => setShowParts((v) => !v)}>
            Parts
          </button>
        )}
        {/* T210 (moved into the toolbar as a chip, T211): the "Hidden" restore bar. Shown for EVERY member
            whenever anyone is band-hidden, so a missing column is never silent; only an admin gets the ↺. */}
        {hiddenMus.length > 0 && (
          <div className="lineup-hidden-bar" data-testid="lineups-hidden-bar">
            <span className="lineup-hidden-label">Hidden:</span>{" "}
            {hiddenMus.map((m, i) => (
              <span key={m.id} className="lineup-hidden-name" data-testid="lineups-hidden-name">
                {i > 0 ? " · " : ""}
                {m.displayName}
                {/* T211: the ↺ shows only in edit mode (the names stay in read-only). */}
                {editing && isAdmin && (
                  <button
                    type="button"
                    className="linklike lineup-unhide"
                    data-testid="lineups-unhide"
                    title={`Bring ${m.displayName} back`}
                    aria-label={`Bring ${m.displayName} back`}
                    onClick={() => void setMusicianHidden(m.id, false)}
                  >
                    ↺
                  </button>
                )}
              </span>
            ))}
          </div>
        )}
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

      {/* T211: Jump to — one chip per visible musician, in the musician's colour, only when the grid is wider
          than its box. A click scrolls the box so that musician's header sits just right of the song column. */}
      {!byPart && overflowX && musicianColumns.length > 0 && (
        <nav className="lineup-jump" data-testid="lineups-jump" aria-label="Jump to a musician">
          <span className="jump-label">Jump to</span>
          {musicianColumns.map((c) => (
            <button key={c.id} type="button" className={"lg-m" + colorOf(c.id)} data-testid="lineups-jump-item" onClick={() => jumpTo(c.id)}>
              {c.label}
            </button>
          ))}
        </nav>
      )}

      {byPart ? (
        <LineupScroll fill editing={editing} scrollRef={scrollRef} onOverflowX={setOverflowX}>
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
        </LineupScroll>
      ) : (
        <LineupScroll fill editing={editing} scrollRef={scrollRef} onOverflowX={setOverflowX}>
          <table className="lineup-grid lineup-grid-main" data-testid="lineup-grid" ref={gridRef} onKeyDown={onGridKey}>
            {/* T211: explicit column widths via a colgroup, so table-layout:fixed sizes the columns past the
                colSpan owner headers (the song column and each cell keep their width, and the table grows wide
                enough to overflow). A trailing flexible "fill" column soaks up any slack when the grid is
                narrower than its box, so the real columns never stretch. */}
            <colgroup>
              <col className="lineup-col-song" />
              {grid.columns.flatMap((col) =>
                col.subs.length === 0
                  ? [<col key={`c-empty:${col.id}`} />]
                  : col.subs.map((sub) => <col key={`c:${sub.key}`} />),
              )}
              <col className="lineup-col-fill" />
            </colgroup>
            <thead>
              <tr>
                <th rowSpan={2} className="lineup-corner" scope="col">Song <span className="lineup-count">{grid.rows.length}</span></th>
                {grid.columns.map((col) => {
                  const mus = col.kind === "musician";
                  return (
                    <th
                      key={col.id}
                      colSpan={Math.max(1, col.subs.length)}
                      className={"lineup-col-owner lg-g0" + (mus ? " lg-m" + colorOf(col.id) : "")}
                      data-testid="lineup-col-owner"
                      data-mus-id={mus ? col.id : undefined}
                      scope="colgroup"
                      draggable={editing && isAdmin && mus}
                      onDragStart={(e) => { if (editing && isAdmin && mus) e.dataTransfer.setData("text/musician", col.id); }}
                      onDragOver={(e) => { if (editing && isAdmin && mus) e.preventDefault(); }}
                      onDrop={(e) => { if (!editing) return; const id = e.dataTransfer.getData("text/musician"); if (id && mus) { e.preventDefault(); moveMusicianTo(id, col.id); } }}
                    >
                      {mus ? (
                        <div className="lineup-mus-in">
                          <span className="lineup-mus-name" title={col.label}>{col.label}</span>
                          {/* T211: a single ⋯ menu per musician (edit mode, admin only), holding Edit musician…,
                              Move left/right and Hide — the controls that used to crowd the header. */}
                          {editing && isAdmin && (
                            <button
                              type="button"
                              className="lineup-more"
                              data-testid="lineup-musician-menu"
                              aria-haspopup="menu"
                              aria-expanded={menuFor === col.id}
                              aria-label={`More for ${col.label}`}
                              onClick={(e) => { menuTriggerRef.current = e.currentTarget; setAssignFor(null); setMenuFor(menuFor === col.id ? null : col.id); }}
                            >
                              ⋯
                            </button>
                          )}
                        </div>
                      ) : (
                        col.label
                      )}
                      {/* The "+ part" / Edit-musician popup still lives in the header cell (so the tab order is
                          unchanged); it is fixed-positioned and placed from its trigger, opened from the ⋯ menu. */}
                      {mus && assignFor === col.id && (
                        <div className="lineup-assign" data-testid="lineup-assign" ref={assignPopupRef}>
                          <button type="button" className="linklike lineup-assign-close" data-testid="lineup-assign-close" aria-label="Close" onClick={closeAssign}>✕</button>
                          {/* Create a new instrument inline */}
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
                    </th>
                  );
                })}
                <th className="lineup-fill" rowSpan={2} aria-hidden="true" />
              </tr>
              <tr>
                {grid.columns.flatMap((col) =>
                  col.subs.length === 0
                    ? [<th key={`empty:${col.id}`} className="muted lg-g0" data-testid="lineup-subcol">—</th>]
                    : col.subs.map((sub, si) => <th key={sub.key} className={"lineup-sub" + (si === 0 ? " lg-g0" : "")} data-testid="lineup-subcol" title={sub.header}>{sub.header}</th>),
                )}
              </tr>
            </thead>
            <tbody>
              {grid.rows.map((r, ri) => (
                <tr key={r.songId} data-testid="lineup-row">
                  <td className="lineup-songcell">
                    <div className="lineup-song-in">
                      {/* T211 stage 2: ✎ opens the song-detail dialog (edit mode only). */}
                      {editing && (
                        <button type="button" className="lineup-edit" data-testid="lineup-song-edit" title="Edit this song" aria-label={`Edit the line-up of ${r.title}`} onClick={(e) => { songEditTriggerRef.current = e.currentTarget; setVoiceMenuFor(null); setDialogSong(r.songId); }}>{PENCIL}</button>
                      )}
                      <span className="lineup-song-title" title={r.title}>
                        {r.title}
                        {r.key ? <span className="muted"> · {r.key}</span> : null}
                      </span>
                    </div>
                  </td>
                  {grid.columns.flatMap((col) =>
                    col.subs.length === 0
                      ? [<td key={`empty:${col.id}`} className="lg-g0" />]
                      : col.subs.map((sub, si) => {
                      const cell = r.cells[sub.key];
                      const txt = cell.optional && cell.text ? `(${cell.text})` : cell.text;
                      const cidx = subColIndex.get(sub.key)!;
                      const g0 = si === 0 ? " lg-g0" : "";
                      const testid = sub.kind === "channel" ? (sub.channelKey === "voice" ? "lineup-cell-voice" : "lineup-cell-inst") : "lineup-cell-shared";
                      if (sub.kind === "channel") {
                        // One cell per channel: click cycles its parts, a letter picks by short. The testid keeps
                        // the voice/inst split so the M4 + line-up e2e selectors still resolve on a default band.
                        // T211: a played cell is a tag in the musician's colour (the td carries lg-m<i>); an empty
                        // cell is blank — no "·". In READ-ONLY the cell is a plain, non-focusable span (no button,
                        // no click, no "+" hover): a tap or drag changes nothing.
                        return (
                          <td key={sub.key} className={"lineup-cell-td lg-m" + colorOf(sub.musicianId) + g0}>
                            {editing ? (
                              <button
                                type="button"
                                className={"lineup-cell" + (cell.text ? " on" : "")}
                                data-testid={testid}
                                {...rc(ri, cidx)}
                                onKeyDown={(e) => channelKeyDown(e, r.songId, sub.musicianId, sub.partIds)}
                                onClick={() => cycleChannel(r.songId, sub.musicianId, sub.partIds)}
                              >
                                {txt}
                              </button>
                            ) : (
                              <span className={"lineup-cell lineup-cell-ro" + (cell.text ? " on" : "")} data-testid={testid}>{txt}</span>
                            )}
                          </td>
                        );
                      }
                      // shared: a select of who plays it. T210: read the holder from the STORED line-up among
                      // visible players, not by matching the projected name over every musician — a hidden name
                      // sharing the holder's first word used to shadow the cell (and the old prefix match was
                      // fragile anyway). T211: read-only shows the holder's name as plain text, not a select.
                      const songLu = songs.find((s) => s.id === r.songId)?.lineup ?? [];
                      const playerId = songLu.find((e) => !bandHiddenIds.has(e.musicianId) && e.partIds.includes(sub.partId))?.musicianId ?? "";
                      const playerName = musicians.find((m) => m.id === playerId)?.displayName ?? "";
                      return (
                        <td key={sub.key} className={"lineup-cell-td" + g0}>
                          {editing ? (
                            <select className="lineup-cell lineup-cell-shared-select" data-testid={testid} aria-label={`Who plays ${sub.partId}`} {...rc(ri, cidx)} value={playerId} onChange={(e) => setShared(r.songId, sub.partId, e.target.value)}>
                              <option value="">—</option>
                              {/* T210: a band-hidden player isn't offered ("as if they don't exist"); their stored
                                  entry is untouched and reappears when they're brought back. */}
                              {musicians.filter((m) => !m.hidden).map((m) => <option key={m.id} value={m.id}>{m.displayName}</option>)}
                            </select>
                          ) : (
                            <span className="lineup-cell lineup-cell-ro lineup-cell-shared-ro" data-testid={testid}>{playerName}</span>
                          )}
                        </td>
                      );
                    }),
                  )}
                  <td className="lineup-fill" aria-hidden="true" />
                </tr>
              ))}
            </tbody>
          </table>
        </LineupScroll>
      )}

      {/* T211: the ⋯ menu — fixed, placed by JS from its trigger, so the scroll box never clips it. */}
      {menuFor && musColorIndex.has(menuFor) && (
        <div className="lineup-menu" role="menu" ref={menuRef} data-testid="lineup-musician-menu-pop">
          <button type="button" role="menuitem" data-testid="lineup-musician-edit" onClick={() => { assignTriggerRef.current = menuTriggerRef.current; const id = menuFor; setMenuFor(null); setAssignFor(id); setNewInstName(""); }}>Edit musician…</button>
          <button type="button" role="menuitem" data-testid="lineup-musician-left" onClick={() => { const id = menuFor; closeMenu(); moveMusician(id, -1); }}>◀ Move left</button>
          <button type="button" role="menuitem" data-testid="lineup-musician-right" onClick={() => { const id = menuFor; closeMenu(); moveMusician(id, 1); }}>Move right ▶</button>
          <button type="button" role="menuitem" className="warn lineup-menu-hide" data-testid="lineup-hide" onClick={() => { const id = menuFor; setMenuFor(null); void setMusicianHidden(id, true); }}>Hide everywhere</button>
        </div>
      )}

      {/* T211 stage 2: the ✎ song-detail dialog — a native <dialog> (modal + backdrop + Escape for free), a
          bottom sheet on a finger. One row per visible musician: a voice chip ‹ value › (arrows step; the middle
          opens a list that is a CHILD of the dialog, or it is inert); each instrument an on/off switch. Every
          change writes through patchLineup, so the grid behind updates at once. */}
      <dialog
        className="lineup-sheet"
        ref={dialogRef}
        data-testid="song-dialog"
        aria-label="Song line-up"
        onClose={() => { setDialogSong(null); setVoiceMenuFor(null); songEditTriggerRef.current?.focus(); }}
        onClick={(e) => { if (e.target === dialogRef.current) dialogRef.current?.close(); }}
      >
        {dialogSong && ((sid: string) => {
          const pos = grid.rows.findIndex((r) => r.songId === sid);
          if (pos < 0) return null;
          const total = grid.rows.length;
          const vparts = parts.filter((p) => p.kind === "voice" && !p.shared);
          const vids = vparts.map((p) => p.id);
          const vopts = [
            { label: "None", ids: [] as string[] },
            ...vparts.map((p) => ({ label: voiceLabel(p), ids: [p.id] })),
            ...(vparts.length >= 2 ? [{ label: vparts.length === 2 ? "Both" : "All", ids: vids }] : []),
          ];
          const partsOf = (mid: string) => new Set((songs.find((s) => s.id === sid)?.lineup ?? []).find((e) => e.musicianId === mid)?.partIds ?? []);
          const curVoice = (have: Set<string>) => vparts.filter((p) => have.has(p.id)).map((p) => p.id);
          const stepVoice = (mid: string, dir: number) => {
            const cur = curVoice(partsOf(mid));
            const k = Math.max(0, vopts.findIndex((o) => sameSet(o.ids, cur)));
            setChannel(sid, mid, vids, vopts[(k + dir + vopts.length) % vopts.length].ids);
          };
          return (
            <>
              <div className="lineup-dlg-head">
                <div>
                  <p className="lineup-dlg-pos" data-testid="song-dialog-pos">Song {pos + 1} of {total}</p>
                  <h2 data-testid="song-dialog-title">{grid.rows[pos].title}</h2>
                </div>
                <button type="button" className="btn-sm" data-testid="song-dialog-close" onClick={() => dialogRef.current?.close()}>Close</button>
              </div>
              <div className="lineup-dlg-body" ref={dlgBodyRef}>
                {musicianColumns.map((col) => {
                  const mid = col.id;
                  const have = partsOf(mid);
                  const cur = curVoice(have);
                  const vk = Math.max(0, vopts.findIndex((o) => sameSet(o.ids, cur)));
                  return (
                    <div key={mid} className={"lineup-dlg-row lg-m" + colorOf(mid)} data-testid="song-dialog-musician" data-mus-id={mid}>
                      <div className="lineup-dlg-name"><i aria-hidden="true" />{col.label}</div>
                      <div className="lineup-dlg-togs">
                        {vparts.length > 0 && (
                          <>
                            <span className="lineup-seg-label">Voice</span>
                            <div className={"lineup-vstep" + (cur.length ? " on" : "")} data-testid="song-dialog-voice">
                              <button type="button" className="lineup-vstep-arr" data-testid="song-dialog-voice-prev" aria-label={`Previous voice for ${col.label}`} onClick={() => stepVoice(mid, -1)}>‹</button>
                              <button type="button" className="lineup-vstep-val" data-testid="song-dialog-voice-value" aria-haspopup="menu" aria-label={`${col.label}'s voice: ${vopts[vk].label}. Choose`} onClick={() => setVoiceMenuFor(voiceMenuFor === mid ? null : mid)}>{vopts[vk].label}</button>
                              <button type="button" className="lineup-vstep-arr" data-testid="song-dialog-voice-next" aria-label={`Next voice for ${col.label}`} onClick={() => stepVoice(mid, 1)}>›</button>
                              {voiceMenuFor === mid && (
                                <div className={"lineup-choice" + (voiceListUp ? " up" : "")} role="menu" data-testid="song-dialog-voice-list" ref={choiceRef}>
                                  {vopts.map((o, oi) => (
                                    <button key={oi} type="button" role="menuitemradio" aria-checked={sameSet(o.ids, cur)} data-testid="song-dialog-voice-option" onClick={() => { setChannel(sid, mid, vids, o.ids); voiceRefocusRef.current = mid; setVoiceMenuFor(null); }}>{o.label}</button>
                                  ))}
                                </div>
                              )}
                            </div>
                            <span className="lineup-togs-break" />
                          </>
                        )}
                        {musicianInstruments(mid).filter((p) => p.kind === "instrument" && !p.shared).map((p) => (
                          <button key={p.id} type="button" className="lineup-tog" data-testid="song-dialog-instrument" aria-pressed={have.has(p.id)} onClick={() => toggleInstrument(sid, mid, p.id)}>{p.name} <small>{p.short?.trim() || p.name}</small></button>
                        ))}
                      </div>
                    </div>
                  );
                })}
                {hiddenMus.length > 0 && (
                  <p className="lineup-dlg-note" data-testid="song-dialog-hidden-note">{hiddenMus.map((m) => m.displayName).join(", ")} {hiddenMus.length === 1 ? "is" : "are"} hidden from the band. Their parts on this song stay saved and come back with them.</p>
                )}
              </div>
              <div className="lineup-dlg-foot">
                <button type="button" className="btn-sm" data-testid="song-dialog-prev" disabled={pos <= 0} onClick={() => { if (pos > 0) { setVoiceMenuFor(null); setDialogSong(grid.rows[pos - 1].songId); } }}>← Previous</button>
                <button type="button" className="btn-sm btn-primary" data-testid="song-dialog-next" disabled={pos >= total - 1} onClick={() => { if (pos < total - 1) { setVoiceMenuFor(null); setDialogSong(grid.rows[pos + 1].songId); } }}>Next →</button>
              </div>
            </>
          );
        })(dialogSong)}
      </dialog>

      {/* The thin status line the prototype shows is optional (T211); it is dropped here so the grid keeps that
          row of height (the "more rows" target). Only the empty-state note remains. */}
      {shown.length === 0 && <p className="muted lineup-status" data-testid="lineups-empty">No songs match.</p>}
    </>
  );
}
