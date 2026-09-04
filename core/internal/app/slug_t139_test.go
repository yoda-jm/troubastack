package app_test

import (
	"encoding/json"
	"testing"

	"troubastack/core/internal/app"
)

// TestSlugify_Vectors pins the ONE slug rule (T139) — apostrophes, slashes, punctuation runs, and an
// empty result. Once nothing re-derives an existing slug, this rule only governs newly-created songs, but
// it must still be stable and single-homed.
func TestSlugify_Vectors(t *testing.T) {
	cases := []struct{ in, want string }{
		{"J'Aime plus Paris", "j-aime-plus-paris"}, // apostrophe → separator
		{"Cet Air-la", "cet-air-la"},               // hyphen kept as one separator
		{"In the Pines / Where Did You Sleep Last Night?", "in-the-pines-where-did-you-sleep-last-night"}, // slash + '?'
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
// `cet-air` is shorter than its title and `jaime-plus-paris` drops the apostrophe-gap — neither is
// reproducible by any derivation, so a passing round-trip proves the slug is stored, not derived.
func TestSlug_RoundTripPreservesAuthoredSlug(t *testing.T) {
	band, _ := json.Marshal(map[string]any{
		"formatVersion": 2, "name": "Authored Slugs",
		"members": []any{map[string]any{"username": "dana", "displayName": "Dana", "role": "admin"}},
	})
	rep, _ := json.Marshal(map[string]any{
		"songs": []any{
			map[string]any{"slug": "cet-air", "title": "Cet Air-la"},
			map[string]any{"slug": "jaime-plus-paris", "title": "J'Aime plus Paris"},
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
	if got["Cet Air-la"] != "cet-air" {
		t.Errorf("authored slug not preserved: Cet Air-la exported as %q, want cet-air", got["Cet Air-la"])
	}
	if got["J'Aime plus Paris"] != "jaime-plus-paris" {
		t.Errorf("authored slug not preserved: J'Aime plus Paris exported as %q, want jaime-plus-paris", got["J'Aime plus Paris"])
	}
	// teeth: the derivation would produce these OTHER values, so the test guards preservation, not luck.
	if app.Slugify("Cet Air-la") != "cet-air-la" || app.Slugify("J'Aime plus Paris") != "j-aime-plus-paris" {
		t.Fatal("derivation changed; update this teeth-check so it still discriminates stored vs derived")
	}
}

// TestSlug_TitleEditKeepsRefs: a title edit must NOT rename the slug, so the annotation file and setlist
// item that point at it (in export/folder form) still resolve afterwards. This is the reference-break the
// task exists to prevent.
func TestSlug_TitleEditKeepsRefs(t *testing.T) {
	band, _ := json.Marshal(map[string]any{
		"formatVersion": 2, "name": "Edit Band",
		"members": []any{map[string]any{"username": "dana", "displayName": "Dana", "role": "admin"}},
	})
	rep, _ := json.Marshal(map[string]any{
		"songs": []any{map[string]any{"slug": "cet-air", "title": "Cet Air-la"}},
	})
	ann, _ := json.Marshal(map[string]any{
		"layers":  []any{map[string]any{"id": "L1", "name": "Cues", "owner": "_shared_", "zone": "shared", "access": "rw"}},
		"objects": []any{},
	})
	zipBytes := rezip(t, map[string][]byte{
		"band.json": band, "repertoire.json": rep, "annotations/cet-air.json": ann,
	})

	st := newStack()
	importer, _ := st.svc.Register("owner", "Owner", "password123", "")
	report, err := st.svc.ImportBand(importer, st.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	songs, _ := st.repo.SongsOfBand(report.Band.ID)
	if len(songs) != 1 || songs[0].Slug != "cet-air" {
		t.Fatalf("imported song slug = %q, want cet-air", songs[0].Slug)
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
	if updated.Slug != "cet-air" {
		t.Fatalf("title edit renamed the slug to %q — reference break", updated.Slug)
	}

	// export: the annotation file and setlist item still point at the stable slug
	out, _, err := st.svc.ExportBand(importer, st.eng, report.Band.ID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	entries := unzip(t, out)
	if _, ok := entries["annotations/cet-air.json"]; !ok {
		t.Fatalf("annotation file renamed off its slug; entries=%v", keysOf(entries))
	}
	var rep2 struct {
		Songs []struct{ Slug string } `json:"songs"`
	}
	mustJSON(t, entries["repertoire.json"], &rep2)
	if len(rep2.Songs) != 1 || rep2.Songs[0].Slug != "cet-air" {
		t.Fatalf("exported slug = %+v, want cet-air", rep2.Songs)
	}
	var sls struct {
		Setlists []struct {
			Items []struct{ Song string } `json:"items"`
		} `json:"setlists"`
	}
	mustJSON(t, entries["setlists.json"], &sls)
	if len(sls.Setlists) != 1 || len(sls.Setlists[0].Items) != 1 || sls.Setlists[0].Items[0].Song != "cet-air" {
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
