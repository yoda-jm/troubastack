/**
 * Setlist detail — edit the setlist metadata, manage its ordered items (add a
 * band song, per-item key/tempo/notes overrides, reorder, bench, remove) and
 * delete. Redesign: page header + status chips, panelled sections, a roomy
 * running-order list whose per-song overrides open in an inline editor, and a
 * distinct "Bench (on call)" section (T23).
 */
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  ApiError,
  api,
  type Concert,
  type Role,
  type Setlist,
  type SetlistItem,
  type Song,
  type SongCue,
} from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { CueGlyph } from "../components/CueGlyphs";
import { AudienceTag } from "../components/AudienceTag";
import { BakeDialog } from "./BakeDialog";

export function SetlistDetail() {
  const { bandId, setlistId } = useParams<{ bandId: string; setlistId: string }>();
  const navigate = useNavigate();
  const [setlist, setSetlist] = useState<Setlist | null>(null);
  const [items, setItems] = useState<SetlistItem[]>([]);
  const [songs, setSongs] = useState<Song[]>([]);
  const [myRole, setMyRole] = useState<Role | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!bandId || !setlistId) return;
    try {
      const [{ setlist, items }, songs, { myRole }] = await Promise.all([
        api.getSetlist(bandId, setlistId),
        api.listSongs(bandId),
        api.getBand(bandId),
      ]);
      // Main order first, then bench; position within each group (matches the server
      // and lets the Items view slice the two groups cleanly — T23).
      items.sort(
        (a, b) => Number(a.onCall ?? false) - Number(b.onCall ?? false) || a.position - b.position,
      );
      setSetlist(setlist);
      setItems(items);
      setSongs(songs);
      setMyRole(myRole);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load setlist");
    }
  }, [bandId, setlistId]);

  useEffect(() => {
    void load();
  }, [load]);

  if (!bandId || !setlistId) return <div className="page">Loading…</div>;
  if (error && !setlist) {
    return (
      <div className="page">
        <Link className="crumb" to={`/bands/${bandId}/setlists`}>
          &larr; Setlists
        </Link>
        <ErrorBanner message={error} />
      </div>
    );
  }
  if (!setlist) return <div className="page">Loading…</div>;

  const mainCount = items.filter((it) => !it.onCall).length;
  const benchCount = items.length - mainCount;
  const sub = [setlist.eventDate, setlist.venue].filter(Boolean).join(" · ");

  return (
    <div className="page">
      {liveNow(setlist) && (
        <div className="live-banner" data-testid="live-banner" role="status">
          <span className="live-dot" aria-hidden="true" />
          LIVE — edits to these songs are auto-publishing to performers
        </div>
      )}
      <Link className="crumb" to={`/bands/${bandId}/setlists`}>
        &larr; Setlists
      </Link>
      <header className="phead">
        <div>
          <div className="eyebrow">Setlist</div>
          <h1 className="title" data-testid="setlist-detail-title">
            {setlist.name}
          </h1>
          {sub && <div className="sub">{sub}</div>}
          <div className="meta">
            <span className="chip mono">
              {mainCount} song{mainCount === 1 ? "" : "s"}
            </span>
            {benchCount > 0 && <span className="chip brand">{benchCount} on call</span>}
          </div>
        </div>
      </header>
      <div className="staff sig" aria-hidden="true" />

      <SetlistMeta bandId={bandId} setlist={setlist} onSaved={setSetlist} />
      <Items bandId={bandId} setlistId={setlistId} items={items} songs={songs} reload={load} />
      <DuplicateAction
        bandId={bandId}
        setlistId={setlistId}
        onDuplicated={(id) => navigate(`/bands/${bandId}/setlists/${id}`)}
      />
      {myRole === "admin" && (
        <LiveModeCard bandId={bandId} setlist={setlist} onChanged={setSetlist} />
      )}
      <BakeCard
        bandId={bandId}
        setlistId={setlistId}
        songIds={[...new Set(items.map((it) => it.songId))]}
        myRole={myRole}
      />
      {myRole === "admin" && (
        <DeleteSetlist
          bandId={bandId}
          setlistId={setlistId}
          onDeleted={() => navigate(`/bands/${bandId}/setlists`)}
        />
      )}
    </div>
  );
}

