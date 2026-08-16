package main

import (
	"crypto/sha1"
	"encoding/hex"
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
	Fill     *bool   `json:"fill,omitempty"`
	Stroke   *bool   `json:"stroke,omitempty"`
	Blend    string  `json:"blend,omitempty"`
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
	// placements is test-only (B13): the target box each anchor-bound object must cover/clear.
	// Not serialized — the import POST carries only layers + objects.
	placements []placement `json:"-"`
}

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

// builderCtx carries everything the per-shape helpers need. Anchor-bound helpers record their
// target boxes into im.placements (B13) for the containment test.
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

// The orchestra pieces are REAL, complete multi-page editions (Mutopia): a conductor's full
// score + separate parts. Per the orchestra workflow, the CONDUCTOR marks up the score (an
// interpretation / recommendation layer) and each PLAYER marks their own part (bowings, a
// dynamic). Real engravings are dense, so marks sit in the clear margins ABOVE the first
// system / BELOW the last staff (no ringing individual notes on a packed page).

// buildCanonAnnotations — the Canon VIOLIN I part (B13 marks 33–36), anchored to canon.* boxes.
func buildCanonAnnotations(songID, fileID string, userID map[string]string, conductorID string, an *anchorSet) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}
	cond := b.layer(wireLayer{ID: layerID(songID, "cues"), Name: "Conductor cues", OwnerID: conductorID, Zone: "conductor", Order: 0, Access: "ro", Mandatory: true, RoleTag: "conductor"})
	shared := b.layer(wireLayer{ID: layerID(songID, "shared"), Name: "Dynamics", OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "rw"})
	mine := b.layer(wireLayer{ID: layerID(songID, "bowing"), Name: "Bowing (Vln I)", OwnerID: userID["ivan"], Zone: "personal", Order: 0, Access: "rw", RoleTag: "strings"})
	// 33) Ivan: blue note stamp at the 2-bar rest.
	rest := an.key("canon.rest-2")
	b.iconAnchor(mine, "cn-ic-note", 0, rest.X0-0.026, rest.Y0-0.002, 0.018, "note", colorPersonal)
	// 34) Ivan: yellow highlighter over the first quarter-note entry.
	b.highlightAnchor(mine, "cn-hi-entry", an.key("canon.first-entry"), "#FACC15", 0.42)
	// 33/35/36) The three texts share ONE clear row in the (narrow) sys1–2 gap — stacking would
	// spill onto system 2 (the ink test catches that). Spread horizontally at small size.
	row := an.key("canon.margin-sys2").cy() + 0.004
	b.labelNear(mine, "cn-rest-lab", 0, 0.115, row, "count 2 bars — then sing out", colorPersonal, 0.011)
	b.labelNear(shared, "cn-cantabile", 0, 0.335, row, "mf, cantabile", "#B45309", 0.011)
	b.labelNear(cond, "cn-steady", 0, 0.455, row, "steady — the bass drives", colorConductor, 0.011)
	return *im
}

