package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
)

// This file makes the demo songs carry MEANINGFUL annotation layers + objects so
// the view-only Studio viewer shows real, layered content. It drives the bulk
// import API (POST …/annotations/import) which is idempotent by layer id / object
// uuid — so every id below is derived deterministically from the songID, making
// re-seeding a no-op rather than a duplicator.
//
// Coordinates are 0..1 page-relative and are computed against the EXACT layout of
// the generated placeholder PDFs (see pdf.go). For the one PDF we fetch (Bach,
// unknown layout) we use generic-but-plausible spots near the top system + margins.

// ---- wire contract (mirror of internal/httpapi/annotations.go) ----

type wirePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type wireStyle struct {
	Color    string  `json:"color"`
	Opacity  float64 `json:"opacity"`
	Width    float64 `json:"width"`
	FontSize float64 `json:"fontSize"`
}

type wireLayer struct {
	ID        string `json:"id"`
	FileID    string `json:"fileId"`
	Name      string `json:"name"`
	OwnerID   string `json:"ownerId"`
	Zone      string `json:"zone"`
	Order     int    `json:"order"`
	Access    string `json:"access"`
	Mandatory bool   `json:"mandatory"`
	RoleTag   string `json:"roleTag"`
}

type wireObject struct {
	UUID    string      `json:"uuid"`
	LayerID string      `json:"layerId"`
	Type    string      `json:"type"`
	Points  []wirePoint `json:"points"`
	Page    int         `json:"page"`
	Text    string      `json:"text"`
	Style   wireStyle   `json:"style"`
}

type annotationsImport struct {
	Layers  []wireLayer  `json:"layers"`
	Objects []wireObject `json:"objects"`
}

// ---- generated-PDF layout, in 0..1 page-relative coords (derived from pdf.go) ----
//
// pdf.go builds A4 (210mm × 297mm). left=20mm, right=190mm. Title sits at y=25mm
// (height 12mm), subtitle at y=38mm, the page marker at x≈150mm/y=25mm. Staff
// "systems" are groups of 5 lines (2.2mm apart, ~8.8mm tall) starting at y=60mm,
// each followed by a 12mm gap → systems repeat every ~20.8mm.
const (
	pageWmm = 210.0
	pageHmm = 297.0

	pdfLeftX  = 20.0 / pageWmm  // ≈ 0.095 — left margin / staff start
	pdfRightX = 190.0 / pageWmm // ≈ 0.905 — staff end / right margin
	pdfMargX  = 12.0 / pageWmm  // ≈ 0.057 — a touch left of the staff, for margin labels
)

// systemTopY returns the 0..1 y of the top line of staff system i (0-based) of a
// generated page (y=60mm start, 20.8mm pitch).
func systemTopY(i int) float64 { return (60.0 + float64(i)*20.8) / pageHmm }

// systemBotY returns the 0..1 y of the bottom (5th) line of system i.
func systemBotY(i int) float64 { return (60.0 + float64(i)*20.8 + 4*2.2) / pageHmm }

var (
	titleTopY = 25.0 / pageHmm // ≈ 0.084
	titleBotY = 37.0 / pageHmm // ≈ 0.125
)

// ---- id derivation (stable per song+layer / song+object) ----

func layerID(songID, key string) string {
	return "L-" + shortHash(songID+"|layer|"+key)
}

func objectID(songID, key string) string {
	return "O-" + shortHash(songID+"|object|"+key)
}

