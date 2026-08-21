/**
 * My-files editor panel: per-member file selection driving the viewer strip.
 *
 * T82 — a checkbox must never move or resize the row it is on. The instability came from deriving
 * POSITION from INCLUSION (included rows rendered first, excluded after; re-including appended to the
 * end; only included rows carried the ↑/↓ actions). The fix decouples the two axes:
 *   - ONE list of every pool file, in a single display order computed once per load (my included
 *     files in my stored order, then the rest in pool order) and then frozen for the session;
 *   - the checkbox toggles membership IN PLACE — it never reorders and never reshapes the row;
 *   - reordering is explicit (the shared T78 drag grip + ↑/↓), and moves rows on the frozen order;
 *   - rows are geometrically uniform (grip + move controls on EVERY row, disabled at the ends);
 *   - writes are optimistic and last-write-wins: flip locally, PUT, and ignore any superseded
 *     response (a slow earlier PUT must not clobber newer intent — that was a silent lost update).
 *
 * No API change: `setMyFiles` still stores the ordered INCLUDED ids, so an excluded file's position
 * is session-local and falls back to pool order after a reload. Persistent full ordering would be an
 * API change and its own task.
 */
import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, type SongFile } from "../../api";
import { useFlipRows, useSortable } from "../../components/SortableList";

