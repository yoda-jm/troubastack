import { useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { ApiError, api, type Band } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { NewItem } from "../components/NewItem";

export function Bands() {
  const [bands, setBands] = useState<Band[]>([]);
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

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

  return (
    <div className="page">
      <div className="page-head">
        <h1>My bands</h1>
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
