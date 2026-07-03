/**
 * My-files editor panel (T10 extraction — moved verbatim from SongEditor.tsx):
 * per-member file selection (exclude / reorder / reset) driving the viewer strip.
 * Behavior + data-testids unchanged.
 */
import { useCallback, useEffect, useMemo, useState } from "react";
import { api, type SongFile } from "../../api";

export function MyFilesEditor({
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
  // My ordered selection of fileIds — seeded from the current strip, then kept
  // in sync after each apply (onChanged returns the server's canonical order).
  const [order, setOrder] = useState<string[]>(selected.map((f) => f.id));
  const [busy, setBusy] = useState(false);

  // Load the whole pool once.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const list = await api.listFiles(bandId, songId);
        if (cancelled) return;
        list.sort((a, b) => a.displayOrder - b.displayOrder);
        setPool(list);
      } catch (err) {
        if (cancelled) return;
        onError(err instanceof Error ? err.message : "Failed to load files");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [bandId, songId, onError]);

  const poolById = useMemo(() => {
    const m = new Map<string, SongFile>();
    for (const f of pool) m.set(f.id, f);
    return m;
  }, [pool]);

  // Apply an ordered selection: PUT, then refresh the strip and re-seed order
  // from the server's canonical response (drops any files removed from the pool).
  const apply = useCallback(
    async (nextOrder: string[]) => {
      setBusy(true);
      onError(null);
      try {
        await api.setMyFiles(bandId, songId, nextOrder);
        const mine = await onChanged();
        setOrder(mine.map((f) => f.id));
      } catch (err) {
        onError(err instanceof Error ? err.message : "Failed to update selection");
      } finally {
        setBusy(false);
      }
    },
    [bandId, songId, onChanged, onError],
  );

  function toggleInclude(id: string) {
    const next = order.includes(id) ? order.filter((x) => x !== id) : [...order, id];
    void apply(next);
  }

  function moveIncluded(index: number, dir: -1 | 1) {
    const other = index + dir;
    if (other < 0 || other >= order.length) return;
    const next = [...order];
    [next[index], next[other]] = [next[other], next[index]];
    void apply(next);
  }

  async function reset() {
    setBusy(true);
    onError(null);
    try {
      await api.clearMyFiles(bandId, songId);
      const mine = await onChanged();
      setOrder(mine.map((f) => f.id));
    } catch (err) {
      onError(err instanceof Error ? err.message : "Failed to reset");
    } finally {
      setBusy(false);
    }
  }

  // Included files (in MY order) first, then the rest of the pool.
  const includedFiles = order.map((id) => poolById.get(id)).filter((f): f is SongFile => !!f);
  const excludedFiles = pool.filter((f) => !order.includes(f.id));

  return (
    <section className="my-files-panel card" data-testid="my-files-panel">
      <div className="my-files-panel-head">
        <h2>My files</h2>
        <button
          type="button"
          className="my-files-reset-btn"
          data-testid="my-files-reset"
          disabled={busy}
          onClick={() => void reset()}
        >
          Reset to all
        </button>
      </div>
      <p className="muted my-files-hint">
        Pick which files appear in your strip and in what order. Everyone shares the same
        pool (managed in “Details &amp; files”).
      </p>

      {pool.length === 0 ? (
        <p className="muted">No files in the pool yet.</p>
      ) : (
        <ul className="list my-files-list">
          {includedFiles.map((f, i) => (
            <li key={f.id} data-testid="my-files-row" className="my-files-row included">
              <label className="my-files-row-main">
                <input
                  type="checkbox"
                  data-testid="my-files-include"
                  checked
                  disabled={busy}
                  onChange={() => toggleInclude(f.id)}
                />
                <span className="my-files-name">{f.filename}</span>
              </label>
              <span className="actions">
                <button
                  type="button"
                  data-testid="my-files-up"
                  disabled={busy || i === 0}
                  onClick={() => moveIncluded(i, -1)}
                >
                  ↑
                </button>
                <button
                  type="button"
                  data-testid="my-files-down"
                  disabled={busy || i === includedFiles.length - 1}
                  onClick={() => moveIncluded(i, 1)}
                >
                  ↓
                </button>
              </span>
            </li>
          ))}
          {excludedFiles.map((f) => (
            <li key={f.id} data-testid="my-files-row" className="my-files-row excluded">
              <label className="my-files-row-main">
                <input
                  type="checkbox"
                  data-testid="my-files-include"
                  checked={false}
                  disabled={busy}
                  onChange={() => toggleInclude(f.id)}
                />
                <span className="my-files-name muted">{f.filename}</span>
              </label>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

