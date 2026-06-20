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
      <h1>Pending invites</h1>
      <ErrorBanner message={error} />

      {invites.length === 0 ? (
        <p className="muted" data-testid="invites-empty">
          No pending invites.
        </p>
      ) : (
        <ul className="list" data-testid="invites-list">
          {invites.map((inv) => (
            <li key={inv.id} data-testid="invite-row">
              <span>
                Invite to band <code>{inv.bandId}</code> ({inv.kind}: {inv.identifier})
              </span>
              <span className="actions">
                <button
                  type="button"
                  data-testid="invite-accept"
                  disabled={busyId === inv.id}
                  onClick={() => act(inv.id, "accept")}
                >
                  Accept
                </button>
                <button
                  type="button"
                  data-testid="invite-decline"
                  disabled={busyId === inv.id}
                  onClick={() => act(inv.id, "decline")}
                >
                  Decline
                </button>
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