// buildEineKleineAnnotations — the Eine kleine VIOLIN I part (B13 marks 27–32), anchored to ek.* boxes.
func buildEineKleineAnnotations(songID, fileID string, userID map[string]string, conductorID string, an *anchorSet) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}
	cond := b.layer(wireLayer{ID: layerID(songID, "conductor"), Name: "Conductor cues", OwnerID: conductorID, Zone: "conductor", Order: 0, Access: "ro", Mandatory: true, RoleTag: "conductor"})
	shared := b.layer(wireLayer{ID: layerID(songID, "shared"), Name: "Dynamics", OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "rw"})
	mine := b.layer(wireLayer{ID: layerID(songID, "personal-bow"), Name: "Bowing (Vln I)", OwnerID: userID["ivan"], Zone: "personal", Order: 0, Access: "rw", RoleTag: "strings"})
	green := wireStyle{Color: colorPersonal2, Opacity: 1, Width: 0.004}
	// 27) Ivan: green highlighter over the opening motif (bars 1–2).
	b.highlightAnchor(mine, "ek-hi-motif", an.key("ek.opening-motif"), colorPersonal2, 0.40)
	// 28) Ivan: hand-drawn down-bow ⊓ + up-bow V above the first two attacks.
	b.freehand(mine, "ek-bow1", 0, downBowMark("ek-bow1", an.key("ek.bow-1")), green)
	b.freehand(mine, "ek-bow2", 0, upBowMark("ek-bow2", an.key("ek.bow-2")), green)
	// 29) Ivan: "détaché, off the string" in the clear band above system 1.
	m1 := an.key("ek.margin-sys1")
	b.labelNear(mine, "ek-detache", 0, m1.X0, m1.cy()+0.005, "détaché, off the string", colorPersonal2, 0.013)
	// 30) Conductor: hand-drawn ring around the trill (system 2) + "tr from the note above".
	tr := an.key("ek.trill-sys2")
	b.ringAnchor(cond, "ek-ring-tr", tr, wireStyle{Color: colorConductor, Opacity: 1, Width: 0.0035})
	b.labelNear(cond, "ek-tr-lab", 0, tr.X0-0.02, tr.Y0-0.016, "tr from the note above", colorConductor, 0.013)
	// 31) Conductor: ⚠ stamp + "don't rush the runs" in the clear band above sys 2 (right).
	m2 := an.key("ek.margin-sys2-right")
	b.iconAnchor(cond, "ek-ic-warn", 0, m2.X0, m2.cy()-0.009, 0.018, "warning", colorConductor)
	b.labelNear(cond, "ek-warn-lab", 0, m2.X0+0.026, m2.cy()+0.004, "don't rush the runs", colorConductor, 0.013)
	// 32) Shared: "p subito — echo" in the clear band BELOW system 2, a short connector up-left to
	//     the p (its old spot ran the label + connector straight through the sys-2 noteheads).
	p := an.key("ek.p-sys2")
	b.line(shared, "ek-p-pt", 0, p.X0+0.045, p.Y1+0.007, p.cx(), p.Y1, wireStyle{Color: colorShared, Opacity: 1, Width: 0.003})
	b.labelNear(shared, "ek-p-lab", 0, p.X0+0.04, p.Y1+0.008, "p subito — echo", "#B45309", 0.013)
	return *im
}

// buildScoreConductorAnnotations — the CONDUCTOR'S full score. A "Conductor's marks" layer
// (tempo/interpretation, mandatory) + an "Interpretation" layer (performance-practice
// recommendations, shared). Text in the clear margins above the top staff / below the bottom
// staff of the first system. `piece` selects the piece-specific notes.
func buildScoreConductorAnnotations(songID, fileID, piece, conductorID string, an *anchorSet) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}
	cond := b.layer(wireLayer{ID: layerID(songID, "score-cond"), Name: "Conductor's marks", OwnerID: conductorID, Zone: "conductor", Order: 0, Access: "ro", Mandatory: true, RoleTag: "conductor"})
	interp := b.layer(wireLayer{ID: layerID(songID, "score-interp"), Name: "Interpretation", OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "rw"})
	// A red conductor bracket down the left edge of system 1 (hand-drawn), from the manifest.
	brk := an.key("sys1-bracket")
	b.freehand(cond, "sc-brk", 0, handBracket("sc-brk-"+piece, brk.X0, brk.Y0, brk.Y1), wireStyle{Color: colorConductor, Opacity: 0.95, Width: 0.005})
	ta, tc, ti := an.key("text-tempo"), an.key("text-cue"), an.key("text-interp")
	if piece == "canon" {
		b.labelNear(cond, "sc-tempo", 0, ta.X0, ta.cy()+0.005, "Andante — don't rush the ground bass", colorConductor, 0.011)
		b.labelNear(cond, "sc-cue", 0, tc.X0, tc.cy()+0.005, "build over each variation; watch the final rit.", colorConductor, 0.011)
		b.labelNear(interp, "sc-int", 0, ti.X0, ti.cy()+0.005, "Baroque: light bowing, minimal vibrato — let the bass drive.", colorShared, 0.011)
	} else {
		b.labelNear(cond, "sc-tempo", 0, ta.X0, ta.cy()+0.005, "Allegro — light & buoyant; watch every f / p contrast", colorConductor, 0.011)
		b.labelNear(cond, "sc-cue", 0, tc.X0, tc.cy()+0.005, "unison attacks together; trills from the note above", colorConductor, 0.011)
		b.labelNear(interp, "sc-int", 0, ti.X0, ti.cy()+0.005, "Classical: crisp, transparent, leggiero — never heavy.", colorShared, 0.011)
	}
	return *im
}