function MyFilesEditorInner({
  bandId,
  songId,
  selected,
  onChanged,
  onError,
}: {
  bandId: string;
  songId: string;
  selected: SongFile[];
  onChanged: () => Promise<SongFile[]>;
  onError: (msg: string | null) => void;
}) {
  const [pool, setPool] = useState<SongFile[]>([]);
  // The frozen per-session display order: EVERY pool file id, once. Reorder mutates this; a toggle
  // never does. Inclusion is a separate axis (`included`), so ticking a box leaves position alone.
  const [displayOrder, setDisplayOrder] = useState<string[]>([]);
  const [included, setIncluded] = useState<Set<string>>(() => new Set(selected.map((f) => f.id)));
  // Serialised writes (T82, Fable): at most ONE PUT in flight. Guarding the response is not enough —
  // two concurrent PUTs can still land at the server out of order, so a slow earlier write wins last
  // and silently discards newer intent. Instead we keep one write in flight and coalesce every
  // further toggle/reorder into a single `pending` latest-state, sent when the current resolves — so
  // the last write to REACH the server is, by construction, the user's latest intent.
  const inFlight = useRef(false);
  const pending = useRef<{ order: string[]; inc: Set<string> } | null>(null);
  // onError via a ref so the seed effect below depends ONLY on (bandId, songId): if onError is not a
  // stable reference, having it in the deps re-runs the seed on every parent re-render (each onChanged),
  // which re-seeds the "frozen" order from the latest selection and makes an excluded row jump to the
  // end — the exact bug this task removes, leaking back in intermittently.
  const onErrorRef = useRef(onError);
  onErrorRef.current = onError;

  // Load the whole pool once, then compute the frozen order: my included (in my order) then the rest.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const list = await api.listFiles(bandId, songId);
        if (cancelled) return;
        list.sort((a, b) => a.displayOrder - b.displayOrder);
        const poolIds = new Set(list.map((f) => f.id));
        const inc = selected.map((f) => f.id).filter((id) => poolIds.has(id));
        const incSet = new Set(inc);
        const rest = list.filter((f) => !incSet.has(f.id)).map((f) => f.id);
        setPool(list);
        setDisplayOrder([...inc, ...rest]);
        setIncluded(incSet);
      } catch (err) {
        if (cancelled) return;
        onErrorRef.current(err instanceof Error ? err.message : "Failed to load files");
      }
    })();
    return () => {
      cancelled = true;
    };
    // Seed ONCE per song: `selected` is the initial seed only, and onError is read via a ref — so a
    // parent re-render (each onChanged) can't re-run this and re-seed the frozen order.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bandId, songId]);

  const poolById = useMemo(() => {
    const m = new Map<string, SongFile>();
    for (const f of pool) m.set(f.id, f);
    return m;
  }, [pool]);

  // drain runs the single in-flight write and, when it resolves, picks up whatever `pending` holds —
  // so intermediate toggles coalesce and only the LATEST desired state is ever the trailing write.
  // We never re-seed local order/inclusion from a success response (that was the lost-update bug); on
  // failure we reconcile the UI to server truth so a rejected write can't leave the UI lying.
  const drain = useCallback(async () => {
    if (inFlight.current) return;
    inFlight.current = true;
    try {
      while (pending.current) {
        const next = pending.current;
        pending.current = null;
        try {
          await api.setMyFiles(bandId, songId, next.order.filter((id) => next.inc.has(id)));
          onError(null);
          await onChanged(); // refresh the viewer strip only
        } catch (err) {
          onError(err instanceof Error ? err.message : "Failed to update selection");
          try {
            const mine = await onChanged();
            setIncluded(new Set(mine.map((f) => f.id)));
          } catch {
            /* leave local state as-is */
          }
        }
      }
    } finally {
      inFlight.current = false;
      if (pending.current) void drain(); // a toggle that arrived in the exit window
    }
  }, [bandId, songId, onChanged, onError]);

  // schedule records the latest desired state and kicks the drain; a write already in flight will
  // pick this up when it resolves (coalescing), so at most one PUT is ever outstanding.
  const schedule = useCallback(
    (order: string[], inc: Set<string>) => {
      pending.current = { order, inc };
      void drain();
    },
    [drain],
  );

  function toggleInclude(id: string) {
    const next = new Set(included);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setIncluded(next); // optimistic + in place — the row does not move
    schedule(displayOrder, next);
  }

  const reorder = useCallback(
    (nextOrder: string[]) => {
      setDisplayOrder(nextOrder);
      schedule(nextOrder, included);
    },
    [included, schedule],
  );

  async function reset() {
    pending.current = null; // cancel any queued selection write; reset is an explicit full override
    onError(null);
    try {
      await api.clearMyFiles(bandId, songId);
      const mine = await onChanged();
      // Reset is explicit → re-seed the frozen order to all-included, pool order.
      setDisplayOrder(pool.map((f) => f.id));
      setIncluded(new Set(mine.map((f) => f.id)));
    } catch (err) {
      onError(err instanceof Error ? err.message : "Failed to reset");
    }
  }

  const flip = useFlipRows(displayOrder);
  const sortable = useSortable(displayOrder, reorder, flip);
  const rows = displayOrder.map((id) => poolById.get(id)).filter((f): f is SongFile => !!f);

  // Render probe (T82b): a toggle must re-render this panel exactly ONCE (its own optimistic
  // setIncluded). Before the memo wrapper, the parent's post-toggle onChanged (viewer-strip refresh)
  // re-rendered it a second time — the reflow the user saw as a flicker. Surfaced for the regression test.
  const renders = useRef(0);
  renders.current += 1;

  return (
    <section className="my-files-panel card" data-testid="my-files-panel" data-renders={renders.current}>
      <div className="my-files-panel-head">
        <h2>My files</h2>
        <button
          type="button"
          className="my-files-reset-btn"
          data-testid="my-files-reset"
          onClick={() => void reset()}
        >
          Reset to all
        </button>
      </div>
      <p className="muted my-files-hint">
        Pick which files appear in your strip and in what order. Ticking a box never moves the row;
        drag the grip or use ↑/↓ to reorder. Everyone shares the same pool (managed under the “Shared
        with the band” tab).
      </p>

      {rows.length === 0 ? (
        <p className="muted">No files in the pool yet.</p>
      ) : (
        <ul className="list my-files-list" data-testid="my-files-list">
          {rows.map((f, i) => {
            const on = included.has(f.id);
            const row = sortable.rowProps(i);
            return (
              <li
                key={f.id}
                ref={row.ref}
                data-testid="my-files-row"
                className={`my-files-row${on ? " included" : " excluded"}${sortable.isDragOver(i) ? " drag-over" : ""}`}
                onDragOver={row.onDragOver}
                onDragLeave={row.onDragLeave}
                onDrop={row.onDrop}
              >
                <span
                  className="grip"
                  data-testid="my-files-grip"
                  title="Drag to reorder"
                  aria-label="Drag to reorder"
                  {...sortable.gripProps(i)}
                >
                  ⠿
                </span>
                <label className="my-files-row-main">
                  <input
                    type="checkbox"
                    data-testid="my-files-include"
                    checked={on}
                    onChange={() => toggleInclude(f.id)}
                  />
                  <span className={`my-files-name${on ? "" : " muted"}`}>{f.filename}</span>
                </label>
                <span className="actions">
                  <button
                    type="button"
                    data-testid="my-files-up"
                    disabled={!sortable.canMoveUp(i)}
                    onClick={() => sortable.move(i, -1)}
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    data-testid="my-files-down"
                    disabled={!sortable.canMoveDown(i)}
                    onClick={() => sortable.move(i, 1)}
                  >
                    ↓
                  </button>
                </span>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

// Memoised on (bandId, songId): the panel is fully self-contained after mount (its own local
// displayOrder/included), so a PARENT re-render — the viewer strip refreshing after each toggle's
// onChanged — must NOT re-render it. Re-rendering it there caused a visible reflow/flicker in the
// panel even though nothing inside it had changed. onChanged/onError are stable per song (the panel
// reads onError via a ref), so freezing on song identity is safe.
export const MyFilesEditor = memo(
  MyFilesEditorInner,
  (a, b) => a.bandId === b.bandId && a.songId === b.songId,
);
