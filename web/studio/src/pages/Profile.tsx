/**
 * Profile / account page (/me). Three sections: edit profile (displayName, email,
 * avatar kind), change password, and a personal invite QR encoding `@username`
 * that a band admin can scan to invite this user. The QR is rendered client-side
 * (offline-safe) via the qrcode library to an inline SVG string.
 */
import { useEffect, useRef, useState, type FormEvent } from "react";
import QRCode from "qrcode";
import { ApiError, api, type AvatarKind } from "../api";
import { useAuth } from "../auth";
import { ErrorBanner } from "../components/ErrorBanner";
import { Avatar } from "../components/Avatar";

const AVATAR_KINDS: Exclude<AvatarKind, "">[] = ["man", "woman", "neutral"];

export function Profile() {
  const { user, refresh } = useAuth();

  if (!user) return <div className="page">Loading…</div>;

  return (
    <div className="page">
      <h1>Your profile</h1>
      <EditProfile
        initialDisplayName={user.displayName}
        initialEmail={user.email ?? ""}
        initialAvatar={user.avatarKind || "neutral"}
        onSaved={refresh}
      />
      <ChangePassword />
      <PersonalQR username={user.username} />
    </div>
  );
}

function EditProfile({
  initialDisplayName,
  initialEmail,
  initialAvatar,
  onSaved,
}: {
  initialDisplayName: string;
  initialEmail: string;
  initialAvatar: AvatarKind;
  onSaved: () => Promise<void>;
}) {
  const [displayName, setDisplayName] = useState(initialDisplayName);
  const [email, setEmail] = useState(initialEmail);
  const [avatarKind, setAvatarKind] = useState<AvatarKind>(initialAvatar || "neutral");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSave(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      await api.updateProfile({ displayName, email, avatarKind });
      await onSaved();
      setNotice("Profile saved.");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save profile");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="card">
      <h2>Edit profile</h2>
      <form onSubmit={onSave}>
        <label>
          Display name
          <input
            data-testid="profile-displayname"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            required
          />
        </label>
        <label>
          Email
          <input
            data-testid="profile-email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
          />
        </label>

        <fieldset className="avatar-picker">
          <legend>Avatar</legend>
          <div className="inline-form">
            {AVATAR_KINDS.map((k) => (
              <label key={k} className={`avatar-option${avatarKind === k ? " selected" : ""}`}>
                <Avatar user={{ displayName, avatarKind: k }} size={40} />
                <span>{k}</span>
                <input
                  type="radio"
                  name="avatarKind"
                  value={k}
                  data-testid={`profile-avatar-${k}`}
                  checked={avatarKind === k}
                  onChange={() => setAvatarKind(k)}
                />
              </label>
            ))}
          </div>
        </fieldset>

        <button type="submit" data-testid="profile-save" disabled={busy}>
          {busy ? "Saving…" : "Save profile"}
        </button>
        {notice && (
          <span className="notice" data-testid="profile-notice">
            {notice}
          </span>
        )}
      </form>
      <ErrorBanner message={error} />
    </section>
  );
}

function ChangePassword() {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSave(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setNotice(null);
    if (next !== confirm) {
      setError("New password and confirmation do not match.");
      return;
    }
    setBusy(true);
    try {
      await api.changePassword(current, next);
      setCurrent("");
      setNext("");
      setConfirm("");
      setNotice("Password changed.");
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setError("Current password is incorrect.");
      } else {
        setError(err instanceof ApiError ? err.message : "Failed to change password");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="card">
      <h2>Change password</h2>
      <form onSubmit={onSave}>
        <label>
          Current password
          <input
            data-testid="pw-current"
            type="password"
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
            autoComplete="current-password"
            required
          />
        </label>
        <label>
          New password
          <input
            data-testid="pw-new"
            type="password"
            value={next}
            onChange={(e) => setNext(e.target.value)}
            autoComplete="new-password"
            required
          />
        </label>
        <label>
          Confirm new password
          <input
            data-testid="pw-confirm"
            type="password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            autoComplete="new-password"
            required
          />
        </label>
        <button type="submit" data-testid="pw-save" disabled={busy}>
          {busy ? "Changing…" : "Change password"}
        </button>
        {notice && (
          <span className="notice" data-testid="pw-notice">
            {notice}
          </span>
        )}
      </form>
      <ErrorBanner message={error} />
    </section>
  );
}

function PersonalQR({ username }: { username: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const handle = `@${username}`;

  useEffect(() => {
    let cancelled = false;
    QRCode.toString(handle, { type: "svg", margin: 1, width: 192 })
      .then((svg) => {
        if (!cancelled && ref.current) ref.current.innerHTML = svg;
      })
      .catch(() => {
        if (!cancelled && ref.current) ref.current.textContent = handle;
      });
    return () => {
      cancelled = true;
    };
  }, [handle]);

  return (
    <section className="card">
      <h2>Your invite QR</h2>
      <p className="muted">Show this to a band admin to invite you</p>
      <div className="qr" data-testid="profile-qr" ref={ref} />
      <p>
        <strong>{handle}</strong>
      </p>
    </section>
  );
}
