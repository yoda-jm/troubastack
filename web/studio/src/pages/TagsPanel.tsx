import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, api } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { useDialogs } from "../components/Dialog";
import {
  adoptOffer,
  compareTagNames,
  deleteMessage,
  planRename,
  validateNewTag,
  type TagCount,
  type TaggedSong,
} from "./band-tags";

// T188 — the order toggle is remembered per browser; a broken localStorage means "default A–Z".
type TagOrder = "az" | "used";
const ORDER_KEY = "troubastack.tags.order";
function readOrder(): TagOrder {
  try {
    return localStorage.getItem(ORDER_KEY) === "used" ? "used" : "az";
  } catch {
    return "az";
  }
}
function writeOrder(o: TagOrder) {
  try {
    localStorage.setItem(ORDER_KEY, o);
  } catch {
    // per-viewer convenience only; a broken store just means the default next load
  }
}

/**
 * The band's tag vocabulary as a panel. T188 made it COMPACT: a chip cloud by default (one wrapping line of
 * name+count chips, a click filters the song list, §3 onPick), in natural A–Z order (toggle to Most-used,
 * remembered per browser). A STORED spelling is still its own chip — encore and Encore are two — which is
 * what lets a reader see and clean up fragmentation. The cleanup lives in **Manage** mode, which switches to
 * the T182 rows with Rename (which merges, ⟨D3⟩) and Delete (⟨D4⟩), each behind the T182 confirmation that
 * states the consequence in numbers and that it cannot be undone (⟨D5⟩) — unchanged here.
 */