func shortHash(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// ---- builders ----

// builderCtx carries everything the per-shape helpers need.
type builderCtx struct {
	songID string
	fileID string
	im     *annotationsImport
}

func (b *builderCtx) layer(l wireLayer) string {
	l.FileID = b.fileID
	b.im.Layers = append(b.im.Layers, l)
	return l.ID
}

func (b *builderCtx) text(layerID, key string, page int, x, y float64, body string, st wireStyle) {
	b.im.Objects = append(b.im.Objects, wireObject{
		UUID: objectID(b.songID, key), LayerID: layerID, Type: "text",
		Points: []wirePoint{{X: x, Y: y}}, Page: page, Text: body, Style: st,
	})
}

func (b *builderCtx) rect(layerID, key string, page int, x0, y0, x1, y1 float64, st wireStyle) {
	b.shape("rect", layerID, key, page, []wirePoint{{X: x0, Y: y0}, {X: x1, Y: y1}}, st)
}

func (b *builderCtx) highlight(layerID, key string, page int, x0, y0, x1, y1 float64, st wireStyle) {
	b.shape("highlight", layerID, key, page, []wirePoint{{X: x0, Y: y0}, {X: x1, Y: y1}}, st)
}

func (b *builderCtx) line(layerID, key string, page int, x0, y0, x1, y1 float64, st wireStyle) {
	b.shape("line", layerID, key, page, []wirePoint{{X: x0, Y: y0}, {X: x1, Y: y1}}, st)
}

func (b *builderCtx) freehand(layerID, key string, page int, pts []wirePoint, st wireStyle) {
	b.shape("freehand", layerID, key, page, pts, st)
}

func (b *builderCtx) shape(typ, layerID, key string, page int, pts []wirePoint, st wireStyle) {
	b.im.Objects = append(b.im.Objects, wireObject{
		UUID: objectID(b.songID, key), LayerID: layerID, Type: typ,
		Points: pts, Page: page, Style: st,
	})
}

// palette — distinct, visually rich per zone.
var (
	colorConductor = "#C62828" // red
	colorShared    = "#F9A825" // amber
	colorPersonal  = "#1565C0" // blue
	colorPersonal2 = "#2E7D32" // green (second personal flavour)
)

// vertBracket draws a left-margin square bracket "[" spanning a staff system, as a
// freehand path. Used by the conductor layer to mark a passage.
func vertBracket(x, yTop, yBot float64) []wirePoint {
	tick := 0.012
	return []wirePoint{
		{X: x + tick, Y: yTop}, {X: x, Y: yTop},
		{X: x, Y: yBot}, {X: x + tick, Y: yBot},
	}
}

// hiSwipe returns a slightly wavy horizontal freehand path from (x0,y) to (x1,y) — a
// highlighter stroke over a word/chord (draw it wide + low-opacity to read as a marker).
func hiSwipe(x0, x1, y float64) []wirePoint {
	d := x1 - x0
	return []wirePoint{
		{X: x0, Y: y}, {X: x0 + d*0.35, Y: y - 0.0015},
		{X: x0 + d*0.7, Y: y + 0.0015}, {X: x1, Y: y},
	}
}

// warnTriangle returns a hand-drawn warning triangle (apex up) centred on (x,y).
func warnTriangle(x, y float64) []wirePoint {
	return []wirePoint{
		{X: x, Y: y - 0.015}, {X: x + 0.017, Y: y + 0.012},
		{X: x - 0.017, Y: y + 0.012}, {X: x, Y: y - 0.015},
	}
}

// warnSign draws a recognizable ⚠ centred on (x,y): the triangle above plus an exclamation
// (a stem + a dot) inside it. Appends 3 freehand objects to the builder in `st`.
func (b *builderCtx) warnSign(layerID, keyBase string, x, y float64, st wireStyle) {
	b.freehand(layerID, keyBase+"-tri", 0, warnTriangle(x, y), st)
	b.freehand(layerID, keyBase+"-stem", 0, []wirePoint{{X: x, Y: y - 0.006}, {X: x, Y: y + 0.004}}, st)
	b.freehand(layerID, keyBase+"-dot", 0, []wirePoint{{X: x, Y: y + 0.008}, {X: x + 0.001, Y: y + 0.0085}}, st)
}

// downBow returns a hand-drawn down-bow mark (⊓, a bracket opening downward) centred on (x,y).
func downBow(x, y float64) []wirePoint {
	w, h := 0.008, 0.009
	return []wirePoint{{X: x - w, Y: y}, {X: x - w, Y: y - h}, {X: x + w, Y: y - h}, {X: x + w, Y: y}}
}

// buildCanonAnnotations places a clean 3-layer showcase on the Canon in D VIOLIN I part (a
// LilyPond A4: the staff sits near the top ~y 0.11–0.15 with whitespace below). Conductor cue
// rings the final bar with the label in the clear space below; a "Dynamics" marking and a
// player's "Bowing" (down-bows + dolce) sit below the staff — nothing overlaps the engraving.
func buildCanonAnnotations(songID, fileID string, userID map[string]string, conductorID string) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}

	// Conductor cues (red, mandatory) — ring the final bar, "rit. — watch me" from below.
	cond := b.layer(wireLayer{
		ID: layerID(songID, "cues"), Name: "Conductor cues",
		OwnerID: conductorID, Zone: "conductor", Order: 0, Access: "ro", Mandatory: true, RoleTag: "conductor",
	})
	condS := wireStyle{Color: colorConductor, Opacity: 1, Width: 0.0035}
	b.shape("ellipse", cond, "cn-ring", 0, []wirePoint{{X: 0.78, Y: 0.106}, {X: 0.865, Y: 0.152}}, condS)
	b.line(cond, "cn-pt", 0, 0.66, 0.205, 0.82, 0.152, condS)
	b.text(cond, "cn-rit", 0, 0.595, 0.212, "rit. — watch me", wireStyle{Color: colorConductor, Opacity: 1, FontSize: 0.014})

	// Dynamics (shared, amber) — a marking under the opening bars.
	shared := b.layer(wireLayer{
		ID: layerID(songID, "shared"), Name: "Dynamics",
		OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "rw",
	})
	b.text(shared, "cn-dyn", 0, 0.18, 0.198, "mf espr.", wireStyle{Color: colorShared, Opacity: 1, FontSize: 0.015})

	// Bowing (personal, owned by the cellist) — down-bow marks over the first two notes + dolce.
	bow := b.layer(wireLayer{
		ID: layerID(songID, "bowing"), Name: "Bowing",
		OwnerID: userID["cory"], Zone: "personal", Order: 0, Access: "rw", RoleTag: "strings",
	})
	bowS := wireStyle{Color: colorPersonal2, Opacity: 1, Width: 0.004}
	b.freehand(bow, "cn-db1", 0, downBow(0.205, 0.093), bowS)
	b.freehand(bow, "cn-db2", 0, downBow(0.30, 0.093), bowS)
	b.text(bow, "cn-dolce", 0, 0.18, 0.238, "dolce", wireStyle{Color: colorPersonal2, Opacity: 1, FontSize: 0.014})
	return *im
}