// buildCelloBowingAnnotations — a player's personal note on the cello (basso) part (Cory).
func buildCelloBowingAnnotations(songID, fileID, piece string, userID map[string]string) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}
	mine := b.layer(wireLayer{ID: layerID(songID, "cello-bow"), Name: "Bowing (Cello)", OwnerID: userID["cory"], Zone: "personal", Order: 0, Access: "rw", RoleTag: "strings"})
	note := "let the bass sing — long, steady bows"
	if piece == "canon" {
		note = "the ground bass — steady, foundational"
	}
	b.text(mine, "vc-bow", 0, 0.10, 0.075, note, wireStyle{Color: colorPersonal2, Opacity: 1, FontSize: 0.013})
	return *im
}

// buildOpenRoadAnnotations places meaningful annotations on "The Open Road" LEAD
// SHEET (a local chart PDF with a known chords-over-lyrics layout — page 0), rather
// than the generated-staff or fetched layouts buildSongAnnotations handles. Coords
// match docs/demo-charts/open-road-leadsheet.pdf (page-relative [0,1]); ink infers
// fill+multiply for `highlight` and stroke for `ellipse`, so no extra style flags
// are needed. Three layers: Form (mandatory), Conductor cues (mandatory, conductor
// role), My notes (personal, owned by the singer/admin).
// buildOpenRoadAnnotations places the B13 hero-page showcase (marks 1–11) on The Open Road lead
// sheet, every mark anchored to a real run (an) and hand-drawn (handStroke/handRing). Four
// layers: Form (shared, mandatory), Conductor cues (mandatory), and Marie's + Sasha's personal
// notes.
func buildOpenRoadAnnotations(songID, fileID string, userID map[string]string, conductorID string, an *anchorSet) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}

	form := b.layer(wireLayer{ID: layerID(songID, "form"), Name: "Form / sections", OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "ro", Mandatory: true})
	cond := b.layer(wireLayer{ID: layerID(songID, "cues"), Name: "Conductor cues", OwnerID: conductorID, Zone: "conductor", Order: 0, Access: "ro", Mandatory: true, RoleTag: "conductor"})
	marie := b.layer(wireLayer{ID: layerID(songID, "mine-marie"), Name: "My notes", OwnerID: userID["marie"], Zone: "personal", Order: 0, Access: "rw"})
	sasha := b.layer(wireLayer{ID: layerID(songID, "mine-sasha"), Name: "My notes", OwnerID: userID["sasha"], Zone: "personal", Order: 1, Access: "rw"})

	chorusLbl := an.run("Chorus", 1)
	lastChorus := an.run("the map is just a rumour", 1)

	// 1) Form: hand-drawn bracket down the left of the Chorus block.
	b.freehand(form, "or-brk", 0, handBracket("or-brk", 0.055, chorusLbl.Y0-0.004, lastChorus.Y1+0.006),
		wireStyle{Color: colorShared, Opacity: 1, Width: 0.005})
	// 2) Form: "everyone in!" right of the printed Chorus heading.
	b.labelNear(form, "or-chorus-lab", 0, chorusLbl.X1+0.012, chorusLbl.cy()+0.006, "everyone in!", "#B45309", 0.015)
	// 3) Form: amber highlight band over chorus line 1 — a real `highlight` TOOL object (exercises
	//    the baker's TypeHighlight fill+multiply path), not a hand-drawn swipe.
	b.highlightTypeAnchor(form, "or-hi-drive", an.text("So drive, drive into the wide unknown,", 1), colorShared, 0.30)

	// 4) Marie: the FLAGSHIP line-wide yellow highlighter over a full lyric.
	b.highlightAnchor(marie, "or-hi-singloud", an.run("Sing loud", 1), "#FACC15", 0.42)
	// 5) Marie: green highlighter over the printed "Capo 2".
	capo := an.text("Capo 2", 1)
	b.highlightAnchor(marie, "or-hi-capo", capo, colorPersonal2, 0.42)
	// 6) Marie: ⚠ warning stamp (T51 icon) in the clear space right of the Capo swipe.
	b.iconAnchor(marie, "or-ic-warn", 0, capo.X1+0.012, capo.Y0-0.003, 0.020, "warning", "#EA580C")

	// 7) Sasha: semi-transparent blue rect around the whole Verse 2 block.
	v2 := union(an.run("Verse 2", 1), an.run("Coffee going cold", 1), an.run("counting every town", 1),
		an.run("Headlights on a hill", 1), an.run("the only way to stay", 1))
	b.rectAnchor(sasha, "or-rect-v2", v2, 0.006, "#2563EB", 0.18)
	// 8) Sasha: shaker icon straddling the rect's right edge, in the clear right margin beside
	//    the (short) "Verse 2" label — half in / half out, clear of the chorus line above.
	shX, shY := v2.X1-0.007, v2.Y0+0.004
	b.iconAnchor(sasha, "or-ic-shaker", 0, shX, shY, 0.030, "shaker", "#2563EB")
	// 9) Sasha: label right of the (now larger) shaker.
	b.labelNear(sasha, "or-shaker-lab", 0, shX+0.040, shY+0.021, "shaker on v2", "#2563EB", 0.014)

	// 10) Conductor: ellipse (tool) around the last chorus line's final G chord cell.
	gCell := subOf(an.runNear(0, 0.508, "G"), "G", -1)
	b.ellipseAnchor(cond, "or-ell-g", gCell, 0.005, wireStyle{Color: colorConductor, Opacity: 1, Width: 0.0035})
	// 11) Conductor: "rit. on the last G" in the clear margin PAST the lyric line's end
	//     (x>lastChorus.X1), with a pointer running back left to the ellipsed G along the
	//     empty chord row (y=cy, above the lyric). Placing it mid-line drew it over "the
	//     wheels have grown." — the ink test now guards this.
	b.line(cond, "or-rit-pt", 0, lastChorus.X1+0.024, gCell.cy(), gCell.X1+0.010, gCell.cy(), wireStyle{Color: colorConductor, Opacity: 1, Width: 0.0035})
	b.labelNear(cond, "or-rit-lab", 0, lastChorus.X1+0.026, gCell.cy()-0.006, "rit. on the last G", colorConductor, 0.015)

	// ── Page 2 (riff tab): marks 12–13 ─────────────────────────────────────────
	leo := b.layer(wireLayer{ID: layerID(songID, "mine-leo"), Name: "My notes", OwnerID: userID["leo"], Zone: "personal", Order: 2, Access: "rw", RoleTag: "guitar"})
	// 12) Leo: green highlighter over bar 1 of the top tab line + a note in the right margin.
	tab := an.run("e|", 1)
	bar1 := anchorBox{Page: tab.Page, X0: tab.X0, Y0: tab.Y0, X1: tab.X0 + tab.w()*0.26, Y1: tab.Y1}
	b.highlightAnchor(leo, "or2-hi-riff", bar1, colorPersonal2, 0.42)
	b.labelNear(leo, "or2-riff-lab", tab.Page, tab.X1+0.02, tab.cy()+0.004, "lead with the thumb", colorPersonal2, 0.014)
	// 13) Conductor: a hand-drawn underline under the performance note + "4x — build each time".
	note := an.run("Riff: play 4x", 1)
	ulBox := anchorBox{Page: note.Page, X0: note.X0, Y0: note.Y1 + 0.001, X1: note.X1, Y1: note.Y1 + 0.003}
	b.freehand(cond, "or2-note-ul", note.Page, handStroke("or2-note-ul", ulBox), wireStyle{Color: colorConductor, Opacity: 1, Width: 0.003})
	b.labelNear(cond, "or2-note-cue", note.Page, note.X0, note.Y1+0.028, "4x — build each time", colorConductor, 0.014)

	return *im
}

