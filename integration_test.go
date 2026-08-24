package fb2c

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/htol/fb2c/fb2"
)

// End-to-end API tests on the fixture corpus. Byte-level output correctness
// is covered by golden_test.go; these tests exercise the converter API,
// metadata extraction and parse validation.

const (
	srcRefTitle  = "Тестовый ознакомительный документ FictionBook 2.1"
	srcRefAuthor = "Дмитрий Петрович Грибов"
)

var srcRefFixture = filepath.Join(fixtureDir, "src_ref.fb2")

// TestConvertCorpusAPI drives the public Converter API over every happy
// fixture for both output formats.
func TestConvertCorpusAPI(t *testing.T) {
	for _, name := range happyFixtures(t) {
		for _, ext := range []string{".mobi", ".epub"} {
			t.Run(name+ext, func(t *testing.T) {
				out := filepath.Join(t.TempDir(), name+ext)
				if err := NewConverter().Convert(filepath.Join(fixtureDir, name+".fb2"), out); err != nil {
					t.Fatalf("Convert() failed: %v", err)
				}
				info, err := os.Stat(out)
				if err != nil {
					t.Fatalf("output not created: %v", err)
				}
				if info.Size() == 0 {
					t.Error("output is empty")
				}
			})
		}
	}
}

// TestCoverExtraction verifies parser-level cover extraction from cover.fb2.
func TestCoverExtraction(t *testing.T) {
	parser := fb2.NewParser()
	data, err := os.ReadFile(filepath.Join(fixtureDir, "cover.fb2"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	fb2Doc, err := parser.ParseBytes(data)
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	metadata, err := fb2.ExtractMetadata(fb2Doc, parser)
	if err != nil {
		t.Fatalf("ExtractMetadata failed: %v", err)
	}
	if metadata.CoverID == "" {
		t.Error("cover ID not extracted")
	}
	if len(metadata.Cover) == 0 {
		t.Error("cover image not extracted")
	}
}

// TestMetadataExtraction checks metadata against known fixture values.
func TestMetadataExtraction(t *testing.T) {
	tests := []struct {
		file       string
		wantTitle  string
		wantAuthor string
	}{
		{file: srcRefFixture, wantTitle: srcRefTitle, wantAuthor: srcRefAuthor},
		{file: filepath.Join(fixtureDir, "minimal.fb2"), wantTitle: "Минимальная книга", wantAuthor: "Иван Тестовый"},
	}

	for _, tt := range tests {
		t.Run(filepath.Base(tt.file), func(t *testing.T) {
			metadata, err := fb2.GetMetadataFromFile(tt.file)
			if err != nil {
				t.Fatalf("GetMetadataFromFile() failed: %v", err)
			}
			if metadata.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", metadata.Title, tt.wantTitle)
			}
			found := slices.Contains(metadata.Authors, tt.wantAuthor)
			if !found {
				t.Errorf("author %q not found in %v", tt.wantAuthor, metadata.Authors)
			}
		})
	}
}

// TestConvertOptions exercises the conversion option matrix on a corpus file.
func TestConvertOptions(t *testing.T) {
	quietLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tests := []struct {
		name    string
		options ConvertOptions
	}{
		{name: "MOBI 6 only", options: ConvertOptions{MobiType: "old", Logger: quietLogger}},
		{name: "KF8 only", options: ConvertOptions{MobiType: "new", Logger: quietLogger}},
		{name: "Joint", options: ConvertOptions{MobiType: "both", Compression: false, Logger: quietLogger}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "opts.mobi")
			converter := NewConverter()
			converter.SetOptions(tt.options)
			if err := converter.Convert(filepath.Join(fixtureDir, "minimal.fb2"), out); err != nil {
				t.Fatalf("Convert() failed: %v", err)
			}
			if info, err := os.Stat(out); err != nil || info.Size() < 100 {
				t.Errorf("output missing or too small: %v, %v", info, err)
			}
		})
	}
}

// TestValidateFB2 checks parse validation, including negative fixtures that
// must fail at parse time.
func TestValidateFB2(t *testing.T) {
	tests := []struct {
		file    string
		wantErr bool
	}{
		{file: srcRefFixture, wantErr: false},
		{file: filepath.Join(fixtureDir, "minimal.fb2"), wantErr: false},
		{file: filepath.Join(fixtureDir, "broken_xml.fb2"), wantErr: true},
		{file: filepath.Join(fixtureDir, "bad_base64.fb2"), wantErr: true},
		{file: "nonexistent.fb2", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(filepath.Base(tt.file), func(t *testing.T) {
			parser := fb2.NewParser()
			_, err := parser.ParseFile(tt.file)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// BenchmarkConversion benchmarks the conversion process.
func BenchmarkConversion(b *testing.B) {
	input := filepath.Join(fixtureDir, "src_ref.fb2")
	output := filepath.Join(b.TempDir(), "bench.mobi")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := NewConverter().Convert(input, output); err != nil {
			b.Fatalf("Convert() failed: %v", err)
		}
	}
}