// buildEineKleineAnnotations places a CLEAN, musically-correct 3-layer showcase on Eine
// kleine Nachtmusik (Mozart, K.525, 1st mvt = Allegro, sonata form). It replaces the
// generic buildSongAnnotations path, whose pop-song "Verse/Chorus/Bridge" labels and
// title-box overlap were wrong for a classical piece. Offline the seed renders the
// generated staff placeholder (the shipped case) — coords are staff-relative there; the
// rare fetched-PDF case gets a minimal top-area set. Nothing overlaps the title.
func buildEineKleineAnnotations(songID, fileID string, userID map[string]string, conductorID string, generated bool) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}

	cond := b.layer(wireLayer{
		ID: layerID(songID, "conductor"), Name: "Conductor cues",
		OwnerID: conductorID, Zone: "conductor", Order: 0, Access: "ro", Mandatory: true, RoleTag: "conductor",
	})
	shared := b.layer(wireLayer{
		ID: layerID(songID, "shared"), Name: "Section markings",
		OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "rw",
	})
	mine := b.layer(wireLayer{
		ID: layerID(songID, "personal-dyn"), Name: "Dynamics/bowing",
		OwnerID: userID["flora"], Zone: "personal", Order: 0, Access: "rw", RoleTag: "strings",
	})
	condText := wireStyle{Color: colorConductor, Opacity: 1, FontSize: 0.016}
	condStroke := wireStyle{Color: colorConductor, Opacity: 1, Width: 0.0035}
	sharedText := wireStyle{Color: colorShared, Opacity: 1, FontSize: 0.016}
	sharedHi := wireStyle{Color: colorShared, Opacity: 0.35}
	greenText := wireStyle{Color: colorPersonal2, Opacity: 1, FontSize: 0.015}

	if generated {
		// Tempo above the first system (clear of the title/subtitle); sonata sections in the
		// left margin, one per system; a conductor cue rings a spot on system 1 with the label
		// out in the right margin; the ending rit. + a player dynamic sit on their own systems.
		b.text(cond, "ek-tempo", 0, pdfMargX, systemTopY(0)-0.018, "Allegro", condText)
		b.highlight(shared, "ek-hi", 0, 0.35, systemTopY(0)-0.004, 0.66, systemBotY(0)+0.004, sharedHi)
		b.text(shared, "ek-exp", 0, pdfMargX, systemTopY(0)+0.004, "Exposition", sharedText)
		b.shape("ellipse", cond, "ek-ring", 0, []wirePoint{{X: 0.30, Y: systemTopY(1) - 0.008}, {X: 0.40, Y: systemBotY(1) + 0.008}}, condStroke)
		b.line(cond, "ek-pt", 0, 0.72, (systemTopY(1)+systemBotY(1))/2, 0.405, (systemTopY(1)+systemBotY(1))/2, condStroke)
		b.text(cond, "ek-cue", 0, 0.73, (systemTopY(1)+systemBotY(1))/2-0.007, "watch the upbeat", condText)
		b.text(mine, "ek-dolce", 0, pdfMargX, systemTopY(2)+0.004, "dolce", greenText)
		b.text(shared, "ek-dev", 0, pdfMargX, systemTopY(3)+0.004, "Development", sharedText)
		b.text(cond, "ek-rit", 0, pdfMargX, systemTopY(4)-0.014, "molto rit. al fine", condText)
	} else {
		// Fetched real PDF (unknown layout) — keep a minimal set in the clear top-left area.
		b.text(cond, "ek-tempo", 0, 0.06, 0.165, "Allegro — crisp attacks", condText)
		b.text(shared, "ek-exp", 0, 0.06, 0.205, "Exposition", sharedText)
		b.text(shared, "ek-dev", 0, 0.06, 0.40, "Development", sharedText)
		b.text(mine, "ek-dolce", 0, 0.06, 0.30, "dolce", greenText)
	}
	return *im
}

