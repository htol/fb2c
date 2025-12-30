package fb2c

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/htol/fb2c/fb2"
	"github.com/htol/fb2c/mobi"
	"github.com/htol/fb2c/opf"
)

// TestConvertSimpleFB2 tests end-to-end conversion of a simple FB2 file
func TestConvertSimpleFB2(t *testing.T) {
	testFile := "testdata/simple.fb2"

	// Parse FB2
	parser := fb2.NewParser()

	// Read the file
	fb2Data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read FB2 file: %v", err)
	}

	fb2Doc, err := parser.ParseBytes(fb2Data)
	if err != nil {
		t.Fatalf("Failed to parse FB2: %v", err)
	}

	// Extract metadata
	metadata, err := parser.ExtractMetadata(fb2Doc)
	if err != nil {
		t.Fatalf("Failed to extract metadata: %v", err)
	}

	// Verify metadata
	if metadata.Title != "Foundation" {
		t.Errorf("Title = %v, want 'Foundation'", metadata.Title)
	}

	if len(metadata.Authors) != 1 {
		t.Fatalf("Author count = %v, want 1", len(metadata.Authors))
	}

	if metadata.Authors[0] != "Isaac Asimov" {
		t.Errorf("Author = %v, want 'Isaac Asimov'", metadata.Authors[0])
	}

	if metadata.Publisher != "Gnome Press" {
		t.Errorf("Publisher = %v, want 'Gnome Press'", metadata.Publisher)
	}

	if metadata.Series != "Foundation Series" {
		t.Errorf("Series = %v, want 'Foundation Series'", metadata.Series)
	}

	if metadata.SeriesIndex != 1 {
		t.Errorf("SeriesIndex = %v, want 1", metadata.SeriesIndex)
	}

	// Transform to HTML
	transformer := fb2.NewTransformer()
	html, _, _, err := transformer.ConvertBytes(fb2Data)
	if err != nil {
		t.Fatalf("Failed to transform FB2: %v", err)
	}

	if html == "" {
		t.Error("HTML output is empty")
	}

	// Convert to MOBI
	book := opf.NewOEBBook()
	book.Metadata = opf.ConvertMetadataFromFB2(
		metadata.Title,
		metadata.Authors,
		metadata.AuthorSort,
		metadata.Publisher,
		metadata.ISBN,
		metadata.Year,
		metadata.Language,
		metadata.PubDate,
		metadata.Series,
		metadata.SeriesIndex,
		metadata.Genres,
		metadata.Keywords,
		metadata.Annotation,
		metadata.Cover,
		metadata.CoverID,
		metadata.CoverExt,
	)
	book.Content = html

	// Generate MOBI
	var output bytes.Buffer
	err = mobi.ConvertOEBToMOBI(book, &output)
	if err != nil {
		t.Fatalf("Failed to convert to MOBI: %v", err)
	}

	// Verify MOBI output
	mobiData := output.Bytes()
	if len(mobiData) < 78 {
		t.Fatalf("MOBI output too short: %d bytes", len(mobiData))
	}

	// Check PalmDB header
	if string(mobiData[60:64]) != "BOOK" {
		t.Errorf("PalmDB type = %v, want 'BOOK'", string(mobiData[60:64]))
	}

	if string(mobiData[64:68]) != "MOBI" {
		t.Errorf("PalmDB creator = %v, want 'MOBI'", string(mobiData[64:68]))
	}

	t.Logf("Generated MOBI: %d bytes", len(mobiData))
}

// TestConvertFB2WithCover tests conversion with cover image
func TestConvertFB2WithCover(t *testing.T) {
	testFile := "testdata/with_coverpage.fb2"

	// Read the file
	fb2Data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read FB2 file: %v", err)
	}

	parser := fb2.NewParser()
	parser.ExtractImages = true // Enable image extraction
	fb2Doc, err := parser.ParseBytes(fb2Data)
	if err != nil {
		t.Fatalf("Failed to parse FB2: %v", err)
	}

	metadata, err := parser.ExtractMetadata(fb2Doc)
	if err != nil {
		t.Fatalf("Failed to extract metadata: %v", err)
	}

	// Verify cover was extracted
	if metadata.CoverID == "" {
		t.Error("Cover ID not found")
	}

	if len(metadata.Cover) == 0 {
		t.Error("Cover image not extracted")
	}

	// Verify annotation
	if metadata.Annotation == "" {
		t.Error("Annotation not extracted")
	}

	t.Logf("Cover: %s, %d bytes, %s", metadata.CoverID, len(metadata.Cover), metadata.CoverExt)
}

