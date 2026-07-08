import { useCallback, useEffect, useState } from "react";
import { ApiError, api, type Invite } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";

export function Invites() {
  const [invites, setInvites] = useState<Invite[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setInvites(await api.listInvites());
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load invites");
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function act(inviteId: string, action: "accept" | "decline") {
    setBusyId(inviteId);
    setError(null);
    try {
      if (action === "accept") {
        await api.acceptInvite(inviteId);
      } else {
        await api.declineInvite(inviteId);
      }
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : `Failed to ${action} invite`);
    } finally {
      setBusyId(null);
    }
  }

  return (
    <div className="page">
      <header className="phead">
        <div>
          <div className="eyebrow">Invitations</div>
          <h1 className="title">Pending invites</h1>
          <div className="sub">Bands that have invited you — accept to join.</div>
        </div>
      </header>
      <div className="staff sig" aria-hidden="true" />
      <ErrorBanner message={error} />

      {invites.length === 0 ? (
        <section className="panel">
          <div className="panel-body">
            <p className="muted" data-testid="invites-empty" style={{ margin: 0 }}>
              No pending invites.
            </p>
          </div>
        </section>
      ) : (
        <section className="panel">
          <div className="panel-head">
            <h2>Invitations</h2>
            <span className="count">{invites.length}</span>
          </div>
          <div className="rows" data-testid="invites-list">
            {invites.map((inv) => (
              <div className="row" key={inv.id} data-testid="invite-row">
                <div className="song">
                  <div className="name">Band invitation</div>
                  <div className="by">
                    {inv.kind}: {inv.identifier}
                  </div>
                </div>
                <div className="rowacts">
                  <button
                    type="button"
                    className="primary btn-sm"
                    data-testid="invite-accept"
                    disabled={busyId === inv.id}
                    onClick={() => act(inv.id, "accept")}
                  >
                    Accept
                  </button>
                  <button
                    type="button"
                    className="btn-sm"
                    data-testid="invite-decline"
                    disabled={busyId === inv.id}
                    onClick={() => act(inv.id, "decline")}
                  >
                    Decline
                  </button>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
