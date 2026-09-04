/**
 * The text-chart source editor (T19/T25/T39/T60) — EXTRACTED from SongDetails (T105) so a single
 * component serves two hosts without forking: the in-place dialog in the Details panel (T104) and the
 * dedicated full-page route `/…/chart/:fileId` (ChartEditorPage). "One editor, two hosts."
 *
 * The only host-specific behaviour is `persist` (T105): on the route, Back is the obvious exit (T68 made
 * `?file=` a `replace:true` mirror precisely so Back leaves the editor), and blocking a react-router
 * navigation would need `useBlocker`, which throws under `<BrowserRouter>`. So instead of warning, the
 * route host opts into DRAFT PERSISTENCE — Back / forward / reload just work, with the unsaved source
 * kept in sessionStorage. The dialog host leaves `persist` off (its Cancel is a button, no Back to lose).
 */
import { useEffect, useRef, useState } from "react";
import { ApiError, api } from "../../api";
import { ErrorBanner } from "../../components/ErrorBanner";
import { tokenizeChartSource } from "./chartHighlight";

// What a host hands the editor: the source to edit and, for an existing generated file, its id + the
// revision the edit is based on (LWW conflict detection + the draft-persistence key).
export type ChartEdit = { fileId?: string; source: string; baseRevision: number };

// HighlightedSource (T39): the chart source pane with dialect syntax highlighting via the
// overlay technique — a colored <pre> sits exactly behind a transparent-text <textarea>
// (caret + all editing from the textarea; color from the <pre>). The pane is MONOSPACE so
// chords line up over words AND the overlay stays glyph-aligned. The <pre> mirrors the
// textarea's scroll. `chart-source` stays the textarea's testid (specs type into it);
// preview is unchanged (still on-demand — no auto-render on type).
function HighlightedSource({
  value,
  onChange,
  disabled,
}: {
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
}) {
  const taRef = useRef<HTMLTextAreaElement>(null);
  const preRef = useRef<HTMLPreElement>(null);
  const syncScroll = () => {
    if (taRef.current && preRef.current) {
      preRef.current.scrollTop = taRef.current.scrollTop;
      preRef.current.scrollLeft = taRef.current.scrollLeft;
    }
  };
  // T135: tokenize the whole source so tab blocks ({sot}…{eot}) are highlighted with cross-line state.
  const lineTokens = tokenizeChartSource(value);
  return (
    <div className="chart-src-wrap">
      <pre className="chart-src-hl" aria-hidden="true" ref={preRef}>
        {lineTokens.map((toks, i) => (
          <span className="hl-line" key={i}>
            {toks.map((t, j) => (
              <span className={t.cls} key={j}>
                {t.text}
              </span>
            ))}
            {"\n"}
          </span>
        ))}
      </pre>
      <textarea
        data-testid="chart-source"
        className="chart-src-ta"
        ref={taRef}
        rows={14}
        value={value}
        spellCheck={false}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value)}
        onScroll={syncScroll}
      />
    </div>
  );
}

