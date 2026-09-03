import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import QRCode from "qrcode";
import {
  ApiError,
  api,
  type Band,
  type ImportReport,
  type Invite,
  type MemberView,
  type Role,
  type Song,
} from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { Avatar } from "../components/Avatar";
import { NewItem } from "../components/NewItem";
import { foldText } from "../foldText";
import { SectionTabs } from "../components/SectionTabs";

/** Sentence-case a short enum label (role, zone) for display. */
function label(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

export function BandDetail() {
  const { bandId } = useParams<{ bandId: string }>();
  const location = useLocation();
  const [band, setBand] = useState<Band | null>(null);
  const [myRole, setMyRole] = useState<Role | null>(null);
  const [error, setError] = useState<string | null>(null);
  // A just-completed import (T62) navigates here with its report in router state.
  const importReport = (location.state as { importReport?: ImportReport } | null)?.importReport;
  const [showReport, setShowReport] = useState(true);

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
      <Link className="crumb" to="/bands">
        &larr; Bands
      </Link>
      <header className="phead">
        <div>
          <div className="eyebrow">Band</div>
          <h1 className="title" data-testid="band-title">
            {band.name}
          </h1>
          <div className="sub">
            Your role: <strong data-testid="my-role">{myRole}</strong>
          </div>
        </div>
      </header>

      <SectionTabs bandId={bandId} active="overview" showSettings={myRole === "admin"} />

      {importReport && showReport && (
        <ImportSummary report={importReport} onDismiss={() => setShowReport(false)} />
      )}

      <Members bandId={bandId} myRole={myRole} />
      <Songs bandId={bandId} />
    </div>
  );
}

/**
 * ImportSummary reports what a just-finished band import (T62) brought in and,
 * crucially, which member accounts were freshly created — those people can't sign
 * in until an admin issues each a reset link (below, on every member row).
 */
function ImportSummary({ report, onDismiss }: { report: ImportReport; onDismiss: () => void }) {
  const matched = report.matched ?? [];
  const created = report.created ?? [];
  const invited = report.invited ?? [];
  const skipped = report.skipped ?? [];
  const dropped =
    (report.droppedLayers ?? 0) +
    (report.droppedObjects ?? 0) +
    (report.droppedCues ?? 0) +
    (report.droppedSelections ?? 0);
  return (
    <section className="panel" data-testid="import-report">
      <div className="panel-head">
        <h2>Import complete</h2>
        <button type="button" className="ghost-btn" onClick={onDismiss} data-testid="import-report-dismiss">
          Dismiss
        </button>
      </div>
      <div className="panel-body">
        <p style={{ marginTop: 0 }}>
          Imported <strong>{report.songs}</strong> song{report.songs === 1 ? "" : "s"} (
          {report.files} file{report.files === 1 ? "" : "s"}) and <strong>{report.setlists}</strong>{" "}
          setlist{report.setlists === 1 ? "" : "s"}.
        </p>
        {matched.length > 0 && (
          <p className="muted" data-testid="import-matched">
            Attached to existing accounts: {matched.join(", ")}.
          </p>
        )}
        {created.length > 0 && (
          <p className="notice" data-testid="import-created">
            New accounts created (no password yet — issue each a reset link below so they can sign
            in): {created.join(", ")}.
          </p>
        )}
        {invited.length > 0 && (
          <p className="muted" data-testid="import-invited">
            Invited (they join when they next sign in): {invited.join(", ")}.
          </p>
        )}
        {skipped.length > 0 && (
          <p className="muted" data-testid="import-skipped">
            Skipped: {skipped.join(", ")}.
          </p>
        )}
        {dropped > 0 && (
          <p className="muted" data-testid="import-dropped">
            {dropped} personal annotation/cue item{dropped === 1 ? "" : "s"} from invited/skipped
            members were not imported (shared and conductor markings were kept).
          </p>
        )}
      </div>
    </section>
  );
}

