package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
)

// AppsAPI serves the native app binaries embedded in the server image (OPS02): a
// self-hosting band installs the app straight from its own server — no store, no
// account (the audience is exactly pre-account members). Binaries live in a runtime
// directory (TROUBA_APPS_DIR, wired by the Docker image); an ABSENT/empty dir ⇒ an
// empty manifest and 404 downloads, so dev runs (`make demo`, no dir) are unaffected
// and the UI simply hides the "Get the app" card.
//
// Only a fixed allow-list of filenames is ever served (no path from the request
// touches the filesystem), so there is no traversal surface.
type AppsAPI struct {
	dir     string // filesystem dir holding app binaries ("" = feature off)
	version string // build version, stamped into the manifest + download filename
}

func NewAppsAPI(dir, version string) *AppsAPI {
	return &AppsAPI{dir: dir, version: version}
}

// knownApp is one servable platform binary: its on-disk filename, download MIME,
// and the base name used for the versioned download filename.
type knownApp struct {
	platform string
	file     string // exact filename in the apps dir
	mime     string
	base     string // download filename base → "<base>-<version>.<ext>"
	ext      string
}

// knownApps is the allow-list. iOS joins when that artifact exists; the UI hides
// whatever the manifest lacks.
var knownApps = []knownApp{
	{platform: "android", file: "troubastage.apk", mime: "application/vnd.android.package-archive", base: "troubastage", ext: "apk"},
}

func (a *AppsAPI) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/apps", a.list)
	mux.HandleFunc("GET /apps/{file}", a.download)
}

// appEntry is the manifest projection for one available binary.
type appEntry struct {
	Platform string `json:"platform"`
	Version  string `json:"version"`
	Size     int64  `json:"size"`
	Path     string `json:"path"`     // the download URL path
	Filename string `json:"filename"` // the versioned download filename
}

// manifest lists the binaries that actually exist in the apps dir right now.
func (a *AppsAPI) manifest() []appEntry {
	out := []appEntry{}
	if a.dir == "" {
		return out
	}
	for _, k := range knownApps {
		fi, err := os.Stat(filepath.Join(a.dir, k.file))
		if err != nil || fi.IsDir() {
			continue
		}
		out = append(out, appEntry{
			Platform: k.platform,
			Version:  a.version,
			Size:     fi.Size(),
			Path:     "/apps/" + k.file,
			Filename: k.base + "-" + a.version + "." + k.ext,
		})
	}
	return out
}

// list is the tiny unauthenticated manifest (like /api/version): what installable
// apps this server carries. Empty array when none — never an error.
func (a *AppsAPI) list(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"apps": a.manifest()})
}

// download streams one allow-listed binary with its platform MIME and a versioned
// attachment filename. Unauthenticated (the APK is not a secret). Unknown name or
// absent file ⇒ 404.
func (a *AppsAPI) download(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	var known *knownApp
	for i := range knownApps {
		if knownApps[i].file == name {
			known = &knownApps[i]
			break
		}
	}
	if known == nil || a.dir == "" {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(a.dir, known.file))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", known.mime)
	w.Header().Set("Content-Disposition", contentDisposition("attachment", known.base+"-"+a.version+"."+known.ext, "download."+known.ext))
	http.ServeContent(w, r, known.file, info.ModTime(), f)
}
