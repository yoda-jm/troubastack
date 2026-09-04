package app

// DefaultFile is the ONE shared rule (T138 ⟨R1⟩) for the file a member reads on stage when they have
// chosen nothing: the lowest-DisplayOrder PDF, ties broken by filename (ascending). Non-PDF files are not
// bakeable (the baker rasterizes with poppler, which takes PDFs, not images), so an image-only or
// audio-only song has NO default — Studio's viewer may still open those files, but they are never the
// file the stage takes. This is the single definition behind both the baker's pick and Studio's my-files
// default; it is pinned across lanes by docs/contracts/default-file.vectors.json (mirrored into
// web/studio and diffed in CI). Do not re-describe "default" anywhere else.
func DefaultFile(files []SongFile) (SongFile, bool) {
	var best SongFile
	found := false
	for _, f := range files {
		if f.ContentType != "application/pdf" {
			continue
		}
		if !found ||
			f.DisplayOrder < best.DisplayOrder ||
			(f.DisplayOrder == best.DisplayOrder && f.Filename < best.Filename) {
			best, found = f, true
		}
	}
	return best, found
}