// buildSongAnnotations assembles ~3 layers (conductor / shared / personal) of
// meaningful objects for one song, shaped by song title + group kind.
//
// userID maps username → user id (for layer ownerId), conductorID is the group's
// conductor-role user id (owns the conductor-zone cues; write access to that zone is
// governed by the conductor ROLE, not ownership — #3), generated says whether the PDF
// is a generated placeholder (precise layout) vs a fetched real PDF (generic placement).
func buildSongAnnotations(songID, fileID, title, groupKind string, userID map[string]string, conductorID string, generated bool, pages int) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}

	// ---- conductor zone: "Conductor cues" (mandatory, red) — owned/authored by the
	// promoted conductor so the conductor role (not the band admin) can edit it (#3). ----
	cond := b.layer(wireLayer{
		ID: layerID(songID, "conductor"), Name: "Conductor cues",
		OwnerID: conductorID, Zone: "conductor", Order: 0, Access: "ro", Mandatory: true,
		RoleTag: "conductor",
	})
	condText := wireStyle{Color: colorConductor, Opacity: 1, FontSize: 0.022}
	condStroke := wireStyle{Color: colorConductor, Opacity: 0.95, Width: 0.006}

	// ---- shared zone: "Section markings" (amber, rw) ----
	shared := b.layer(wireLayer{
		ID: layerID(songID, "shared"), Name: "Section markings",
		OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "rw",
	})
	sharedText := wireStyle{Color: colorShared, Opacity: 1, FontSize: 0.020}
	sharedHi := wireStyle{Color: colorShared, Opacity: 0.35}

	if generated {
		// Conductor: bracket + two cues anchored on real staff systems.
		b.freehand(cond, "cond-bracket", 0, vertBracket(pdfMargX, systemTopY(0), systemBotY(0)), condStroke)
		b.text(cond, "cond-cue1", 0, pdfLeftX, titleTopY-0.018, "Watch me — pickup", condText)
		b.rect(cond, "cond-box", 0, pdfLeftX-0.006, titleTopY-0.006, pdfRightX*0.55, titleBotY+0.006, condStroke)
		b.text(cond, "cond-cue2", 0, pdfMargX, systemTopY(3)-0.012, "rit.", condText)

		// Shared: highlight a bar on systems 1 & 2, with Verse / Chorus labels.
		b.highlight(shared, "sh-hi-verse", 0, 0.30, systemTopY(1)-0.004, 0.55, systemBotY(1)+0.004, sharedHi)
		b.text(shared, "sh-verse", 0, pdfMargX, systemTopY(1)+0.002, "Verse 1", sharedText)
		b.highlight(shared, "sh-hi-chorus", 0, 0.30, systemTopY(2)-0.004, 0.70, systemBotY(2)+0.004, sharedHi)
		b.text(shared, "sh-chorus", 0, pdfMargX, systemTopY(2)+0.002, "Chorus", sharedText)
		b.text(shared, "sh-bridge", 0, pdfMargX, systemTopY(4)+0.002, "Bridge", sharedText)

		// Page 1 (second page) for multi-page songs: an outro section + a D.C. cue.
		if pages >= 2 {
			b.highlight(shared, "sh-hi-outro", 1, 0.30, systemTopY(0)-0.004, 0.75, systemBotY(0)+0.004, sharedHi)
			b.text(shared, "sh-outro", 1, pdfMargX, systemTopY(0)+0.002, "Outro", sharedText)
			b.text(cond, "cond-dc", 1, pdfMargX, systemTopY(2)-0.012, "D.C. al Fine", condText)
		}
	} else {
		// Fetched PDF (Bach) — unknown layout: keep to top-left + top system.
		b.text(cond, "cond-tempo", 0, 0.06, 0.10, "Adagio — molto espr.", condText)
		b.rect(cond, "cond-box", 0, 0.05, 0.09, 0.40, 0.135, condStroke)
		b.text(cond, "cond-cue", 0, 0.05, 0.27, "watch the phrasing", condText)
		b.highlight(shared, "sh-hi-open", 0, 0.12, 0.20, 0.55, 0.26, sharedHi)
		b.text(shared, "sh-theme", 0, 0.05, 0.205, "Theme", sharedText)
		b.text(shared, "sh-section", 0, 0.05, 0.40, "B section", sharedText)
		if pages >= 2 {
			b.text(shared, "sh-recap", 1, 0.05, 0.16, "Recap", sharedText)
			b.text(cond, "cond-dc", 1, 0.05, 0.30, "molto rit. al fine", condText)
		}
	}

	// ---- personal zone: instrument-specific, owner = a fitting member ----
	addPersonal(b, title, groupKind, userID, generated)

	return *im
}

