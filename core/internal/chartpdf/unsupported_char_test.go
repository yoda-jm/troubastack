package chartpdf

import (
	"strings"
	"testing"
)

// VLL, 2026-09-09, after pasting a lyric sheet: the error named the rune and nothing else, and "message
// should point a line or things arround it to be useful". This failure mode is the one where naming alone
// hurts most — the offenders are CONFUSABLES, drawn identically to ASCII in every editor, so the author is
// told a character is wrong and then cannot see it.

func TestValidateChars_NamesTheLineColumnAndMarksTheCharacter(t *testing.T) {
	// A Cyrillic е (U+0435) on the fourth line, in the middle of ordinary words.
	src := "# Title\nArtist\n\nthe big day may b\u0435 tonight\nand a line after\n"
	err := validateChars(src)
	if err == nil {
		t.Fatal("a Cyrillic е must be refused")
	}
	msg := err.Error()
	for _, want := range []string{
		"line 4",           // WHERE, which is the whole point of the change
		"column 18",        // …to the character
		"b[\u0435]",        // the character MARKED, because it is invisible on its own
		"U+0435",           // the code point, for anyone grepping
		"retype it as 'e'", // what to do about it
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
	// The surrounding words must be there — that is what "things around it" meant.
	if !strings.Contains(msg, "the big day may b") {
		t.Errorf("message does not show the text around the character:\n%s", msg)
	}
}

func TestValidateChars_CountsLinesAndColumnsFromOne(t *testing.T) {
	if msg := validateChars("\u0435bc").Error(); !strings.Contains(msg, "line 1, column 1") {
		t.Errorf("the very first rune should be line 1, column 1:\n%s", msg)
	}
	// Column counts RUNES, not bytes: a line of accented Latin-1 before the bad rune must not inflate it.
	if msg := validateChars("éé\u0435").Error(); !strings.Contains(msg, "column 3") {
		t.Errorf("column must count runes, not bytes:\n%s", msg)
	}
}

func TestValidateChars_LongLineIsWindowedAroundTheCharacter(t *testing.T) {
	long := strings.Repeat("la ", 40) + "b\u0435 " + strings.Repeat("da ", 40)
	msg := validateChars("# T\n\n" + long).Error()
	if !strings.Contains(msg, "…") {
		t.Errorf("a long line should be windowed with an ellipsis:\n%s", msg)
	}
	if len(msg) > 400 {
		t.Errorf("message is %d chars — too long to read in a 400 body:\n%s", len(msg), msg)
	}
	if !strings.Contains(msg, "b[\u0435]") {
		t.Errorf("the window must still contain the marked character:\n%s", msg)
	}
}

func TestValidateChars_UnknownRuneStillRefusedWithoutAHint(t *testing.T) {
	// The hint is a courtesy for confusables, never a gate: a rune outside the map is refused just the
	// same, with position and context, and no invented advice.
	msg := validateChars("# T\n\nsome 漢字 here").Error()
	if !strings.Contains(msg, "line 3") || !strings.Contains(msg, "[漢]") {
		t.Errorf("a non-confusable must still be located and marked:\n%s", msg)
	}
	if strings.Contains(msg, "retype it as") {
		t.Errorf("no lookalike advice should be invented for a non-confusable:\n%s", msg)
	}
}

func TestValidateChars_AcceptsWhatItAlwaysDid(t *testing.T) {
	// Latin-1, tabs, newlines and the typographic allowlist stay valid — the change is the MESSAGE, not
	// the rule.
	if err := validateChars("# Café\n\nchœur\tdash – quote ’ ellipsis …\nŠž"); err != nil {
		t.Fatalf("valid chart refused: %v", err)
	}
}
