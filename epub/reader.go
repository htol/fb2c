// Package epub: EPUB archive reading for content extraction.
package epub

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// TextListing renders the text members of an EPUB archive as readable text
// (binary members are listed with their size only). This is the content
// golden format a human diffs when an EPUB byte golden breaks.
func TextListing(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("failed to open EPUB archive: %w", err)
	}

	var b strings.Builder
	for _, f := range r.File {
		fmt.Fprintf(&b, "=== %s (%d bytes) ===\n", f.Name, f.UncompressedSize64)
		if !isTextMember(f.Name) {
			b.WriteString("[binary]\n\n")
			continue
		}
		content, err := readMember(f)
		if err != nil {
			fmt.Fprintf(&b, "[read error: %v]\n\n", err)
			continue
		}
		b.Write(content)
		if !bytes.HasSuffix(content, []byte("\n")) {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func readMember(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// isTextMember reports whether an EPUB archive member is human-readable text.
func isTextMember(name string) bool {
	switch filepath.Ext(name) {
	case ".xml", ".opf", ".ncx", ".xhtml", ".html", ".css", ".txt":
		return true
	}
	return name == "mimetype"
}