// BakeCard (B02 + B07): flatten this setlist into a downloadable .tstage (I11).
// The band bake (admin-only) is the shared concert; "Bake my parts" (any member)
// mints the caller's PERSONAL variant — same setlist, but each song resolves to
// the member's own "my files" pick (concertId `${setlistId}~${userId}`). One card.
// liveNow: is the setlist in rehearsal live mode right now? Self-expiring server-side,
// so we also check the client clock against liveUntil (a stale page shouldn't claim live).
function liveNow(sl: Setlist): boolean {
  return !!sl.liveUntil && new Date(sl.liveUntil).getTime() > Date.now();
}

// LiveModeCard (P201, admin-only): toggle rehearsal live mode. While on, edits to the
// setlist's songs auto-bake — the banner up top says so. Bounded window server-side.
function LiveModeCard({
  bandId,
  setlist,
  onChanged,
}: {
  bandId: string;
  setlist: Setlist;
  onChanged: (sl: Setlist) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const on = liveNow(setlist);

  async function toggle() {
    setBusy(true);
    setError(null);
    try {
      const { setlist: updated } = await api.setSetlistLive(bandId, setlist.id, !on);
      onChanged(updated);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't change live mode");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="card live-card" data-testid="live-card">
      <div className="card-head">
        <h2>Rehearsal live mode</h2>
        {on && <span className="chip live">● LIVE</span>}
      </div>
      <p className="muted">
        While live, edits to this setlist&rsquo;s songs auto-publish to performers (they
        auto-update in the app if they opt in). Turns itself off after a few hours.
      </p>
      <button
        type="button"
        className={on ? "btn danger" : "btn brand"}
        data-testid="live-toggle"
        disabled={busy}
        onClick={toggle}
      >
        {busy ? "…" : on ? "Stop live mode" : "Go live (rehearsal)"}
      </button>
      {error && <p className="notice" role="alert">{error}</p>}
    </section>
  );
}

function BakeCard({
  bandId,
  setlistId,
  songIds,
  myRole,
}: {
  bandId: string;
  setlistId: string;
  songIds: string[];
  myRole: Role | null;
}) {
  const [concerts, setConcerts] = useState<Concert[]>([]);
  const [busy, setBusy] = useState<"" | "band" | "mine">("");
  const [error, setError] = useState<string | null>(null);
  const [warnings, setWarnings] = useState<string[]>([]); // T60: per-song bake warnings
  // P205: the open bake dialog (which scope), or null. Baking goes THROUGH the dialog
  // so default-layer capture is never silent.
  const [dialog, setDialog] = useState<{ scope?: "mine" } | null>(null);
  const isAdmin = myRole === "admin";

  const load = useCallback(async () => {
    try {
      const all = await api.listConcerts(bandId);
      setConcerts(
        all.filter((c) => c.concertId === setlistId || c.concertId.startsWith(`${setlistId}~`)),
      );
    } catch {
      // A missing/empty concert list is not an error worth surfacing here.
    }
  }, [bandId, setlistId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function bake(scope: "mine" | undefined, layerDefaults: Record<string, boolean>) {
    setDialog(null);
    setBusy(scope === "mine" ? "mine" : "band");
    setError(null);
    setWarnings([]);
    try {
      const concert = await api.bakeSetlist(bandId, setlistId, scope, layerDefaults);
      setWarnings(concert.warnings ?? []);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Bake failed");
    } finally {
      setBusy("");
    }
  }

  const bandConcert = concerts.find((c) => c.concertId === setlistId) ?? null;
  const myConcert = concerts.find((c) => c.concertId.startsWith(`${setlistId}~`)) ?? null;

  return (
    <section className="panel" data-testid="bake-card">
      <div className="panel-head">
        <h2>Bake</h2>
      </div>
      <div className="panel-body">
        <p className="muted" style={{ marginTop: 0 }}>
          Flatten this setlist into a performable <code>.tstage</code> bundle (page images +
          annotation overlays) to download and load on a phone.
        </p>
        {isAdmin && (
          <div className="inline-form">
            <button
              type="button"
              className="primary"
              data-testid="bake-setlist"
              disabled={busy !== ""}
              onClick={() => setDialog({})}
            >
              {busy === "band" ? "Baking…" : "Bake setlist"}
            </button>
            <AudienceTag audience="band" />{/* T56: the shared band bundle */}
            {bandConcert && (
              <a
                data-testid="bake-download"
                href={bandConcert.downloadUrl}
                download={`${bandConcert.name || "concert"}.tstage`}
              >
                Download .tstage (rev {bandConcert.currentRev})
              </a>
            )}
            {bandConcert && (
              <a
                data-testid="bake-pdf-download"
                href={api.concertPdfUrl(bandConcert)}
                download={`${bandConcert.name || "concert"}.pdf`}
                title="A printable paper backup of this concert — your view, composited to PDF (T57)"
              >
                Download PDF
              </a>
            )}
          </div>
        )}
        <div className="inline-form">
          <button
            type="button"
            data-testid="bake-mine"
            disabled={busy !== ""}
            onClick={() => setDialog({ scope: "mine" })}
          >
            {busy === "mine" ? "Baking…" : "Bake my parts"}
          </button>
          <AudienceTag audience="mine" />{/* T56: your personal variant */}
          {myConcert && (
            <a
              data-testid="bake-mine-download"
              href={myConcert.downloadUrl}
              download={`${myConcert.name || "my-parts"}.tstage`}
            >
              Download my parts (rev {myConcert.currentRev})
            </a>
          )}
        </div>
        <p className="muted">
          “My parts” bakes your own <em>my files</em> pick for each song. Annotations are the
          shared snapshot — they were made on the default part, so they may not line up with a
          different part’s layout.
        </p>
        <ErrorBanner message={error} />
        {warnings.length > 0 && (
          <div className="notice warn" data-testid="bake-warnings" role="status">
            Baked, with warnings:
            <ul style={{ margin: ".3rem 0 0", paddingLeft: "1.2rem" }}>
              {warnings.map((wm, i) => (
                <li key={i}>{wm}</li>
              ))}
            </ul>
          </div>
        )}
        {concerts.length > 0 && (
          <ul className="list" data-testid="bake-history">
            {concerts.map((c) => (
              <li key={c.concertId} data-testid="bake-history-row">
                <span>
                  <AudienceTag audience={c.concertId === setlistId ? "band" : "mine"} /> · Rev{" "}
                  {c.currentRev} · {c.songs.length} song{c.songs.length === 1 ? "" : "s"}
                  {c.bakedBy ? ` · by ${c.bakedBy}` : ""}
                </span>
                <span className="muted">
                  {c.updatedAt ? new Date(Number(c.updatedAt) * 1000).toLocaleString() : ""}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
      {dialog && (
        <BakeDialog
          bandId={bandId}
          setlistId={setlistId}
          songIds={songIds}
          scope={dialog.scope}
          onConfirm={(layerDefaults) => void bake(dialog.scope, layerDefaults)}
          onCancel={() => setDialog(null)}
        />
      )}
    </section>
  );
}

function SetlistMeta({
  bandId,
  setlist,
  onSaved,
}: {
  bandId: string;
  setlist: Setlist;
  onSaved: (s: Setlist) => void;
}) {
  const [name, setName] = useState(setlist.name);
  const [eventDate, setEventDate] = useState(setlist.eventDate ?? "");
  const [venue, setVenue] = useState(setlist.venue ?? "");
  const [notes, setNotes] = useState(setlist.notes ?? "");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSave(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      const updated = await api.updateSetlist(bandId, setlist.id, { name, eventDate, venue, notes });
      onSaved(updated);
      setNotice("Saved.");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel">
      <div className="panel-head">
        <h2>Details</h2>
      </div>
      <div className="panel-body">
        <form onSubmit={onSave} data-testid="setlist-meta-form">
          <div className="form-grid">
            <div className="field wide">
              <label htmlFor="sl-name">Name</label>
              <input
                id="sl-name"
                data-testid="sl-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
            </div>
            <div className="field">
              <label htmlFor="sl-eventDate">Event date</label>
              <input
                id="sl-eventDate"
                data-testid="sl-eventDate"
                type="date"
                value={eventDate}
                onChange={(e) => setEventDate(e.target.value)}
              />
            </div>
            <div className="field">
              <label htmlFor="sl-venue">Venue</label>
              <input
                id="sl-venue"
                data-testid="sl-venue"
                value={venue}
                onChange={(e) => setVenue(e.target.value)}
              />
            </div>
            <div className="field wide">
              <label htmlFor="sl-notes">Notes</label>
              <textarea
                id="sl-notes"
                data-testid="sl-notes"
                rows={3}
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
              />
            </div>
          </div>
          <div className="form-foot">
            <button type="submit" className="primary" data-testid="sl-save" disabled={busy}>
              {busy ? "Saving…" : "Save details"}
            </button>
            {notice && (
              <span className="saved" data-testid="sl-notice">
                ✓ {notice}
              </span>
            )}
          </div>
        </form>
        <ErrorBanner message={error} />
      </div>
    </section>
  );
}

// T52 — FLIP reorder motion. Rows register their element by id into ONE map (both the
// running-order and bench lists), so on each commit we measure every tracked row, and
// for any that moved we apply the inverse translate then transition it to zero — drag,
// ↑/↓, and ★ cross-group moves all animate uniformly, dependency-free, on every
// browser. `prefers-reduced-motion` skips the transforms (instant, as before).
// keyRe: a bare musical key, for the T60 transpose-checkbox greying (UX only — the
// transpose algorithm lives on the server). Mirrors app.TransposeEligible / ParseKey.
const keyRe = /^[A-G](#|b)?m?$/;

const FLIP_MS = 200;
function useFlipRows(dep: unknown): (id: string, el: HTMLElement | null) => void {
  const els = useRef(new Map<string, HTMLElement>());
  const prev = useRef(new Map<string, DOMRect>());
  const register = useCallback((id: string, el: HTMLElement | null) => {
    if (el) els.current.set(id, el);
    else els.current.delete(id);
  }, []);
  useLayoutEffect(() => {
    const reduce = window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches ?? false;
    const next = new Map<string, DOMRect>();
    els.current.forEach((el, id) => next.set(id, el.getBoundingClientRect()));
    if (!reduce) {
      next.forEach((r, id) => {
        const p = prev.current.get(id);
        if (!p) return; // newly mounted row — nothing to animate from
        const dx = p.left - r.left;
        const dy = p.top - r.top;
        if (Math.abs(dx) < 1 && Math.abs(dy) < 1) return;
        const el = els.current.get(id);
        if (!el) return;
        // Invert: jump back to the old position with no transition…
        el.style.transition = "none";
        el.style.transform = `translate(${dx}px, ${dy}px)`;
        el.getBoundingClientRect(); // force reflow so the jump is applied before playing
        // …then play forward to the natural position.
        requestAnimationFrame(() => {
          el.style.transition = `transform ${FLIP_MS}ms ease`;
          el.style.transform = "";
        });
      });
    }
    prev.current = next;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dep]);
  return register;
}

function Items({
  bandId,
  setlistId,
  items,
  songs,
  reload,
}: {
  bandId: string;
  setlistId: string;
  items: SetlistItem[];
  songs: Song[];
  reload: () => Promise<void>;
}) {
  const [songId, setSongId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // FLIP over the current item order (re-measures whenever the list changes).
  const registerRow = useFlipRows(items.map((it) => `${it.id}:${it.onCall ? "b" : "m"}`).join(","));

  async function addSong(e: FormEvent) {
    e.preventDefault();
    if (!songId) return;
    setError(null);
    setBusy(true);
    try {
      await api.addSetlistItem(bandId, setlistId, songId);
      setSongId("");
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to add song");
    } finally {
      setBusy(false);
    }
  }

  async function remove(itemId: string) {
    setError(null);
    try {
      await api.removeSetlistItem(bandId, setlistId, itemId);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to remove item");
    }
  }

  // Main running order vs the bench (on-call). The server already returns
  // main-then-bench; splitting here keeps the main numbering independent (T23).
  const main = items.filter((it) => !it.onCall);
  const bench = items.filter((it) => it.onCall);

  // T50: the caller's own cues per song (listSongs carries them as myCues) → the
  // glanceable "what to prepare" chips on each row.
  const cuesBySong = new Map<string, SongCue[]>();
  for (const s of songs) if (s.myCues?.length) cuesBySong.set(s.id, s.myCues);

  // move reorders WITHIN a group, then sends the full order (the other group
  // unchanged) since ReorderSetlist rewrites every item's position.
  async function move(group: "main" | "bench", index: number, dir: -1 | 1) {
    const arr = group === "main" ? main.slice() : bench.slice();
    const other = index + dir;
    if (other < 0 || other >= arr.length) return;
    const [moved] = arr.splice(index, 1);
    arr.splice(other, 0, moved);
    const full = group === "main" ? [...arr, ...bench] : [...main, ...arr];
    setError(null);
    try {
      await api.reorderSetlist(
        bandId,
        setlistId,
        full.map((i) => i.id),
      );
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to reorder");
    }
  }

  async function setOnCall(itemId: string, onCall: boolean) {
    setError(null);
    try {
      await api.updateSetlistItem(bandId, setlistId, itemId, { onCall });
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to move item");
    }
  }

  // Drag-to-reorder within a group (the grip handle is the drag source; each row is
  // a drop target). Same group only — cross-group moves use the ★ / "To order"
  // buttons. The ↑/↓ buttons remain as a keyboard/fallback path.
  const dragRef = useRef<{ group: "main" | "bench"; index: number } | null>(null);
  function onDragStart(group: "main" | "bench", index: number) {
    dragRef.current = { group, index };
  }
  // Only the dragged item's own group is a valid drop zone (cross-group moves use
  // ★ / "To order"). Used to gate both the drop and its highlight.
  function canDrop(group: "main" | "bench") {
    return dragRef.current?.group === group;
  }
  async function onDropRow(group: "main" | "bench", to: number) {
    const d = dragRef.current;
    dragRef.current = null;
    if (!d || d.group !== group || d.index === to) return;
    const arr = group === "main" ? main.slice() : bench.slice();
    const [moved] = arr.splice(d.index, 1);
    // The drop hint is the top border of the hovered row (= "land above this
    // row"). After removing the dragged item, a target BELOW the source shifted up
    // by one, so insert at to-1 to land where the hint shows; a target above keeps
    // its index. Without this, downward drops land one slot too low.
    const insertAt = d.index < to ? to - 1 : to;
    arr.splice(insertAt, 0, moved);
    const full = group === "main" ? [...arr, ...bench] : [...main, ...arr];
    setError(null);
    try {
      await api.reorderSetlist(
        bandId,
        setlistId,
        full.map((i) => i.id),
      );
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to reorder");
    }
  }

  return (
    <section className="panel">
      <div className="panel-head">
        <h2>Running order</h2>
        <span className="count">
          {main.length} song{main.length === 1 ? "" : "s"}
        </span>
      </div>

      {main.length === 0 ? (
        <p className="muted" data-testid="items-empty" style={{ padding: "1.15rem 1.25rem" }}>
          No songs in the running order yet — add one below.
        </p>
      ) : (
        <div className="rows" data-testid="items-list">
          {main.map((item, i) => (
            <ItemRow
              key={item.id}
              group="main"
              label={`${i + 1}.`}
              bandId={bandId}
              setlistId={setlistId}
              item={item}
              index={i}
              count={main.length}
              cues={cuesBySong.get(item.songId)}
              registerRef={registerRow}
              onMove={move}
              onRemove={remove}
              onSetOnCall={setOnCall}
              onDragStart={onDragStart}
              onDropRow={onDropRow}
              canDrop={canDrop}
              reload={reload}
            />
          ))}
        </div>
      )}

      <div className="bench-head">
        <span className="lbl">★ Bench · on call</span>
      </div>
      <p className="bench-note">
        Baked into the concert and jumpable on stage, but outside the running order and its
        numbering.
      </p>
      {bench.length === 0 ? (
        <p className="muted" data-testid="bench-empty" style={{ padding: "0 1.25rem .8rem" }}>
          No on-call songs. Use “To bench” on a running-order song to add one.
        </p>
      ) : (
        <div className="rows bench" data-testid="bench-list">
          {bench.map((item, i) => (
            <ItemRow
              key={item.id}
              group="bench"
              label="★"
              bandId={bandId}
              setlistId={setlistId}
              item={item}
              index={i}
              count={bench.length}
              cues={cuesBySong.get(item.songId)}
              registerRef={registerRow}
              onMove={move}
              onRemove={remove}
              onSetOnCall={setOnCall}
              onDragStart={onDragStart}
              onDropRow={onDropRow}
              canDrop={canDrop}
              reload={reload}
            />
          ))}
        </div>
      )}

      <form onSubmit={addSong} className="addbar" data-testid="add-item-form">
        <select
          data-testid="add-item-song"
          value={songId}
          onChange={(e) => setSongId(e.target.value)}
          required
        >
          <option value="">Add a song from the band library…</option>
          {songs.map((s) => (
            <option key={s.id} value={s.id}>
              {s.title}
              {s.artist ? ` — ${s.artist}` : ""}
            </option>
          ))}
        </select>
        <button type="submit" data-testid="add-item" disabled={busy}>
          Add to order
        </button>
      </form>

      <div style={{ padding: "0 1.25rem 1rem" }}>
        <ErrorBanner message={error} />
      </div>
    </section>
  );
}

function ItemRow({
  group,
  label,
  bandId,
  setlistId,
  item,
  index,
  count,
  cues,
  onMove,
  onRemove,
  onSetOnCall,
  onDragStart,
  onDropRow,
  canDrop,
  reload,
  registerRef,
}: {
  group: "main" | "bench";
  label: string;
  bandId: string;
  setlistId: string;
  item: SetlistItem;
  index: number;
  count: number;
  cues?: SongCue[];
  onMove: (group: "main" | "bench", index: number, dir: -1 | 1) => void;
  onRemove: (itemId: string) => void;
  onSetOnCall: (itemId: string, onCall: boolean) => void;
  onDragStart: (group: "main" | "bench", index: number) => void;
  onDropRow: (group: "main" | "bench", index: number) => void | Promise<void>;
  canDrop: (group: "main" | "bench") => boolean;
  reload: () => Promise<void>;
  registerRef: (id: string, el: HTMLElement | null) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const [keyOverride, setKeyOverride] = useState(item.keyOverride ?? "");
  const [tempoOverride, setTempoOverride] = useState(
    item.tempoOverride != null && item.tempoOverride !== 0 ? String(item.tempoOverride) : "",
  );
  const [notes, setNotes] = useState(item.notes ?? "");
  const [transposeChords, setTransposeChords] = useState(item.transposeChords ?? false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  // T60 transpose-checkbox greying. The song key + generated-chart presence come from
  // the item view (they don't change while editing); the override is parsed live so the
  // checkbox enables the moment a valid override is typed. reason names the first gap.
  const songKeyOk = keyRe.test((item.songKey ?? "").trim());
  const overrideOk = keyRe.test(keyOverride.trim());
  const transposeEligible = Boolean(item.hasChart) && songKeyOk && overrideOk;
  const transposeReason = !item.hasChart
    ? "no text chart on this song"
    : !songKeyOk
      ? "song key not set or not parseable"
      : !overrideOk
        ? "override key not parseable"
        : "transpose the chords from the song key to this override at bake";

  async function save() {
    setError(null);
    setBusy(true);
    try {
      await api.updateSetlistItem(bandId, setlistId, item.id, {
        keyOverride,
        tempoOverride: tempoOverride === "" ? 0 : Number(tempoOverride),
        notes,
        transposeChords,
      });
      setEditing(false);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save item");
    } finally {
      setBusy(false);
    }
  }

  function cancel() {
    setKeyOverride(item.keyOverride ?? "");
    setTempoOverride(item.tempoOverride ? String(item.tempoOverride) : "");
    setNotes(item.notes ?? "");
    setTransposeChords(item.transposeChords ?? false);
    setEditing(false);
  }

  return (
    <div
      ref={(el) => registerRef(item.id, el)}
      className={`row${editing ? " editing" : ""}${dragOver ? " drag-over" : ""}`}
      data-testid={group === "bench" ? "bench-row" : "item-row"}
      onDragOver={(e) => {
        if (!canDrop(group)) return; // cross-group hover: not a drop target, no hint
        e.preventDefault();
        e.dataTransfer.dropEffect = "move";
        setDragOver(true);
      }}
      onDragLeave={(e) => {
        // Only clear when the pointer truly leaves the row — dragleave also fires when
        // it crosses a CHILD (grip, buttons, cue chips) still inside the row, which
        // made the blue hint flicker (T52).
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setDragOver(false);
      }}
      onDrop={(e) => {
        e.preventDefault();
        setDragOver(false);
        void onDropRow(group, index);
      }}
    >
      <span
        className="grip"
        draggable
        data-testid="item-grip"
        title="Drag to reorder"
        aria-label="Drag to reorder"
        onDragStart={(e) => {
          e.dataTransfer.effectAllowed = "move";
          onDragStart(group, index);
        }}
      >
        ⠿
      </span>
      <div className="song">
        <div className="name" data-testid="item-title">
          {/* T61: the title links to the song's editor. The running-order number stays
              plain text; drag-to-reorder is grip-only so the link never intercepts it.
              draggable=false stops the browser's own anchor-drag. A real <Link> gives
              middle/ctrl-click + keyboard for free (hover-only affordance, no blue noise). */}
          {label}{" "}
          <Link
            to={`/bands/${bandId}/songs/${item.songId}`}
            className="item-title-link"
            data-testid="item-title-link"
            draggable={false}
          >
            {item.songTitle ?? item.songId}
          </Link>
        </div>
        {item.songArtist && <div className="by">{item.songArtist}</div>}
        {cues && cues.length > 0 && (
          <div className="cue-row" data-testid="item-cues" aria-label="My cues">
            {cues.map((c, ci) => (
              <CueGlyph key={`${c.icon}-${ci}`} icon={c.icon ?? ""} color={c.color} size={18} />
            ))}
          </div>
        )}
      </div>

      <div className="tags">
        {item.keyOverride && <span className="chip mono">{item.keyOverride}</span>}
        {item.tempoOverride ? <span className="chip mono">{item.tempoOverride} bpm</span> : null}
      </div>

      <div className="rowacts">
        <button
          type="button"
          className="icon-btn"
          data-testid="item-edit"
          title="Edit key / tempo / notes"
          aria-expanded={editing}
          onClick={() => setEditing((v) => !v)}
        >
          ✎
        </button>
        <button
          type="button"
          className="icon-btn"
          data-testid="item-up"
          title="Move up"
          disabled={index === 0}
          onClick={() => onMove(group, index, -1)}
        >
          ↑
        </button>
        <button
          type="button"
          className="icon-btn"
          data-testid="item-down"
          title="Move down"
          disabled={index === count - 1}
          onClick={() => onMove(group, index, 1)}
        >
          ↓
        </button>
        {group === "main" ? (
          <button
            type="button"
            className="icon-btn"
            data-testid="item-tobench"
            title="Move to the bench (on call, outside the running order)"
            onClick={() => onSetOnCall(item.id, true)}
          >
            ★
          </button>
        ) : (
          <button
            type="button"
            className="btn-sm"
            data-testid="item-tomain"
            title="Move back into the running order"
            onClick={() => onSetOnCall(item.id, false)}
          >
            To order
          </button>
        )}
        <button
          type="button"
          className="icon-btn"
          data-testid="item-remove"
          title="Remove from setlist"
          onClick={() => onRemove(item.id)}
        >
          ✕
        </button>
      </div>

      {editing && (
        <div className="row-edit">
          <div className="form-grid">
            <div className="field">
              <label>Key</label>
              <input
                data-testid="item-key"
                placeholder="e.g. Bb"
                value={keyOverride}
                onChange={(e) => setKeyOverride(e.target.value)}
              />
            </div>
            <div className="field">
              <label>Tempo</label>
              <div className="input-affix">
                <input
                  data-testid="item-tempo"
                  type="number"
                  placeholder="—"
                  value={tempoOverride}
                  onChange={(e) => setTempoOverride(e.target.value)}
                />
                <span className="affix">bpm</span>
              </div>
            </div>
            <div className="field">
              <label>Performance note</label>
              <input
                data-testid="item-notes"
                placeholder="e.g. half-time feel"
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
              />
            </div>
            {/* T60: transpose the chart's chords to the key override, burned at bake.
                Greyed (with the reason) unless the song has a chart and both keys parse. */}
            <div className="field">
              <label>Chords</label>
              <label
                className={`check${transposeEligible ? "" : " is-disabled"}`}
                data-testid="item-transpose-label"
                title={transposeReason}
              >
                <input
                  type="checkbox"
                  data-testid="item-transpose"
                  checked={transposeChords}
                  disabled={!transposeEligible}
                  onChange={(e) => setTransposeChords(e.target.checked)}
                />
                transpose chords
              </label>
            </div>
          </div>
          <div className="form-foot">
            <button type="button" className="primary btn-sm" data-testid="item-save" disabled={busy} onClick={save}>
              {busy ? "Saving…" : "Save"}
            </button>
            {item.hasChart && (
              <a
                className="btn-sm ghost-btn"
                data-testid="item-chart-preview"
                href={`/api/bands/${bandId}/setlists/${setlistId}/items/${item.id}/chart-preview`}
                target="_blank"
                rel="noreferrer"
                title="Preview this chart as it will bake (transposed if enabled + saved)"
              >
                Preview chart
              </a>
            )}
            <button type="button" className="btn-sm ghost-btn" onClick={cancel}>
              Cancel
            </button>
          </div>
          {error ? <ErrorBanner message={error} /> : null}
        </div>
      )}
    </div>
  );
}

// DuplicateAction (T20): member-visible — deep-copy this setlist ("… (copy)", same
// songs + overrides) and jump to the copy. Handy for "same as last month, swap two".
function DuplicateAction({
  bandId,
  setlistId,
  onDuplicated,
}: {
  bandId: string;
  setlistId: string;
  onDuplicated: (newId: string) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function duplicate() {
    setBusy(true);
    setError(null);
    try {
      const copy = await api.duplicateSetlist(bandId, setlistId);
      onDuplicated(copy.id);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to duplicate");
      setBusy(false);
    }
  }

  return (
    <section className="panel">
      <div className="panel-body">
        <div className="inline-form">
          <button type="button" data-testid="duplicate-setlist" disabled={busy} onClick={duplicate}>
            {busy ? "Duplicating…" : "Duplicate setlist"}
          </button>
          <span className="muted">
            Make an editable copy — same songs, order and per-song overrides.
          </span>
        </div>
        <ErrorBanner message={error} />
      </div>
    </section>
  );
}

function DeleteSetlist({
  bandId,
  setlistId,
  onDeleted,
}: {
  bandId: string;
  setlistId: string;
  onDeleted: () => void;
}) {
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onDelete() {
    if (!window.confirm("Delete this setlist?")) return;
    setError(null);
    setBusy(true);
    try {
      await api.deleteSetlist(bandId, setlistId);
      onDeleted();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to delete setlist");
      setBusy(false);
    }
  }

  return (
    <section className="panel">
      <div className="panel-body">
        <h2 style={{ marginTop: 0 }}>Danger zone</h2>
        <div className="inline-form">
          <button type="button" data-testid="delete-setlist" disabled={busy} onClick={onDelete}>
            Delete setlist
          </button>
        </div>
        <ErrorBanner message={error} />
      </div>
    </section>
  );
}
