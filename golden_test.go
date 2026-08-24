package fb2c

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/htol/fb2c/epub"
	"github.com/htol/fb2c/fb2"
	"github.com/htol/fb2c/mobi"
)

// Golden tests are the regression oracle: every fixture in testdata/fb2 must
// convert byte-identically to its committed golden in testdata/golden
// (docs/TESTING.md). Negative fixtures must fail with the recorded error.

const (
	fixtureDir = "testdata/fb2"
	goldenDir  = "testdata/golden"
)

// fixtureNames returns all fixture basenames without extension.
func fixtureNames(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(fixtureDir, "*.fb2"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures found in %s: %v", fixtureDir, err)
	}
	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = strings.TrimSuffix(filepath.Base(p), ".fb2")
	}
	return names
}

// happyFixtures returns fixtures that must convert successfully: those
// without a negative (error) golden.
func happyFixtures(t *testing.T) []string {
	t.Helper()
	var happy []string
	for _, name := range fixtureNames(t) {
		if _, err := os.Stat(filepath.Join(goldenDir, "negative", name+".txt")); os.IsNotExist(err) {
			happy = append(happy, name)
		}
	}
	if len(happy) == 0 {
		t.Fatal("no happy fixtures found")
	}
	return happy
}

// convertFixture converts a fixture to the format chosen by ext in a temp
// directory and returns the output bytes.
func convertFixture(t *testing.T, name, ext string) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), name+ext)
	converter := NewConverter()
	if err := converter.Convert(filepath.Join(fixtureDir, name+".fb2"), out); err != nil {
		t.Fatalf("conversion failed: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	return data
}

func readGolden(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read golden %s: %v", path, err)
	}
	return data
}

// TestGoldenMOBI6 compares full generated MOBI files byte-by-byte.
func TestGoldenMOBI6(t *testing.T) {
	for _, name := range happyFixtures(t) {
		t.Run(name, func(t *testing.T) {
			got := convertFixture(t, name, ".mobi")
			want := readGolden(t, filepath.Join(goldenDir, "mobi6", name+".mobi"))

			if !bytes.Equal(got, want) {
				t.Errorf("MOBI output differs from golden: first difference at offset %d (got %d bytes, golden %d bytes)",
					firstDiffOffset(got, want), len(got), len(want))
			}
		})
	}
}

// TestRoundTripMOBI6 is the validity oracle: read the generated MOBI back
// through our own reader; the extracted rawml must equal the content golden,
// and header fields must match the source document semantically.
func TestRoundTripMOBI6(t *testing.T) {
	for _, name := range happyFixtures(t) {
		t.Run(name, func(t *testing.T) {
			got := convertFixture(t, name, ".mobi")

			rawml, err := mobi.ExtractRawML(got)
			if err != nil {
				t.Fatalf("ExtractRawML failed: %v", err)
			}
			want := readGolden(t, filepath.Join(goldenDir, "mobi6", name+".rawml"))
			if rawml != string(want) {
				t.Errorf("extracted rawml differs from content golden (got %d bytes, golden %d bytes)",
					len(rawml), len(want))
			}

			// The header must carry the book's FULL title, untruncated
			// (regression: byte-level truncation used to split UTF-8 codepoints).
			metadata, err := fb2.GetMetadataFromFile(filepath.Join(fixtureDir, name+".fb2"))
			if err != nil {
				t.Fatalf("GetMetadataFromFile failed: %v", err)
			}
			dump, err := mobi.ReadDump(got)
			if err != nil {
				t.Fatalf("ReadDump failed: %v", err)
			}
			if dump.MOBI.FullName != metadata.Title {
				t.Errorf("FullName = %q, want full title %q", dump.MOBI.FullName, metadata.Title)
			}
		})
	}
}

// TestGoldenEPUB compares full generated EPUB files byte-by-byte and their
// readable text listing against the content golden.
func TestGoldenEPUB(t *testing.T) {
	for _, name := range happyFixtures(t) {
		t.Run(name, func(t *testing.T) {
			got := convertFixture(t, name, ".epub")
			want := readGolden(t, filepath.Join(goldenDir, "epub", name+".epub"))

			if !bytes.Equal(got, want) {
				t.Errorf("EPUB output differs from golden: first difference at offset %d (got %d bytes, golden %d bytes)",
					firstDiffOffset(got, want), len(got), len(want))
			}

			listing, err := epub.TextListing(got)
			if err != nil {
				t.Fatalf("TextListing failed: %v", err)
			}
			wantListing := readGolden(t, filepath.Join(goldenDir, "epub", name+".txt"))
			if listing != string(wantListing) {
				t.Errorf("EPUB text listing differs from content golden")
			}
		})
	}
}

// TestGoldenNegative verifies that corrupt fixtures fail with exactly the
// recorded error message.
func TestGoldenNegative(t *testing.T) {
	for _, name := range fixtureNames(t) {
		goldenPath := filepath.Join(goldenDir, "negative", name+".txt")
		want, err := os.ReadFile(goldenPath)
		if err != nil {
			continue // happy fixture, covered above
		}

		t.Run(name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), name+".mobi")
			converter := NewConverter()
			gotErr := converter.Convert(filepath.Join(fixtureDir, name+".fb2"), out)
			if gotErr == nil {
				t.Fatalf("conversion succeeded, want failure with %q", strings.TrimSpace(string(want)))
			}
			if gotErr.Error() != strings.TrimSpace(string(want)) {
				t.Errorf("error = %q, want %q", gotErr.Error(), strings.TrimSpace(string(want)))
			}
		})
	}
}

// firstDiffOffset returns the offset of the first differing byte, or the
// length of the shorter input when one is a prefix of the other.
func firstDiffOffset(a, b []byte) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
