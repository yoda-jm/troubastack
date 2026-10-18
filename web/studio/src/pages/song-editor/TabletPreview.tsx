import { useEffect, useRef, useState } from "react";
import { api, ApiError, type Crop } from "../../api";

// TabletPreview (T198 U1b) shows ONE page as it will bake under the current (draft) crop — the server-
// rendered PNG (raster cropped + annotations composited). No client-side transform: the image IS the bake's
// output. It fetches LAZILY (only once the page scrolls near the viewport) and caches per
// (file, page, crop, doc-signature), so toggling the view or scrolling back is instant, a crop change
// re-renders just that page, and — crucially — an ANNOTATION or LAYER edit invalidates the cached PNG (the
// signature folds the doc state), so toggling back after a mark never shows a stale preview.

// Bounded LRU of object URLs: key → URL, in insertion/recency order. Capped so a long editing session can't
// leak blobs, and a URL is revoked when it is evicted or superseded by a newer key for the same (file, page).
const CACHE_CAP = 40;
const previewURLs = new Map<string, string>();

function cacheGet(key: string): string | undefined {
  const u = previewURLs.get(key);
  if (u !== undefined) {
    previewURLs.delete(key); // re-insert to mark most-recently-used
    previewURLs.set(key, u);
  }
  return u;
}

function cachePut(key: string, url: string, fileId: string, page: number): void {
  const prefix = `${fileId}:${page}:`;
  for (const [k, u] of previewURLs) {
    // Supersede: a newer key for the SAME page (its crop or doc state changed) retires the old blob.
    if (k !== key && k.startsWith(prefix)) {
      URL.revokeObjectURL(u);
      previewURLs.delete(k);
    }
  }
  previewURLs.set(key, url);
  while (previewURLs.size > CACHE_CAP) {
    const oldest = previewURLs.keys().next().value as string | undefined;
    if (oldest === undefined) break;
    const u = previewURLs.get(oldest);
    if (u) URL.revokeObjectURL(u);
    previewURLs.delete(oldest);
  }
}

function keyFor(fileId: string, page: number, crop: Crop, sig: string): string {
  return `${fileId}:${page}:${JSON.stringify(crop.pages?.[String(page)] ?? null)}:${sig}`;
}

export type TabletPreviewProps = {
  bandId: string;
  songId: string;
  fileId: string;
  page: number;
  crop: Crop;
  /** A signature of the doc state the bake depends on (page objects + layer set); folded into the cache key
   *  so an edit invalidates the cached preview. */
  sig: string;
  /** Visible layer ids to paint; undefined ⇒ the bake's default view (what the tablet shows). */
  layers?: string[];
};

export default function TabletPreview({ bandId, songId, fileId, page, crop, sig, layers }: TabletPreviewProps) {
  const ref = useRef<HTMLDivElement>(null);
  const key = keyFor(fileId, page, crop, sig);
  const [url, setUrl] = useState<string | null>(() => cacheGet(key) ?? null);
  const [error, setError] = useState<string | null>(null);
  const [seen, setSeen] = useState(false); // has the page entered (near) the viewport?

  // Lazy: only fetch once the page is near the viewport.
  useEffect(() => {
    if (seen) return;
    const el = ref.current;
    if (!el) return;
    const io = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          setSeen(true);
          io.disconnect();
        }
      },
      { rootMargin: "400px" }, // prefetch a little ahead of the scroll
    );
    io.observe(el);
    return () => io.disconnect();
  }, [seen]);

  // Fetch (or reuse the cache) when seen, and whenever the key (page's crop or doc state) changes.
  useEffect(() => {
    if (!seen) return;
    const cached = cacheGet(key);
    if (cached) {
      setUrl(cached);
      setError(null);
      return;
    }
    let live = true;
    setError(null);
    api
      .cropPreview(bandId, songId, fileId, page, crop, layers)
      .then((blob) => {
        if (!live) return;
        const u = URL.createObjectURL(blob);
        cachePut(key, u, fileId, page);
        setUrl(u);
      })
      .catch((e) => {
        if (live) setError(e instanceof ApiError ? e.message : "Couldn't render the tablet preview.");
      });
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [seen, key]);

  return (
    <div ref={ref} className="tablet-preview" data-testid={`tablet-preview-${page}`}>
      {url ? (
        <img className="tablet-preview-img" src={url} alt={`Page ${page + 1} as it will bake`} />
      ) : error ? (
        <div className="tablet-preview-msg" data-testid={`tablet-preview-error-${page}`} role="alert">
          {error}
        </div>
      ) : (
        <div className="tablet-preview-msg">Rendering…</div>
      )}
    </div>
  );
}
