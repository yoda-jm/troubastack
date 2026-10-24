package soundsheet

import (
	"bytes"
	"encoding/csv"
	"strings"

	"troubastack/core/internal/app"
)

// subcolLabel is the CSV/column label for a channel: "Ann · Voice", "Ann · Guitar", "Bass (shared)".
func subcolLabel(sc app.SheetSubcol) string {
	if sc.Kind == "shared" {
		return sc.Channel + " (shared)"
	}
	// OwnerName is the musician name here (not a shared header).
	return sc.OwnerName + " · " + sc.Channel
}

// RenderCSV renders the sheet model to CSV: one column per sub-column (Fable's "screen = PDF = CSV"), plus the
// running number, title, changes and sound-note. A cell shows the channel text, parenthesised when optional.
func RenderCSV(m app.SheetModel) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	head := []string{"#", "Title"}
	for _, sc := range m.Subcols {
		head = append(head, subcolLabel(sc))
	}
	head = append(head, "Changes", "Sound note")
	_ = w.Write(head)

	for _, row := range m.Rows {
		rec := []string{itoa(row.Position), row.Title}
		for _, sc := range m.Subcols {
			c := row.Cells[sc.Key]
			txt := c.Text
			if c.Optional && txt != "" {
				txt = "(" + txt + ")"
			}
			rec = append(rec, txt)
		}
		rec = append(rec, strings.Join(row.Changes, "; "), row.SoundNote)
		_ = w.Write(rec)
	}
	w.Flush()
	return buf.Bytes()
}

func itoa(n int) string {
	if n == 0 {
		return ""
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
