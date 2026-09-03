/**
 * Concerts (setlists) list for a band (T127): a small "+ New concert" popup (progressive
 * disclosure) so the list is not lost below the fold on a phone; a search + "view more" like the
 * songs list; a per-row "…" menu (Duplicate for everyone, Delete for admins); and one list ordered
 * next-gig-first, with the past folded under a muted "Past" heading. All the parts are reused:
 * NewItem, RowMenu, foldText, and the confirm dialog.
 */
import { Fragment, useCallback, useEffect, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import { ApiError, api, type Role, type Setlist } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { SectionTabs } from "../components/SectionTabs";
import { NewItem } from "../components/NewItem";
import { RowMenu, RowMenuItem } from "../components/RowMenu";
import { useDialogs } from "../components/Dialog";
import { foldText } from "../foldText";
import { partitionSetlists, todayLocal } from "../setlistOrder";

const SETLISTS_PAGE = 12;

export function Setlists() {
  const { bandId } = useParams<{ bandId: string }>();
  const { confirm } = useDialogs(); // T91 — in-app confirm, not a blockable window.confirm
  const [setlists, setSetlists] = useState<Setlist[]>([]);
  const [name, setName] = useState("");
  const [eventDate, setEventDate] = useState("");
  const [venue, setVenue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [query, setQuery] = useState("");
  const [limit, setLimit] = useState(SETLISTS_PAGE);

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

  // One band fetch drives BOTH the admin-only Settings tab and the admin-only Delete action.
  const [myRole, setMyRole] = useState<Role | null>(null);
  useEffect(() => {
    if (!bandId) return;
    api
      .getBand(bandId)
      .then(({ myRole }) => setMyRole(myRole))
      .catch(() => {});
  }, [bandId]);

  async function onCreate(e: FormEvent): Promise<boolean> {
    e.preventDefault();
    if (!bandId) return false;
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
      return true;
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to create setlist");
      return false;
    } finally {
      setBusy(false);
    }
  }

  async function onDuplicate(id: string) {
    if (!bandId) return;
    try {
      await api.duplicateSetlist(bandId, id);
      await load(); // stay on the list — the useful outcome from an index is seeing the copy appear
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to duplicate setlist");
    }
  }

  async function onDelete(sl: Setlist) {
    if (!bandId) return;
    if (!(await confirm({ title: `Delete “${sl.name}”?`, danger: true, confirmLabel: "Delete" }))) return;
    try {
      await api.deleteSetlist(bandId, sl.id);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to delete setlist");
    }
  }

  if (!bandId) return <div className="page">Loading…</div>;

  const q = foldText(query.trim());
  const filtered = q
    ? setlists.filter((s) => foldText(`${s.name} ${s.venue ?? ""} ${s.eventDate ?? ""}`).includes(q))
    : setlists;
  const { current, past } = partitionSetlists(filtered, todayLocal());
  const ordered = [...current, ...past];
  const shown = ordered.slice(0, limit);

  return (
    <div className="page">
      <Link to={`/bands/${bandId}`}>&larr; Back to band</Link>
      <h1 data-testid="setlists-title">Setlists</h1>
      <SectionTabs bandId={bandId} active="setlists" showSettings={myRole === "admin"} />

      <NewItem label="New concert" testId="new-setlist-btn">
        {(close) => (
          <form
            onSubmit={(e) => {
              void onCreate(e).then((ok) => ok && close());
            }}
            className="inline-form"
            data-testid="setlist-create-form"
          >
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
            <button type="button" className="ghost-btn" onClick={close}>
              Cancel
            </button>
          </form>
        )}
      </NewItem>
      <ErrorBanner message={error} />

      {setlists.length === 0 ? (
        <p className="muted" data-testid="setlists-empty">
          No setlists yet.
        </p>
      ) : (
        <>
          {setlists.length > SETLISTS_PAGE && (
            <input
              type="search"
              className="song-filter"
              placeholder="Filter by name, venue or date…"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                setLimit(SETLISTS_PAGE);
              }}
              data-testid="setlists-filter"
              aria-label="Filter concerts"
            />
          )}
          {filtered.length === 0 ? (
            <p className="muted" data-testid="setlists-no-match">
              No concerts match “{query}”.
            </p>
          ) : (
            <>
              <ul className="list" data-testid="setlists-list">
                {shown.map((sl, i) => (
                  <Fragment key={sl.id}>
                    {i === current.length && past.length > 0 && (
                      <li className="list-section-head muted" data-testid="setlists-past-heading">
                        Past
                      </li>
                    )}
                    <li className={i >= current.length ? "concert-past" : undefined}>
                      {/* The "…" trigger is a SIBLING of the link, never a descendant — a menu tap
                          inside the <Link> would navigate into the concert instead of opening it. */}
                      <div className="row-with-menu">
                        <Link
                          to={`/bands/${bandId}/setlists/${sl.id}`}
                          data-testid="setlist-link"
                        >
                          {sl.name}
                          {sl.eventDate ? <span className="muted"> — {sl.eventDate}</span> : null}
                        </Link>
                        <RowMenu testId="setlist-menu" label="Concert actions">
                          {(closeMenu) => (
                            <>
                              <RowMenuItem
                                testId="setlist-duplicate"
                                onClick={() => {
                                  closeMenu();
                                  void onDuplicate(sl.id);
                                }}
                              >
                                Duplicate
                              </RowMenuItem>
                              {myRole === "admin" && (
                                <RowMenuItem
                                  testId="setlist-delete"
                                  danger
                                  onClick={() => {
                                    closeMenu();
                                    void onDelete(sl);
                                  }}
                                >
                                  Delete
                                </RowMenuItem>
                              )}
                            </>
                          )}
                        </RowMenu>
                      </div>
                    </li>
                  </Fragment>
                ))}
              </ul>
              {filtered.length > limit && (
                <button
                  type="button"
                  className="ghost-btn"
                  data-testid="setlists-view-more"
                  onClick={() => setLimit((l) => l + SETLISTS_PAGE)}
                >
                  View more ({filtered.length - limit} more)
                </button>
              )}
            </>
          )}
        </>
      )}
    </div>
  );
}
