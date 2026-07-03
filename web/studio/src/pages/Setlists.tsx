/**
 * Setlists list for a band — create a setlist and link into each one's detail.
 */
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import { ApiError, api, type Setlist } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { SectionTabs } from "../components/SectionTabs";

export function Setlists() {
  const { bandId } = useParams<{ bandId: string }>();
  const [setlists, setSetlists] = useState<Setlist[]>([]);
  const [name, setName] = useState("");
  const [eventDate, setEventDate] = useState("");
  const [venue, setVenue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    if (!bandId) return;
    try {
      setSetlists(await api.listSetlists(bandId));
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load setlists");
    }
  }, [bandId]);

  useEffect(() => {
    void load();
  }, [load]);

  // Role drives whether the admin-only Settings tab shows in the section strip.
  const [showSettings, setShowSettings] = useState(false);
  useEffect(() => {
    if (!bandId) return;
    api
      .getBand(bandId)
      .then(({ myRole }) => setShowSettings(myRole === "admin"))
      .catch(() => {});
  }, [bandId]);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    if (!bandId) return;
    setError(null);
    setBusy(true);
    try {
      await api.createSetlist(bandId, {
        name,
        eventDate: eventDate || undefined,
        venue: venue || undefined,
      });
      setName("");
      setEventDate("");
      setVenue("");
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to create setlist");
    } finally {
      setBusy(false);
    }
  }

  if (!bandId) return <div className="page">Loading…</div>;

  return (
    <div className="page">
      <Link to={`/bands/${bandId}`}>&larr; Back to band</Link>
      <h1 data-testid="setlists-title">Setlists</h1>
      <SectionTabs bandId={bandId} active="setlists" showSettings={showSettings} />

      <section className="card">
        <h2>New setlist</h2>
        <form onSubmit={onCreate} className="inline-form" data-testid="setlist-create-form">
          <input
            data-testid="setlist-name"
            placeholder="Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
          <input
            data-testid="setlist-eventDate"
            type="date"
            value={eventDate}
            onChange={(e) => setEventDate(e.target.value)}
          />
          <input
            data-testid="setlist-venue"
            placeholder="Venue (optional)"
            value={venue}
            onChange={(e) => setVenue(e.target.value)}
          />
          <button type="submit" data-testid="create-setlist" disabled={busy}>
            Create
          </button>
        </form>
        <ErrorBanner message={error} />
      </section>

      {setlists.length === 0 ? (
        <p className="muted" data-testid="setlists-empty">
          No setlists yet.
        </p>
      ) : (
        <ul className="list" data-testid="setlists-list">
          {setlists.map((sl) => (
            <li key={sl.id}>
              <Link to={`/bands/${bandId}/setlists/${sl.id}`} data-testid="setlist-link">
                {sl.name}
                {sl.eventDate ? <span className="muted"> — {sl.eventDate}</span> : null}
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
