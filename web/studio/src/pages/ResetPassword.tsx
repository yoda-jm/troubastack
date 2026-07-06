import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ApiError, api, type User } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";

/**
 * ResetPassword is the public page a one-time reset link lands on (T21). The
 * token in the URL is the credential — no session required. It validates the
 * token (naming whose account it is), sets a new password, and sends the user to
 * log in. Consuming the token invalidates every existing session server-side.
 */
export function ResetPassword() {
  const { token } = useParams<{ token: string }>();
  const navigate = useNavigate();
  const [target, setTarget] = useState<User | null>(null);
  const [checking, setChecking] = useState(true);
  const [invalid, setInvalid] = useState<string | null>(null);
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);

  useEffect(() => {
    let live = true;
    if (!token) return;
    void api
      .previewPasswordReset(token)
      .then((u) => live && setTarget(u))
      .catch((err) =>
        live && setInvalid(err instanceof ApiError ? err.message : "This reset link is not valid."),
      )
      .finally(() => live && setChecking(false));
    return () => {
      live = false;
    };
  }, [token]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    setError(null);
    setBusy(true);
    try {
      await api.submitPasswordReset(token, password);
      setDone(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to set password");
    } finally {
      setBusy(false);
    }
  }

  if (checking) {
    return (
      <div className="auth-page">
        <h1>Reset password</h1>
        <p className="muted" data-testid="reset-checking">
          Checking your link…
        </p>
      </div>
    );
  }

  if (invalid) {
    return (
      <div className="auth-page">
        <h1>Reset password</h1>
        <p className="notice" data-testid="reset-invalid">
          {invalid}
        </p>
        <p>
          Ask a band admin for a fresh link, then <Link to="/login">log in</Link>.
        </p>
      </div>
    );
  }

  if (done) {
    return (
      <div className="auth-page">
        <h1>Password set</h1>
        <p className="notice" data-testid="reset-done">
          Your password has been changed and you have been signed out everywhere.
        </p>
        <button type="button" data-testid="reset-go-login" onClick={() => navigate("/login", { replace: true })}>
          Log in
        </button>
      </div>
    );
  }

  return (
    <div className="auth-page">
      <h1>Reset password</h1>
      <p className="muted">
        Setting a new password for <strong data-testid="reset-target">@{target?.username}</strong>.
      </p>
      <form onSubmit={onSubmit} className="card">
        <ErrorBanner message={error} />
        <label>
          New password
          <input
            data-testid="reset-new-password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="new-password"
            required
          />
        </label>
        <button type="submit" data-testid="reset-submit" disabled={busy}>
          {busy ? "Setting…" : "Set new password"}
        </button>
      </form>
    </div>
  );
}
