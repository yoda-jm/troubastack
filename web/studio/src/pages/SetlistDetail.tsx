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
      <DuplicateAction
        bandId={bandId}
        setlistId={setlistId}
        onDuplicated={(id) => navigate(`/bands/${bandId}/setlists/${id}`)}
      />
      <BakeCard bandId={bandId} setlistId={setlistId} myRole={myRole} />
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
function BakeCard({
  bandId,
  setlistId,
  myRole,
}: {
  bandId: string;
  setlistId: string;
  myRole: Role | null;
}) {
  const [concerts, setConcerts] = useState<Concert[]>([]);
  const [busy, setBusy] = useState<"" | "band" | "mine">("");
  const [error, setError] = useState<string | null>(null);
  const isAdmin = myRole === "admin";

  // A concert belongs to this setlist if it IS the band concert (id === setlist)
  // or the caller's variant (id starts with `${setlist}~`). The server only ever
  // returns the caller's own variants, so any `~` match here is mine.
  const load = useCallback(async () => {
    try {
      const all = await api.listConcerts(bandId);
      setConcerts(all.filter((c) => c.concertId === setlistId || c.concertId.startsWith(`${setlistId}~`)));
    } catch {
      // A missing/empty concert list is not an error worth surfacing here.
    }
  }, [bandId, setlistId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function bake(scope?: "mine") {
    setBusy(scope === "mine" ? "mine" : "band");
    setError(null);
    try {
      await api.bakeSetlist(bandId, setlistId, scope);
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
    <section className="card" data-testid="bake-card">
      <h2>Bake</h2>
      <p className="muted">
        Flatten this setlist into a performable <code>.tstage</code> bundle (page images +
        annotation overlays) to download and load on a phone.
      </p>
      {isAdmin && (
        <div className="inline-form">
          <button type="button" data-testid="bake-setlist" disabled={busy !== ""} onClick={() => void bake()}>
            {busy === "band" ? "Baking…" : "Bake setlist"}
          </button>
          {bandConcert && (
            <a
              data-testid="bake-download"
              href={bandConcert.downloadUrl}
              download={`${bandConcert.name || "concert"}.tstage`}
            >
              Download .tstage (rev {bandConcert.currentRev})
            </a>
          )}
        </div>
      )}
      <div className="inline-form">
        <button type="button" data-testid="bake-mine" disabled={busy !== ""} onClick={() => void bake("mine")}>
          {busy === "mine" ? "Baking…" : "Bake my parts"}
        </button>
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
      {concerts.length > 0 && (
        <ul className="list" data-testid="bake-history">
          {concerts.map((c) => (
            <li key={c.concertId} data-testid="bake-history-row">
              <span>
                {c.concertId === setlistId ? "Band" : "My parts"} · Rev {c.currentRev} ·{" "}
                {c.songs.length} song{c.songs.length === 1 ? "" : "s"}
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

  // Main running order vs the bench (on-call). The server already returns
  // main-then-bench; splitting here keeps the main numbering independent of the
  // bench (T23).
  const main = items.filter((it) => !it.onCall);
  const bench = items.filter((it) => it.onCall);

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

      {main.length === 0 ? (
        <p className="muted" data-testid="items-empty">
          No songs in the running order yet.
        </p>
      ) : (
        <ul className="list" data-testid="items-list">
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
              onMove={move}
              onRemove={remove}
              onSetOnCall={setOnCall}
              reload={reload}
            />
          ))}
        </ul>
      )}

      <h3 style={{ marginTop: "1rem" }}>Bench (on call)</h3>
      <p className="muted">
        Encores and likely requests: baked into the concert and jumpable on stage, but
        outside the running order and its numbering.
      </p>
      {bench.length === 0 ? (
        <p className="muted" data-testid="bench-empty">
          No on-call songs. Use “To bench” on a song above to add one.
        </p>
      ) : (
        <ul className="list" data-testid="bench-list">
          {bench.map((item, i) => (
            <ItemRow
              key={item.id}
              group="bench"
              label="•"
              bandId={bandId}
              setlistId={setlistId}
              item={item}
              index={i}
              count={bench.length}
              onMove={move}
              onRemove={remove}
              onSetOnCall={setOnCall}
              reload={reload}
            />
          ))}
        </ul>
      )}
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
  onMove,
  onRemove,
  onSetOnCall,
  reload,
}: {
  group: "main" | "bench";
  label: string;
  bandId: string;
  setlistId: string;
  item: SetlistItem;
  index: number;
  count: number;
  onMove: (group: "main" | "bench", index: number, dir: -1 | 1) => void;
  onRemove: (itemId: string) => void;
  onSetOnCall: (itemId: string, onCall: boolean) => void;
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
    <li data-testid={group === "bench" ? "bench-row" : "item-row"} style={{ flexWrap: "wrap" }}>
      <span data-testid="item-title">
        {label} {item.songTitle ?? item.songId}
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
          onClick={() => onMove(group, index, -1)}
        >
          ↑
        </button>
        <button
          type="button"
          data-testid="item-down"
          disabled={index === count - 1}
          onClick={() => onMove(group, index, 1)}
        >
          ↓
        </button>
        {group === "main" ? (
          <button
            type="button"
            data-testid="item-tobench"
            title="Move to the bench (on call, outside the running order)"
            onClick={() => onSetOnCall(item.id, true)}
          >
            To bench
          </button>
        ) : (
          <button
            type="button"
            data-testid="item-tomain"
            title="Move back into the running order"
            onClick={() => onSetOnCall(item.id, false)}
          >
            To order
          </button>
        )}
        <button type="button" data-testid="item-remove" onClick={() => onRemove(item.id)}>
          Remove
        </button>
      </span>
      {error ? <ErrorBanner message={error} /> : null}
    </li>
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
    <section className="card">
      <div className="inline-form">
        <button type="button" data-testid="duplicate-setlist" disabled={busy} onClick={duplicate}>
          {busy ? "Duplicating…" : "Duplicate setlist"}
        </button>
        <span className="muted">Make an editable copy — same songs, order and per-song overrides.</span>
      </div>
      <ErrorBanner message={error} />
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