// TestConverterEndToEnd tests the full converter API
func TestConverterEndToEnd(t *testing.T) {
	converter := NewConverter()

	// Convert simple FB2
	inputFile := "testdata/simple.fb2"
	outputFile := filepath.Join(os.TempDir(), "test_output.mobi")
	defer os.Remove(outputFile)

	err := converter.Convert(inputFile, outputFile)
	if err != nil {
		t.Fatalf("Convert() failed: %v", err)
	}

	// Verify output file exists
	info, err := os.Stat(outputFile)
	if err != nil {
		t.Fatalf("Output file not created: %v", err)
	}

	if info.Size() == 0 {
		t.Error("Output file is empty")
	}

	// Read and verify output
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	if len(data) < 100 {
		t.Errorf("Output too small: %d bytes", len(data))
	}

	// Check for PalmDB signature
	if string(data[60:64]) != "BOOK" {
		t.Errorf("Invalid PalmDB type: %s", string(data[60:64]))
	}

	t.Logf("Generated MOBI: %s (%d bytes)", outputFile, info.Size())
}

// TestConverterOptions tests different conversion options
func TestConverterOptions(t *testing.T) {
	tests := []struct {
		name    string
		options ConvertOptions
	}{
		{
			name: "MOBI 6 only",
			options: ConvertOptions{
				MobiType:    "old",
				Compression: true,
			},
		},
		{
			name: "KF8 only",
			options: ConvertOptions{
				MobiType:        "new",
				Compression:     true,
				EnableChunking:  true,
				TargetChunkSize: 8192,
			},
		},
		{
			name: "No compression",
			options: ConvertOptions{
				MobiType:    "both",
				Compression: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewConverter()
			converter.SetOptions(tt.options)

			inputFile := "testdata/simple.fb2"
			outputFile := filepath.Join(os.TempDir(), "test_opts.mobi")
			defer os.Remove(outputFile)

			err := converter.Convert(inputFile, outputFile)
			if err != nil {
				t.Fatalf("Convert() failed: %v", err)
			}

			// Verify output
			info, err := os.Stat(outputFile)
			if err != nil {
				t.Fatalf("Output file not created: %v", err)
			}

			if info.Size() < 100 {
				t.Errorf("Output too small: %d bytes", info.Size())
			}

			t.Logf("Generated with options '%s': %d bytes", tt.name, info.Size())
		})
	}
}

// TestMetadataExtraction tests metadata extraction from various files
func TestMetadataExtraction(t *testing.T) {
	tests := []struct {
		file     string
		wantTitle string
		wantAuthor string
		wantSeries string
	}{
		{
			file:     "testdata/simple.fb2",
			wantTitle: "Foundation",
			wantAuthor: "Isaac Asimov",
			wantSeries: "Foundation Series",
		},
		{
			file:     "testdata/with_cover.fb2",
			wantTitle: "Neuromancer",
			wantAuthor: "William Ford Gibson",
			wantSeries: "Sprawl Trilogy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			metadata, err := ExtractMetadata(tt.file)
			if err != nil {
				t.Fatalf("ExtractMetadata() failed: %v", err)
			}

			if metadata.Title != tt.wantTitle {
				t.Errorf("Title = %v, want %v", metadata.Title, tt.wantTitle)
			}

			foundAuthor := false
			for _, author := range metadata.Authors {
				if author == tt.wantAuthor {
					foundAuthor = true
					break
				}
			}
			if !foundAuthor {
				t.Errorf("Author %v not found in %v", tt.wantAuthor, metadata.Authors)
			}

			if metadata.Series != tt.wantSeries {
				t.Errorf("Series = %v, want %v", metadata.Series, tt.wantSeries)
			}

			t.Logf("✓ %s: '%s' by %s", tt.file, metadata.Title, metadata.Authors)
		})
	}
}

// TestValidateFB2 validates FB2 file structure
func TestValidateFB2(t *testing.T) {
	tests := []struct {
		file    string
		wantErr bool
	}{
		{
			file:    "testdata/simple.fb2",
			wantErr: false,
		},
		{
			file:    "testdata/with_cover.fb2",
			wantErr: false,
		},
		{
			file:    "nonexistent.fb2",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			err := ValidateFB2(tt.file)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFB2() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// BenchmarkConversion benchmarks the conversion process
func BenchmarkConversion(b *testing.B) {
	testFile := "testdata/simple.fb2"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		converter := NewConverter()
		inputFile := testFile
		outputFile := filepath.Join(os.TempDir(), "bench.mobi")

		err := converter.Convert(inputFile, outputFile)
		if err != nil {
			b.Fatalf("Convert() failed: %v", err)
		}

		os.Remove(outputFile)
	}
}
