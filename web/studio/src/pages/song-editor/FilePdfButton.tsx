import { useState } from "react";
import { api, ApiError, type AnnotationLayer } from "../../api";
import { ErrorBanner } from "../../components/ErrorBanner";
import type { LayerVisibility } from "./helpers";

/** The ids sent as `layers` for a file print: the layers of THIS file that are visible on screen right now.
 *  Song-level layers (no fileId) are not drawn over a selected file on screen, so they are not sent either;
 *  the server adds mandatory layers on its own (they can't be hidden anywhere). */
export function visibleLayerIdsForFile(
  layers: readonly AnnotationLayer[],
  visible: LayerVisibility,
  fileId: string,
): string[] {
  return layers.filter((l) => l.fileId === fileId && !!visible[l.id]).map((l) => l.id);
}

/** Hands a Blob to the browser as a download named `filename`. */
function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

/** FILEPDF — "Download PDF" for the file being viewed, with the layers shown on screen drawn on it. Lives in
 *  the "This file" rail's Layers tab, beside the very toggles that decide what it prints. */
export function FilePdfButton({
  bandId,
  songId,
  fileId,
  layers,
  visible,
}: {
  bandId: string;
  songId: string;
  fileId: string;
  layers: readonly AnnotationLayer[];
  visible: LayerVisibility;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function download() {
    setBusy(true);
    setError(null);
    try {
      const { blob, filename } = await api.filePdf(bandId, songId, fileId, visibleLayerIdsForFile(layers, visible, fileId));
      saveBlob(blob, filename);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to make the PDF");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="file-pdf-action">
      <button
        type="button"
        className="secondary file-pdf-btn"
        data-testid="file-pdf-download"
        disabled={busy}
        aria-busy={busy}
        title="Download this file as a PDF, with the layers shown on screen"
        onClick={() => void download()}
      >
        {busy ? "Preparing PDF…" : "Download PDF"}
      </button>
      <ErrorBanner message={error} />
    </div>
  );
}
