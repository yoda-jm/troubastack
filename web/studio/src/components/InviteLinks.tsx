/**
 * Invite-links admin panel (band settings). Admins mint tokenized join links
 * (role member|conductor, optional expiry hours, optional max uses), see existing
 * links with a client-rendered QR of the join URL, copy the URL, and revoke.
 */
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import QRCode from "qrcode";
import { ApiError, api, type InviteLink } from "../api";
import { ErrorBanner } from "./ErrorBanner";

// T122: a minted invite link should EXPIRE and be SINGLE-USE unless the admin says otherwise. Once the
// same link is a QR anyone in the room can photograph, the natural act (fill nothing, click Create) must
// not mint a member/conductor credential that grants access forever. These pre-fill the form; both
// fields stay fully editable, so a standing link for a big ensemble is still one clear.
export const INVITE_DEFAULT_EXPIRY_HOURS = "24";
export const INVITE_DEFAULT_MAX_USES = "1";

type InviteInput = { role: "member" | "conductor"; expiresInHours?: number; maxUses?: number };

// inviteInputFromForm computes exactly what the form submits from its (string) fields — the seam a unit
// test can assert without rendering. A BLANK field means "no limit": the API's zero-value semantics
// (maxUses 0 = unlimited, no expiry = never) are unchanged — this task only changes the studio DEFAULTS.
export function inviteInputFromForm(role: "member" | "conductor", expiry: string, maxUses: string): InviteInput {
  const input: InviteInput = { role };
  if (expiry.trim()) input.expiresInHours = Number(expiry);
  if (maxUses.trim()) input.maxUses = Number(maxUses);
  return input;
}

export function InviteLinks({ bandId }: { bandId: string }) {
  const [links, setLinks] = useState<InviteLink[]>([]);
  const [role, setRole] = useState<"member" | "conductor">("member");
  const [expiry, setExpiry] = useState(INVITE_DEFAULT_EXPIRY_HOURS);
  const [maxUses, setMaxUses] = useState(INVITE_DEFAULT_MAX_USES);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      setLinks(await api.listInviteLinks(bandId));
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load invite links");
    }
  }, [bandId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await api.createInviteLink(bandId, inviteInputFromForm(role, expiry, maxUses));
      setExpiry(INVITE_DEFAULT_EXPIRY_HOURS);
      setMaxUses(INVITE_DEFAULT_MAX_USES);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to create invite link");
    } finally {
      setBusy(false);
    }
  }

  async function revoke(id: string) {
    setError(null);
    try {
      await api.revokeInviteLink(bandId, id);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to revoke invite link");
    }
  }

  return (
    <section className="panel">
      <div className="panel-head">
        <h2>Invite links</h2>
      </div>
      <div className="panel-body">
      <p className="muted" style={{ marginTop: 0 }}>
        Share a link to let anyone join this band. The link role can be member or conductor (never
        admin). It defaults to a single-use link that expires in 24 hours — clear a field to lift that
        limit (blank = no expiry / unlimited uses).
      </p>

      <form onSubmit={onCreate} className="inline-form" data-testid="invite-link-form">
        <label className="field">
          <span>Role</span>
          <select
            data-testid="invite-link-role"
            value={role}
            onChange={(e) => setRole(e.target.value as "member" | "conductor")}
          >
            <option value="member">member</option>
            <option value="conductor">conductor</option>
          </select>
        </label>
        <label className="field">
          <span>Expiry (hours)</span>
          <input
            data-testid="invite-link-expiry"
            type="number"
            min="0"
            placeholder="no expiry"
            value={expiry}
            onChange={(e) => setExpiry(e.target.value)}
          />
        </label>
        <label className="field">
          <span>Max uses</span>
          <input
            data-testid="invite-link-maxuses"
            type="number"
            min="0"
            placeholder="unlimited"
            value={maxUses}
            onChange={(e) => setMaxUses(e.target.value)}
          />
        </label>
        <button type="submit" data-testid="create-invite-link" disabled={busy}>
          Create link
        </button>
      </form>

      <ErrorBanner message={error} />

      {links.length === 0 ? (
        <p className="muted" data-testid="invite-links-empty">
          No invite links yet.
        </p>
      ) : (
        <ul className="list" data-testid="invite-links-list">
          {links.map((l) => (
            <InviteLinkRow key={l.id} link={l} onRevoke={() => revoke(l.id)} />
          ))}
        </ul>
      )}
      </div>
    </section>
  );
}

function InviteLinkRow({ link, onRevoke }: { link: InviteLink; onRevoke: () => void }) {
  const qrRef = useRef<HTMLDivElement>(null);
  const [copied, setCopied] = useState(false);
  // The QR is a live, scannable credential: rendered in full, anyone who glances at (or screen-shares)
  // this settings page can photograph a working join link. Keep it blurred behind a cover until the admin
  // deliberately reveals it — the same reveal-on-intent guard the room-facing native QR screen relies on.
  const [revealed, setRevealed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    QRCode.toString(link.url, { type: "svg", margin: 1, width: 128 })
      .then((svg) => {
        if (!cancelled && qrRef.current) qrRef.current.innerHTML = svg;
      })
      .catch(() => {
        if (!cancelled && qrRef.current) qrRef.current.textContent = link.url;
      });
    return () => {
      cancelled = true;
    };
  }, [link.url]);

  async function copy() {
    try {
      await navigator.clipboard.writeText(link.url);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      setCopied(false);
    }
  }

  const usesText = link.maxUses > 0 ? `${link.uses}/${link.maxUses}` : `${link.uses}/∞`;
  // T122: a link with NO use cap AND NO expiry is a standing invitation — say so in plain words, near the
  // QR, so it reads as the open door it is rather than being buried in the meta line.
  const isStanding = !link.revoked && link.maxUses <= 0 && !link.expiresAt;
  const validity = link.revoked
    ? "revoked"
    : link.valid
      ? "valid"
      : (link.reason ?? "invalid");

  return (
    <li data-testid="invite-link-row" className="invite-link-row">
      <div className={`invite-link-qr-box${revealed ? " revealed" : ""}`}>
        <div className="qr" data-testid="invite-link-qr" ref={qrRef} />
        <button
          type="button"
          className="qr-toggle"
          data-testid="invite-link-reveal-qr"
          aria-pressed={revealed}
          onClick={() => setRevealed((v) => !v)}
          title={revealed ? "Hide QR" : "Reveal the scannable join QR"}
        >
          {revealed ? "Hide" : "🔒 Show QR"}
        </button>
      </div>
      <div className="invite-link-body">
        {isStanding && (
          <p className="invite-link-standing" data-testid="invite-link-standing" role="note">
            Standing invitation — anyone who photographs this QR can join as {link.role}, with no expiry and
            no limit on uses.
          </p>
        )}
        <div className="invite-link-meta">
          <input data-testid="invite-link-url" readOnly value={link.url} onFocus={(e) => e.target.select()} />
          <div className="muted">
            role <strong>{link.role}</strong> · uses <strong data-testid="invite-link-uses">{usesText}</strong> ·{" "}
            {link.expiresAt ? `expires ${new Date(link.expiresAt).toLocaleString()}` : "no expiry"} ·{" "}
            <span data-testid="invite-link-validity">{validity}</span>
          </div>
          <div className="actions">
            <button type="button" data-testid="invite-link-copy" onClick={copy}>
              {copied ? "Copied" : "Copy URL"}
            </button>
            {!link.revoked && (
              <button type="button" data-testid="invite-link-revoke" onClick={onRevoke}>
                Revoke
              </button>
            )}
          </div>
        </div>
      </div>
    </li>
  );
}