// keyRe: a bare musical key for UX gating only (prefill + key-vs-semitone path). This
// is NOT transposition — the transpose algorithm lives only on the server (T60). Mirrors
// the server's ParseKey grammar.
const keyRe = /^[A-G](#|b)?m?$/;

export function ChartEditor({
  bandId,
  songId,
  initial,
  songKey,
  onSongKeyChanged,
  onDone,
  onCancel,
  cancelLabel,
  persist,
}: {
  bandId: string;
  songId: string;
  initial: ChartEdit;
  songKey?: string;
  onSongKeyChanged?: (key: string) => void;
  onDone: () => void;
  onCancel: () => void;
  /** Label for the leave button. The route host reads "Back to song" (it navigates back and, with
   *  `persist`, keeps the draft — so "Cancel" would misdescribe it). Defaults to "Cancel". */
  cancelLabel?: string;
  /** T105: persist the unsaved draft to sessionStorage so a route host survives Back/forward/reload. */
  persist?: boolean;
}) {
  const [source, setSource] = useState(initial.source);
  // The last persisted source, for the dirty-editor guard: transposing Apply
  // overwrites the source in place, so it must be blocked while the textarea has
  // unsaved edits (else those edits are clobbered silently). Updated on a transpose
  // Apply (which persists); a normal Save closes the editor.
  const [savedSource, setSavedSource] = useState(initial.source);
  const [baseRevision, setBaseRevision] = useState(initial.baseRevision);
  const dirty = source !== savedSource;
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [previewing, setPreviewing] = useState(false);

  // T105 draft persistence (route host). Keyed by fileId + baseRevision: keying on the revision is
  // load-bearing — T60/T67 can move the source underneath, and a draft must never resurrect on a base
  // it wasn't written against. A different revision is simply a different (absent) key, so the draft
  // drops instead of restoring onto the wrong source. Tracks the CURRENT baseRevision so a post-transpose
  // draft persists under the new revision, not the stale one.
  const draftKeyFor = (rev: number) =>
    persist && initial.fileId ? `chartdraft:${bandId}:${songId}:${initial.fileId}:${rev}` : null;
  const draftKey = draftKeyFor(baseRevision);
  const [restored, setRestored] = useState(false);

  // Restore once, on mount, at the revision we loaded against.
  useEffect(() => {
    const key = draftKeyFor(initial.baseRevision);
    if (!key) return;
    const draft = sessionStorage.getItem(key);
    if (draft != null && draft !== initial.source) {
      setSource(draft);
      setRestored(true);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Persist while dirty; clear the key the moment the source matches the saved baseline again.
  useEffect(() => {
    if (!draftKey) return;
    if (source !== savedSource) sessionStorage.setItem(draftKey, source);
    else sessionStorage.removeItem(draftKey);
  }, [draftKey, source, savedSource]);

  function clearDraft() {
    if (draftKey) sessionStorage.removeItem(draftKey);
  }
  // A confirmed discard of the restored draft: drop it and fall back to the loaded source.
  function discardDraft() {
    clearDraft();
    setSource(savedSource);
    setRestored(false);
  }

  // T60 transpose control (existing generated charts only). The song key drives the
  // path: a parseable key → key picker + "also update song key"; otherwise a ± semitone
  // stepper (interval-only transpose is still well-defined server-side).
  const keyParses = keyRe.test((songKey ?? "").trim());
  const [transposeOpen, setTransposeOpen] = useState(false);
  const [targetKey, setTargetKey] = useState(keyParses ? (songKey ?? "").trim() : "");
  const [semitones, setSemitones] = useState(1);
  const [alsoUpdateKey, setAlsoUpdateKey] = useState(keyParses);
  const [transposing, setTransposing] = useState(false);

  // Revoke the object URL when it's replaced or the editor unmounts (cleanup runs
  // with the PREVIOUS url) — no blob leaks.
  useEffect(() => {
    return () => {
      if (previewUrl) URL.revokeObjectURL(previewUrl);
    };
  }, [previewUrl]);

  async function preview() {
    setPreviewing(true);
    setError(null);
    try {
      const blob = await api.previewTextChart(bandId, songId, source);
      setPreviewUrl(URL.createObjectURL(blob)); // effect revokes the old one
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to render preview");
    } finally {
      setPreviewing(false);
    }
  }

  async function save() {
    setBusy(true);
    setError(null);
    try {
      if (initial.fileId) {
        await api.saveChartSource(bandId, songId, initial.fileId, baseRevision, source);
      } else {
        await api.createTextChart(bandId, songId, source);
      }
      clearDraft(); // saved to the server — the draft is now redundant
      onDone();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save chart");
    } finally {
      setBusy(false);
    }
  }

  // Transpose request args: a parseable song key uses the key path (+ optional key
  // update); otherwise the ± semitone path. dryRun for preview, persist for apply.
  const transposeArgs = (dryRun: boolean) =>
    keyParses
      ? { targetKey: targetKey.trim(), updateSongKey: alsoUpdateKey, baseRevision, dryRun }
      : { semitones, baseRevision, dryRun };
  // Enabled only with a valid transpose to request (a parseable target key, or any
  // semitone count in the fallback path).
  const canTranspose = keyParses ? keyRe.test(targetKey.trim()) : true;

  async function previewTranspose() {
    if (!initial.fileId) return;
    setTransposing(true);
    setError(null);
    try {
      const { source: t } = await api.transposeChartSource(bandId, songId, initial.fileId, transposeArgs(true));
      const blob = await api.previewTextChart(bandId, songId, t);
      setPreviewUrl(URL.createObjectURL(blob)); // effect revokes the old one
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to preview transpose");
    } finally {
      setTransposing(false);
    }
  }

  async function applyTranspose() {
    if (!initial.fileId) return;
    setTransposing(true);
    setError(null);
    try {
      const { source: t, file } = await api.transposeChartSource(bandId, songId, initial.fileId, transposeArgs(false));
      setSource(t); // the editor now shows the transposed source (persisted in place)
      setSavedSource(t); // the transposed source is now the saved baseline (no longer dirty)
      if (file?.revision != null) setBaseRevision(file.revision);
      if (keyParses && alsoUpdateKey) onSongKeyChanged?.(targetKey.trim());
      setTransposeOpen(false);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to transpose");
    } finally {
      setTransposing(false);
    }
  }

  return (
    <div className="card chart-editor" data-testid="chart-editor">
      <div className="chart-editor-head">
        <h3>Lyrics &amp; chords</h3>
        <p className="muted" style={{ margin: 0 }}>
          Title, sections, chords and lyrics — <code># title</code> · <code>## section</code> ·
          chord lines over words.
        </p>
      </div>
      {restored && (
        <div className="chart-restored" data-testid="chart-restored">
          <span>Restored your unsaved edits.</span>
          <button
            type="button"
            className="ghost-btn btn-sm"
            data-testid="chart-restored-discard"
            onClick={discardDraft}
          >
            Discard
          </button>
        </div>
      )}
      <div className="chart-editor-panes">
        {/* D4: lock the source while a transpose Apply is in flight — typing mid-round-trip
            would be clobbered by the setSource/setSavedSource on completion. */}
        <HighlightedSource value={source} onChange={setSource} disabled={transposing} />
        <div className="chart-preview-pane">
          {previewUrl ? (
            <object
              data-testid="chart-preview"
              data={previewUrl}
              type="application/pdf"
              className="chart-preview-obj"
            >
              <a href={previewUrl}>Open preview PDF</a>
            </object>
          ) : (
            <p className="muted" data-testid="chart-preview-empty" style={{ margin: 0 }}>
              Click Preview to render this chart.
            </p>
          )}
        </div>
      </div>
      {initial.fileId && (
        <p className="muted" data-testid="chart-edit-caveat">
          Editing re-renders the PDF — layout may shift, so existing annotations on this
          chart can end up off their original spot.
        </p>
      )}
      <details className="muted">
        <summary>Chart format</summary>
        <pre>{`# Title
## Section          (Verse 1, Chorus, Bridge…)
G     D             a line of chords renders above the next lyric line
lyrics go here
**bold** in a normal text line
(blank line = paragraph gap)`}</pre>
      </details>
      {/* Transpose (T60 surface 1) — existing generated charts only. */}
      {initial.fileId && transposeOpen && (
        <div className="transpose-form" data-testid="transpose-form">
          {keyParses ? (
            <label className="field">
              <span>Transpose to key</span>
              <input
                data-testid="transpose-target-key"
                value={targetKey}
                onChange={(e) => setTargetKey(e.target.value)}
                placeholder="e.g. A, F#m, Bb"
                size={6}
              />
            </label>
          ) : (
            <label className="field">
              <span>Shift semitones</span>
              <input
                type="number"
                data-testid="transpose-semitones"
                value={semitones}
                min={-11}
                max={11}
                onChange={(e) => setSemitones(Number(e.target.value) || 0)}
                size={4}
              />
            </label>
          )}
          {keyParses && (
            <label className="check">
              <input
                type="checkbox"
                data-testid="transpose-update-key"
                checked={alsoUpdateKey}
                onChange={(e) => setAlsoUpdateKey(e.target.checked)}
              />
              Also update the song key
            </label>
          )}
          <button
            type="button"
            className="ghost-btn"
            data-testid="transpose-preview"
            disabled={transposing || !canTranspose}
            onClick={() => void previewTranspose()}
          >
            {transposing ? "…" : "Preview"}
          </button>
          <button
            type="button"
            data-testid="transpose-apply"
            disabled={transposing || !canTranspose || dirty}
            title={dirty ? "Save your chart edits first" : undefined}
            onClick={() => void applyTranspose()}
          >
            Apply
          </button>
          <button type="button" className="ghost-btn" onClick={() => setTransposeOpen(false)}>
            Close
          </button>
        </div>
      )}
      <div className="inline-form">
        <button type="button" data-testid="chart-save" disabled={busy} onClick={() => void save()}>
          {busy ? "Saving…" : "Save chart"}
        </button>
        <button
          type="button"
          className="ghost-btn"
          data-testid="chart-preview-btn"
          disabled={previewing}
          onClick={() => void preview()}
        >
          {previewing ? "Rendering…" : "Preview"}
        </button>
        {initial.fileId && (
          <button
            type="button"
            className="ghost-btn"
            data-testid="chart-transpose-btn"
            aria-expanded={transposeOpen}
            disabled={dirty}
            title={dirty ? "Save your chart edits first" : "Transpose the chords"}
            onClick={() => setTransposeOpen((o) => !o)}
          >
            Transpose…
          </button>
        )}
        <button type="button" className="ghost-btn" data-testid="chart-cancel" onClick={onCancel}>
          {cancelLabel ?? "Cancel"}
        </button>
      </div>
      <ErrorBanner message={error} />
    </div>
  );
}