// buildOpenRoadAnnotations places meaningful annotations on "The Open Road" LEAD
// SHEET (a local chart PDF with a known chords-over-lyrics layout — page 0), rather
// than the generated-staff or fetched layouts buildSongAnnotations handles. Coords
// match docs/demo-charts/open-road-leadsheet.pdf (page-relative [0,1]); ink infers
// fill+multiply for `highlight` and stroke for `ellipse`, so no extra style flags
// are needed. Three layers: Form (mandatory), Conductor cues (mandatory, conductor
// role), My notes (personal, owned by the singer/admin).
func buildOpenRoadAnnotations(songID, fileID string, userID map[string]string, conductorID string) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}

	// Form / sections (shared, mandatory, amber) — a left-margin BRACKET on the chorus (clearly
	// hand-drawn markup, not a flat block fill) + a label in the clear right margin.
	form := b.layer(wireLayer{
		ID: layerID(songID, "form"), Name: "Form / sections",
		OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "ro", Mandatory: true,
	})
	b.freehand(form, "or-chorus-brk", 0, vertBracket(0.045, 0.378, 0.520), wireStyle{Color: colorShared, Opacity: 1, Width: 0.005})
	b.text(form, "or-chorus", 0, 0.70, 0.392, "everyone in!", wireStyle{Color: "#B45309", Opacity: 1, FontSize: 0.016})

	// Conductor cues (conductor role, mandatory, red) — circle the LAST chorus's landing chord
	// (G) and point "rit. — watch me" at it from the clear space to its right.
	cond := b.layer(wireLayer{
		ID: layerID(songID, "cues"), Name: "Conductor cues",
		OwnerID: conductorID, Zone: "conductor", Order: 0, Access: "ro", Mandatory: true, RoleTag: "conductor",
	})
	condStroke := wireStyle{Color: colorConductor, Opacity: 1, Width: 0.0035}
	b.shape("ellipse", cond, "or-turn", 0, []wirePoint{{X: 0.255, Y: 0.499}, {X: 0.32, Y: 0.520}}, condStroke)
	b.line(cond, "or-point", 0, 0.395, 0.507, 0.325, 0.510, condStroke)
	b.text(cond, "or-rit", 0, 0.40, 0.498, "rit. — watch me", wireStyle{Color: colorConductor, Opacity: 1, FontSize: 0.015})

	// My notes (personal, owned by marie) — the SEMI-TRANSPARENT FREEHAND highlighter: a GREEN
	// marker over the printed "Capo 2" with a ⚠ warning sign, and a YELLOW marker over the hook
	// word "drive". (Green marks it as a personal reminder, distinct from the amber sections.)
	mine := b.layer(wireLayer{
		ID: layerID(songID, "mine"), Name: "My notes",
		OwnerID: userID["marie"], Zone: "personal", Order: 0, Access: "rw",
	})
	b.freehand(mine, "or-capo-hi", 0, hiSwipe(0.432, 0.492, 0.128), wireStyle{Color: colorPersonal2, Opacity: 0.4, Width: 0.02})
	b.warnSign(mine, "or-capo-warn", 0.525, 0.126, wireStyle{Color: "#EA580C", Opacity: 1, Width: 0.004})
	b.freehand(mine, "or-hook-hi", 0, hiSwipe(0.105, 0.225, 0.408), wireStyle{Color: "#FACC15", Opacity: 0.4, Width: 0.02})

	return *im
}

