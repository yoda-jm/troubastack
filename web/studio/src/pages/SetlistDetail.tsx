/**
 * Setlist detail — edit the setlist metadata, manage its ordered items (add a
 * band song, per-item key/tempo/notes overrides, reorder, remove) and delete.
 */
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  ApiError,
  api,
  type Concert,
  type Role,
  type Setlist,
  type SetlistItem,
  type Song,
} from "../api";
import { ErrorBanner } from "../components/ErrorBanner";

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
      items.sort((a, b) => a.position - b.position);
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
        <Link to={`/bands/${bandId}/setlists`}>&larr; Setlists</Link>
        <ErrorBanner message={error} />
      </div>
    );
  }
  if (!setlist) return <div className="page">Loading…</div>;

  return (
    <div className="page">
      <Link to={`/bands/${bandId}/setlists`}>&larr; Setlists</Link>
      <h1 data-testid="setlist-detail-title">{setlist.name}</h1>

      <SetlistMeta bandId={bandId} setlist={setlist} onSaved={setSetlist} />
      <Items
        bandId={bandId}
        setlistId={setlistId}
        items={items}
        songs={songs}
        reload={load}
      />
      {myRole === "admin" && <BakeCard bandId={bandId} setlistId={setlistId} />}
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

// BakeCard (B02): admin bakes this setlist into a downloadable .tstage (I11), with
// a download link for the latest bake and a short history. One card, no new route.
function BakeCard({ bandId, setlistId }: { bandId: string; setlistId: string }) {
  const [concerts, setConcerts] = useState<Concert[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadHistory = useCallback(async () => {
    try {
      const all = await api.listConcerts(bandId);
      setConcerts(
        all
          .filter((c) => c.concertId === setlistId)
          .sort((a, b) => Number(b.currentRev) - Number(a.currentRev)),
      );
    } catch {
      // A missing/empty concert list is not an error worth surfacing here.
    }
  }, [bandId, setlistId]);

  useEffect(() => {
    void loadHistory();
  }, [loadHistory]);

  async function bake() {
    setBusy(true);
    setError(null);
    try {
      await api.bakeSetlist(bandId, setlistId);
      await loadHistory();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Bake failed");
    } finally {
      setBusy(false);
    }
  }

  const latest = concerts[0] ?? null;

  return (
    <section className="card" data-testid="bake-card">
      <h2>Bake</h2>
      <p className="muted">
        Flatten this setlist into a performable <code>.tstage</code> bundle (page images +
        annotation overlays) to download and load on a phone.
      </p>
      <div className="inline-form">
        <button type="button" data-testid="bake-setlist" disabled={busy} onClick={bake}>
          {busy ? "Baking…" : "Bake setlist"}
        </button>
        {latest && (
          <a
            data-testid="bake-download"
            href={latest.downloadUrl}
            download={`${latest.name || "concert"}.tstage`}
          >
            Download .tstage (rev {latest.currentRev})
          </a>
        )}
      </div>
      <ErrorBanner message={error} />
      {concerts.length > 0 && (
        <ul className="list" data-testid="bake-history">
          {concerts.map((c) => (
            <li key={c.currentRev} data-testid="bake-history-row">
              <span>
                Rev {c.currentRev} · {c.songs.length} song{c.songs.length === 1 ? "" : "s"}
                {c.bakedBy ? ` · by ${c.bakedBy}` : ""}
              </span>
              <span className="muted">
                {c.updatedAt ? new Date(Number(c.updatedAt) * 1000).toLocaleString() : ""}
              </span>
            </li>
          ))}
        </ul>
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
      const updated = await api.updateSetlist(bandId, setlist.id, {
        name,
        eventDate,
        venue,
        notes,
      });
      onSaved(updated);
      setNotice("Saved.");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="card">
      <h2>Details</h2>
      <form onSubmit={onSave} data-testid="setlist-meta-form">
        <label>
          Name
          <input
            data-testid="sl-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        </label>
        <label>
          Event date
          <input
            data-testid="sl-eventDate"
            type="date"
            value={eventDate}
            onChange={(e) => setEventDate(e.target.value)}
          />
        </label>
        <label>
          Venue
          <input data-testid="sl-venue" value={venue} onChange={(e) => setVenue(e.target.value)} />
        </label>
        <label>
          Notes
          <textarea
            data-testid="sl-notes"
            rows={3}
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
          />
        </label>
        <div className="inline-form">
          <button type="submit" data-testid="sl-save" disabled={busy}>
            Save details
          </button>
          {notice && (
            <span className="notice" data-testid="sl-notice">
              {notice}
            </span>
          )}
        </div>
      </form>
      <ErrorBanner message={error} />
    </section>
  );
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

  async function move(index: number, dir: -1 | 1) {
    const other = index + dir;
    if (other < 0 || other >= items.length) return;
    const reordered = items.slice();
    const [moved] = reordered.splice(index, 1);
    reordered.splice(other, 0, moved);
    setError(null);
    try {
      await api.reorderSetlist(
        bandId,
        setlistId,
        reordered.map((i) => i.id),
      );
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to reorder");
    }
  }

  return (
    <section className="card">
      <h2>Songs</h2>

      <form onSubmit={addSong} className="inline-form" data-testid="add-item-form">
        <select
          data-testid="add-item-song"
          value={songId}
          onChange={(e) => setSongId(e.target.value)}
          required
        >
          <option value="">Select a song…</option>
          {songs.map((s) => (
            <option key={s.id} value={s.id}>
              {s.title}
              {s.artist ? ` — ${s.artist}` : ""}
            </option>
          ))}
        </select>
        <button type="submit" data-testid="add-item" disabled={busy}>
          Add song
        </button>
      </form>

      <ErrorBanner message={error} />

      {items.length === 0 ? (
        <p className="muted" data-testid="items-empty">
          No songs in this setlist yet.
        </p>
      ) : (
        <ul className="list" data-testid="items-list">
          {items.map((item, i) => (
            <ItemRow
              key={item.id}
              bandId={bandId}
              setlistId={setlistId}
              item={item}
              index={i}
              count={items.length}
              onMove={move}
              onRemove={remove}
              reload={reload}
            />
          ))}
        </ul>
      )}
    </section>
  );
}

function ItemRow({
  bandId,
  setlistId,
  item,
  index,
  count,
  onMove,
  onRemove,
  reload,
}: {
  bandId: string;
  setlistId: string;
  item: SetlistItem;
  index: number;
  count: number;
  onMove: (index: number, dir: -1 | 1) => void;
  onRemove: (itemId: string) => void;
  reload: () => Promise<void>;
}) {
  const [keyOverride, setKeyOverride] = useState(item.keyOverride ?? "");
  const [tempoOverride, setTempoOverride] = useState(
    item.tempoOverride != null ? String(item.tempoOverride) : "",
  );
  const [notes, setNotes] = useState(item.notes ?? "");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function save() {
    setError(null);
    setBusy(true);
    try {
      await api.updateSetlistItem(bandId, setlistId, item.id, {
        keyOverride,
        tempoOverride: tempoOverride === "" ? 0 : Number(tempoOverride),
        notes,
      });
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save item");
    } finally {
      setBusy(false);
    }
  }

  return (
    <li data-testid="item-row" style={{ flexWrap: "wrap" }}>
      <span data-testid="item-title">
        {index + 1}. {item.songTitle ?? item.songId}
        {item.songArtist ? <span className="muted"> — {item.songArtist}</span> : null}
      </span>
      <span className="actions">
        <input
          data-testid="item-key"
          placeholder="Key"
          style={{ width: "5rem" }}
          value={keyOverride}
          onChange={(e) => setKeyOverride(e.target.value)}
        />
        <input
          data-testid="item-tempo"
          type="number"
          placeholder="BPM"
          style={{ width: "5.5rem" }}
          value={tempoOverride}
          onChange={(e) => setTempoOverride(e.target.value)}
        />
        <input
          data-testid="item-notes"
          placeholder="Notes"
          value={notes}
          onChange={(e) => setNotes(e.target.value)}
        />
        <button type="button" data-testid="item-save" disabled={busy} onClick={save}>
          Save
        </button>
        <button
          type="button"
          data-testid="item-up"
          disabled={index === 0}
          onClick={() => onMove(index, -1)}
        >
          ↑
        </button>
        <button
          type="button"
          data-testid="item-down"
          disabled={index === count - 1}
          onClick={() => onMove(index, 1)}
        >
          ↓
        </button>
        <button type="button" data-testid="item-remove" onClick={() => onRemove(item.id)}>
          Remove
        </button>
      </span>
      {error ? <ErrorBanner message={error} /> : null}
    </li>
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
    <section className="card">
      <h2>Danger zone</h2>
      <div className="inline-form">
        <button type="button" data-testid="delete-setlist" disabled={busy} onClick={onDelete}>
          Delete setlist
        </button>
      </div>
      <ErrorBanner message={error} />
    </section>
  );
}
