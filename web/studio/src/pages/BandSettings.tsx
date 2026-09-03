/**
 * Band settings — rename, member roles & removal, leave, pending invites, and
 * delete. Most controls are admin-only; "Leave band" is available to everyone.
 */
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import {
  ApiError,
  api,
  type Band,
  type Invite,
  type MemberView,
  type Role,
} from "../api";
import { useAuth } from "../auth";
import { useDialogs } from "../components/Dialog";
import { ErrorBanner } from "../components/ErrorBanner";
import { Avatar } from "../components/Avatar";
import { InviteLinks } from "../components/InviteLinks";
import { useBand } from "./BandLayout";

export function BandSettings() {
  // T130: band + role from the shared BandLayout; setBand lets Rename update the shared copy so the
  // masthead name updates without a full refetch. No own crumb or tab strip here.
  const { band, myRole, setBand } = useBand();
  const bandId = band.id;

  return (
    <>
      <header className="phead">
        <div>
          <div className="eyebrow">Band settings</div>
          <h1 className="title" data-testid="settings-title">
            {band.name}
          </h1>
          <div className="sub">
            Your role: <strong data-testid="settings-my-role">{myRole}</strong>
          </div>
        </div>
      </header>

      {myRole === "admin" && <Rename bandId={bandId} band={band} onRenamed={setBand} />}
      <MembersAdmin bandId={bandId} myRole={myRole} />
      {myRole === "admin" && <PendingInvites bandId={bandId} />}
      {myRole === "admin" && <InviteLinks bandId={bandId} />}
      {myRole === "admin" && <ExportBand bandId={bandId} bandName={band.name} />}
      {myRole === "admin" && <DeleteBand bandId={bandId} />}
    </>
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
    <section className="panel">
      <div className="panel-head">
        <h2>Band name</h2>
      </div>
      <div className="panel-body">
        <form onSubmit={onSave} className="inline-form" data-testid="rename-form">
          <input
            data-testid="band-name-input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
          <button type="submit" className="primary" data-testid="rename-save" disabled={busy}>
            Save
          </button>
          {notice && (
            <span className="saved" data-testid="rename-notice">
              ✓ {notice}
            </span>
          )}
        </form>
        <ErrorBanner message={error} />
      </div>
    </section>
  );
}

function MembersAdmin({ bandId, myRole }: { bandId: string; myRole: Role | null }) {
  const { user } = useAuth();
  const { confirm } = useDialogs(); // T91
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
    if (!(await confirm({ title: "Remove this member?", danger: true, confirmLabel: "Remove" }))) return;
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
    if (!(await confirm({ title: "Leave this band?", danger: true, confirmLabel: "Leave" }))) return;
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
    <section className="panel">
      <div className="panel-head">
        <h2>Members</h2>
        <span className="count">{members.length}</span>
      </div>
      <div className="panel-body">
        <ul className="list" data-testid="settings-members-list">
          {members.map((m) => (
            <li key={m.user.id} data-testid="settings-member-row" className="member-row">
              <span className="member-identity">
                <Avatar user={m.user} size={30} />
                <span className="member-name">{m.user.displayName}</span>
                <span className="muted member-handle">@{m.user.username}</span>
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
                  <span className="chip">{m.role}</span>
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

        <div className="inline-form" style={{ marginTop: ".9rem" }}>
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
      </div>
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
    <section className="panel">
      <div className="panel-head">
        <h2>Pending invites</h2>
        {invites.length > 0 && <span className="count">{invites.length}</span>}
      </div>
      <div className="panel-body">
        {invites.length === 0 ? (
          <p className="muted" data-testid="band-invites-empty" style={{ margin: 0 }}>
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
      </div>
    </section>
  );
}

function ExportBand({ bandId, bandName }: { bandId: string; bandName: string }) {
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onExport() {
    setError(null);
    setBusy(true);
    try {
      const { blob, filename } = await api.exportBand(bandId);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to export band");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel">
      <div className="panel-head">
        <h2>Export band</h2>
      </div>
      <div className="panel-body">
        <p className="muted" style={{ marginTop: 0 }}>
          Download <strong>{bandName}</strong> as a portable <code>.tband</code> archive — its
          members, songs, files, charts, annotations, and setlists. Baked concerts are not
          included; re-bake them after importing.
        </p>
        <div className="inline-form">
          <button
            type="button"
            className="primary"
            data-testid="export-band"
            disabled={busy}
            onClick={onExport}
          >
            {busy ? "Preparing…" : "Export band (.zip)"}
          </button>
        </div>
        <ErrorBanner message={error} />
      </div>
    </section>
  );
}

function DeleteBand({ bandId }: { bandId: string }) {
  const { confirm } = useDialogs(); // T91
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onDelete() {
    if (
      !(await confirm({
        title: "Delete this band?",
        body: "This cannot be undone.",
        danger: true,
        confirmLabel: "Delete band",
      }))
    )
      return;
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
    <section className="panel">
      <div className="panel-head">
        <h2>Danger zone</h2>
      </div>
      <div className="panel-body">
        <div className="inline-form">
          <button type="button" data-testid="delete-band" disabled={busy} onClick={onDelete}>
            Delete band
          </button>
        </div>
        <ErrorBanner message={error} />
      </div>
    </section>
  );
}
