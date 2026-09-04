package app_test

import (
	"encoding/json"
	"testing"

	"troubastack/core/internal/app"
)

// TestSlugify_Vectors pins the ONE slug rule (T139) — apostrophes, slashes, punctuation runs, and an
// empty result. Once nothing re-derives an existing slug, this rule only governs newly-created songs, but
// it must still be stable and single-homed. (Fictional inputs — no real repertoire data in tracked code.)
func TestSlugify_Vectors(t *testing.T) {
	cases := []struct{ in, want string }{
		{"L'Ete Indien", "l-ete-indien"},                              // apostrophe → separator
		{"Well-Worn Path", "well-worn-path"},                          // hyphen kept as one separator
		{"A Long Title / With A Slash?", "a-long-title-with-a-slash"}, // slash + '?'
		{"a---b", "a-b"},                    // punctuation RUN collapses to one '-'
		{"a . . . b", "a-b"},                // spaces + dots run collapses
		{"  Hello  World  ", "hello-world"}, // trim + inner run
		{"!!!", "song"},                     // empty result → fallback
		{"", "song"},                        // empty input → fallback
	}
	for _, c := range cases {
		if got := app.Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestSlug_RoundTripPreservesAuthoredSlug is the T139 acceptance test that fails today: a folder whose
// slugs are hand-chosen (deliberately NOT what the rule would compute) must export back byte-identical.
// `refrain` is shorter than its title and `lete-indien` drops the apostrophe-gap — neither is reproducible
// by any derivation, so a passing round-trip proves the slug is stored, not derived. (Fictional data.)
func TestSlug_RoundTripPreservesAuthoredSlug(t *testing.T) {
	band, _ := json.Marshal(map[string]any{
		"formatVersion": 2, "name": "Authored Slugs",
		"members": []any{map[string]any{"username": "dana", "displayName": "Dana", "role": "admin"}},
	})
	rep, _ := json.Marshal(map[string]any{
		"songs": []any{
			map[string]any{"slug": "refrain", "title": "Ce Vieux Refrain"},
			map[string]any{"slug": "lete-indien", "title": "L'Ete Indien"},
		},
	})
	zipBytes := rezip(t, map[string][]byte{"band.json": band, "repertoire.json": rep})

	tgt := newStack()
	importer, _ := tgt.svc.Register("owner", "Owner", "password123", "")
	report, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	out, _, err := tgt.svc.ExportBand(importer, tgt.eng, report.Band.ID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	var exp struct {
		Songs []struct{ Slug, Title string } `json:"songs"`
	}
	mustJSON(t, unzip(t, out)["repertoire.json"], &exp)

	got := map[string]string{} // title -> exported slug
	for _, s := range exp.Songs {
		got[s.Title] = s.Slug
	}
	if got["Ce Vieux Refrain"] != "refrain" {
		t.Errorf("authored slug not preserved: Ce Vieux Refrain exported as %q, want refrain", got["Ce Vieux Refrain"])
	}
	if got["L'Ete Indien"] != "lete-indien" {
		t.Errorf("authored slug not preserved: L'Ete Indien exported as %q, want lete-indien", got["L'Ete Indien"])
	}
	// teeth: the derivation would produce these OTHER values, so the test guards preservation, not luck.
	if app.Slugify("Ce Vieux Refrain") != "ce-vieux-refrain" || app.Slugify("L'Ete Indien") != "l-ete-indien" {
		t.Fatal("derivation changed; update this teeth-check so it still discriminates stored vs derived")
	}
}

// TestSlug_TitleEditKeepsRefs: a title edit must NOT rename the slug, so the annotation file and setlist
// item that point at it (in export/folder form) still resolve afterwards. This is the reference-break the
// task exists to prevent. (Fictional data.)
func TestSlug_TitleEditKeepsRefs(t *testing.T) {
	band, _ := json.Marshal(map[string]any{
		"formatVersion": 2, "name": "Edit Band",
		"members": []any{map[string]any{"username": "dana", "displayName": "Dana", "role": "admin"}},
	})
	rep, _ := json.Marshal(map[string]any{
		"songs": []any{map[string]any{"slug": "refrain", "title": "Ce Vieux Refrain"}},
	})
	ann, _ := json.Marshal(map[string]any{
		"layers":  []any{map[string]any{"id": "L1", "name": "Cues", "owner": "_shared_", "zone": "shared", "access": "rw"}},
		"objects": []any{},
	})
	zipBytes := rezip(t, map[string][]byte{
		"band.json": band, "repertoire.json": rep, "annotations/refrain.json": ann,
	})

	st := newStack()
	importer, _ := st.svc.Register("owner", "Owner", "password123", "")
	report, err := st.svc.ImportBand(importer, st.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	songs, _ := st.repo.SongsOfBand(report.Band.ID)
	if len(songs) != 1 || songs[0].Slug != "refrain" {
		t.Fatalf("imported song slug = %q, want refrain", songs[0].Slug)
	}

	// a setlist referencing the song (by SongID at runtime)
	sl, err := st.svc.CreateSetlist(importer, report.Band.ID, "Gig", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.svc.AddSetlistItem(importer, report.Band.ID, sl.ID, songs[0].ID); err != nil {
		t.Fatal(err)
	}

	// rename the song — the slug must not move
	title := "A Completely Different Title"
	updated, err := st.svc.UpdateSong(importer, report.Band.ID, songs[0].ID, app.SongPatch{Title: &title})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Slug != "refrain" {
		t.Fatalf("title edit renamed the slug to %q — reference break", updated.Slug)
	}

	// export: the annotation file and setlist item still point at the stable slug
	out, _, err := st.svc.ExportBand(importer, st.eng, report.Band.ID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	entries := unzip(t, out)
	if _, ok := entries["annotations/refrain.json"]; !ok {
		t.Fatalf("annotation file renamed off its slug; entries=%v", keysOf(entries))
	}
	var rep2 struct {
		Songs []struct{ Slug string } `json:"songs"`
	}
	mustJSON(t, entries["repertoire.json"], &rep2)
	if len(rep2.Songs) != 1 || rep2.Songs[0].Slug != "refrain" {
		t.Fatalf("exported slug = %+v, want refrain", rep2.Songs)
	}
	var sls struct {
		Setlists []struct {
			Items []struct{ Song string } `json:"items"`
		} `json:"setlists"`
	}
	mustJSON(t, entries["setlists.json"], &sls)
	if len(sls.Setlists) != 1 || len(sls.Setlists[0].Items) != 1 || sls.Setlists[0].Items[0].Song != "refrain" {
		t.Fatalf("setlist item lost its slug ref: %+v", sls.Setlists)
	}
}

// TestCreateSong_DerivesUniqueStableSlug: create derives the slug ONCE (unique within the band), and a
// later title edit leaves it alone.
func TestCreateSong_DerivesUniqueStableSlug(t *testing.T) {
	st := newStack()
	admin, err := st.svc.Register("marie", "Marie", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.svc.CreateBand(admin, "Band")
	if err != nil {
		t.Fatal(err)
	}
	one, err := st.svc.CreateSong(admin, b.ID, "My Song", "")
	if err != nil {
		t.Fatal(err)
	}
	if one.Slug != "my-song" {
		t.Fatalf("first slug = %q, want my-song", one.Slug)
	}
	// same title again → must NOT collide
	two, err := st.svc.CreateSong(admin, b.ID, "My Song", "")
	if err != nil {
		t.Fatal(err)
	}
	if two.Slug != "my-song-2" {
		t.Fatalf("second slug = %q, want my-song-2 (unique within band)", two.Slug)
	}
	// title edit does not re-derive
	nt := "My Song Renamed"
	up, err := st.svc.UpdateSong(admin, b.ID, one.ID, app.SongPatch{Title: &nt})
	if err != nil {
		t.Fatal(err)
	}
	if up.Slug != "my-song" {
		t.Fatalf("title edit changed slug to %q, want my-song", up.Slug)
	}
}