function Members({ bandId, myRole }: { bandId: string; myRole: Role | null }) {
  const [members, setMembers] = useState<MemberView[]>([]);
  const [identifier, setIdentifier] = useState("");
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

  async function onInvite(e: FormEvent): Promise<boolean> {
    e.preventDefault();
    setError(null);
    setNotice(null);
    setBusy(true);
    // Auto-detect the identifier kind: an "@" means email, otherwise a username.
    // (The server still accepts a raw uuid; we just never surface it in the UI.)
    const kind: Invite["kind"] = identifier.includes("@") ? "email" : "username";
    try {
      await api.invite(bandId, identifier, kind);
      setIdentifier("");
      setNotice(`Invited "${identifier}".`);
      return true;
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to invite");
      return false;
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel">
      <div className="panel-head">
        <h2>Members</h2>
        <span className="count">{members.length}</span>
      </div>
      <div className="panel-body">
        {myRole === "admin" && (
          <div className="panel-toolbar">
            <NewItem label="Invite member" testId="invite-toggle">
              {(close) => (
                <form
                  onSubmit={(e) => void onInvite(e).then((ok) => ok && close())}
                  className="inline-form"
                  data-testid="invite-form"
                >
                  <input
                    data-testid="invite-identifier"
                    placeholder="Username or email"
                    value={identifier}
                    onChange={(e) => setIdentifier(e.target.value)}
                    required
                  />
                  <button type="submit" className="primary" data-testid="invite-submit" disabled={busy}>
                    Invite
                  </button>
                  <button type="button" className="ghost-btn" onClick={close}>
                    Cancel
                  </button>
                </form>
              )}
            </NewItem>
          </div>
        )}
        <ul className="list member-list" data-testid="members-list">
          {members.map((m) => (
            <li key={m.user.id} data-testid="member-row" className="member-row">
              <span className="member-identity">
                <Avatar user={m.user} size={30} />
                <span className="member-name">{m.user.displayName}</span>
                <span className="muted member-handle">@{m.user.username}</span>
              </span>
              <span className="chip member-role">{label(m.role)}</span>
              {myRole === "admin" && <MemberResetAction bandId={bandId} userId={m.user.id} />}
            </li>
          ))}
        </ul>

        {notice && (
          <p className="notice" data-testid="invite-notice">
            {notice}
          </p>
        )}
        <ErrorBanner message={error} />
      </div>
    </section>
  );
}

/**
 * MemberResetAction (admin) mints a one-time password-reset link for a member
 * and shows the full URL to hand over out-of-band — the same trust model as
 * invite links (T21). There is no email pipeline; copying the link IS delivery.
 */
function MemberResetAction({ bandId, userId }: { bandId: string; userId: string }) {
  const [link, setLink] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onIssue() {
    setError(null);
    setBusy(true);
    try {
      const { resetPath } = await api.issuePasswordReset(bandId, userId);
      setLink(window.location.origin + resetPath);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to issue reset");
    } finally {
      setBusy(false);
    }
  }

  if (link) {
    return <ResetLinkPanel link={link} onDone={() => setLink(null)} />;
  }

  return (
    <span className="member-reset">
      <button
        type="button"
        className="ghost-btn"
        data-testid="reset-password"
        disabled={busy}
        onClick={() => void onIssue()}
        title="Issue a one-time password-reset link to hand over in person"
      >
        Reset password…
      </button>
      <ErrorBanner message={error} />
    </span>
  );
}

// ResetLinkPanel shows the one-time reset link as a QR (scan it on the member's
// phone — the in-person handoff the design intends, same as invite links) plus
// the raw URL to copy. Purely client-rendered (offline-safe).
function ResetLinkPanel({ link, onDone }: { link: string; onDone: () => void }) {
  const qrRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let cancelled = false;
    QRCode.toString(link, { type: "svg", margin: 1, width: 128 })
      .then((svg) => {
        if (!cancelled && qrRef.current) qrRef.current.innerHTML = svg;
      })
      .catch(() => {
        if (!cancelled && qrRef.current) qrRef.current.textContent = link;
      });
    return () => {
      cancelled = true;
    };
  }, [link]);

  return (
    <span className="member-reset member-reset-open">
      <div className="qr" data-testid="reset-qr" ref={qrRef} />
      <input
        className="reset-link"
        data-testid="reset-link"
        readOnly
        value={link}
        onFocus={(e) => e.target.select()}
      />
      <button type="button" className="ghost-btn" onClick={onDone}>
        Done
      </button>
    </span>
  );
}

const SONGS_PAGE = 12;

function Songs({ bandId }: { bandId: string }) {
  const [songs, setSongs] = useState<Song[]>([]);
  const [title, setTitle] = useState("");
  const [artist, setArtist] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [query, setQuery] = useState("");
  const [limit, setLimit] = useState(SONGS_PAGE);

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

  async function onCreate(e: FormEvent): Promise<boolean> {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await api.createSong(bandId, title, artist || undefined);
      setTitle("");
      setArtist("");
      await load();
      return true;
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to create song");
      return false;
    } finally {
      setBusy(false);
    }
  }

  const q = foldText(query.trim());
  const filtered = q ? songs.filter((s) => foldText(`${s.title} ${s.artist ?? ""}`).includes(q)) : songs;
  const shown = filtered.slice(0, limit);

  return (
    <section className="panel">
      <div className="panel-head">
        <h2>Songs</h2>
        <span className="count">{songs.length}</span>
      </div>
      <div className="panel-body">
        <div className="panel-toolbar">
          <NewItem label="Add song" testId="new-song-btn">
            {(close) => (
              <form
                onSubmit={(e) => void onCreate(e).then((ok) => ok && close())}
                className="inline-form"
              >
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
                <button type="submit" className="primary" data-testid="create-song" disabled={busy}>
                  Add song
                </button>
                <button type="button" className="ghost-btn" onClick={close}>
                  Cancel
                </button>
              </form>
            )}
          </NewItem>
        </div>

        <ErrorBanner message={error} />

        {songs.length === 0 ? (
          <p className="muted" data-testid="songs-empty">
            No songs yet — add your first one.
          </p>
        ) : (
          <>
            {songs.length > SONGS_PAGE && (
              <input
                type="search"
                className="song-filter"
                placeholder="Filter by title or artist…"
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value);
                  setLimit(SONGS_PAGE);
                }}
                data-testid="songs-filter"
                aria-label="Filter songs"
              />
            )}
            {filtered.length === 0 ? (
              <p className="muted" data-testid="songs-no-match">
                No songs match “{query}”.
              </p>
            ) : (
              <>
                <ul className="list song-list" data-testid="songs-list">
                  {shown.map((s) => (
                    <li key={s.id}>
                      <Link to={`/bands/${bandId}/songs/${s.id}`} data-testid="song-link">
                        <span className="song-link-title">{s.title}</span>
                        {s.artist ? <span className="muted"> — {s.artist}</span> : null}
                      </Link>
                    </li>
                  ))}
                </ul>
                {filtered.length > limit && (
                  <button
                    type="button"
                    className="ghost-btn"
                    data-testid="songs-view-more"
                    onClick={() => setLimit((l) => l + SONGS_PAGE)}
                  >
                    View more ({filtered.length - limit} more)
                  </button>
                )}
              </>
            )}
          </>
        )}
      </div>
    </section>
  );
}
