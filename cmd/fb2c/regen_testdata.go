package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/htol/fb2c"
	"github.com/htol/fb2c/epub"
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
		listing, listErr := epub.TextListing(epubData)
		exitOnError(listErr)
		exitOnError(os.WriteFile(
			filepath.Join(goldenRoot, "epub", name+".txt"),
			[]byte(listing), 0o644))

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
