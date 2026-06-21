import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import { ApiError, api, type Band, type Invite, type MemberView, type Role, type Song } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { Avatar } from "../components/Avatar";

export function BandDetail() {
  const { bandId } = useParams<{ bandId: string }>();
  const [band, setBand] = useState<Band | null>(null);
  const [myRole, setMyRole] = useState<Role | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadBand = useCallback(async () => {
    if (!bandId) return;
    try {
      const { band, myRole } = await api.getBand(bandId);
      setBand(band);
      setMyRole(myRole);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load band");
    }
  }, [bandId]);

  useEffect(() => {
    void loadBand();
  }, [loadBand]);

  if (error && !band) {
    return (
      <div className="page">
        <Link to="/bands">&larr; Bands</Link>
        <ErrorBanner message={error} />
      </div>
    );
  }

  if (!band || !bandId) {
    return <div className="page">Loading…</div>;
  }

  return (
    <div className="page">
      <Link to="/bands">&larr; Bands</Link>
      <h1 data-testid="band-title">{band.name}</h1>
      <p className="muted">
        Your role: <strong data-testid="my-role">{myRole}</strong>
      </p>

      <nav className="inline-form">
        <Link to={`/bands/${bandId}/setlists`} data-testid="nav-setlists">
          Setlists
        </Link>
        {myRole === "admin" && (
          <Link to={`/bands/${bandId}/settings`} data-testid="nav-settings">
            Settings
          </Link>
        )}
      </nav>

      <Members bandId={bandId} myRole={myRole} />
      <Songs bandId={bandId} />
    </div>
  );
}

function Members({ bandId, myRole }: { bandId: string; myRole: Role | null }) {
  const [members, setMembers] = useState<MemberView[]>([]);
  const [identifier, setIdentifier] = useState("");
  const [kind, setKind] = useState<Invite["kind"]>("username");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      setMembers(await api.members(bandId));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load members");
    }
  }, [bandId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onInvite(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      await api.invite(bandId, identifier, kind);
      setIdentifier("");
      setNotice(`Invited ${kind} "${identifier}".`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to invite");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="card">
      <h2>Members</h2>
      <ul className="list" data-testid="members-list">
        {members.map((m) => (
          <li key={m.user.id} data-testid="member-row">
            <Avatar user={m.user} size={26} />{" "}
            <span>{m.user.displayName}</span> <span className="muted">@{m.user.username}</span>{" "}
            <span className="pill">{m.role}</span>
          </li>
        ))}
      </ul>

      {myRole === "admin" && (
        <form onSubmit={onInvite} className="inline-form" data-testid="invite-form">
          <input
            data-testid="invite-identifier"
            placeholder="username / email / uuid"
            value={identifier}
            onChange={(e) => setIdentifier(e.target.value)}
            required
          />
          <select
            data-testid="invite-kind"
            value={kind}
            onChange={(e) => setKind(e.target.value as Invite["kind"])}
          >
            <option value="username">username</option>
            <option value="email">email</option>
            <option value="uuid">uuid</option>
          </select>
          <button type="submit" data-testid="invite-submit" disabled={busy}>
            Invite
          </button>
        </form>
      )}

      {notice && (
        <p className="notice" data-testid="invite-notice">
          {notice}
        </p>
      )}
      <ErrorBanner message={error} />
    </section>
  );
}

function Songs({ bandId }: { bandId: string }) {
  const [songs, setSongs] = useState<Song[]>([]);
  const [title, setTitle] = useState("");
  const [artist, setArtist] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      setSongs(await api.listSongs(bandId));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load songs");
    }
  }, [bandId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await api.createSong(bandId, title, artist || undefined);
      setTitle("");
      setArtist("");
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to create song");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="card">
      <h2>Songs</h2>

      <form onSubmit={onCreate} className="inline-form">
        <input
          data-testid="song-title"
          placeholder="Song title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          required
        />
        <input
          data-testid="song-artist"
          placeholder="Artist (optional)"
          value={artist}
          onChange={(e) => setArtist(e.target.value)}
        />
        <button type="submit" data-testid="create-song" disabled={busy}>
          Add song
        </button>
      </form>

      <ErrorBanner message={error} />

      {songs.length === 0 ? (
        <p className="muted" data-testid="songs-empty">
          No songs yet.
        </p>
      ) : (
        <ul className="list" data-testid="songs-list">
          {songs.map((s) => (
            <li key={s.id}>
              <Link to={`/bands/${bandId}/songs/${s.id}`} data-testid="song-link">
                {s.title}
                {s.artist ? <span className="muted"> — {s.artist}</span> : null}
              </Link>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
