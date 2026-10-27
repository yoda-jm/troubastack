import { useCallback, useEffect, useState, type Dispatch, type SetStateAction } from "react";
import { Link, Outlet, useOutletContext, useParams } from "react-router-dom";
import {
  ApiError,
  api,
  type Concert,
  type ConcertChanges,
  type Role,
  type Setlist,
  type SetlistItem,
} from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { ConcertTabs } from "../components/ConcertTabs";

/**
 * T206 — "Running order" and "Sound sheet" are TWO TABS OF ONE CONCERT, so they share one layout route
 * (the T130 BandLayout pattern): this element owns the crumb, the concert header (name, date · venue, the
 * page-level chips) and the tab strip, fetches the concert ONCE, and renders the active tab through
 * <Outlet/>. Switching tabs no longer re-mounts the header or re-fetches the concert, and the header is
 * defined here ONCE so the two routes can never diverge (the old sound-sheet page carried its own crumb +
 * h1). The exports live in the tab that owns them (the running-order PDF + bake controls on the running
 * order; PDF / CSV / patch on the sound sheet) — nothing floats in the shared header but the chips.
 */
export type ConcertContext = {
  bandId: string;
  setlistId: string;
  setlist: Setlist;
  setSetlist: (s: Setlist) => void;
  items: SetlistItem[];
  setItems: Dispatch<SetStateAction<SetlistItem[]>>;
  myRole: Role | null;
  // The band-wide concert for this setlist (the exact-id match), when one has been baked. Carries the
  // T201 edited-since-bake signal for the header chip; the running-order tab's BakeCard reuses it + the
  // full history list (`concerts`) and `reload` instead of fetching /concerts a second time.
  concerts: Concert[];
  bandConcert: Concert | null;
  reload: () => Promise<void>;
};

/** The two tab routes read the shared concert here instead of fetching their own. */
export function useConcert(): ConcertContext {
  return useOutletContext<ConcertContext>();
}

// editedSummary (T201) is the tooltip behind the "Edited since last bake" chip — what a re-bake would
// change. Lives here now that the chip is in the shared header (was in SetlistDetail's BakeCard).
export function editedSummary(ch?: ConcertChanges): string {
  if (!ch) return "A re-bake would change this concert.";
  const parts: string[] = [];
  const songs = (ch.songs ?? []).map((s) => `${s.title} (${s.what})`);
  if (songs.length) parts.push(songs.join(", "));
  if (ch.setlist) parts.push("the running order changed");
  return parts.length ? `Changed since the last bake: ${parts.join("; ")}.` : "A re-bake would change this concert.";
}

export function ConcertLayout() {
  const { bandId, setlistId } = useParams<{ bandId: string; setlistId: string }>();
  const [setlist, setSetlist] = useState<Setlist | null>(null);
  const [items, setItems] = useState<SetlistItem[]>([]);
  const [myRole, setMyRole] = useState<Role | null>(null);
  const [concerts, setConcerts] = useState<Concert[]>([]);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    if (!bandId || !setlistId) return;
    try {
      const [{ setlist, items }, { myRole }, allConcerts] = await Promise.all([
        api.getSetlist(bandId, setlistId),
        api.getBand(bandId),
        api.listConcerts(bandId).catch(() => [] as Concert[]),
      ]);
      // Main order first, then bench; position within each group (matches the server and lets the
      // running-order view slice the two groups cleanly — T23).
      items.sort(
        (a, b) => Number(a.onCall ?? false) - Number(b.onCall ?? false) || a.position - b.position,
      );
      setSetlist(setlist);
      setItems(items);
      setMyRole(myRole);
      // This setlist's concerts: the exact-id band bundle plus any legacy `${setlistId}~${userId}` variants.
      setConcerts(allConcerts.filter((c) => c.concertId === setlistId || c.concertId.startsWith(`${setlistId}~`)));
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load concert");
    }
  }, [bandId, setlistId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  if (!bandId || !setlistId) return <div className="page">Loading…</div>;
  if (error && !setlist) {
    return (
      <div className="page">
        <Link className="crumb" to={`/bands/${bandId}/setlists`}>
          &larr; Setlists
        </Link>
        <ErrorBanner message={error} />
      </div>
    );
  }
  if (!setlist) return <div className="page">Loading…</div>;

  const mainCount = items.filter((it) => !it.onCall).length;
  const benchCount = items.length - mainCount;
  const sub = [setlist.eventDate, setlist.venue].filter(Boolean).join(" · ");
  const bandConcert = concerts.find((c) => c.concertId === setlistId) ?? null;

  // The crumb + header are defined ONCE, here, so the two tabs can't diverge. "Up" from either tab is the
  // setlists list — one label, one destination (the old sound-sheet "← Concert" crumb is gone; the Running
  // order tab IS the concert).
  return (
    <div className="page">
      <Link className="crumb" to={`/bands/${bandId}/setlists`}>
        &larr; Setlists
      </Link>
      <header className="phead">
        <div>
          {/* T206 binding 2 (Fable): the eyebrow keeps its existing word "Setlist" — "Concert" was a
              product word changing as a side effect; it's on the label list for VLL to settle. */}
          <div className="eyebrow">Setlist</div>
          <h1 className="title" data-testid="setlist-detail-title">
            {setlist.name}
          </h1>
          {sub && <div className="sub">{sub}</div>}
          <div className="meta">
            <span className="chip mono">
              {mainCount} song{mainCount === 1 ? "" : "s"}
            </span>
            {benchCount > 0 && <span className="chip brand">{benchCount} on call</span>}
            {setlist.archived && (
              <span className="chip" data-testid="setlist-archived-tag" title="Archived — kept in Studio, not offered on the tablets">
                Archived
              </span>
            )}
            {bandConcert?.editedSinceBake && (
              <span className="chip edited" data-testid="setlist-edited-tag" title={editedSummary(bandConcert.changes)}>
                Edited since last bake
              </span>
            )}
          </div>
        </div>
      </header>
      {/* The staff-line ornament sits between the header and the tabs (not between the tabs and the body),
          so it reads as part of the header rather than the tab panel (Fable nit). */}
      <div className="staff sig" aria-hidden="true" />
      <ConcertTabs bandId={bandId} setlistId={setlistId} />
      <Outlet
        context={{ bandId, setlistId, setlist, setSetlist, items, setItems, myRole, concerts, bandConcert, reload } satisfies ConcertContext}
      />
    </div>
  );
}
