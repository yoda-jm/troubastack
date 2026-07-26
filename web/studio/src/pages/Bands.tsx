import { useEffect, useRef, useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  ApiError,
  api,
  type Band,
  type ImportDisposition,
  type ImportPreview,
} from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { NewItem } from "../components/NewItem";

export function Bands() {
  const navigate = useNavigate();
  const [bands, setBands] = useState<Band[]>([]);
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [importing, setImporting] = useState(false);
  // A previewed import awaiting the admin's per-missing-member choices (T63).
  const [pending, setPending] = useState<{ file: File; preview: ImportPreview } | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  async function load() {
    try {
      setBands(await api.listBands());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load bands");
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function onCreate(e: FormEvent): Promise<boolean> {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await api.createBand(name);
      setName("");
      await load();
      return true;
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to create band");
      return false;
    } finally {
      setBusy(false);
    }
  }

  // Pick a file → preview it (validates + classifies members) → open the dialog.
  async function onPick(e: FormEvent<HTMLInputElement>) {
    const input = e.currentTarget;
    const file = input.files?.[0];
    input.value = ""; // allow re-picking the same file
    if (!file) return;
    setError(null);
    setImporting(true);
    try {
      const preview = await api.previewImport(file);
      setPending({ file, preview });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to read band file");
    } finally {
      setImporting(false);
    }
  }

  // Confirm the import with the chosen dispositions.
  async function onConfirm(dispositions: Record<string, ImportDisposition>) {
    if (!pending) return;
    setError(null);
    setImporting(true);
    try {
      const rep = await api.importBand(pending.file, dispositions);
      setPending(null);
      await load();
      navigate(`/bands/${rep.band.id}`, { state: { importReport: rep } });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to import band");
    } finally {
      setImporting(false);
    }
  }

  return (
    <div className="page">
      <div className="page-head">
        <h1>My bands</h1>
        <input
          ref={fileInput}
          type="file"
          accept=".zip,.tband,application/zip"
          hidden
          data-testid="import-band-input"
          onChange={onPick}
        />
        <button
          type="button"
          className="ghost-btn"
          data-testid="import-band-btn"
          disabled={importing}
          onClick={() => fileInput.current?.click()}
        >
          {importing && !pending ? "Reading…" : "Import band…"}
        </button>
        <NewItem label="New band" testId="new-band-btn">
          {(close) => (
            <form
              onSubmit={(e) => void onCreate(e).then((ok) => ok && close())}
              className="inline-form"
            >
              <input
                data-testid="band-name"
                placeholder="New band name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
              <button type="submit" data-testid="create-band" disabled={busy}>
                Create band
              </button>
              <button type="button" className="ghost-btn" onClick={close}>
                Cancel
              </button>
            </form>
          )}
        </NewItem>
      </div>

      <ErrorBanner message={error} />

      {pending && (
        <ImportDialog
          preview={pending.preview}
          busy={importing}
          onCancel={() => setPending(null)}
          onConfirm={onConfirm}
        />
      )}

      {bands.length === 0 ? (
        <p data-testid="bands-empty" className="muted">
          You are not in any bands yet.
        </p>
      ) : (
        <ul className="list" data-testid="bands-list">
          {bands.map((b) => (
            <li key={b.id}>
              <Link to={`/bands/${b.id}`} data-testid="band-link">
                {b.name}
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/**
 * ImportDialog (T63): after previewing a .tband, the admin sees who's already here (will
 * be attached) and who's missing, and chooses per missing member — Create account
 * (default), Invite (they join when they next sign in), or Skip. Invite/Skip drop that
 * member's personal annotations + cues.
 */
function ImportDialog({
  preview,
  busy,
  onCancel,
  onConfirm,
}: {
  preview: ImportPreview;
  busy: boolean;
  onCancel: () => void;
  onConfirm: (dispositions: Record<string, ImportDisposition>) => void;
}) {
  // The importer is attached automatically; everyone else is a choice. An existing account
  // is consent-required (invite/skip only); a new username defaults to create.
  const choosable = preview.members.filter((m) => !m.isCaller);
  const [choices, setChoices] = useState<Record<string, ImportDisposition>>(() =>
    Object.fromEntries(
      choosable.map((m) => [m.username, (m.existing ? "invite" : "create") as ImportDisposition]),
    ),
  );

  return (
    <section className="panel" data-testid="import-dialog">
      <div className="panel-head">
        <h2>Import “{preview.bandName}”</h2>
      </div>
      <div className="panel-body">
        <p style={{ marginTop: 0 }}>
          {preview.songs} song{preview.songs === 1 ? "" : "s"}, {preview.files} file
          {preview.files === 1 ? "" : "s"}, {preview.setlists} setlist
          {preview.setlists === 1 ? "" : "s"}.
        </p>

        {choosable.length === 0 ? (
          <p className="muted">No other members to add — you’ll be the band’s admin.</p>
        ) : (
          <>
            <p style={{ marginBottom: ".4rem" }}>Choose what to do for each member:</p>
            <ul className="list" data-testid="import-missing-list">
              {choosable.map((m) => (
                <li key={m.username} data-testid="import-missing-row" className="member-row">
                  <span className="member-identity">
                    <span className="member-name">{m.displayName || m.username}</span>
                    <span className="muted member-handle">@{m.username}</span>
                    {m.existing && <span className="chip">has an account here</span>}
                  </span>
                  <select
                    data-testid={`disposition-${m.username}`}
                    value={choices[m.username]}
                    disabled={busy}
                    onChange={(e) =>
                      setChoices((c) => ({
                        ...c,
                        [m.username]: e.target.value as ImportDisposition,
                      }))
                    }
                  >
                    {/* An existing account can't be re-created — only invited or skipped. */}
                    {!m.existing && <option value="create">Create account</option>}
                    <option value="invite">Invite</option>
                    <option value="skip">Skip</option>
                  </select>
                </li>
              ))}
            </ul>
            <p className="muted" style={{ fontSize: ".82rem" }}>
              Members with an account here are never added without consent — Invite sends them a
              request they accept on next sign-in. Invite/Skip drop that member’s personal
              annotations, cues, and file choices; shared and conductor markings are always kept.
              Created accounts need a password-reset link.
            </p>
          </>
        )}

        <div className="inline-form" style={{ marginTop: ".6rem" }}>
          <button
            type="button"
            className="primary"
            data-testid="import-confirm"
            disabled={busy}
            onClick={() => onConfirm(choices)}
          >
            {busy ? "Importing…" : "Import band"}
          </button>
          <button type="button" className="ghost-btn" data-testid="import-cancel" onClick={onCancel}>
            Cancel
          </button>
        </div>
      </div>
    </section>
  );
}
