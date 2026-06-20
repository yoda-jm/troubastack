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

// buildSongAnnotations assembles ~3 layers (conductor / shared / personal) of
// meaningful objects for one song, shaped by song title + group kind.
//
// userID maps username → user id (for layer ownerId), adminID is the group
// admin/conductor user id, generated says whether the PDF is a generated placeholder
// (precise layout) vs a fetched real PDF (generic placement).
func buildSongAnnotations(songID, fileID, title, groupKind string, userID map[string]string, adminID string, generated bool, pages int) annotationsImport {
	im := &annotationsImport{Layers: []wireLayer{}, Objects: []wireObject{}}
	b := &builderCtx{songID: songID, fileID: fileID, im: im}

	// ---- conductor zone: "Conductor cues" (mandatory, red) ----
	cond := b.layer(wireLayer{
		ID: layerID(songID, "conductor"), Name: "Conductor cues",
		OwnerID: adminID, Zone: "conductor", Order: 0, Access: "ro", Mandatory: true,
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

	// Pick chords per song; default to a plausible progression.
	chords := []string{"Em", "G", "D", "A"}
	capo := "Capo 2"
	switch title {
	case "Wonderwall":
		chords = []string{"Em7", "G", "Dsus4", "A7sus4"}
		capo = "Capo 2"
	case "Hallelujah":
		chords = []string{"C", "Am", "F", "G", "E7", "Am"}
		capo = ""
	case "Black Hole Sun":
		chords = []string{"G", "D", "F", "C", "Bb"}
		capo = ""
	}

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
