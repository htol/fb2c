package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/htol/fb2c"
	"github.com/htol/fb2c/mobi"
)

// Corpus layout: hand-written inputs under testdata/fb2, generated goldens
// under testdata/golden. regen-testdata regenerates goldens ONLY; fixture
// inputs are never written by tooling (docs/TESTING.md Q13, Q16).
const (
	fixtureDir = "testdata/fb2"
	goldenRoot = "testdata/golden"
)

// regenTestdataCmd implements `fb2c regen-testdata`.
func regenTestdataCmd(args []string) {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "Error: regen-testdata takes no arguments")
		printUsage()
		os.Exit(1)
	}
	if _, err := os.Stat(fixtureDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s not found (run from the repository root): %v\n", fixtureDir, err)
		os.Exit(1)
	}

	fixtures, err := filepath.Glob(filepath.Join(fixtureDir, "*.fb2"))
	exitOnError(err)
	sort.Strings(fixtures)

	for _, dir := range []string{
		filepath.Join(goldenRoot, "mobi6"),
		filepath.Join(goldenRoot, "epub"),
		filepath.Join(goldenRoot, "negative"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			exitOnError(fmt.Errorf("failed to create %s: %w", dir, err))
		}
	}

	var converted, failed int
	for _, fixture := range fixtures {
		name := strings.TrimSuffix(filepath.Base(fixture), ".fb2")

		mobiPath := filepath.Join(goldenRoot, "mobi6", name+".mobi")
		epubPath := filepath.Join(goldenRoot, "epub", name+".epub")

		err := convertTo(fixture, mobiPath)
		if err != nil {
			// Expected failures are themselves goldens (docs/TESTING.md Q15).
			if writeErr := os.WriteFile(
				filepath.Join(goldenRoot, "negative", name+".txt"),
				[]byte(err.Error()+"\n"), 0o644); writeErr != nil {
				exitOnError(writeErr)
			}
			fmt.Printf("  %-24s conversion fails (negative golden): %v\n", name, err)
			failed++
			continue
		}
		exitOnError(convertTo(fixture, epubPath))

		// Content goldens: what a human diffs when a byte golden breaks.
		mobiData, readErr := os.ReadFile(mobiPath)
		exitOnError(readErr)
		rawml, extractErr := mobi.ExtractRawML(mobiData)
		exitOnError(extractErr)
		exitOnError(os.WriteFile(
			filepath.Join(goldenRoot, "mobi6", name+".rawml"),
			[]byte(rawml), 0o644))

		epubData, readErr := os.ReadFile(epubPath)
		exitOnError(readErr)
		exitOnError(os.WriteFile(
			filepath.Join(goldenRoot, "epub", name+".txt"),
			[]byte(EPUBTextListing(epubData)), 0o644))

		fmt.Printf("  %-24s mobi6 + epub goldens regenerated\n", name)
		converted++
	}

	fmt.Printf("done: %d converted, %d negative goldens\n", converted, failed)
}

// convertTo converts an FB2 fixture to the output format selected by the
// golden file extension, using the default conversion options.
func convertTo(input, output string) error {
	converter := fb2c.NewConverter()
	return converter.Convert(input, output)
}

// EPUBTextListing renders the text members of an EPUB archive as readable
// text; binary members are listed with their size only.
func EPUBTextListing(epubData []byte) string {
	r, err := zip.NewReader(bytes.NewReader(epubData), int64(len(epubData)))
	if err != nil {
		return fmt.Sprintf("failed to open EPUB archive: %v\n", err)
	}

	var b strings.Builder
	for _, f := range r.File {
		fmt.Fprintf(&b, "=== %s (%d bytes) ===\n", f.Name, f.UncompressedSize64)
		if !isTextMember(f.Name) {
			fmt.Fprintf(&b, "[binary]\n\n")
			continue
		}
		rc, err := f.Open()
		if err != nil {
			fmt.Fprintf(&b, "[open error: %v]\n\n", err)
			continue
		}
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(rc); err != nil {
			fmt.Fprintf(&b, "[read error: %v]\n\n", err)
			rc.Close()
			continue
		}
		rc.Close()
		b.WriteString(buf.String())
		if !strings.HasSuffix(buf.String(), "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// isTextMember reports whether an EPUB archive member is human-readable text.
func isTextMember(name string) bool {
	switch filepath.Ext(name) {
	case ".xml", ".opf", ".ncx", ".xhtml", ".html", ".css", ".txt":
		return true
	}
	return name == "mimetype"
}
