/**
 * T208 — "Download PDF" dialog for a file row's ⋯ menu. Two choices for a PDF / text-chart file:
 *
 *   • "With annotations" (default): THIS file's layers, pre-ticked by defaultVisibility — exactly what
 *     the viewer shows when it opens the file. A mandatory layer is locked on ("always printed"). The
 *     request reuses api.filePdf and visibleLayerIdsForFile (the one layer rule FilePdfButton uses), so
 *     it ALWAYS sends `layers=` explicitly — the ticked ids, empty when none are ticked, NEVER omitted.
 *     Omitting it would hand the server its own absent-param default (role ""), which drops a role-tagged
 *     shared layer that the client's defaultVisibility shows — the dialog would print less than it listed.
 *     Mandatory layers aren't sent: the server forces them on regardless (bake.fileLayerVisible), so
 *     leaving them out is what lets "untick everything" reach a truly empty `layers=`.
 *
 *   • "Without annotations": the stored file for the current revision — an uploaded PDF untouched, no
 *     crop; a text chart as rendered. Exactly the bytes the file-name link already serves.
 *
 * Both are real downloads (saveBlob), never a new tab. Built like BakeDialog/PdfDialog; errors stay in
 * the dialog via ErrorBanner.
 */
import { useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { ApiError, api, type AnnotationLayer, type SongFile } from "../../api";
import { ErrorBanner } from "../../components/ErrorBanner";
import type { LayerVisibility } from "./helpers";
import { saveBlob, visibleLayerIdsForFile } from "./FilePdfButton";
import { defaultVisibility } from "./useSongSync";

type Choice = "with" | "without";

export function DownloadPdfDialog({
  bandId,
  songId,
  file,
  layers,
  myUserId,
  onClose,
}: {
  bandId: string;
  songId: string;
  file: SongFile;
  layers: readonly AnnotationLayer[];
  myUserId: string | null;
  onClose: () => void;
}) {
  // THIS file's layers only — song-level layers (no fileId) are never drawn over a file, so they are
  // neither listed nor sent. Same `l.fileId === fileId` set visibleLayerIdsForFile draws from.
  const fileLayers = useMemo(
    () => layers.filter((l) => l.fileId === file.id),
    [layers, file.id],
  );
  const hasLayers = fileLayers.length > 0;

  // Seed the ticks at what the viewer shows on open. Mandatory layers are left OUT of this map on
  // purpose (the server always paints them); the checklist shows them locked-on from `l.mandatory`.
  const [checked, setChecked] = useState<LayerVisibility>(() => {
    const defaults = defaultVisibility([...fileLayers], myUserId);
    const seed: LayerVisibility = {};
    for (const l of fileLayers) if (!l.mandatory) seed[l.id] = defaults[l.id];
    return seed;
  });
  // A file with no layers can only be downloaded as uploaded, so that choice is selected and the
  // annotated one is disabled.
  const [choice, setChoice] = useState<Choice>(hasLayers ? "with" : "without");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function toggle(id: string) {
    setChecked((c) => ({ ...c, [id]: !c[id] }));
  }

  async function run() {
    setBusy(true);
    setError(null);
    try {
      if (choice === "with") {
        const ids = visibleLayerIdsForFile(fileLayers, checked, file.id);
        const { blob, filename } = await api.filePdf(bandId, songId, file.id, ids);
        saveBlob(blob, filename);
      } else {
        const { blob, filename } = await api.fileDownload(file.id, file.revision);
        saveBlob(blob, filename);
      }
      onClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to download the file");
    } finally {
      setBusy(false);
    }
  }

  // Portalled to <body>: the dialog is rendered from inside the Details panel, which has a `transform`
  // and `backdrop-filter` (a containing block for position:fixed) and `overflow:auto` — nested, the
  // fixed backdrop would be clamped to the panel and clipped (the RowMenu T87 trap). On <body> it has
  // no clipping/transformed ancestor and covers the real viewport.
  return createPortal(
    <div
      className="download-dialog-backdrop"
      data-testid="download-dialog"
      data-portal="download-dialog"
      role="dialog"
      aria-modal="true"
      aria-label="Download PDF"
    >
      <div className="download-dialog card">
        <div className="card-head">
          <h2>Download PDF</h2>
        </div>

        <label className="download-choice">
          <input
            type="radio"
            name="download-choice"
            data-testid="download-choice-with"
            checked={choice === "with"}
            disabled={!hasLayers || busy}
            onChange={() => setChoice("with")}
          />
          <span>With annotations</span>
        </label>

        {hasLayers ? (
          <ul className="list download-layer-list" data-testid="download-layer-list">
            {fileLayers.map((l) => (
              <li key={l.id} className="download-layer-row" data-testid="download-layer" data-layer={l.id}>
                <label>
                  <input
                    type="checkbox"
                    data-testid="download-layer-toggle"
                    checked={l.mandatory ? true : !!checked[l.id]}
                    disabled={l.mandatory || choice !== "with" || busy}
                    onChange={() => toggle(l.id)}
                  />
                  <span className="download-layer-name">{l.name}</span>
                </label>
                {l.mandatory && (
                  <span className="muted download-always" data-testid="download-always">
                    always printed
                  </span>
                )}
              </li>
            ))}
          </ul>
        ) : (
          <p className="muted download-no-annotations" data-testid="download-no-annotations">
            This file has no annotations.
          </p>
        )}

        <label className="download-choice">
          <input
            type="radio"
            name="download-choice"
            data-testid="download-choice-without"
            checked={choice === "without"}
            disabled={busy}
            onChange={() => setChoice("without")}
          />
          <span>Without annotations</span>
        </label>
        <p className="muted download-without-note" data-testid="download-without-note">
          The file as uploaded: no annotations, no crop.
        </p>

        <ErrorBanner message={error} />

        <div className="download-dialog-actions">
          <button type="button" data-testid="download-dialog-cancel" disabled={busy} onClick={onClose}>
            Cancel
          </button>
          <button
            type="button"
            className="primary"
            data-testid="download-dialog-confirm"
            disabled={busy}
            aria-busy={busy}
            onClick={() => void run()}
          >
            {busy ? "Preparing PDF…" : "Download"}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
