// Package pdftest extracts text from PDF bytes via poppler's pdftotext, for tests that assert on rendered
// content (T205). CI installs poppler-utils; Available lets a test skip where it is absent. It imports no
// testing, so only test binaries that import it pull it in.
package pdftest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Available reports whether pdftotext is on PATH.
func Available() bool {
	_, err := exec.LookPath("pdftotext")
	return err == nil
}

// Text returns the whole document's text (layout-preserving). Pages are separated by form feed (\f).
func Text(pdf []byte) (string, error) {
	dir, err := os.MkdirTemp("", "pdftest")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	f := filepath.Join(dir, "in.pdf")
	if err := os.WriteFile(f, pdf, 0o600); err != nil {
		return "", err
	}
	out, err := exec.Command("pdftotext", "-layout", f, "-").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Pages returns the per-page text (split on the form feed pdftotext emits between pages). A trailing empty
// element from the final form feed is dropped.
func Pages(pdf []byte) ([]string, error) {
	t, err := Text(pdf)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(t, "\f")
	for len(parts) > 0 && strings.TrimSpace(parts[len(parts)-1]) == "" {
		parts = parts[:len(parts)-1]
	}
	return parts, nil
}