export function TagsPanel({
  bandId,
  onPick,
  onChanged,
}: {
  bandId: string;
  onPick: (tag: string) => void;
  onChanged: () => void;
}) {
  const [tags, setTags] = useState<TagCount[]>([]);
  // The songs (with their tags) back the EXACT counts the confirmation states (⟨D5⟩/Fable ⟨1⟩): counting
  // distinct carriers, not summing per-spelling counts, which would double-count a song carrying both the
  // merged spellings. This is the same list the song panel reads; a refetch keeps the numbers current.
  const [songs, setSongs] = useState<TaggedSong[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // T188 ⟨D1⟩/⟨D2⟩ — chips by default, rows only in Manage (never persisted — it is a cleanup mode, not a
  // way to read the panel). The A–Z/Most-used order IS remembered per browser.
  const [manage, setManage] = useState(false);
  const [order, setOrder] = useState<TagOrder>(() => readOrder());
  const setOrderPersisted = useCallback((o: TagOrder) => {
    setOrder(o);
    writeOrder(o);
  }, []);
  const { confirm, prompt } = useDialogs();

  // T188 ⟨D2⟩/⟨D4⟩ — one natural order (compareTagNames) for A–Z; count-descending for Most-used with the
  // same natural tie-break. The panel sorts on the CLIENT; it does not rely on the server's /tags order.
  const ordered = useMemo(() => {
    const t = [...tags];
    if (order === "used") t.sort((a, b) => b.count - a.count || compareTagNames(a.tag, b.tag));
    else t.sort((a, b) => compareTagNames(a.tag, b.tag));
    return t;
  }, [tags, order]);

  const load = useCallback(async () => {
    try {
      const [t, s] = await Promise.all([api.bandTags(bandId), api.listSongs(bandId)]);
      setTags(t);
      setSongs(s);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load tags");
    }
  }, [bandId]);

  useEffect(() => {
    void load();
  }, [load]);

  // After a successful edit, report the SERVER's number (not the dialog's preview — they differ if another
  // member edited in between, §5), then refetch this panel and signal the song list to refetch.
  const afterEdit = useCallback(
    async (changed: number, verb: string) => {
      setNotice(`${verb} on ${changed} ${changed === 1 ? "song" : "songs"}.`);
      await load();
      onChanged();
    },
    [load, onChanged],
  );

  // Map a failed edit to a legible line. A 404 means the row is stale — the tag is no longer in use.
  const reportError = useCallback((err: unknown, stale: string) => {
    if (err instanceof ApiError) {
      setError(err.status === 404 ? stale : err.message);
    } else {
      setError("The change could not be saved.");
    }
  }, []);

  async function onRename(from: string) {
    setError(null);
    setNotice(null);
    const typed = await prompt({
      title: `Rename “${from}”`,
      label: "New name for this tag",
      initial: from,
      placeholder: "tag name",
      confirmLabel: "Continue",
    });
    if (typed === null) return; // cancelled
    const invalid = validateNewTag(typed);
    if (invalid) {
      setError(invalid);
      return;
    }
    // ⟨D3⟩ adopt-existing: if the typed spelling folds onto a DIFFERENT existing one, offer that first, so a
    // rename cannot mint a third spelling. Declining keeps what they typed.
    let to = typed.trim();
    const offer = adoptOffer(typed, tags, from);
    if (offer) {
      const adopt = await confirm({
        title: `Use “${offer.tag}”?`,
        body: `“${offer.tag}” already exists on ${offer.count} ${offer.count === 1 ? "song" : "songs"}. Use that spelling instead of “${to}”, so the band keeps one spelling?`,
        confirmLabel: `Use “${offer.tag}”`,
        cancelLabel: `Keep “${to}”`,
      });
      if (adopt) to = offer.tag;
    }
    if (to === from) return; // no-op after adopt resolution

    const plan = planRename(from, to, songs);
    const ok = await confirm({
      title: plan.kind === "merge" ? "Merge tags?" : "Rename tag?",
      body: (
        <>
          <span data-testid="tag-consequence">{plan.message}</span>
          <br />
          <strong>This cannot be undone.</strong>
        </>
      ),
      danger: true,
      confirmLabel: plan.kind === "merge" ? "Merge" : "Rename",
    });
    if (!ok) return;

    setBusy(true);
    try {
      const changed = await api.renameTag(bandId, from, to);
      await afterEdit(changed, plan.kind === "merge" ? "Merged" : "Renamed");
    } catch (err) {
      reportError(err, `“${from}” is no longer in use — the list was out of date.`);
      await load();
    } finally {
      setBusy(false);
    }
  }

  async function onDelete(tag: string) {
    setError(null);
    setNotice(null);
    // Exact count from the held songs, for the same reason as the merge number.
    const count = songs.filter((s) => (s.tags ?? []).includes(tag)).length;
    const ok = await confirm({
      title: "Delete tag?",
      body: (
        <>
          <span data-testid="tag-consequence">{deleteMessage(tag, count)}</span>
          <br />
          <strong>This cannot be undone.</strong>
        </>
      ),
      danger: true,
      confirmLabel: "Delete",
    });
    if (!ok) return;

    setBusy(true);
    try {
      const changed = await api.deleteTag(bandId, tag);
      await afterEdit(changed, "Removed");
    } catch (err) {
      reportError(err, `“${tag}” is no longer in use — the list was out of date.`);
      await load();
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel" data-testid="tags-panel">
      <div className="panel-head">
        <h2>Tags</h2>
        <span className="count">{tags.length}</span>
        {tags.length > 0 && (
          <span className="tags-head-controls">
            {/* T188 ⟨D2⟩ — order toggle, A–Z default; remembered per browser. */}
            <span className="segmented" role="group" aria-label="Tag order">
              <button
                type="button"
                className={"seg-btn" + (order === "az" ? " on" : "")}
                data-testid="tags-order-az"
                aria-pressed={order === "az"}
                onClick={() => setOrderPersisted("az")}
              >
                A–Z
              </button>
              <button
                type="button"
                className={"seg-btn" + (order === "used" ? " on" : "")}
                data-testid="tags-order-used"
                aria-pressed={order === "used"}
                onClick={() => setOrderPersisted("used")}
              >
                Most used
              </button>
            </span>
            {/* T188 ⟨D1⟩ — Manage switches to the T182 rows (Rename/Delete); not remembered. */}
            <button
              type="button"
              className={"ghost-btn" + (manage ? " active" : "")}
              data-testid="tags-manage"
              aria-pressed={manage}
              onClick={() => setManage((v) => !v)}
            >
              {manage ? "Done" : "Manage"}
            </button>
          </span>
        )}
      </div>
      <div className="panel-body">
        <ErrorBanner message={error} />
        {notice && (
          <p className="muted" data-testid="tags-notice" role="status">
            {notice}
          </p>
        )}
        {tags.length === 0 ? (
          <p className="muted" data-testid="tags-empty">
            No tags yet — add them from a song’s details.
          </p>
        ) : manage ? (
          // Manage mode: the T182 rows (name · count · Rename · Delete), in the selected order. All of
          // T182's rename/merge/delete behaviour is unchanged — only when the rows are shown is new.
          <ul className="tag-list" data-testid="tag-list">
            {ordered.map((t) => (
              <li key={t.tag} className="tag-row" data-testid="tag-row">
                <button
                  type="button"
                  className="link-button tag-row-name"
                  data-testid="tag-row-name"
                  onClick={() => onPick(t.tag)}
                  title="Filter the song list by this tag"
                >
                  {t.tag}
                </button>
                <span className="count tag-row-count" data-testid="tag-row-count">
                  {t.count}
                </span>
                <span className="tag-row-actions">
                  <button
                    type="button"
                    className="ghost-btn"
                    data-testid="tag-rename"
                    disabled={busy}
                    onClick={() => void onRename(t.tag)}
                  >
                    Rename
                  </button>
                  <button
                    type="button"
                    className="ghost-btn danger"
                    data-testid="tag-delete"
                    disabled={busy}
                    onClick={() => void onDelete(t.tag)}
                  >
                    Delete
                  </button>
                </span>
              </li>
            ))}
          </ul>
        ) : (
          // Chip mode (default): a compact wrapping cloud, one chip look shared with the refine strip
          // (.tag-cloud-item/.tag-cloud-count). A click filters the song list (T182 §3, onPick). No cap.
          <div className="tag-cloud" data-testid="tag-cloud">
            {ordered.map((t) => (
              <button
                key={t.tag}
                type="button"
                className="tag-cloud-item"
                data-testid="tag-chip"
                onClick={() => onPick(t.tag)}
                title="Filter the song list by this tag"
              >
                {t.tag}
                <span className="tag-cloud-count" data-testid="tag-chip-count">
                  {t.count}
                </span>
              </button>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
