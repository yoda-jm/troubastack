import { useCallback, useEffect, useState } from "react";
import { ApiError, api } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { useDialogs } from "../components/Dialog";
import {
  adoptOffer,
  deleteMessage,
  planRename,
  validateNewTag,
  type TagCount,
  type TaggedSong,
} from "./band-tags";

/**
 * T182 ⟨D1⟩ — the band's tag vocabulary as a panel: one row per STORED spelling (encore and Encore are two
 * rows, on purpose — this is where a reader cleans up that fragmentation), most-used first. A tag name is a
 * button that adds the tag as a search chip on the song list (§3, via onPick). Each row can rename (which
 * merges, ⟨D3⟩) or delete (⟨D4⟩), always behind a confirmation that states the consequence in numbers and
 * that it cannot be undone (⟨D5⟩) — the only safety a feature with no metadata history has.
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
  const { confirm, prompt } = useDialogs();

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
        ) : (
          <ul className="tag-list" data-testid="tag-list">
            {tags.map((t) => (
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
        )}
      </div>
    </section>
  );
}
