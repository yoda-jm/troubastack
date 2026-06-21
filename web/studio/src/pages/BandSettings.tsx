/**
 * Band settings — rename, member roles & removal, leave, pending invites, and
 * delete. Most controls are admin-only; "Leave band" is available to everyone.
 */
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  ApiError,
  api,
  type Band,
  type Invite,
  type MemberView,
  type Role,
} from "../api";
import { useAuth } from "../auth";
import { ErrorBanner } from "../components/ErrorBanner";
import { Avatar } from "../components/Avatar";
import { InviteLinks } from "../components/InviteLinks";

export function BandSettings() {
  const { bandId } = useParams<{ bandId: string }>();
  const [band, setBand] = useState<Band | null>(null);
  const [myRole, setMyRole] = useState<Role | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
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
    void load();
  }, [load]);

  if (!bandId) return <div className="page">Loading…</div>;
  if (error && !band) {
    return (
      <div className="page">
        <Link to="/bands">&larr; Bands</Link>
        <ErrorBanner message={error} />
      </div>
    );
  }
  if (!band) return <div className="page">Loading…</div>;

  return (
    <div className="page">
      <Link to={`/bands/${bandId}`}>&larr; Back to band</Link>
      <h1 data-testid="settings-title">{band.name} — Settings</h1>
      <p className="muted">
        Your role: <strong data-testid="settings-my-role">{myRole}</strong>
      </p>

      {myRole === "admin" && <Rename bandId={bandId} band={band} onRenamed={setBand} />}
      <MembersAdmin bandId={bandId} myRole={myRole} />
      {myRole === "admin" && <PendingInvites bandId={bandId} />}
      {myRole === "admin" && <InviteLinks bandId={bandId} />}
      {myRole === "admin" && <DeleteBand bandId={bandId} />}
    </div>
  );
}

function Rename({
  bandId,
  band,
  onRenamed,
}: {
  bandId: string;
  band: Band;
  onRenamed: (b: Band) => void;
}) {
  const [name, setName] = useState(band.name);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSave(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      const updated = await api.updateBand(bandId, name);
      onRenamed(updated);
      setNotice("Saved.");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to rename");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="card">
      <h2>Band name</h2>
      <form onSubmit={onSave} className="inline-form" data-testid="rename-form">
        <input
          data-testid="band-name-input"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
        <button type="submit" data-testid="rename-save" disabled={busy}>
          Save
        </button>
        {notice && (
          <span className="notice" data-testid="rename-notice">
            {notice}
          </span>
        )}
      </form>
      <ErrorBanner message={error} />
    </section>
  );
}

function MembersAdmin({ bandId, myRole }: { bandId: string; myRole: Role | null }) {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [members, setMembers] = useState<MemberView[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

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

  async function changeRole(userId: string, role: Role) {
    setError(null);
    setBusyId(userId);
    try {
      await api.updateMemberRole(bandId, userId, role);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to change role");
    } finally {
      setBusyId(null);
    }
  }

  async function remove(userId: string) {
    if (!window.confirm("Remove this member?")) return;
    setError(null);
    setBusyId(userId);
    try {
      await api.removeMember(bandId, userId);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to remove member");
    } finally {
      setBusyId(null);
    }
  }

  async function leave() {
    if (!window.confirm("Leave this band?")) return;
    setError(null);
    setBusyId("self");
    try {
      await api.leaveBand(bandId);
      navigate("/bands");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to leave band");
      setBusyId(null);
    }
  }

  return (
    <section className="card">
      <h2>Members</h2>
      <ul className="list" data-testid="settings-members-list">
        {members.map((m) => (
          <li key={m.user.id} data-testid="settings-member-row">
            <span>
              <Avatar user={m.user} size={24} /> {m.user.displayName}{" "}
              <span className="muted">@{m.user.username}</span>
            </span>
            <span className="actions">
              {myRole === "admin" ? (
                <select
                  data-testid="member-role-select"
                  value={m.role}
                  disabled={busyId === m.user.id}
                  onChange={(e) => changeRole(m.user.id, e.target.value as Role)}
                >
                  <option value="admin">admin</option>
                  <option value="conductor">conductor</option>
                  <option value="member">member</option>
                </select>
              ) : (
                <span className="pill">{m.role}</span>
              )}
              {myRole === "admin" && m.user.id !== user?.id && (
                <button
                  type="button"
                  data-testid="member-remove"
                  disabled={busyId === m.user.id}
                  onClick={() => remove(m.user.id)}
                >
                  Remove
                </button>
              )}
            </span>
          </li>
        ))}
      </ul>

      <div className="inline-form">
        <button
          type="button"
          data-testid="leave-band"
          disabled={busyId === "self"}
          onClick={leave}
        >
          Leave band
        </button>
      </div>

      <ErrorBanner message={error} />
    </section>
  );
}

function PendingInvites({ bandId }: { bandId: string }) {
  const [invites, setInvites] = useState<Invite[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const all = await api.listBandInvites(bandId);
      setInvites(all.filter((i) => i.status === "pending"));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load invites");
    }
  }, [bandId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function revoke(inviteId: string) {
    setError(null);
    setBusyId(inviteId);
    try {
      await api.revokeInvite(bandId, inviteId);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to revoke invite");
    } finally {
      setBusyId(null);
    }
  }

  return (
    <section className="card">
      <h2>Pending invites</h2>
      {invites.length === 0 ? (
        <p className="muted" data-testid="band-invites-empty">
          No pending invites.
        </p>
      ) : (
        <ul className="list" data-testid="band-invites-list">
          {invites.map((inv) => (
            <li key={inv.id} data-testid="band-invite-row">
              <span>
                {inv.identifier} <span className="muted">({inv.kind})</span>
              </span>
              <span className="actions">
                <button
                  type="button"
                  data-testid="invite-revoke"
                  disabled={busyId === inv.id}
                  onClick={() => revoke(inv.id)}
                >
                  Revoke
                </button>
              </span>
            </li>
          ))}
        </ul>
      )}
      <ErrorBanner message={error} />
    </section>
  );
}

function DeleteBand({ bandId }: { bandId: string }) {
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onDelete() {
    if (!window.confirm("Delete this band? This cannot be undone.")) return;
    setError(null);
    setBusy(true);
    try {
      await api.deleteBand(bandId);
      navigate("/bands");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to delete band");
      setBusy(false);
    }
  }

  return (
    <section className="card">
      <h2>Danger zone</h2>
      <div className="inline-form">
        <button type="button" data-testid="delete-band" disabled={busy} onClick={onDelete}>
          Delete band
        </button>
      </div>
      <ErrorBanner message={error} />
    </section>
  );
}
