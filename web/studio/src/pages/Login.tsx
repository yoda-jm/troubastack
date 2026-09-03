import { useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { ApiError } from "../api";
import { useAuth } from "../auth";
import { AuthWordmark } from "../components/AuthWordmark";
import { ErrorBanner } from "../components/ErrorBanner";

// safeNext returns a same-origin in-app path from the ?next= param, or /bands.
// Guards against open-redirects (must be a single leading slash, not //).
function safeNext(raw: string | null): string {
  if (raw && raw.startsWith("/") && !raw.startsWith("//")) return raw;
  return "/bands";
}

export function Login() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const next = safeNext(params.get("next"));
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await login(username, password);
      navigate(next, { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Login failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="auth-page">
      <AuthWordmark />
      <h1>Log in</h1>
      <form onSubmit={onSubmit} className="card">
        <ErrorBanner message={error} />
        <label>
          Username
          <input
            data-testid="username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoComplete="username"
            required
          />
        </label>
        <label>
          Password
          <input
            data-testid="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </label>
        <button type="submit" data-testid="submit" disabled={busy}>
          {busy ? "Logging in…" : "Log in"}
        </button>
      </form>
      <p>
        No account?{" "}
        <Link to={params.get("next") ? `/register?next=${encodeURIComponent(params.get("next")!)}` : "/register"}>
          Register
        </Link>
      </p>
    </div>
  );
}
