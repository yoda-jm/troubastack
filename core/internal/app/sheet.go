package app

import (
	"sort"
	"strings"
)

// T203 stage 4 — the sound sheet MODEL. Go is AUTHORITATIVE (Fable, 2026-10-24): BuildSheet returns this model
// as JSON and Studio, the PDF and the CSV all render it, so screen = PDF = CSV. The person-major projection
// mirrors the TS editor's buildGrid in SHEET mode (used-only), filtered to the setlist's running order; the two
// are kept honest by a shared set of test vectors.

// Centralized sheet label words (Fable Q2): product words live in ONE place, so a wording change is one edit.
// VLL may rename any of these; nothing should hard-code the string elsewhere.
const (
	SheetLabelSameChannel = "same channel" // a shared instrument changes player — no repatch
	SheetLabelRepatch     = "repatch"      // a personal instrument moves between people — repatch
	SheetLabelAcoustic    = "— acoustic"   // an unamplified part's console-input row
	SheetLabelNotPatched  = "not patched"  // a channel with no patch entry
	sheetLabelInSuffix    = " in"          // "D in"
	sheetLabelOutSuffix   = " out"         // "C out"
)

// SheetOrder is the column order: stage order (Musician.order) or patch order (by input number).
type SheetOrder string

const (
	SheetOrderStage SheetOrder = "stage"
	SheetOrderPatch SheetOrder = "patch"
)

// SheetSubcol is one channel column of the sheet (the x-axis unit).
type SheetSubcol struct {
	Key       string `json:"key"`       // channel key: voice:<m> | inst:<m>:<p> | shared:<p>
	Kind      string `json:"kind"`      // "voice" | "inst" | "shared"
	OwnerID   string `json:"ownerId"`   // the row-1 colspan owner: musicianId (voice/inst) or partId (shared)
	OwnerName string `json:"ownerName"` // row 1 text: "Ann" or a shared part's "Bass — C/V"
	Channel   string `json:"channel"`   // row 2 text: "Voice" | "Guitar" | …
	Input     string `json:"input"`     // row 3 text: "In 4 · A GTR" | "— acoustic" | "not patched"
	Amplified bool   `json:"amplified"` // false when the part's input is "none"
	// The RAW patch for this channel (0/"" when unpatched), so the Studio editor can bind to it — the formatted
	// Input above is for display/PDF/CSV. Omitted for an unamplified channel (never patched).
	PatchInput int    `json:"patchInput,omitempty"`
	PatchStrip string `json:"patchStrip,omitempty"`
}

// SheetOwnerSpan is a run of adjacent same-owner subcols for the row-1 colspan, in the CURRENT column order.
type SheetOwnerSpan struct {
	OwnerName string `json:"ownerName"`
	Span      int    `json:"span"`
}

type SheetCell struct {
	Text     string `json:"text"`
	Optional bool   `json:"optional,omitempty"`
}

type SheetRow struct {
	SongID    string               `json:"songId"`
	Position  int                  `json:"position"`
	Title     string               `json:"title"`
	Cells     map[string]SheetCell `json:"cells"` // keyed by subcol key
	Changes   []string             `json:"changes,omitempty"`
	SoundNote string               `json:"soundNote,omitempty"`
}

// SheetPatchLine is one line of the input list (patched channels, sorted by input).
type SheetPatchLine struct {
	Input int    `json:"input"`
	Strip string `json:"strip"`
	Who   string `json:"who"`  // musician display name, or a shared part's "Bass (shared)"
	Part  string `json:"part"` // the channel's part (voice part name, instrument name)
	Mic   string `json:"mic"`  // mic | di | line
	Key   string `json:"key"`
}

// SheetModel is the whole authoritative sheet.
type SheetModel struct {
	SetlistID   string           `json:"setlistId"`
	SetlistName string           `json:"setlistName"`
	Order       SheetOrder       `json:"order"`
	Subcols     []SheetSubcol    `json:"subcols"`
	OwnerSpans  []SheetOwnerSpan `json:"ownerSpans"`
	Rows        []SheetRow       `json:"rows"`
	InputList   []SheetPatchLine `json:"inputList"`
	NotPatched  []string         `json:"notPatched,omitempty"` // descriptions of channels with no patch entry
}

