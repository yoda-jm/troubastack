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
  // A join link is a credential in TWO forms on this row: the QR (scan) and the URL (read/transcribe).
  // One toggle conceals BOTH until the admin reveals them — concealing only the QR while the URL sat
  // legible beside it just moved the leak from "scan" to "type" (Fable). This raises the cost of a
  // casual capture (a glance, a screen-share); it is not a guarantee against a deliberate photograph.
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
          title={revealed ? "Hide the join link" : "Reveal the join link (QR + URL)"}
        >
          {revealed ? "Hide" : "🔒 Reveal"}
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
          <div className={`invite-link-url${revealed ? " revealed" : ""}`}>
            {/* Blurred + unselectable until revealed (same toggle as the QR); copy still works while
                hidden, so an admin can hand off the link without ever putting it on screen. */}
            <input
              data-testid="invite-link-url"
              readOnly
              value={link.url}
              tabIndex={revealed ? 0 : -1}
              onFocus={revealed ? (e) => e.target.select() : undefined}
            />
            {/* Copy is the common inline affordance inside the field, not a separate button (VLL). */}
            <button
              type="button"
              className="invite-link-copy"
              data-testid="invite-link-copy"
              onClick={copy}
              title={copied ? "Copied" : "Copy URL"}
              aria-label={copied ? "Copied" : "Copy URL"}
            >
              {copied ? <CheckIcon /> : <CopyIcon />}
            </button>
          </div>
          <div className="muted">
            role <strong>{link.role}</strong> · uses <strong data-testid="invite-link-uses">{usesText}</strong> ·{" "}
            {link.expiresAt ? `expires ${new Date(link.expiresAt).toLocaleString()}` : "no expiry"} ·{" "}
            <span data-testid="invite-link-validity">{validity}</span>
          </div>
          {!link.revoked && (
            <div className="actions">
              {/* Revoke is destructive and irreversible — wear the error colour so it doesn't read as
                  a neutral action next to Copy (VLL). */}
              <button type="button" className="invite-link-revoke" data-testid="invite-link-revoke" onClick={onRevoke}>
                Revoke
              </button>
            </div>
          )}
        </div>
      </div>
    </li>
  );
}

/** Two-rectangle "copy" glyph, inlined so the URL field's copy affordance needs no icon dependency. */
function CopyIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="9" y="9" width="11" height="11" rx="2" />
      <path d="M5 15V5a2 2 0 0 1 2-2h10" />
    </svg>
  );
}

/** Check glyph shown briefly after a successful copy. */
function CheckIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M20 6 9 17l-5-5" />
    </svg>
  );
}
