/**
 * T202 — the concert-PDF dialog. "Print for" the logged-in user by default; an admin may pick any band
 * member (a non-admin sees a disabled field — someone else's annotations are theirs). It kicks an async
 * job, shows a progress bar ("Composing page 12 of 87…"), then downloads the PDF by itself, leaving a
 * "Download again" link until the dialog closes. Modeled on BakeDialog's kick+poll (the poll owns the
 * outcome, and every interval is bounded by the dialog's mount).
 */
import { useEffect, useRef, useState } from "react";
import { ApiError, api, type MemberView, type PdfJobProgress } from "../api";
import { useAuth } from "../auth";

export function PdfDialog({
  bandId,
  concertId,
  concertName,
  isAdmin,
  onClose,
}: {
  bandId: string;
  concertId: string;
  concertName: string;
  isAdmin: boolean;
  onClose: () => void;
}) {
  const { user } = useAuth();
  const meName = user?.displayName || user?.username || "Me";
  const fileName = `${concertName || "concert"}.pdf`;

  const [members, setMembers] = useState<MemberView[]>([]);
  const [member, setMember] = useState(""); // "" = the logged-in user (the default)
  const [busy, setBusy] = useState(false);
  const [jobId, setJobId] = useState<string | null>(null);
  const [progress, setProgress] = useState<PdfJobProgress | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [doneUrl, setDoneUrl] = useState<string | null>(null);

  // Admins get the roster for the picker; a member never picks another.
  useEffect(() => {
    if (!isAdmin) return;
    let cancelled = false;
    void api
      .members(bandId)
      .then((m) => {
        if (!cancelled) setMembers(m);
      })
      .catch(() => {
        /* no roster → the picker just shows "Me" */
      });
    return () => {
      cancelled = true;
    };
  }, [bandId, isAdmin]);

  // Revoke the blob URL when it's replaced or the dialog unmounts.
  useEffect(() => {
    return () => {
      if (doneUrl) URL.revokeObjectURL(doneUrl);
    };
  }, [doneUrl]);

  // The poll owns the outcome (the POST only kicks). Bounded by the dialog's mount; overlapping ticks are
  // skipped (inFlight); the terminal handoff is one-shot (settled).
  const inFlight = useRef(false);
  const settled = useRef(false);
  useEffect(() => {
    if (!busy || !jobId) return;
    let stopped = false;
    const tick = async () => {
      if (inFlight.current) return;
      inFlight.current = true;
      try {
        const p = await api.pdfJobProgress(bandId, concertId, jobId);
        if (stopped || !p) return; // null ⇒ transient error: keep the last line
        setProgress(p);
        if (settled.current) return;
        if (p.state === "done") {
          settled.current = true;
          await downloadFinished(jobId);
        } else if (p.state === "failed") {
          settled.current = true;
          setBusy(false);
          setError(p.error ?? "Couldn't make the PDF.");
        }
      } finally {
        inFlight.current = false;
      }
    };
    void tick();
    const timer = window.setInterval(() => void tick(), 1000);
    return () => {
      stopped = true;
      window.clearInterval(timer);
    };
  }, [busy, jobId, bandId, concertId]);

  async function downloadFinished(id: string) {
    try {
      const res = await fetch(api.pdfJobFileUrl(bandId, concertId, id), { credentials: "include" });
      if (!res.ok) throw new Error(`file ${res.status}`);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = fileName;
      document.body.appendChild(a);
      a.click();
      a.remove();
      setDoneUrl(url); // the "Download again" link holds the same blob until the dialog closes
      setBusy(false);
    } catch {
      setBusy(false);
      setError("The PDF was composed but the download didn't start. Try again.");
    }
  }

  async function makePdf() {
    setError(null);
    setProgress(null);
    settled.current = false;
    if (doneUrl) {
      URL.revokeObjectURL(doneUrl);
      setDoneUrl(null);
    }
    setBusy(true);
    try {
      const id = await api.kickPdfJob(bandId, concertId, member ? { member } : {});
      setJobId(id);
    } catch (err) {
      setBusy(false);
      settled.current = true;
      setError(err instanceof ApiError ? err.message : "Couldn't start the PDF.");
    }
  }

  const label =
    progress && progress.state === "running" && progress.total > 0
      ? `Composing page ${progress.done} of ${progress.total}…`
      : "Composing…";

  return (
    <div className="bake-dialog-backdrop" data-testid="pdf-dialog" role="dialog" aria-modal="true" aria-label="Download PDF">
      <div className="bake-dialog card">
        <div className="card-head">
          <h2>Download PDF</h2>
        </div>
        <label className="pdf-dialog-for">
          Print for:{" "}
          {isAdmin ? (
            <select
              data-testid="pdf-member"
              value={member}
              onChange={(e) => setMember(e.target.value)}
              disabled={busy}
            >
              <option value="">Me ({meName})</option>
              {members
                .filter((m) => m.user.id !== user?.id)
                .map((m) => (
                  <option key={m.user.id} value={m.user.id}>
                    {m.user.displayName || m.user.username}
                  </option>
                ))}
            </select>
          ) : (
            <input data-testid="pdf-member" value={`Me (${meName})`} disabled readOnly />
          )}
        </label>
        {!isAdmin && (
          <p className="muted pdf-dialog-note">
            You print your own view — someone else’s annotations are theirs.
          </p>
        )}
        <p className="muted">Layers: as on my tablet.</p>
        {busy && (
          <p className="muted" data-testid="pdf-progress" role="status" aria-live="polite">
            {label}
          </p>
        )}
        {doneUrl && (
          <p className="muted" data-testid="pdf-done">
            Downloaded.{" "}
            <a href={doneUrl} download={fileName} data-testid="pdf-download-again">
              Download again
            </a>
          </p>
        )}
        {error && (
          <p className="notice error" data-testid="pdf-error" role="alert">
            {error}
          </p>
        )}
        <div className="bake-dialog-actions">
          <button type="button" data-testid="pdf-close" onClick={onClose} disabled={busy}>
            Close
          </button>
          <button type="button" className="primary" data-testid="pdf-make" onClick={makePdf} disabled={busy}>
            {busy ? "Composing…" : error ? "Retry" : "Make PDF"}
          </button>
        </div>
      </div>
    </div>
  );
}