// buildBandChartAnnotations places a light 3-layer showcase (conductor / shared / personal)
// on a committed BAND chart — the House of the Rising Sun guitar tab or the Amazing Grace
// lead sheet, both mkcharts A4 with a title block above ~0.17 and content below it. Not the
// pixel-tuned Open Road showcase; enough to demo the layer model (mandatory conductor cues,
// shared section highlight, a personal note) over a real chart with sensible placement.
func buildBandChartAnnotations(songID, fileID, title string, userID map[string]string, conductorID string) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}

	// Three layers, placed CLEANLY per chart: highlights/circles land ON content; every text
	// label sits in the clear RIGHT margin (lyrics/tab end well before it) with a connector, so
	// nothing overlaps the chart's own text.
	shared := b.layer(wireLayer{
		ID: layerID(songID, "shared"), Name: "Section markings",
		OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "rw",
	})
	cond := b.layer(wireLayer{
		ID: layerID(songID, "conductor"), Name: "Conductor cues",
		OwnerID: conductorID, Zone: "conductor", Order: 0, Access: "ro", Mandatory: true, RoleTag: "conductor",
	})
	mine := b.layer(wireLayer{
		ID: layerID(songID, "mine"), Name: "My notes",
		OwnerID: userID["marie"], Zone: "personal", Order: 0, Access: "rw",
	})
	amberT := wireStyle{Color: "#B45309", Opacity: 1, FontSize: 0.015}
	amberBrk := wireStyle{Color: colorShared, Opacity: 1, Width: 0.005}
	condS := wireStyle{Color: colorConductor, Opacity: 1, Width: 0.0035}
	condT := wireStyle{Color: colorConductor, Opacity: 1, FontSize: 0.014}
	green := wireStyle{Color: colorPersonal2, Opacity: 0.4, Width: 0.02}

	switch title {
	case "House of the Rising Sun":
		// Guitar tab. Green marker on the opening Am chord; a left bracket on the arpeggio with
		// "let it ring" in the right margin; ring the E turnaround, "watch me" pointing in.
		b.freehand(mine, "bc-hi", 0, hiSwipe(0.155, 0.215, 0.212), green)
		b.freehand(shared, "bc-brk", 0, vertBracket(0.05, 0.205, 0.325), amberBrk)
		b.text(shared, "bc-lab", 0, 0.74, 0.255, "let it ring", amberT)
		b.shape("ellipse", cond, "bc-ring", 0, []wirePoint{{X: 0.555, Y: 0.197}, {X: 0.625, Y: 0.220}}, condS)
		b.line(cond, "bc-pt", 0, 0.70, 0.192, 0.625, 0.206, condS)
		b.text(cond, "bc-cue", 0, 0.705, 0.184, "watch me", condT)
	case "Amazing Grace":
		// Lead sheet. Green marker on "grace"; a left bracket on verse 1; ring the closing G of
		// verse 1, "rit. on 'I see'" pointing in from the right.
		b.freehand(mine, "bc-hi", 0, hiSwipe(0.113, 0.185, 0.222), green)
		// Shared marking: a left-margin bracket grouping verse 1, with a small label at its
		// TOP (clear space above the first chord row) — self-explanatory, no floating text.
		b.freehand(shared, "bc-brk", 0, vertBracket(0.05, 0.19, 0.322), amberBrk)
		b.text(shared, "bc-lab", 0, 0.07, 0.183, "v.1 — softly", amberT)
		b.shape("ellipse", cond, "bc-ring", 0, []wirePoint{{X: 0.30, Y: 0.310}, {X: 0.40, Y: 0.332}}, condS)
		b.line(cond, "bc-pt", 0, 0.54, 0.320, 0.40, 0.322, condS)
		b.text(cond, "bc-cue", 0, 0.55, 0.313, "rit. on 'I see'", condT)
	}
	return *im
}

