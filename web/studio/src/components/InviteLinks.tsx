/**
 * Invite-links admin panel (band settings). Admins mint tokenized join links
 * (role member|conductor, optional expiry hours, optional max uses), see existing
 * links with a client-rendered QR of the join URL, copy the URL, and revoke.
 */
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import QRCode from "qrcode";
import { ApiError, api, type InviteLink } from "../api";
import { ErrorBanner } from "./ErrorBanner";

export function InviteLinks({ bandId }: { bandId: string }) {
  const [links, setLinks] = useState<InviteLink[]>([]);
  const [role, setRole] = useState<"member" | "conductor">("member");
  const [expiry, setExpiry] = useState("");
  const [maxUses, setMaxUses] = useState("");
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
      const input: { role: "member" | "conductor"; expiresInHours?: number; maxUses?: number } = {
        role,
      };
      if (expiry.trim()) input.expiresInHours = Number(expiry);
      if (maxUses.trim()) input.maxUses = Number(maxUses);
      await api.createInviteLink(bandId, input);
      setExpiry("");
      setMaxUses("");
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
        admin).
      </p>

      <form onSubmit={onCreate} className="inline-form" data-testid="invite-link-form">
        <select
          data-testid="invite-link-role"
          value={role}
          onChange={(e) => setRole(e.target.value as "member" | "conductor")}
        >
          <option value="member">member</option>
          <option value="conductor">conductor</option>
        </select>
        <input
          data-testid="invite-link-expiry"
          type="number"
          min="0"
          placeholder="Expiry (hours)"
          value={expiry}
          onChange={(e) => setExpiry(e.target.value)}
        />
        <input
          data-testid="invite-link-maxuses"
          type="number"
          min="0"
          placeholder="Max uses"
          value={maxUses}
          onChange={(e) => setMaxUses(e.target.value)}
        />
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
  const validity = link.revoked
    ? "revoked"
    : link.valid
      ? "valid"
      : (link.reason ?? "invalid");

  return (
    <li data-testid="invite-link-row" className="invite-link-row">
      <div className="qr" data-testid="invite-link-qr" ref={qrRef} />
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
    </li>
  );
}