// buildOpenRoadGuitarAnnotations annotates the guitarist's chart (open-road-guitar.pdf: an
// intro riff + Verse/Chorus chord-bar charts). Every mark is explained: a green capo reminder
// on the printed "Capo 2", a shared "quick change" on the Em→C turnaround bar, and the
// conductor's "rit. on the last bar" ringing the final chorus measure.
func buildOpenRoadGuitarAnnotations(songID, fileID string, userID map[string]string, conductorID string, an *anchorSet) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}

	leo := b.layer(wireLayer{ID: layerID(songID, "gtr-mine"), Name: "My notes", OwnerID: userID["leo"], Zone: "personal", Order: 0, Access: "rw", RoleTag: "guitar"})
	marie := b.layer(wireLayer{ID: layerID(songID, "gtr-marie"), Name: "My notes", OwnerID: userID["marie"], Zone: "personal", Order: 1, Access: "rw"})
	shared := b.layer(wireLayer{ID: layerID(songID, "gtr-shared"), Name: "Section markings", OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "rw"})
	cond := b.layer(wireLayer{ID: layerID(songID, "gtr-cues"), Name: "Conductor cues", OwnerID: conductorID, Zone: "conductor", Order: 0, Access: "ro", Mandatory: true, RoleTag: "conductor"})

	// Leo: green highlighter over the printed "Capo 2" + a "capo on!" note in the clear gap below.
	capo := an.text("Capo 2", 1)
	b.highlightAnchor(leo, "gtr-capo-hi", capo, colorPersonal2, 0.42)
	b.labelNear(leo, "gtr-capo-lab", 0, capo.X0-0.01, capo.Y1+0.024, "capo on!", colorPersonal2, 0.014)

	// 14) Marie: red guitar-electric stamp beside the title meta + a note (ties to her T50 cue).
	meta := an.run("Standard tuning", 1)
	b.iconAnchor(marie, "gtr-ic-electric", 0, meta.X1+0.012, meta.Y0-0.005, 0.022, "guitar-electric", "#e11d48")
	b.labelNear(marie, "gtr-electric-lab", 0, meta.X1+0.040, meta.Y0+0.010, "Marie takes the red electric", "#e11d48", 0.013)

	// Shared: hand-drawn ring around the Em→C turnaround bar (verse row 2, bar 3) + a note below.
	emc := an.boxAt(0, 0.60, 0.395)
	b.ringAnchor(shared, "gtr-emc-ring", emc, wireStyle{Color: colorShared, Opacity: 1, Width: 0.0035})
	b.labelNear(shared, "gtr-emc-lab", 0, emc.X0, emc.Y1+0.012, "quick Em → C", "#B45309", 0.014)

	// Conductor: ellipse around the final chorus bar (last G) + the ritardando cue below-left.
	lastBar := an.boxAt(0, 0.81, 0.537)
	b.ellipseAnchor(cond, "gtr-last", lastBar, 0.004, wireStyle{Color: colorConductor, Opacity: 1, Width: 0.0035})
	b.labelNear(cond, "gtr-last-lab", 0, 0.60, lastBar.Y1+0.014, "rit. on the last bar", colorConductor, 0.014)
	return *im
}

