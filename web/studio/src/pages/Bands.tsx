import { useEffect, useRef, useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ApiError, api, type Band } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { NewItem } from "../components/NewItem";

export function Bands() {
  const navigate = useNavigate();
  const [bands, setBands] = useState<Band[]>([]);
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [importing, setImporting] = useState(false);
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

  async function onImport(e: FormEvent<HTMLInputElement>) {
    const input = e.currentTarget;
    const file = input.files?.[0];
    input.value = ""; // allow re-picking the same file
    if (!file) return;
    setError(null);
    setImporting(true);
    try {
      const rep = await api.importBand(file);
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
          onChange={onImport}
        />
        <button
          type="button"
          className="ghost-btn"
          data-testid="import-band-btn"
          disabled={importing}
          onClick={() => fileInput.current?.click()}
        >
          {importing ? "Importing…" : "Import band…"}
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