// addPersonal adds an instrument-appropriate personal layer for the song's group.
func addPersonal(b *builderCtx, title, groupKind string, userID map[string]string, generated bool) {
	if groupKind == "Orchestra" {
		// Bowing / breath marks — owned by the flute player (flora) if present.
		owner := userID["flora"]
		pl := b.layer(wireLayer{
			ID: layerID(b.songID, "personal-bowing"), Name: "Bowing/Breath marks",
			OwnerID: owner, Zone: "personal", Order: 0, Access: "rw", RoleTag: "flute",
		})
		penG := wireStyle{Color: colorPersonal2, Opacity: 1, FontSize: 0.018}
		strokeG := wireStyle{Color: colorPersonal2, Opacity: 0.9, Width: 0.004}
		if generated {
			// Breath marks (apostrophes) above two systems + a down-bow tick.
			b.text(pl, "pers-breath1", 0, 0.42, systemTopY(1)-0.010, "'", penG)
			b.text(pl, "pers-breath2", 0, 0.66, systemTopY(2)-0.010, "'", penG)
			b.line(pl, "pers-downbow", 0, 0.34, systemTopY(1)-0.012, 0.38, systemTopY(1)-0.012, strokeG)
			b.text(pl, "pers-dolce", 0, pdfMargX, systemTopY(3)+0.002, "dolce", penG)
		} else {
			b.text(pl, "pers-breath1", 0, 0.40, 0.18, "'", penG)
			b.text(pl, "pers-breath2", 0, 0.62, 0.18, "'", penG)
			b.line(pl, "pers-downbow", 0, 0.30, 0.175, 0.34, 0.175, strokeG)
			b.text(pl, "pers-dolce", 0, 0.05, 0.34, "dolce", penG)
		}
		return
	}

	// Band: chord names — owned by the guitarist (leo) if present.
	owner := userID["leo"]
	pl := b.layer(wireLayer{
		ID: layerID(b.songID, "personal-chords"), Name: "Chords",
		OwnerID: owner, Zone: "personal", Order: 0, Access: "rw", RoleTag: "guitar",
	})
	penB := wireStyle{Color: colorPersonal, Opacity: 1, FontSize: 0.020}

	// A plausible default progression (band songs with committed charts use their own
	// dedicated builders — buildOpenRoadAnnotations / buildBandChartAnnotations — so this
	// generic staff-relative path only backs any future generated-placeholder band song).
	chords := []string{"Em", "G", "D", "A"}
	capo := ""

	if generated {
		if capo != "" {
			b.text(pl, "pers-capo", 0, pdfMargX, titleTopY+0.004, capo, penB)
		}
		// Chord names above system 0, spread across the bar.
		startX := 0.18
		span := pdfRightX - startX
		for i, ch := range chords {
			x := startX + span*float64(i)/float64(len(chords))
			b.text(pl, fmt.Sprintf("pers-chord-%d", i), 0, x, systemTopY(0)-0.012, ch, penB)
		}
	} else {
		if capo != "" {
			b.text(pl, "pers-capo", 0, 0.05, 0.15, capo, penB)
		}
		startX := 0.20
		for i, ch := range chords {
			x := startX + 0.14*float64(i)
			if x > 0.9 {
				break
			}
			b.text(pl, fmt.Sprintf("pers-chord-%d", i), 0, x, 0.165, ch, penB)
		}
	}
}