// BuildSheet projects the sound sheet for one setlist. order ∈ {stage, patch}; empty defaults to stage.
func (s *Service) BuildSheet(caller User, bandID, setlistID string, order SheetOrder) (SheetModel, error) {
	if order != SheetOrderPatch {
		order = SheetOrderStage
	}
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return SheetModel{}, err
	}
	detail, err := s.Setlist(caller, bandID, setlistID)
	if err != nil {
		return SheetModel{}, err
	}
	views, err := s.Musicians(caller, bandID)
	if err != nil {
		return SheetModel{}, err
	}
	name := make(map[string]string, len(views))
	for _, v := range views {
		name[v.ID] = v.DisplayName
	}
	allSongs, err := s.Songs(caller, bandID)
	if err != nil {
		return SheetModel{}, err
	}
	byID := make(map[string]Song, len(allSongs))
	for _, sg := range allSongs {
		byID[sg.ID] = sg
	}
	// Running order: the setlist items by position (bench/on-call items sort last, as elsewhere).
	items := append([]SetlistItemView(nil), detail.Items...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].OnCall != items[j].OnCall {
			return !items[i].OnCall
		}
		return items[i].Position < items[j].Position
	})
	songs := make([]Song, 0, len(items))
	for _, it := range items {
		if sg, ok := byID[it.SongID]; ok {
			songs = append(songs, sg)
		}
	}

	partByID := make(map[string]Part, len(band.Parts))
	for _, p := range band.Parts {
		partByID[p.ID] = p
	}
	patchByKey := make(map[string]ChannelPatch, len(detail.Setlist.Patch))
	for _, c := range detail.Setlist.Patch {
		patchByKey[c.Channel] = c
	}

	// Project the person-major column structure (pure, shared with the TS editor via test vectors).
	projMus := make([]projMusician, 0, len(views))
	for _, v := range views {
		projMus = append(projMus, projMusician{ID: v.ID, Name: v.DisplayName, Order: v.Order})
	}
	subcols := projectColumns(songs, projMus, partByID)
	// Fill the console-input row (row 3) from the concert's patch; set shared headers' players.
	for i := range subcols {
		subcols[i].Input = inputText(patchByKey, subcols[i].Key, subcols[i].Amplified)
		if subcols[i].Amplified {
			if c, ok := patchByKey[subcols[i].Key]; ok {
				subcols[i].PatchInput = c.Input
				subcols[i].PatchStrip = c.Name
			}
		}
		if subcols[i].Kind == "shared" {
			p := partByID[sharedPartID(subcols[i].Key)]
			if players := sharedPlayerNames(songs, p.ID, name); len(players) > 0 {
				subcols[i].OwnerName = p.Name + " — " + strings.Join(players, "/")
			}
		}
	}
	// Patch order: re-sort by input number, then unpatched, then unamplified.
	if order == SheetOrderPatch {
		sort.SliceStable(subcols, func(i, j int) bool {
			return patchRank(subcols[i], patchByKey) < patchRank(subcols[j], patchByKey)
		})
	}

	// --- rows + cells ---
	rows := make([]SheetRow, 0, len(songs))
	for _, sg := range songs {
		cells := map[string]SheetCell{}
		byMus := map[string]LineupEntry{}
		for _, e := range sg.Lineup {
			byMus[e.MusicianID] = e
		}
		for _, sc := range subcols {
			cells[sc.Key] = sheetCell(sc, byMus, partByID, name)
		}
		rows = append(rows, SheetRow{SongID: sg.ID, Position: positionOf(items, sg.ID), Title: sg.Title, Cells: cells, SoundNote: sg.SoundNote})
	}
	// changes (per row vs the previous row)
	for i := 1; i < len(rows); i++ {
		rows[i].Changes = sheetChanges(songs[i-1], songs[i], partByID, name)
	}

	model := SheetModel{
		SetlistID:   setlistID,
		SetlistName: detail.Setlist.Name,
		Order:       order,
		Subcols:     subcols,
		OwnerSpans:  ownerSpans(subcols),
		Rows:        rows,
	}
	model.InputList, model.NotPatched = inputList(subcols, patchByKey, partByID, name)
	return model, nil
}

