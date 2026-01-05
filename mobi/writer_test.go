package mobi

import (
	"bytes"
	"testing"

	"github.com/htol/fb2c/opf"
)

func TestNewWriter(t *testing.T) {
	book := opf.NewOEBBook()
	book.Metadata = opf.Metadata{
		Title: "Test Book",
	}

	writer := NewWriter(book)
	if writer == nil {
		t.Fatal("NewWriter() returned nil")
	}

	if writer.book != book {
		t.Error("Writer book not set correctly")
	}
}

func TestDefaultWriteOptions(t *testing.T) {
	opts := DefaultWriteOptions()

	if opts.CompressionType != NoCompression {
		t.Errorf("CompressionType = %v, want %d (NoCompression)", opts.CompressionType, NoCompression)
	}

	if !opts.WithEXTH {
		t.Error("WithEXTH should be true by default")
	}

	if !opts.GenerateTOC {
		t.Error("GenerateTOC should be true by default")
	}
}

func TestGetBookName(t *testing.T) {
	book := opf.NewOEBBook()
	book.Metadata = opf.Metadata{
		Title: "Test Book Title",
	}

	writer := NewWriter(book)

	// Test with default title
	name := writer.GetBookName()
	if name != "Test Book Title" {
		t.Errorf("GetBookName() = %v, want 'Test Book Title'", name)
	}

	// Test with custom title
	writer.options.Title = "Custom Title"
	name = writer.GetBookName()
	if name != "Custom Title" {
		t.Errorf("GetBookName() with custom = %v, want 'Custom Title'", name)
	}

	// Test long title truncation
	book.Metadata.Title = "This is a very long book title that should be truncated to 31 characters"
	writer.options.Title = ""
	name = writer.GetBookName()
	if len(name) > 31 {
		t.Errorf("GetBookName() length = %v, want max 31", len(name))
	}
}

func TestSplitTextRecords(t *testing.T) {
	writer := NewWriter(opf.NewOEBBook())

	tests := []struct {
		name     string
		data     []byte
		wantRecs int
	}{
		{
			name:     "empty",
			data:     []byte{},
			wantRecs: 0,
		},
		{
			name:     "single byte",
			data:     []byte("A"),
			wantRecs: 1,
		},
		{
			name:     "exactly one record",
			data:     make([]byte, 4096),
			wantRecs: 1,
		},
		{
			name:     "one record plus one byte",
			data:     make([]byte, 4097),
			wantRecs: 2,
		},
		{
			name:     "multiple records",
			data:     make([]byte, 10000),
			wantRecs: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records := writer.splitTextRecords(tt.data)
			if len(records) != tt.wantRecs {
				t.Errorf("splitTextRecords() returned %v records, want %v", len(records), tt.wantRecs)
			}

			// Verify each record is max 4096 bytes (except possibly last)
			for i, rec := range records {
				if len(rec) > 4096 {
					t.Errorf("Record %v has length %v, want max 4096", i, len(rec))
				}
			}
		})
	}
}

func TestCalculateRecordCount(t *testing.T) {
	tests := []struct {
		textSize int
		want     int
	}{
		{0, 0},
		{1, 1},
		{4096, 1},
		{4097, 2},
		{8192, 2},
		{8193, 3},
		{10000, 3},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := CalculateRecordCount(tt.textSize)
			if got != tt.want {
				t.Errorf("CalculateRecordCount(%v) = %v, want %v", tt.textSize, got, tt.want)
			}
		})
	}
}

func TestConvertOEBToMOBI(t *testing.T) {
	book := opf.NewOEBBook()
	book.Metadata = opf.Metadata{
		Title:       "Test Book",
		Language:    "en",
		Publisher:   "Test Publisher",
		ISBN:        "978-0-123456-78-9",
		Annotation:  "Test annotation",
	}
	book.Metadata.Authors = []opf.Author{
		opf.NewAuthor("John", "", "Doe", ""),
	}

	// Add some content
	book.Content = "<html><body><h1>Chapter 1</h1><p>Test content</p></body></html>"

	var output bytes.Buffer
	err := ConvertOEBToMOBI(book, &output)
	if err != nil {
		t.Fatalf("ConvertOEBToMOBI() error = %v", err)
	}

	// Check that we got some output
	if output.Len() == 0 {
		t.Error("ConvertOEBToMOBI() produced no output")
	}

	// Check for PalmDB type/creator
	outputBytes := output.Bytes()
	if len(outputBytes) < 78 {
		t.Error("Output too short to contain PalmDB header")
	} else {
		// Check for "BOOK" type at offset 60-63
		typeStr := string(outputBytes[60:64])
		if typeStr != "BOOK" {
			t.Errorf("PalmDB type = %v, want 'BOOK'", typeStr)
		}

		// Check for "MOBI" creator at offset 64-67
		creatorStr := string(outputBytes[64:68])
		if creatorStr != "MOBI" {
			t.Errorf("PalmDB creator = %v, want 'MOBI'", creatorStr)
		}
	}

	t.Logf("Generated MOBI file size: %d bytes", output.Len())
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		strs     []string
		sep      string
		expected string
	}{
		{[]string{}, ", ", ""},
		{[]string{"a"}, ", ", "a"},
		{[]string{"a", "b"}, ", ", "a, b"},
		{[]string{"a", "b", "c"}, "-", "a-b-c"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := joinStrings(tt.strs, tt.sep)
			if got != tt.expected {
				t.Errorf("joinStrings(%v, %q) = %q, want %q", tt.strs, tt.sep, got, tt.expected)
			}
		})
	}
}

func TestEXTHWriter(t *testing.T) {
	writer := NewEXTHWriter()

	writer.AddAuthor("Test Author")
	writer.AddPublisher("Test Publisher")
	writer.AddDescription("Test description")
	writer.AddISBN("978-0-123456-78-9")

	var buf bytes.Buffer
	n, err := writer.Write(&buf)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if n == 0 {
		t.Error("Write() returned 0 bytes")
	}

	// Check for EXTH identifier
	data := buf.Bytes()
	if len(data) < 4 {
		t.Error("EXTH data too short")
	} else if string(data[0:4]) != "EXTH" {
		t.Errorf("EXTH identifier = %v, want 'EXTH'", string(data[0:4]))
	}
}

func TestPalmDOCCompression(t *testing.T) {
	tests := []struct {
		name string
		input string
	}{
		{"empty", ""},
		{"short", "Hello"},
		{"repeated", "AAAAABBBBBCCCCC"},
		{"spaces", "Hello World Test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte(tt.input)
			compressed := CompressPalmDOC(input)

			// Compressed data should not be larger than 2x original (rough check)
			if len(compressed) > len(input)*2 && len(input) > 100 {
				t.Errorf("Compression ratio too poor: %d -> %d", len(input), len(compressed))
			}

			// For empty input, should get empty output
			if len(input) == 0 && len(compressed) != 0 {
				t.Errorf("Empty input should produce empty output, got %d bytes", len(compressed))
			}
		})
	}
}