// ---- B11: per-part annotations for House of the Rising Sun's Drums file ----
//
// The first PDF (the guitar tab) carries the full section-form annotations
// (buildSongAnnotations). This gives the DRUMS part its own, role-appropriate notes so
// switching file tabs visibly demonstrates per-file scoping (T40): each part shows
// different ink over a different PDF. The committed drum-groove PDF (mkcharts) has its
// groove grid near the top, so coords stay in the upper area (generic, not staff-relative).
// Layer/object keys are file-distinct so ids never collide with the tab's (idempotent by id).

// buildDrumPartAnnotations: a shared "Drummer's notes" layer (green) on the Drums part —
// a feel note over the groove grid and a soft-dynamics reminder.
func buildDrumPartAnnotations(songID, fileID string) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}

	notes := b.layer(wireLayer{
		ID: layerID(songID, "drum-notes"), Name: "Drummer's notes",
		OwnerID: "_shared_", Zone: "shared", Order: 1, Access: "rw", RoleTag: "drums",
	})
	txt := wireStyle{Color: colorPersonal2, Opacity: 1, FontSize: 0.015}

	// The groove grid: count row ~0.21, hi-hat ~0.235, snare ~0.262, kick ~0.285; columns 1–6
	// span x ~0.19–0.44. Text labels sit in the clear space to the RIGHT of the grid (x > 0.50).
	// A semi-transparent freehand highlighter marks the SNARE hit on beat 4 (the backbeat).
	b.freehand(notes, "drum-snare-hi", 0, hiSwipe(0.315, 0.365, 0.262), wireStyle{Color: "#FACC15", Opacity: 0.4, Width: 0.02})
	b.text(notes, "drum-snare", 0, 0.50, 0.258, "snare — lay back", txt)
	b.text(notes, "drum-soft", 0, 0.50, 0.232, "keep it soft under the vocal", txt)
	return *im
}