// sheetCell is the cell value for a subcol in one song.
func sheetCell(sc SheetSubcol, byMus map[string]LineupEntry, partByID map[string]Part, name map[string]string) SheetCell {
	switch sc.Kind {
	case "voice":
		e, ok := byMus[sc.OwnerID]
		if !ok {
			return SheetCell{}
		}
		var shorts []string
		for _, pid := range e.PartIDs {
			if p, ok := partByID[pid]; ok && p.Kind == PartVoice {
				shorts = append(shorts, PartShort(p))
			}
		}
		return SheetCell{Text: strings.Join(shorts, "+"), Optional: e.Optional}
	case "inst":
		e, ok := byMus[sc.OwnerID]
		if !ok {
			return SheetCell{}
		}
		pid := instPartID(sc.Key)
		if containsString(e.PartIDs, pid) {
			return SheetCell{Text: PartShort(partByID[pid]), Optional: e.Optional}
		}
		return SheetCell{}
	default: // shared: every player of this part in the song, joined
		pid := sharedPartID(sc.Key)
		var players []string
		allOpt := true
		for mid, e := range byMus {
			if containsString(e.PartIDs, pid) {
				players = append(players, firstName(name[mid]))
				if !e.Optional {
					allOpt = false
				}
			}
		}
		if len(players) == 0 {
			return SheetCell{}
		}
		sort.Strings(players)
		return SheetCell{Text: strings.Join(players, "+"), Optional: allOpt}
	}
}

// ownerSpans is the run-length of adjacent same-OwnerID subcols (the row-1 colspans).
func ownerSpans(subcols []SheetSubcol) []SheetOwnerSpan {
	var out []SheetOwnerSpan
	for i := 0; i < len(subcols); {
		j := i + 1
		for j < len(subcols) && subcols[j].OwnerID == subcols[i].OwnerID {
			j++
		}
		out = append(out, SheetOwnerSpan{OwnerName: subcols[i].OwnerName, Span: j - i})
		i = j
	}
	return out
}

// inputList is the patch list: patched channels sorted by input, plus the descriptions of unpatched amplified
// channels. Unamplified channels never appear.
func inputList(subcols []SheetSubcol, patchByKey map[string]ChannelPatch, partByID map[string]Part, name map[string]string) ([]SheetPatchLine, []string) {
	var lines []SheetPatchLine
	var notPatched []string
	for _, sc := range subcols {
		if !sc.Amplified {
			continue
		}
		c, ok := patchByKey[sc.Key]
		if !ok || c.Input <= 0 {
			notPatched = append(notPatched, sc.OwnerName+" · "+sc.Channel)
			continue
		}
		who, part, mic := channelDescription(sc, partByID, name)
		lines = append(lines, SheetPatchLine{Input: c.Input, Strip: c.Name, Who: who, Part: part, Mic: mic, Key: sc.Key})
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Input < lines[j].Input })
	return lines, notPatched
}

func channelDescription(sc SheetSubcol, partByID map[string]Part, name map[string]string) (who, part, mic string) {
	switch sc.Kind {
	case "voice":
		return name[sc.OwnerID], "Voice", InputMic
	case "inst":
		p := partByID[instPartID(sc.Key)]
		return name[sc.OwnerID], p.Name, p.Input
	default:
		p := partByID[sharedPartID(sc.Key)]
		return p.Name + " (shared)", p.Name, p.Input
	}
}

// --- ordering helpers ---

func orderThenFirst(oa, ob, fa, fb int) bool {
	if (oa == 0) != (ob == 0) {
		return oa != 0 // explicit order before unset
	}
	if oa != ob {
		return oa < ob
	}
	return fa < fb
}

func partOrderLess(oa, ob int) bool {
	if (oa == 0) != (ob == 0) {
		return oa != 0
	}
	return oa < ob
}

// patchRank sorts for patch order: patched (by input) first, then unpatched amplified, then unamplified.
func patchRank(sc SheetSubcol, patchByKey map[string]ChannelPatch) int {
	if !sc.Amplified {
		return 2_000_000
	}
	if c, ok := patchByKey[sc.Key]; ok && c.Input > 0 {
		return c.Input
	}
	return 1_000_000
}

func sharedPlayerNames(songs []Song, partID string, name map[string]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, sg := range songs {
		for _, e := range sg.Lineup {
			if containsString(e.PartIDs, partID) && !seen[e.MusicianID] {
				seen[e.MusicianID] = true
				out = append(out, firstName(name[e.MusicianID]))
			}
		}
	}
	return out
}

func positionOf(items []SetlistItemView, songID string) int {
	for _, it := range items {
		if it.SongID == songID {
			return it.Position
		}
	}
	return 0
}

func firstName(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t"); i > 0 {
		return s[:i]
	}
	if s == "" {
		return "?"
	}
	return s
}

func instPartID(key string) string { // "inst:<m>:<p>"
	parts := strings.SplitN(key, ":", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	return ""
}
func sharedPartID(key string) string { // "shared:<p>"
	return strings.TrimPrefix(key, "shared:")
}
