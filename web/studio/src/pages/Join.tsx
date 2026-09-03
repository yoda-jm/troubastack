/**
 * Join page (/join/:token). Previews a tokenized invite link (band name + role)
 * and lets the authenticated user accept it (the click IS consent). On success
 * it redirects to the band detail page; the band then appears in /bands.
 *
 * Logged-out access is handled upstream by the Shell guard, which bounces to
 * /login?next=/join/:token and returns here after auth.
 */
import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ApiError, api, type InviteLinkPreview } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";

const REASON_TEXT: Record<string, string> = {
  expired: "This invite link has expired.",
  revoked: "This invite link has been revoked.",
  exhausted: "This invite link has reached its maximum number of uses.",
};

export function Join() {
  const { token } = useParams<{ token: string }>();
  const navigate = useNavigate();
  const [preview, setPreview] = useState<InviteLinkPreview | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    if (!token) return;
    try {
      setPreview(await api.previewInviteLink(token));
      setError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setError("This invite link was not found.");
      } else {
        setError(err instanceof ApiError ? err.message : "Failed to load invite link");
      }
    }
  }, [token]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onJoin() {
    if (!token) return;
    setError(null);
    setBusy(true);
    try {
      const band = await api.acceptInviteLink(token);
      navigate(`/bands/${band.id}`, { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to join");
      setBusy(false);
      // Refresh the preview so the reason (e.g. now-exhausted) is shown.
      void load();
    }
  }

  if (error && !preview) {
    return (
      <div className="page">
        <Link className="crumb" to="/bands">&larr; Bands</Link>
        <ErrorBanner message={error} />
      </div>
    );
  }
  if (!preview) return <div className="page">Loading…</div>;

  const reasonText = preview.reason ? REASON_TEXT[preview.reason] ?? "This invite link is no longer usable." : null;

  return (
    <div className="page">
      <Link className="crumb" to="/bands">&larr; Bands</Link>
      <section className="card">
        <h1 data-testid="join-band-name">{preview.band.name}</h1>
        <p className="muted">
          You have been invited to join as <strong data-testid="join-role">{preview.role}</strong>.
        </p>
        {preview.valid ? (
          <button type="button" data-testid="join-accept" disabled={busy} onClick={onJoin}>
            {busy ? "Joining…" : `Join ${preview.band.name}`}
          </button>
        ) : (
          <p className="notice" data-testid="join-invalid">
            {reasonText}
          </p>
        )}
        <ErrorBanner message={error} />
      </section>
    </div>
  );
}