// buildBandChartAnnotations places a light 3-layer showcase (conductor / shared / personal)
// on a committed BAND chart — the House of the Rising Sun guitar tab or the Amazing Grace
// lead sheet, both mkcharts A4 with a title block above ~0.17 and content below it. Not the
// pixel-tuned Open Road showcase; enough to demo the layer model (mandatory conductor cues,
// shared section highlight, a personal note) over a real chart with sensible placement.
func buildBandChartAnnotations(songID, fileID, title string, userID map[string]string, conductorID string, an *anchorSet) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}

	switch title {
	case "Amazing Grace":
		// B13 marks 18–20 — all anchored to real lyric runs.
		mine := b.layer(wireLayer{ID: layerID(songID, "mine"), Name: "My notes", OwnerID: userID["marie"], Zone: "personal", Order: 0, Access: "rw"})
		shared := b.layer(wireLayer{ID: layerID(songID, "shared"), Name: "Section markings", OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "rw"})
		cond := b.layer(wireLayer{ID: layerID(songID, "conductor"), Name: "Conductor cues", OwnerID: conductorID, Zone: "conductor", Order: 0, Access: "ro", Mandatory: true, RoleTag: "conductor"})
		// 18) yellow freehand highlighter over the full phrase "that saved a wretch like me".
		b.highlightAnchor(mine, "ag-hi-wretch", an.run("that saved a wretch like me", 1), "#FACC15", 0.42)
		// 19) shared highlight-band (multiply rect) over the whole of verse 3 + a label.
		v3 := union(an.run("Verse 3", 1), an.run("Through many dangers", 1), an.run("I have already come", 1),
			an.run("'Tis grace hath brought", 1), an.run("and grace will lead me home", 1))
		b.rectAnchor(shared, "ag-rect-v3", v3, 0.006, colorShared, 0.16)
		b.labelNear(shared, "ag-v3-lab", 0, v3.X1+0.02, v3.Y0+0.02, "v.3 a cappella — listen", "#B45309", 0.014)
		// 20) conductor hand-drawn ring around "home" (v.3 last line) + "fermata".
		home := subOf(an.run("and grace will lead me home", 1), "home", 1)
		b.ringAnchor(cond, "ag-ring-home", home, wireStyle{Color: colorConductor, Opacity: 1, Width: 0.0035})
		b.labelNear(cond, "ag-home-lab", 0, home.X1+0.014, home.Y1+0.008, "fermata", colorConductor, 0.014)
		return *im
	}

	switch title {
	case "House of the Rising Sun":
		// B13 marks 15–17 + migrated F-barre / let-it-ring / E-turnaround, all anchored.
		leo := b.layer(wireLayer{ID: layerID(songID, "mine"), Name: "My notes", OwnerID: userID["leo"], Zone: "personal", Order: 0, Access: "rw", RoleTag: "guitar"})
		shared := b.layer(wireLayer{ID: layerID(songID, "shared"), Name: "Section markings", OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "rw"})
		cond := b.layer(wireLayer{ID: layerID(songID, "conductor"), Name: "Conductor cues", OwnerID: conductorID, Zone: "conductor", Order: 0, Access: "ro", Mandatory: true, RoleTag: "conductor"})
		verse := an.runNear(0, 0.202, "Am")
		// Leo: green highlighter over the F (barre) chord + a note.
		fCell := subOf(verse, "F", 1)
		b.highlightAnchor(leo, "hr-hi-f", fCell, colorPersonal2, 0.42)
		b.labelNear(leo, "hr-f-lab", 0, fCell.X0-0.012, fCell.Y0-0.016, "F = barre", colorPersonal2, 0.013)
		// Shared: hand-drawn bracket down the verse tab + "let it ring" + a guitar-acoustic stamp (15).
		b.freehand(shared, "hr-brk", 0, handBracket("hr-brk", 0.05, verse.Y0-0.004, verse.Y0+0.130), wireStyle{Color: colorShared, Opacity: 1, Width: 0.005})
		b.labelNear(shared, "hr-ring-lab", 0, 0.045, verse.Y0-0.016, "let it ring", "#B45309", 0.014)
		// guitar-acoustic stamp tucked in the clear left margin just below the label (its old
		// spot at x0.14 sat over the "Verse" heading / first chord — ink test guards it now).
		b.iconAnchor(shared, "hr-ic-acoustic", 0, 0.065, verse.Y0+0.004, 0.020, "guitar-acoustic", colorShared)
		// Conductor: ⚠ stamp (16) + "watch the 6/8 lilt" in the clear space at the end of the
		// header meter line (which carries the 6/8); ring the E turnaround.
		meta := an.run("Key: A minor", 1)
		b.iconAnchor(cond, "hr-ic-warn", 0, meta.X1+0.014, meta.Y0-0.004, 0.020, "warning", colorConductor)
		b.labelNear(cond, "hr-warn-lab", 0, meta.X1+0.042, meta.Y0+0.010, "watch the 6/8 lilt", colorConductor, 0.013)
		b.ringAnchor(cond, "hr-ring-e", subOf(verse, "E", 1), wireStyle{Color: colorConductor, Opacity: 1, Width: 0.0035})
		// Leo: quick-change highlight over the chorus D→F cells (17) + label.
		df := subOf(an.runNear(0, 0.389, "Am"), "D    F", 1)
		b.highlightAnchor(leo, "hr-hi-df", df, "#FACC15", 0.42)
		b.labelNear(leo, "hr-df-lab", 0, df.X0, df.Y1+0.016, "quick change!", "#B45309", 0.014)
	case "Greensleeves":
		// B13 marks 21–26 — engraved (Mutopia); positions from the hand-calibrated gs.* anchors.
		shared := b.layer(wireLayer{ID: layerID(songID, "shared"), Name: "Section markings", OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "rw"})
		cond := b.layer(wireLayer{ID: layerID(songID, "conductor"), Name: "Conductor cues", OwnerID: conductorID, Zone: "conductor", Order: 0, Access: "ro", Mandatory: true, RoleTag: "conductor"})
		mine := b.layer(wireLayer{ID: layerID(songID, "mine"), Name: "My notes", OwnerID: userID["marie"], Zone: "personal", Order: 0, Access: "rw"})
		// 21) personal yellow highlighter over the verse-1 lyric line.
		b.highlightAnchor(mine, "gs-hi-v1", an.key("gs.v1-lyric-open"), "#FACC15", 0.42)
		// 22) conductor "gently, in 3" in the clear space right of the subtitle. The label draws
		//     DOWN from its point (textBaseline top), so anchor the point at the box TOP.
		tempo := an.key("gs.tempo-space")
		b.labelNear(cond, "gs-tempo", 0, tempo.X0, tempo.Y0+0.001, "gently, in 3", colorConductor, 0.014)
		// 23) shared "guitar: let the arpeggio breathe" below the tab + a connector up to it.
		tab := an.key("gs.tab-sys1")
		b.line(shared, "gs-tab-pt", 0, tab.X0+0.02, tab.Y1+0.02, tab.cx(), tab.Y1, wireStyle{Color: colorShared, Opacity: 1, Width: 0.003})
		b.labelNear(shared, "gs-tab-lab", 0, tab.X0+0.02, tab.Y1+0.03, "guitar: let the arpeggio breathe", "#B45309", 0.014)
		// 24) shared "v.4 — guitar tacet, voice alone" in the clear inter-system band on p.2
		//     (the full-width gap below the middle voice line — verified empty by the ink test).
		b.labelNear(shared, "gs2-tacet", 1, 0.06, 0.505, "v.4 — guitar tacet, voice alone", "#B45309", 0.013)
		// 25) conductor hand-drawn ring around the final cadence + "rit. — die away" just above it
		//     in the same clear band (p.2).
		cad := an.key("gs2.final-cadence")
		b.ringAnchor(cond, "gs2-ring-cad", cad, wireStyle{Color: colorConductor, Opacity: 1, Width: 0.0035})
		b.labelNear(cond, "gs2-cad-lab", 1, cad.X0, 0.505, "rit. — die away", colorConductor, 0.013)
		// 26) personal green note stamp at the harmonic X + "harmonics!" (p.2).
		hx := an.key("gs2.harmonic-x")
		b.iconAnchor(mine, "gs2-ic-note", 1, hx.X1+0.008, hx.Y0, 0.018, "note", colorPersonal2)
		b.labelNear(mine, "gs2-note-lab", 1, hx.X0-0.03, hx.Y0-0.016, "harmonics!", colorPersonal2, 0.013)
	}
	return *im
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
func buildDrumPartAnnotations(songID, fileID string, an *anchorSet) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}

	notes := b.layer(wireLayer{ID: layerID(songID, "drum-notes"), Name: "Drummer's notes", OwnerID: "_shared_", Zone: "shared", Order: 1, Access: "rw", RoleTag: "drums"})
	// Anchored to the groove-grid rows. Highlight the SNARE hit on beat 4 (the only 'x' in the
	// Snare row); labels sit in the clear space right of the grid (rows end well before it).
	snare := subOf(an.run("Snare", 1), "x", 1)
	b.highlightAnchor(notes, "drum-snare-hi", snare, "#FACC15", 0.42)
	b.line(notes, "drum-snare-pt", 0, 0.55, snare.cy(), snare.X1+0.008, snare.cy(), wireStyle{Color: colorPersonal2, Opacity: 1, Width: 0.003})
	b.labelNear(notes, "drum-snare", 0, 0.56, snare.cy()+0.005, "snare (beat 4) — lay back", colorPersonal2, 0.013)
	b.labelNear(notes, "drum-soft", 0, 0.56, an.run("Kick", 1).cy()+0.005, "keep it soft under the vocal", colorPersonal2, 0.013)
	return *im
}
