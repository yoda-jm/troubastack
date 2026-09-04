import { useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { ApiError } from "../api";
import { useAuth } from "../auth";
import { AboutLink } from "../components/AboutLink";
import { AuthWordmark } from "../components/AuthWordmark";
import { ErrorBanner } from "../components/ErrorBanner";

function safeNext(raw: string | null): string {
  if (raw && raw.startsWith("/") && !raw.startsWith("//")) return raw;
  return "/bands";
}

export function Register() {
  const { register } = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const nextRaw = params.get("next");
  const next = safeNext(nextRaw);
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await register({
        username,
        displayName: displayName || username,
        password,
        email: email || undefined,
      });
      navigate(next, { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Registration failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="auth-page">
      <AuthWordmark />
      <h1>Register</h1>
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
          Display name
          <input
            data-testid="displayName"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
          />
        </label>
        <label>
          Email (optional)
          <input
            data-testid="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
          />
        </label>
        <label>
          Password
          <input
            data-testid="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="new-password"
            required
          />
        </label>
        <button type="submit" data-testid="submit" disabled={busy}>
          {busy ? "Creating…" : "Create account"}
        </button>
      </form>
      <p>
        Already have an account?{" "}
        <Link to={nextRaw ? `/login?next=${encodeURIComponent(nextRaw)}` : "/login"}>Log in</Link>
      </p>
      <AboutLink />
    </div>
  );
}
