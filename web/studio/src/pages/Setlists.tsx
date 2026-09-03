/**
 * Concerts (setlists) list for a band (T127): a small "+ New concert" popup (progressive
 * disclosure) so the list is not lost below the fold on a phone; a search + "view more" like the
 * songs list; a per-row "…" menu (Duplicate for everyone, Delete for admins); and one list ordered
 * next-gig-first, with the past folded under a muted "Past" heading. All the parts are reused:
 * NewItem, RowMenu, foldText, and the confirm dialog.
 */
import { Fragment, useCallback, useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { ApiError, api, type Setlist } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { NewItem } from "../components/NewItem";
import { useBand } from "./BandLayout";
import { RowMenu, RowMenuItem } from "../components/RowMenu";
import { useDialogs } from "../components/Dialog";
import { foldText } from "../foldText";
import { partitionSetlists, todayLocal } from "../setlistOrder";
import { BakeDialog } from "./BakeDialog";
import { bakeSetlistDisabled } from "./SetlistDetail";

// T131: rehearsal live mode is on when liveUntil is set and still in the future (P201). Prefer the
// server's own read-time liveness, but the list only carries liveUntil, so compute it the same way.
function isLive(sl: Setlist): boolean {
  return !!sl.liveUntil && new Date(sl.liveUntil).getTime() > Date.now();
}

// A menu action can't be an <a download> (RowMenuItem is a button), so trigger the download with a
// transient anchor. The server sends Content-Disposition, so this saves the file rather than navigating.
function triggerDownload(url: string, filename: string) {
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
}

const SETLISTS_PAGE = 12;

export function Setlists() {
  // T130: band + role from the shared BandLayout — no own band fetch, crumb or tab strip here.
  const { band, myRole } = useBand();
  const bandId = band.id;
  const { confirm } = useDialogs(); // T91 — in-app confirm, not a blockable window.confirm
  const [setlists, setSetlists] = useState<Setlist[]>([]);
  const [name, setName] = useState("");
  const [eventDate, setEventDate] = useState("");
  const [venue, setVenue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [query, setQuery] = useState("");
  const [limit, setLimit] = useState(SETLISTS_PAGE);
  // T131: the concert being re-baked from its row. BakeDialog needs the song ids (for the layer
  // defaults), which the list doesn't carry — so re-bake fetches the detail once, on click.
  const [bakeTarget, setBakeTarget] = useState<{ setlistId: string; name: string; songIds: string[] } | null>(
    null,
  );

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

  // T131: re-bake from the row. Confirm NAMING the concert (mis-clicking the wrong row is the risk the
  // dialog catches), then fetch the detail once for the song ids and open the SAME BakeDialog the detail
  // page uses — so it is the identical kick-and-poll flow (T103), not a second bake path.
  async function onRebake(sl: Setlist) {
    const verb = sl.lastBakedAt ? "Re-bake" : "Bake";
    if (!(await confirm({ title: `${verb} “${sl.name}”?`, confirmLabel: verb }))) return;
    try {
      const { items } = await api.getSetlist(bandId, sl.id);
      setBakeTarget({ setlistId: sl.id, name: sl.name, songIds: items.map((it) => it.songId) });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to start re-bake");
    }
  }

  const q = foldText(query.trim());
  const filtered = q
    ? setlists.filter((s) => foldText(`${s.name} ${s.venue ?? ""} ${s.eventDate ?? ""}`).includes(q))
    : setlists;
  const { current, past } = partitionSetlists(filtered, todayLocal());
  const ordered = [...current, ...past];
  const shown = ordered.slice(0, limit);

  return (
    <>
      <h1 data-testid="setlists-title">Setlists</h1>

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
                    {/* The "…" trigger and the link are DIRECT children of the flex <li> (which is
                        already `display:flex; justify-content:space-between`), so the menu sits inline
                        at the right rather than wrapping to its own line. The trigger is a SIBLING of
                        the link, never a descendant — a menu tap inside the <Link> would navigate into
                        the concert instead of opening the menu. */}
                    <li className={i >= current.length ? "concert-past" : undefined}>
                      <Link
                        to={`/bands/${bandId}/setlists/${sl.id}`}
                        data-testid="setlist-link"
                      >
                        {sl.name}
                        {sl.eventDate ? <span className="muted"> — {sl.eventDate}</span> : null}
                        {isLive(sl) ? (
                          <span
                            className="live-chip"
                            data-testid="setlist-live"
                            title="Rehearsal live mode is on — edits to this concert's songs auto-bake"
                          >
                            Live
                          </span>
                        ) : null}
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
                                  testId="setlist-rebake"
                                  disabled={bakeSetlistDisabled(false, sl.songCount ?? 0)}
                                  title={
                                    (sl.songCount ?? 0) === 0
                                      ? "Add at least one song to this concert before baking."
                                      : undefined
                                  }
                                  onClick={() => {
                                    closeMenu();
                                    void onRebake(sl);
                                  }}
                                >
                                  {sl.lastBakedAt ? "Re-bake" : "Bake"}
                                </RowMenuItem>
                              )}
                              {sl.downloadUrl && (
                                <RowMenuItem
                                  testId="setlist-pdf"
                                  onClick={() => {
                                    closeMenu();
                                    triggerDownload(
                                      sl.downloadUrl!.replace(/\/bundle$/, "/pdf"),
                                      `${sl.name || "concert"}.pdf`,
                                    );
                                  }}
                                >
                                  Download PDF
                                </RowMenuItem>
                              )}
                              {sl.downloadUrl && (
                                <RowMenuItem
                                  testId="setlist-bundle"
                                  onClick={() => {
                                    closeMenu();
                                    triggerDownload(sl.downloadUrl!, `${sl.name || "concert"}.tstage`);
                                  }}
                                >
                                  Download .tstage
                                </RowMenuItem>
                              )}
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

      {/* T131: the SAME bake flow the detail page uses (T103 kick-and-poll), opened from a row's
          Re-bake. On done, reload the list so the row's lastBakedAt/PDF/bundle refresh. */}
      {bakeTarget && (
        <BakeDialog
          bandId={bandId}
          setlistId={bakeTarget.setlistId}
          songIds={bakeTarget.songIds}
          onBake={(layerDefaults, bakeId) => api.kickBake(bandId, bakeTarget.setlistId, bakeId, layerDefaults)}
          onDone={() => {
            setBakeTarget(null);
            void load();
          }}
          onCancel={() => setBakeTarget(null)}
        />
      )}
    </>
  );
}
