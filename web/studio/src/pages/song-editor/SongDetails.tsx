/**
 * Song-details section of the editor page (T10 extraction — moved verbatim from
 * SongEditor.tsx; behavior + data-testids unchanged): the collapsible wrapper,
 * metadata form, files list, and delete-song danger zone.
 */
import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import { ApiError, api, type Song, type SongFile } from "../../api";
import { ErrorBanner } from "../../components/ErrorBanner";
import { normalizeLyrics, detectSections } from "./lyrics";
import { tokenizeChartLine } from "./chartHighlight";

export function Details({ title, children }: { title: string; children: ReactNode }) {
  // Default OPEN: the viewer is the headline, but the existing flows expect the
  // metadata/files controls to be reachable without an extra click.
  const [open, setOpen] = useState(true);
  return (
    <section className="card details-section" data-testid="details-section">
      <button
        type="button"
        className="details-toggle"
        data-testid="details-toggle"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
      >
        {open ? "▾" : "▸"} {title}
      </button>
      {open && <div className="details-body">{children}</div>}
    </section>
  );
}

export function Metadata({
  bandId,
  song,
  onSaved,
}: {
  bandId: string;
  song: Song;
  onSaved: (s: Song) => void;
}) {
  const [title, setTitle] = useState(song.title);
  const [artist, setArtist] = useState(song.artist ?? "");
  const [key, setKey] = useState(song.key ?? "");
  const [tempo, setTempo] = useState(song.tempo != null ? String(song.tempo) : "");
  const [tags, setTags] = useState((song.tags ?? []).join(", "));
  const [notes, setNotes] = useState(song.notes ?? "");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  // T60: a chart-source transpose can update the song key server-side (from the
  // ChartEditor). Re-sync the key field when the song prop's key changes so the
  // displayed key reflects it (the field is otherwise seeded once on mount).
  useEffect(() => {
    setKey(song.key ?? "");
  }, [song.key]);

  async function onSave(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      const tagList = tags
        .split(",")
        .map((t) => t.trim())
        .filter(Boolean);
      const updated = await api.updateSong(bandId, song.id, {
        title,
        artist,
        key,
        tempo: tempo === "" ? 0 : Number(tempo),
        tags: tagList,
        notes,
      });
      onSaved(updated);
      setNotice("Saved.");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel">
      <div className="panel-head">
        <h2>Details</h2>
      </div>
      <div className="panel-body">
        <form onSubmit={onSave} data-testid="song-meta-form">
          <div className="form-grid">
            <div className="field wide">
              <label htmlFor="meta-title">Title</label>
              <input
                id="meta-title"
                data-testid="meta-title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                required
              />
            </div>
            <div className="field">
              <label htmlFor="meta-artist">Artist</label>
              <input
                id="meta-artist"
                data-testid="meta-artist"
                value={artist}
                onChange={(e) => setArtist(e.target.value)}
              />
            </div>
            <div className="field">
              <label htmlFor="meta-key">Key</label>
              <input
                id="meta-key"
                data-testid="meta-key"
                value={key}
                onChange={(e) => setKey(e.target.value)}
              />
            </div>
            <div className="field">
              <label htmlFor="meta-tempo">Tempo</label>
              <div className="input-affix">
                <input
                  id="meta-tempo"
                  data-testid="meta-tempo"
                  type="number"
                  value={tempo}
                  onChange={(e) => setTempo(e.target.value)}
                />
                <span className="affix">bpm</span>
              </div>
            </div>
            <div className="field">
              <label htmlFor="meta-tags">Tags</label>
              <input
                id="meta-tags"
                data-testid="meta-tags"
                value={tags}
                onChange={(e) => setTags(e.target.value)}
              />
              <span className="hint">Comma-separated.</span>
            </div>
            <div className="field wide">
              <label htmlFor="meta-notes">Notes</label>
              <textarea
                id="meta-notes"
                data-testid="meta-notes"
                rows={3}
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
              />
            </div>
          </div>
          <div className="form-foot">
            <button type="submit" className="primary" data-testid="meta-save" disabled={busy}>
              {busy ? "Saving…" : "Save details"}
            </button>
            {notice && (
              <span className="saved" data-testid="meta-notice">
                ✓ {notice}
              </span>
            )}
          </div>
        </form>
        <ErrorBanner message={error} />
      </div>
    </section>
  );
}

function fmtSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// chartEdit is the open text-chart editor state: a new chart (no fileId) or an
// existing generated file's source (fileId + the revision we based the edit on).
type chartEdit = { fileId?: string; source: string; baseRevision: number };

export function Files({
  bandId,
  songId,
  songTitle,
  songKey,
  onSongKeyChanged,
}: {
  bandId: string;
  songId: string;
  songTitle?: string;
  // T60: the song's current key drives the ChartEditor's Transpose control (prefill +
  // key/semitone path). onSongKeyChanged bubbles up a transpose that also updated the key.
  songKey?: string;
  onSongKeyChanged?: (key: string) => void;
}) {
  const [files, setFiles] = useState<SongFile[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [chart, setChart] = useState<chartEdit | null>(null);
  const [lyricsOpen, setLyricsOpen] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    try {
      const list = await api.listFiles(bandId, songId);
      list.sort((a, b) => a.displayOrder - b.displayOrder);
      setFiles(list);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load files");
    }
  }, [bandId, songId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onUpload(e: FormEvent) {
    e.preventDefault();
    const f = fileInput.current?.files?.[0];
    if (!f) return;
    setError(null);
    setBusy(true);
    try {
      await api.uploadFile(bandId, songId, f);
      if (fileInput.current) fileInput.current.value = "";
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to upload");
    } finally {
      setBusy(false);
    }
  }

  async function rename(file: SongFile) {
    const next = window.prompt("New filename", file.filename);
    if (!next || next === file.filename) return;
    setError(null);
    try {
      await api.updateFile(bandId, songId, file.id, { filename: next });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to rename");
    }
  }

  async function remove(file: SongFile) {
    if (!window.confirm(`Delete "${file.filename}"?`)) return;
    setError(null);
    try {
      await api.deleteFile(bandId, songId, file.id);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to delete");
    }
  }

  async function editChartSource(file: SongFile) {
    setError(null);
    try {
      const { source, file: cur } = await api.getChartSource(bandId, songId, file.id);
      setChart({ fileId: file.id, source, baseRevision: cur.revision ?? 1 });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load chart source");
    }
  }

  async function move(index: number, dir: -1 | 1) {
    const other = index + dir;
    if (other < 0 || other >= files.length) return;
    const a = files[index];
    const b = files[other];
    setError(null);
    try {
      // Swap display orders.
      await api.updateFile(bandId, songId, a.id, { displayOrder: b.displayOrder });
      await api.updateFile(bandId, songId, b.id, { displayOrder: a.displayOrder });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to reorder");
    }
  }

  return (
    <section className="panel">
      <div className="panel-head">
        <h2>Files</h2>
        <span className="count">
          {files.length} in the pool
        </span>
        <div className="head-actions">
          <button
            type="button"
            className="btn-sm ghost-btn"
            data-testid="new-text-chart"
            onClick={() => setChart({ source: "# New chart\n\n## Verse 1\n", baseRevision: 0 })}
          >
            ＋ New text chart
          </button>
          <button
            type="button"
            className="btn-sm ghost-btn"
            data-testid="new-lyrics-chart"
            onClick={() => setLyricsOpen(true)}
          >
            ＋ New chart from lyrics
          </button>
        </div>
      </div>
      <div className="panel-body">
        <form onSubmit={onUpload} className="inline-form" data-testid="file-upload-form">
          <input ref={fileInput} type="file" data-testid="file-input" />
          <button type="submit" className="primary btn-sm" data-testid="file-upload" disabled={busy}>
            Upload
          </button>
        </form>

        {lyricsOpen && (
          <LyricsImportDialog
            bandId={bandId}
            defaultName={songTitle ?? ""}
            onCancel={() => setLyricsOpen(false)}
            onCreate={(source) => {
              setLyricsOpen(false);
              setChart({ source, baseRevision: 0 });
            }}
          />
        )}

        {chart && (
          <ChartEditor
            bandId={bandId}
            songId={songId}
            initial={chart}
            songKey={songKey}
            onSongKeyChanged={onSongKeyChanged}
            onCancel={() => setChart(null)}
            onDone={() => {
              setChart(null);
              void load();
            }}
          />
        )}

        <ErrorBanner message={error} />

        {files.length === 0 ? (
          <p className="muted" data-testid="files-empty">
            No files yet — upload a PDF or image, or create a text chart.
          </p>
        ) : (
          <div className="file-grid" data-testid="files-list">
            {files.map((f, i) => (
              <div key={f.id} className={`file${f.generated ? " gen" : ""}`} data-testid="file-row">
                <div className="ftop">
                  <div className="thumb" aria-hidden="true">
                    {f.generated ? "✎" : f.contentType.startsWith("image/") ? "🖼" : "♪"}
                  </div>
                  <div style={{ minWidth: 0 }}>
                    <a
                      href={api.fileUrl(f.id)}
                      target="_blank"
                      rel="noreferrer"
                      className="fname"
                      data-testid="file-download"
                    >
                      {f.filename}
                    </a>
                    <div className="fmeta">{fmtSize(f.size)}</div>
                  </div>
                  {f.generated && (
                    <span className="chip brand" data-testid="file-chart-badge">
                      text chart
                    </span>
                  )}
                </div>
                <div className="facts">
                  <button
                    type="button"
                    className="icon-btn"
                    data-testid="file-up"
                    title="Move up"
                    disabled={i === 0}
                    onClick={() => move(i, -1)}
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    className="icon-btn"
                    data-testid="file-down"
                    title="Move down"
                    disabled={i === files.length - 1}
                    onClick={() => move(i, 1)}
                  >
                    ↓
                  </button>
                  {f.generated && (
                    <button
                      type="button"
                      className="btn-sm"
                      data-testid="file-edit-source"
                      onClick={() => void editChartSource(f)}
                    >
                      Edit source
                    </button>
                  )}
                  <button type="button" className="btn-sm" data-testid="file-rename" onClick={() => rename(f)}>
                    Rename
                  </button>
                  <button
                    type="button"
                    className="icon-btn"
                    data-testid="file-delete"
                    title="Delete"
                    onClick={() => remove(f)}
                  >
                    ✕
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}

// ChartEditor is the text-chart source editor: a plain textarea in the tiny chart
// dialect beside a rendered-PDF preview pane (T25), saved server-side to a rendered
// PDF (new file, or re-render in place for an existing generated file). A save
// conflict (someone edited first) surfaces the server's "reload" message. Preview
// renders on demand (no per-keystroke round-trips) via the no-persistence endpoint.
// LyricsImportDialog (T37): paste-first "New chart from lyrics". A "Fetch from URL"
// accelerator sits above a paste textarea; on success it FILLS the textarea (the user
// still reviews), on a blocked/failed fetch it shows an honest message and leaves focus
// in the textarea — never a dead end. "Create" hands a normalized chart source up; the
// existing ChartEditor then opens for cleanup. Fetch is best-effort (azlyrics is
// Cloudflare-gated); paste always works.
function LyricsImportDialog({
  bandId,
  defaultName,
  onCreate,
  onCancel,
}: {
  bandId: string;
  defaultName: string;
  onCreate: (source: string) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState(defaultName);
  const [url, setUrl] = useState("");
  const [text, setText] = useState("");
  const [msg, setMsg] = useState<string | null>(null);
  const [fetching, setFetching] = useState(false);
  const [labelSections, setLabelSections] = useState(false); // T38: opt-in, default OFF
  const textRef = useRef<HTMLTextAreaElement>(null);

  async function onFetch() {
    if (!url.trim() || fetching) return;
    setMsg(null);
    setFetching(true);
    try {
      const r = await api.lyricsImport(bandId, url.trim());
      if (r.status === "ok" && r.text) {
        setText(r.text);
        setMsg("Fetched — review the lyrics below, then create the chart.");
      } else {
        setMsg("Couldn’t fetch — the site blocked the request. Paste the lyrics below instead.");
        textRef.current?.focus();
      }
    } catch {
      setMsg("Couldn’t fetch — paste the lyrics below instead.");
      textRef.current?.focus();
    } finally {
      setFetching(false);
    }
  }

  function create() {
    const heading = name.trim() || "New chart";
    const body = normalizeLyrics(text);
    const structured = labelSections ? detectSections(body) : body;
    onCreate(`# ${heading}\n\n${structured}\n`);
  }

  return (
    <div className="card chart-editor" data-testid="lyrics-dialog">
      <label className="field">
        <span>Chart name</span>
        <input data-testid="lyrics-name" value={name} onChange={(e) => setName(e.target.value)} />
      </label>
      <div className="inline-form lyrics-fetch-row">
        <input
          data-testid="lyrics-url"
          placeholder="Paste an azlyrics (or any) song URL"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
        />
        <button type="button" className="btn-sm" data-testid="lyrics-fetch" disabled={fetching} onClick={() => void onFetch()}>
          {fetching ? "Fetching…" : "Fetch from URL"}
        </button>
      </div>
      {msg && (
        <p className="muted" data-testid="lyrics-fetch-msg">
          {msg}
        </p>
      )}
      <textarea
        data-testid="lyrics-text"
        ref={textRef}
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder="…or paste the lyrics here"
        rows={10}
      />
      <div className="lyrics-sections-toggle">
        <input
          type="checkbox"
          id="lyrics-sections-cb"
          data-testid="lyrics-sections"
          checked={labelSections}
          onChange={(e) => setLabelSections(e.target.checked)}
        />
        <label htmlFor="lyrics-sections-cb">
          Label verses &amp; choruses (groups the stanzas into ## sections — you can edit them next)
        </label>
      </div>
      <div className="inline-form">
        <button type="button" className="primary btn-sm" data-testid="lyrics-create" disabled={!text.trim()} onClick={create}>
          Create chart
        </button>
        <button type="button" className="ghost-btn btn-sm" onClick={onCancel}>
          Cancel
        </button>
      </div>
      <p className="muted" data-testid="chart-edit-caveat" style={{ margin: 0 }}>
        Fetching is best-effort — many lyric sites (azlyrics) block automated requests, so
        paste is always available. You’ll review and tidy the chart next.
      </p>
    </div>
  );
}

// HighlightedSource (T39): the chart source pane with dialect syntax highlighting via the
// overlay technique — a colored <pre> sits exactly behind a transparent-text <textarea>
// (caret + all editing from the textarea; color from the <pre>). The pane is MONOSPACE so
// chords line up over words AND the overlay stays glyph-aligned. The <pre> mirrors the
// textarea's scroll. `chart-source` stays the textarea's testid (specs type into it);
// preview is unchanged (still on-demand — no auto-render on type).
function HighlightedSource({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const taRef = useRef<HTMLTextAreaElement>(null);
  const preRef = useRef<HTMLPreElement>(null);
  const syncScroll = () => {
    if (taRef.current && preRef.current) {
      preRef.current.scrollTop = taRef.current.scrollTop;
      preRef.current.scrollLeft = taRef.current.scrollLeft;
    }
  };
  const lines = value.split("\n");
  return (
    <div className="chart-src-wrap">
      <pre className="chart-src-hl" aria-hidden="true" ref={preRef}>
        {lines.map((ln, i) => (
          <span className="hl-line" key={i}>
            {tokenizeChartLine(ln).map((t, j) => (
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

function ChartEditor({
  bandId,
  songId,
  initial,
  songKey,
  onSongKeyChanged,
  onDone,
  onCancel,
}: {
  bandId: string;
  songId: string;
  initial: chartEdit;
  songKey?: string;
  onSongKeyChanged?: (key: string) => void;
  onDone: () => void;
  onCancel: () => void;
}) {
  const [source, setSource] = useState(initial.source);
  const [baseRevision, setBaseRevision] = useState(initial.baseRevision);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [previewing, setPreviewing] = useState(false);

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
      <div className="chart-editor-panes" style={{ display: "flex", flexWrap: "wrap", gap: "0.75rem" }}>
        <HighlightedSource value={source} onChange={setSource} />
        <div style={{ flex: "1 1 20rem", minWidth: "16rem" }}>
          {previewUrl ? (
            <object
              data-testid="chart-preview"
              data={previewUrl}
              type="application/pdf"
              style={{ width: "100%", height: "22rem", border: "1px solid var(--border, #ccc)" }}
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
            disabled={transposing || !canTranspose}
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
            onClick={() => setTransposeOpen((o) => !o)}
          >
            Transpose…
          </button>
        )}
        <button type="button" className="ghost-btn" onClick={onCancel}>
          Cancel
        </button>
      </div>
      <ErrorBanner message={error} />
    </div>
  );
}

export function DeleteSong({
  bandId,
  songId,
  onDeleted,
}: {
  bandId: string;
  songId: string;
  onDeleted: () => void;
}) {
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onDelete() {
    if (!window.confirm("Delete this song? This cannot be undone.")) return;
    setError(null);
    setBusy(true);
    try {
      await api.deleteSong(bandId, songId);
      onDeleted();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to delete song");
      setBusy(false);
    }
  }

  return (
    <section className="card">
      <h2>Danger zone</h2>
      <div className="inline-form">
        <button type="button" data-testid="delete-song" disabled={busy} onClick={onDelete}>
          Delete song
        </button>
      </div>
      <ErrorBanner message={error} />
    </section>
  );
}
