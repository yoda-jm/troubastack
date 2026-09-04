// T138 ⟨R1⟩ — the ONE shared "default file" rule (see docs/contracts/default-file.vectors.json and Go
// app.DefaultFile): the file a member reads on stage when they have chosen nothing. Lowest-displayOrder
// PDF, ties broken by filename (ascending); a non-PDF is never the default (the baker rasterizes PDFs,
// not images — Studio's viewer may still OPEN an image, but that is browsing, not the bake selection).
// Do not re-describe "default" anywhere else in Studio; call this.

export interface DefaultFileCandidate {
  filename: string;
  contentType: string;
  displayOrder: number;
}

export function defaultFile<T extends DefaultFileCandidate>(files: readonly T[]): T | null {
  let best: T | null = null;
  for (const f of files) {
    if (f.contentType !== "application/pdf") continue;
    if (
      best === null ||
      f.displayOrder < best.displayOrder ||
      (f.displayOrder === best.displayOrder && f.filename < best.filename)
    ) {
      best = f;
    }
  }
  return best;
}
